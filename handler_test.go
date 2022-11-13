package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"testing"
)

func TestName(t *testing.T) {
	cli, _ := redis.Dial("tcp", "101.133.131.188:6379")
	defer cli.Close()

	ops, _ := redis.Strings(cli.Do("LRANGE", fmt.Sprintf("room:%s:ops", "5300374513"), 0, 3))
	for i := 0; i < len(ops); i++ {
		fmt.Println(ops[i])
	}

}
