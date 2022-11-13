package main

import (
	"collaborative-whiteboard-server/model"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
)

type Hub struct {
	roomId       string
	owner        string
	boardNums    int
	OpMaxNum     int64  //房间已经进行的操作的最大时间戳，每有一个操作就加一，初始为0
	OpCurrentNum int64  //当前保存的全量帧的时间戳
	mode         string //房间模式，1代表协作模式，0代表只读模式，只有房主可以更改房间的模式，只读模式下每个人都不能编辑
	clients      map[*Client]bool
	broadcast    chan []byte
	unregister   chan *Client
}

func newHub(roomId string) *Hub {
	cli := model.Pool.Get()
	defer cli.Close()

	cli.Do("SETNX", fmt.Sprintf("room:%s:fullTimestamp", roomId), 0)
	cli.Do("SETNX", fmt.Sprintf("room:%s:maxTimestamp", roomId), 0)

	max, err := redis.Int64(cli.Do("GET", fmt.Sprintf("room:%s:maxTimestamp", roomId)))
	if err != nil {
		log.Println(err)
	}
	log.Println(fmt.Sprintf("room: %s , maxTimeStamp: %d", roomId, max))

	full, err := redis.Int64(cli.Do("GET", fmt.Sprintf("room:%s:fullTimestamp", roomId)))
	if err != nil {
		log.Println(err)
	}
	log.Println(fmt.Sprintf("room: %s , fullTimeStamp: %d", roomId, full))

	return &Hub{
		OpMaxNum:     max,
		OpCurrentNum: full,
		roomId:       roomId,
		broadcast:    make(chan []byte),
		unregister:   make(chan *Client),
		clients:      make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	defer func() {
		close(h.unregister)
		close(h.broadcast)
	}()
	for {
		select {
		case client := <-h.unregister:
			h.OnDisconnect(client)
		case message := <-h.broadcast:
			h.BroadcastToRoom(message)
		}
	}
}

func (h *Hub) OnDisconnect(client *Client) {
	cli := model.Pool.Get()
	defer cli.Close()

	roomMutex := roomMutexes[h.roomId]
	roomMutex.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		//房间无人时从房间列表删去该房间
		if len(h.clients) == 0 {
			house.Delete(h.roomId)
			roomMutex.Unlock()
			mutexForRoomMutexes.Lock()
			if roomMutex.TryLock() {
				if len(h.clients) == 0 {
					delete(roomMutexes, h.roomId)
				}
				roomMutex.Unlock()
			}
			mutexForRoomMutexes.Unlock()
			cli.Do("SET", fmt.Sprintf("room:%s:maxTimestamp", h.roomId), h.OpMaxNum)
			return
		}
	}
	roomMutex.Unlock()
}

func (h *Hub) BroadcastToRoom(message []byte) {
	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}
