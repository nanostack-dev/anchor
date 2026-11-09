// Package app wires anchor: its configuration (Config, bound by the
// generated Bind) and its components (the manifest in manifest.go, built by
// the generated Build). Regenerate both after a module changes a provider or
// a config field:
//
//	go generate ./cmd/app
package app

//go:generate go tool nano gen config -dir . -env ../../.env.example
//go:generate go tool nano gen wire -dir . -out .

import (
	_ "embed"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/core/nano"
	"github.com/nanostack-dev/nanostack-framework/otel/nanotrace"
	"github.com/nanostack-dev/nanostack-framework/pg/nanomigrate"
	"github.com/nanostack-dev/nanostack-framework/pg/nanopg"
	"github.com/nanostack-dev/nanostack-framework/redis/nanocache"

	httpserver "anchor/cmd/http"
	serviceconfig "anchor/internal/service/config"
)

// Config is anchor's configuration. Each section names the env variables
// that the deployed stacks set: POSTGRES_*, REDIS_*, SERVER_PORT, LOG_*,
// OTEL_* and the core settings' own names (see CONFIG.md).
type Config struct {
	App        nano.AppConfig           `config:"app"`
	Log        log.Settings             `config:"log"`
	Tracing    nanotrace.Config         `config:"tracing"`
	Postgres   nanopg.Config            `config:"postgres"`
	Migrations nanomigrate.Config       `config:"migrations"`
	Redis      nanocache.Config         `config:"redis"`
	Server     httpserver.Config        `config:"server"`
	Core       serviceconfig.CoreConfig `config:"core"`
}

// defaults is the lowest configuration layer, compiled in: anchor's defaults
// that differ from the framework's. The legacy log wire is anchor's log
// contract until the collector and the alerts read the slog shape;
// LOG_WIRE=slog opts out.
//
//go:embed defaults.yaml
var defaults []byte
