package embedns

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/coro-sh/coro/entity"
)

type ResolverConfig struct {
	Operator      *entity.Operator
	SystemAccount *entity.Account
}

type ClusterConfig struct {
	ClusterName     string
	ClusterHostPort string
	Routes          []*url.URL
}

type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CACertFile string
}

// EmbeddedNATSConfig represents the configuration for an embedded NATS server
// with clustering and TLS/mTLS support.
type EmbeddedNATSConfig struct {
	NodeName string
	Resolver ResolverConfig
	Cluster  *ClusterConfig
	TLS      *TLSConfig
}

func NewEmbeddedNATS(cfg EmbeddedNATSConfig) (*server.Server, error) {
	if cfg.Resolver.Operator == nil {
		return nil, errors.New("resolver operator required")
	}
	if cfg.Resolver.SystemAccount == nil {
		return nil, errors.New("resolver system account required")
	}

	cfgContent, err := newInMemResolverConfig(cfg.Resolver.Operator, cfg.Resolver.SystemAccount)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.CreateTemp("", "embedded-nats-*.conf")
	if err != nil {
		return nil, fmt.Errorf("create embedded nats temp config file: %w", err)
	}
	defer os.Remove(cfgFile.Name())

	cfgFileName := cfgFile.Name()
	if err = cfgFile.Close(); err != nil {
		return nil, fmt.Errorf("close embedded nats temp config file: %w", err)
	}

	if err = os.WriteFile(cfgFileName, []byte(cfgContent), 0666); err != nil {
		return nil, fmt.Errorf("write embedded nats temp config file: %w", err)
	}

	opts, err := server.ProcessConfigFile(cfgFileName)
	if err != nil {
		return nil, fmt.Errorf("process embedded nats config file: %w", err)
	}

	opts.ServerName = cfg.NodeName
	opts.Host = server.DEFAULT_HOST
	opts.Port = server.RANDOM_PORT
	opts.NoSigs = true

	if cfg.TLS != nil {
		tlsConfig, err := configureTLS(cfg.TLS)
		if err != nil {
			return nil, err
		}
		opts.TLSConfig = tlsConfig
		opts.TLS = true
	}

	if cfg.Cluster != nil {
		clusterHost, clusterPortStr, err := net.SplitHostPort(cfg.Cluster.ClusterHostPort)
		if err != nil {
			return nil, fmt.Errorf("invalid cluster host/port: %w", err)
		}

		clusterPort, err := strconv.Atoi(clusterPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid cluster port: %w", err)
		}

		opts.Routes = cfg.Cluster.Routes
		opts.Cluster = server.ClusterOpts{
			Name: cfg.Cluster.ClusterName,
			Host: clusterHost,
			Port: clusterPort,
		}
	}

	opts.MaxPayload = 256 * 1024        // 256KB max message size
	opts.MaxPending = 5 * 1024 * 1024   // 5MB max pending bytes per connection
	opts.WriteDeadline = 10 * time.Second

	return server.NewServer(opts)
}

func configureTLS(cfg *TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert and key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if cfg.CACertFile != "" {
		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("read ca cert: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to append ca cert")
		}

		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}
