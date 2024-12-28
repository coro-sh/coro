package embedns

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/entity"
	"github.com/coro-sh/coro/internal/testutil"
)

const (
	serverCertFile = "../internal/testutil/testdata/certs/server-cert.pem"
	serverKeyFile  = "../internal/testutil/testdata/certs/server-key.pem"
	clientCertFile = "../internal/testutil/testdata/certs/client-cert.pem"
	clientKeyFile  = "../internal/testutil/testdata/certs/client-key.pem"
	caCertFile     = "../internal/testutil/testdata/certs/ca-cert.pem"
)

func TestNewEmbeddedNATS(t *testing.T) {
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	require.NoError(t, err)
	caCert, err := os.ReadFile(caCertFile)
	require.NoError(t, err)
	caCertPool := x509.NewCertPool()
	require.True(t, caCertPool.AppendCertsFromPEM(caCert))

	tests := []struct {
		name      string
		serverTLS *TLSConfig
		connOpts  []nats.Option
	}{
		{
			name:      "insecure server",
			serverTLS: nil,
		},
		{
			name: "server with tls",
			serverTLS: &TLSConfig{
				CertFile: serverCertFile,
				KeyFile:  serverKeyFile,
			},
			connOpts: []nats.Option{
				nats.Secure(&tls.Config{
					InsecureSkipVerify: true,
				}),
			},
		},
		{
			name: "server with mtls",
			serverTLS: &TLSConfig{
				CertFile:   serverCertFile,
				KeyFile:    serverKeyFile,
				CACertFile: caCertFile,
			},
			connOpts: []nats.Option{
				nats.Secure(&tls.Config{
					Certificates:       []tls.Certificate{clientCert},
					RootCAs:            caCertPool,
					InsecureSkipVerify: false,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := entity.NewOperator(testutil.RandName(), entity.NewID[entity.NamespaceID]())
			require.NoError(t, err)
			sysAcc, sysUsr, err := op.SetNewSystemAccountAndUser()
			require.NoError(t, err)

			cfg := EmbeddedNATSConfig{
				NodeName: "test_node",
				Resolver: ResolverConfig{
					Operator:      op,
					SystemAccount: sysAcc,
				},
			}

			if tt.serverTLS != nil {
				cfg.TLS = tt.serverTLS
			}

			srv, err := NewEmbeddedNATS(cfg)
			require.NoError(t, err)
			srv.Start()
			defer srv.Shutdown()

			ok := srv.ReadyForConnections(3 * time.Second)
			require.True(t, ok, "nats unhealthy")

			defaultOpts := []nats.Option{nats.UserJWTAndSeed(sysUsr.JWT(), string(sysUsr.NKey().Seed))}
			opts := append(defaultOpts, tt.connOpts...)

			nc, err := nats.Connect(srv.ClientURL(), opts...)
			require.NoError(t, err)
			defer nc.Close()

			verifyClientConn(t, nc)
		})
	}
}

func verifyClientConn(t *testing.T, nc *nats.Conn) {
	msgChan := make(chan *nats.Msg, 1)
	sub, err := nc.ChanSubscribe("test.subject", msgChan)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	require.NoError(t, nc.Publish("test.subject", []byte("hello world")))
	select {
	case msg := <-msgChan:
		assert.Equal(t, "hello world", string(msg.Data))
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message")
	}
}
