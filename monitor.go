package localtracing

import (
	"fmt"
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

type HTTPHandler interface {
	Context(val interface{}) (*http.Request, http.ResponseWriter, error) // 获取request与response
	Get(string, func(interface{}))                                       // 用于挂载路由
}

type MonitorServer struct {
	httpHandler HTTPHandler
}

// 挂载路由
func NewMonitor(fn HTTPHandler) {
	s := MonitorServer{httpHandler: fn}
	// 健康检查
	fn.Get("/heath", s.health)
	// 获取当前所有的日志列表
	fn.Get("/log/list", s.LogList)

	// 根据日志文件获取内容 需要使用websocket持续连接
	fn.Get("/log/data", s.LogData)
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
