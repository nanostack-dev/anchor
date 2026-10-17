package service_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/auth"
	"anchor/internal/domain/permission"
	"anchor/internal/repository"
)

const concurrentWriters = 8

// assertOneInsertWins runs concurrentWriters inserts of one unique value and
// expects every loser to get the repository sentinel instead of a raw driver
// error. These writes have no public route that races on the constraint:
// registration serializes on its invitation row unless several tenants invite
// one email, and product permissions are seeded by the server.
func assertOneInsertWins(t *testing.T, sentinel error, insert func(writer int) error) {
	t.Helper()
	errs := make([]error, concurrentWriters)
	var wg sync.WaitGroup
	for writer := range concurrentWriters {
		wg.Go(func() { errs[writer] = insert(writer) })
	}
	wg.Wait()

	winners := 0
	for _, err := range errs {
		if err == nil {
			winners++
			continue
		}
		require.ErrorIs(t, err, sentinel)
	}
	assert.Equal(t, 1, winners)
}

func TestConcurrentUniqueInsertsReturnSentinel(t *testing.T) {
	t.Run("User email", func(t *testing.T) {
		email := "race-" + uuid.NewString() + "@example.com"
		assertOneInsertWins(t, repository.ErrUserEmailExists, func(int) error {
			user := auth.User{Email: email, HashedPassword: "not-a-real-hash"}
			user.GenerateID()
			_, err := UserRepository.Create(t.Context(), user)
			return err
		})
	})

	t.Run("Product permission name", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		name := "race:Perm-" + uuid.NewString()
		assertOneInsertWins(t, repository.ErrProductPermissionExists, func(writer int) error {
			perm := permission.ProductPermission{ProductID: productID, Name: name}
			if writer%2 == 1 {
				perm.Name = strings.ToLower(name)
			}
			_, err := PermissionRepository.Create(t.Context(), perm)
			return err
		})
	})
}
