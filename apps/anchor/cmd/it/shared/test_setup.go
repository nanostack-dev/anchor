package itshared

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jaswdr/faker/v2"

	"anchor/cmd/app"
	"anchor/internal/repository"
	"anchor/internal/service"
	sessionservice "anchor/internal/session/service"

	_ "github.com/lib/pq" // Required for PostgreSQL driver
	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	TestLogger                   *slog.Logger                                //nolint:gochecknoglobals // Required for test setup
	ServerURL                    string                                      //nolint:gochecknoglobals // Required for test setup
	APIKeyService                service.ProductAPIKeyService                //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	OrganizationAPIKeyService    service.OrganizationAPIKeyService           //nolint:gochecknoglobals // Required for org API key validation tests
	PermissionRepository         repository.ProductPermissionRepository      //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	OrganizationAPIKeyRepository repository.OrganizationAPIKeyRepository     //nolint:gochecknoglobals // Required for org API key validation tests
	OrganizationRepository       repository.OrganizationRepository           //nolint:gochecknoglobals // Required for org-scoped service tests
	ProductRepository            repository.ProductRepository                //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	ProductUserRepository        repository.ProductUserRepository            //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	OrgMembershipRepository      repository.OrganizationMembershipRepository //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	TenantRepository             repository.TenantRepository                 //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	UserRepository               repository.UserRepository                   //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	PlatformTenantUserRepo       repository.PlatformTenantUserRepository     //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	JWTHelper                    service.JWTHelper                           //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	SessionService               sessionservice.Service                      //nolint:gochecknoglobals // Required for DSL/repo-backed tests
	setupOnce                    sync.Once                                   //nolint:gochecknoglobals,unused // Required for test setup
	postgresContainer            testcontainers.Container                    //nolint:gochecknoglobals // Required for test setup
	redisContainer               testcontainers.Container                    //nolint:gochecknoglobals // Required for test setup
	Faker                        = faker.New()                               //nolint:gochecknoglobals // Required for test setup
)

const postgresReadyLogOccurrences = 2

const defaultTestEncryptionKey = "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA="

const defaultTestPostgresValue = "anchor"

type TestConfig struct {
	EnableRedis                  bool
	PopulateRepositories         bool
	APIKeyService                *service.ProductAPIKeyService
	OrganizationAPIKeyService    *service.OrganizationAPIKeyService
	APIKeyRepository             *repository.ProductAPIKeyRepository
	OrganizationAPIKeyRepository *repository.OrganizationAPIKeyRepository
	OrganizationRepository       *repository.OrganizationRepository
	PermissionRepository         *repository.ProductPermissionRepository
	ProductRepository            *repository.ProductRepository
	ProductUserRepository        *repository.ProductUserRepository
	OrgMembershipRepository      *repository.OrganizationMembershipRepository
	TenantRepository             *repository.TenantRepository
	UserRepository               *repository.UserRepository
	PlatformUserRepository       *repository.PlatformTenantUserRepository
	JWTHelper                    *service.JWTHelper
	SessionService               *sessionservice.Service
	// ExtraPopulateTargets holds additional fx.Populate targets (e.g. *queue.Client)
	// for test packages that need direct access to FX-provided values beyond the standard
	// repository/service set. Each entry must be a pointer to the destination variable.
	ExtraPopulateTargets []any
	AfterInit            func()
	// AfterRun runs once the package's tests have finished, before the shared
	// containers are torn down. Packages use it to release their own process-wide
	// resources (e.g. mailpit.StopShared); a defer in TestMain would never run
	// because RunTestMain exits the process.
	AfterRun func()
	// CheckAfterRun runs once the package's tests passed; an error fails the
	// package, for a rule that holds over the whole suite.
	CheckAfterRun func() error
}

