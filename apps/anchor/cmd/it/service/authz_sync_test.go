package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
	authzservice "anchor/internal/authz/service"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
)

// syncWorld is a product with one permission, a role template holding it,
// and one organization.
type syncWorld struct {
	productID      string
	permission     string
	templateID     string
	organizationID string
}

func givenSyncWorld(t *testing.T) syncWorld {
	t.Helper()
	ctx := t.Context()
	productID := GivenATenantAndProduct(t).Product.ID
	permission := givenPermission(t, productID)
	template := role.ProductRole{
		ProductID:      productID,
		Name:           "Reader " + uuid.NewString(),
		ResourceTypeID: organizationTypeID(t, productID),
	}
	template.Key = strings.ToLower(template.Name)
	template.GenerateID()
	_, err := ProductRoleRepo.Create(ctx, template)
	require.NoError(t, err)
	givenRolePermission(t, productID, template.ID, permission)
	return syncWorld{
		productID: productID, permission: permission, templateID: template.ID,
		organizationID: givenOrganization(t, productID),
	}
}

func givenOrganization(t *testing.T, productID string) string {
	t.Helper()
	org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
	org.GenerateID()
	_, err := OrganizationRepo.Create(t.Context(), org)
	require.NoError(t, err)
	return org.ID
}

func givenMember(t *testing.T, w syncWorld, organizationID, roleID string) string {
	t.Helper()
	userID := givenProductUser(t, w.productID)
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, organizationID, userID, roleID)
	require.NoError(t, err)
	return userID
}

// can asks the store directly, under the model the sync wrote.
func can(t *testing.T, productID, subject, permission, organizationID string) bool {
	t.Helper()
	ctx := t.Context()
	found, err := AuthzRepo.FindStore(ctx, productID)
	require.NoError(t, err)
	stored, ok := found.Get()
	require.True(t, ok, "the product has no store yet")
	resp, err := openFGA(t).Check(ctx, &openfgav1.CheckRequest{
		StoreId:              stored.StoreID,
		AuthorizationModelId: stored.ModelID,
		TupleKey: &openfgav1.CheckRequestTupleKey{
			User: subject, Relation: model.CanRelation(permission), Object: model.Organization(organizationID),
		},
	})
	require.NoError(t, err)
	return resp.GetAllowed()
}

func TestAuthzSync(t *testing.T) {
	openFGA(t)

	t.Run("A member of an organization holds the permissions of their role", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		stranger := givenProductUser(t, w.productID)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.True(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
		assert.False(t, can(t, w.productID, model.User(stranger), w.permission, w.organizationID))
	})

	t.Run("A removed member loses the permission at the next sync", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM organization_memberships WHERE organization_id = $1`, w.organizationID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("A permission removed from a role leaves its members", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM product_role_resource_permissions WHERE product_role_id = $1`, w.templateID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncProduct(t.Context(), w.productID))

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("An active API key holds its permissions on its organization", func(t *testing.T) {
		w := givenSyncWorld(t)
		keyID := givenSyncAPIKey(t, w, w.permission)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.True(t, can(t, w.productID, model.APIKey(keyID), w.permission, w.organizationID))
	})

	t.Run("A role template two organizations still hold keeps granting when one lets it go", func(t *testing.T) {
		w := givenSyncWorld(t)
		other := givenOrganization(t, w.productID)
		givenMember(t, w, w.organizationID, w.templateID)
		elsewhere := givenMember(t, w, other, w.templateID)
		require.NoError(t, AuthzSyncer.SyncProduct(t.Context(), w.productID))
		_, err := TestDB.ExecContext(t.Context(),
			`DELETE FROM organization_memberships WHERE organization_id = $1`, w.organizationID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.True(t, can(t, w.productID, model.User(elsewhere), w.permission, other))
	})

	t.Run("A catalog that did not change keeps its model", func(t *testing.T) {
		w := givenSyncWorld(t)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		first, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		again, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Equal(t, first.Value().ModelID, again.Value().ModelID)
	})

	t.Run("Syncs of one product at once all finish", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		var wg sync.WaitGroup
		for range 5 {
			wg.Go(func() {
				assert.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
			})
		}
		wg.Wait()

		assert.True(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})
}

func TestAuthzReconcile(t *testing.T) {
	openFGA(t)

	t.Run("Reconcile removes the grants of a role that no longer exists", func(t *testing.T) {
		w := givenSyncWorld(t)
		givenMember(t, w, w.organizationID, w.templateID)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		found, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)
		orphan := &openfgav1.TupleKey{
			Object: model.Role("product_role_" + uuid.NewString()), Relation: model.GrantsRelation(w.permission),
			User: model.AllUsers,
		}
		_, err = openFGA(t).Write(t.Context(), &openfgav1.WriteRequest{
			StoreId: found.Value().StoreID, AuthorizationModelId: found.Value().ModelID,
			Writes: &openfgav1.WriteRequestWrites{TupleKeys: []*openfgav1.TupleKey{orphan}},
		})
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		read, err := openFGA(t).Read(t.Context(), &openfgav1.ReadRequest{
			StoreId: found.Value().StoreID, TupleKey: &openfgav1.ReadRequestTupleKey{Object: orphan.GetObject()},
		})
		require.NoError(t, err)
		assert.Empty(t, read.GetTuples())
	})
}

