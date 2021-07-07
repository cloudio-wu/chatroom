package main

import (
	viewmodels "chungcheng/chatroom/src/viewmodel"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
)

func main() {
	var conw []socketio.Conn = make([]socketio.Conn, 0)
	fmt.Println("process start")

	server := socketio.NewServer(nil)
	//socket.OnEvent("ssdss", "", func() {})
	server.OnConnect("connection", func(conn socketio.Conn) (err error) {
		conn.SetContext("")
		conw = append(conw, conn)
		fmt.Println("connected:", conn.ID())
		return nil
	})

	server.OnEvent("/", "notice", func(s socketio.Conn, msg string) {
		fmt.Println("notice:", msg)
		for _, val := range conw {
			val.Emit("reply", msg)
		}
	})
	server.OnEvent("/chat", "msg", func(s socketio.Conn, msg string) string {
		s.SetContext(msg)
		return "recv " + msg
	})
	server.OnEvent("/", "msg", func(s socketio.Conn, msg string) string {
		s.SetContext(msg)
		return "recv " + msg
	})
	server.OnEvent("/", "bye", func(s socketio.Conn) string {
		last := s.Context().(string)
		s.Emit("bye", last)
		s.Close()
		return last
	})
	go server.Serve()
	defer server.Close()

	router := gin.Default()
	router.LoadHTMLGlob("template/*")
	router.GET("/", handlerHtml)
	router.GET("socket.io", gin.WrapH(server))
	router.POST("socket.io", func(contex *gin.Context) {
		server.ServeHTTP(contex.Writer, contex.Request)
	})
	log.Fatal(router.Run(":5000"))
}

func handlerHtml(c *gin.Context) {
	data := new(viewmodels.IndexData)
	data.Title = "首頁 hi"
	data.Content = "我的第一個首頁"
	c.HTML(http.StatusOK, "index.html", data)
}
