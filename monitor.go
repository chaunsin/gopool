package gopool

import (
	"fmt"
	"log"
	"runtime"
	"sync/atomic"
	"time"
)

const text = `{"starve":%d,"active":%d,worker":%d,"pool":%d,"err":%s}`

type Monitor interface {
	Starve() int //饥饿的worker数
	Active() int //worker活跃数量
	Worker() int //当前worker的总数量
	Pool() int   //池的大小
	String() string
	Err() error
}

type monitor struct {
	starve int
	active int
	worker int
	pool   int
	err    error
}

func (m *monitor) Starve() int {
	return m.starve
}

func (m *monitor) Active() int {
	return m.active
}

func (m *monitor) Worker() int {
	return m.worker
}

func (m *monitor) Pool() int {
	return m.pool
}

func (m *monitor) Err() error {
	return m.err
}

func (m *monitor) String() string {
	return fmt.Sprintf(text, m.starve, m.worker, m.active, m.pool, m.err)
}

//---------------------------------------------------
var (
	ready   uint32
	add     uint32
	lose    uint32
	destroy uint32
	adjust  uint32
)

func (p *Pool) monitor() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			time.Sleep(time.Millisecond * 500)
			p.Debug()
		}
	}
}

func (p *Pool) Debug() (result map[string]interface{}) {
	defer func() {
		if x:=recover();x!=nil{
			fmt.Println("ERR:",x)
		}
	}()
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
