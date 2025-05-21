package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/cohesivestack/valgo"
	"github.com/urfave/cli/v2"

	"github.com/coro-sh/coro/command"
	"github.com/coro-sh/coro/internal/valgoutil"
	"github.com/coro-sh/coro/log"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	logger := log.NewLogger()

	if err := run(ctx, os.Args, logger); err != nil {
		logger.Error("failed to start proxy agent", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, logger log.Logger) error {
	app := cli.NewApp()
	app.Name = "coro-proxy-agent"
	app.Usage = "Proxy to connect an Operator NATS server to the Coro Broker"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "broker-url",
			Aliases: []string{"b"},
			Value:   "",
			Usage:   "[required] broker websocket url e.g. ws://<host>:<port>/api/v1/broker",
			EnvVars: []string{"BROKER_WEBSOCKET_URL"},
		},
		&cli.StringFlag{
			Name:    "nats-url",
			Aliases: []string{"n"},
			Value:   "",
			Usage:   "[required] operator nats server url e.g. nats://<host>:<port>",
			EnvVars: []string{"NATS_URL"},
		},
		&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"t"},
			Value:   "",
			Usage:   "[required] operator proxy authorization token",
			EnvVars: []string{"PROXY_TOKEN"},
		},
		&cli.StringFlag{
			Name:    "broker-cert",
			Aliases: []string{"bc"},
			Value:   "",
			Usage:   "cert for broker client (mtls)",
			EnvVars: []string{"BROKER_CERT"},
		},
		&cli.StringFlag{
			Name:    "broker-key",
			Aliases: []string{"bk"},
			Value:   "",
			Usage:   "key for broker client (mtls)",
			EnvVars: []string{"BROKER_KEY"},
		},
		&cli.StringFlag{
			Name:    "broker-ca-cert",
			Aliases: []string{"bca"},
			Value:   "",
			Usage:   "ca cert to verify broker (tls/mtls)",
			EnvVars: []string{"BROKER_CA_CERT"},
		},
		&cli.StringFlag{
			Name:    "broker-skip-insecure",
			Aliases: []string{"bs"},
			Value:   "",
			Usage:   "skip broker tls verification (TLS/mTLS)",
			EnvVars: []string{"BROKER_SKIP_INSECURE"},
		},
		&cli.StringFlag{
			Name:    "nats-cert",
			Aliases: []string{"nc"},
			Value:   "",
			Usage:   "cert for operator nats client (mtls)",
			EnvVars: []string{"NATS_CERT"},
		},
		&cli.StringFlag{
			Name:    "nats-key",
			Aliases: []string{"nk"},
			Value:   "",
			Usage:   "key for operator nats client (mtls)",
			EnvVars: []string{"NATS_KEY"},
		},
		&cli.StringFlag{
			Name:    "nats-ca-cert",
			Aliases: []string{"nca"},
			Value:   "",
			Usage:   "ca cert to verify operator nats (tls/mtls)",
			EnvVars: []string{"NATS_CA_CERT"},
		},
		&cli.StringFlag{
			Name:    "nats-skip-insecure",
			Aliases: []string{"ns"},
			Value:   "",
			Usage:   "skip operator nats tls verification (TLS/mTLS)",
			EnvVars: []string{"NATS_SKIP_INSECURE"},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "run",
			Usage: "[default] runs the proxy",
			Action: func(c *cli.Context) error {
				cfg := loadConfig(c)
				return cmdRun(ctx, logger, cfg)
			},
		},
	}

	app.DefaultCommand = "run"

	return app.RunContext(ctx, args)
}

func cmdRun(ctx context.Context, logger log.Logger, cfg config) error {
	// Broker TLS
	var brokerTLSConfig *command.TLSConfig
	if cfg.brokerCertFile != "" && cfg.brokerKeyFile != "" {
		brokerTLSConfig = &command.TLSConfig{
			CertFile:           cfg.brokerCertFile,
			KeyFile:            cfg.brokerKeyFile,
			CACertFile:         cfg.brokerCACertFile,
			InsecureSkipVerify: cfg.brokerSkipVerify,
		}
	}

	// NATS TLS
	var natsTLSConfig *tls.Config
	if cfg.natsCertFile != "" && cfg.natsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.natsCertFile, cfg.natsKeyFile)
		if err != nil {
			return fmt.Errorf("load nats certificates: %w", err)
		}

		natsTLSConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: cfg.natsSkipVerify,
		}

		if cfg.natsCACertFile != "" {
			caCertPool, err := loadCACert(cfg.natsCACertFile)
			if err != nil {
				return err
			}
			natsTLSConfig.RootCAs = caCertPool
		}
	}

	proxyOptions := []command.ProxyOption{command.WithProxyLogger(logger)}
	if brokerTLSConfig != nil {
		proxyOptions = append(proxyOptions, command.WithProxyBrokerTLS(*brokerTLSConfig))
	}
	if natsTLSConfig != nil {
		proxyOptions = append(proxyOptions, command.WithProxyNatsTLS(natsTLSConfig))
	}

	errCh := make(chan error)
	var stop func() error

	go func() {
		pxy, err := command.NewProxy(ctx, cfg.natsURL, cfg.brokerURL, cfg.token, proxyOptions...)
		if err != nil {
			errCh <- err
			return
		}

		pxy.Start(ctx)
		stop = pxy.Stop
		logger.Info("proxy started")
	}()

	defer func() {
		if stop != nil {
			stop() //nolint:errcheck
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("proxy stopped")
		return nil
	}
}

type config struct {
	brokerURL        string
	natsURL          string
	token            string
	brokerCertFile   string
	brokerKeyFile    string
	brokerCACertFile string
	brokerSkipVerify bool
	natsCertFile     string
	natsKeyFile      string
	natsCACertFile   string
	natsSkipVerify   bool
}

func (c config) validate() *valgo.Validation {
	return valgo.Is(
		valgoutil.URLValidator(c.brokerURL, "broker-url"),
		valgoutil.URLValidator(c.natsURL, "nats-url"),
		valgo.String(c.token, "token").Not().Blank("must not be empty"),
	)
}

func loadConfig(c *cli.Context) config {
	cfg := config{
		brokerURL:        c.String("broker-url"),
		natsURL:          c.String("nats-url"),
		token:            c.String("token"),
		brokerCertFile:   c.String("broker-cert"),
		brokerKeyFile:    c.String("broker-key"),
		brokerCACertFile: c.String("broker-ca-cert"),
		brokerSkipVerify: c.Bool("broker-skip-verify"),
		natsCertFile:     c.String("nats-cert"),
		natsKeyFile:      c.String("nats-key"),
		natsCACertFile:   c.String("nats-ca-cert"),
		natsSkipVerify:   c.Bool("nats-skip-verify"),
	}
	exitOnInvalidFlags(c, cfg.validate())
	return cfg
}

func exitOnInvalidFlags(c *cli.Context, v *valgo.Validation) {
	if v.Error() == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Flag errors:")

	for _, verr := range v.Error().(*valgo.Error).Errors() {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", verr.Name(), strings.Join(verr.Messages(), ","))
	}

	fmt.Fprintln(os.Stdout)
	cli.ShowAppHelpAndExit(c, 1)
}

func loadCACert(caCertFile string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("read ca certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append ca certificate")
	}

	return caCertPool, nil
}
