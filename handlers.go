package main

import (
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

func test(c *gin.Context) {
	room := c.Param("room")
	c.JSON(200, gin.H{
		"room": room,
	})
}

func getId(c *gin.Context) {
	uid := uuid.NewV4().String()
	c.JSON(200, gin.H{
		"userId": uid,
	})
}

func getRoom(c *gin.Context) {
	rand.Seed(time.Now().Unix())
	rid := ""
	for i := 0; i < 10; i++ {
		rid += strconv.Itoa(rand.Intn(10))
	}
	c.JSON(200, gin.H{
		"roomId": rid,
	})
}

func roomHandler(c *gin.Context) {
	roomId := c.Param("roomId")
	userId := c.Param("userId")
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
	serveWs(userId, hub, c.Writer, c.Request)
}
