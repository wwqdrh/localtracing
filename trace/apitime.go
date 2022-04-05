package trace

import (
	"container/heap"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wwqdrh/localtracing/internal/cache"
	"github.com/wwqdrh/localtracing/internal/data"
	"github.com/wwqdrh/localtracing/logger"
	"github.com/wwqdrh/localtracing/utils"
	"go.uber.org/zap"
)

var (
	tracingContext                    = cache.NewLruCache(10000) // 协程id与tracingid的映射
	DefaultStrage  ApiTimeHeapBuilder = data.NewMemoryHeap
	ApiMapping     sync.Map           = sync.Map{} // map[string]*ApiTimeParse
)

type (
	ApiTp struct {
		left  int
		right int
		cnt   int64
	}

	ApiTimeParse struct {
		mu        sync.RWMutex
		minVal    int64
		maxVal    int64
		totalCnt  int64
		totalTime int64 // mill 毫秒
		// bigQueue   queue // 小顶堆 存正数
		// smallQueue queue // 大顶堆 存负数
		bigQueue   ApiTimeHeap
		smallQueue ApiTimeHeap
		tpBucket   []*ApiTp
	}

	ApiMemoInfo struct {
		minSize    int64
		maxSize    int64
		bigQSize   int64
		smallQSize int64
		Total      int64
		DiskTotaal int64
	}
)

func AddContext(tracingID string) {
	tracingContext.Add(utils.GoID(), tracingID)
}

func NewApiTimeParse(strages ...ApiTimeHeapBuilder) (*ApiTimeParse, error) {
	var starge ApiTimeHeapBuilder
	if len(strages) == 0 {
		starge = DefaultStrage
	} else {
		starge = strages[0]
	}

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

	big, err := starge("./temp/heap", "apitime", "bigQueue")
	if err != nil {
		return nil, err
	}

	small, err := starge("./temp/heap", "apitime", "smallQueue")
	if err != nil {
		return nil, err
	}

	return &ApiTimeParse{
		mu:         sync.RWMutex{},
		minVal:     1<<31 - 1,
		maxVal:     -1 << 31,
		bigQueue:   big,
		smallQueue: small,
		tpBucket:   buckets,
	}, nil
}

// 添加新的数据
// 不要加大锁
func (p *ApiTimeParse) Add(val int) {
	defer DefaultTimer.Time("Add")()

	// p.mu.Lock()
	// defer p.mu.Unlock()
	if c := atomic.LoadInt64(&p.minVal); int64(val) < c {
		atomic.CompareAndSwapInt64(&p.minVal, c, int64(val))
	}
	if c := atomic.LoadInt64(&p.maxVal); int64(val) > c {
		atomic.CompareAndSwapInt64(&p.maxVal, c, int64(val))
	}
	atomic.AddInt64(&p.totalCnt, 1)
	atomic.AddInt64(&p.totalTime, int64(val))

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
	atomic.AddInt64(&p.tpBucket[left].cnt, 1)

	// 如果两边长度一样 调整结构 让左边多1
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bigQueue.Len() == p.smallQueue.Len() {
		if p.bigQueue.Len() == 0 {
			heap.Push(p.smallQueue, -val)
			return
		}

		if val <= p.bigQueue.Top().(int) {
			heap.Push(p.smallQueue, -val)
		} else {
			heap.Push(p.smallQueue, -heap.Pop(p.bigQueue).(int))
			heap.Push(p.bigQueue, val)
		}
	} else if p.smallQueue.Len() > p.bigQueue.Len() {
		if val >= p.smallQueue.Top().(int) {
			heap.Push(p.bigQueue, val)
		} else {
			heap.Push(p.bigQueue, -heap.Pop(p.smallQueue).(int))
			heap.Push(p.smallQueue, -val)
		}
	}
}

// 求最大值
func (p *ApiTimeParse) MaxVal() int {
	p.mu.RLock()
	p.mu.RUnlock()

	return int(p.maxVal)
}

// 求最小值
func (p *ApiTimeParse) MinVal() int {
	p.mu.RLock()
	p.mu.RUnlock()

	return int(p.minVal)
}

// 求中位数
func (p *ApiTimeParse) MidVal() float64 {
	p.mu.RLock()
	p.mu.RUnlock()

	if p.smallQueue.Len() > p.bigQueue.Len() {
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

// 求解api执行的情况
func ApiTime(fnName string, strage ApiTimeHeapBuilder) func() {
	if p, err := NewApiTimeParse(strage); err != nil {
		return func() {}
	} else {
		ApiMapping.LoadOrStore(fnName, p)
	}

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
		// info := val.GetMemory()

		// fmt.Printf("[%s]当前占用总值: %dbyte", fnName, (val.smallQueue.GetSize()+val.bigQueue.GetSize())*8)
		fmt.Printf("[%s]执行情况: 最小值: %d, 最大值: %d, 中位数: %.2f, 平均数: %.2f, TP99: %.2f\n", fnName, val.minVal, val.maxVal, val.MidVal(), val.AvgVal(), val.Tp99Val())
	}
}

func TracingTime(funcName string) func() {
	tracingID := ""
	if val, ok := tracingContext.Get(utils.GoID()); !ok {
		return func() {}
	} else {
		tracingID = val.(string)
	}

	now := time.Now()
	logger.TraceLogger.Info("任务开始",
		zap.String("traceid", tracingID),
		zap.Int64("start_time", time.Now().UnixMicro()),
	)

	return func() {
		logger.TraceLogger.Info("任务结束",
			zap.String("traceid", tracingID),
			zap.Int64("end_time", time.Now().Unix()),
			zap.String("duration", time.Since(now).String()),
		)
	}
}
