package infra

import (
	"context"
	"testing"
	"time"
)

func TestSqlHook(t *testing.T) {
	Infra.Init("TestSqlHook",
		InfraEnableApm("localhost:54317", "TestSqlHook", "", 1),
		InfraDbOption("root:123456@tcp(127.0.0.1:3306)/ordersvc"),
	)
	defer EndPoint.Close()
	ctx, span := Tracer.Start(context.Background(), "test1111")
	defer span.End()
	Infra.Db.ExecContext(ctx, "select *, sleep(2) from t_order limit ?;", 2)
}

func TestLongTx(t *testing.T) {
	Infra.Init("TestLongTx",
		InfraEnableApm("localhost:54317", "TestLongTx", "", 1),
		InfraDbOption("root:123456@tcp(127.0.0.1:3306)/ordersvc"),
	)
	defer EndPoint.Close()
	ctx, span := Tracer.Start(context.Background(), "test-longtx")
	defer span.End()
	tx, err := Infra.Db.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}
	_, _ = tx.QueryContext(ctx, "select * from t_order limit ?;", 2)
	time.Sleep(5 * time.Second)
	tx.Commit()
}
