package ct_test

import (
	"testing"
	"time"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	organizationAPIKeyPermissionFileRead   = "file:read"
	organizationAPIKeyPermissionFileCreate = "file:create"
	organizationAPIKeyPermissionFileUpdate = "file:update"
	organizationAPIKeyPermissionFileDelete = "file:delete"
)

// organizationAPIKeyExpiryMargin is how far ahead the expiry of a near-expiry
// test key is placed. It has to cover the round trip of the create call itself,
// because the service rejects an expiry that is not strictly after the current
// second.
const organizationAPIKeyExpiryMargin = 3 * time.Second

// nearFutureExpiry returns an expiry the create endpoint accepts and that lapses
// a few seconds later, for tests that then wait on the expiration worker.
//
// The margin is added to the current second boundary rather than to the current
// instant, which guarantees at least organizationAPIKeyExpiryMargin-1s of real
// headroom. Adding first and truncating after — Now().Add(margin).Truncate(
// time.Second) — throws the sub-second part of Now() away instead, shrinking the
// headroom to whatever is left of the current second. With a one-second margin
// that leaves as little as a millisecond, so a create that is slow to land
// arrives after the expiry it asked for and is rejected with 400.
func nearFutureExpiry() time.Time {
	return time.Now().UTC().Truncate(time.Second).Add(organizationAPIKeyExpiryMargin)
}

type organizationAPIKeyResourcePermissions struct {
	FileRead   string
	FileCreate string
	FileUpdate string
	FileDelete string
}

func givenOrganizationAPIKeyResourcePermissions(
	t *testing.T,
	product *itdsl.ProductContext,
) organizationAPIKeyResourcePermissions {
	t.Helper()

	createdPermissions := product.CreateDefaultProductResourcePermissions(t)

	permissionNames := make(map[string]bool, len(createdPermissions))
	for _, permission := range createdPermissions {
		permissionNames[permission.Name] = true
	}

	requiredPermissions := []string{
		organizationAPIKeyPermissionFileRead,
		organizationAPIKeyPermissionFileCreate,
		organizationAPIKeyPermissionFileUpdate,
		organizationAPIKeyPermissionFileDelete,
	}
	for _, permissionName := range requiredPermissions {
		if !permissionNames[permissionName] {
			t.Fatalf("expected default product resource permission %q to exist", permissionName)
		}
	}

	return organizationAPIKeyResourcePermissions{
		FileRead:   organizationAPIKeyPermissionFileRead,
		FileCreate: organizationAPIKeyPermissionFileCreate,
		FileUpdate: organizationAPIKeyPermissionFileUpdate,
		FileDelete: organizationAPIKeyPermissionFileDelete,
	}
}
