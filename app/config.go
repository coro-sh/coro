package app

import (
	"fmt"

	"github.com/cohesivestack/valgo"
	"github.com/joshjon/kit/log"
	"github.com/joshjon/kit/valgoutil"

	"github.com/coro-sh/coro/postgres"
)

const (
	defaultServerPort   = 5400
	defaultUIServerPort = 8400
)

// Service configs

type AllConfig struct {
	BaseConfig  `yaml:",inline"`
	CorsOrigins []string `yaml:"corsOrigins" env:"CORS_ORIGINS"`
}

func (c *AllConfig) Validation() *valgo.Validation {
	v := c.BaseConfig.Validation()
	for i, origin := range c.CorsOrigins {
		v.InRow("corsOrigins", i, valgo.Is(valgoutil.CORSValidator(origin, "origin")))
	}
	return v
}

type UIConfig struct {
	Port       int          `yaml:"port" env:"PORT"` // default: 8400
	Logger     LoggerConfig `yaml:"logger" envPrefix:"LOGGER_"`
	APIAddress string       `yaml:"apiAddress" env:"API_ADDRESS"` // default: http://localhost:5400
	TLS        *TLSConfig   `yaml:"tls" envPrefix:"TLS_"`
}

func (c *UIConfig) InitDefaults() {
	c.Port = defaultUIServerPort
	c.Logger.InitDefaults()
	c.APIAddress = fmt.Sprintf("http://localhost:%d", defaultServerPort)
}

func (c *UIConfig) Validation() *valgo.Validation {
	v := valgo.New()
	v.Is(valgo.Int(c.Port, "port").GreaterOrEqualTo(0))
	v.In("logger", c.Logger.Validation())
	v.Is(valgoutil.URLValidator(c.APIAddress, "apiAddress"))

	if c.TLS != nil {
		v.In("tls", c.TLS.Validation())
	}

	return v
}

type ControllerConfig struct {
	BaseConfig  `yaml:",inline"`
	CorsOrigins []string                `yaml:"corsOrigins" env:"CORS_ORIGINS"`
	Broker      *ControllerBrokerConfig `yaml:"broker,omitempty" envPrefix:"BROKER_"`
}

func (c *ControllerConfig) InitDefaults() {
	c.BaseConfig.InitDefaults()
}

func (c *ControllerConfig) Validation() *valgo.Validation {
	v := c.BaseConfig.Validation()

	for i, origin := range c.CorsOrigins {
		v.InRow("corsOrigins", i, valgo.Is(valgoutil.URLValidator(origin, "origin")))
	}

	if c.Broker != nil {
		v.In("broker", c.Broker.Validation())
	}

	return v
}

type BrokerConfig struct {
	BaseConfig   `yaml:",inline"`
	EmbeddedNats EmbeddedNATSConfig `yaml:"embeddedNats" envPrefix:"EMBEDDED_NATS_"`
}

func (c *BrokerConfig) InitDefaults() {
	c.BaseConfig.InitDefaults()
}

func (c *BrokerConfig) Validation() *valgo.Validation {
	v := c.BaseConfig.Validation()
	v.In("embeddedNats", c.EmbeddedNats.Validation())
	return v
}

// BaseConfig is the common configuration shared between ControllerConfig and
// BrokerConfig.
type BaseConfig struct {
	Port   int          `yaml:"port" env:"PORT"` // default: 5400
	Logger LoggerConfig `yaml:"logger" envPrefix:"LOGGER_"`
	TLS    *TLSConfig   `yaml:"tls" envPrefix:"TLS_"`
	// EncryptionSecretKey (optional):
	// Enables encryption for sensitive data like nkeys and proxy tokens.
	// The key must be a hex-encoded key that decodes to 16, 24, or 32 bytes (AES-128/192/256).
	// Hex-encoded keys can be generated using:
	//   openssl rand -hex 16 # AES-128
	//   openssl rand -hex 24 # AES-192
	//   openssl rand -hex 32 # AES-256
	EncryptionSecretKey *string        `yaml:"encryptionSecretKey" env:"ENCRYPTION_SECRET_KEY"`
	Postgres            PostgresConfig `yaml:"postgres" envPrefix:"POSTGRES_"`
}

