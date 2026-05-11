package security_test

import (
	"os"
	"testing"

	itshared "anchor/cmd/it/shared"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
		panic(err)
	}

	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:             true,
			PopulateRepositories:    true,
			APIKeyService:           &itshared.APIKeyService,
			PermissionRepository:    &itshared.PermissionRepository,
			ProductRepository:       &itshared.ProductRepository,
			ProductUserRepository:   &itshared.ProductUserRepository,
			OrgMembershipRepository: &itshared.OrgMembershipRepository,
			TenantRepository:        &itshared.TenantRepository,
			UserRepository:          &itshared.UserRepository,
			PlatformUserRepository:  &itshared.PlatformTenantUserRepo,
			JWTHelper:               &itshared.JWTHelper,
		},
	)
}
