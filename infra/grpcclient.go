package infra

import (
	"context"
	"dogapm/internal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"
)

type GrpcClient struct {
	*grpc.ClientConn
}

func NewGrpcClient(addr string, server string) *GrpcClient {
	conn, err := grpc.Dial(addr,
		grpc.WithUnaryInterceptor(unaryInterceptor(server)),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	return &GrpcClient{conn}
}

const (
	grpcClientTracerName = "dogapm/grpc_client"
	peerApp              = "peerApp"
	peerHost             = "peerHost"
)

// grpc拦截器
func unaryInterceptor(server string) grpc.UnaryClientInterceptor {
	tracer := otel.Tracer(grpcClientTracerName)
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		//创建span
		ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindClient))
		start := time.Now()
		defer func() {
			//grpc prometheus 埋点
			clientHandleHistogram.WithLabelValues(TypeGrpc, method, server).Observe(time.Since(start).Seconds())
			//记录grpc调用耗时
			span.SetAttributes(attribute.Float64("grpc.duration", time.Since(start).Seconds()))
			span.End()
		}()
		//调用者context中获取metadata,如果获取不到再创建metadata
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.MD{}
		}
		md.Set(peerApp, internal.BuildInfo.AppName())
		md.Set(peerHost, internal.BuildInfo.Hostname())
		//我们上面仅仅初始化了metadata，metadata中并没有trace context信息，想要将trace context信息加入metadata，需要用到otel的传播者
		otel.GetTextMapPropagator().Inject(ctx, &metadataSupplier{metadata: md})
		//将metadata中的信息注入到context中，这样在下面的invoker中的ctx中就可以拿到metadata信息
		ctx = metadata.NewOutgoingContext(ctx, md)
		//grpc prometheus 埋点
		clientHandleCounter.WithLabelValues(TypeGrpc, method, server).Inc()
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			s, _ := status.FromError(err)
			span.RecordError(err, trace.WithTimestamp(time.Now()), trace.WithStackTrace(true))
			span.SetAttributes(
				attribute.Bool("error", true),
				attribute.String("grpc.status_code", s.Code().String()))
		}
		return err
	}
}
