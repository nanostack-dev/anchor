package service_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"
	"anchor/internal/repository"
	"anchor/internal/service"

	"github.com/nanostack-dev/pgkit/pgqueue"
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
	Queue                  *pgqueue.Client
	APIKeyEventSvc         service.OrganizationAPIKeyEventService
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
			},
		},
	)
}
