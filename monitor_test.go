package localtracing

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
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

func (g GinHanlderAdapter) Static(url, path string) {
	g.engine.StaticFS(url, http.Dir(path))
}

func (g GinHanlderAdapter) Get(url string, fn func(interface{})) {
	g.engine.GET(url, func(ctx *gin.Context) {
		fn(ctx)
	})
}

func TestGinMonitor(t *testing.T) {
	engine := gin.Default()

	NewMonitor(&GinHanlderAdapter{engine: engine})

	srv := http.Server{
		Handler: engine,
		Addr:    ":8080",
	}

	srv.ListenAndServe()
}
