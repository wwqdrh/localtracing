package localtracing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

type localValue = *sync.Map // 每个协程存储值的位置

var (
	goroutineSpace = []byte("goroutine ")

	goroutineLocal = sync.Map{} // 协程id与上下文值的存储
	// localValue     = sync.Map{} // 每个协程存储值的位置
)

// GoroutineID 获取当前goroutine的id
func GoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		panic(fmt.Sprintf("No space found in %q", b))
	}
	b = b[:i]
	n, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse goroutine ID out of %q: %v", b, err))
	}
	return n
}

// 获取指定上一层级调用函数的名字
func FnName(bottom int) string {
	pc, _, _, ok := runtime.Caller(bottom)
	if !ok {
		panic("not found caller")
	}
	fn := runtime.FuncForPC(pc)
	name := fn.Name()
	return name
}

func HasContext() bool {
	id := GoroutineID()
	_, ok := goroutineLocal.Load(id)
	return ok
}

func ClearContext() {
	id := GoroutineID()
	goroutineLocal.Delete(id)
}

func GetContextValue(key string) interface{} {
	id := GoroutineID()
	val, ok := goroutineLocal.Load(id)
	if !ok {
		return nil
	}

	ctx, ok := val.(localValue)
	if !ok {
		return nil
	}

	if val, ok := ctx.Load(key); !ok {
		return nil
	} else {
		return val
	}
}

// 返回当前上下文中所有数据用字符串返回
func GetContextJson() string {
	id := GoroutineID()
	val, ok := goroutineLocal.Load(id)
	if !ok {
		return ""
	}

	ctx, ok := val.(localValue)
	if !ok {
		return ""
	}

	values := map[string]interface{}{}
	ctx.Range(func(key, value interface{}) bool {
		values[key.(string)] = value
		return true
	})
	bydata, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(bydata)
}

func SetContextValue(key string, value interface{}) error {
	id := GoroutineID()
	val, _ := goroutineLocal.LoadOrStore(id, &sync.Map{})

	ctx, ok := val.(localValue)
	if !ok {
		return errors.New("localvalue格式错误")
	}

	ctx.Store(key, value)
	return nil
}
