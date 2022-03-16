package localtracing

import (
	"container/heap"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

var ApiMapping sync.Map = sync.Map{} // map[string]*ApiTimeParse

type (
	queue struct {
		sort.IntSlice
	}

	ApiTp struct {
		left  int
		right int
		cnt   int
	}

	ApiTimeParse struct {
		mu         sync.RWMutex
		minVal     int
		maxVal     int
		totalCnt   int
		totalTime  int   // mill 毫秒
		bigQueue   queue // 小顶堆 存正数
		smallQueue queue // 大顶堆 存负数
		tpBucket   []*ApiTp
	}

	ApiMemoInfo struct {
		minSize    int64
		maxSize    int64
		bigQSize   int64
		smallQSize int64
		Total      int64
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
	// 构造tpbucket
	buckets := []*ApiTp{}
	pre := 0
	for cur := 1; cur < 100000; cur = cur << 1 {
		buckets = append(buckets, &ApiTp{
			left:  pre,
			right: cur,
			cnt:   0,
		})
		pre = cur
	}

	return &ApiTimeParse{
		mu:         sync.RWMutex{},
		minVal:     1<<31 - 1,
		maxVal:     -1 << 31,
		bigQueue:   queue{},
		smallQueue: queue{},
		tpBucket:   buckets,
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

	// tpbuckets计数
	left, right := 0, len(p.tpBucket)-1
	flag := false
	for left < right {
		if flag {
			break
		}
		if left+1 == right {
			flag = true
		}
		mid := left + (right-left)/2
		if val > p.tpBucket[mid].right {
			left = mid + 1
		} else if val > p.tpBucket[mid].left {
			left = mid
		} else if val < p.tpBucket[mid].left {
			right = mid - 1
		} else if val < p.tpBucket[mid].right {
			right = mid
		}
	}
	p.tpBucket[left].cnt++

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

// tp99值
func (p *ApiTimeParse) Tp99Val() float64 {
	v := p.totalCnt * 99 / 100
	for i := 0; i < len(p.tpBucket); i++ {
		if v-p.tpBucket[i].cnt < 0 {
			return float64(p.tpBucket[i].left)
		}
		v -= p.tpBucket[i].cnt
	}
	return float64(p.tpBucket[len(p.tpBucket)-1].right)
}

func (p *ApiTimeParse) GetMemory() *ApiMemoInfo {
	minSizeStr := fmt.Sprint(unsafe.Sizeof(p.minVal))
	maxSizeStr := fmt.Sprint(unsafe.Sizeof(p.maxVal))
	bigQSizeStr := fmt.Sprint(uintptr(len(p.bigQueue.IntSlice)) * reflect.TypeOf(p.bigQueue.IntSlice).Elem().Size())
	smallQSizeStr := fmt.Sprint(uintptr(len(p.smallQueue.IntSlice)) * reflect.TypeOf(p.smallQueue.IntSlice).Elem().Size())

	minSize, _ := strconv.ParseInt(minSizeStr, 10, 64)
	maxSize, _ := strconv.ParseInt(maxSizeStr, 10, 64)
	bigQSize, _ := strconv.ParseInt(bigQSizeStr, 10, 64)
	smallQSize, _ := strconv.ParseInt(smallQSizeStr, 10, 64)

	return &ApiMemoInfo{
		minSize: minSize,
		maxSize: maxSize,
		// bigQSize:   fmt.Sprint(unsafe.Sizeof(p.bigQueue)),
		bigQSize: bigQSize,
		// smallQSize: fmt.Sprint(unsafe.Sizeof(p.smallQueue)),
		smallQSize: smallQSize,
		Total:      minSize + maxSize + bigQSize + smallQSize,
	}
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
		info := val.GetMemory()
		fmt.Printf("[%s]当前内存状态: 总值: %dbyte", fnName, info.Total*8)
		fmt.Printf("[%s]执行情况: 最小值: %d, 最大值: %d, 中位数: %.2f, 平均数: %.2f\n, TP99: %.2f", fnName, val.minVal, val.maxVal, val.MidVal(), val.AvgVal(), val.Tp99Val())
	}
}
