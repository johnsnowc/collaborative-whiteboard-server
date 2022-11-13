package main

import (
	"collaborative-whiteboard-server/middleware"
	"collaborative-whiteboard-server/model"
	"fmt"
	jwtgo "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type HttpMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func test(c *gin.Context) {
	room := c.Param("room")
	c.JSON(200, HttpMessage{
		Code:    "0",
		Message: "",
		Data:    gin.H{"room": room},
	})
}

func CreateRoom(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	username, _ := c.Get("username")
	rand.Seed(time.Now().Unix())
	rid := ""
	for i := 0; i < 10; i++ {
		rid += strconv.Itoa(rand.Intn(10))
	}
	cli.Do("SET", fmt.Sprintf("room:%s", rid), username)
	//cli.Do("SET", fmt.Sprintf("room:%s:mode", rid), "1")
	cli.Do("SADD", fmt.Sprintf("user:%s:rooms", username), rid)

	c.JSON(200, HttpMessage{
		Code:    "0",
		Message: "Create Room Success!",
		Data:    gin.H{"roomId": rid},
	})

}

func GetRooms(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	userId, _ := c.Get("username")
	rooms, _ := redis.Strings(cli.Do("SMEMBERS", fmt.Sprintf("user:%s:rooms", userId)))
	c.JSON(http.StatusOK, HttpMessage{
		Code:    "0",
		Message: "Get Rooms Success",
		Data: gin.H{
			"rooms": rooms,
		},
	})

}

func RoomHandler(c *gin.Context) {
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
	log.Println(room, ok)
	var hub *Hub
	if ok {
		hub = room.(*Hub)
	} else {
		hub = newHub(roomId)
		house.Store(roomId, hub)
		go hub.run()
	}
	serveWs(roomId, userId, hub, c)
}

func GetUserInfo(c *gin.Context) {
	username, _ := c.Get("username")

	c.JSON(http.StatusOK, HttpMessage{
		Code:    "0",
		Message: "Valid request",
		Data:    gin.H{"username": username},
	})
}

func Register(c *gin.Context) {
	var loginReq model.LoginRequestBody
	if c.BindJSON(&loginReq) == nil {
		isPass, err := model.Register(loginReq)
		if isPass {
			c.JSON(http.StatusOK, HttpMessage{
				Code:    "0",
				Message: "Register success",
				Data:    nil,
			})
		} else {
			c.JSON(http.StatusOK, HttpMessage{
				Code:    "-1",
				Message: "Register fails," + err.Error(),
				Data:    nil,
			})
		}
	} else {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "JSON parsing failed",
			Data:    nil,
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
			c.JSON(http.StatusOK, HttpMessage{
				Code:    "-1",
				Message: "Validation Failed," + err.Error(),
				Data:    nil,
			})
		}
	} else {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "JSON Parsing Failed!",
			Data:    nil,
		})
	}
}

func IsRoomExist(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	roomId := c.Param("roomId")
	res, err := cli.Do("EXISTS", fmt.Sprintf("room:%s", roomId))
	if err != nil {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "Search db Failed," + err.Error(),
			Data:    nil,
		})
		return
	}
	c.JSON(http.StatusOK, HttpMessage{
		Code:    "0",
		Message: "Search db Success",
		Data: gin.H{
			"exist": res,
		},
	})
}

func IsOwner(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	roomId := c.Param("roomId")
	currentUser, _ := c.Get("username")
	user, _ := redis.String(cli.Do("GET", fmt.Sprintf("room:%s", roomId)))
	if currentUser != user {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "This User is not Owner",
			Data: gin.H{
				"isOwner": false,
			},
		})
	} else {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "0",
			Message: "This User is Owner",
			Data: gin.H{
				"isOwner": true,
			},
		})
	}
}

func DeleteRoom(c *gin.Context) {
	cli := model.Pool.Get()
	defer cli.Close()

	currentUser, _ := c.Get("username")
	roomId := c.Param("roomId")
	res, err := redis.Int64(cli.Do("EXISTS", fmt.Sprintf("room:%s", roomId)))
	if err != nil {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "Search db Failed," + err.Error(),
			Data:    nil,
		})
		return
	}
	if res == 0 {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "Room not Exist",
			Data:    nil,
		})
		return
	}
	owner, _ := redis.String(cli.Do("GET", fmt.Sprintf("room:%s", roomId)))
	log.Println("current user:", currentUser)
	log.Println("room owner:", owner)
	if currentUser == owner {
		cli.Do("DEL", fmt.Sprintf("room:%s", roomId))
		cli.Do("SREM", fmt.Sprintf("user:%s:rooms", owner), roomId)
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "0",
			Message: "Delete Success",
			Data:    nil,
		})
	} else {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "Delete Failed. You Have No Access To Delete Room Which Not Belong To You ",
			Data:    nil,
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
			NotBefore: time.Now().Unix() - 1000,  // Effective time of signature
			ExpiresAt: time.Now().Unix() + 36000, // Expiration time is ten hour
			Issuer:    "johnsnowc",               // Signed issuer
		},
	}
	token, err := jwt.CreateToken(claims)
	if err != nil {
		c.JSON(http.StatusOK, HttpMessage{
			Code:    "-1",
			Message: "Generate Token Failed",
			Data:    gin.H{"error": err.Error()},
		})
		return
	}
	rooms, _ := redis.Strings(cli.Do("SMEMBERS", fmt.Sprintf("user:%s:rooms", user.Username)))
	id, _ := redis.String(cli.Do("Get", fmt.Sprintf("user:%s:id", user.Username)))
	c.JSON(http.StatusOK, HttpMessage{
		Code:    "0",
		Message: "Login Successful!",
		Data: gin.H{
			"id":    id,
			"token": token,
			"rooms": rooms,
		},
	})
	return
}
