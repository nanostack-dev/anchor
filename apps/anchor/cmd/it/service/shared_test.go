package service_test

import (
	"database/sql"
	"testing"

	itshared "anchor/cmd/it/shared"
	accessservice "anchor/internal/access/service"
	authzengine "anchor/internal/authz/engine"
	"anchor/internal/authz/fgasync"
	authzstore "anchor/internal/authz/store"
	invitationservice "anchor/internal/invitation/service"
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
	AuthzMarks             fgasync.Repository
	AuthzSyncer            *fgasync.Syncer
	Evaluator              accessservice.Evaluator
	ProductUserRepo        repository.ProductUserRepository
	MembershipRepo         repository.OrganizationMembershipRepository
	WorkspaceRepo          repository.WorkspaceRepository
	ResourceTypeService    rtservice.Service
	ResourceService        resourceservice.Service
	RoleAssignmentService  assignmentservice.Service
	ResourceRepo           resourcerepository.Repository
	RoleAssignmentRepo     assignmentrepository.Repository
	WorkspaceBackfill      *resourceservice.WorkspaceBackfill
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
				&Transactor,
				&AuthzStores,
				&AuthzEngines,
				&AuthzMarks,
				&AuthzSyncer,
				&Evaluator,
				&ProductUserRepo,
				&MembershipRepo,
				&WorkspaceRepo,
				&WorkspaceBackfill,
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
