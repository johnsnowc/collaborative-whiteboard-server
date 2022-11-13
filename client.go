package main

import (
	"bytes"
	"collaborative-whiteboard-server/model"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	jsoniter "github.com/json-iterator/go"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 5 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 30 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1000000
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1000000,
	WriteBufferSize: 1000000,
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
				//log.Printf("error: %v", err)
			}
			//log.Println("err", err)
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
			case "export":
				go export(c.hub.roomId, ms)
			//case "keep-alive":
			//	c.start = time.Now()
			case "join":
				c.hub.broadcast <- messages[i]
			case "leave":
				c.hub.broadcast <- messages[i]
			default:
				newMaxNum := atomic.AddInt64(&c.hub.OpMaxNum, 1)
				if newMaxNum > 0 && newMaxNum%100 == 0 {
					exportMessage := model.Message{
						RoomId:    c.hub.roomId,
						UserId:    c.id,
						Operation: "export",
						Data:      nil,
					}
					message1, _ := jsoniter.Marshal(exportMessage)
					c.writeMessage(message1)
				}
				go push(c.hub.roomId, messages[i])
				c.hub.broadcast <- messages[i]
			}

		}
	}
}

func export(roomId string, message model.Message) {
	cli := model.Pool.Get()
	defer cli.Close()

	cli.Do("SET", fmt.Sprintf("room:%s:full", roomId), message.Data)
	cli.Do("INCRBY", fmt.Sprintf("room:%s:fullTimestamp", roomId), 100)
}

func push(roomId string, message []byte) {
	cli := model.Pool.Get()
	defer cli.Close()

	cli.Do("INCR", fmt.Sprintf("room:%s:maxTimestamp", roomId))
	cli.Do("RPUSH", fmt.Sprintf("room:%s:ops", roomId), message)
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
				//log.Println("c.conn.WriteMessage(websocket.CloseMessage, []byte{})")
				return
			}
			c.writeMessage(message)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("err:", err)
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

func (c *Client) writeMessage(message []byte) {
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		log.Println("err:", err)
		return
	}
	nums, err := w.Write(message)
	if err != nil {
		log.Println(err)
	} else {
		log.Println(nums)
	}

	// Add queued chat messages to the current websocket message.
	//n := len(c.send)
	//for i := 0; i < n; i++ {
	//	w.Write(newline)
	//	w.Write(<-c.send)
	//}

	if err := w.Close(); err != nil {
		log.Println("err := w.Close()")
		return
	}
}

func (c *Client) reproduce(roomId string) {
	cli := model.Pool.Get()
	defer cli.Close()

	fullFrame, _ := redis.Bytes(cli.Do("GET", fmt.Sprintf("room:%s:full", roomId)))

	if fullFrame == nil {
		log.Println("no full frame yet")
	} else {
		log.Println("full frame:", fullFrame)
		message := model.Message{
			RoomId:    c.hub.roomId,
			UserId:    c.id,
			Operation: "full-frame",
			Data:      fullFrame,
		}
		message1, _ := jsoniter.Marshal(message)
		c.writeMessage(message1)
	}

	incrFrames, err := redis.ByteSlices(cli.Do("LRANGE", fmt.Sprintf("room:%s:ops", roomId), c.hub.OpCurrentNum, c.hub.OpMaxNum))
	log.Println(c.hub.OpCurrentNum, c.hub.OpMaxNum)
	if err != nil {
		log.Println("get incre frame err:", err)
	} else {
		log.Println("incr frame nums:", len(incrFrames))
		for i := 0; i < len(incrFrames); i++ {
			c.writeMessage(incrFrames[i])
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(roomId, userid string, hub *Hub, c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	var wg sync.WaitGroup
	wg.Add(1)

	//cli.Do("SETNX", fmt.Sprintf("room:%s:mode", roomId), "1")
	client := &Client{id: userid, start: time.Now(), hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.clients[client] = true
	failed := false
	go func() {
		cli := model.Pool.Get()
		defer cli.Close()

		owner, _ := redis.String(cli.Do("GET", fmt.Sprintf("room:%s", roomId)))

		if owner == "" {
			failed = true
			c.JSON(500, HttpMessage{
				Code:    "-1",
				Message: "This Room is not Created!",
				Data:    nil,
			})
			return
		}
		log.Println(fmt.Sprintf("room %s owner %s", roomId, owner))
		client.hub.owner = owner
		wg.Done()
	}()
	wg.Wait()
	if failed == true {
		roomMutexes[hub.roomId].Unlock()
		return
	}
	roomMutexes[hub.roomId].Unlock()

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.reproduce(roomId)
	go client.writePump()
	go client.readPump()
}
