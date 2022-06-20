package infra

import (
	"context"
	"dogapm/internal"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type log struct {
}

const traceId = "traceId"

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.AddHook(&logrusHook{})
}

var Logger = &log{}

func (*log) Debug(ctx context.Context, action string, kv map[string]any) {
	kv["action"] = action
	logrus.WithFields(kv).Debug()
}

func (*log) Info(ctx context.Context, action string, kv map[string]any) {
	kv["action"] = action
	if span := trace.SpanFromContext(ctx); span != nil {
		kv[traceId] = span.SpanContext().TraceID().String()
	}
	logrus.WithFields(kv).Info()
}

func (*log) Warn(ctx context.Context, action string, kv map[string]any) {
	kv["action"] = action
	if span := trace.SpanFromContext(ctx); span != nil {
		kv[traceId] = span.SpanContext().TraceID().String()
	}
	logrus.WithFields(kv).Warn()
}

func (*log) Error(ctx context.Context, action string, kv map[string]any, err error) {
	kv["action"] = action
	if span := trace.SpanFromContext(ctx); span != nil {
		kv[traceId] = span.SpanContext().TraceID().String()
		span.SetAttributes(attribute.Bool("error", true))
		span.RecordError(err, trace.WithAttributes(attribute.Bool("error", true)), trace.WithTimestamp(time.Now()))
	}
	logrus.WithFields(kv).WithError(err).Error()
}

type logrusHook struct{}

func (l *logrusHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (l *logrusHook) Fire(entry *logrus.Entry) error {
	entry.Data["host"] = internal.BuildInfo.Hostname()
	entry.Data["app"] = internal.BuildInfo.AppName()
	return nil
}
