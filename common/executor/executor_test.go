package executor

import (
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"sync"
	"testing"
	"time"
)

func TestExecutor(t *testing.T) {
	wg := sync.WaitGroup{}
	executor := NewExecutor[int](context.Background(), 2, 100, func(task int) {
		logx.Infof("Running %d", task)
		time.Sleep(time.Second)
		wg.Done()
	})
	wg.Add(8)
	go executor.Start()
	for i := 0; i < 20; i++ {
		executor.Commit(i)
	}
	wg.Wait()
	logx.Infof("Stop")
	executor.Stop()
}
