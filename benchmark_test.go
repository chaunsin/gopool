package gopool_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chaunsin/gopool"
)

const (
	TaskNum    = 1000000
	WorkerSize = 200000
)

func interval() {
	time.Sleep(time.Duration(10) * time.Millisecond)
}

func init() {
	//runtime.GOMAXPROCS(1)
}

func BenchmarkGoroutines(b *testing.B) {
	b.ReportAllocs()
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(TaskNum)
		for j := 0; j < TaskNum; j++ {
			go func() {
				interval()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkGoPool(b *testing.B) {
	b.ReportAllocs()

	var (
		wg  sync.WaitGroup
		ctx = context.Background()
		p   = gopool.NewContextPool(ctx, gopool.Worker(WorkerSize), gopool.Queue(TaskNum))
	)
	defer p.Close()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(TaskNum)
		for j := 0; j < TaskNum; j++ {
			_ = p.SyncAdd(ctx, func(ctx2 context.Context) {
				interval()
				wg.Done()
			})
		}
		wg.Wait()
	}
	b.StopTimer()
}

func BenchmarkGoPoolThroughput(b *testing.B) {
	b.ReportAllocs()

	var (
		ctx = context.TODO()
		p   = gopool.NewContextPool(ctx, gopool.Worker(WorkerSize))
	)
	defer p.Close()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < TaskNum; j++ {
			p.SyncAdd(ctx, func(ctx context.Context) {
				interval()
			})
		}
	}
	b.StopTimer()
}
