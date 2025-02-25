package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/cohesivestack/valgo"
	"gopkg.in/yaml.v3"

	"github.com/coro-sh/coro/internal/valgoutil"
	"github.com/coro-sh/coro/log"
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

func (c *AllConfig) InitDefaults() {
	c.BaseConfig.InitDefaults()
}

func (c *AllConfig) Validation() *valgo.Validation {
	v := c.BaseConfig.Validation()

	for i, origin := range c.CorsOrigins {
		v.InRow("corsOrigins", i, valgo.Is(valgoutil.URLValidator(origin, "origin")))
	}

	return v
}

type UIConfig struct {
	Port        int          `yaml:"port" env:"PORT"` // default: 8400
	Logger      LoggerConfig `yaml:"logger" envPrefix:"LOGGER_"`
	APIAddress  string       `yaml:"apiAddress" env:"API_ADDRESS"` // default: http://localhost:5400
	TLS         *TLSConfig   `yaml:"tls" envPrefix:"TLS"`
	CorsOrigins []string     `yaml:"corsOrigins" env:"CORS_ORIGINS"`
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

	for i, origin := range c.CorsOrigins {
		v.InRow("corsOrigins", i, valgo.Is(valgoutil.URLValidator(origin, "origin")))
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
	TLS    *TLSConfig   `yaml:"tls" envPrefix:"TLS"`
	// EncryptionSecretKey (optional):
	// Enables encryption for sensitive data like nkeys and proxy tokens.
	// The key must be one of the following lengths: 16, 24, or 32 bytes,
	// corresponding to AES-128, AES-192, and AES-256, respectively. A key can
	// be generated using a tool such as openssl e.g. `openssl rand -hex 32`.
	EncryptionSecretKey *string        `yaml:"encryptionSecretKey" env:"ENCRYPTION_SECRET_KEY"`
	Postgres            PostgresConfig `yaml:"postgres" envPrefix:"POSTGRES_"`
}

func (c *BaseConfig) InitDefaults() {
	c.Port = defaultServerPort
	c.Logger.InitDefaults()
}

func (c *BaseConfig) Validation() *valgo.Validation {
	v := valgo.New()
	v.Is(valgo.Int(c.Port, "port").GreaterOrEqualTo(0))
	v.In("logger", c.Logger.Validation())
	v.In("postgres", c.Postgres.Validation())

	if c.EncryptionSecretKey != nil {
		v.Is(valgo.String(*c.EncryptionSecretKey, "encryptionSecretKey").
			OfLength(16).Or().OfLength(24).Or().OfLength(32))
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
	HostPort string     `yaml:"hostPort"  env:"HOST_PORT"`
	User     string     `yaml:"user"  env:"USER"`
	Password string     `yaml:"password"  env:"PASSWORD"`
	TLS      *TLSConfig `yaml:"tls" envPrefix:"TLS"`
}

func (c PostgresConfig) Validation() *valgo.Validation {
	v := valgo.Is(
		valgoutil.HostPortValidator(c.HostPort, "hostPort"),
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

type Configurable interface {
	InitDefaults()
	Validation() *valgo.Validation
}

func LoadConfig(configFile string, out Configurable) {
	err := func() error {
		out.InitDefaults()

		if configFile != "" {
			file, err := os.Open(configFile)
			if err != nil {
				return fmt.Errorf("open config file: %w", err)
			}
			defer file.Close()
			decoder := yaml.NewDecoder(file)
			if err = decoder.Decode(out); err != nil {
				return fmt.Errorf("decode config file: %w", err)
			}
		}

		if err := env.Parse(out); err != nil {
			return fmt.Errorf("parse config environment variables: %w", err)
		}

		if err := out.Validation().Error(); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Config errors:")
		var verr *valgo.Error
		if errors.As(err, &verr) {
			for _, valErr := range verr.Errors() {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", valErr.Name(), strings.Join(valErr.Messages(), ","))
			}
		} else {
			fmt.Fprintln(os.Stderr, fmt.Errorf("  %s", err.Error()))
		}
		os.Exit(1)
	}
}
