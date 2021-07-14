package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func generateToken(encodString string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(encodString), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Hash to store:", string(hash))

	hasher := md5.New()
	hasher.Write(hash)
	return hex.EncodeToString(hasher.Sum(nil))
}

func setRouter(router *gin.Engine, h *hub) {
	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", nil)
	})

	router.GET("/ws", func(c *gin.Context) {
		fmt.Println("ws start")
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Fatal(err)
		}
		cl := &client{conn: conn, hub: h, send: make(chan []byte)}
		cl.hub.join <- cl
		go cl.write()
		go cl.read()
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})

	router.POST("/login", func(c *gin.Context) {
		token := generateToken(c.Request.FormValue("username"))
		fmt.Println(token)
		c.Writer.Header().Set("auth", token)
		//username := c.Request.FormValue("username")
		//c.Writer.Write([]byte(fmt.Sprintf("hi %s", username)))
		//c.HTML(302, "home.html", nil)
		c.JSON(302, "ok")
	})
}

const port int = 5050

type IndexData struct {
	Title   string
	Content string
}

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("template/*")
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	h := New()
	go h.run()
	fmt.Println("start run")

	setRouter(router, h)
	fmt.Println(fmt.Sprintf(":%d", port))
	/*
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatal(err)
		}
	*/
	log.Fatal(router.Run(fmt.Sprintf(":%d", port)))
}

type hub struct {
	clients   map[*client]bool
	boradcast chan []byte
	join      chan *client
	leave     chan *client
}

//回傳指標、整個app共用
func New() *hub {
	return &hub{
		clients:   make(map[*client]bool),
		boradcast: make(chan []byte),
		join:      make(chan *client),
		leave:     make(chan *client),
	}
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.join:
			fmt.Println("join")
			h.clients[client] = true
			fmt.Println("h.clients len = ", len(h.clients))
		case client := <-h.leave:
			fmt.Println("leave")
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case msg := <-h.boradcast:
			fmt.Println("message = ", string(msg), ", count = ", len(h.clients))
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					fmt.Println("hub clients run default")
					delete(h.clients, client)
					close(client.send)
				}
			}
		default:
		}
	}
}

type client struct {
	conn *websocket.Conn
	hub  *hub
	send chan []byte
}

// method of client for select read message from webscoket and write to hub borad channel
func (c *client) read() {
	defer func() {
		c.conn.Close()
		c.hub.leave <- c
	}()
	fmt.Println("client start read")
	//不設定會中斷連結
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			//function終止了
			break
		}
		fmt.Println("write to borad = ", string(message))
		c.hub.boradcast <- message
	}
}

// method of client for select write message to webscoket
func (c *client) write() {
	ticker := time.NewTicker((60 * time.Second * 9) / 10)
	fmt.Println("client start write")
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg := <-c.send:
			fmt.Println("write=", string(msg))
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(msg)
			w.Close() //馬上寫出去
		case <-ticker.C:
			fmt.Println("ticker 延長")
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
