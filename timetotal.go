package localtracing

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 计算函数调用情况总耗时

var DefaultTimer = NewTotalTimer()

type (
	TotalTimer struct {
		fnMap sync.Map // string: {mutex, val}
	}

	fnItem struct {
		mu  sync.Mutex
		val int64
	}
)

func NewTotalTimer() *TotalTimer {
	return &TotalTimer{fnMap: sync.Map{}}
}

func (t *TotalTimer) getName(fnName string) string {
	return fmt.Sprintf("%d-%s", goID(), fnName)
}

// TODO 存在并发情况，不能单纯的将所有累加
// 需要根据协程id判断
func (t *TotalTimer) Time(fnName string) func() {
	fnName = t.getName(fnName)

	now := time.Now()
	v, _ := t.fnMap.LoadOrStore(fnName, &fnItem{mu: sync.Mutex{}, val: 0})
	curV := v.(*fnItem)

	return func() {
		durat := time.Since(now).Milliseconds()
		curV.mu.Lock()
		defer curV.mu.Unlock()
		curV.val += durat
	}
}

// 同样名字但是协程id不同需要去重
func (t *TotalTimer) AllInfo() {
	visit := map[string]int64{}

	t.fnMap.Range(func(key, value interface{}) bool {
		fnName := strings.Split(key.(string), "-")[1]
		if val, ok := visit[fnName]; !ok {
			// fmt.Printf("%s耗时%d毫秒\n", fnName, value.(*fnItem).val)
			visit[fnName] = value.(*fnItem).val
		} else if value.(*fnItem).val > val {
			visit[fnName] = value.(*fnItem).val
		}
		return true
	})

	for key, val := range visit {
		fmt.Printf("%s耗时%d毫秒\n", key, val)
	}
}
