package logger

import (
	"os"
	"path"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/wwqdrh/localtracing/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	TraceLogger *zap.Logger
)

func NewTracingLog(logDir string) error {
	if ok, _ := utils.PathExists(logDir); !ok {
		_ = os.MkdirAll(logDir, os.ModePerm)
	}
	logPath := path.Join(logDir, "trace.log")

	// 日志级别
	infoPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev == zap.InfoLevel
	})

	// 日志config
	encoderConfig := zapcore.EncoderConfig{
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

	// lumberjack rotate
	// writer := zapcore.AddSync(&lumberjack.Logger{
	// 	Filename:   fmt.Sprintf("./%s/server_info.log", logPath), // 日志文件的位置
	// 	MaxSize:    10,                                           // 在进行切割之前，日志文件的最大大小（以MB为单位）
	// 	MaxBackups: 200,                                          // 保留旧文件的最大个数
	// 	MaxAge:     30,                                           // 保留旧文件的最大天数
	// 	Compress:   true,                                         // 是否压缩/归档旧文件
	// })

	/* 日志轮转相关函数
	`WithLinkName` 为最新的日志建立软连接
	`WithRotationTime` 设置日志分割的时间，隔多久分割一次
	WithMaxAge 和 WithRotationCount二者只能设置一个
	  `WithMaxAge` 设置文件清理前的最长保存时间
	  `WithRotationCount` 设置文件清理前最多保存的个数
	*/
	// rotatelogs.New
	// 下面配置日志每隔 四小时 轮转一个新文件，保留最近 7天的日志文件，多余的自动清理掉。
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d%H",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(4)*time.Hour),
	)
	if err != nil {
		return err
	}

	TraceLogger = zap.New(
		zapcore.NewTee(
			zapcore.NewCore(
				zapcore.NewJSONEncoder(encoderConfig),
				zapcore.AddSync(writer), infoPriority,
			),
		),
		zap.AddCaller(),
	)

	return nil
}
