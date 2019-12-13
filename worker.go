package gopool

import (
	"log"
	"runtime"
	"sync/atomic"
)

/*
	如何动态调整worker的大小
	1.创建用户指定的worker大小然后copy 通过信号通知来
	2.切片数组方式来维护worker
	3.用堆 二叉树等进行实现
*/

const dumpSize = 64 << 10


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

type worker struct {
	p        *Pool
	terminal chan struct{} //销毁用于动态扩容
	/*
		使用场景:
		1.可以使用在当调整worker的数量时使用遍历到这个对象的时候
		判断这个状态值如果是空闲的则优先剔除掉这个worker
		2.可能可以统计当前worker的数量关系
	*/
	state uint8 //标记当前这个worker的状态
}

//var empty jobUnit

//执行队列任务
func (w *worker) process() {
	go func() {
		defer func() {
			atomic.AddInt32(&w.p.active, -1)
			if x := recover(); x != nil {
				buf := make([]byte, dumpSize)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("pool:the task in the pool failed: %v\n%s", x, buf)
				//忽略错误则把错误写入到日志中
				if w.p.skipErr {
					if _, err := w.p.log.Write(buf); err != nil {
						panic(err)
					}
				} else {
					//当大量goroutine都发生了recover后很多占用这个方法
					//任务不在继续执行杀掉所有正在执行的任务 错误是否返回？还是直接panic
					w.p.once.Do(w.p.Close)
				}
			}
		}()

		for task := range w.p.queue {
			//if task == nil {
			//	return
			//}

			atomic.AddInt32(&w.p.active, 1)
			//task.job(task.ctx)

			task()
			//cache.Put(task)
			atomic.AddInt32(&w.p.active, -1)
			//w.recover(task.job)(task.ctx)

			if len(w.terminal)>0{
				return
			}
		}
	}()
}

//func (w *worker) recover(j Job) func(ctx context.Context) {
//	return func(ctx context.Context) {
//		atomic.AddInt32(&w.p.active, 1)
//		defer func() {
//			atomic.AddInt32(&w.p.active, -1)
//			if x := recover(); x != nil {
//				buf := make([]byte, dumpSize)
//				buf = buf[:runtime.Stack(buf, false)]
//				log.Printf("pool:the task in the pool failed: %v\n%s", x, buf)
//				//忽略错误则把错误写入到日志中
//				if w.p.skipErr {
//					if _, err := w.p.log.Write(buf); err != nil {
//						panic(err)
//					}
//				} else {
//					//当大量goroutine都发生了recover后很多占用这个方法
//					//任务不在继续执行杀掉所有正在执行的任务 错误是否返回？还是直接panic
//					w.p.once.Do(w.p.Close)
//				}
//			}
//		}()
//		j(ctx)
//
//		//if err := task.Run(c); err != nil {
//		//	//忽略错误则把错误写入到日志中
//		//	if w.p.skipErr {
//		//		if _, err := w.p.log.Write([]byte(err.Error())); err != nil {
//		//			panic(err)
//		//		}
//		//	} else {
//		//		//任务不在继续执行杀掉所有正在执行的任务 错误是否返回？还是直接panic
//		//		w.p.once.Do(w.p.Close)
//		//	}
//		//}
//	}
//}

func (w *worker) close() {
	close(w.terminal)
}
