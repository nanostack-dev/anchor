package service_test

import (
	"database/sql"
	"testing"

	itshared "anchor/cmd/it/shared"
	authzengine "anchor/internal/authz/engine"
	authzstore "anchor/internal/authz/store"
	orgrolerepository "anchor/internal/organizationrole/repository"
	orgroleservice "anchor/internal/organizationrole/service"
	"anchor/internal/repository"
	rtrepository "anchor/internal/resourcetype/repository"
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
	OrganizationRoleRepo    orgrolerepository.Repository
	OrganizationRoleService orgroleservice.Service
	ProductRoleService      service.ProductRoleService
	MembershipService       service.OrganizationMembershipService
	Transactor              transactor.Transactor
	AuthzStores             authzstore.Stores
	AuthzEngines            authzengine.Provider
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
				&OrganizationRoleRepo,
				&OrganizationRoleService,
				&ProductRoleService,
				&MembershipService,
				&Transactor,
				&AuthzStores,
				&AuthzEngines,
				&TestDB,
			},
		},
	)
}
