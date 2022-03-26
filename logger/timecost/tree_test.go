package timecost

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestTree(t *testing.T) {
	names := []string{
		"test",
		"test1",
		"test2",
	}

	wg := &sync.WaitGroup{}

	rand.Seed(time.Now().Unix())
	for i := range names {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			n := rand.Intn(20) + 45
			for i := 0; i < n; i++ {
				AggCost(name, time.Duration(rand.Intn(1000))*time.Millisecond+time.Duration(rand.Intn(100))*time.Second)
			}
		}(names[i])
	}

	wg.Wait()

	AggCost("name", 55*time.Second+time.Millisecond*5)
	names = append(names, "name")

	for i := range names {
		r := GetCost(names[i])
		fmt.Print(names[i], ":")
		r.lprWalk(func(p *node) bool {
			fmt.Print("{", p.level, ",", p.avgCost, "} ")
			return true
		})
		bs, _ := r.MarshalJSON()
		fmt.Println(string(bs))
	}
}
