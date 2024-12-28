package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coro-sh/coro/log"
)

const (
	serverCertFile = "../internal/testutil/testdata/certs/server-cert.pem"
	serverKeyFile  = "../internal/testutil/testdata/certs/server-key.pem"
	clientCertFile = "../internal/testutil/testdata/certs/client-cert.pem"
	clientKeyFile  = "../internal/testutil/testdata/certs/client-key.pem"
	caCertFile     = "../internal/testutil/testdata/certs/ca-cert.pem"
)

func TestServer_NewServer(t *testing.T) {
	srv, err := NewServer(443,
		WithLogger(log.NewLogger(log.WithNop())),
		WithCORS("localhost:9999"),
		WithRequestTimeout(time.Second),
	)
	require.NoError(t, err)

	go srv.Start()
	defer srv.Stop(context.Background())
	err = srv.WaitHealthy(5, time.Millisecond)
	require.NoError(t, err)
}

func TestServer_TLS(t *testing.T) {
	srv, err := NewServer(443, WithTLS(serverCertFile, serverKeyFile, ""))
	require.NoError(t, err)

	go srv.Start()
	defer srv.Stop(context.Background())
	time.Sleep(5 * time.Millisecond)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Using self signed certs so InsecureSkipVerify=true
				InsecureSkipVerify: true,
			},
		},
	}

	url := fmt.Sprintf("https://%s/healthz", srv.HostPort())
	httpRes, err := client.Get(url)
	require.NoError(t, err)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusOK, httpRes.StatusCode)
}

func TestServer_mTLS(t *testing.T) {
	srv, err := NewServer(443, WithTLS(serverCertFile, serverKeyFile, caCertFile))
	require.NoError(t, err)

	go srv.Start()
	defer srv.Stop(context.Background())
	time.Sleep(5 * time.Millisecond)

	clientCert, caCertPool := loadClientCerts(t, err)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{clientCert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: false,
			},
		},
	}

	url := fmt.Sprintf("https://%s/healthz", srv.HostPort())
	httpRes, err := client.Get(url)
	require.NoError(t, err)
	defer httpRes.Body.Close()
	assert.Equal(t, http.StatusOK, httpRes.StatusCode)
}

func TestServer_TLSWebSocket(t *testing.T) {
	srv, err := NewServer(443, WithTLS(serverCertFile, serverKeyFile, ""))
	require.NoError(t, err)

	wantMsg := []byte("connected")

	srv.echo.GET("", testWebSocketHandler(wantMsg))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go srv.Start()
	defer srv.Stop(ctx)
	time.Sleep(5 * time.Millisecond)

	conn, _, err := websocket.Dial(ctx, srv.WebsSocketAddress(), &websocket.DialOptions{
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// Using self signed certs so InsecureSkipVerify=true
					InsecureSkipVerify: true,
				},
			},
		},
	})
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "success")

	_, gotMsg, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, wantMsg, gotMsg)
}

func TestServer_mTLSWebSocket(t *testing.T) {
	srv, err := NewServer(443, WithTLS(serverCertFile, serverKeyFile, caCertFile))
	require.NoError(t, err)

	wantMsg := []byte("connected")

	srv.echo.GET("", testWebSocketHandler(wantMsg))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go srv.Start()
	defer srv.Stop(ctx)
	time.Sleep(5 * time.Millisecond)

	clientCert, caCertPool := loadClientCerts(t, err)

	conn, _, err := websocket.Dial(ctx, "wss://127.0.0.1:443", &websocket.DialOptions{
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:       []tls.Certificate{clientCert},
					RootCAs:            caCertPool,
					InsecureSkipVerify: false,
				},
			},
		},
	})
	require.NoError(t, err)

	_, gotMsg, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, wantMsg, gotMsg)
}

func loadClientCerts(t *testing.T, err error) (tls.Certificate, *x509.CertPool) {
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	require.NoError(t, err)
	caCert, err := os.ReadFile(caCertFile)
	require.NoError(t, err)
	caCertPool := x509.NewCertPool()
	require.True(t, caCertPool.AppendCertsFromPEM(caCert))
	return clientCert, caCertPool
}

func testWebSocketHandler(wantMsg []byte) func(c echo.Context) error {
	return func(c echo.Context) error {
		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{})
		if err != nil {
			return err
		}

		ctx := c.Request().Context()
		if err = conn.Write(ctx, websocket.MessageText, wantMsg); err != nil {
			conn.Close(websocket.StatusInternalError, "failure")
			return err
		}

		return conn.Close(websocket.StatusNormalClosure, "success")
	}
}
