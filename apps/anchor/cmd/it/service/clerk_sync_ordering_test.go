package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/integration"
	productuser "anchor/internal/domain/product/user"
	"anchor/internal/integration/provider"
	"anchor/internal/integration/provider/clerk"
	"anchor/internal/repository"
)

func givenClerkInstance(t *testing.T, tenantID, productID string) *integration.Instance {
	t.Helper()
	secret := "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	inst := integration.Instance{
		PlatformTenantID: tenantID,
		ProductID:        productID,
		ProviderType:     integration.ProviderTypeClerk,
		Status:           integration.StatusActive,
		IsEnabled:        true,
		ConfigVersion:    1,
		ConfigJSON:       []byte(`{}`),
		WebhookSecret:    &secret,
	}
	inst.GenerateID()
	created, err := IntegrationInstanceRepo.Create(t.Context(), inst)
	require.NoError(t, err)
	return &created
}

func TestClerkSyncOrderingAndConflicts(t *testing.T) {
	t.Run(
		"Verified Clerk upsert adopts an existing NULL external_id row and preserves suspended status",
		func(t *testing.T) {
			ctx := GivenATenantAndProduct(t)
			inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

			email := "adopt-" + uuid.NewString() + "@example.com"
			invited := productuser.ProductUser{
				ProductID: ctx.Product.ID,
				Email:     email,
				Name:      "Pre Created",
				Status:    productuser.ProductUserStatusSuspended,
			}
			invited.GenerateID()
			created, err := ProductUserRepo.Create(t.Context(), invited)
			require.NoError(t, err)

			extID := "user_clerk_" + uuid.NewString()
			t1 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
			err = ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
				Type: clerk.CommandUpsertUser,
				Data: clerk.UpsertUserData{
					ExternalID:    extID,
					Email:         email,
					EmailVerified: true,
					Name:          "Adopted Name",
					EventAt:       &t1,
					Source:        clerk.SyncSourceWebhook,
				},
			})
			require.NoError(t, err)

			found, err := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, extID)
			require.NoError(t, err)
			require.True(t, found.IsPresent())
			got := found.Value()
			assert.Equal(t, created.ID, got.ID, "adoption must preserve the existing product_users ID")
			assert.Equal(t, "Adopted Name", got.Name)
			assert.Equal(
				t,
				productuser.ProductUserStatusSuspended,
				got.Status,
				"adoption must preserve suspended status",
			)
			require.NotNil(t, got.LastSyncedEventAt)
			assert.Equal(t, t1, got.LastSyncedEventAt.UTC())
		},
	)

	t.Run("Unverified Clerk upsert does not adopt an existing NULL external_id row", func(t *testing.T) {
		ctx := GivenATenantAndProduct(t)
		inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

		email := "unverified-" + uuid.NewString() + "@example.com"
		invited := productuser.ProductUser{
			ProductID: ctx.Product.ID,
			Email:     email,
			Name:      "Invited Person",
			Status:    productuser.ProductUserStatusInvited,
		}
		invited.GenerateID()
		created, err := ProductUserRepo.Create(t.Context(), invited)
		require.NoError(t, err)

		extID := "user_clerk_" + uuid.NewString()
		t1 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
		err = ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
			Type: clerk.CommandUpsertUser,
			Data: clerk.UpsertUserData{
				ExternalID:    extID,
				Email:         email,
				EmailVerified: false,
				Name:          "Attacker Name",
				EventAt:       &t1,
				Source:        clerk.SyncSourceWebhook,
			},
		})
		require.NoError(t, err)

		byExt, err := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, extID)
		require.NoError(t, err)
		assert.True(t, byExt.IsAbsent(), "unverified Clerk email must not link to existing row")

		stillInvited, err := ProductUserRepo.FindByProductIDAndID(t.Context(), ctx.Product.ID, created.ID)
		require.NoError(t, err)
		require.True(t, stillInvited.IsPresent())
		assert.Nil(t, stillInvited.Value().ExternalID)
	})

	t.Run(
		"Email conflict with different external_id fails retryably on webhook and skips on reconcile",
		func(t *testing.T) {
			ctx := GivenATenantAndProduct(t)
			inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

			email := "conflict-" + uuid.NewString() + "@example.com"
			holderExt := "user_clerk_holder_" + uuid.NewString()
			t1 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
			require.NoError(t, ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
				Type: clerk.CommandUpsertUser,
				Data: clerk.UpsertUserData{
					ExternalID:    holderExt,
					Email:         email,
					EmailVerified: true,
					Name:          "Original Holder",
					EventAt:       &t1,
					Source:        clerk.SyncSourceWebhook,
				},
			}))

			otherExt := "user_clerk_other_" + uuid.NewString()
			t2 := t1.Add(time.Minute)

			// Webhook path returns typed retryable ErrProductUserEmailConflict.
			webhookErr := ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
				Type: clerk.CommandUpsertUser,
				Data: clerk.UpsertUserData{
					ExternalID:    otherExt,
					Email:         email,
					EmailVerified: true,
					Name:          "Conflicting User",
					EventAt:       &t2,
					Source:        clerk.SyncSourceWebhook,
				},
			})
			require.ErrorIs(t, webhookErr, repository.ErrProductUserEmailConflict)

			// Reconcile path skips without aborting a shared batch transaction so
			// remaining users in the same batch commit cleanly.
			thirdExt := "user_clerk_third_" + uuid.NewString()
			thirdEmail := "third-" + uuid.NewString() + "@example.com"
			require.NoError(t, Transactor.InTx(t.Context(), func(txCtx context.Context) error {
				if err := ClerkProvider.ExecuteCommand(txCtx, TestLogger, inst, provider.Command{
					Type: clerk.CommandUpsertUser,
					Data: clerk.UpsertUserData{
						ExternalID:    otherExt,
						Email:         email,
						EmailVerified: true,
						Name:          "Conflicting User",
						EventAt:       &t2,
						Source:        clerk.SyncSourceReconcile,
					},
				}); err != nil {
					return err
				}
				return ClerkProvider.ExecuteCommand(txCtx, TestLogger, inst, provider.Command{
					Type: clerk.CommandUpsertUser,
					Data: clerk.UpsertUserData{
						ExternalID:    thirdExt,
						Email:         thirdEmail,
						EmailVerified: true,
						Name:          "Healthy User",
						EventAt:       &t2,
						Source:        clerk.SyncSourceReconcile,
					},
				})
			}))

			third, err := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, thirdExt)
			require.NoError(t, err)
			assert.True(t, third.IsPresent())
		},
	)

	t.Run("API Update preserves last_synced_event_at", func(t *testing.T) {
		ctx := GivenATenantAndProduct(t)
		inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

		extID := "user_clerk_" + uuid.NewString()
		email := "preserve-" + uuid.NewString() + "@example.com"
		t1 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
		require.NoError(t, ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
			Type: clerk.CommandUpsertUser,
			Data: clerk.UpsertUserData{
				ExternalID:    extID,
				Email:         email,
				EmailVerified: true,
				Name:          "Clerk Name",
				EventAt:       &t1,
				Source:        clerk.SyncSourceWebhook,
			},
		}))

		found, err := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, extID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
		toUpdate := found.Value()
		toUpdate.Name = "API Edited Name"
		toUpdate.LastSyncedEventAt = nil // simulate API Update struct without stamp

		updated, err := ProductUserRepo.Update(t.Context(), ctx.Product.ID, toUpdate.ID, toUpdate)
		require.NoError(t, err)
		require.NotNil(t, updated.LastSyncedEventAt, "API Update must not reset last_synced_event_at to NULL")
		assert.Equal(t, t1, updated.LastSyncedEventAt.UTC())
	})

	t.Run("Concurrent delete and stale upsert never resurrect the user", func(t *testing.T) {
		ctx := GivenATenantAndProduct(t)
		inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

		extID := "user_clerk_race_" + uuid.NewString()
		email := "race-del-" + uuid.NewString() + "@example.com"
		tCreate := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
		tDelete := tCreate.Add(time.Second)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		wg.Go(func() {
			<-start
			errs[0] = ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
				Type: clerk.CommandDeleteUser,
				Data: clerk.DeleteUserData{
					ExternalID: extID,
					EventAt:    &tDelete,
				},
			})
		})
		wg.Go(func() {
			<-start
			errs[1] = ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
				Type: clerk.CommandUpsertUser,
				Data: clerk.UpsertUserData{
					ExternalID:    extID,
					Email:         email,
					EmailVerified: true,
					Name:          "Racing User",
					EventAt:       &tCreate,
					Source:        clerk.SyncSourceWebhook,
				},
			})
		})
		close(start)
		wg.Wait()

		require.NoError(t, errs[0])
		require.NoError(t, errs[1])

		// Regardless of which goroutine won the advisory lock first, once both
		// finish the older create must not leave a live user behind.
		after, err := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, extID)
		require.NoError(t, err)
		assert.True(t, after.IsAbsent(), "deleted user must not be resurrected by a concurrent older create")

		tomb, err := ProductUserRepo.FindTombstone(t.Context(), ctx.Product.ID, extID)
		require.NoError(t, err)
		require.True(t, tomb.IsPresent())
		assert.Equal(t, tDelete, tomb.Value().UTC())
	})

	t.Run("Deletion candidate query includes only Clerk-stamped rows before snapshotStart", func(t *testing.T) {
		ctx := GivenATenantAndProduct(t)
		inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

		// 1. API-created user with external_id but NULL last_synced_event_at (must be excluded).
		apiExt := "api_ext_" + uuid.NewString()
		apiUser := productuser.ProductUser{
			ProductID:  ctx.Product.ID,
			Email:      "api-" + uuid.NewString() + "@example.com",
			Name:       "API User",
			ExternalID: &apiExt,
			Status:     productuser.ProductUserStatusActive,
		}
		apiUser.GenerateID()
		_, err := ProductUserRepo.Create(t.Context(), apiUser)
		require.NoError(t, err)

		// 2. Clerk-synced user stamped before snapshotStart (must be included).
		oldExt := "user_clerk_old_" + uuid.NewString()
		tBefore := time.Now().UTC().Add(-time.Hour)
		require.NoError(t, ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
			Type: clerk.CommandUpsertUser,
			Data: clerk.UpsertUserData{
				ExternalID:    oldExt,
				Email:         "old-" + uuid.NewString() + "@example.com",
				EmailVerified: true,
				Name:          "Old Clerk User",
				EventAt:       &tBefore,
				Source:        clerk.SyncSourceReconcile,
			},
		}))

		snapshotStart := time.Now().UTC().Add(time.Minute)

		// 3. Clerk-synced user stamped after snapshotStart (must be excluded).
		newExt := "user_clerk_new_" + uuid.NewString()
		tAfter := snapshotStart.Add(time.Minute)
		require.NoError(t, ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
			Type: clerk.CommandUpsertUser,
			Data: clerk.UpsertUserData{
				ExternalID:    newExt,
				Email:         "new-" + uuid.NewString() + "@example.com",
				EmailVerified: true,
				Name:          "New Clerk User",
				EventAt:       &tAfter,
				Source:        clerk.SyncSourceReconcile,
			},
		}))

		stampedCount, err := ProductUserRepo.CountClerkStampedUsers(t.Context(), ctx.Product.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), stampedCount, "only the two Clerk-stamped users count")

		beforeIDs, err := ProductUserRepo.ListClerkStampedExternalIDsBefore(t.Context(), ctx.Product.ID, snapshotStart)
		require.NoError(t, err)
		assert.Equal(t, []string{oldExt}, beforeIDs,
			"only Clerk-stamped users synced and created before snapshotStart qualify")
	})

	t.Run(
		"Two consecutive reconciles against a swapped Clerk app abort both runs and preserve real users",
		func(t *testing.T) {
			ctx := GivenATenantAndProduct(t)
			inst := givenClerkInstance(t, ctx.Tenant.ID, ctx.Product.ID)

			// Seed two real Clerk-stamped users from the original Clerk app.
			tOriginal := time.Now().UTC().Add(-time.Hour)
			realIDs := []string{"user_real_1_" + uuid.NewString(), "user_real_2_" + uuid.NewString()}
			for _, extID := range realIDs {
				require.NoError(t, ClerkProvider.ExecuteCommand(t.Context(), TestLogger, inst, provider.Command{
					Type: clerk.CommandUpsertUser,
					Data: clerk.UpsertUserData{
						ExternalID:    extID,
						Email:         extID + "@example.com",
						EmailVerified: true,
						Name:          "Real User",
						EventAt:       &tOriginal,
						Source:        clerk.SyncSourceReconcile,
					},
				}))
			}
			// Backdate created_at so reconcile's real SnapshotStart is strictly after both rows.
			_, err := TestDB.ExecContext(
				t.Context(),
				`UPDATE product_users SET created_at = $1 WHERE product_id = $2`,
				tOriginal,
				ctx.Product.ID,
			)
			require.NoError(t, err)

			// Configure an encrypted Clerk API key on the instance and point the
			// Clerk provider at a stub server representing a different Clerk app.
			storedCfg, err := ClerkProvider.PrepareConfigForStorage(
				t.Context(),
				[]byte(`{"api_key":"sk_test_wrong_app"}`),
			)
			require.NoError(t, err)
			_, err = TestDB.ExecContext(
				t.Context(),
				`UPDATE integration_instances SET config_json = $1 WHERE id = $2`,
				storedCfg,
				inst.ID,
			)
			require.NoError(t, err)

			wrongID1 := "user_wrong_1_" + uuid.NewString()
			wrongID2 := "user_wrong_2_" + uuid.NewString()
			wrongUsersJSON, err := json.Marshal([]map[string]any{
				{
					"id":                       wrongID1,
					"primary_email_address_id": "em_1",
					"email_addresses": []map[string]any{{
						"id":            "em_1",
						"email_address": wrongID1 + "@wrong.example.com",
						"verification":  map[string]any{"status": "verified"},
					}},
					"updated_at": time.Now().UTC().UnixMilli(),
				},
				{
					"id":                       wrongID2,
					"primary_email_address_id": "em_2",
					"email_addresses": []map[string]any{{
						"id":            "em_2",
						"email_address": wrongID2 + "@wrong.example.com",
						"verification":  map[string]any{"status": "verified"},
					}},
					"updated_at": time.Now().UTC().UnixMilli(),
				},
			})
			require.NoError(t, err)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/users/count":
					_, _ = w.Write([]byte(`{"total_count":2}`))
				case "/v1/users":
					_, _ = w.Write(wrongUsersJSON)
				default:
					// Any per-ID GET for real users would return 404 on the wrong app.
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"errors":[{"code":"resource_not_found","message":"not found"}]}`))
				}
			}))
			defer srv.Close()
			t.Cleanup(ClerkProvider.OverrideEndpointForTesting(srv.URL+"/v1", srv.Client()))

			payload, err := json.Marshal(map[string]any{"integration_instance_id": inst.ID})
			require.NoError(t, err)
			job := queue.Job{Payload: payload}

			// Run the real IntegrationService reconcile path twice in a row.
			// If Run 1 failed to abort upserts, wrong-app users would be inserted
			// and Run 2 would see overlap and 404-delete both real users.
			require.NoError(t, IntegrationService.ProcessReconcileQueueJob(t.Context(), job))
			require.NoError(t, IntegrationService.ProcessReconcileQueueJob(t.Context(), job))

			// Wrong-app users were never inserted and both real users survived untouched.
			stamped, err := ProductUserRepo.CountClerkStampedUsers(t.Context(), ctx.Product.ID)
			require.NoError(t, err)
			assert.Equal(t, int64(2), stamped)

			for _, wrongID := range []string{wrongID1, wrongID2} {
				found, findErr := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, wrongID)
				require.NoError(t, findErr)
				assert.True(t, found.IsAbsent(), "wrong-app user %s must not be inserted", wrongID)
			}
			for _, realID := range realIDs {
				found, findErr := ProductUserRepo.FindByExternalID(t.Context(), ctx.Product.ID, realID)
				require.NoError(t, findErr)
				assert.True(t, found.IsPresent(), "real user %s must survive both aborted runs", realID)
				tomb, tombErr := ProductUserRepo.FindTombstone(t.Context(), ctx.Product.ID, realID)
				require.NoError(t, tombErr)
				assert.True(t, tomb.IsAbsent(), "real user %s must not be tombstoned", realID)
			}

			// Both aborted runs recorded a WARNING audit row with reason + aborted_run=true.
			var abortAuditCount int
			err = TestDB.QueryRowContext(
				t.Context(),
				`SELECT COUNT(*) FROM integration_audit_logs
				 WHERE integration_instance_id = $1
				   AND action = $2
				   AND severity = $3
				   AND metadata_json->>'reason' = 'no_overlap_with_existing_users'
				   AND (metadata_json->>'aborted_run')::boolean = true`,
				inst.ID,
				string(integration.AuditActionDeleteUser),
				string(integration.AuditSeverityWarning),
			).Scan(&abortAuditCount)
			require.NoError(t, err)
			assert.Equal(t, 2, abortAuditCount, "each aborted reconcile run must persist a warning audit row")
		},
	)
}
