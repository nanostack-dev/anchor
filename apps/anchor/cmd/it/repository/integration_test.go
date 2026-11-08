// Package repository_test is the repository-layer integration suite: real
// Postgres via testcontainers, the production pool (pg/nanopg) and migrator
// (pg/nanomigrate), and only the repositories under test with their mappers
// (no HTTP server, no email/clerk/cache modules).
package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"anchor/internal/mapper"
	"anchor/internal/repository"

	_ "github.com/lib/pq"
	"github.com/nanostack-dev/nanostack-framework/core/secret"
	"github.com/nanostack-dev/nanostack-framework/pg/nanomigrate"
	"github.com/nanostack-dev/nanostack-framework/pg/nanopg"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	repoTestDatabaseName = "anchor_repo_test"
	repoTestStartTimeout = time.Minute
	repoDBStopTimeout    = 10 * time.Second
)

// repositoryTestContext exposes every repository under migration to
// transactor.Page, plus the raw DB handle for fixture backdating.
type repositoryTestContext struct {
	DB                                  *sql.DB
	TenantRepository                    repository.TenantRepository
	ProductRepository                   repository.ProductRepository
	OrganizationRepository              repository.OrganizationRepository
	OrganizationMembershipRepository    repository.OrganizationMembershipRepository
	ProductRoleRepository               repository.ProductRoleRepository
	ProductPermissionRepository         repository.ProductPermissionRepository
	ProductResourcePermissionRepository repository.ProductResourcePermissionRepository
	ProductUserRepository               repository.ProductUserRepository
	OrganizationAPIKeyRepository        repository.OrganizationAPIKeyRepository
	ProductAPIKeyRepository             repository.ProductAPIKeyRepository
	InvitationRepository                repository.InvitationRepository
	PlatformTenantUserRepository        repository.PlatformTenantUserRepository

	pool              *nanopg.DB
	postgresContainer *tcpostgres.PostgresContainer
}

var repoCtx *repositoryTestContext

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), repoTestStartTimeout)

	var err error
	repoCtx, err = setupRepositoryIntegration(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "repository integration setup failed: %v\n", err)
		teardownRepositoryIntegration(repoCtx)
		os.Exit(1)
	}

	code := m.Run()
	teardownRepositoryIntegration(repoCtx)
	os.Exit(code)
}

func setupRepositoryIntegration(ctx context.Context) (*repositoryTestContext, error) {
	container, err := tcpostgres.Run(
		ctx,
		"timescale/timescaledb:2.23.0-pg18",
		// timescaledb-tune panics in a nested Docker daemon (no memory.max).
		testcontainers.WithEnv(map[string]string{"NO_TS_TUNE": "true"}),
		tcpostgres.WithDatabase(repoTestDatabaseName),
		tcpostgres.WithUsername(repoTestDatabaseName),
		tcpostgres.WithPassword(repoTestDatabaseName),
		testcontainers.WithWaitStrategy(
			// The image logs this line once for the init server, then
			// restarts; only the second line means the final server is up
			// (as cmd/it/shared waits for it).
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(time.Minute),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}
	testCtx := &repositoryTestContext{postgresContainer: container}

	cfg, err := repositoryPostgresConfig(ctx, container)
	if err != nil {
		return testCtx, err
	}
	logger := slog.New(slog.DiscardHandler)
	pool, err := nanopg.NewDB(ctx, cfg, logger, noop.NewTracerProvider())
	if err != nil {
		return testCtx, fmt.Errorf("open postgres: %w", err)
	}
	testCtx.pool = pool
	db := nanopg.SQL(pool)

	migrationsPath, err := repositoryProjectRootPath("migrations")
	if err != nil {
		return testCtx, err
	}
	migrations := nanomigrate.DefaultConfig()
	migrations.BasePath = migrationsPath
	if migrateErr := nanomigrate.Run(ctx, nanomigrate.NewMigrator(migrations, db, logger)); migrateErr != nil {
		return testCtx, fmt.Errorf("run migrations: %w", migrateErr)
	}

	testCtx.DB = db
	testCtx.TenantRepository = repository.NewTenantRepository(db, mapper.NewTenantMapper(), logger)
	testCtx.ProductRepository = repository.NewProductRepository(db, mapper.NewProductMapper(), logger)
	testCtx.OrganizationRepository = repository.NewOrganizationRepository(db, mapper.NewOrganizationMapper(), logger)
	testCtx.OrganizationMembershipRepository = repository.NewOrganizationMembershipRepository(db, logger)
	testCtx.ProductRoleRepository = repository.NewProductRoleRepository(db, mapper.NewProductRoleMapper(), logger)
	testCtx.ProductPermissionRepository = repository.NewProductPermissionRepository(
		db, mapper.NewProductPermissionMapper(), logger)
	testCtx.ProductResourcePermissionRepository = repository.NewProductResourcePermissionRepository(
		db, mapper.NewProductResourcePermissionMapper(), logger)
	testCtx.ProductUserRepository = repository.NewProductUserRepository(db, mapper.NewProductUserMapper(), logger)
	testCtx.OrganizationAPIKeyRepository = repository.NewOrganizationAPIKeyRepository(
		db, mapper.NewOrganizationAPIKeyMapper(), logger)
	testCtx.ProductAPIKeyRepository = repository.NewProductAPIKeyRepository(db, mapper.NewProductAPIKeyMapper(), logger)
	testCtx.InvitationRepository = repository.NewPlatformInvitationRepository(db, mapper.NewInvitationMapper(), logger)
	testCtx.PlatformTenantUserRepository = repository.NewPlatformTenantUserRepository(
		db, mapper.NewPlatformUserMapper(), logger)

	return testCtx, nil
}

func teardownRepositoryIntegration(ctx *repositoryTestContext) {
	if ctx == nil {
		return
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), repoDBStopTimeout)
	defer cancel()
	if ctx.pool != nil {
		_ = ctx.pool.Stop(stopCtx)
	}
	if ctx.postgresContainer != nil {
		_ = testcontainers.TerminateContainer(ctx.postgresContainer)
	}
}

func repositoryPostgresConfig(ctx context.Context, container *tcpostgres.PostgresContainer) (nanopg.Config, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return nanopg.Config{}, fmt.Errorf("postgres host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nanopg.Config{}, fmt.Errorf("postgres port: %w", err)
	}
	portNumber, err := strconv.Atoi(port.Port())
	if err != nil {
		return nanopg.Config{}, fmt.Errorf("postgres port: %w", err)
	}

	cfg := nanopg.DefaultConfig()
	cfg.Host = host
	cfg.Port = portNumber
	cfg.User = repoTestDatabaseName
	cfg.Password = secret.New(repoTestDatabaseName)
	cfg.DB = repoTestDatabaseName
	cfg.SSLMode = "disable"
	return cfg, nil
}

func repositoryProjectRootPath(parts ...string) (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	segments := append([]string{root}, parts...)
	return filepath.Join(segments...), nil
}
