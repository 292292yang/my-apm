package infra

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"strconv"
	"time"
)

type GrpcServer struct {
	*grpc.Server
	addr string
}

func NewGrpcServer(addr string) *GrpcServer {
	svc := grpc.NewServer(grpc.UnaryInterceptor(unaryServerInterceptor()))
	server := &GrpcServer{
		Server: svc,
		addr:   addr,
	}
	globalStarters = append(globalStarters, server)
	globalClosers = append(globalClosers, server)
	return server
}

func (g *GrpcServer) Start() {
	l, err := net.Listen("tcp", g.addr)
	if err != nil {
		panic(err)
	}
	go func() {
		err = g.Serve(l)
		if err != nil {
			panic(err)
		}
	}()
}

func (g *GrpcServer) Close() {
	g.Server.GracefulStop()
}

const (
	grpcServerTracerName = "grpcServerTracer"
)

func unaryServerInterceptor() grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(grpcServerTracerName)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		//从context中获取metadata信息
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		clientApp := md.Get(peerApp)[0]
		clientHost := md.Get(peerHost)[0]
		//借助 传播者，将metadata中的信息提取到context中
		ctx = otel.GetTextMapPropagator().Extract(ctx, &metadataSupplier{metadata: md})
		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		start := time.Now()
		statusCode := codes.OK
		defer func() {
			span.SetAttributes(
				attribute.Float64("grpc.duration", time.Since(start).Seconds()),
			)
			span.End()
			// rpc server prometheus 埋点
			serverHandleHistogram.WithLabelValues(TypeGrpc, info.FullMethod, strconv.Itoa(int(statusCode)), clientApp, clientHost).Observe(time.Since(start).Seconds())
		}()
		serverHandleCounter.WithLabelValues(TypeGrpc, info.FullMethod, clientApp, clientHost).Inc()
		resp, err := handler(ctx, req)
		if err != nil {
			s, _ := status.FromError(err)
			statusCode = s.Code()
			span.RecordError(err, trace.WithTimestamp(time.Now()), trace.WithStackTrace(true))
			span.SetAttributes(
				attribute.Bool("error", true),
				attribute.String("grpc.status_code", s.Code().String()))
		}
		return resp, err
	}
}
