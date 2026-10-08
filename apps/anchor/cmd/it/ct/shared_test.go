package ct_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"
	"anchor/cmd/it/shared/mailpit"
	stripeprovider "anchor/internal/integration/provider/stripe"
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
	StripeProvider   *stripeprovider.Provider
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
			ExtraPopulateTargets: []any{
				&authCfg,
				&EventQueue,
				&IntegrationRepo,
				&StripeProvider,
				&reconcileQueue,
				&testDB,
				&adjustmentBackfill,
				&templateSync,
			},
			AfterRun: mailpit.StopShared,
		},
	)
}
