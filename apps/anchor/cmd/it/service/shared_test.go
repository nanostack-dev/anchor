package service_test

import (
	"database/sql"
	"testing"

	itshared "anchor/cmd/it/shared"
	authzengine "anchor/internal/authz/engine"
	authzrepository "anchor/internal/authz/repository"
	authzservice "anchor/internal/authz/service"
	authzstore "anchor/internal/authz/store"
	invitationservice "anchor/internal/invitation/service"
	"anchor/internal/repository"
	rtrepository "anchor/internal/resourcetype/repository"
	"anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
)

var (
	TestLogger             = itshared.TestLogger
	ServerURL              = itshared.ServerURL
	Faker                  = itshared.Faker
	APIKeyService          service.ProductAPIKeyService
	OrgAPIKeyService       service.OrganizationAPIKeyService
	APIKeyRepository       repository.ProductAPIKeyRepository
	OrgAPIKeyRepository    repository.OrganizationAPIKeyRepository
	OrganizationRepo       repository.OrganizationRepository
	PermissionRepository   repository.ProductPermissionRepository
	ResourcePermissionRepo repository.ProductResourcePermissionRepository
	ProductRepository      repository.ProductRepository
	TenantRepository       repository.TenantRepository
	UserRepository         repository.UserRepository
	Queue                  *queue.Client
	APIKeyEventSvc         service.OrganizationAPIKeyEventService
	ProductRoleRepo        repository.ProductRoleRepository
	ResourceTypeRepo       rtrepository.Repository
	TestDB                 *sql.DB
	MembershipService      service.OrganizationMembershipService
	InvitationService      invitationservice.OrganizationInvitationService
	OrganizationService    service.OrganizationService
	ProductRoleService     service.ProductRoleService
	ResourcePermissionSvc  service.ResourcePermissionService
	Transactor             transactor.Transactor
	AuthzStores            authzstore.Stores
	AuthzEngines           authzengine.Provider
	AuthzRepo              authzrepository.Repository
	AuthzSyncer            authzservice.Syncer
	AuthzSweeper           *authzservice.Sweeper
	AuthzReplayer          *authzservice.Replayer
)

func TestMain(m *testing.M) {
	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:                  true,
			PopulateRepositories:         true,
			APIKeyService:                &APIKeyService,
			OrganizationAPIKeyService:    &OrgAPIKeyService,
			APIKeyRepository:             &APIKeyRepository,
			OrganizationAPIKeyRepository: &OrgAPIKeyRepository,
			OrganizationRepository:       &OrganizationRepo,
			PermissionRepository:         &PermissionRepository,
			ProductRepository:            &ProductRepository,
			TenantRepository:             &TenantRepository,
			UserRepository:               &UserRepository,
			ExtraPopulateTargets: []any{
				&Queue,
				&APIKeyEventSvc,
				&ResourcePermissionRepo,
				&ProductRoleRepo,
				&ResourceTypeRepo,
				&Transactor,
				&AuthzStores,
				&AuthzEngines,
				&AuthzRepo,
				&AuthzSyncer,
				&AuthzSweeper,
				&AuthzReplayer,
				&TestDB,
				&MembershipService,
				&InvitationService,
				&OrganizationService,
				&ProductRoleService,
				&ResourcePermissionSvc,
			},
		},
	)
}
