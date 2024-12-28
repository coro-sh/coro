package broker

import (
	"time"

	"go.jetify.com/typeid"
)

type SysUserCreds struct {
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

// Message is sent by the proxy notifier and consumed by the
// proxy forwarder so that it can forward the message to the proxy. Once
// received by the proxy, it then forwards the data to the NATS server on the
// specified subject.
type Message[T any] struct {
	// ID is the identifier of the message used to trace the end-to-end journey.
	ID MessageID `json:"id"`
	// Inbox is the subject of the reply inbox if a response is expected.
	Inbox string `json:"inbox"`
	// Subject is the external subject on the downstream NATS server to forward
	// the message data to.
	Subject string `json:"subject"`
	// Data is the content of the message being forward.
	Data T `json:"data"`
}

type messageIDPayload struct {
	ID MessageID `json:"id"`
}

type brokerPingReplyMessage struct {
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
