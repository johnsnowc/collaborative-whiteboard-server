package model

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	uuid "github.com/satori/go.uuid"
	"log"
)

// User model
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequestBody
type LoginRequestBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(loginRequestBody LoginRequestBody) (bool, error) {
	cli := Pool.Get()
	defer cli.Close()

	isKeyExit, err := redis.Bool(cli.Do("EXISTS", fmt.Sprintf("user:%s", loginRequestBody.Username)))
	if !isKeyExit {
		err = fmt.Errorf("The user does not exist ")
		return false, err
	}
	password, err := redis.String(cli.Do("Get", fmt.Sprintf("user:%s", loginRequestBody.Username)))
	log.Println(password)
	if password != loginRequestBody.Password {
		err = fmt.Errorf("Password mistake ")
		return false, err
	}
	return true, nil
}

func Register(loginRequestBody LoginRequestBody) (bool, error) {
	cli := Pool.Get()
	defer cli.Close()

	isKeyExit, err := redis.Bool(cli.Do("EXISTS", fmt.Sprintf("user:%s", loginRequestBody.Username)))
	if isKeyExit {
		err = fmt.Errorf("The user exist ")
		return false, err
	}
	_, err = cli.Do("Set", fmt.Sprintf("user:%s", loginRequestBody.Username), loginRequestBody.Password)
	if err != nil {
		return false, err
	}
	id := uuid.NewV4()
	cli.Do("Set", fmt.Sprintf("user:%s:id", loginRequestBody.Username), id)
	cli.Do("Set", fmt.Sprintf("id:%s:user", id), loginRequestBody.Username)
	return true, nil
}
