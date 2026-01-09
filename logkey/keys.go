package logkey

const (
	Service   = "service"
	Component = "component"
	
	NamespaceID = "namespace.id"

	OperatorID        = "operator.id"
	OperatorPublicKey = "operator.public_key"

	AccountID        = "account.id"
	AccountPublicKey = "account.public_key"
	SystemAccountID  = "system_account.id"

	UserID       = "user.id"
	SystemUserID = "system_user.id"

	StreamName             = "stream.name"
	ConsumerStartSequence  = "consumer.start_sequence"
	ConsumerFetchBatchSize = "consumer.fetch_batch_size"

	BrokerMessageID             = "broker_message.id"
	BrokerMessageReplyInbox     = "broker_message.reply_inbox"
	BrokerMessageOperation      = "broker_message.operation"
	BrokerMessageRequestSubject = "broker_message.request.subject"
)

var HTTPKeys = []string{
	NamespaceID,

	OperatorID,
	OperatorPublicKey,

	AccountID,
	AccountPublicKey,
	SystemAccountID,

	UserID,
	SystemUserID,

	StreamName,
	ConsumerStartSequence,
	ConsumerFetchBatchSize,

	BrokerMessageID,
	BrokerMessageRequestSubject,
	BrokerMessageReplyInbox,
}
