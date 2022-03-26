package logger

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/wwqdrh/localtracing/logger/timecost"
)

type Handler struct {
	path          string      // 日志根目录，默认目录：./log
	label         string      // 日志标签，子目录
	clone         io.Writer   // 日志克隆输出接口
	compressCount int         // 仅在按日压缩模式下有效，设置为压缩几天前的日志，支持大于等于1的数字
	compressKeep  int         // 前多少次的压缩文件删除掉。例如：1=保留最近1个压缩日志，2=保留最近2个压缩日志，依次类推。。。
	async         bool        // 一步io
	arrLogger     []*zlog     // 按文件名创建的句柄表，是一个不存在则增加，存在则不变的列表
	lock          *sync.Mutex // 句柄表保护锁
	level         int         // 当前日志级别
	defaultLevel  int         // 默认日志级别
}

type zlog struct {
	name string
	zerolog.Logger
}

type LogLevel struct {
	Path  string
	Label string
	Name  string
	Level int
}

type resetLevelCmd struct {
	h *Handler
	d time.Duration
}

var (
	fileHook zerolog.HookFunc

	ch = make(chan *resetLevelCmd, 100)
)

func init() {
	// 默认的输出错误堆栈信息方式
	zerolog.ErrorStackMarshaler = func(err error) interface{} {
		var arr []string
		size := 10
		for i := 2; i < size; i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			if strings.Contains(file, "runtime") {
				break
			}
			arr = append(arr, fmt.Sprintf("%s:%d", file, line))
		}
		return strings.Join(arr, "<-")
	}

	// 默认的调用信息处理
	fileHook = func(e *zerolog.Event, level zerolog.Level, message string) {
		_, file, line, _ := runtime.Caller(4)
		e.Str("File", fmt.Sprintf("%s:%d", file, line))
	}

	go resetLogLevel()
}

// NewHandler 同步io的日志句柄
func NewHandler(path, label string, clone io.Writer, count, keep int, defaultlevel ...int) *Handler {
	return newHandler(path, label, clone, count, keep, false, defaultlevel...)
}

// NewAsyncHandler 异步io的日志句柄
func NewAsyncHandler(path, label string, clone io.Writer, count, keep int, defaultlevel ...int) *Handler {
	return newHandler(path, label, clone, count, keep, true, defaultlevel...)
}

func newHandler(path, label string, clone io.Writer, count, keep int, async bool, defaultlevel ...int) *Handler {
	h := &Handler{
		path:          path,
		label:         label,
		clone:         clone,
		compressCount: count,
		compressKeep:  keep,
		lock:          &sync.Mutex{},
		async:         true,
		level:         int(zerolog.ErrorLevel),
		defaultLevel:  int(zerolog.ErrorLevel),
	}

	if len(defaultlevel) > 0 {
		h.defaultLevel = defaultlevel[0]
		h.level = h.defaultLevel
	}

	return h
}

// RouteWithLogTo 路由包装，日志输出到指定文件中
func (o *Handler) RouteWithLogTo(file string, do func(w http.ResponseWriter, r *http.Request, et *zerolog.Event), aggCost ...bool) http.HandlerFunc {
	fc := reflect.ValueOf(do).Pointer()
	f, l := runtime.FuncForPC(fc).FileLine(fc)
	name := runtime.FuncForPC(fc).Name()
	info := fmt.Sprintf("%s(%s:%d)", name, f, l)
	b := len(aggCost) > 0 && aggCost[0] //是否需要聚合该接口的耗时情况

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		et := zerolog.Dict()
		et.Str("method", r.Method)
		et.Str("host", r.Host)
		et.Str("url", fmt.Sprintf("%s %s", r.RequestURI, r.Proto))
		et.Interface("header", r.Header)
		et.Str("remote", r.RemoteAddr)
		if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "form") && r.Method != http.MethodGet {
			r.ParseForm()
			if len(r.Form) > 0 {
				et.Str("form", r.Form.Encode())
			}
		}

		ae := zerolog.Dict()
		defer func() {
			cost := time.Since(start)

			if e := recover(); e != nil {
				defer func() { recover() }()
				crashet := zerolog.Dict()
				crashet.Interface("crash", e)
				dep := 0
				size := 10
				arr := make([]string, 0, size)
				for i := 1; i < size; i++ {
					_, f, l, ok := runtime.Caller(i)
					if !ok {
						break
					}
					if strings.Contains(f, "/runtime/") || strings.Contains(f, "/reflect/") {
						continue
					}
					arr = append(arr, fmt.Sprintf("%s:%d", f, l))
					dep++
				}
				crashet.Str("stack", strings.Join(arr, " <- "))

				et.Str("call", fmt.Sprintf("%s %v", info, cost))
				et.Dict("exception", crashet)

				lg := o.Error(file).Dur("cost", cost).Dict("request", et).Dict("params", ae)
				lg.Msg("Internal Server Error")
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				o.Debug(file).Dur("cost", cost).Dict("request", et).Dict("params", ae).Msg("success")
			}

			if b {
				timecost.AggCost(fmt.Sprintf("%s %s", r.Method, r.URL.Path), cost)
			}
		}()

		do(w, r, ae)
	}
}

func (o *Handler) File(file ...string) *zlog {
	name := "default"
	if len(file) > 0 {
		name = strings.ToLower(file[0])
	}
	lg, size := o.tryGetLogger(name)
	if lg != nil {
		return lg
	}

	return o.tryAppendLogger(name, size)
}

