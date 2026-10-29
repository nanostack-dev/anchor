package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
)

// syncWorld is a product with one permission, a managed role holding it,
// and one organization, all mirrored into the product's store.
type syncWorld struct {
	productID      string
	permission     string
	managedID      string
	organizationID string
}

func givenSyncWorld(t *testing.T) syncWorld {
	t.Helper()
	productID := GivenATenantAndProduct(t).Product.ID
	permission := givenPermission(t, productID)
	w := syncWorld{
		productID: productID, permission: permission,
		managedID:      givenManagedRole(t, productID, "Reader", permission),
		organizationID: givenOrganization(t, productID),
	}
	// The fixtures write the tables directly, without events.
	syncProduct(t, productID)
	return w
}

func givenManagedRole(t *testing.T, productID, name, permission string) string {
	t.Helper()
	managed := role.NewManaged(productID, name+" "+uuid.NewString(), "")
	managed.ResourceTypeID = organizationTypeID(t, productID)
	_, err := ProductRoleRepo.Create(t.Context(), managed)
	require.NoError(t, err)
	givenRolePermission(t, productID, managed.ID, permission)
	return managed.ID
}

func givenSyncCustomRole(t *testing.T, w syncWorld, organizationID, permission string) string {
	t.Helper()
	custom := role.NewCustom(w.productID, organizationID, "Auditor "+uuid.NewString(), "")
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_roles
		(id, product_id, organization_id, type, name, resource_type_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		custom.ID, w.productID, organizationID, string(custom.Type), custom.Name, organizationTypeID(t, w.productID))
	require.NoError(t, err)
	givenRolePermission(t, w.productID, custom.ID, permission)
	return custom.ID
}

func givenMember(t *testing.T, w syncWorld, organizationID, roleID string) string {
	t.Helper()
	userID := givenProductUser(t, w.productID)
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, organizationID, userID, roleID)
	require.NoError(t, err)
	return userID
}

