package main

import (
	"github.com/gin-gonic/gin"
	"sync"
)

func test(c *gin.Context) {
	room := c.Param("room")
	c.JSON(200, gin.H{
		"room": room,
	})
}

func room(c *gin.Context) {
	roomId := c.Param("room")
	mutexForRoomMutexes.Lock()
	roomMutex, ok := roomMutexes[roomId]
	if ok {
		roomMutex.Lock()
	} else {
		roomMutexes[roomId] = new(sync.Mutex)
		roomMutexes[roomId].Lock()
	}
	mutexForRoomMutexes.Unlock()
	room, ok := house.Load(roomId)
	var hub *Hub
	if ok {
		hub = room.(*Hub)
	} else {
		hub = newHub(roomId)
		house.Store(roomId, hub)
		go hub.run()
	}
	serveWs(hub, c.Writer, c.Request)
}