func TestAuthzMarks(t *testing.T) {
	openFGA(t)

	t.Run("A membership write leaves a mark that the sweeper replays", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)

		_, err := MembershipService.AddMember(t.Context(), organization.AddMemberInput{
			ProductID: w.productID, OrganizationID: w.organizationID, ProductUserID: userID, RoleID: w.templateID,
		})
		require.NoError(t, err)
		marks, err := AuthzRepo.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		require.NotEmpty(t, marks)

		AuthzSweeper.Sweep(t.Context())

		assert.True(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
		left, err := AuthzRepo.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})
}

func TestAuthzReplayAfterCommit(t *testing.T) {
	openFGA(t)

	t.Run("A write's request replays its organization right after it commits", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)
		ctx, written := authzservice.CollectWrites(t.Context())

		_, err := MembershipService.AddMember(ctx, organization.AddMemberInput{
			ProductID: w.productID, OrganizationID: w.organizationID, ProductUserID: userID, RoleID: w.templateID,
		})
		require.NoError(t, err)
		AuthzReplayer.Replay(t.Context(), written())

		assert.Equal(t, []authzservice.OrganizationRef{{ProductID: w.productID, OrganizationID: w.organizationID}},
			written())
		assert.True(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
		left, err := AuthzRepo.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})

	t.Run("A member add refused before it writes leaves no mark and collects nothing", func(t *testing.T) {
		w := givenSyncWorld(t)
		ctx, written := authzservice.CollectWrites(t.Context())

		_, err := MembershipService.AddMember(ctx, organization.AddMemberInput{
			ProductID:      w.productID,
			OrganizationID: w.organizationID,
			ProductUserID:  "pusr_missing",
			RoleID:         w.templateID,
		})
		require.Error(t, err)

		assert.Empty(t, written())
		left, err := AuthzRepo.Pending(t.Context(), w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})
}

func TestAuthzOrphanStores(t *testing.T) {
	openfga := openFGA(t)

	t.Run("A store whose product is gone is deleted by a reconcile pass", func(t *testing.T) {
		orphan, err := openfga.CreateStore(
			t.Context(),
			&openfgav1.CreateStoreRequest{Name: "prd_gone_" + uuid.NewString()},
		)
		require.NoError(t, err)
		w := givenSyncWorld(t)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		_, err = TestDB.ExecContext(t.Context(), `UPDATE authz_stores SET reconciled_at = NULL`)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			AuthzSweeper.Sweep(t.Context())
			_, getErr := openfga.GetStore(t.Context(), &openfgav1.GetStoreRequest{StoreId: orphan.GetId()})
			return getErr != nil
		}, 30*time.Second, 10*time.Millisecond)
	})
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

