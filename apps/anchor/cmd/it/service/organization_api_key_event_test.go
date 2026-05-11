package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orgapikey "anchor/internal/domain/organization/apikey"
)

const organizationAPIKeyEventQueueName = "organization_api_key_event"

func TestOrganizationAPIKeyExpirationEvents(t *testing.T) {
	t.Run("Create enqueues expiration event when expires_at is set", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

		createdKey, _ := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			&expiresAt,
		)

		job := findOrganizationAPIKeyExpirationJob(t, ctxData.Organization.ID, createdKey.ID)
		assert.NotNil(t, job)
	})

	t.Run("Expiration event marks api key inactive when processed", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

		createdKey, _ := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			&expiresAt,
		)

		job := findOrganizationAPIKeyExpirationJob(t, ctxData.Organization.ID, createdKey.ID)
		require.NotNil(t, job)

		createdKey.ExpiresAt = ptrTime(time.Now().UTC().Add(-time.Minute))
		updated, updateErr := OrgAPIKeyRepository.Update(t.Context(), createdKey, nil)
		require.NoError(t, updateErr)

		processErr := APIKeyEventSvc.ProcessQueueJob(context.Background(), *job)
		require.NoError(t, processErr)

		reloaded, reloadErr := OrgAPIKeyRepository.GetByID(t.Context(), ctxData.Organization.ID, updated.ID, nil)
		require.NoError(t, reloadErr)
		require.NotNil(t, reloaded)
		assert.Equal(t, orgapikey.StatusInactive, reloaded.Status)
	})

	t.Run("Delete removes pending expiration event job", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

		createdKey, _ := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			&expiresAt,
		)

		job := findOrganizationAPIKeyExpirationJob(t, ctxData.Organization.ID, createdKey.ID)
		require.NotNil(t, job)

		deleteErr := OrgAPIKeyService.Delete(
			t.Context(),
			orgapikey.DeleteOrganizationAPIKeyInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				ID:             createdKey.ID,
			},
		)
		require.NoError(t, deleteErr)

		deletedKey, getErr := OrgAPIKeyRepository.GetByID(t.Context(), ctxData.Organization.ID, createdKey.ID, nil)
		require.NoError(t, getErr)
		assert.Nil(t, deletedKey)

		remainingJob := findOrganizationAPIKeyExpirationJob(t, ctxData.Organization.ID, createdKey.ID)
		assert.Nil(t, remainingJob)
	})
}

func findOrganizationAPIKeyExpirationJob(t *testing.T, organizationID, apiKeyID string) *pgqueue.Job {
	t.Helper()

	jobs, err := Queue.ListJobs(context.Background(), pgqueue.ListJobsParams{
		QueueName: organizationAPIKeyEventQueueName,
		Status:    pgqueue.StatusPending,
		Limit:     1000,
	})
	require.NoError(t, err)

	for _, job := range jobs {
		var payload struct {
			EventType      string `json:"event_type"`
			OrganizationID string `json:"organization_id"`
			APIKeyID       string `json:"api_key_id"`
		}
		if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr != nil {
			continue
		}
		if payload.EventType == "expiration" &&
			payload.OrganizationID == organizationID &&
			payload.APIKeyID == apiKeyID {
			jobCopy := job
			return &jobCopy
		}
	}

	return nil
}

func ptrTime(v time.Time) *time.Time {
	return &v
}
