package localtracing

import (
	"io"
	"log"
	"os"
	"path"
	"time"
)

var (
	Trace   *log.Logger
	logFile *os.File
)

func NewTracingLog(logPath string) {
	os.MkdirAll(logPath, 0666)

	var err error
	logFile, err = os.OpenFile(path.Join(logPath, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open error log file: ", err)
	}
	// defer file.Close()

	Trace = log.New(io.MultiWriter(logFile), "TRACING: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func TracingTime(funcName string) func() {
	tracingID := ""
	if val, ok := tracingContext.Get(goID()); !ok {
		return func() {}
	} else {
		tracingID = val.(string)
	}

	now := time.Now()
	Trace.Println(tracingID, funcName, "开始执行时间", time.Now().Format("2006-01-02 15:05:06"))
	return func() {
		Trace.Println(tracingID, funcName, "结束执行时间", time.Now().Format("2006-01-02 15:05:06"), "耗时", time.Since(now))
	}
}

func Close() {
	logFile.Close()
}