func SetupTest(config TestConfig) func() {
	logLevel := slog.LevelInfo
	if levelStr := os.Getenv("TEST_LOG_LEVEL"); levelStr != "" {
		if lvl, err := log.ParseLevel(levelStr); err == nil {
			logLevel = lvl
		}
	}
	TestLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: true, Level: logLevel})).
		With(slog.String("service", "shared_test_setup"))
	testLogger := TestLogger
	testLogger.Info("Setting up shared resources...")

	configPath := filepath.Join(".", "application.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic(fmt.Sprintf("application.yaml not found at expected path '%s': %v", configPath, err))
	}
	if err := os.Setenv("CONFIG_PATH", configPath); err != nil {
		panic(fmt.Sprintf("Failed to set CONFIG_PATH: %v", err))
	}
	if err := os.Setenv("LOG_REQUEST_BODY", "true"); err != nil {
		panic(fmt.Sprintf("Failed to set LOG_REQUEST_BODY: %v", err))
	}
	if strings.TrimSpace(os.Getenv("APP_ENCRYPTION_KEY")) == "" {
		if err := os.Setenv("APP_ENCRYPTION_KEY", defaultTestEncryptionKey); err != nil {
			panic(fmt.Sprintf("Failed to set APP_ENCRYPTION_KEY: %v", err))
		}
	}
	testLogger.Info("Set NANOSTACK_CONFIG_PATH for application", slog.String("path", configPath))

	ctx := context.Background()
	var teardownFuncs []func()

	postgresContainer = setupPostgres(ctx, testLogger)
	teardownFuncs = append(
		teardownFuncs, func() {
			if err := postgresContainer.Terminate(ctx); err != nil {
				testLogger.ErrorContext(ctx, "Failed to terminate postgres container", log.Err(err))
			} else {
				testLogger.InfoContext(ctx, "Postgres container terminated")
			}
		},
	)

	if config.EnableRedis {
		redisContainer = setupRedis(ctx, testLogger)
		teardownFuncs = append(
			teardownFuncs, func() {
				if err := redisContainer.Terminate(ctx); err != nil {
					testLogger.ErrorContext(ctx, "Failed to terminate redis container", log.Err(err))
				} else {
					testLogger.InfoContext(ctx, "Redis container terminated")
				}
			},
		)
	}

	setupAppServer(testLogger, config)

	teardown := func() {
		testLogger.InfoContext(ctx, "Tearing down shared resources...")
		for _, teardownFunc := range slices.Backward(teardownFuncs) {
			teardownFunc()
		}
		testLogger.InfoContext(ctx, "Shared resources teardown complete")
	}

	testLogger.InfoContext(ctx, "Shared resources setup complete")
	return teardown
}

func setupPostgres(ctx context.Context, testLogger *slog.Logger) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:        "timescale/timescaledb:2.23.0-pg18",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").WithOccurrence(postgresReadyLogOccurrences),
		).
			WithDeadline(2 * time.Minute), //nolint:mnd // Reasonable timeout for container startup

		Env: map[string]string{
			"POSTGRES_DB":       defaultTestPostgresValue,
			"POSTGRES_USER":     defaultTestPostgresValue,
			"POSTGRES_PASSWORD": defaultTestPostgresValue,
			// timescaledb-tune reads the container's memory.max, which a nested
			// Docker daemon (CI's Dagger engine) does not provide; it panics and
			// Postgres exits with code 2. The tests need no tuning.
			"NO_TS_TUNE": "true",
		},
	}
	container, err := testcontainers.GenericContainer(
		ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to start postgres container: %v", err))
	}

	host, err := container.Host(ctx)
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			testLogger.ErrorContext(ctx, "Failed to terminate container after host error", log.Err(termErr))
		}
		panic(fmt.Sprintf("Failed to get container host: %v", err))
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			testLogger.ErrorContext(ctx, "Failed to terminate container after port error", log.Err(termErr))
		}
		panic(fmt.Sprintf("Failed to get container port: %v", err))
	}
	if setenvErr := os.Setenv("POSTGRES_HOST", host); setenvErr != nil {
		panic(fmt.Sprintf("Failed to set POSTGRES_HOST: %v", setenvErr))
	}
	if setenvErr := os.Setenv("POSTGRES_PORT", port.Port()); setenvErr != nil {
		panic(fmt.Sprintf("Failed to set POSTGRES_PORT: %v", setenvErr))
	}
	testLogger.InfoContext(ctx, "PostgreSQL container started",
		slog.String("host", host),
		slog.String("port", port.Port()),
	)

	return container
}

func setupRedis(ctx context.Context, testLogger *slog.Logger) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForListeningPort("6379/tcp").
			WithStartupTimeout(2 * time.Minute), //nolint:mnd // Reasonable timeout for container startup
	}
	container, err := testcontainers.GenericContainer(
		ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to start redis container: %v", err))
	}

	host, err := container.Host(ctx)
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			testLogger.ErrorContext(ctx, "Failed to terminate redis container after host error", log.Err(termErr))
		}
		panic(fmt.Sprintf("Failed to get redis container host: %v", err))
	}
	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			testLogger.ErrorContext(ctx, "Failed to terminate redis container after port error", log.Err(termErr))
		}
		panic(fmt.Sprintf("Failed to get redis container port: %v", err))
	}
	if setenvErr := os.Setenv(
		"REDIS_ADDRESS", fmt.Sprintf("%s:%s", host, port.Port()),
	); setenvErr != nil {
		panic(fmt.Sprintf("Failed to set REDIS_ADDRESS: %v", setenvErr))
	}
	if setenvErr := os.Setenv("REDIS_PASSWORD", ""); setenvErr != nil {
		panic(fmt.Sprintf("Failed to set REDIS_PASSWORD: %v", setenvErr))
	}
	if setenvErr := os.Setenv("REDIS_DB", "0"); setenvErr != nil {
		panic(fmt.Sprintf("Failed to set REDIS_DB: %v", setenvErr))
	}
	testLogger.InfoContext(ctx, "Redis container started",
		slog.String("host", host),
		slog.String("port", port.Port()),
	)

	return container
}

