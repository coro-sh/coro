package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/coro-sh/coro/log"
)

const (
	DefaultRequestTimeout = 100 * time.Second
	APIPath               = "/api"
)

// Option optionally configures a Server.
type Option func(opts *options) error

// WithLogger sets a custom Logger.
func WithLogger(logger log.Logger) Option {
	return func(opts *options) error {
		opts.logger = logger.With(log.KeyComponent, "server")
		return nil
	}
}

// WithRequestTimeout sets the timeout for request handlers.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(opts *options) error {
		opts.timeout = &timeout
		return nil
	}
}

// WithCORS configures the server to use Cross-Origin Resource Sharing (CORS).
func WithCORS(origins ...string) Option {
	return func(opts *options) error {
		opts.corsOrigins = origins
		return nil
	}
}

// WithMiddleware adds custom middleware to the server.
func WithMiddleware(middlewares ...echo.MiddlewareFunc) Option {
	return func(opts *options) error {
		opts.middlewares = middlewares
		return nil
	}
}

type tlsConfig struct {
	cert   string
	key    string
	caCert string // mTLS
}

// WithTLS configures the server to use TLS with the specified certificate, key,
// and optional CA certificate for mTLS. If caCertFile is provided, the server
// requires client certificates and validates them against the CA.
func WithTLS(certFile string, keyFile string, caCertFile string) Option {
	return func(opts *options) error {
		opts.tlsConfig = &tlsConfig{
			cert:   certFile,
			key:    keyFile,
			caCert: caCertFile,
		}
		return nil
	}
}

type options struct {
	logger      log.Logger
	timeout     *time.Duration
	corsOrigins []string
	middlewares []echo.MiddlewareFunc
	tlsConfig   *tlsConfig // nil to disable
}

// Server serves an API for managing NATS operators, accounts, and users.
type Server struct {
	hostPort  string
	echo      *echo.Echo
	tlsConfig *tlsConfig
	apiGroup  *echo.Group
	logger    log.Logger
}

// NewServer creates a new Server with the given options.
func NewServer(port int, opts ...Option) (*Server, error) {
	srvOpts := options{
		logger: log.NewLogger(),
	}

	for _, opt := range opts {
		if err := opt(&srvOpts); err != nil {
			return nil, err
		}
	}

	srv := &Server{
		hostPort:  fmt.Sprintf("0.0.0.0:%d", port),
		echo:      echo.New(),
		logger:    srvOpts.logger,
		tlsConfig: srvOpts.tlsConfig,
	}

	srv.echo.HideBanner = true
	srv.echo.HidePort = true
	srv.echo.Pre(middleware.RemoveTrailingSlash())
	srv.echo.Use(middleware.Recover())
	srv.echo.Use(middleware.RequestLoggerWithConfig(newRequestLoggerConfig(srv.logger)))
	srv.echo.Use(errorTransformMiddleware)
	srv.echo.HTTPErrorHandler = httpErrorHandlerFunc(srv.logger)
	if len(srvOpts.corsOrigins) > 0 {
		srv.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: srvOpts.corsOrigins,
		}))
	}

	for _, m := range srvOpts.middlewares {
		srv.echo.Use(m)
	}

	timeoutCfg := middleware.TimeoutConfig{
		Timeout: DefaultRequestTimeout,
		Skipper: func(c echo.Context) bool {
			return c.IsWebSocket()
		},
	}
	if srvOpts.timeout != nil {
		timeoutCfg.Timeout = *srvOpts.timeout
	}
	srv.echo.Use(middleware.TimeoutWithConfig(timeoutCfg))

	srv.echo.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, HealthResponse{
			Status: http.StatusText(http.StatusOK),
		})
	})

	srv.apiGroup = srv.echo.Group(APIPath)

	return srv, nil
}

// Start begins serving on the configured host and port.
func (s *Server) Start() error {
	if s.tlsConfig == nil {
		err := s.echo.Start(s.hostPort)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{},
	}

	serverCert, err := tls.LoadX509KeyPair(s.tlsConfig.cert, s.tlsConfig.key)
	if err != nil {
		return fmt.Errorf("load server certificate: %w", err)
	}
	tlsCfg.Certificates = []tls.Certificate{serverCert}

	if s.tlsConfig.caCert != "" {
		caCertPool := x509.NewCertPool()
		caCert, err := os.ReadFile(s.tlsConfig.caCert)
		if err != nil {
			return fmt.Errorf("read ca certificate: %w", err)
		}
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("append ca certificate")
		}
		tlsCfg.ClientCAs = caCertPool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	s.echo.TLSServer.TLSConfig = tlsCfg

	err = s.echo.StartTLS(s.hostPort, s.tlsConfig.cert, s.tlsConfig.key)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// HostPort returns the host and port the server is running on.
func (s *Server) HostPort() string {
	return s.hostPort
}

func (s *Server) WaitHealthy(maxRetries int, interval time.Duration) error {
	healthzURL := fmt.Sprintf("%s/healthz", s.Address())

	var res *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		res, err = http.Get(healthzURL)
		if err == nil && res.StatusCode == http.StatusOK {
			return nil
		}

		time.Sleep(interval)
	}

	if err != nil {
		return fmt.Errorf("server unhealthy: %w", err)
	} else if res != nil {
		return fmt.Errorf("server unhealthy: %s", http.StatusText(res.StatusCode))
	}

	return errors.New("server unhealthy")
}

// Address returns the server address which clients can connect to.
func (s *Server) Address() string {
	if s.tlsConfig == nil {
		return "http://" + s.HostPort()
	}
	return "https://" + s.HostPort()
}

// WebsSocketAddress returns the server WebSocket address which clients can connect to.
func (s *Server) WebsSocketAddress() string {
	if s.tlsConfig == nil {
		return "ws://" + s.HostPort()
	}
	return "wss://" + s.HostPort()
}

type Handler interface {
	Register(g *echo.Group)
}

func (s *Server) Register(h Handler) {
	h.Register(s.apiGroup)
}

func (s *Server) Add(method string, path string, handler echo.HandlerFunc) {
	s.echo.Add(method, path, handler)
}

func (s *Server) Any(path string, handler echo.HandlerFunc) {
	s.echo.Any(path, handler)
}
