package entity

import (
	"errors"
	"fmt"
)

const DefaultResolverDir = "./data"

const dirResolverConfigTemplate = `# Operator named %s
operator: %s

# System Account named %s
system_account: %s

jetstream {
  store_dir: "%s/js"
  max_mem: 0
  max_file: 10GB
}

# Configuration of the nats based resolver
resolver {
    type: full
    # Directory in which the account jwt will be stored
    dir: '%s/jwt'
    # In order to support jwt deletion, set to true
    # If the resolver type is full delete will rename the jwt.
    # This is to allow manual restoration in case of inadvertent deletion.
    # To restore a jwt, remove the added suffix .delete and restart or send a reload signal.
    # To free up storage you must manually delete files with the suffix .delete.
    allow_delete: false
    # Interval at which a nats-server with a nats based account resolver will compare
    # it's state with one random nats based account resolver in the cluster and if needed, 
    # exchange jwt and converge on the same set of jwt.
    interval: "2m"
    # Timeout for lookup requests in case an account does not exist locally.
    timeout: "1.9s"
}

# Preload the nats based resolver with the system account jwt.
# This only applies to the system account. Therefore other account jwt are not included here.
#
# To populate the resolver with additional accounts or account updates:
#
#     Coro notifications:
#        1) Establish a connection between Coro and your nats server using the Coro Proxy Agent.
#        2) Account changes made via Coro will automatically be sent to your nats server.
#
#     NSC tool:
#        1) Import the operator of your nats cluster into nsc.
#        3) make sure that your operator has the account server URL pointing at your nats servers.
#           The url must start with: "nats://"
#           nsc edit operator --account-jwt-server-url nats://localhost:4222
#        3) push your accounts using: nsc push --all
#           The argument to push -u is optional if your account server url is set as described.
#        3) to prune accounts use: nsc push --prune
#           In order to enable prune you must set above allow_delete to true
resolver_preload: {
    # Later changes to the system account take precedence over the system account jwt listed below.
	%s: %s
}
`

// NewDirResolverConfig creates a configuration for a directory resolver.
// The returned string can be written to a file and used to start a NATS server.
func NewDirResolverConfig(op *Operator, sysAcc *Account, dirpath string) (string, error) {
	opData, err := op.Data()
	if err != nil {
		return "", err
	}

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
		dirResolverConfigTemplate,
		opData.Name, op.JWT, sysAccData.Name, sysAccData.PublicKey, dirpath, dirpath, sysAccData.PublicKey, sysAcc.JWT,
	)

	return cfgContent, nil
}
