package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/coro-sh/coro/constants"
)

const (
	AppDBName           = constants.AppName
	healthRetryInterval = time.Second
	healthMaxRetries    = 5
)

type DialOption func(opts *dialOpts)

type TLSConfig struct {
	CertFile           string // Path to the client certificate file.
	KeyFile            string // Path to the client key file.
	CACertFile         string // Path to the CA certificate file.
	InsecureSkipVerify bool   // Allows skipping TLS certificate verification.
}

func WithTLS(tls TLSConfig) DialOption {
	return func(opts *dialOpts) {
		opts.tls = &tls
	}
}

type dialOpts struct {
	tls *TLSConfig
}

func Dial(ctx context.Context, username string, password string, hostPort string, database string, opts ...DialOption) (*pgxpool.Pool, error) {
	var options dialOpts
	for _, opt := range opts {
		opt(&options)
	}

	url := fmt.Sprintf("postgres://%s:%s@%s/%s", username, password, hostPort, database)

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	if options.tls != nil {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: options.tls.InsecureSkipVerify,
		}

		if options.tls.CertFile != "" && options.tls.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(options.tls.CertFile, options.tls.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("load client certificate/key: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		if options.tls.CACertFile != "" {
			var err error
			tlsConfig.RootCAs, err = loadCACert(options.tls.CACertFile)
			if err != nil {
				return nil, err
			}
		}

		cfg.ConnConfig.TLSConfig = tlsConfig
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err = waitHealthy(ctx, pool); err != nil {
		return nil, err
	}

	return pool, nil
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

func waitHealthy(ctx context.Context, pool *pgxpool.Pool) error {
	pingFn := func() error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return pool.Ping(ctx)
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(healthRetryInterval), healthMaxRetries)
	if err := backoff.Retry(pingFn, bo); err != nil {
		return fmt.Errorf("postgres connection unhealthy: %w", err)
	}
	return nil
}
