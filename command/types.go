package command

import (
	"go.jetify.com/typeid"
)

type UserCreds struct {
	JWT  string `json:"jwt"`
	Seed string `json:"seed"`
}

type messagePrefix struct{}

func (messagePrefix) Prefix() string { return "msg" }

type MessageID struct {
	typeid.TypeID[messagePrefix]
}

func NewMessageID() MessageID {
	return typeid.Must(typeid.New[MessageID]())
}

type streamConsumerPrefix struct{}

func (streamConsumerPrefix) Prefix() string { return "cons" }

type StreamConsumerID struct {
	typeid.TypeID[streamConsumerPrefix]
}

func NewStreamConsumerID() StreamConsumerID {
	return typeid.Must(typeid.New[StreamConsumerID]())
}

type natsAccountMessageData struct {
	Code    int    `json:"code"`
	Account string `json:"account"`
	Message string `json:"message"`
}

type accountUpdateReplyMessage struct {
	Data natsAccountMessageData `json:"data"`
}
