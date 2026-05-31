package dslfactory

import (
	"context"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"

	"anchor/internal/domain/auth"
	platformdomain "anchor/internal/domain/platform"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	itshared "anchor/cmd/it/shared"
)

type PlatformUserResult struct {
	ID                  string
	UserID              string
	Email               string
	Password            string
	AccessToken         string
	RefreshToken        string
	AuthenticatedClient *nanostackClient.ClientWithResponses
}

func CreatePlatformUserWithRole(
	t require.TestingT,
	tenantID string,
	role platformdomain.TenantRole,
) *PlatformUserResult {
	require.NotNil(t, itshared.UserRepository, "user repository is not available in test setup")
	require.NotNil(
		t, itshared.PlatformTenantUserRepo,
		"platform user repository is not available in test setup",
	)
	require.NotNil(t, itshared.JWTHelper, "jwt helper is not available in test setup")

	emailPrefix := "platform_user_"
	if role == platformdomain.TenantRoleOwner {
		emailPrefix = "platform_owner_"
	}
	if role == platformdomain.TenantRoleAdmin {
		emailPrefix = "platform_admin_"
	}

	email := emailPrefix + itshared.Faker.UUID().V4() + "@example.test"
	password := itshared.Faker.Internet().Password() + "l@1Q-"

	hashedPassword, hashErr := bcrypt.GenerateFromPassword(
		[]byte(password), bcrypt.DefaultCost,
	)
	require.NoError(t, hashErr)

	newUser := auth.User{
		Email:          email,
		HashedPassword: string(hashedPassword),
	}
	newUser.GenerateID()

	createdUser, userErr := itshared.UserRepository.Create(context.Background(), newUser)
	require.NoError(t, userErr)

	platformUser := platformdomain.User{
		UserID:           createdUser.ID,
		ExternalID:       createdUser.ExternalID,
		Name:             createdUser.Name,
		Email:            createdUser.Email,
		HashedPassword:   createdUser.HashedPassword,
		CreatedAt:        createdUser.CreatedAt,
		UpdatedAt:        createdUser.UpdatedAt,
		PlatformTenantID: tenantID,
		Role:             role,
	}
	platformUser.GenerateID()

	createdPlatformUser, platformUserErr := itshared.PlatformTenantUserRepo.Create(
		context.Background(), platformUser,
	)
	require.NoError(t, platformUserErr)

	accessToken, refreshToken, tokenErr := itshared.JWTHelper.GenerateTokens(
		createdUser.ID, tenantID,
	)
	require.NoError(t, tokenErr)

	return &PlatformUserResult{
		ID:                  createdPlatformUser.ID,
		UserID:              createdUser.ID,
		Email:               email,
		Password:            password,
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		AuthenticatedClient: NewBearerClient(t, itshared.ServerURL, accessToken),
	}
}
