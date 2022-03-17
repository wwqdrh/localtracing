package localtracing

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeTotal(t *testing.T) {
	var totalTime int64 = 0

	var a = func(w *sync.WaitGroup) {
		defer w.Done()
		defer DefaultTimer.Time("a")()

		c := rand.Intn(1000)
		fmt.Printf("执行了%d毫秒\n", c)
		atomic.AddInt64(&totalTime, int64(c))
		time.Sleep(time.Duration(c) * time.Millisecond)
	}

	wait := sync.WaitGroup{}
	wait.Add(100)
	for i := 0; i < 100; i++ {
		go a(&wait)
	}
	wait.Wait()
	DefaultTimer.AllInfo()
	fmt.Printf("总数: %d毫秒\n", totalTime)
}
