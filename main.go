package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/cohesivestack/valgo"
	"github.com/urfave/cli/v2"

	"github.com/coro-sh/coro/app"
	"github.com/coro-sh/coro/log"
)

const (
	serviceTypeAll        string = "all"
	serviceTypeUI         string = "ui"
	serviceTypeAllBackend string = "backend"
	serviceTypeController string = "controller"
	serviceTypeBroker     string = "broker"
)

var serviceTypes = []string{serviceTypeAll, serviceTypeUI, serviceTypeAllBackend, serviceTypeController, serviceTypeBroker}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cliApp := cli.NewApp()
	cliApp.Name = "coro"
	cliApp.Usage = "Coro"

	cliApp.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "service",
			Aliases: []string{"s"},
			Value:   "",
			Usage:   fmt.Sprintf("[required] service to start (%s)", strings.Join(serviceTypes, ", ")),
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "",
			Usage:   "path to yaml config file (required if not using env vars)",
		},
	}

	logger := log.NewLogger()

	cliApp.Commands = []*cli.Command{
		{
			Name:  "run",
			Usage: "[default] runs the service",
			Action: func(c *cli.Context) error {
				f := parseFlags(c)
				switch f.service {
				case serviceTypeAll:
					var cfg app.AllConfig
					app.LoadConfig(f.configFile, &cfg)
					logger = loggerFromConfig(cfg.Logger).With(log.KeyService, serviceTypeAll)
					return app.RunAll(ctx, logger, cfg, true)
				case serviceTypeUI:
					var cfg app.UIConfig
					app.LoadConfig(f.configFile, &cfg)
					logger = loggerFromConfig(cfg.Logger).With(log.KeyService, serviceTypeUI)
					return app.RunUI(ctx, logger, cfg)
				case serviceTypeAllBackend:
					var cfg app.AllConfig
					app.LoadConfig(f.configFile, &cfg)
					logger = loggerFromConfig(cfg.Logger).With(log.KeyService, serviceTypeAllBackend)
					return app.RunAll(ctx, logger, cfg, false)
				case serviceTypeController:
					var cfg app.ControllerConfig
					app.LoadConfig(f.configFile, &cfg)
					logger = loggerFromConfig(cfg.Logger).With(log.KeyService, serviceTypeController)
					return app.RunController(ctx, logger, cfg)
				case serviceTypeBroker:
					var cfg app.BrokerConfig
					app.LoadConfig(f.configFile, &cfg)
					logger = loggerFromConfig(cfg.Logger).With(log.KeyService, serviceTypeBroker)
					return app.RunBroker(ctx, logger, cfg)
				default:
					return fmt.Errorf("invalid service type: %s", f.service)
				}
			},
		},
	}

	cliApp.DefaultCommand = "run"

	if err := cliApp.RunContext(ctx, os.Args); err != nil {
		logger.Error("failed to start service", "error", err)
		os.Exit(1)
	}
}

type flags struct {
	service    string
	configFile string
}

func (c flags) validate() *valgo.Validation {
	svcTypeTemplate := fmt.Sprintf("must be one of [%s] or left empty", strings.Join(serviceTypes, ", "))
	return valgo.Is(valgo.String(c.service, "service").InSlice(serviceTypes, svcTypeTemplate))
}

func parseFlags(c *cli.Context) flags {
	f := flags{
		service:    c.String("service"),
		configFile: c.String("config"),
	}
	exitOnInvalidFlags(c, f.validate())
	return f
}

func exitOnInvalidFlags(c *cli.Context, v *valgo.Validation) {
	if v.Error() == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Flag errors:")

	for _, verr := range v.Error().(*valgo.Error).Errors() {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", verr.Name(), strings.Join(verr.Messages(), ","))
	}

	fmt.Fprintln(os.Stdout) //nolint:errcheck
	cli.ShowAppHelpAndExit(c, 1)
}

func loggerFromConfig(cfg app.LoggerConfig) log.Logger {
	level, ok := log.ParseLevel(cfg.Level)
	if !ok {
		level = slog.LevelInfo
	}
	opts := []log.LoggerOption{log.WithLevel(level)}
	if !cfg.Structured {
		opts = append(opts, log.WithDevelopment())
	}
	return log.NewLogger(opts...)
}
