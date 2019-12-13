package gopool_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/chaunsin/gopool"
)

func sleep() {
	time.Sleep(time.Millisecond * 500)
}

func TestNewPool(t *testing.T) {
	var (
		ctx = context.TODO()
		p   = gopool.NewPool()
	)
	defer p.Close()

	p.SyncAdd(ctx, func(ctx2 context.Context) {
		fmt.Println("hello SyncAdd!")
	})

	p.Add(ctx, func(ctx context.Context) {
		fmt.Println("hello Add!")
	})

	sleep()
}

func TestNewContextPool(t *testing.T) {
	var (
		ctx = context.TODO()
		p   = gopool.NewContextPool(ctx, gopool.Worker(2), gopool.Queue(2))
	)
	defer p.Close()

	p.SyncAdd(ctx, func(ctx2 context.Context) {
		fmt.Println("hello SyncAdd!")
	})

	p.Add(ctx, func(ctx context.Context) {
		fmt.Println("hello Add!")
	})

	sleep()
}

func TestPool_Add(t *testing.T) {
	var (
		ctx  = context.TODO()
		p    = gopool.NewContextPool(ctx, gopool.Worker(1), gopool.Queue(1))
		task = func(ctx context.Context) {
			fmt.Println("hello Add!")
		}
		closeTask = func(ctx context.Context) {
			fmt.Println("close")
		}
	)

	t.Run("add", func(t *testing.T) {
		if err := p.Add(ctx, task); err != nil {
			if err == gopool.PoolCloseErr {
				t.Fatalf("want [nil] real [%s]\n", gopool.PoolCloseErr)
			}
		}
	})

	t.Run("full", func(t *testing.T) {
		p.Add(ctx, func(ctx context.Context) {
			sleep()
			sleep()
		})

		if err := p.Add(ctx, task); err != nil {
			if err != gopool.PoolFullErr {
				t.Fatalf("want [%s] real [%s]\n", gopool.PoolFullErr, err)
			}
		}
	})

	t.Run("close", func(t *testing.T) {
		p.Close()
		if err := p.Add(ctx, closeTask); err != nil {
			if err != gopool.PoolCloseErr {
				t.Fatalf("want [%s] real [%s]\n", gopool.PoolCloseErr, err)
			}
		}
	})
	sleep()
}

func TestPool_SyncAdd(t *testing.T) {
	var (
		ctx  = context.TODO()
		p    = gopool.NewContextPool(ctx, gopool.Worker(2), gopool.Queue(2))
		task = func(ctx context.Context) {
			fmt.Println("hello SyncAdd!")
		}
		closeTask = func(ctx context.Context) {
			fmt.Println("close")
		}
	)

	t.Run("syncAdd", func(t *testing.T) {
		if err := p.SyncAdd(ctx, task); err != nil {
			t.Fatalf("want [nil] real [%s]\n", err)
		}
	})

	t.Run("close", func(t *testing.T) {
		p.Close()
		if err := p.SyncAdd(ctx, closeTask); err != nil {
			if err != gopool.PoolCloseErr {
				t.Fatalf("want [%s] real [%s]\n", gopool.PoolCloseErr, err)
			}
		}
	})
	sleep()
}

func TestPool_Resize(t *testing.T) {
	var (
		ctx = context.TODO()
		p   = gopool.NewContextPool(ctx, gopool.Worker(2), gopool.Queue(2))
	)

	tables := []struct {
		size int
		want int
		no   int
	}{
		{size: 2, want: 4, no: 0},
		{size: 2, want: 6, no: 1},
		{size: -2, want: 4, no: 2},
		{size: -3, want: 1, no: 3},
		{size: -1, want: 1, no: 4},
		{size: -math.MinInt32, want: 1, no: 5},
	}

	for _, v := range tables {
		p.Resize(v.size)
		sleep()
		if real := p.Monitor().Worker(); real != v.want {
			t.Fatalf("no.%d want [%d] real [%d]",v.no, v.want, real, )
		}
	}
}