func givenSyncAPIKey(t *testing.T, w syncWorld, permission string) string {
	t.Helper()
	keyID := "orgapikey_" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_api_keys
		(id, organization_id, name, hashed_value, obfuscated_value, status)
		VALUES ($1, $2, 'key', $3, 'ob', 'ACTIVE')`, keyID, w.organizationID, uuid.NewString())
	require.NoError(t, err)
	_, err = TestDB.ExecContext(t.Context(), `INSERT INTO organization_api_key_permissions
		(api_key_id, organization_id, product_id, permission_name) VALUES ($1, $2, $3, $4)`,
		keyID, w.organizationID, w.productID, permission)
	require.NoError(t, err)
	return keyID
}

func syncProduct(t *testing.T, productID string) {
	t.Helper()
	require.NoError(t, AuthzSyncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
}

// rebuild reconciles the Product, as the sweeper does: decisions read a store
// only once a rebuild filled it.
func rebuild(t *testing.T, productID string) {
	t.Helper()
	require.NoError(t, AuthzSyncer.Reconcile(t.Context(), productID))
}

func syncKey(t *testing.T, productID, source, key string) {
	t.Helper()
	require.NoError(
		t,
		AuthzSyncer.Sync(t.Context(), fgasync.Scope{ProductID: productID, Source: source, Key: key}),
	)
}

const (
	assignments = "organization_assignments"
	roleGrants  = "role_grants"
	apiKeys     = "organization_api_keys"
)

// productStore is the product's store at the model the syncer recorded.
func productStore(t *testing.T, productID string) fga.Store {
	t.Helper()
	storeID, err := AuthzStores.StoreID(t.Context(), productID)
	require.NoError(t, err)
	recorded, err := AuthzMarks.RecordedModel(t.Context(), productID)
	require.NoError(t, err)
	require.NotEmpty(t, recorded.ID, "the product has no model yet")
	return fga.NewStore(openFGA(t), storeID, recorded.ID)
}

// can asks the store directly, under the model the sync wrote.
func can(t *testing.T, productID, subject, permission, organizationID string) bool {
	t.Helper()
	allowed, err := productStore(t, productID).
		Check(t.Context(), subject, model.CanRelation(permission), model.Organization(organizationID))
	require.NoError(t, err)
	return allowed
}

func TestAuthzSync(t *testing.T) {
	openFGA(t)

	t.Run("A member of an organization holds the permissions of their role", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		stranger := givenProductUser(t, w.productID)

		syncKey(t, w.productID, assignments, w.organizationID)

		assert.True(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
		assert.False(t, can(t, w.productID, model.User(stranger), w.permission, w.organizationID))
	})

	t.Run("A removed member loses the permission at the next sync", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		syncKey(t, w.productID, assignments, w.organizationID)
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM organization_memberships WHERE organization_id = $1`, w.organizationID)
		require.NoError(t, err)

		syncKey(t, w.productID, assignments, w.organizationID)

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("A permission removed from a role leaves its members", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		syncKey(t, w.productID, assignments, w.organizationID)
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM product_role_resource_permissions WHERE product_role_id = $1`, w.managedID)
		require.NoError(t, err)

		syncKey(t, w.productID, roleGrants, w.managedID)

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("An active API key holds its permissions on its organization", func(t *testing.T) {
		w := givenSyncWorld(t)
		keyID := givenSyncAPIKey(t, w, w.permission)

		syncKey(t, w.productID, apiKeys, w.organizationID)

		assert.True(t, can(t, w.productID, model.APIKey(keyID), w.permission, w.organizationID))
	})

	t.Run("A managed role two organizations hold keeps granting when one lets it go", func(t *testing.T) {
		w := givenSyncWorld(t)
		other := givenOrganization(t, w.productID)
		givenMember(t, w, w.organizationID, w.managedID)
		elsewhere := givenMember(t, w, other, w.managedID)
		syncProduct(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM organization_memberships WHERE organization_id = $1`, w.organizationID)
		require.NoError(t, err)

		syncKey(t, w.productID, assignments, w.organizationID)

		assert.True(t, can(t, w.productID, model.User(elsewhere), w.permission, other))
	})

	t.Run("A catalog that did not change keeps its model", func(t *testing.T) {
		w := givenSyncWorld(t)
		first, err := AuthzMarks.RecordedModel(t.Context(), w.productID)
		require.NoError(t, err)

		syncProduct(t, w.productID)

		again, err := AuthzMarks.RecordedModel(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, again.ID)
	})

	t.Run("Five concurrent syncs of one product all succeed", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		var wg sync.WaitGroup
		for range 5 {
			wg.Go(func() {
				assert.NoError(t, AuthzSyncer.Sync(t.Context(), fgasync.WholeProduct(w.productID)))
			})
		}
		wg.Wait()

		assert.True(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("A member who changes role holds only the new role's permissions", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		other := givenPermission(t, w.productID)
		otherRoleID := givenManagedRole(t, w.productID, "Writer", other)
		syncProduct(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(), `UPDATE organization_memberships SET product_role_id = $1
			WHERE organization_id = $2`, otherRoleID, w.organizationID)
		require.NoError(t, err)

		syncKey(t, w.productID, assignments, w.organizationID)

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
		assert.True(t, can(t, w.productID, model.User(member), other, w.organizationID))
	})

	t.Run("A custom role grants its permissions in its own organization only", func(t *testing.T) {
		w := givenSyncWorld(t)
		audit := givenPermission(t, w.productID)
		custom := givenSyncCustomRole(t, w, w.organizationID, audit)
		member := givenMember(t, w, w.organizationID, custom)
		other := givenOrganization(t, w.productID)
		elsewhere := givenMember(t, w, other, w.managedID)

		syncProduct(t, w.productID)

		assert.True(t, can(t, w.productID, model.User(member), audit, w.organizationID))
		assert.False(t, can(t, w.productID, model.User(elsewhere), audit, other))
	})

	t.Run("A permission removed from a custom role leaves its members at the role's sync", func(t *testing.T) {
		w := givenSyncWorld(t)
		audit := givenPermission(t, w.productID)
		custom := givenSyncCustomRole(t, w, w.organizationID, audit)
		member := givenMember(t, w, w.organizationID, custom)
		syncProduct(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM product_role_resource_permissions WHERE product_role_id = $1`, custom)
		require.NoError(t, err)

		syncKey(t, w.productID, roleGrants, custom)

		assert.False(t, can(t, w.productID, model.User(member), audit, w.organizationID))
	})

	t.Run("A member without a role holds nothing", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
			(organization_id, product_user_id) VALUES ($1, $2)`, w.organizationID, userID)
		require.NoError(t, err)

		syncKey(t, w.productID, assignments, w.organizationID)

		assert.False(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
	})

	t.Run("A deleted organization implies nothing any more", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		keyID := givenSyncAPIKey(t, w, w.permission)
		syncProduct(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(), `DELETE FROM organizations WHERE id = $1`, w.organizationID)
		require.NoError(t, err)

		syncKey(t, w.productID, assignments, w.organizationID)
		syncKey(t, w.productID, apiKeys, w.organizationID)

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
		assert.False(t, can(t, w.productID, model.APIKey(keyID), w.permission, w.organizationID))
	})

	t.Run("An inactive API key holds nothing", func(t *testing.T) {
		w := givenSyncWorld(t)
		keyID := givenSyncAPIKey(t, w, w.permission)
		syncKey(t, w.productID, apiKeys, w.organizationID)
		_, err := TestDB.ExecContext(t.Context(),
			`UPDATE organization_api_keys SET status = 'INACTIVE' WHERE id = $1`, keyID)
		require.NoError(t, err)

		syncKey(t, w.productID, apiKeys, w.organizationID)

		assert.False(t, can(t, w.productID, model.APIKey(keyID), w.permission, w.organizationID))
	})

	t.Run("A deleted permission leaves the model and every grant", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.managedID)
		kept := givenPermission(t, w.productID)
		givenRolePermission(t, w.productID, w.managedID, kept)
		syncProduct(t, w.productID)
		before, err := AuthzMarks.RecordedModel(t.Context(), w.productID)
		require.NoError(t, err)
		_, err = TestDB.ExecContext(t.Context(), `DELETE FROM product_resource_permissions
			WHERE product_id = $1 AND name = $2`, w.productID, w.permission)
		require.NoError(t, err)

		syncProduct(t, w.productID)

		after, err := AuthzMarks.RecordedModel(t.Context(), w.productID)
		require.NoError(t, err)
		assert.NotEqual(t, before.ID, after.ID)
		assert.True(t, can(t, w.productID, model.User(member), kept, w.organizationID))
	})
}

