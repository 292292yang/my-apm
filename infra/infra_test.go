package infra

import (
	"context"
	"fmt"
	"net/http"
	"protos"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	Infra.Init("TestInit",
		InfraDbOption("root:123456@tcp(127.0.0.1:3306)/ordersvc"),
		InfraRdbOption("127.0.0.1:6379"),
	)
}

func TestNewHttpServer(t *testing.T) {
	s := NewHttpServer(":8999")
	s.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Ok\n"))
	})
	s.Start()
	time.Sleep(1 * time.Hour)
}

type helloSvc struct {
	protos.UnimplementedHelloServiceServer
}

func (h *helloSvc) Receive(ctx context.Context, msg *protos.HelloMsg) (*protos.HelloMsg, error) {
	return msg, nil
}

func TestGrpc(t *testing.T) {
	go func() {
		s := NewGrpcServer(":9990")
		protos.RegisterHelloServiceServer(s, &helloSvc{})
		s.Start()
	}()
	client := NewGrpcClient("127.0.0.1:9990", "hellosvc")
	res, err := protos.NewHelloServiceClient(client).Receive(context.TODO(), &protos.HelloMsg{Msg: "hello world"})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(res)
}