func TestAuthzSyncCases(t *testing.T) {
	openFGA(t)

	t.Run("A member who changes role holds only the new role's permissions", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		other := givenPermission(t, w.productID)
		otherRole := role.ProductRole{
			ProductID:      w.productID,
			Name:           "Writer " + uuid.NewString(),
			ResourceTypeID: organizationTypeID(t, w.productID),
		}
		otherRole.Key = strings.ToLower(otherRole.Name)
		otherRole.GenerateID()
		_, err := ProductRoleRepo.Create(t.Context(), otherRole)
		require.NoError(t, err)
		givenRolePermission(t, w.productID, otherRole.ID, other)
		_, err = TestDB.ExecContext(t.Context(), `UPDATE organization_memberships SET product_role_id = $1
			WHERE organization_id = $2`, otherRole.ID, w.organizationID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncProduct(t.Context(), w.productID))

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
		assert.True(t, can(t, w.productID, model.User(member), other, w.organizationID))
	})

	t.Run("A deleted organization implies nothing any more", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		_, err := TestDB.ExecContext(t.Context(), `DELETE FROM organizations WHERE id = $1`, w.organizationID)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.False(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})

	t.Run("An inactive API key holds nothing", func(t *testing.T) {
		w := givenSyncWorld(t)
		keyID := givenSyncAPIKey(t, w, w.permission)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		_, err := TestDB.ExecContext(
			t.Context(),
			`UPDATE organization_api_keys SET status = 'INACTIVE' WHERE id = $1`,
			keyID,
		)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))

		assert.False(t, can(t, w.productID, model.APIKey(keyID), w.permission, w.organizationID))
	})

	t.Run("A deleted permission leaves the model and every grant", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)
		kept := givenPermission(t, w.productID)
		givenRolePermission(t, w.productID, w.templateID, kept)
		require.NoError(t, AuthzSyncer.SyncProduct(t.Context(), w.productID))
		before, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)
		_, err = TestDB.ExecContext(t.Context(), `DELETE FROM product_resource_permissions
			WHERE product_id = $1 AND name = $2`, w.productID, w.permission)
		require.NoError(t, err)

		require.NoError(t, AuthzSyncer.SyncProduct(t.Context(), w.productID))

		after, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)
		assert.NotEqual(t, before.Value().ModelID, after.Value().ModelID)
		assert.True(t, can(t, w.productID, model.User(member), kept, w.organizationID))
	})

	t.Run("A product never written to gets its store from the reconcile", func(t *testing.T) {
		w := givenSyncWorld(t)
		member := givenMember(t, w, w.organizationID, w.templateID)

		require.Eventually(t, func() bool {
			AuthzSweeper.Sweep(t.Context())
			found, err := AuthzRepo.FindStore(t.Context(), w.productID)
			return err == nil && found.IsPresent() && found.Value().ReconciledAt != nil
		}, 60*time.Second, 10*time.Millisecond)
		assert.True(t, can(t, w.productID, model.User(member), w.permission, w.organizationID))
	})
}

func TestAuthzMarkRevisions(t *testing.T) {
	t.Run("A mark made again after a sync cleared it is not cleared by an older sync", func(t *testing.T) {
		ctx := t.Context()
		w := givenSyncWorld(t)
		require.NoError(t, Transactor.InTx(ctx, func(txCtx context.Context) error {
			return AuthzRepo.Mark(txCtx, w.productID, w.organizationID)
		}))
		read, err := AuthzRepo.Pending(ctx, w.productID)
		require.NoError(t, err)
		require.Len(t, read, 1)
		require.NoError(t, AuthzRepo.Clear(ctx, read[0]))
		require.NoError(t, AuthzRepo.Mark(ctx, w.productID, w.organizationID))

		require.NoError(t, AuthzRepo.Clear(ctx, read[0]))

		left, err := AuthzRepo.Pending(ctx, w.productID)
		require.NoError(t, err)
		assert.Len(t, left, 1)
	})

	t.Run("A product whose sync failed waits behind the others", func(t *testing.T) {
		ctx := t.Context()
		failing := givenSyncWorld(t)
		healthy := givenSyncWorld(t)
		require.NoError(t, AuthzRepo.Mark(ctx, failing.productID, failing.organizationID))
		require.NoError(t, AuthzRepo.Mark(ctx, healthy.productID, healthy.organizationID))

		require.NoError(t, AuthzRepo.PostponeInternal(ctx, failing.productID, time.Now().Add(time.Hour)))

		oldest, err := AuthzRepo.OldestInternal(ctx, 1000)
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
			if markErr := AuthzRepo.Mark(txCtx, w.productID, w.organizationID); markErr != nil {
				return markErr
			}
			return errRollback
		})

		require.ErrorIs(t, err, errRollback)
		left, err := AuthzRepo.Pending(ctx, w.productID)
		require.NoError(t, err)
		assert.Empty(t, left)
	})
}

var errRollback = errors.New("roll back")
