package localtracing

import (
	"container/heap"
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	DefaultStrage ApiTimeHeapBuilder = NewDiskHeap
	ApiMapping    sync.Map           = sync.Map{} // map[string]*ApiTimeParse
)

type (
	ApiTp struct {
		left  int
		right int
		cnt   int
	}

	ApiTimeParse struct {
		mu        sync.RWMutex
		chMu      chan bool
		freeQueue *EsQueue
		cCtx      context.Context
		cCancel   context.CancelFunc
		DoneSigle chan bool
		doTime    int64
		putTime   int64 // 用于统计是否执行完了
		minVal    int
		maxVal    int
		totalCnt  int
		totalTime int // mill 毫秒
		// bigQueue   queue // 小顶堆 存正数
		// smallQueue queue // 大顶堆 存负数
		bigQueue   ApiTimeHeapInterface
		smallQueue ApiTimeHeapInterface
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

	FreeLockFn func()
)

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

	chMu := make(chan bool, 1)
	chMu <- true // 首次启动
	ctx, cancel := context.WithCancel(context.TODO())

	return &ApiTimeParse{
		mu:         sync.RWMutex{},
		chMu:       chMu,
		freeQueue:  NewQueue(10000),
		cCtx:       ctx,
		cCancel:    cancel,
		DoneSigle:  make(chan bool, 1),
		minVal:     1<<31 - 1,
		maxVal:     -1 << 31,
		bigQueue:   big,
		smallQueue: small,
		tpBucket:   buckets,
	}, nil
}

func (p *ApiTimeParse) Clean() {
	p.freeQueue.cache = p.freeQueue.cache[:0]
	p.freeQueue = nil
	p.bigQueue = nil
	p.smallQueue = nil
	p.tpBucket = p.tpBucket[:0]
}

// StartDo 额外线程处理add函数
func (p *ApiTimeParse) StartFn() {
	defer func() {
		fmt.Println("该协程退出")
	}()

	for {
		select {
		case <-p.cCtx.Done():
			return
		default:
			if atomic.LoadInt64(&p.doTime) <= atomic.LoadInt64(&p.putTime) {
				if atomic.LoadInt64(&p.doTime) == atomic.LoadInt64(&p.putTime) {
					// 如果都是0则一直等待
					time.Sleep(100 * time.Millisecond) // 等待100毫秒如果还没有新的数据要处理
					if atomic.LoadInt64(&p.putTime) == 0 {
						continue
					} else if atomic.LoadInt64(&p.doTime) == atomic.LoadInt64(&p.putTime) {
						p.DoneSigle <- true
					}
				}
				val, ok, _ := p.freeQueue.Get()
				if !ok {
					fmt.Printf("Get.Fail\n")
					runtime.Gosched()
				} else {
					fn, ok := val.(FreeLockFn)
					if !ok {
						fmt.Println("类型不对")
					} else {
						fn()
					}
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (p *ApiTimeParse) AddFn(val int) {
	var c FreeLockFn = func() {
		p.Add(val)
	}

	ok, _ := p.freeQueue.Put(c)

	for !ok {
		time.Sleep(time.Microsecond)
		ok, _ = p.freeQueue.Put(c)
	}

	atomic.AddInt64(&p.putTime, 1)
}

// 添加新的数据
// 1、使用锁
// 2、使用通道代替锁
// 3、使用无锁队列: Apitime将add加入到无锁队列中，另起一个协程来获取无锁队列中的数据，有数据了就执行
func (p *ApiTimeParse) Add(val int) {
	defer func() {
		atomic.AddInt64(&p.doTime, 1)
	}()

	defer DefaultTimer.Time("Add")()

	// f := DefaultTimer.Time("Add通道锁")
	// <-p.chMu
	// defer func() { p.chMu <- true }()
	// f()

	// p.mu.Lock()
	// defer p.mu.Unlock()

	if val < p.minVal {
		p.minVal = val
	}
	if val > p.maxVal {
		p.maxVal = val
	}
	p.totalCnt++
	p.totalTime += val

	// tpbuckets计数
	t1 := DefaultTimer.Time("Add桶计数")
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
	t1()

	// 如果两边长度一样 调整结构 让左边多1
	t2 := DefaultTimer.Time("Add对顶堆重排序")
	defer t2()
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
	defer DefaultTimer.Time("MidVal")()

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
	defer DefaultTimer.Time("AvgVal")()

	p.mu.RLock()
	p.mu.RUnlock()

	return float64(p.totalTime) / float64(p.totalCnt)
}

// tp99值
func (p *ApiTimeParse) Tp99Val() float64 {
	defer DefaultTimer.Time("Tp99Val")()

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
// 如果
func ApiTime(fnName string, strage ApiTimeHeapBuilder) func() {
	if p, err := NewApiTimeParse(strage); err != nil {
		return func() {}
	} else {
		if _, loaded := ApiMapping.LoadOrStore(fnName, p); loaded {
			p.Clean()
		} else {
			go p.StartFn()
		}
	}

	start := time.Now()
	return func() {
		defer DefaultTimer.Time("ApiTime添加计数时的消耗")()
		durat := time.Since(start).Milliseconds()
		if val, ok := ApiMapping.Load(fnName); ok {
			val.(*ApiTimeParse).AddFn(int(durat))
		}
	}
}

func GetApiParse(fnName string) *ApiTimeParse {
	if v, ok := ApiMapping.Load(fnName); !ok {
		return nil
	} else {
		return v.(*ApiTimeParse)
	}
}

// 打印apiinfo信息
func ApiParseInfo(fnName string) {
	if v, ok := ApiMapping.Load(fnName); ok {
		val := v.(*ApiTimeParse)
		// info := val.GetMemory()

		fmt.Printf("[%s]当前占用总值: %dbyte", fnName, (val.smallQueue.GetSize()+val.bigQueue.GetSize())*8)
		fmt.Printf("[%s]执行情况: 最小值: %d, 最大值: %d, 中位数: %.2f, 平均数: %.2f, TP99: %.2f\n", fnName, val.minVal, val.maxVal, val.MidVal(), val.AvgVal(), val.Tp99Val())
	}
}
