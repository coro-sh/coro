package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cohesivestack/valgo"
	"github.com/nats-io/jwt/v2"
	natsrv "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/urfave/cli/v2"

	"github.com/coro-sh/coro/client"
	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/natsutil"
	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/testutil"
)

func exitOnInvalidFlags(c *cli.Context, v *valgo.Validation) {
	if v.ToError() == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Flag errors:")

	for _, verr := range v.ToError().(*valgo.Error).Errors() {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", verr.Name(), strings.Join(verr.Messages(), ","))
	}

	fmt.Fprintln(os.Stdout) //nolint:errcheck
	cli.ShowAppHelpAndExit(c, 1)
}

func waitForClientConnectionHealthy(ctx context.Context, port int, maxRetries int, interval time.Duration) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := func() error {
			c := http.DefaultClient
			c.Timeout = 3 * time.Second

			healthzURL := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)

			var res *http.Response
			var err error

			for i := 0; i < maxRetries; i++ {
				req, err := http.NewRequest(http.MethodGet, healthzURL, nil)
				if err != nil {
					return err
				}
				res, err = c.Do(req.WithContext(ctx))
				if err == nil && res.StatusCode == http.StatusOK {
					return nil
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return ctx.Err()
				}

				time.Sleep(interval)
			}

			if err != nil {
				return fmt.Errorf("client connection unhealthy: %w", err)
			} else if res != nil {
				return fmt.Errorf("client connection unhealthy: %s", http.StatusText(res.StatusCode))
			}

			return errors.New("client connection unhealthy: max retries exceeded")
		}()
		if err != nil {
			errCh <- err
		}
	}()
	return errCh
}

func createNamespace(ctx context.Context, c *client.Client, logger log.Logger) (client.Namespace, error) {
	namespace, err := c.CreateNamespace(ctx, client.CreateNamespaceRequest{
		Name: testutil.RandName(),
	})
	if err != nil {
		return client.Namespace{}, fmt.Errorf("create namespace: %w", err)
	}
	logger.Info(fmt.Sprintf("created namespace '%s'", namespace.Name))
	return namespace, nil
}

func createOperator(ctx context.Context, c *client.Client, logger log.Logger, namespaceID string) (client.Operator, string, []byte, error) {
	operator, err := c.CreateOperator(ctx, namespaceID, client.CreateOperatorRequest{
		Name: testutil.RandName(),
	})
	if err != nil {
		return client.Operator{}, "", nil, fmt.Errorf("create operator: %w", err)
	}
	logger.Info(fmt.Sprintf("created operator '%s'", operator.Name))

	proxyTkn, err := c.GenerateOperatorProxyToken(ctx, namespaceID, operator.Id)
	if err != nil {
		return client.Operator{}, "", nil, fmt.Errorf("generated operator proxy token: %w", err)
	}
	logger.Info(fmt.Sprintf("generated operatory proxy token: %s", proxyTkn))

	natsCfg, err := c.GetOperatorNATSConfig(ctx, namespaceID, operator.Id)
	if err != nil {
		return client.Operator{}, "", nil, fmt.Errorf("get operator nats config: %w", err)
	}
	logger.Info("fetched operator nats config")

	resolverDirPath := os.TempDir() + "/coro/" + time.Now().Format("20060102150405")
	defer os.RemoveAll(resolverDirPath)
	natsCfg = replaceNATSConfigDir(natsCfg, resolverDirPath)

	return operator, proxyTkn, natsCfg, nil
}

func createAccount(ctx context.Context, c *client.Client, logger log.Logger, namespaceID string, operatorID string) (client.Account, error) {
	account, err := c.CreateAccount(ctx, namespaceID, operatorID, client.CreateAccountRequest{
		Name: testutil.RandName(),
	})
	if err != nil {
		return client.Account{}, fmt.Errorf("create account: %w", err)
	}
	logger.Info(fmt.Sprintf("created account '%s'", account.Name))
	return account, nil
}

func createUser(ctx context.Context, c *client.Client, logger log.Logger, namespaceID string, accountID string) (client.User, string, string, error) {
	user, err := c.CreateUser(ctx, namespaceID, accountID, client.CreateUserRequest{
		Name: testutil.RandName(),
	})
	if err != nil {
		return client.User{}, "", "", fmt.Errorf("create user: %w", err)
	}
	logger.Info(fmt.Sprintf("created user '%s'", user.Name))

	userCreds, err := c.GetUserCreds(ctx, namespaceID, user.Id)
	if err != nil {
		return client.User{}, "", "", fmt.Errorf("get user creds: %w", err)
	}
	logger.Info(fmt.Sprintf("fetched user '%s' nats credentials", user.Name))

	userJWT, err := jwt.ParseDecoratedJWT(userCreds)
	if err != nil {
		return client.User{}, "", "", fmt.Errorf("parse user nkey: %w", err)
	}
	userNkey, err := jwt.ParseDecoratedNKey(userCreds)
	if err != nil {
		return client.User{}, "", "", fmt.Errorf("parse user nkey: %w", err)
	}
	userSeed, err := userNkey.Seed()
	if err != nil {
		return client.User{}, "", "", fmt.Errorf("get seed from user jwt: %w", err)
	}

	return user, userJWT, string(userSeed), nil
}

