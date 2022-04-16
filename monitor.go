package localtracing

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/websocket"
	"github.com/wwqdrh/localtracing/logger"
)

////////////////////
// 监控组件 提供服务让外部实时访问日志
// 同时也可以实现让control组件来一起管理这些服务(在同一的地方来查看与管理这些服务的中的日志内容，服务注册与发现的思想)
////////////////////

var upgrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
} // use default options

var (
	htmlTplEngine *bindataTemplate

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
		Static(string, http.FileSystem)                                      // 挂载静态资源目录
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
func NewMonitor(fn HTTPHandler) {
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
	_, w, err := s.httpHandler.Context(ctx)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	} else {
		// htmlTplEngine.ExecuteTemplate()
		if err := ExecuteBinTemplate(
			w,
			"index",
			"views/index.html",
			map[string]interface{}{"PageTitle": "实时日志"},
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
	r, w, err := s.httpHandler.Context(ctx)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("upgrade error: " + err.Error()))
		return
	}
	defer ws.Close()

	file := r.URL.Query().Get("file")
	ch := logger.TailLog(file)

	go func() {
		for {
			mt, message, err := ws.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				// close(ch) // TODO 进行关闭
				break
			}
			fmt.Printf("messageType: %d, recv: %s\n", mt, string(message))
		}
	}()

	for {
		select {
		case line := <-ch:
			err = ws.WriteMessage(websocket.TextMessage, []byte(line))
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}
