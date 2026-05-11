package sentry

import (
	"context"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/nanostack-dev/shared/fxmodules/config"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const flushTimeout = 2 * time.Second

type Config struct {
	DSN              string  `yaml:"dsn"                optional:"true"`
	Environment      string  `yaml:"environment"        optional:"true"`
	Release          string  `yaml:"release"            optional:"true"`
	EnableTracing    bool    `yaml:"enable_tracing"     optional:"true"`
	TracesSampleRate float64 `yaml:"traces_sample_rate" optional:"true"`
}

func NewModule() fx.Option {
	return fx.Module("sentry", fx.Invoke(initialize))
}

func initialize(lifecycle fx.Lifecycle, loader config.Loader, logger zerolog.Logger) {
	var cfg Config
	if err := loader.LoadConfig("sentry", &cfg); err != nil {
		logger.Info().Msg("Sentry configuration not found; issues disabled")
		return
	}

	if strings.TrimSpace(cfg.DSN) == "" {
		logger.Info().Msg("Sentry DSN empty; issues disabled")
		return
	}

	opts := sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		AttachStacktrace: true,
	}

	if cfg.EnableTracing || cfg.TracesSampleRate > 0 {
		opts.EnableTracing = true
		opts.TracesSampleRate = cfg.TracesSampleRate
		if opts.TracesSampleRate <= 0 {
			opts.TracesSampleRate = 1.0
		}
	}

	if err := sentry.Init(opts); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize Sentry")
		return
	}

	logger.Info().Msg("Sentry initialized")
	lifecycle.Append(
		fx.Hook{
			OnStop: func(context.Context) error {
				sentry.Flush(flushTimeout)
				return nil
			},
		},
	)
}
