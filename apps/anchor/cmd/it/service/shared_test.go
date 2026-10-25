package service_test

import (
	"database/sql"
	"testing"

	itshared "anchor/cmd/it/shared"
	accessservice "anchor/internal/access/service"
	authzengine "anchor/internal/authz/engine"
	authzrepository "anchor/internal/authz/repository"
	authzservice "anchor/internal/authz/service"
	authzstore "anchor/internal/authz/store"
	orgrolerepository "anchor/internal/organizationrole/repository"
	orgroleservice "anchor/internal/organizationrole/service"
	"anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
	resourceservice "anchor/internal/resource/service"
	rtrepository "anchor/internal/resourcetype/repository"
	rtservice "anchor/internal/resourcetype/service"
	assignmentrepository "anchor/internal/roleassignment/repository"
	assignmentservice "anchor/internal/roleassignment/service"
	"anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
)

var (
	TestLogger              = itshared.TestLogger
	ServerURL               = itshared.ServerURL
	Faker                   = itshared.Faker
	APIKeyService           service.ProductAPIKeyService
	OrgAPIKeyService        service.OrganizationAPIKeyService
	APIKeyRepository        repository.ProductAPIKeyRepository
	OrgAPIKeyRepository     repository.OrganizationAPIKeyRepository
	OrganizationRepo        repository.OrganizationRepository
	PermissionRepository    repository.ProductPermissionRepository
	ResourcePermissionRepo  repository.ProductResourcePermissionRepository
	ProductRepository       repository.ProductRepository
	TenantRepository        repository.TenantRepository
	UserRepository          repository.UserRepository
	Queue                   *queue.Client
	APIKeyEventSvc          service.OrganizationAPIKeyEventService
	ProductRoleRepo         repository.ProductRoleRepository
	ResourceTypeRepo        rtrepository.Repository
	ResourceTypeService     rtservice.Service
	ResourceService         resourceservice.Service
	ResourceRepo            resourcerepository.Repository
	RoleAssignmentRepo      assignmentrepository.Repository
	RoleAssignmentService   assignmentservice.Service
	OrganizationRoleRepo    orgrolerepository.Repository
	OrganizationRoleService orgroleservice.Service
	ProductRoleService      service.ProductRoleService
	MembershipService       service.OrganizationMembershipService
	Transactor              transactor.Transactor
	AuthzStores             authzstore.Stores
	AuthzEngines            authzengine.Provider
	AuthzRepo               authzrepository.Repository
	AuthzSyncer             authzservice.Syncer
	AuthzSweeper            *authzservice.Sweeper
	AuthzReplayer           *authzservice.Replayer
	Evaluator               accessservice.Evaluator
	ProductUserRepo         repository.ProductUserRepository
	MembershipRepo          repository.OrganizationMembershipRepository
	WorkspaceRepo           repository.WorkspaceRepository
	TestDB                  *sql.DB
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
				&ResourceTypeService,
				&ResourceService,
				&ResourceRepo,
				&RoleAssignmentRepo,
				&RoleAssignmentService,
				&OrganizationRoleRepo,
				&OrganizationRoleService,
				&ProductRoleService,
				&MembershipService,
				&Transactor,
				&AuthzStores,
				&AuthzEngines,
				&AuthzRepo,
				&AuthzSyncer,
				&AuthzSweeper,
				&AuthzReplayer,
				&Evaluator,
				&ProductUserRepo,
				&MembershipRepo,
				&WorkspaceRepo,
				&TestDB,
			},
		},
	)
}