func TestAuthzReconcile(t *testing.T) {
	openFGA(t)

	t.Run("Reconcile removes the grants of a role that no longer exists", func(t *testing.T) {
		w := givenSyncWorld(t)
		orphan := fga.Tuple{
			Object: model.Role("product_role_" + uuid.NewString()), Relation: model.GrantsRelation(w.permission),
			User: model.AllUsers,
		}
		store := productStore(t, w.productID)
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{orphan}}))

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		read, err := store.Read(t.Context(), fga.Filter{Object: orphan.Object})
		require.NoError(t, err)
		assert.Empty(t, read)
	})

	t.Run("A product never written to gets its store from the sweeper's reconcile", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		w := syncWorld{productID: productID, permission: givenPermission(t, productID)}
		w.managedID = givenManagedRole(t, productID, "Reader", w.permission)
		w.organizationID = givenOrganization(t, productID)
		member := givenMember(t, w, w.organizationID, w.managedID)

		require.Eventually(t, func() bool {
			AuthzSyncer.Sweep(t.Context())
			recorded, err := AuthzMarks.RecordedModel(t.Context(), productID)
			return err == nil && recorded.ID != "" &&
				can(t, productID, model.User(member), w.permission, w.organizationID)
		}, 60*time.Second, 10*time.Millisecond)
	})

	t.Run("A store whose product is gone is deleted by a reconcile pass", func(t *testing.T) {
		openfga := openFGA(t)
		orphan, err := openfga.CreateStore(
			t.Context(),
			&openfgav1.CreateStoreRequest{Name: "prd_gone_" + uuid.NewString()},
		)
		require.NoError(t, err)
		w := givenSyncWorld(t)
		// Make this product due, so the next sweep reconciles and removes orphan stores.
		_, err = TestDB.ExecContext(t.Context(),
			`UPDATE authz_stores SET reconciled_at = NULL WHERE product_id = $1`, w.productID)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			AuthzSyncer.Sweep(t.Context())
			_, getErr := openfga.GetStore(t.Context(), &openfgav1.GetStoreRequest{StoreId: orphan.GetId()})
			return getErr != nil
		}, 30*time.Second, 10*time.Millisecond)
	})
}