func (c *BaseConfig) InitDefaults() {
	c.Port = defaultServerPort
	c.Logger.InitDefaults()
	c.Postgres.InitDefaults()
}

func (c *BaseConfig) Validation() *valgo.Validation {
	v := valgo.New()
	v.Is(valgo.Int(c.Port, "port").GreaterOrEqualTo(0))
	v.In("logger", c.Logger.Validation())
	v.In("postgres", c.Postgres.Validation())

	if c.EncryptionSecretKey != nil {
		v.Is(valgoutil.HexAESKeyValidator(*c.EncryptionSecretKey, "encryptionSecretKey"))
	}

	if c.TLS != nil {
		v.In("tls", c.TLS.Validation())
	}

	return v
}

// Component configs

type LoggerConfig struct {
	Level      string `yaml:"level" env:"LEVEL"`           // default: info
	Structured bool   `yaml:"structured" env:"STRUCTURED"` // default: true
}

func (c *LoggerConfig) InitDefaults() {
	c.Structured = true
	c.Level = "info"
}

func (c *LoggerConfig) Validation() *valgo.Validation {
	return valgo.Is(valgo.String(c.Level, "level").Passing(func(_ string) bool {
		_, ok := log.ParseLevel(c.Level)
		return ok
	}, "Must be one of [debug, info, warn, error]"))
}

type TLSConfig struct {
	CertFile           string `yaml:"certFile" env:"CERT_FILE"`
	KeyFile            string `yaml:"keyFile" env:"KEY_FILE"`
	CACertFile         string `yaml:"caCertFile" env:"CA_CERT_FILE"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" env:"INSECURE_SKIP_VERIFY"` // client TLS only (not intended for production environments)
}

func (c *TLSConfig) Validation() *valgo.Validation {
	return valgo.Is(
		valgo.String(c.CertFile, "certFile").Not().Blank(),
		valgo.String(c.KeyFile, "keyFile").Not().Blank(),
	)
}

type PostgresConfig struct {
	// Database defaults to "coro". Overriding is discouraged for compatibility with pgtool.
	Database string     `yaml:"database" env:"DATABASE"`
	HostPort string     `yaml:"hostPort"  env:"HOST_PORT"`
	User     string     `yaml:"user"  env:"USER"`
	Password string     `yaml:"password"  env:"PASSWORD"`
	TLS      *TLSConfig `yaml:"tls" envPrefix:"TLS_"`
}

func (c *PostgresConfig) InitDefaults() {
	if c.Database == "" {
		c.Database = postgres.AppDBName
	}
}

func (c *PostgresConfig) Validation() *valgo.Validation {
	v := valgo.Is(
		valgoutil.HostPortValidator(c.HostPort, "hostPort"),
		valgo.String(c.Database, "database").Not().Blank(),
		valgo.String(c.User, "user").Not().Blank(),
	)
	if c.TLS != nil {
		v.In("tls", c.TLS.Validation())
	}
	return v
}

type ControllerBrokerConfig struct {
	// Specifies the external broker NATS URLs to publish notify messages to.
	// At least 1 URL is required. Additional URLs are permitted in a clustered
	// setup.
	NatsURLs []string `yaml:"natsURLs"  env:"NATS_URLS"`
}

func (c *ControllerBrokerConfig) Validation() *valgo.Validation {
	v := valgo.New()

	if len(c.NatsURLs) > 0 {
		v.Is(valgoutil.NonEmptySliceValidator(c.NatsURLs, "natsURLs", "NATS URLs"))
	}

	return v
}

type EmbeddedNATSConfig struct {
	HostPort   string   `yaml:"hostPort"  env:"HOST_PORT"`
	NodeRoutes []string `yaml:"nodeRoutes"  env:"NODE_ROUTES"` // clustering
}

func (c *EmbeddedNATSConfig) Validation() *valgo.Validation {
	v := valgo.New()
	v.Is(valgoutil.HostPortValidator(c.HostPort, "hostPort"))

	for i, route := range c.NodeRoutes {
		v.InRow("nodeRoutes", i, valgo.Is(valgoutil.URLValidator(route, "route")))
	}

	return v
}
