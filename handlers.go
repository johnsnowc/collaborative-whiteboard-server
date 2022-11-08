package main

import (
	"collaborative-whiteboard-server/middleware"
	"collaborative-whiteboard-server/model"
	"fmt"
	jwtgo "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"math/rand"
	"net/http"
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

func CreateRoom(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	userId, _ := c.Get("username")
	rand.Seed(time.Now().Unix())
	rid := ""
	for i := 0; i < 10; i++ {
		rid += strconv.Itoa(rand.Intn(10))
	}
	cli.Do("SET", fmt.Sprintf("room:%s", rid), userId)
	cli.Do("SADD", fmt.Sprintf("user:%s:rooms", userId), rid)
	c.JSON(200, gin.H{
		"roomId": rid,
	})
}

func RoomHandler(c *gin.Context) {
	roomId := c.Param("roomId")
	userId, _ := c.Get("username")
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
	serveWs(roomId, userId.(string), hub, c.Writer, c.Request)
}

func GetUserInfo(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{
		"status":   0,
		"msg":      "Valid request",
		"data":     "test",
		"username": username,
	})
}

func Register(c *gin.Context) {
	var loginReq model.LoginRequestBody
	if c.BindJSON(&loginReq) == nil {
		isPass, err := model.Register(loginReq)
		if isPass {
			c.JSON(http.StatusOK, gin.H{
				"status": -1,
				"msg":    "Register success",
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status": -1,
				"msg":    "Register fails," + err.Error(),
			})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": -1,
			"msg":    "JSON parsing failed",
		})
	}
}

func Login(c *gin.Context) {
	var loginReq model.LoginRequestBody
	if c.BindJSON(&loginReq) == nil {
		isPass, err := model.Login(loginReq)
		if isPass {
			generateToken(c, model.User{Username: loginReq.Username, Password: loginReq.Password})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status": -1,
				"msg":    "Validation fails," + err.Error(),
			})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": -1,
			"msg":    "JSON parsing failed",
		})
	}
}

func generateToken(c *gin.Context, user model.User) {
	cli := model.Pool.Get()
	defer cli.Close()

	jwt := &middleware.JWT{
		[]byte("johnsnowc"),
	}
	claims := middleware.CustomClaims{
		user.Username,
		jwtgo.StandardClaims{
			NotBefore: time.Now().Unix() - 1000, // Effective time of signature
			ExpiresAt: time.Now().Unix() + 3600, // Expiration time is one hour
			Issuer:    "johnsnowc",              // Signed issuer
		},
	}
	token, err := jwt.CreateToken(claims)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status": -1,
			"msg":    err.Error(),
		})
		return
	}
	rooms, _ := cli.Do("SMEMBERS", fmt.Sprintf("user:%s:rooms", user.Username))
	c.JSON(http.StatusOK, gin.H{
		"status": 0,
		"msg":    "Login successful！",
		"token":  token,
		"rooms":  rooms.([]interface{}),
	})
	return
}
