package localtracing

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestTimeParse(t *testing.T) {
	p, err := NewApiTimeParse(NewDiskHeap)
	if err != nil {
		t.Error(err)
	}

	p.Add(2)
	if p.maxVal != 2 || p.minVal != 2 || p.MidVal() != 2 || p.AvgVal() != 2 {
		t.Error("add 2出错")
	}
	p.Add(3)
	if p.maxVal != 3 || p.minVal != 2 || p.MidVal() != 2.5 || p.AvgVal() != 2.5 {
		t.Error("add 3出错")
	}
	p.Add(4)
	if p.maxVal != 4 || p.minVal != 2 || p.MidVal() != 3 || p.AvgVal() != 3 {
		t.Error("add 4出错")
	}
}

// 假设一秒所有的接口调用共有10次，一天有864000
// 10w: [testapifn]当前内存状态: 总值: 6400128byte[testapifn]执行情况: 最小值: 0, 最大值: 2999, 中位数: 1498.00, 平均数: 1498.22
// 20w: [testapifn]当前内存状态: 总值: 12800128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1501.00, 平均数: 1500.75
// 30w: [testapifn]当前内存状态: 总值: 19200128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1502.00, 平均数: 1501.14
// 40w: [testapifn]当前内存状态: 总值: 25600128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1497.50, 平均数: 1499.83
// 50w: [testapifn]当前内存状态: 总值: 32000128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1502.00, 平均数: 1500.94
// 70w: [testapifn]当前内存状态: 总值: 44800128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1504.50, 平均数: 1502.40
// 86.4w: [testapifn]当前内存状态: 总值: 55296128byte[testapifn]执行情况: 最小值: 0, 最大值: 3000, 中位数: 1503.00, 平均数: 1502.32
func TestMemoryApiTime(t *testing.T) {
	var fn = func(w *sync.WaitGroup) {
		defer w.Done()
		defer ApiTime("testapifn", NewMemoryHeap)()

		time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
	}

	step := time.Now()
	wait := sync.WaitGroup{}
	wait.Add(10000)
	for i := 0; i < 10000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("1w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// return
	// 10w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(90000)
	for i := 0; i < 90000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("10w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 20w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("20w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 30w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("30w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 40w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("40w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 50w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("50w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 70w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(200000)
	for i := 0; i < 200000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("70w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")

	// 86.4w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(164000)
	for i := 0; i < 164000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("86.4w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
}

// 粗略统计耗时，主要占用在Len、swap上（将len进行缓存）
// 第二次测试发现不是，无论是对顶堆重排序还是统计数都没有占用多少，反而是锁的竞争导致的大量耗时
// 10w: 执行耗时:136.33[testapifn]当前占用总值: 23989776byte[testapifn]执行情况: 最小值: 0, 最大值: 3148, 中位数: 1514.00, 平均数: 1556.42, TP99: 2048.00
// Add对顶堆重排序耗时1028毫秒
// ApiTime2耗时133846毫秒
// GetSize耗时0毫秒
// Pop耗时664毫秒
// Swap耗时88毫秒
// ApiTime1耗时1156毫秒
// Add桶计数耗时398毫秒
// Top耗时7毫秒
// Less耗时937毫秒
// AvgVal耗时0毫秒
// Push耗时41毫秒
// getVal耗时683毫秒
// Len耗时72毫秒
// Add耗时133706毫秒
// Tp99Val耗时0毫秒
// MidVal耗时0毫秒
func TestDiskApiTime(t *testing.T) {
	var fn = func(w *sync.WaitGroup) {
		defer w.Done()
		defer ApiTime("testapifn", NewDiskHeap)()

		time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
	}

	step := time.Now()
	wait := sync.WaitGroup{}
	wait.Add(1000)
	for i := 0; i < 1000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	// 等待所有的消费者执行完成
	p := GetApiParse("testapifn")
	if p == nil {
		t.Error("获取apiparse错误")
	}
	<-p.DoneSigle
	fmt.Print("0.1w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 10w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(99000)
	for i := 0; i < 99000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	p = GetApiParse("testapifn")
	if p == nil {
		t.Error("获取apiparse错误")
	}
	<-p.DoneSigle
	fmt.Print("10w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()
	return
	// 20w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("20w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 30w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("30w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 40w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("40w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 50w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("50w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 70w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(200000)
	for i := 0; i < 200000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("70w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()

	// 86.4w
	step = time.Now()
	wait = sync.WaitGroup{}
	wait.Add(164000)
	for i := 0; i < 164000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("86.4w: ")
	fmt.Printf("执行耗时:%.2f", time.Since(step).Seconds())
	ApiParseInfo("testapifn")
	DefaultTimer.AllInfo()
}
