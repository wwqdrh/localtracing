package localtracing

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"

	"net/http/pprof"

	"github.com/hpcloud/tail"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/websocket"
)

////////////////////
// 监控组件 提供服务让外部实时访问日志
// 同时也可以实现让control组件来一起管理这些服务(在同一的地方来查看与管理这些服务的中的日志内容，服务注册与发现的思想)
////////////////////

var tailHandler = map[string]*tailInfo{} // 文件名与channel的映射

type tailInfo struct {
	cmd *tail.Tail // 获取最新的日志数据
	chs []connNode // 多个连接进行复用
}

type connNode struct {
	ch  chan string
	ctx context.Context
}

var upgrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
} // use default options

var (
	// 静态资源
	fs = &assetfs.AssetFS{
		Asset: func(name string) ([]byte, error) {
			return Asset(name)
		},
		AssetDir: func(name string) ([]string, error) {
			return AssetDir(name)
		},
	}
)

type (
	HTTPHandler interface {
		Context(val interface{}) (*http.Request, http.ResponseWriter, error) // 获取request与response
		Get(string, func(interface{}))                                       // 用于挂载路由
		Post(string, func(interface{}))
		Static(string, http.FileSystem) // 挂载静态资源目录
	}

	// bindata-template包装
	AssetFunc func(string) ([]byte, error)

	bindataTemplate struct {
		*template.Template

		AssetFunc AssetFunc
	}
)
type MonitorServer struct {
	httpHandler HTTPHandler
}

// 挂载路由
func NewMonitor(fn HTTPHandler, logDir string) (*LocalTracing, error) {
	handler, err := NewLocaltracing(logDir)
	if err != nil {
		return nil, err
	}

	s := MonitorServer{httpHandler: fn}
	// 静态资源
	fn.Static("/static", fs)
	// 实时日志页面
	fn.Get("/view", s.indexView)
	// 健康检查
	fn.Get("/heath", s.health)
	// 获取当前所有的日志列表
	fn.Get("/log/list", s.LogList)

	// 根据日志文件获取内容 需要使用websocket持续连接
	fn.Get("/log/data", s.LogData)

	// 开启pprof
	s.EnableProf()

	return handler, nil
}

// bindatatemplate 方法
func ExecuteBinTemplate(wr io.Writer, name, path string, data interface{}) error {
	tmpl := &bindataTemplate{
		Template:  template.New(name),
		AssetFunc: Asset,
	}

	tmplBytes, err := tmpl.AssetFunc(path)
	if err != nil {
		return err
	}
	newTmpl, err := tmpl.Parse(string(tmplBytes))
	if err != nil {
		return err
	}
	return newTmpl.Execute(wr, data)
}

// 路由函数
func (s *MonitorServer) indexView(ctx interface{}) {
	r, w, err := s.httpHandler.Context(ctx)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	} else {
		// htmlTplEngine.ExecuteTemplate()
		logfile := r.URL.Query().Get("file")
		if err := ExecuteBinTemplate(
			w,
			"index",
			"views/index.html",
			map[string]interface{}{"PageTitle": "实时日志", "LogFile": logfile},
		); err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}
	}
}

// 健康检查
func (s *MonitorServer) health(ctx interface{}) {
	_, w, err := s.httpHandler.Context(ctx)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}
}

// 日志文件列表
func (s *MonitorServer) LogList(ctx interface{}) {
	_, w, err := s.httpHandler.Context(ctx)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}
}

// ws: 日志实时记录
func (s *MonitorServer) LogData(ctx interface{}) {
	r, w, _ := s.httpHandler.Context(ctx)

	// check log is exist?
	file := path.Join(Tracing.LogDir, r.URL.Query().Get("file"))
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		w.WriteHeader(500)
		w.Write([]byte("log error: 日志文件不存在"))
		return
	}

	// protocol upgrade
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("upgrade error: " + err.Error()))
		return
	}
	// ctx
	conte, cancel := context.WithCancel(context.TODO())
	go WsRead(ws, conte, cancel)
	go WsWrite(ws, Tracing.TailLog(file, conte), conte, cancel)
}

func (s *MonitorServer) EnableProf() {
	prefix := "/pprof"

	s.httpHandler.Get(prefix+"/", WrapF(s.httpHandler, pprof.Index))
	s.httpHandler.Get(prefix+"/cmdline", WrapF(s.httpHandler, pprof.Cmdline))
	s.httpHandler.Get(prefix+"/profile", WrapF(s.httpHandler, pprof.Profile))
	s.httpHandler.Get(prefix+"/Symbol", WrapF(s.httpHandler, pprof.Symbol))
	s.httpHandler.Post(prefix+"/Symbol", WrapF(s.httpHandler, pprof.Symbol))
	s.httpHandler.Get(prefix+"/trace", WrapF(s.httpHandler, pprof.Trace))
	s.httpHandler.Get(prefix+"/allocs", WrapH(s.httpHandler, pprof.Handler("allocs")))
	s.httpHandler.Get(prefix+"/block", WrapH(s.httpHandler, pprof.Handler("block")))
	s.httpHandler.Get(prefix+"/goroutine", WrapH(s.httpHandler, pprof.Handler("goroutine")))
	s.httpHandler.Get(prefix+"/heap", WrapH(s.httpHandler, pprof.Handler("heap")))
	s.httpHandler.Get(prefix+"/mutex", WrapH(s.httpHandler, pprof.Handler("mutex")))
	s.httpHandler.Get(prefix+"/threadcreate", WrapH(s.httpHandler, pprof.Handler("threadcreate")))
}
