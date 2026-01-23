package command

import (
	commandv1 "github.com/coro-sh/coro/proto/gen/command/v1"
)

func getOperationType(msg *commandv1.PublishMessage) string {
	switch {
	case msg.GetRequest() != nil:
		return "OPERATION_REQUEST"
	case msg.GetListStream() != nil:
		return "OPERATION_LIST_STREAMS"
	case msg.GetGetStream() != nil:
		return "OPERATION_GET_STREAM"
	case msg.GetFetchStreamMessages() != nil:
		return "OPERATION_FETCH_STREAM_MESSAGES"
	case msg.GetGetStreamMessageContent() != nil:
		return "OPERATION_GET_STREAM_MESSAGE_CONTENT"
	case msg.GetStartStreamConsumer() != nil:
		return "OPERATION_START_STREAM_CONSUMER"
	case msg.GetStopStreamConsumer() != nil:
		return "OPERATION_STOP_STREAM_CONSUMER"
	case msg.GetSendStreamConsumerHeartbeat() != nil:
		return "OPERATION_SEND_STREAM_CONSUMER_HEARTBEAT"
	default:
		return ""
	}
}
