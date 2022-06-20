package infra

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/xwb1989/sqlparser"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"
)

const (
	mysqlTracerName = "dogapm/mysql"
)

func truncate(query string) string {
	if len(query) > 1024 {
		return query[:1024]
	}
	return query
}

func wrap(d driver.Driver, connectURL string) driver.Driver {
	tracer := otel.Tracer(mysqlTracerName)
	return &Driver{
		Driver: d,
		hooks: Hooks{
			Before: func(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
				ctx = context.WithValue(ctx, "beginTime", time.Now())
				if ctx, span := tracer.Start(ctx, "sqltrace"); span != nil {
					span.SetAttributes(
						attribute.String("sql", truncate(query)),
						attribute.String("param", truncate(fmt.Sprint(args...))),
					)
					return ctx, nil
				}
				return ctx, nil
			},
			After: func(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
				//增加 mysql 单表 qps metric
				//"type", "method", "name", "server"
				name, queryType, err, mutilTable := SqlParser.parseTable(query)
				if err == nil && !mutilTable {
					libraryCounter.WithLabelValues(TypeMySql, sqlparser.StmtType(queryType), name, connectURL).Inc()
				}
				//记录 insert、update、delete sql语句
				switch queryType {
				case sqlparser.StmtInsert, sqlparser.StmtUpdate, sqlparser.StmtDelete:
					Logger.Info(ctx, "auditsql", map[string]any{
						"query": query,
						"args":  args,
					})
				}
				beginTime := time.Now()
				if begin := ctx.Value("beginTime"); begin != nil {
					beginTime = begin.(time.Time)
				}
				span := trace.SpanFromContext(ctx)
				if time.Now().Sub(beginTime).Seconds() > 1 {
					span.SetAttributes(
						attribute.Bool("slowsql", true),
					)
				}
				span.End()
				return ctx, nil
			},
			OnError: func(ctx context.Context, err error, query string, args ...interface{}) error {
				//1.获取span
				span := trace.SpanFromContext(ctx)
				if span != nil {
					if errors.Is(err, driver.ErrSkip) {
						span.SetAttributes(
							attribute.Bool("drop", true),
						)
					} else {
						span.SetAttributes(
							attribute.Bool("error", true),
						)
						span.RecordError(err, trace.WithStackTrace(true))
					}
					span.End()
				}
				return err
			},
		},
	}
}
