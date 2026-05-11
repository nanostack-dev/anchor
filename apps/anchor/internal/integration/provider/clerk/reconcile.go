package clerk

import (
	"context"
	"fmt"
	"strings"

	clerkapi "github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"

	"anchor/internal/integration/provider"
)

// Reconcile fetches all users from Clerk and emits UPSERT_USER commands.
func (p *Provider) Reconcile(
	ctx context.Context,
	configJSON []byte,
) ([]provider.Command, error) {
	cfg, err := p.resolveConfig(configJSON)
	if err != nil {
		return nil, err
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		p.logger.Info().Msg("clerk api key not configured, reconciliation skipped")
		return nil, nil
	}

	users, err := p.fetchAllUsers(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	commands := make([]provider.Command, 0, len(users))
	for _, u := range users {
		if strings.TrimSpace(u.ID) == "" {
			continue
		}

		commands = append(commands, provider.Command{
			Type: CommandUpsertUser,
			Data: UpsertUserData{
				ExternalID: u.ID,
				Email:      extractPrimaryEmailFromSDKUser(u),
				Name:       buildFullName(u.FirstName, u.LastName),
			},
		})
	}

	return commands, nil
}

func (p *Provider) fetchAllUsers(ctx context.Context, apiKey string) ([]*clerkapi.User, error) {
	usersListClient := user.NewClient(&clerkapi.ClientConfig{
		BackendConfig: clerkapi.BackendConfig{
			Key:        &apiKey,
			HTTPClient: p.httpClient,
			URL:        &p.baseURL,
		},
	})

	users := make([]*clerkapi.User, 0)
	offset := int64(0)
	limit := int64(clerkUsersPageSize)

	for {
		page, err := usersListClient.List(ctx, &user.ListParams{
			ListParams: clerkapi.ListParams{
				Limit:  &limit,
				Offset: &offset,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list clerk users: %w", err)
		}

		users = append(users, page.Users...)
		offset += int64(len(page.Users))

		if len(page.Users) == 0 || offset >= page.TotalCount {
			break
		}
	}

	return users, nil
}

func extractPrimaryEmailFromSDKUser(u *clerkapi.User) string {
	if u == nil {
		return ""
	}

	if u.PrimaryEmailAddressID != nil {
		for _, ea := range u.EmailAddresses {
			if ea != nil && ea.ID == *u.PrimaryEmailAddressID {
				return ea.EmailAddress
			}
		}
	}

	if len(u.EmailAddresses) > 0 && u.EmailAddresses[0] != nil {
		return u.EmailAddresses[0].EmailAddress
	}

	return ""
}
