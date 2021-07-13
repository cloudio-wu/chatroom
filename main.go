package main

import (
	"fmt"
	"html/template"
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
	clients map[*client]bool
	borad   chan []byte
	join    chan *client
	leave   chan *client
}

//回傳指標、整個app共用
func New() *hub {
	return &hub{
		clients: make(map[*client]bool),
		borad:   make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
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
		case msg := <-h.borad:
			fmt.Println("message = ", msg, ", count = ", len(h.clients))
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					fmt.Println("hub clients run default")
					//delete(h.clients, client)
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

func (c *client) read() {
	defer func() {
		c.conn.Close()
		c.hub.leave <- c
	}()
	fmt.Println("client start read")
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
		c.hub.borad <- message
	}
}

func (c *client) write() {
	ticker := time.NewTicker((60 * time.Second * 9) / 10)
	fmt.Println("client start write")
	for {
		select {
		case msg := <-c.hub.borad:
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
	http.HandleFunc("/ws", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Println("ws start")
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			fmt.Println("error")
			log.Fatal(err)
		}
		client := &client{conn: conn, hub: h}
		client.hub.join <- client
		go client.read()
		go client.write()

	})

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		temp, err := template.ParseFiles("./index.html")
		if err != nil {
			log.Fatal(err)
		}
		temp.Execute(rw, nil)
	})
	fmt.Println("start listen 5050")
	if err := http.ListenAndServe(":5050", nil); err != nil {
		log.Fatal(err)
	}

}
