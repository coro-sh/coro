package embedns

import (
	"errors"
	"fmt"

	"github.com/coro-sh/coro/entity"
)

const inMemResolverTemplate = `# Operator
operator: %s

# System Account
system_account: %s

# Configuration of the nats based resolver
resolver: MEMORY

# Preload the nats based resolver with the system account jwt.
# This only applies to the system account. Therefore other account jwt are not included here.
resolver_preload: {
    # Later changes to the system account take precedence over the system account jwt listed below.
	%s: %s
}
`

func newInMemResolverConfig(op *entity.Operator, sysAcc *entity.Account) (string, error) {
	ok, err := sysAcc.IsSystemAccount()
	if err != nil {
		return "", fmt.Errorf("check system account: %w", err)
	} else if !ok {
		return "", errors.New("resolver system account is not a system account")
	}

	sysAccData, err := sysAcc.Data()
	if err != nil {
		return "", err
	}

	cfgContent := fmt.Sprintf(
		inMemResolverTemplate,
		op.JWT, sysAccData.PublicKey, sysAccData.PublicKey, sysAcc.JWT,
	)

	return cfgContent, nil
}
