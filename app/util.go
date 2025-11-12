package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	natserver "github.com/nats-io/nats-server/v2/server"
	"go.jetify.com/typeid"

	"github.com/coro-sh/coro/constants"
	"github.com/coro-sh/coro/embedns"
	"github.com/coro-sh/coro/encrypt"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/errtag"
	"github.com/coro-sh/coro/log"
	"github.com/coro-sh/coro/server"
)

func NewEntityStore(repo entity.Repository, encKey *string) (*entity.Store, error) {
	var storeOpts []entity.StoreOption
	if encKey != nil {
		enc, err := encrypt.NewAES(*encKey)
		if err != nil {
			return nil, fmt.Errorf("create aes encrypter: %w", err)
		}
		storeOpts = append(storeOpts, entity.WithEncryption(enc))
	}
	return entity.NewStore(repo, storeOpts...), nil
}

func Serve(ctx context.Context, srv *server.Server, logger log.Logger) error {
	errs := make(chan error)

	logger.Info("starting server", "address", srv.Address())
	go func() {
		defer close(errs)
		if err := srv.Start(); err != nil {
			errs <- fmt.Errorf("start server: %w", err)
		}
	}()
	defer srv.Stop(ctx) //nolint:errcheck

	logger.Info("waiting for server to be healthy")
	if err := srv.WaitHealthy(15, time.Second); err != nil {
		return err
	}
	logger.Info("server healthy")

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		logger.Info("server stopped")
		return nil
	}
}

func InitNamespace(ctx context.Context, store *entity.Store, logger log.Logger, name string, owner string) (*entity.Namespace, error) {
	ns, err := store.ReadNamespaceByName(ctx, name, owner)
	if err != nil {
		if errtag.HasTag[errtag.NotFound](err) {
			ns = entity.NewNamespace(name, owner)
			if err = store.CreateNamespace(ctx, ns); err != nil {
				if !errtag.HasTag[errtag.Conflict](err) {
					return nil, err
				}
				// Another server instance created Namespace first, so we can read the Namespace again
				ns, err = store.ReadNamespaceByName(ctx, name, owner)
				if err != nil {
					return nil, fmt.Errorf("read existing internal namespace: %w", err)
				}
			}
			logger.Info("created new namespace", "namespace.id", ns.ID, "namespace.name", name)
			return ns, nil
		}
		return nil, err
	}
	return ns, nil
}

func InitBrokerNATSEntities(
	ctx context.Context,
	store *entity.Store,
	logger log.Logger,
	internalNamespaceID entity.NamespaceID,
) (*entity.Operator, *entity.Account, *entity.User, error) {
	createEntities := func() (op *entity.Operator, sysAcc *entity.Account, sysUser *entity.User, err error) {
		op, err = entity.NewOperator(constants.BrokerOperatorName, internalNamespaceID)
		if err != nil {
			return nil, nil, nil, err
		}

		sysAcc, sysUser, err = op.SetNewSystemAccountAndUser()
		if err != nil {
			return nil, nil, nil, err
		}

		err = store.BeginTxFunc(ctx, func(ctx context.Context, store *entity.Store) error {
			if err = store.CreateOperator(ctx, op); err != nil {
				return fmt.Errorf("create broker internal operator: %w", err)
			}
			logger.Info("created new broker internal operator", "operator.id", op.ID, "operator.name", constants.BrokerOperatorName)

			if err = store.CreateAccount(ctx, sysAcc); err != nil {
				return fmt.Errorf("create broker internal system account: %w", err)
			}
			logger.Info("created new broker internal system account", "account.id", sysAcc.ID, "operator.name", constants.SysAccountName)

			if err = store.CreateUser(ctx, sysUser); err != nil {
				return fmt.Errorf("create broker internal system user: %w", err)
			}
			logger.Info("created new broker internal system user", "user.id", sysUser.ID, "user.name", constants.SysUserName)

			return nil
		})
		if err != nil {
			return nil, nil, nil, err
		}

		return op, sysAcc, sysUser, nil
	}

	op, err := store.ReadOperatorByName(ctx, constants.BrokerOperatorName)
	if errtag.HasTag[errtag.NotFound](err) {
		createOp, createdSysAcc, createdSysUser, err := createEntities()
		if err == nil {
			return createOp, createdSysAcc, createdSysUser, nil
		}
		if !errtag.HasTag[errtag.Conflict](err) {
			return nil, nil, nil, err
		}
		// Another server instance created entities first, so we can read the Operator again
		op, err = store.ReadOperatorByName(ctx, constants.BrokerOperatorName)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read broker existing internal operator: %w", err)
		}
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read broker internal operator: %w", err)
	}

	opClaims, err := op.Claims()
	if err != nil {
		return nil, nil, nil, err
	}

	sysAcc, err := store.ReadAccountByPublicKey(ctx, opClaims.SystemAccount)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read broker internal sys account: %w", err)
	}

	sysUser, err := store.ReadSystemUser(ctx, op.ID, sysAcc.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read broker internal sys user: %w", err)
	}

	return op, sysAcc, sysUser, nil
}

func StartEmbeddedNATS(logger log.Logger, op *entity.Operator, sysAcc *entity.Account, cfg *EmbeddedNATSConfig, tlsConfig *TLSConfig) (*natserver.Server, error) {
	nodeID, err := typeid.WithPrefix("node")
	if err != nil {
		return nil, err
	}

	natsCfg := embedns.EmbeddedNATSConfig{
		Resolver: embedns.ResolverConfig{
			Operator:      op,
			SystemAccount: sysAcc,
		},
		NodeName: nodeID.String(),
	}

	if cfg != nil {
		natsCfg.Cluster = &embedns.ClusterConfig{
			ClusterName:     "broker_cluster",
			ClusterHostPort: cfg.HostPort,
			Routes:          natserver.RoutesFromStr(strings.Join(cfg.NodeRoutes, ",")),
		}
	}

	if tlsConfig != nil {
		natsCfg.TLS = &embedns.TLSConfig{
			CertFile:   tlsConfig.CertFile,
			KeyFile:    tlsConfig.KeyFile,
			CACertFile: tlsConfig.CACertFile,
		}
	}

	ns, err := embedns.NewEmbeddedNATS(natsCfg)
	if err != nil {
		return nil, fmt.Errorf("create embedded nats server: %w", err)
	}
	ns.Start()

	logger.Info("waiting for embedded nats server to start")
	if !ns.ReadyForConnections(10 * time.Second) {
		return nil, fmt.Errorf("embedded nats server not healthy")
	}
	logger.Info("embedded nats server is ready for connections on " + ns.Addr().String())

	return ns, nil
}
