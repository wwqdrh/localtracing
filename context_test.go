package localtracing

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

var mainID = GoroutineID()

func TestGroutineID(t *testing.T) {
	if mainID != 1 {
		t.Error("获取线程id失败")
	}
}

func TestGroutineContext(t *testing.T) {
	// 测试在a协程写数据 b协程是否能读取到
	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		defer wait.Done()
		SetContextValue("key", "value")
		val := GetContextValue("key")
		if val == nil || val.(string) != "value" {
			t.Error("读取上下文失败")
		}
	}()
	wait.Wait()

	wait.Add(1)
	go func() {
		defer wait.Done()
		if val := GetContextValue("key"); val != nil {
			t.Error("被其他上下文读取到了")
		}
	}()
	wait.Wait()
}

func TestHasContext(t *testing.T) {
	if HasContext() {
		t.Error("刚开始有context")
	}

	SetContextValue("key", "value")

	if !HasContext() {
		t.Error("没有context")
	}
	ClearContext()
	if HasContext() {
		t.Error("context清理失败")
	}
}

func TestFnName(t *testing.T) {
	fn := func() func() {
		start := time.Now()
		fmt.Println(FnName(2))
		return func() {
			fmt.Println(time.Since(start).Seconds())
		}
	}
	a := func() {
		defer fn()()
		fmt.Println(FnName(1))
		time.Sleep(1 * time.Second)
	}
	a()
}
