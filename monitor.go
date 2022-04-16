package localtracing

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

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
	htmlTplEngine *template.Template

	// 静态资源
	fs      = http.FileServer(http.Dir("views/assets/"))
	handler = http.StripPrefix("/static/", fs)
)

type HTTPHandler interface {
	Context(val interface{}) (*http.Request, http.ResponseWriter, error) // 获取request与response
	Get(string, func(interface{}))                                       // 用于挂载路由
	Static(string, string)                                               // 挂载静态资源目录
}

type MonitorServer struct {
	httpHandler HTTPHandler
}

// 模板引擎初始化
func init() {
	// 初始化模板引擎 并加载各层级的模板文件
	// 注意 views/* 不会对子目录递归处理 且会将子目录匹配 作为模板处理造成解析错误
	// 若存在与模板文件同级的子目录时 应指定模板文件扩展名来防止目录被作为模板文件处理
	// 然后通过 view/*/*.html 来加载 view 下的各子目录中的模板文件
	htmlTplEngine = template.New("htmlTplEngine")

	// 模板根目录下的模板文件 一些公共文件
	htmlTplEngine.ParseGlob("views/*.html")
	htmlTplEngine.ParseGlob("views/*/*.html")
}

// 挂载路由
func NewMonitor(fn HTTPHandler) {
	s := MonitorServer{httpHandler: fn}
	// 静态资源
	fn.Static("/static", "views/assets")
	// 实时日志页面
	fn.Get("/view", s.indexView)
	// 健康检查
	fn.Get("/heath", s.health)
	// 获取当前所有的日志列表
	fn.Get("/log/list", s.LogList)

	// 根据日志文件获取内容 需要使用websocket持续连接
	fn.Get("/log/data", s.LogData)
}

func (s *MonitorServer) indexView(ctx interface{}) {
	_, w, err := s.httpHandler.Context(ctx)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	} else {
		if err := htmlTplEngine.ExecuteTemplate(
			w,
			"index",
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