// buildPopulateTargets returns the fx.Populate target slice for the given config.
// It is extracted here to keep setupAppServer under the gocognit limit.
func buildPopulateTargets(config TestConfig) []any {
	var targets []any
	if config.PopulateRepositories {
		maybeTargets := []any{
			config.APIKeyService,
			config.OrganizationAPIKeyService,
			config.APIKeyRepository,
			config.OrganizationAPIKeyRepository,
			config.OrganizationRepository,
			config.PermissionRepository,
			config.ProductRepository,
			config.ProductUserRepository,
			config.OrgMembershipRepository,
			config.TenantRepository,
			config.UserRepository,
			config.PlatformUserRepository,
			config.JWTHelper,
			config.SessionService,
		}
		for _, target := range maybeTargets {
			if target != nil && isValidPopulateTarget(target) {
				targets = append(targets, target)
			}
		}
	}
	targets = append(targets, config.ExtraPopulateTargets...)
	return targets
}

func isValidPopulateTarget(target any) bool {
	value := reflect.ValueOf(target)
	return value.Kind() == reflect.Pointer && !value.IsNil()
}

// startApp launches the Anchor FX application with the appropriate populate
// targets derived from config.
func startApp(config TestConfig) {
	targets := buildPopulateTargets(config)
	if len(targets) > 0 {
		app.StartAnchorWithPopulate(targets...)
	} else {
		app.StartAnchor()
	}
}

func setupAppServer(testLogger *slog.Logger, config TestConfig) {
	randomAppPort := strconv.Itoa(mustAllocateFreePort())
	if err := os.Setenv("SERVER_PORT", randomAppPort); err != nil {
		panic(fmt.Sprintf("Failed to set SERVER_PORT: %v", err))
	}
	testLogger.Info("Assigned app server port", slog.String("port", randomAppPort))

	go func() {
		testLogger.Info("Starting Anchor application...")
		startApp(config)
		testLogger.Info("Anchor application goroutine finished")
	}()

	ServerURL = "http://localhost:" + os.Getenv("SERVER_PORT")
	testLogger.Info("Waiting for server health check...", slog.String("url", ServerURL))

	ok := false
	for range 120 {
		req, reqErr := http.NewRequestWithContext(
			context.Background(), http.MethodGet, ServerURL+"/health", nil,
		)
		if reqErr != nil {
			testLogger.Debug("Health check request error", log.Err(reqErr))
			time.Sleep(time.Second)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			ok = true
			if closeErr := resp.Body.Close(); closeErr != nil {
				testLogger.Error("Failed to close response body", log.Err(closeErr))
			}
			break
		}
		if resp != nil {
			testLogger.Debug("Health check failed", slog.Int("status", resp.StatusCode))
			if closeErr := resp.Body.Close(); closeErr != nil {
				testLogger.Error("Failed to close response body", log.Err(closeErr))
			}
		} else if err != nil {
			testLogger.Debug("Health check connection error", log.Err(err))
		}
		time.Sleep(time.Second)
	}
	if !ok {
		panic("Server did not become healthy in time")
	}

	testLogger.Info("Server health check successful")
}

func mustAllocateFreePort() int {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("Failed to allocate free port: %v", err))
	}
	defer func() {
		_ = listener.Close()
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("Unexpected listener address type: %T", listener.Addr()))
	}

	return addr.Port
}

func RunTestMain(m *testing.M, config TestConfig) {
	teardown := SetupTest(config)
	if config.APIKeyService != nil {
		APIKeyService = *config.APIKeyService
	}
	if config.OrganizationAPIKeyService != nil {
		OrganizationAPIKeyService = *config.OrganizationAPIKeyService
	}
	if config.OrganizationAPIKeyRepository != nil {
		OrganizationAPIKeyRepository = *config.OrganizationAPIKeyRepository
	}
	if config.OrganizationRepository != nil {
		OrganizationRepository = *config.OrganizationRepository
	}
	if config.PermissionRepository != nil {
		PermissionRepository = *config.PermissionRepository
	}
	if config.ProductRepository != nil {
		ProductRepository = *config.ProductRepository
	}
	if config.ProductUserRepository != nil {
		ProductUserRepository = *config.ProductUserRepository
	}
	if config.OrgMembershipRepository != nil {
		OrgMembershipRepository = *config.OrgMembershipRepository
	}
	if config.TenantRepository != nil {
		TenantRepository = *config.TenantRepository
	}
	if config.UserRepository != nil {
		UserRepository = *config.UserRepository
	}
	if config.PlatformUserRepository != nil {
		PlatformTenantUserRepo = *config.PlatformUserRepository
	}
	if config.JWTHelper != nil {
		JWTHelper = *config.JWTHelper
	}
	if config.SessionService != nil {
		SessionService = *config.SessionService
	}
	if config.AfterInit != nil {
		config.AfterInit()
	}
	exitCode := m.Run()
	if exitCode == 0 && config.CheckAfterRun != nil {
		if err := config.CheckAfterRun(); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			exitCode = 1
		}
	}
	if config.AfterRun != nil {
		config.AfterRun()
	}
	if teardown != nil {
		teardown()
	}
	os.Exit(exitCode)
}
