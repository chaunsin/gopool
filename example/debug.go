package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/chaunsin/gopool"
)

var (
	gp    *gopool.Pool
	i    int64
	done = make(chan struct{}, 1)
)

func do() {
	var (
		ctx = context.TODO()
		//gp=gopool.NewPool()//todo：这样声明的对象lock会空指针？？？
	)
	gp   = gopool.NewContextPool(ctx, gopool.Queue(1000), gopool.Worker(2))
	defer gp.Close()

	for {
		select {
		case <-done:
			log.Println("exit...............")
			return
		default:
			atomic.AddInt64(&i, 1)
			if err := gp.SyncAdd(ctx, func(ctx context.Context) {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				//fmt.Printf("[%d]\n", atomic.LoadInt64(&i))
			}); err != nil {
				log.Println("[sync]", atomic.LoadInt64(&i), err)
			}
		}
	}
}

func resize(w http.ResponseWriter, r *http.Request) {
	s := r.URL.Query().Get("num")
	if s == "" {
		fmt.Fprintln(w, "num params is nil")
		return
	}

	size, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintln(w, "strconv is err", err)
		return
	}

	fmt.Fprintln(w, "旧的worker数量:", gp.Resize(size))
}

func exit(w http.ResponseWriter, r *http.Request) {
	select {
	case done <- struct{}{}:
		go func() {
			time.Sleep(time.Second * 5)
			runtime.GC()
			debug.FreeOSMemory()
			gp = nil
		}()
		fmt.Fprintln(w, "done发送成功")
	default:
		fmt.Fprintln(w, "不能重复关闭")
	}
}

func monitor(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, gp.Debug())
	fmt.Fprintln(w, `\n`)
	fmt.Fprintln(w, gp.Monitor().String())
}

func create(w http.ResponseWriter, r *http.Request) {
	go do()
	fmt.Fprintln(w, "创建成功")
}

func main() {
	//gopool.Test2()
	go do()

	http.HandleFunc("/resize", resize)
	http.HandleFunc("/exit", exit)
	http.HandleFunc("/create", create)
	http.HandleFunc("/monitor", monitor)
	log.Fatal(http.ListenAndServe(":12345", nil))
}
