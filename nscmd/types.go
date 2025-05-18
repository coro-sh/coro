package nscmd

import (
	"time"

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

type messageIDPayload struct {
	ID string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
}

type natsAccountMessageData struct {
	Code    int    `json:"code"`
	Account string `json:"account"`
	Message string `json:"message"`
}

type accountUpdateReplyMessage struct {
	Data natsAccountMessageData `json:"data"`
}

type pingReplyMessage struct {
	Server struct {
		Name      string    `json:"name"`
		Host      string    `json:"host"`
		ID        string    `json:"id"`
		Cluster   string    `json:"cluster"`
		Ver       string    `json:"ver"`
		Jetstream bool      `json:"jetstream"`
		Flags     int       `json:"flags"`
		Seq       int       `json:"seq"`
		Time      time.Time `json:"time"`
	} `json:"server"`
	Statsz struct {
		Start            time.Time `json:"start"`
		Mem              int       `json:"mem"`
		Cores            int       `json:"cores"`
		CPU              float32   `json:"cpu"`
		Connections      int       `json:"connections"`
		TotalConnections int       `json:"total_connections"`
		ActiveAccounts   int       `json:"active_accounts"`
		Subscriptions    int       `json:"subscriptions"`
		Sent             struct {
			Msgs  int `json:"msgs"`
			Bytes int `json:"bytes"`
		} `json:"sent"`
		Received struct {
			Msgs  int `json:"msgs"`
			Bytes int `json:"bytes"`
		} `json:"received"`
		SlowConsumers int `json:"slow_consumers"`
		Routes        []struct {
			Rid  int    `json:"rid"`
			Name string `json:"name"`
			Sent struct {
				Msgs  int `json:"msgs"`
				Bytes int `json:"bytes"`
			} `json:"sent"`
			Received struct {
				Msgs  int `json:"msgs"`
				Bytes int `json:"bytes"`
			} `json:"received"`
			Pending int `json:"pending"`
		} `json:"routes"`
		ActiveServers int `json:"active_servers"`
	} `json:"statsz"`
}
