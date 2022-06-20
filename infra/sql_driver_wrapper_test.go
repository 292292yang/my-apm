package infra

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
)

func TestMySqlWrapper(t *testing.T) {
	beforeAtomic := atomic.Bool{}
	afterAtomic := atomic.Bool{}
	errorAtomic := atomic.Bool{}
	testDriver := &Driver{
		Driver: mysql.MySQLDriver{},
		hooks: Hooks{
			Before: func(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
				beforeAtomic.Store(true)
				fmt.Println("before....")
				return ctx, nil
			},
			After: func(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
				afterAtomic.Store(true)
				fmt.Println("after....")
				return ctx, nil
			},
			OnError: func(ctx context.Context, err error, query string, args ...interface{}) error {
				errorAtomic.Store(true)
				fmt.Println("error....")
				return err
			},
		},
	}
	sql.Register("test-mysql", testDriver)

	db, err := sql.Open("test-mysql", "root:123456@tcp(127.0.0.1:3306)/ordersvc")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("select 1")
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, beforeAtomic.Load())
	assert.True(t, afterAtomic.Load())
	assert.False(t, errorAtomic.Load())

	_, err = db.Exec("select 1 from non_existent_table")
	assert.Error(t, err)
	assert.True(t, errorAtomic.Load())
}
