// Package repository_test is the repository-layer integration suite: real
// Postgres via testcontainers, only the repository + mapper layers wired
// (no HTTP server, no email/clerk/cache modules) — mirrors echopoint's
// cmd/it/repository harness, the canonical shape for this test type.
package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"anchor/internal/mapper"
	"anchor/internal/repository"

	"github.com/golang-migrate/migrate/v4"
	mgpostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/modules/migrations"
	"github.com/nanostack-dev/nanostack-framework/modules/postgres"
	"github.com/nanostack-dev/nanostack-framework/modules/transactor"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	repoTestDatabaseName = "anchor_repo_test"
	repoTestStartTimeout = time.Minute
	repoAppStartTimeout  = 30 * time.Second
	repoAppStopTimeout   = 10 * time.Second
)

// repositoryTestContext exposes every repository under migration to
// transactor.Page, plus the raw DB handle for fixture backdating.
type repositoryTestContext struct {
	DB                                  *sql.DB
	TenantRepository                    repository.TenantRepository
	ProductRepository                   repository.ProductRepository
	OrganizationRepository              repository.OrganizationRepository
	WorkspaceRepository                 repository.WorkspaceRepository
	OrganizationMembershipRepository    repository.OrganizationMembershipRepository
	ProductRoleRepository               repository.ProductRoleRepository
	ProductPermissionRepository         repository.ProductPermissionRepository
	ProductResourcePermissionRepository repository.ProductResourcePermissionRepository
	ProductUserRepository               repository.ProductUserRepository
	OrganizationAPIKeyRepository        repository.OrganizationAPIKeyRepository
	ProductAPIKeyRepository             repository.ProductAPIKeyRepository
	InvitationRepository                repository.InvitationRepository
	PlatformTenantUserRepository        repository.PlatformTenantUserRepository

	app               *fx.App
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
		os.Exit(1)
	}

	code := m.Run()
	teardownRepositoryIntegration(repoCtx)
	os.Exit(code)
}

func setupRepositoryIntegration(ctx context.Context) (*repositoryTestContext, error) {
	projectRoot, err := repositoryProjectRootPath()
	if err != nil {
		return nil, err
	}
	if chdirErr := os.Chdir(projectRoot); chdirErr != nil {
		return nil, fmt.Errorf("change working directory to project root: %w", chdirErr)
	}
	// Use this suite's own test config instead of the root application.yaml, whose
	// ${file:*_FILE} secret indirections would demand secret files in test runs.
	_ = os.Setenv("CONFIG_PATH", filepath.Join(projectRoot, "cmd", "it", "repository", "application.yaml"))

	container, err := tcpostgres.Run(
		ctx,
		"timescale/timescaledb:2.23.0-pg18",
		tcpostgres.WithDatabase(repoTestDatabaseName),
		tcpostgres.WithUsername(repoTestDatabaseName),
		tcpostgres.WithPassword(repoTestDatabaseName),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithStartupTimeout(time.Minute),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("postgres connection string: %w", err)
	}

	if envErr := setRepositoryTestEnv(ctx, container); envErr != nil {
		return nil, envErr
	}
	if waitErr := waitForRepositoryDatabase(connStr); waitErr != nil {
		return nil, waitErr
	}

	migrationsPath, err := repositoryProjectRootPath("migrations")
	if err != nil {
		return nil, err
	}
	if migrateErr := runRepositoryMigrations(connStr, migrationsPath); migrateErr != nil {
		return nil, migrateErr
	}

	testCtx := &repositoryTestContext{}

	app := fx.New(
		logging.Module,
		config.Module,
		postgres.Module,
		transactor.Module,
		migrations.Module,
		mapper.NewModule(),
		repository.NewModule(),
		fx.Populate(
			&testCtx.DB,
			&testCtx.TenantRepository,
			&testCtx.ProductRepository,
			&testCtx.OrganizationRepository,
			&testCtx.WorkspaceRepository,
			&testCtx.OrganizationMembershipRepository,
			&testCtx.ProductRoleRepository,
			&testCtx.ProductPermissionRepository,
			&testCtx.ProductResourcePermissionRepository,
			&testCtx.ProductUserRepository,
			&testCtx.OrganizationAPIKeyRepository,
			&testCtx.ProductAPIKeyRepository,
			&testCtx.InvitationRepository,
			&testCtx.PlatformTenantUserRepository,
		),
	)

	startCtx, startCancel := context.WithTimeout(context.Background(), repoAppStartTimeout)
	defer startCancel()
	if startErr := app.Start(startCtx); startErr != nil {
		return nil, fmt.Errorf("start fx app: %w", startErr)
	}

	testCtx.app = app
	testCtx.postgresContainer = container

	return testCtx, nil
}

func teardownRepositoryIntegration(ctx *repositoryTestContext) {
	if ctx == nil {
		return
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), repoAppStopTimeout)
	defer cancel()
	if ctx.app != nil {
		_ = ctx.app.Stop(stopCtx)
	}
	if ctx.postgresContainer != nil {
		_ = testcontainers.TerminateContainer(ctx.postgresContainer)
	}
}

func setRepositoryTestEnv(ctx context.Context, container *tcpostgres.PostgresContainer) error {
	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("postgres host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return fmt.Errorf("postgres port: %w", err)
	}

	_ = os.Setenv("POSTGRES_HOST", host)
	_ = os.Setenv("POSTGRES_PORT", port.Port())
	_ = os.Setenv("POSTGRES_USER", repoTestDatabaseName)
	_ = os.Setenv("POSTGRES_PASSWORD", repoTestDatabaseName)
	_ = os.Setenv("POSTGRES_DB", repoTestDatabaseName)
	_ = os.Setenv("POSTGRES_SSLMODE", "disable")
	_ = os.Setenv("ENVIRONMENT", "test")

	return nil
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

func runRepositoryMigrations(databaseURL, migrationsPath string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	driver, err := mgpostgres.WithInstance(db, &mgpostgres.Config{MigrationsTable: "schema_migrations"})
	if err != nil {
		return fmt.Errorf("migration postgres driver: %w", err)
	}

	migrator, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if upErr := migrator.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", upErr)
	}

	return nil
}

func waitForRepositoryDatabase(databaseURL string) error {
	var lastErr error
	for range 20 {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}

		pingErr := db.Ping()
		_ = db.Close()
		if pingErr == nil {
			return nil
		}

		lastErr = pingErr
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("wait for postgres readiness: %w", lastErr)
}
