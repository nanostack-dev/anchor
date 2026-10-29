package ct_test

import (
	"context"
	"testing"

	itshared "anchor/cmd/it/shared"
	"anchor/cmd/it/shared/mailpit"
	"anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
	resourceservice "anchor/internal/resource/service"
	rtservice "anchor/internal/resourcetype/service"
	assignmentservice "anchor/internal/roleassignment/service"
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
	// The workspace backfill's dependencies, so a test builds one with its
	// own logger.
	workspaceRepo     repository.WorkspaceRepository
	resourceSvc       resourceservice.Service
	resourceRepo      resourcerepository.Repository
	resourceTypeSvc   rtservice.Service
	roleAssignmentSvc assignmentservice.Service
	workspaceReplay   resourceservice.WriteReplay
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
				&authzStores,
				&workspaceRepo,
				&resourceSvc,
				&resourceRepo,
				&resourceTypeSvc,
				&roleAssignmentSvc,
				&workspaceReplay,
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
