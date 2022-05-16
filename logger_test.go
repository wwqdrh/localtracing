package localtracing

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLocalTracingMiddleware(t *testing.T) {
	r := gin.Default()
	handler, err := NewLocaltracing("./log")
	if err != nil {
		t.Fatal(err)
	}

	handleB := func() {
		defer handler.Time()()
		time.Sleep(200 * time.Millisecond)
	}

	handleA := func() {
		defer handler.Time()()
		time.Sleep(300 * time.Millisecond)
		handleB()
		// panic("测试")
	}

	r.Use(handler.HandlerFunc())
	r.GET("/heath", func(ctx *gin.Context) {
		handleA()
		ctx.String(200, "hello")
	})

	wait := sync.WaitGroup{}
	wait.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wait.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/heath", nil)
			r.ServeHTTP(w, req)
			fmt.Println(w.Body.String())
		}()
	}
	wait.Wait()
}
