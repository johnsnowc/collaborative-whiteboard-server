package main

import (
	"bytes"
	"collaborative-whiteboard-server/model"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	id    string
	start time.Time
	hub   *Hub
	conn  *websocket.Conn
	send  chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			log.Println("err!=nil")
			break
		}
		log.Println(fmt.Sprintf("recv from id: %s ,message: %s", c.id, message))
		message = bytes.TrimSpace(message)
		//ws收发消息过快会合并消息，以换行符分割
		messages := bytes.Split(message, newline)
		for i := 0; i < len(messages); i++ {
			var ms model.Message
			_ = jsoniter.Unmarshal(messages[i], &ms)
			switch ms.Operation {
			//case "keep-alive":
			//	c.start = time.Now()
			default:
				c.hub.broadcast <- messages[i]
			}

		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	//alive := time.NewTicker(time.Second * 5)
	defer func() {
		//alive.Stop()
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				log.Println("c.conn.WriteMessage(websocket.CloseMessage, []byte{})")
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				log.Println("err := w.Close()")
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			//case <-alive.C:
			//	if !c.start.Add(time.Minute).After(time.Now()) {
			//		log.Println("time out")
			//		return
			//	}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(roomId, userid string, hub *Hub, w http.ResponseWriter, r *http.Request) {
	cli := model.Pool.Get()
	defer cli.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{id: userid, start: time.Now(), hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.clients[client] = true
	owner, _ := cli.Do("GET", fmt.Sprintf("room:%s", roomId))
	client.hub.owner = owner.(string)
	roomMutexes[hub.roomId].Unlock()

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