// 修改某一业务日志的级别，低于默认级别时要设置发送切回请求
func (o *Handler) SetLevel(lv int, d ...time.Duration) {
	for _, lg := range o.arrLogger {
		lg.Level(lv)
	}
	o.level = lv
	if lv >= int(o.defaultLevel) {
		return
	}
	if len(d) > 0 {
		ch <- &resetLevelCmd{h: o, d: d[0]}
	} else {
		ch <- &resetLevelCmd{h: o}
	}
}

// 修改某一业务日志的默认级别，所有操作对象的级别变更到不低于新的默认级别
func (o *Handler) SetDefaultLevel(lv int) {
	o.defaultLevel = lv
	if o.level >= int(o.defaultLevel) {
		return
	}
	for _, lg := range o.arrLogger {
		lg.Level(lv)
	}
}

func (o *Handler) GetLevel() (res []LogLevel) {
	for _, lg := range o.arrLogger {
		res = append(res, LogLevel{Path: o.path, Label: o.label, Name: lg.name, Level: int(lg.GetLevel())})
	}
	return
}

func (o *Handler) Trace(file ...string) *zerolog.Event {
	return o.File(file...).Trace()
}

func (o *Handler) Debug(file ...string) *zerolog.Event {
	return o.File(file...).Debug()
}

func (o *Handler) Info(file ...string) *zerolog.Event {
	return o.File(file...).Info()
}

func (o *Handler) Warn(file ...string) *zerolog.Event {
	return o.File(file...).Warn()
}

func (o *Handler) Error(file ...string) *zerolog.Event {
	return o.File(file...).Error()
}

func (o *Handler) Err(err error, file ...string) *zerolog.Event {
	return o.File(file...).Err(err)
}

func (o *Handler) Panic(file ...string) *zerolog.Event {
	return o.File(file...).Panic()
}

func (o *Handler) WithLevel(level zerolog.Level, file ...string) *zerolog.Event {
	return o.File(file...).WithLevel(level)
}

func (o *Handler) Log(file ...string) *zerolog.Event {
	return o.File(file...).Log()
}

func (o *Handler) tryGetLogger(file string) (*zlog, int) {
	size := len(o.arrLogger)
	for _, lg := range o.arrLogger {
		if lg.name == file {
			return lg, 0
		}
	}

	return nil, size
}

func (o *Handler) tryAppendLogger(file string, size int) *zlog {
	o.lock.Lock()
	defer o.lock.Unlock()

	if len(o.arrLogger) > size {
		for _, lg := range o.arrLogger[size:] {
			if lg.name == file {
				return lg
			}
		}
	}

	var opts []FileOptionHandle
	if o.path != "" {
		opts = append(opts, WithPath(o.path))
	}
	if o.label != "" {
		opts = append(opts, WithLabel(o.label))
	}
	if o.clone != nil {
		opts = append(opts, WithClone(o.clone))
	}
	if o.compressCount > 0 {
		if o.compressKeep > 0 {
			o.compressKeep += o.compressCount
		}
		opts = append(opts, WithCompress(o.compressCount, o.compressKeep))
	}
	opts = append(opts, WithName(file))
	opts = append(opts, WithAsync(o.async))

	lg := &zlog{name: file, Logger: zerolog.New(NewFileWriter(NewOption(opts...)))}
	lg.Logger = lg.Hook(fileHook)
	// 新的日志操作对象级别设置为当前句柄的级别，时间到了之后会自动切换回默认级别
	lg.Logger = lg.Logger.Level(zerolog.Level(o.level))

	o.arrLogger = append(o.arrLogger, lg)
	return lg
}

func (o *zlog) Level(l int) {
	if o == nil {
		return
	}
	o.Logger = o.Logger.Level(zerolog.Level(l))
}

func (o *zlog) Trace() *zerolog.Event {
	return o.Logger.Trace().Timestamp()
}

func (o *zlog) Debug() *zerolog.Event {
	return o.Logger.Debug().Timestamp()
}

func (o *zlog) Info() *zerolog.Event {
	return o.Logger.Info().Timestamp()
}

func (o *zlog) Warn() *zerolog.Event {
	return o.Logger.Warn().Timestamp()
}

func (o *zlog) Error() *zerolog.Event {
	return o.Logger.Error().Timestamp()
}

func (o *zlog) Err(err error) *zerolog.Event {
	if err != nil {
		return o.Error().Err(err)
	}
	return o.Info()
}

func (o *zlog) Panic() *zerolog.Event {
	return o.Logger.Panic().Timestamp()
}

func (o *zlog) WithLevel(level zerolog.Level) *zerolog.Event {
	return o.Logger.WithLevel(level).Timestamp()
}

func (o *zlog) Log() *zerolog.Event {
	return o.Logger.Log().Timestamp()
}

// 查看所有队列中的消息总数
func MessageRemaining() int {
	return msgRemaining()
}

// 查看因处理不过来而丢弃了多少条日志
func MessageDroped() int64 {
	return msgDroped()
}

// 设置警告信息处理方法
func SetWarnFunc(f func(string)) {
	warnFunc = f
}

func SetGlobalLevel(lv int, d time.Duration) {
	zerolog.SetGlobalLevel(zerolog.Level(lv))
}

// 自动恢复日志级别到h.defaultLevel级别
func resetLogLevel() {
	m := make(map[*Handler]*time.Timer)
	// 恢复默认日志级别的默认间隔时间
	d := time.Minute * 10
	for {
		select {
		case cmd := <-ch:
			if cmd.d == 0 {
				cmd.d = d
			}
			t := m[cmd.h]
			if t == nil {
				t = time.NewTimer(cmd.d)
				m[cmd.h] = t
			} else {
				t.Reset(cmd.d)
			}
		default:
			for h, t := range m {
				select {
				case <-t.C:
					h.SetLevel(h.defaultLevel)
				default:
				}
			}
			time.Sleep(time.Second)
		}
	}
}
