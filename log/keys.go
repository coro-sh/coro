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

	KeyBrokerMessageID         = "broker_message.id"
	KeyBrokerMessageSubject    = "broker_message.subject"
	KeyBrokerMessageReplyInbox = "broker_message.reply_inbox"
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

	KeyBrokerMessageID,
	KeyBrokerMessageSubject,
	KeyBrokerMessageReplyInbox,
}
