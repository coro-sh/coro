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
		return "OPERATION_LIST_STREAMS", nil
	case msg.GetGetStream() != nil:
		return "OPERATION_GET_STREAM", nil
	case msg.GetFetchStreamMessages() != nil:
		return "OPERATION_FETCH_STREAM_MESSAGES", nil
	case msg.GetGetStreamMessageContent() != nil:
		return "OPERATION_GET_STREAM_MESSAGE_CONTENT", nil
	case msg.GetStartStreamConsumer() != nil:
		return "OPERATION_START_STREAM_CONSUMER", nil
	case msg.GetStopStreamConsumer() != nil:
		return "OPERATION_STOP_STREAM_CONSUMER", nil
	case msg.GetSendStreamConsumerHeartbeat() != nil:
		return "OPERATION_SEND_STREAM_CONSUMER_HEARTBEAT", nil
	default:
		return "", errors.New("message does not contain an operation")
	}
}
