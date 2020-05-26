package gopool

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

var (
	PoolFullErr  = errors.New("pool is full")
	PoolCloseErr = errors.New("pool is close")
)

type Task interface {
	Run(ctx context.Context)
}

//It implements the runner interface.
type Job func(ctx context.Context)

//function wrapper,implements the task interface.
func (f Job) Run(ctx context.Context) {
	f(ctx)
}

//type jobUnit struct {
//	ctx context.Context
//	job Job
//}

type Pool struct {
	queue  chan func()
	worker []*worker

	ctx     context.Context
	cancel  func()
	log     io.Writer
	skipErr bool

	adjust chan int
	active int32

	lock sync.RWMutex
	once sync.Once
	done uint32
}

func NewPool() *Pool {
	return NewContextPool(context.Background())
}

func NewContextPool(ctx context.Context, opts ...Option) *Pool {
	var (
		cancel func()
		c      context.Context
	)
	if ctx == nil {
		c, cancel = context.WithCancel(context.Background())
	} else {
		c, cancel = context.WithCancel(ctx)
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
		queue:   make(chan func(), op.queue),
		ctx:     c,
		cancel:  cancel,
		log:     op.log,
		skipErr: op.skip,
		adjust:  make(chan int, 1),
	}
	p.worker = newWorker(p, op.worker)

	go p.dispatch()
	//go p.monitor()
	return p
}

func (p *Pool) dispatch() {
	for _, v := range p.worker {
		v.process()
	}

	for {
		select {
		case <-p.ctx.Done():
			p.Close()
			return
		case size := <-p.adjust:
			if size > 0 {
				newWorker := newWorker(p, size)
				p.lock.Lock()
				p.worker = append(p.worker, newWorker...)
				p.lock.Unlock()
				for _, v := range newWorker {
					v.process()
				}
			} else if size < 0 {
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
				terminal = terminal[:0]
			}
			//	else {
			//	//清理退出
			//	for range p.queue {
			//		fmt.Println("cccccccccccccccccccc")
			//	}
			//	fmt.Println("llllllllllllllllllll")
			//	return
			//}
		}
	}

	//for size := range p.adjust {
	//	if size > 0 {
	//		newWorker := newWorker(p, size)
	//		p.lock.Lock()
	//		p.worker = append(p.worker, newWorker...)
	//		p.lock.Unlock()
	//		for _, v := range newWorker {
	//			v.process()
	//		}
	//	} else if size < 0 {
	//		/*
	//			减少worker数量
	//			考虑清理空闲的worker 采用链表的方式目前用数组代替
	//		*/
	//		p.lock.Lock()
	//		cut := len(p.worker) - (-size)
	//		terminal := p.worker[ cut:len(p.worker)]
	//		p.worker = p.worker[0:cut]
	//		p.lock.Unlock()
	//		for _, v := range terminal {
	//			v.close()
	//			//v = nil
	//		}
	//		terminal = terminal[:0]
	//	} else {
	//		//清理退出
	//		for range p.queue {
	//			fmt.Println("cccccccccccccccccccc")
	//		}
	//		fmt.Println("llllllllllllllllllll")
	//		return
	//	}
	//}
}

//添加一个任务同步
func (p *Pool) SyncAdd(ctx context.Context, j func(ctx2 context.Context)) error {
	//atomic.AddUint32(&ready, 1)
	if atomic.LoadUint32(&p.done) == 1 {
		return PoolCloseErr
	}

	//p.queue <- func() { j.Run(ctx) }
	p.queue <- func() { j(ctx) }
	atomic.AddUint32(&add, 1)
	return nil
}

//添加一个任务
func (p *Pool) Add(ctx context.Context, j Job) error {
	if atomic.LoadUint32(&p.done) == 1 {
		return PoolCloseErr
	}

	select {
	case p.queue <- func() { j.Run(ctx) } /*jobUnit{ctx: ctx, job: j}*/ :
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
	if atomic.LoadUint32(&p.done) == 1 {
		return -1
	}

	p.lock.RLock()
	ws := len(p.worker)
	p.lock.RUnlock()

	if size == 0 || size < 0 && abs(int64(size)) >= int64(ws) {
		log.Printf("resize is err [%d] [%d]\n", size, ws)
		return -1
	}
	p.adjust <- size
	return ws
}

func (p *Pool) Monitor() Monitor {
	p.lock.RLock()
	w := len(p.worker)
	p.lock.RUnlock()

	var (
		a = int(atomic.LoadInt32(&p.active))
		m = monitor{
			starve: w - a,
			active: a,
			worker: w,
			pool:   cap(p.queue),
			err:    nil,
		}
	)
	return &m
}

func (p *Pool) Close() {
	p.once.Do(func() {
		atomic.AddUint32(&p.done, 1)
		p.cancel()
		//p.adjust <- 0
		close(p.queue)
	})
}

/*
	1.日志接口?
	2.全局context 和单个context
	3.统计当前队列活跃数量数值等
	4.动态修改池的大小 和worker数
	5.线程安全支持recover调用
	6.支持任务执行完关闭还是未完成直接关闭

	1.如果出错是否继续执行？
	2.如何保证每个worker均匀的执行任务

	1.worker调整
		1).思路使用链表 遍历所有
		2).使用map
	2.内存不释放问题
	3.close会丢弃池的任务 用户不清楚明确丢弃还是不明确丢弃

	1.通过-race检测
	2.报告分配内存大小10000万 100000万 1000000万情况
	3.无协程泄露
	4.测试单核状态下 以及runtime.GOMAXPROCS(1)的情况
*/

//var cache=sync.Pool{
//	New: func() interface{} {
//		return jobUnit{
//			ctx: nil,
//			job: nil,
//		}
//	},
//}
