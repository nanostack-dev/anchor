// Package engine runs OpenFGA inside the Anchor process, on Anchor's Postgres.
//
// OpenFGA keeps its tables in the openfga schema (created by an Anchor
// migration) and owns them: its own migrations run here at startup, before the
// server opens. Nothing else in Anchor reads those tables.
package engine

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	frameworkconfig "github.com/nanostack-dev/nanostack-framework/modules/config"
	frameworkpostgres "github.com/nanostack-dev/nanostack-framework/modules/postgres"
	"github.com/nanostack-dev/pgkit/pglock"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
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
	migrationLockKey = "openfga-migrations"
	maxOpenConns     = 10
	maxIdleConns     = 5
	connMaxIdleTime  = 5 * time.Minute
)

// Engine is OpenFGA's own service API, served in-process. Callers depend on
// openfgav1.OpenFGAServiceServer, so moving OpenFGA to its own deployment later
// only changes what this package returns.
type Engine = openfgav1.OpenFGAServiceServer

type Params struct {
	fx.In
	Lifecycle fx.Lifecycle
	Loader    frameworkconfig.Loader
	DB        *sql.DB
	Logger    zerolog.Logger
}

// New migrates OpenFGA's schema, then starts the server on its datastore. Both
// are released when the application stops. It runs while the app is built,
// after Anchor's own migrations, which create the openfga schema.
func New(params Params) (Engine, error) {
	var pg frameworkpostgres.Config
	if err := params.Loader.LoadConfig("postgres", &pg); err != nil {
		return nil, fmt.Errorf("load postgres config for openfga: %w", err)
	}
	uri := datastoreURI(pg)

	if err := migrateExclusively(params.DB, uri); err != nil {
		return nil, fmt.Errorf("migrate openfga schema: %w", err)
	}

	datastore, err := postgres.New(uri, sqlcommon.NewConfig(
		sqlcommon.WithMaxOpenConns(maxOpenConns),
		sqlcommon.WithMaxIdleConns(maxIdleConns),
		sqlcommon.WithConnMaxIdleTime(connMaxIdleTime),
	))
	if err != nil {
		return nil, fmt.Errorf("open openfga datastore: %w", err)
	}

	// The check cache stays off: Anchor promises that a write applies to the
	// next evaluation, and a cached answer could outlive the write.
	openfga, err := server.NewServerWithOpts(
		server.WithDatastore(datastore),
		server.WithCheckQueryCacheEnabled(false),
	)
	if err != nil {
		datastore.Close()
		return nil, fmt.Errorf("start openfga: %w", err)
	}

	params.Lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		openfga.Close()
		datastore.Close()
		return nil
	}})
	params.Logger.Info().Str("schema", schema).Msg("openfga started in-process")
	return openfga, nil
}

// migrateExclusively runs OpenFGA's migrations while holding a session advisory
// lock, because replicas start together and OpenFGA's migrator takes no lock
// of its own.
func migrateExclusively(db *sql.DB, uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	key := pglock.KeyHash(migrationLockKey)
	if _, lockErr := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); lockErr != nil {
		return fmt.Errorf("lock openfga migrations: %w", lockErr)
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", key) }()
	return migrate.RunMigrations(migrate.MigrationConfig{
		Engine:      datastoreEngine,
		URI:         uri,
		Timeout:     migrationTimeout,
		PingTimeout: pingTimeout,
	})
}

// datastoreURI points OpenFGA at Anchor's database with its own schema first on
// the search path, so its unqualified table names land in that schema.
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
