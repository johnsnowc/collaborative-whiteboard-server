package model

import jsoniter "github.com/json-iterator/go"

type Message struct {
	RoomId string `json:"roomId"`
	//BoardId   string              `json:"boardId"`
	UserId    string              `json:"userId"`
	Operation string              `json:"operation"`
	Data      jsoniter.RawMessage `json:"data"`
}
