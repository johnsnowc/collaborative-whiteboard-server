package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"testing"
)

func TestName(t *testing.T) {
	cli, _ := redis.Dial("tcp", "127.0.0.1:6379")
	defer cli.Close()

	id, _ := cli.Do("GET", fmt.Sprintf("user:%s:id", "johnsnowc1"))
	fmt.Println(id)

	bytesBuffer := bytes.NewBuffer(id.([]byte))
	var full1 string
	binary.Read(bytesBuffer, binary.LittleEndian, &full1)
	fmt.Println(full1)
}
