package notif

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nats-io/jwt/v2"

	"github.com/coro-sh/coro/entity"
)

const accClaimsUpdateSubjectFormat = "$SYS.REQ.ACCOUNT.%s.CLAIMS.UPDATE"

type Publisher interface {
	Publish(ctx context.Context, operatorID entity.OperatorID, subject string, data []byte) ([]byte, error)
	Ping(ctx context.Context, operatorID entity.OperatorID) (entity.OperatorNATSStatus, error)
}

type Notifier struct {
	store *entity.Store
	pub   Publisher
}

func NewNotifier(store *entity.Store, pub Publisher) (*Notifier, error) {
	return &Notifier{
		store: store,
		pub:   pub,
	}, nil
}

// NotifyAccountClaimsUpdate sends a notification about an account claims update.
func (n *Notifier) NotifyAccountClaimsUpdate(ctx context.Context, account *entity.Account) error {
	claims, err := account.Claims()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf(accClaimsUpdateSubjectFormat, claims.Subject)
	reply, err := n.pub.Publish(ctx, account.OperatorID, subject, []byte(account.JWT))
	if err != nil {
		return err
	}

	return validateAccountReply(reply, claims, "jwt updated")
}

// Ping sends a ping to the Broker and waits for a pong if there is an
// active WebSocket connection for the Operator. Boolean value true is returned
// if a pong is received, false if no Operator WebSocket exists, or an error
// if no response is received within context's timeout.
func (n *Notifier) Ping(ctx context.Context, operator *entity.Operator) (entity.OperatorNATSStatus, error) {
	return n.pub.Ping(ctx, operator.ID)
}

func validateAccountReply(reply []byte, claims *jwt.AccountClaims, wantMsg string) error {
	if len(reply) == 0 {
		return errors.New("empty account reply")
	}

	var replyMsg AccountUpdateReplyMessage
	if err := json.Unmarshal(reply, &replyMsg); err != nil {
		return fmt.Errorf("unmarshal account reply message: %w", err)
	}

	data := replyMsg.Data

	switch {
	case data.Code < 200 && data.Code > 299:
		return fmt.Errorf("non 200 status code: %d", data.Code)
	case claims.Subject != data.Account:
		return errors.New("claims subject mismatch")
	case data.Message != wantMsg:
		return fmt.Errorf("expected reply '%s', got '%s'", wantMsg, data.Message)
	}

	return nil
}
