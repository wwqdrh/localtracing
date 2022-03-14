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

func TestGinMiddleware(t *testing.T) {
	r := gin.Default()
	r.Use(TracingMiddleware())

	r.GET("/heath", func(ctx *gin.Context) {
		defer TracingTime("heath")()
		fmt.Println(goID())
		handleA()
		ctx.String(200, "hello")
	})

	wait := sync.WaitGroup{}
	wait.Add(10)
	for i := 0; i < 10; i++ {
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

func TestGinRegister(t *testing.T) {
	r := gin.Default()

	Register(r, "./temp")

	r.GET("/heath", func(ctx *gin.Context) {
		defer TracingTime("heath")()
		handleA()
		ctx.String(200, "hello")
	})

	wait := sync.WaitGroup{}
	wait.Add(10)
	for i := 0; i < 10; i++ {
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

func handleA() {
	defer TracingTime("handleA")()
	time.Sleep(300 * time.Millisecond)
	handleB()
}

func handleB() {
	time.Sleep(200 * time.Millisecond)
}
