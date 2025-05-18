package nscmd

import (
	"errors"

	brokerv1 "github.com/coro-sh/coro/proto/gen/broker/v1"
)

func getOperationType(msg *brokerv1.PublishMessage) (string, error) {
	switch {
	case msg.GetRequest() != nil:
		return "OPERATION_REQUEST", nil
	case msg.GetStartConsumer() != nil:
		return "OPERATION_START_CONSUMER", nil
	case msg.GetStopConsumer() != nil:
		return "OPERATION_STOP_CONSUMER", nil
	case msg.GetSendConsumerHeartbeat() != nil:
		return "OPERATION_SEND_CONSUMER_HEARTBEAT", nil
	default:
		return "", errors.New("message does not contain an operation")
	}
}
