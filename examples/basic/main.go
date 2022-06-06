package main

import (
	"errors"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wwqdrh/localtracing"
)

type GinHanlderAdapter struct {
	engine *gin.Engine
}

func (g GinHanlderAdapter) Context(val interface{}) (*http.Request, http.ResponseWriter, error) {
	ctx, err := val.(*gin.Context)
	if !err {
		return nil, nil, errors.New("类型转换失败")
	}

	return ctx.Request, ctx.Writer, nil
}

func (g GinHanlderAdapter) Static(url string, filepath http.FileSystem) {
	f := http.FileServer(filepath)
	url = path.Join(url, "/*filepath")
	g.engine.HEAD(url, func(ctx *gin.Context) {
		ctx.Request.URL.Path = strings.TrimPrefix(ctx.Request.URL.Path, url)
		f.ServeHTTP(ctx.Writer, ctx.Request)
	})
	g.engine.GET(url, func(ctx *gin.Context) {
		ctx.Request.URL.Path = strings.TrimPrefix(ctx.Request.URL.Path, url)
		f.ServeHTTP(ctx.Writer, ctx.Request)
	})
}

func (g GinHanlderAdapter) Get(url string, fn func(interface{})) {
	g.engine.GET(url, func(ctx *gin.Context) {
		fn(ctx)
	})
}

func (g GinHanlderAdapter) Post(url string, fn func(interface{})) {
	g.engine.POST(url, func(ctx *gin.Context) {
		fn(ctx)
	})
}

var (
	handler *localtracing.LocalTracing
)

func main() {
	engine := gin.Default()
	hand, err := localtracing.NewMonitor(&GinHanlderAdapter{
		engine: engine,
	}, "./logs")
	if err != nil {
		panic(err)
	}
	handler = hand
	engine.Use(handler.HandlerFunc())

	// 注册路由函数
	engine.GET("/random", func(ctx *gin.Context) {
		defer handler.Time()()

		randomRepo1()
		ctx.String(200, "OK")
	})

	srv := http.Server{
		Addr:    ":8080",
		Handler: engine,
	}
	go func() {
		srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	srv.Close()
	time.Sleep(3 * time.Second)
}

func randomRepo1() {
	defer handler.Time()()
	t := int64(rand.Float64() * 1000) // 毫秒

	time.Sleep(time.Duration(t) * time.Millisecond)
	if t > 800 {
		panic("timeout")
	}

	randomRepo2()
}

func randomRepo2() {
	defer handler.Time()()
	t := int64(rand.Float64() * 100) // 毫秒

	time.Sleep(time.Duration(t) * time.Millisecond)
	if t > 80 {
		panic("timeout")
	}
}
