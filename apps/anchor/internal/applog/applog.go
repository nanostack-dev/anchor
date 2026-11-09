// Package applog builds anchor's slog loggers on the framework's core/log
// handler, with the zerolog line shape the shared collector, the alerts and
// the golden contract suite read (cmd/it/contract/README.md).
//
// Two loggers come out of one handler:
//
//   - Scoped reads the request scope from the context (request_id, method,
//     path, and the caller that auth adds with log.SetAttrs) and the trace ids.
//     The HTTP boundary writes through it: the request summary, the error
//     hooks and the few call sites that used pkg/log's log.Ctx.
//   - The component logger, the plain *slog.Logger of the wiring, writes
//     without the context's attributes, as the injected zerolog logger did.
//     The golden suite pins every line that carries a request_id, so giving
//     these lines the request scope adds lines to the request goldens: that is
//     a deliberate contract change for a later step, not part of this one.
package applog

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/tracing"
)

// Scoped is the logger that reads the request scope from the context. It is
// a distinct type so the wiring can provide it next to the component
// *slog.Logger.
type Scoped struct {
	*slog.Logger
}

// Loggers holds the two loggers built from one configuration.
type Loggers struct {
	// Scoped writes the request scope and trace ids of the context.
	Scoped Scoped
	// Component writes without the context's attributes.
	Component *slog.Logger
	// Identity is the service identity of every line outside a request.
	Identity Identity
}

// Identity is the service identity: env, and service and version when set.
// The handler writes it on every line outside a request. A request's lines
// get it from auth, which adds it to the request scope with log.SetAttrs
// next to the caller, as pkg/log's log.Bind did: the lines of a public route,
// and of a request that auth refuses, have none.
type Identity []slog.Attr

// New builds the loggers of cfg. The identity in cfg is not written by the
// output handler; see Loggers.Identity.
func New(cfg log.Config) Loggers {
	identity := identityAttrs(cfg)
	cfg.Environment, cfg.Service, cfg.Version = "", "", ""
	cfg.ContextAttrs = func(ctx context.Context) []slog.Attr {
		attrs := tracing.LogAttrs(ctx)
		if log.RequestID(ctx) == "" {
			attrs = append(attrs, identity...)
		}
		return attrs
	}
	handler := log.NewHandler(cfg)
	return Loggers{
		Scoped:    Scoped{slog.New(handler)},
		Component: slog.New(detachedHandler{handler}),
		Identity:  identity,
	}
}

// NewLoggers builds the loggers of the bound log settings and installs the
// scoped one as the slog default, so the standard library log package and
// code without an injected logger write through it. Anchor binds the legacy
// wire by default (cmd/app's defaults layer): the zerolog shape is its
// contract until the collector and the alerts read the slog shape.
func NewLoggers(settings log.Settings) (Loggers, error) {
	cfg, err := settings.Config()
	if err != nil {
		return Loggers{}, err
	}
	loggers := New(cfg)
	slog.SetDefault(loggers.Scoped.Logger)
	return loggers, nil
}

func identityAttrs(cfg log.Config) Identity {
	var attrs Identity
	if cfg.Service != "" {
		attrs = append(attrs, slog.String(log.KeyService, cfg.Service))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, slog.String("env", cfg.Environment))
	}
	if cfg.Version != "" {
		attrs = append(attrs, slog.String("version", cfg.Version))
	}
	return attrs
}

// detachedHandler writes a record without the attributes of its context: no
// request scope and no trace ids. See the package comment.
type detachedHandler struct {
	slog.Handler
}

func (h detachedHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.Handler.Enabled(context.Background(), level)
}

func (h detachedHandler) Handle(_ context.Context, r slog.Record) error {
	return h.Handler.Handle(context.Background(), r)
}

func (h detachedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return detachedHandler{h.Handler.WithAttrs(attrs)}
}

func (h detachedHandler) WithGroup(name string) slog.Handler {
	return detachedHandler{h.Handler.WithGroup(name)}
}
