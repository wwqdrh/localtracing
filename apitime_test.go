package localtracing

import (
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

func TestApiTime(t *testing.T) {
	var fn = func(w *sync.WaitGroup) {
		defer w.Done()
		defer ApiTime("testapifn")()

		time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
	}

	wait := sync.WaitGroup{}
	wait.Add(100)
	for i := 0; i < 100; i++ {
		go fn(&wait)
	}
	wait.Wait()
	ApiParseInfo("testapifn")
}
