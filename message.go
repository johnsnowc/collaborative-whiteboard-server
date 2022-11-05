package main

import jsoniter "github.com/json-iterator/go"

type Message struct {
	UserId    string              `json:"userId"`
	Operation string              `json:"operation"`
	Data      jsoniter.RawMessage `json:"data"`
}
