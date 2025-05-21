package command

import (
	"errors"

	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

func getOperationType(msg *commandv1.PublishMessage) (string, error) {
	switch {
	case msg.GetRequest() != nil:
		return "OPERATION_REQUEST", nil
	case msg.GetListStream() != nil:
		return "OPERATION_LIST_STREAM", nil
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
