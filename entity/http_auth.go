package entity

import (
	"github.com/labstack/echo/v4"

	"github.com/joshjon/kit/errtag"

	"github.com/coro-sh/coro/constants"
)

type NamespaceIDGetter interface {
	GetNamespaceID() NamespaceID
}

func VerifyEntityNamespace(c echo.Context, entity NamespaceIDGetter) error {
	nsID, err := NamespaceIDFromContext(c)
	if err != nil {
		return err
	}
	if nsID != entity.GetNamespaceID() {
		return errtag.NewTagged[errtag.Unauthorized]("entity namespace id mismatch")
	}
	return nil
}

func VerifyPublicAccount(acc *Account) error {
	accData, err := acc.Data()
	if err != nil {
		return err
	}
	if constants.IsReservedAccountName(accData.Name) {
		return errtag.NewTagged[errtag.Unauthorized]("account is reserved for internal use only")
	}

	return nil
}

func VerifyPublicUser(usr *User) error {
	usrData, err := usr.Data()
	if err != nil {
		return err
	}
	if constants.IsReservedUserName(usrData.Name) {
		return errtag.NewTagged[errtag.Unauthorized]("user is reserved for internal use only")
	}
	return nil
}
