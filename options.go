package gopool

import (
	"io"
	"runtime"
)

var (
	defaultWork  = runtime.NumCPU()
	defaultQueue = 128
)

type options struct {
	worker int
	queue  int
	skip   bool
	log    io.Writer
}

type Option func(*options)

func Worker(n int) Option {
	if n <= 0 {
		panic("gopool: worker should > 0")
	}
	return func(o *options) {
		o.worker = n
	}
}

func Queue(n int) Option {
	if n <= 0 {
		panic("gopool: Queue should > 0")
	}
	return func(o *options) {
		o.worker = n
	}
}

func Skip(b bool) Option {
	return func(o *options) {
		o.skip = b
	}
}

func Log(w io.Writer) Option {
	if w == nil {
		panic("gopool: log is nil")
	}
	return func(o *options) {
		o.log = w
	}
}
