package logger

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wwqdrh/localtracing/utils"
)

// 检查协程id是否会重复，通过则不会重复
func TestGetGoId(t *testing.T) {
	mapping := sync.Map{}
	var cnt int64 = 0

	wait := sync.WaitGroup{}
	wait.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wait.Done()
			id := utils.GoID()
			if _, ok := mapping.LoadOrStore(id, true); !ok {
				// 存在
				atomic.AddInt64(&cnt, 1)
			}

		}()
	}
	wait.Wait()
	if cnt != 1000 {
		t.Error("协程id存在重复")
	}
}
