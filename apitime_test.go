package localtracing

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestTimeParse(t *testing.T) {
	p := NewApiTimeParse()

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
func TestApiTime(t *testing.T) {
	var fn = func(w *sync.WaitGroup) {
		defer w.Done()
		defer ApiTime("testapifn")()

		time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
	}

	// 10w
	wait := sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("10w: ")
	ApiParseInfo("testapifn")

	// 20w
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("20w: ")
	ApiParseInfo("testapifn")

	// 30w
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("30w: ")
	ApiParseInfo("testapifn")

	// 40w
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("40w: ")
	ApiParseInfo("testapifn")

	// 50w
	wait = sync.WaitGroup{}
	wait.Add(100000)
	for i := 0; i < 100000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("50w: ")
	ApiParseInfo("testapifn")

	// 70w
	wait = sync.WaitGroup{}
	wait.Add(200000)
	for i := 0; i < 200000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("70w: ")
	ApiParseInfo("testapifn")

	// 86.4w
	wait = sync.WaitGroup{}
	wait.Add(164000)
	for i := 0; i < 164000; i++ {
		go fn(&wait)
	}
	wait.Wait()
	fmt.Print("86.4w: ")
	ApiParseInfo("testapifn")
}
