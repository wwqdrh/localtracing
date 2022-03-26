package localtracing

import (
	"github.com/gin-gonic/gin"
	"github.com/wwqdrh/localtracing/logger"
	"github.com/wwqdrh/localtracing/trace"
	"github.com/wwqdrh/localtracing/utils"
)

var (
	TracingTime = trace.TracingTime
	DefaultTime = trace.DefaultTimer
)

func Register(r *gin.Engine, tracingPath string) {
	logger.NewTracingLog(tracingPath)
	r.Use(TracingMiddleware())
	r.GET("/tracing", func(ctx *gin.Context) {
		ctx.String(200, "web搜索界面")
	})
}

func TracingMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		trace.AddContext(utils.GenTracingID(2))
		ctx.Next()
	}
}
