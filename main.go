package main

import (
	"collaborative-whiteboard-server/middleware"
	"collaborative-whiteboard-server/model"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

var house sync.Map
var roomMutexes = make(map[string]*sync.Mutex)
var mutexForRoomMutexes = new(sync.Mutex)

func GinMiddleware(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for i := 0; i < len(allowOrigins); i++ {
			c.Writer.Header().Add("Access-Control-Allow-Origin", allowOrigins[i])
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding, Accept-Language, X-Requested-With")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Request.Header.Del("Origin")

		c.Next()
	}
}

func init() {
	// 禁用控制台颜色，将日志写入文件时不需要控制台颜色。
	gin.DisableConsoleColor()

	// 记录到文件。
	f, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

	file := "./" + "log" + ".txt"
	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile) // 将文件设置为log输出的文件
	log.SetPrefix("[whiteboard-server]")
	log.SetFlags(log.LstdFlags | log.Llongfile | log.Ldate | log.Ltime)
}

func main() {
	model.Pool = model.PoolInitRedis("127.0.0.1:6379", "")

	r := gin.Default()
	r.Use(GinMiddleware([]string{"*"}))
	r.GET("/test/:room", test)

	r.POST("/register", Register)
	r.POST("/login", Login)
	r.GET("/ws/room/:roomId/user/:userId", RoomHandler)
	auth := r.Group("/auth")
	auth.Use(middleware.Auth())
	{
		auth.GET("/getUserInfo", GetUserInfo)
		auth.POST("/room", CreateRoom)
		auth.GET("/rooms", GetRooms)
		auth.GET("/room/:roomId", IsRoomExist)
		auth.GET("/room/:roomId/isOwner", IsOwner)
		auth.DELETE("/room/:roomId", DeleteRoom)
		//auth.GET("/ws/room/:roomId/", RoomHandler)
	}

	r.Run(":8000")
}
