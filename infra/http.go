package infra

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
	"time"
)

const (
	httpTracerName = "dogapm/http"
)

type HttpServer struct {
	mux    *http.ServeMux
	server *http.Server
	tracer trace.Tracer
}

func NewHttpServer(addr string) *HttpServer {
	mux := http.NewServeMux()
	server := http.Server{Addr: addr, Handler: mux}
	s := &HttpServer{mux: mux, server: &server}
	s.tracer = otel.Tracer(httpTracerName)
	s.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	s.Handle("/metrics", promhttp.HandlerFor(MetricReg, promhttp.HandlerOpts{Registry: MetricReg}))
	globalStarters = append(globalStarters, s)
	globalClosers = append(globalClosers, s)
	return s
}

func (h *HttpServer) Handle(pattern string, handler http.Handler) {
	h.mux.Handle(pattern, &traceHandler{
		handler: handler,
		tracer:  h.tracer,
	})
}

func (h *HttpServer) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	h.mux.Handle(pattern, &traceHandler{
		handler: http.HandlerFunc(handler),
		tracer:  h.tracer,
	})
}

type traceHandler struct {
	handler http.Handler
	tracer  trace.Tracer
}

type respWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *respWriterWrapper) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (t traceHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if t.tracer == nil {
		t.handler.ServeHTTP(writer, request)
		return
	}
	ctx := request.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(request.Header))
	ctx, span := t.tracer.Start(ctx, request.URL.Path)
	defer span.End()
	request = request.Clone(ctx)
	start := time.Now()
	//prometheus metric统计
	serverHandleCounter.WithLabelValues(TypeHttp, request.Method+"."+request.URL.Path, "", "").Inc()
	respWrapper := &respWriterWrapper{ResponseWriter: writer}
	t.handler.ServeHTTP(respWrapper, request)
	if respWrapper.status == 0 {
		respWrapper.status = http.StatusOK
	}
	end := time.Now()
	//prometheus metric统计
	serverHandleHistogram.WithLabelValues(TypeHttp, request.Method+"."+request.URL.Path, strconv.Itoa(respWrapper.status), "", "").Observe(end.Sub(start).Seconds())
	span.SetAttributes(
		attribute.KeyValue{
			Key:   "http.status",
			Value: attribute.StringValue(strconv.Itoa(respWrapper.status)),
		},
		attribute.KeyValue{
			Key:   "http.duration",
			Value: attribute.IntValue(int(end.Sub(start).Seconds())),
		},
	)
}

func (h *HttpServer) Start() {
	go func() {
		err := h.server.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
}

func (h *HttpServer) Close() {
	h.server.Shutdown(context.Background())
}
