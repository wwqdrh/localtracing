package localtracing

import (
	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, tracingPath string) {
	NewTracingLog(tracingPath)
	r.Use(TracingMiddleware())
	r.GET("/tracing", func(ctx *gin.Context) {
		ctx.String(200, "web搜索界面")
	})
}

func TracingMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		addContext(GenTracingID(2))
		ctx.Next()
	}
}
