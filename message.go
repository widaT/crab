package crab

import "encoding/json"

type MessageType uint8

const (
	TJoin MessageType = iota
)

type Message struct {
	Type    MessageType `json:"type"`
	Payload string      `json:"payload"`
	Channel string      `json:"channel"`
}

func (m *Message) ToJSON() []byte {
	b, _ := json.Marshal(m)
	return b
}
