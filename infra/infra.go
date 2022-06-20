package infra

import (
	"context"
	"database/sql"
	"dogapm/internal"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/google/gops/agent"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"mosn.io/holmes"
	"os"
	"path/filepath"
	"time"
)

// infrastructure 基础设施
type infra struct {
	Db  *sql.DB
	Rdb *redis.Client
}

var Infra = &infra{}

type InfraOption func(*infra)

// InfraMetricOption 注册metric
func InfraMetricOption(collectors ...prometheus.Collector) InfraOption {
	return func(infra *infra) {
		MetricReg.MustRegister(collectors...)
	}
}

func InfraDbOption(connectUrl string) InfraOption {
	return func(i *infra) {
		sql.Register("mysql-wrap", wrap(mysql.MySQLDriver{}, connectUrl))
		var err error
		i.Db, err = sql.Open("mysql-wrap", connectUrl)
		if err != nil {
			panic(nil)
		}
		err = i.Db.Ping()
		if err != nil {
			panic(err)
		}
	}
}

func InfraRdbOption(addr string) InfraOption {
	return func(i *infra) {
		var err error
		rdb := redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   0,
		})
		rdb.AddHook(&redisHook{})
		res, err := rdb.Ping(context.TODO()).Result()
		if err != nil {
			panic(err)
		}
		if res != "PONG" {
			panic("redis client init fail")
		}
		i.Rdb = rdb
	}
}

func InfraEnableApm(otelEndpoint string, serviceName string, logPathPrefix string, maxLogCnt uint) InfraOption {
	return func(i *infra) {
		ctx := context.Background()
		// 资源
		res, err := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceName(serviceName),
			),
		)
		if err != nil {
			panic(err)
		}
		// 创建 grpc连接，为了连接 otel的collector
		conn, err := grpc.NewClient(otelEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(err)
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		// 创建导出器
		traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(bsp),
		)
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)
		// Use shutdown to flush and close the tracer provider when the application exits
		globalClosers = append(globalClosers, &traceProviderComponent{tracerProvider})
		// logs
		path := filepath.Join(logPathPrefix, internal.BuildInfo.AppName()+".%Y%m%d.log")
		writer, err := rotatelogs.New(path,
			rotatelogs.WithRotationCount(maxLogCnt),
			rotatelogs.WithRotationTime(time.Hour*24),
		)
		if err != nil {
			panic(err)
		}
		// 创建输出，既输出到console 也输出到文件
		consoleWriter := os.Stdout
		logrus.SetOutput(io.MultiWriter(consoleWriter, writer))
	}
}

type traceProviderComponent struct {
	provider *sdktrace.TracerProvider
}

func (t *traceProviderComponent) Close() {
	t.provider.Shutdown(context.Background())
}

func (i *infra) Init(serviceName string, options ...InfraOption) {
	for _, option := range options {
		option(i)
	}
	Tracer = otel.Tracer(serviceName)
}

type AutoPProfOpt struct {
	EnableCPU       bool
	EnableMem       bool
	EnableGoroutine bool
}

type autoPProfReporter struct{}

func (a *autoPProfReporter) Report(pType string, buf []byte, reason string, eventID string) error {
	Logger.Error(
		context.Background(),
		"homesGen",
		map[string]any{
			"reason":  reason,
			"eventID": eventID,
			"pType":   pType,
		},
		errors.New("auto record running state"),
	)
	return nil
}

func EnableAutoPProf(autoPProfOpts *AutoPProfOpt, opts ...holmes.Option) InfraOption {
	if err := agent.Listen(agent.Options{}); err != nil {
		panic(err)
	}
	opts = append(opts, holmes.WithProfileReporter(&autoPProfReporter{}))
	return func(i *infra) {
		h, err := holmes.New(opts...)
		if err == nil && autoPProfOpts != nil {
			if autoPProfOpts.EnableCPU {
				h.EnableCPUDump()
			}
			if autoPProfOpts.EnableMem {
				h.EnableMemDump()
			}
			if autoPProfOpts.EnableGoroutine {
				h.EnableGoroutineDump()
			}
			h.Start()
		}
	}
}
