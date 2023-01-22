package executor

import (
	"context"
	"github.com/zeromicro/go-zero/core/threading"
)

type Executor[P interface{}] struct {
	ctx     context.Context
	tasks   chan P
	handler func(task P)
	workers int
	cancel  context.CancelFunc
}

func NewExecutor[P interface{}](ctx context.Context, workers int, queueSize int, handler func(task P)) *Executor[P] {
	ret := &Executor[P]{
		tasks:   make(chan P, queueSize),
		handler: handler,
		workers: workers,
	}
	ret.ctx, ret.cancel = context.WithCancel(ctx)
	return ret
}

func (e *Executor[P]) Start() {
	for i := 0; i < e.workers; i++ {
		go func() {
			for {
				select {
				case <-e.ctx.Done():
					return
				case task := <-e.tasks:
					threading.RunSafe(func() {
						e.handler(task)
					})
				}
			}
		}()
	}
}

func (e *Executor[P]) Stop() {
	e.cancel()
}

func (e *Executor[P]) QueueSize() int {
	return len(e.tasks)
}

func (e *Executor[P]) Commit(task P) {
	e.tasks <- task
}
