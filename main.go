package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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
				//close(client.send)
			}
		case msg := <-h.boradcast:
			fmt.Println("message = ", msg, ", count = ", len(h.clients))
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					fmt.Println("hub clients run default")
					delete(h.clients, client)
					//close(client.send)
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

func main() {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	h := New()
	go h.run()
	fmt.Println("start run")

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		http.ServeFile(rw, r, "index.html")
	})

	http.HandleFunc("/ws", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Println("ws start")
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		c := &client{conn: conn, hub: h, send: make(chan []byte)}
		c.hub.join <- c
		go c.write()
		go c.read()

	})

	fmt.Println("start listen 5050")
	if err := http.ListenAndServe(":5050", nil); err != nil {
		log.Fatal(err)
	}

}
