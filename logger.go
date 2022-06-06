package localtracing

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hpcloud/tail"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LocalTracing struct {
	LogDir       string
	DebugLogger  *zap.Logger
	InfoLogger   *zap.Logger
	WarnLogger   *zap.Logger
	ErrorLogger  *zap.Logger
	DPanicLogger *zap.Logger
	PanicLogger  *zap.Logger
	FatalLogger  *zap.Logger
}

var (
	localTracing *LocalTracing
	// 默认日志文件
	baseLog = "base.log"

	debugPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.DebugLevel
	})
	infoPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.InfoLevel
	})
	warnPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.WarnLevel
	})
	errorPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.ErrorLevel
	})
	dPanicPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.DPanicLevel
	})
	panicPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.PanicLevel
	})
	fatalPriority = zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.FatalLevel
	})

	// 日志config
	encoderConfig = zapcore.EncoderConfig{
		TimeKey:  "time",
		LevelKey: "level",
		NameKey:  "logger",
		// CallerKey:      "linenum",
		MessageKey: "msg",
		// StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,  // 小写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,     // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		EncodeCaller:   zapcore.FullCallerEncoder,      // 全路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}
)

func NewLocaltracing(logDir string) (*LocalTracing, error) {
	if ok, _ := PathExists(logDir); !ok {
		_ = os.MkdirAll(logDir, os.ModePerm)
	}
	logPath := path.Join(logDir, baseLog)

	// rotatelogs.New
	// 下面配置日志每隔 四小时 轮转一个新文件，保留最近 7天的日志文件，多余的自动清理掉。
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d%H",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(4)*time.Hour),
	)
	if err != nil {
		return nil, err
	}

	handler := &LocalTracing{
		LogDir: logDir,
		DebugLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), debugPriority,
				),
			),
			zap.AddCaller(),
		),
		InfoLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), infoPriority,
				),
			),
			zap.AddCaller(),
		),
		WarnLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), warnPriority,
				),
			),
			zap.AddCaller(),
		),
		ErrorLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), errorPriority,
				),
			),
			zap.AddCaller(),
		),
		DPanicLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), dPanicPriority,
				),
			),
			zap.AddCaller(),
		),
		PanicLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), panicPriority,
				),
			),
			zap.AddCaller(),
		),
		FatalLogger: zap.New(
			zapcore.NewTee(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(writer), fatalPriority,
				),
			),
			zap.AddCaller(),
		),
	}
	localTracing = handler
	go handler.Sync()
	return handler, nil
}

func GetLocalTracing() *LocalTracing {
	return localTracing
}

func (l *LocalTracing) HandlerFunc() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer ClearContext() // 清除当前上下文

		start := time.Now()
		errField := l.LogCallInfo(ctx)
		fields := []zap.Field{
			zap.String("method", ctx.Request.Method),
			zap.String("host", ctx.Request.Host),
			zap.String("url", fmt.Sprintf("%s %s", ctx.Request.RequestURI, ctx.Request.Proto)),
			zap.String("remote", ctx.Request.RemoteAddr),
			zap.String("call", fmt.Sprintf("%s %v", GetContextJson(), time.Since(start))),
		}
		if errField != nil {
			fields = append(fields,
				zap.Any("exception", errField),
				zap.Error(errors.New("crash")),
				zap.String("status", "Internal Servre Error"),
			)
			l.ErrorLogger.Error("request:", fields...)
			ctx.String(500, "found error")
		} else {
			fields = append(fields,
				zap.String("status", "Success"),
			)
			l.InfoLogger.Info("request:", fields...)
		}
	}
}

// 执行next并处理异常
func (l *LocalTracing) LogCallInfo(ctx *gin.Context) (res *zapcore.Field) {
	defer func() {
		if err := recover(); err != nil {
			defer func() { recover() }()
			switch err.(type) {
			default:
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
				field := zap.String("stack", strings.Join(arr, " <- "))
				res = &field
			}
		}
	}()
	ctx.Next()
	return nil
}

func (l *LocalTracing) Sync() {
	c1 := make(chan os.Signal, 1)
	signal.Notify(c1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 接受信号同步日志
	l.InfoLogger.Sync()
	l.DebugLogger.Sync()
	l.WarnLogger.Sync()
	l.ErrorLogger.Sync()
	l.DPanicLogger.Sync()
	l.PanicLogger.Sync()
	l.FatalLogger.Sync()
}

// time时间
// 设置当前上下文的函数名字以及执行时间
func (l *LocalTracing) Time() func() {
	start := time.Now()
	fnname := FnName(2)
	return func() {
		SetContextValue(fnname, fmt.Sprintf("%dms", int(time.Since(start).Milliseconds())))
	}
}

// 没一个要读取的file可能由多个ws连接， 要复用则包装tails，并加上一系列channel

func (l *LocalTracing) TailLog(fileName string) chan string {
	cur := make(chan string, 1000)
	if val, ok := tailHandler[fileName]; ok {
		val.chs = append(val.chs, cur)
		return cur
	}

	config := tail.Config{
		ReOpen:    true,                                 // 重新打开
		Follow:    true,                                 // 是否跟随
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件的哪个地方开始读
		MustExist: false,                                // 文件不存在不报错
		Poll:      true,
	}
	tails, err := tail.TailFile(fileName, config)
	if err != nil {
		fmt.Println("tail file failed, err:", err)
		return nil
	}
	handler := &tailInfo{
		cmd: tails,
		chs: []chan string{cur},
	}
	tailHandler[fileName] = handler
	go func() {
		defer func() {
			recover() // 可能会向close channel写数据
		}()

		var (
			line *tail.Line
			ok   bool
		)
		for {
			line, ok = <-tails.Lines
			if !ok {
				fmt.Printf("tail file close reopen, filename:%s\n", tails.Filename)
				time.Sleep(time.Second)
				continue
			}

			// 为所有的channel发送
			for _, ch := range handler.chs {
				select {
				case ch <- line.Text: // TODO 有可能是close channel 需要加上判断
				default:
				}
			}
		}
	}()
	return cur
}
