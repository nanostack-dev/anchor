package ct_test

import (
	"context"
	"testing"

	"anchor/cmd/app"

	itshared "anchor/cmd/it/shared"
	"anchor/cmd/it/shared/mailpit"
	"anchor/internal/repository"
	"anchor/internal/service"
	"anchor/internal/service/config"
	sessionservice "anchor/internal/session/service"

	"github.com/nanostack-dev/pgkit/queue"
)

var (
	TenantRepository repository.TenantRepository
	UserRepository   repository.UserRepository
	PlatformUserRepo repository.PlatformTenantUserRepository
	TokenHelper      service.JWTHelper
	SessionSvc       sessionservice.Service
	authCfg          config.AuthConfig
	ProductRepo      repository.ProductRepository
	PermissionRepo   repository.ProductPermissionRepository
	ProductAPIKeySvc service.ProductAPIKeyService
	ProductUserRepo  repository.ProductUserRepository
	OrgMemberRepo    repository.OrganizationMembershipRepository
	EventQueue       *queue.Client
	IntegrationRepo  repository.IntegrationInstanceRepository
)

func TestMain(m *testing.M) {
	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:             true,
			PopulateRepositories:    true,
			APIKeyService:           &ProductAPIKeySvc,
			PermissionRepository:    &PermissionRepo,
			ProductRepository:       &ProductRepo,
			ProductUserRepository:   &ProductUserRepo,
			OrgMembershipRepository: &OrgMemberRepo,
			TenantRepository:        &TenantRepository,
			UserRepository:          &UserRepository,
			PlatformUserRepository:  &PlatformUserRepo,
			JWTHelper:               &TokenHelper,
			SessionService:          &SessionSvc,
			Populate: func(c *app.Components) {
				authCfg = c.AuthConfig
				EventQueue = c.QueueClient
				IntegrationRepo = c.IntegrationInstanceRepository
				reconcileQueue = c.QueueClient
				testDB = c.SQLDB
				templateSync = c.LicenseTemplateSyncService
				authzStores = c.StoreStores
			},
			AfterRun: mailpit.StopShared,
			// OpenFGA must hold what Anchor's tables imply once the suite ran
			// (ADR-0022 E5).
			CheckAfterRun: func() error {
				return checkAuthzSync(context.Background())
			},
		},
	)
}