// Hack: overwrite resolver directory path with another one to avoid unwanted
// files in the current working directory.
func replaceNATSConfigDir(cfgContent []byte, newDir string) []byte {
	newCfg := strings.ReplaceAll(string(cfgContent), entity.DefaultResolverDir, newDir)
	return []byte(newCfg)
}

func startNATS(cfg []byte) (*natsrv.Server, error) {
	cfgFile, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()
	defer os.RemoveAll(cfgFile.Name())

	err = os.WriteFile(cfgFile.Name(), cfg, 0666)
	if err != nil {
		return nil, err
	}

	opts, err := natsrv.ProcessConfigFile(cfgFile.Name())
	if err != nil {
		return nil, err
	}

	opts.Port = natsrv.RANDOM_PORT

	ns, err := natsrv.NewServer(opts)
	if err != nil {
		return nil, err
	}
	ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, errors.New("nats server unhealthy")
	}
	return ns, nil
}

func connectNATS(ns *natsrv.Server, logger log.Logger, user client.User, userJWT string, userSeed string) (*nats.Conn, jetstream.JetStream, error) {
	logger.Info(fmt.Sprintf("connecting to nats server with user '%s' credentials", user.Name))
	nc, err := natsutil.Connect(ns.ClientURL(), userJWT, userSeed)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to nats server with user creds: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, nil, fmt.Errorf("create jetstream client: %w", err)
	}
	return nc, js, nil
}

func createStreamAndPublisher(ctx context.Context, js jetstream.JetStream, logger log.Logger, errCh chan error) error {
	streamSubject := "dev-server.*"
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "stream-" + testutil.RandName(),
		Description: "A test stream created by the dev server",
		Subjects:    []string{streamSubject},
		Retention:   jetstream.LimitsPolicy,
		MaxMsgs:     100,
		Discard:     jetstream.DiscardOld,
		MaxAge:      time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create jetstream dev server stream: %w", err)
	}
	// Immediately publish 10 messages to the stream
	for i := 0; i < 10; i++ {
		if _, perr := js.Publish(ctx, streamSubject, []byte(fmt.Sprintf("devserver message %d", i))); perr != nil {
			return fmt.Errorf("publish message to dev server stream subject '%s': %w", streamSubject, perr)
		}
		logger.Info(fmt.Sprintf("published message to dev server stream subject '%s' (total: %d)", streamSubject, i))
	}
	// Then publish messages every 30s
	go func() {
		i := 1
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				if _, perr := js.Publish(ctx, streamSubject, []byte(fmt.Sprintf("devserver message %d", i))); perr != nil {
					errCh <- fmt.Errorf("publish message to dev server stream subject '%s': %w", streamSubject, perr)
					return
				}
				logger.Info(fmt.Sprintf("published message to dev server stream subject '%s' (total: %d)", streamSubject, i+10))
				i++
			}
		}
	}()
	return nil
}

func startProxy(
	ctx context.Context,
	c *client.Client,
	logger log.Logger,
	natsURL string,
	brokerURL string,
	namespaceID string,
	operatorID string,
	proxyTkn string,
) (*command.Proxy, error) {
	proxy, err := command.NewProxy(ctx, natsURL, brokerURL, proxyTkn, command.WithProxyLogger(logger))
	if err != nil {
		return nil, err
	}
	proxy.Start(ctx)
	logger.Info("proxy started")

	for i := 0; i < clientMaxReconnectAttempts; i++ {
		statusRes, err := c.GetOperatorProxyConnectionStatus(ctx, namespaceID, operatorID)
		if err != nil {
			return nil, fmt.Errorf("get operator proxy connection status: %w", err)
		}
		if statusRes.Connected {
			logger.Info("proxy healthy")
			break
		}
		if i == clientMaxReconnectAttempts-1 {
			return nil, fmt.Errorf("proxy unhealthy")
		}
		time.Sleep(clientReconnectDelay)
	}

	logger.Info("proxy connection to nats is healthy")
	return proxy, nil
}