func TestAuthzMarks(t *testing.T) {
	openFGA(t)

	assignmentsOf := func(w syncWorld) fgasync.Scope {
		return fgasync.Scope{ProductID: w.productID, Source: assignments, Key: w.organizationID}
	}

	t.Run("A membership write leaves a mark that the sweeper replays", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)

		_, err := MembershipService.AddMember(t.Context(), organization.AddMemberInput{
			ProductID: w.productID, OrganizationID: w.organizationID, ProductUserID: userID, RoleID: &w.managedID,
		})
		require.NoError(t, err)
		marks, err := AuthzMarks.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		require.Len(t, marks, 1)
		assert.Equal(t, assignmentsOf(w), marks[0].Scope)

		require.Eventually(t, func() bool {
			AuthzSyncer.Sweep(t.Context())
			left, pendingErr := AuthzMarks.Pending(t.Context(), w.productID)
			return pendingErr == nil && len(left) == 0
		}, 30*time.Second, 10*time.Millisecond)
		assert.True(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
	})

	t.Run("A mark made again after a sync read it is not cleared by that sync", func(t *testing.T) {
		ctx := t.Context()
		w := givenSyncWorld(t)
		require.NoError(t, Transactor.InTx(ctx, func(txCtx context.Context) error {
			return AuthzMarks.Mark(txCtx, assignmentsOf(w))
		}))
		read, err := AuthzMarks.Pending(ctx, w.productID)
		require.NoError(t, err)
		require.Len(t, read, 1)
		require.NoError(t, AuthzMarks.ClearIfUnchanged(ctx, read[0]))
		require.NoError(t, AuthzMarks.Mark(ctx, assignmentsOf(w)))

		require.NoError(t, AuthzMarks.ClearIfUnchanged(ctx, read[0]))

		left, err := AuthzMarks.Pending(ctx, w.productID)
		require.NoError(t, err)
		assert.Len(t, left, 1)
	})

	t.Run("A product whose sync failed waits behind the others", func(t *testing.T) {
		ctx := t.Context()
		failing := givenSyncWorld(t)
		healthy := givenSyncWorld(t)
		require.NoError(t, AuthzMarks.Mark(ctx, assignmentsOf(failing)))
		require.NoError(t, AuthzMarks.Mark(ctx, assignmentsOf(healthy)))

		require.NoError(t, AuthzMarks.Postpone(ctx, failing.productID, time.Now().Add(time.Hour)))

		oldest, err := AuthzMarks.Oldest(ctx, 1000)
		require.NoError(t, err)
		position := map[string]int{}
		for i, mark := range oldest {
			position[mark.ProductID] = i
		}
		assert.Less(t, position[healthy.productID], position[failing.productID])
	})

	t.Run("A write whose transaction rolls back leaves no mark", func(t *testing.T) {
		ctx := t.Context()
		w := givenSyncWorld(t)

		err := Transactor.InTx(ctx, func(txCtx context.Context) error {
			if markErr := AuthzMarks.Mark(txCtx, assignmentsOf(w)); markErr != nil {
				return markErr
			}
			return errRollback
		})

		require.ErrorIs(t, err, errRollback)
		left, err := AuthzMarks.Pending(ctx, w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})
}

func TestAuthzReplayAfterCommit(t *testing.T) {
	openFGA(t)

	t.Run("A write's request replays what it marked right after it commits", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)
		ctx, marked := fgasync.CollectMarks(t.Context())

		_, err := MembershipService.AddMember(ctx, organization.AddMemberInput{
			ProductID: w.productID, OrganizationID: w.organizationID, ProductUserID: userID, RoleID: &w.managedID,
		})
		require.NoError(t, err)
		AuthzSyncer.Replay(t.Context(), marked())

		assert.Equal(t, []fgasync.Scope{{ProductID: w.productID, Source: assignments, Key: w.organizationID}},
			marked())
		assert.True(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
		left, err := AuthzMarks.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})

	t.Run("A member add refused before it writes leaves no mark and collects nothing", func(t *testing.T) {
		w := givenSyncWorld(t)
		ctx, marked := fgasync.CollectMarks(t.Context())

		_, err := MembershipService.AddMember(ctx, organization.AddMemberInput{
			ProductID:      w.productID,
			OrganizationID: w.organizationID,
			ProductUserID:  "pusr_missing",
			RoleID:         &w.managedID,
		})
		require.Error(t, err)

		assert.Empty(t, marked())
		left, err := AuthzMarks.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})
}

var errRollback = errors.New("roll back")
