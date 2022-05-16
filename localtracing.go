package localtracing

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/wwqdrh/localtracing/logger"
)

var (
	// TracingTime = trace.TracingTime
	// DefaultTime = trace.DefaultTimer
	logHandler *logger.Handler
)

func NewLocaltracingHandler() {
	//var err error
	//var Logger *zap.Logger
	//Logger, err = zap.NewProduction()
	//if err != nil {
	//	fmt.Println("Cannot initialize logging")
	//	return
	//}
	//Logger.Info("test, test")

	ZapRotateSync := &RotateLogWriteSyncer{}
	ZapRotateSync.RotateLoggerInit("MIDNIGHT", 0, "try.log", 3)
	zapcore.Lock(ZapRotateSync)

	productionEncoderConfig := zap.NewProductionEncoderConfig()
	productionEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	Logger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(productionEncoderConfig),
		zapcore.NewMultiWriteSyncer(os.Stdout, ZapRotateSync), zap.NewAtomicLevelAt(zapcore.InfoLevel)), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	defer Logger.Sync()
	for i := 1; i < 100000; i++ {
		Logger.Info("test >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> " + strconv.FormatInt(time.Now().Unix(), 10))

		Logger.Error("Kartor HandleKafkaMessage", zap.String("topic", "topic"), zap.String("key", "key"), zap.String("value", "value"))

		Logger.Debug("test >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> " + strconv.FormatInt(time.Now().Unix(), 10))
	}

	Logger.Info("*************************************************** Done ***************************************************")
}

func SetLogLevel(level int) {
	logHandler.SetLevel(level)
}

// 链路调用的logger记录工具
func TracingMiddleware(path string) gin.HandlerFunc {
	logHandler = logger.NewAsyncHandler(path, "router", nil, 2, 5, 2)

	return func(ctx *gin.Context) {
		ctx.Set("lg", logHandler) // 设置在上下文中，这样函数就能使用了

		et := zerolog.Dict()
		et.Str("method", ctx.Request.Method)
		et.Str("host", ctx.Request.Host)
		et.Str("url", fmt.Sprintf("%s %s", ctx.Request.RequestURI, ctx.Request.Proto))
		et.Interface("header", ctx.Request.Header)
		et.Str("remote", ctx.Request.RemoteAddr)
		if strings.Contains(strings.ToLower(ctx.Request.Header.Get("Content-Type")), "form") && ctx.Request.Method != http.MethodGet {
			ctx.Request.ParseForm()
			if len(ctx.Request.Form) > 0 {
				et.Interface("form", ctx.Request.Form)
			}
		}

		isCrashed := false
		start := time.Now()
		stack := handle(ctx)
		et.Str("call", fmt.Sprintf("%s %v", "TODO", time.Since(start)))
		if stack != nil {
			et.Dict("exception", stack)
			isCrashed = true
		}

		var lg *zerolog.Event
		if isCrashed {
			lg = logHandler.Error("crash")
		} else if ctx.Value("error") != nil {
			lg = logHandler.Error("error")
		} else {
			lg = logHandler.Debug("debug")
		}

		lg.Dur("cost", time.Since(start)).Dict("request", et).Dict("params", et)
		if isCrashed {
			lg.Msg("Internal Server Error")
		} else {
			lg.Msg("Success")
		}
	}
}

// 执行next并处理异常
func handle(ctx *gin.Context) (crashet *zerolog.Event) {
	defer func() {
		if err := recover(); err != nil {
			defer func() { recover() }()
			ctx.Set("error", true)

			switch err.(type) {
			default:
				crashet = zerolog.Dict()
				crashet.Interface("crash", err)
				dep := 0
				size := 10
				arr := make([]string, 0, size)
				for i := 1; i < size; i++ {
					_, file, line, ok := runtime.Caller(i)
					if !ok {
						break
					}
					if strings.Contains(file, "/runtime/") || strings.Contains(file, "/reflect/") {
						continue
					}
					arr = append(arr, fmt.Sprintf("%s:%d", file, line))
					dep++
				}
				crashet.Str("stack", strings.Join(arr, " <- "))
				ctx.String(500, "found error")
			}
		}
	}()

	ctx.Next()
	return
}
