// Package engine runs OpenFGA inside the Anchor process, on Anchor's Postgres
// (ADR-0021). OpenFGA owns its tables in the openfga schema, which an Anchor
// migration creates; its own migrations run here, before its server opens.
// Nothing else in Anchor reads those tables.
//
// The engine starts beside the application, not before it: a slow or failing
// OpenFGA migration never keeps Anchor from serving. Until it is ready, Get
// answers ErrNotReady, and whatever needed it is retried later.
package engine

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	frameworkconfig "github.com/nanostack-dev/nanostack-framework/modules/config"
	frameworkpostgres "github.com/nanostack-dev/nanostack-framework/modules/postgres"
	"github.com/nanostack-dev/pgkit/pglock"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	openfgalogger "github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/migrate"
	"github.com/openfga/openfga/pkg/storage/postgres"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	schema           = "openfga"
	datastoreEngine  = "postgres"
	migrationTimeout = time.Minute
	pingTimeout      = 10 * time.Second
	unlockTimeout    = 5 * time.Second
	lockPollInterval = 500 * time.Millisecond
	migrationLockKey = "openfga-migrations"
	maxOpenConns     = 10
	connMaxIdleTime  = 5 * time.Minute
	// maxModelBytes lifts OpenFGA's 256 KiB model limit: a model grows by
	// about half a kilobyte per permission and type that holds it, so a large
	// catalog of a thousand permission placements still fits.
	maxModelBytes = 1 << 20
	retryMin      = time.Second
	retryMax      = 30 * time.Second
	retryGrowth   = 2
)

// ErrNotReady is returned until OpenFGA's migrations ran and its server opened.
var ErrNotReady = errors.New("openfga is not ready")

// Engine is OpenFGA's own service API, served in-process. Callers depend on
// openfgav1.OpenFGAServiceServer, so running OpenFGA as its own deployment
// later changes only this package.
type Engine = openfgav1.OpenFGAServiceServer

// Provider hands out the engine once it is ready.
type Provider interface {
	Get() (Engine, error)
}

type Params struct {
	fx.In
	Lifecycle fx.Lifecycle
	Loader    frameworkconfig.Loader
	DB        *sql.DB
	Logger    zerolog.Logger
}

type provider struct {
	db     *sql.DB
	uri    string
	logger zerolog.Logger
	ready  atomic.Pointer[running]
}

type running struct {
	server    *server.Server
	datastore interface{ Close() }
}

func (p *provider) Get() (Engine, error) {
	current := p.ready.Load()
	if current == nil {
		return nil, ErrNotReady
	}
	return current.server, nil
}

// New starts OpenFGA in the background once the application starts, retrying
// until it is up, and stops it with the application. It runs after Anchor's
// own migrations, which create the openfga schema.
func New(params Params) (Provider, error) {
	var pg frameworkpostgres.Config
	if err := params.Loader.LoadConfig("postgres", &pg); err != nil {
		return nil, fmt.Errorf("load postgres config for openfga: %w", err)
	}
	p := &provider{
		db: params.DB, uri: datastoreURI(pg),
		logger: params.Logger.With().Str("component", "openfga").Logger(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				p.startWithRetry(ctx)
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			select {
			case <-done:
			case <-stopCtx.Done():
			}
			if current := p.ready.Load(); current != nil {
				current.server.Close()
				current.datastore.Close()
			}
			return nil
		},
	})
	return p, nil
}

func (p *provider) startWithRetry(ctx context.Context) {
	backoff := retryMin
	for {
		err := p.start(ctx)
		if err == nil {
			p.logger.Info().Str("schema", schema).Msg("openfga started in-process")
			return
		}
		p.logger.Error().Err(err).Dur("retry_in", backoff).Msg("openfga failed to start")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(retryGrowth*backoff, retryMax)
	}
}

