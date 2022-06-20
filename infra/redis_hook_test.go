package infra

import (
	"context"
	"fmt"
	"testing"
)

func TestRedisHook(t *testing.T) {
	Infra.Init("TestRedisHook",
		InfraRdbOption("127.0.0.1:6379"),
		InfraEnableApm("127.0.0.1:54317", "TestRedisHook", "", 1),
	)
	defer EndPoint.Close()
	res, err := Infra.Rdb.Get(context.TODO(), "haha").Result()
	fmt.Println(res, err)
}
