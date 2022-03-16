package localtracing

import (
	"container/heap"
	"fmt"
	"sort"
	"sync"
	"time"
)

var ApiMapping sync.Map = sync.Map{} // map[string]*ApiTimeParse

type (
	queue struct {
		sort.IntSlice
	}

	ApiTimeParse struct {
		mu         sync.RWMutex
		minVal     int
		maxVal     int
		totalCnt   int
		totalTime  int   // mill 毫秒
		bigQueue   queue // 小顶堆 存正数
		smallQueue queue // 大顶堆 存负数
	}
)

func (h *queue) Top() interface{} {
	if len(h.IntSlice) == 0 {
		return nil
	}
	return h.IntSlice[0]
}
func (h *queue) Push(val interface{}) { h.IntSlice = append(h.IntSlice, val.(int)) }
func (h *queue) Pop() interface{} {
	res := h.IntSlice[len(h.IntSlice)-1]
	h.IntSlice = h.IntSlice[:len(h.IntSlice)-1]
	return res
}

func NewApiTimeParse() *ApiTimeParse {
	return &ApiTimeParse{
		mu:         sync.RWMutex{},
		minVal:     1<<31 - 1,
		maxVal:     -1 << 31,
		bigQueue:   queue{},
		smallQueue: queue{},
	}
}

// 添加新的数据
func (p *ApiTimeParse) Add(val int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val < p.minVal {
		p.minVal = val
	}
	if val > p.maxVal {
		p.maxVal = val
	}
	p.totalCnt++
	p.totalTime += val

	// 如果两边长度一样 调整结构 让左边多1
	if len(p.bigQueue.IntSlice) == len(p.smallQueue.IntSlice) {
		if len(p.bigQueue.IntSlice) == 0 {
			heap.Push(&p.smallQueue, -val)
			return
		}

		if val <= p.bigQueue.IntSlice[0] {
			heap.Push(&p.smallQueue, -val)
		} else {
			heap.Push(&p.smallQueue, -heap.Pop(&p.bigQueue).(int))
			heap.Push(&p.bigQueue, val)
		}
	} else if len(p.smallQueue.IntSlice) > len(p.bigQueue.IntSlice) {
		if val >= p.smallQueue.IntSlice[0] {
			heap.Push(&p.bigQueue, val)
		} else {
			heap.Push(&p.bigQueue, -heap.Pop(&p.smallQueue).(int))
			heap.Push(&p.smallQueue, -val)
		}
	}
}

// 求最大值
func (p *ApiTimeParse) MaxVal() int {
	p.mu.RLock()
	p.mu.RUnlock()

	return p.minVal
}

// 求最小值
func (p *ApiTimeParse) MinVal() int {
	p.mu.RLock()
	p.mu.RUnlock()

	return p.minVal
}

// 求中位数
func (p *ApiTimeParse) MidVal() float64 {
	p.mu.RLock()
	p.mu.RUnlock()

	if len(p.smallQueue.IntSlice) > len(p.bigQueue.IntSlice) {
		// 左边的只会比右边的大1，这一个就是中位数
		return float64(-p.smallQueue.Top().(int))
	}

	var a, b int
	if val := p.smallQueue.Top(); val != nil {
		a = -val.(int)
	}
	if val := p.bigQueue.Top(); val != nil {
		b = val.(int)
	}

	return float64(a+b) / 2
}

func (p *ApiTimeParse) AvgVal() float64 {
	p.mu.RLock()
	p.mu.RUnlock()

	return float64(p.totalTime) / float64(p.totalCnt)
}

// 求解api执行的情况
func ApiTime(fnName string) func() {
	ApiMapping.LoadOrStore(fnName, NewApiTimeParse())

	start := time.Now()
	return func() {
		durat := time.Since(start).Milliseconds()
		if val, ok := ApiMapping.Load(fnName); ok {
			val.(*ApiTimeParse).Add(int(durat))
		}
	}
}

// 打印apiinfo信息
func ApiParseInfo(fnName string) {
	if v, ok := ApiMapping.Load(fnName); ok {
		val := v.(*ApiTimeParse)
		fmt.Printf("[%s]执行情况: 最小值: %d, 最大值: %d, 中位数: %.2f, 平均数: %.2f\n", fnName, val.minVal, val.maxVal, val.MidVal(), val.AvgVal())
	}
}
