package log

const (
	KeyService   = "service"
	KeyComponent = "component"

	KeyNamespaceID = "namespace.id"

	KeyOperatorID        = "operator.id"
	KeyOperatorPublicKey = "operator.public_key"

	KeyAccountID        = "account.id"
	KeyAccountPublicKey = "account.public_key"
	KeySystemAccountID  = "system_account.id"

	KeyUserID       = "user.id"
	KeySystemUserID = "system_user.id"

	KeyStreamName             = "stream.name"
	KeyConsumerStartSequence  = "consumer.start_sequence"
	KeyConsumerFetchBatchSize = "consumer.fetch_batch_size"

	KeyBrokerMessageID             = "broker_message.id"
	KeyBrokerMessageReplyInbox     = "broker_message.reply_inbox"
	KeyBrokerMessageOperation      = "broker_message.operation"
	KeyBrokerMessageRequestSubject = "broker_message.request.subject"
)

var HTTPKeys = []string{
	KeyNamespaceID,

	KeyOperatorID,
	KeyOperatorPublicKey,

	KeyAccountID,
	KeyAccountPublicKey,
	KeySystemAccountID,

	KeyUserID,
	KeySystemUserID,

	KeyStreamName,
	KeyConsumerStartSequence,
	KeyConsumerFetchBatchSize,

	KeyBrokerMessageID,
	KeyBrokerMessageRequestSubject,
	KeyBrokerMessageReplyInbox,
}
