package model

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
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
	password, err := cli.Do("Get", fmt.Sprintf("user:%s", loginRequestBody.Username))
	log.Println(string(password.([]byte)))
	if string(password.([]byte)) != loginRequestBody.Password {
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
	return true, nil
}