func (p *provider) start(ctx context.Context) error {
	if err := p.migrateExclusively(ctx); err != nil {
		return fmt.Errorf("migrate openfga schema: %w", err)
	}
	datastore, err := postgres.New(p.uri, sqlcommon.NewConfig(
		sqlcommon.WithMaxOpenConns(maxOpenConns),
		sqlcommon.WithConnMaxIdleTime(connMaxIdleTime),
	))
	if err != nil {
		return fmt.Errorf("open openfga datastore: %w", err)
	}
	logger, err := openfgalogger.NewLogger(openfgalogger.WithFormat("json"), openfgalogger.WithLevel("warn"))
	if err != nil {
		datastore.Close()
		return fmt.Errorf("build openfga logger: %w", err)
	}
	// The check cache stays off: a write must apply to the next decision, and
	// a cached answer could outlive it.
	openfga, err := server.NewServerWithOpts(
		server.WithDatastore(datastore),
		server.WithLogger(logger),
		server.WithCheckQueryCacheEnabled(false),
		server.WithMaxAuthorizationModelSizeInBytes(maxModelBytes),
	)
	if err != nil {
		datastore.Close()
		return fmt.Errorf("start openfga: %w", err)
	}
	p.ready.Store(&running{server: openfga, datastore: datastore})
	return nil
}

// migrateExclusively runs OpenFGA's migrations under a session advisory lock:
// replicas start together, and OpenFGA's migrator takes no lock of its own.
func (p *provider) migrateExclusively(ctx context.Context) error {
	lockCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	conn, err := p.db.Conn(lockCtx)
	if err != nil {
		return err
	}
	defer conn.Close()
	key := pglock.KeyHash(migrationLockKey)
	if err = lockPolling(lockCtx, conn, key); err != nil {
		return fmt.Errorf("lock openfga migrations: %w", err)
	}
	defer unlock(conn, key)
	if err = migrate.RunMigrations(migrate.MigrationConfig{
		Engine:      datastoreEngine,
		URI:         p.uri,
		Timeout:     migrationTimeout,
		PingTimeout: pingTimeout,
	}); err != nil {
		return err
	}
	if err = p.repairInvalidIndexes(ctx); err != nil {
		return fmt.Errorf("repair openfga indexes: %w", err)
	}
	return nil
}

// lockPolling waits for the session lock between short tries rather than inside
// pg_advisory_lock: a statement blocked there holds a snapshot, and the lock
// holder's CREATE INDEX CONCURRENTLY would wait for it until lockCtx expires.
func lockPolling(ctx context.Context, conn *sql.Conn, key int64) error {
	for {
		var acquired bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
			return err
		}
		if acquired {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockPollInterval):
		}
	}
}

// unlock releases the session lock; a connection that can't is dropped, so it
// never goes back to the pool still holding the lock.
func unlock(conn *sql.Conn, key int64) {
	ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
	defer cancel()
	var released bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", key).Scan(&released); err == nil && released {
		return
	}
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}

// repairInvalidIndexes rebuilds any index of the openfga schema that a
// concurrent build left invalid, such as one interrupted by a restart: OpenFGA
// builds some of its indexes concurrently, and a rerun skips an index that
// exists even when it is invalid.
func (p *provider) repairInvalidIndexes(ctx context.Context) error {
	rows, err := p.db.QueryContext(ctx, `SELECT c.relname FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND NOT i.indisvalid`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	var invalid []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return err
		}
		invalid = append(invalid, name)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, name := range invalid {
		p.logger.Warn().Str("index", name).Msg("rebuilding an invalid openfga index")
		if _, err = p.db.ExecContext(ctx, "REINDEX INDEX CONCURRENTLY "+schema+"."+quoteIdentifier(name)); err != nil {
			return fmt.Errorf("reindex %s: %w", name, err)
		}
	}
	return nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// datastoreURI points OpenFGA at Anchor's database with the openfga schema as
// its only search path, so its unqualified table names land there.
func datastoreURI(pg frameworkpostgres.Config) string {
	query := url.Values{}
	query.Set("search_path", schema)
	if pg.SSLMode != "" {
		query.Set("sslmode", pg.SSLMode)
	}
	uri := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(pg.User, pg.Password),
		Host:     net.JoinHostPort(pg.Host, strconv.Itoa(pg.Port)),
		Path:     "/" + pg.DBName,
		RawQuery: query.Encode(),
	}
	return uri.String()
}
