package ct_test

import (
	"fmt"
	"testing"

	itshared "anchor/cmd/it/shared"
	"anchor/cmd/it/shared/mailpit"
	authzservice "anchor/internal/authz/service"
	"anchor/internal/repository"
	"anchor/internal/service"
	"anchor/internal/service/config"
	sessionservice "anchor/internal/session/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
)

var (
	TenantRepository              repository.TenantRepository
	UserRepository                repository.UserRepository
	PlatformUserRepo              repository.PlatformTenantUserRepository
	TokenHelper                   service.JWTHelper
	SessionSvc                    sessionservice.Service
	authCfg                       config.AuthConfig
	ProductRepo                   repository.ProductRepository
	PermissionRepo                repository.ProductPermissionRepository
	ProductAPIKeySvc              service.ProductAPIKeyService
	ProductUserRepo               repository.ProductUserRepository
	OrgMemberRepo                 repository.OrganizationMembershipRepository
	EventQueue                    *queue.Client
	IntegrationRepo               repository.IntegrationInstanceRepository
	initialWorkspaceOrganizations service.OrganizationService
	initialWorkspaceTx            transactor.Transactor
	shadowStats                   *authzservice.ShadowStats
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
				&initialWorkspaceOrganizations,
				&initialWorkspaceTx,
				&authCfg,
				&EventQueue,
				&IntegrationRepo,
				&reconcileQueue,
				&testDB,
				&adjustmentBackfill,
				&templateSync,
				&shadowStats,
			},
			AfterRun: mailpit.StopShared,
			// Every decision the suite makes also runs on OpenFGA (ADR-0022
			// G4, PR 6); decisions move there only once they always agree.
			CheckAfterRun: func() error {
				if mismatches := shadowStats.Mismatches(); mismatches > 0 {
					return fmt.Errorf("%d access decisions differed between the evaluator and OpenFGA; "+
						"search the log for authz.shadow_mismatch", mismatches)
				}
				return nil
			},
		},
	)
}
