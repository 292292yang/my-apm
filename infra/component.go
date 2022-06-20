package infra

import (
	"os"
	"os/signal"
	"syscall"
)

type starter interface {
	Start()
}

type closer interface {
	Close()
}

var (
	globalStarters = make([]starter, 0)
	globalClosers  = make([]closer, 0)
)

type endPoint struct {
	stop chan int
}

var EndPoint = &endPoint{stop: make(chan int)}

func (e *endPoint) Start() {
	for _, comp := range globalStarters {
		comp.Start()
	}
	go func() {
		//监听服务的结束
		quit := make(chan os.Signal)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		<-quit
		e.ShutDown()
	}()
	<-e.stop
}

func (e *endPoint) ShutDown() {
	for _, comp := range globalClosers {
		comp.Close()
	}
	e.stop <- 1
}

func (e *endPoint) Close() {
	for _, comp := range globalClosers {
		comp.Close()
	}
}
