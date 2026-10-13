package defaultworkspaces_test

import (
	"os"
	"testing"

	itshared "anchor/cmd/it/shared"
	"anchor/cmd/it/shared/mailpit"
	"anchor/internal/service"
)

var defaultWorkspaceStartup *service.DefaultWorkspaceStartupSync

// This sweep touches every organization: keep its runtime separate from other fixtures.
func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
		panic(err)
	}
	itshared.RunTestMain(m, itshared.TestConfig{
		EnableRedis:            true,
		PopulateRepositories:   true,
		APIKeyService:          &itshared.APIKeyService,
		PermissionRepository:   &itshared.PermissionRepository,
		ProductRepository:      &itshared.ProductRepository,
		TenantRepository:       &itshared.TenantRepository,
		UserRepository:         &itshared.UserRepository,
		PlatformUserRepository: &itshared.PlatformTenantUserRepo,
		JWTHelper:              &itshared.JWTHelper,
		SessionService:         &itshared.SessionService,
		ExtraPopulateTargets:   []any{&defaultWorkspaceStartup},
		AfterRun:               mailpit.StopShared,
	})
}
