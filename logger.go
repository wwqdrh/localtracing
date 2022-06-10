package localtracing

import (
	"context"
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
	"github.com/wwqdrh/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Tracing *LocalTracing
	// 默认日志文件
	baseLog = "base.log"
)

type LocalTracing struct {
	*zap.Logger

	LogDir string
}

func NewLocaltracing(logDir string) (*LocalTracing, error) {
	if ok, _ := PathExists(logDir); !ok {
		_ = os.MkdirAll(logDir, os.ModePerm)
	}

	handler := &LocalTracing{
		Logger: logger.NewLogger(logger.WithColor(true), logger.WithLogPath(path.Join(logDir, baseLog))),
		LogDir: logDir,
	}
	go func() {
		c1 := make(chan os.Signal, 1)
		signal.Notify(c1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		// 接受信号同步日志
		handler.Sync()
	}()
	return handler, nil
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
			l.Logger.Error("request:", fields...)
			ctx.String(500, "found error")
		} else {
			fields = append(fields,
				zap.String("status", "Success"),
			)
			l.Info("request:", fields...)
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

// time时间
// 设置当前上下文的函数名字以及执行时间
func (l *LocalTracing) Time() func() {
	start := time.Now()
	fnname := FnName(2)
	return func() {
		SetContextValue(fnname, fmt.Sprintf("%dms", int(time.Since(start).Milliseconds())))
	}
}

// 每一个要读取的file可能由多个ws连接， 要复用则包装tails，并加上一系列channel
func (l *LocalTracing) TailLog(fileName string, ctx context.Context) chan string {
	cur := make(chan string, 1000)
	if val, ok := tailHandler[fileName]; ok {
		val.chs = append(val.chs, connNode{
			ch:  cur,
			ctx: ctx,
		})
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
		chs: []connNode{
			{
				ch:  cur,
				ctx: ctx,
			},
		},
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

			// 删除已经关闭的协程数
			old := handler.chs
			handler.chs = handler.chs[:0]
			for _, item := range old {
				select {
				case <-item.ctx.Done():
					continue
				default:
					handler.chs = append(handler.chs, item)
				}
			}

			// 为所有的channel发送
			for _, item := range handler.chs {
				select {
				case <-item.ctx.Done():
					continue
				case item.ch <- line.Text: // TODO 有可能是close channel 需要加上判断
				default:
				}
			}
		}
	}()
	return cur
}
