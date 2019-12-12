package gopool

import (
	"fmt"
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

//执行队列任务
func (w *worker) run() {
	go func() {
		for {
			select {
			//	单个context
			//case <-ctx.Done():
			//	fmt.Println("end plan")
			//	//if _, err := w.p.write.Write([]byte(ctx.Err().Error())); err != nil {
			//	//	panic(err)
			//	//}
			case <-w.terminal:
				fmt.Println("receive terminal destroy......")
				atomic.AddUint32(&destroy, 1)
				//销毁自身 不知道是否也会销毁pool的对应？？ 或者在发送通知的地方进行销毁
				//w=nil //调用自身为nil 貌似不行
				return
			case <-w.p.ctx.Done():
				fmt.Println("Worker中全局context", w.p.ctx.Err())
				if w.p.skipErr {
					if _, err := w.p.log.Write([]byte(w.p.ctx.Err().Error())); err != nil {
						panic(err)
					}
				}
				w.p.once.Do(w.p.Close)
				return
			case task, ok := <-w.p.queue:
				if !ok {
					log.Println("worker.queue is close")
					return
				}
				w.recover(task)
			}
		}
	}()
}

func (w *worker) recover(task Task) {
	defer func() {
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
				//任务不在继续执行杀掉所有正在执行的任务 错误是否返回？还是直接panic
				w.p.once.Do(w.p.Close)
			}
		}
	}()

	if err := task.Run(); err != nil {
		//忽略错误则把错误写入到日志中
		if w.p.skipErr {
			if _, err := w.p.log.Write([]byte(err.Error())); err != nil {
				panic(err)
			}
		} else {
			//任务不在继续执行杀掉所有正在执行的任务 错误是否返回？还是直接panic
			w.p.once.Do(w.p.Close)
		}
	}
}

func (w *worker) close() {
	close(w.terminal)
}
