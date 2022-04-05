package localtracing

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/wwqdrh/localtracing/logger"
)

// const (
// 	// DebugLevel defines debug log level.
// 	DebugLevel Level = iota
// 	// InfoLevel defines info log level.
// 	InfoLevel
// 	// WarnLevel defines warn log level.
// 	WarnLevel
// 	// ErrorLevel defines error log level.
// 	ErrorLevel
// 	// FatalLevel defines fatal log level.
// 	FatalLevel
// 	// PanicLevel defines panic log level.
// 	PanicLevel
// 	// NoLevel defines an absent log level.
// 	NoLevel
// 	// Disabled disables the logger.
// 	Disabled

// 	// TraceLevel defines trace log level.
// 	TraceLevel Level = -1
// 	// Values less than TraceLevel are handled as numbers.
// )

func TestGinMiddleware(t *testing.T) {
	r := gin.Default()
	r.Use(TracingMiddleware("./log"))
	SetLogLevel(-1)
	r.GET("/heath", func(ctx *gin.Context) {
		ctx.Value("lg").(*logger.Handler).Debug().Msg("测试请求")
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
	time.Sleep(300 * time.Millisecond)
	handleB()
}

func handleB() {
	time.Sleep(200 * time.Millisecond)
}

func TestTimeTotal(t *testing.T) {
	var totalTime int64 = 0

	var a = func(w *sync.WaitGroup) {
		defer w.Done()
		// defer DefaultTime.Time("a")()

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
	// DefaultTime.AllInfo()
	fmt.Printf("总数: %d毫秒\n", totalTime)
}

func TestLg(t *testing.T) {
	h := logger.NewAsyncHandler("log", "zxlog", os.Stdout, 3, 2)

	go func() { h.Debug().Str("name", "jack").Msg("111") }()
	h.SetLevel(0, time.Second)
	go func() { h.Debug().Str("name", "jack").Msg("222") }()
	time.Sleep(time.Second * 2)
	go func() { h.Debug().Str("name", "jack").Msg("333") }()
	go func() { h.Error("err").Str("name", "jack").Msg("444") }()
	go func() { h.Error("err").Str("name", "jack").Err(errors.New("lll")).Msg("555") }()
	log.Println(logger.MessageRemaining())
	time.Sleep(5 * time.Second)
}

func TestError(t *testing.T) {
	e := errors.New("test")
	fmt.Println(e)
}

func TestRoute(t *testing.T) {
	h := logger.NewAsyncHandler("log", "zxlog", os.Stdout, 3, 2)
	http.HandleFunc("/", h.RouteWithLogTo("ddd", func(w http.ResponseWriter, r *http.Request, et *zerolog.Event) {
		panic("eee")
	}))
	go http.ListenAndServe(":8081", nil)
	time.Sleep(time.Second)

	http.Get("http://localhost:8081/sss")
	time.Sleep(time.Millisecond)
}
