package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"testing"
)

func TestName(t *testing.T) {
	cli, _ := redis.Dial("tcp", "127.0.0.1:6379")
	defer cli.Close()

	vals, e := cli.Do("SMEMBERS", "1234")
	fmt.Println(vals.([]interface{}))
	fmt.Println(e)
}
