package gopool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ready   uint32
	add     uint32
	lose    uint32
	destroy uint32
	adjust  uint32
)

var (
	PoolFullErr  = errors.New("pool is full")
	PoolCloseErr = errors.New("pool is close")
)

type Task interface {
	Run() error
}

//It implements the runner interface.
type Job func() error

//function wrapper,implements the task interface.
func (f Job) Run() error {
	return f()
}

type Pool struct {
	queue  chan Task //任务可以积压池的大小
	worker []*worker //并发度 并行工作的数量
	//starve uint32    //当前饥饿的数量(ps:worker数量的设定值-当前正在运行的数量)

	ctx     context.Context //维护一个全局的ctx
	cancel  func()
	log     io.Writer //暴露日志接口
	skipErr bool      //是否忽略错误继续处理

	adjust chan int //调整worker 大小

	lock  sync.RWMutex  //保护worker
	once  sync.Once     //用于关闭
	close chan struct{} //关闭
}

func newWorker(p *Pool, num int) []*worker {
	work := make([]*worker, num)
	for i := 0; i < num; i++ {
		work[i] = &worker{
			p:        p,
			state:    0,
			terminal: make(chan struct{}, 1),
		}
	}
	return work
}

func NewContextPool(ctx context.Context, opts ...Option) *Pool {
	var (
		c      context.Context
		cancel func()
	)
	if ctx == nil {
		c, cancel = context.WithCancel(context.Background())
	}

	op := &options{
		worker: defaultWork,
		queue:  defaultQueue,
		skip:   false,
	}

	for _, o := range opts {
		o(op)
	}

	p := &Pool{
		queue:   make(chan Task, op.queue),
		ctx:     c,
		cancel:  cancel,
		log:     op.log,
		skipErr: op.skip,
		adjust:  make(chan int, 1),
		once:    sync.Once{},
		close:   make(chan struct{}, 1),
	}
	p.worker = newWorker(p, op.worker)

	go p.dispatch()
	//go p.monitor()
	return p
}

func (p *Pool) Info() (result map[string]interface{}) {
	result = make(map[string]interface{})
	p.lock.RLock()
	w := len(p.worker)
	c := cap(p.worker)
	p.lock.RUnlock()

	result["goroutine"] = runtime.NumGoroutine()
	result["len"] = w
	result["cap"] = c
	result["ready"] = atomic.LoadUint32(&ready)
	result["add"] = atomic.LoadUint32(&add)
	result["destroy"] = atomic.LoadUint32(&destroy)
	result["lose"] = atomic.LoadUint32(&lose)
	result["adjust"] = atomic.LoadUint32(&adjust)

	log.Printf("%+v\n", result)
	return result
}

func (p *Pool) monitor() {
	for {
		select {
		case <-call:
			return
		default:
			time.Sleep(time.Millisecond * 500)
			p.Info()
		}
	}
}

var call = make(chan struct{}, 1)

func (p *Pool) dispatch() {
	for _, v := range p.worker {
		v.run()
	}

	for {
		select {
		case size := <-p.adjust:
			atomic.AddUint32(&adjust, 1)
			//create worker
			if size > 0 {
				newWorker := newWorker(p, size)
				p.lock.Lock()
				p.worker = append(p.worker, newWorker...)
				p.lock.Unlock()
				for _, v := range newWorker {
					v.p = p
					v.run()
				}
			} else {
				/*
					减少worker数量
					考虑清理空闲的worker 采用链表的方式目前用数组代替
				*/
				p.lock.Lock()
				cut := len(p.worker) - (-size)
				terminal := p.worker[ cut:len(p.worker)]
				p.worker = p.worker[0:cut]
				p.lock.Unlock()
				for _, v := range terminal {
					v.close()
					//v = nil
				}
			}
		case <-p.close:
			fmt.Println("dispatch.close.............")
			for {
				select {
				case <-p.queue:
					atomic.AddUint32(&lose, 1)
				default:
					//p.queue = nil
					//close(p.queue)
					log.Println("clean queue is finish!!!!!!")
					return
				}
			}
		}
	}
}

//添加一个任务同步
func (p *Pool) SyncAdd(j Job) error {
	atomic.AddUint32(&ready, 1)

	select {
	case p.queue <- j:
		atomic.AddUint32(&add, 1)
		return nil
	case <-p.close:
		fmt.Println("SyncAdd关闭")
		return PoolCloseErr
	}
}

//添加一个任务
func (p *Pool) Add(j Job) error {
	select {
	case <-p.close:
		fmt.Println("Add关闭")
		return PoolCloseErr
	case p.queue <- j:
		return nil
	default:
		return PoolFullErr
	}
}

func abs(n int64) int64 {
	y := n >> 63
	return (n ^ y) - y
}

/*
	调整worker的大小 返回旧的worker数量
*/
func (p *Pool) Resize(size int) int {
	p.lock.RLock()
	ws := len(p.worker)
	p.lock.RUnlock()

	if size == 0 || size < 0 && abs(int64(size)) > int64(ws) {
		log.Printf("resize is err [%d] [%d]\n", size, ws)
		return -1
	}

	select {
	case p.adjust <- size:
		log.Println("send adjust notify...")
	case <-p.close:
		log.Println("resize err pool is close")
	}
	return ws
}

func (p *Pool) Close() {
	p.once.Do(func() {
		p.cancel()
		close(p.close)
		//close(p.queue)
		call <- struct{}{}
		p.queue = nil
		log.Println("EXIT!!!!!!!!!!!!!!!!!!!!")
		//go func() {
		//	close(p.close)
		//	p.cancel()
		//	for {
		//		select {
		//		case <-p.queue:
		//		default:
		//			//close(p.queue)
		//			log.Println("clean queue is finish!!!!!!")
		//			call <- struct{}{}
		//			return
		//		}
		//	}
		//}()
		//<-call
		////close(p.queue)
	})
}

/*
	1.日志接口
	2.全局context 和单个context
	3.统计当前队列活跃数量数值等
	4.动态修改池的大小 和worker数
	5.线程安全支持recover调用

	1.如果出错是否继续执行？
	2.如何保证每个worker均匀的执行任务

	1.options 顺序有问题
	2.worker调整
		1).思路使用链表 遍历所有
		2).使用map
	3.内存不释放问题
	4.close会丢弃池的任务 用户不清楚明确丢弃还是不明确丢弃

	1.通过-race检测
	2.报告分配内存大小10000万 100000万 1000000万情况
	3.无协程泄露
*/
