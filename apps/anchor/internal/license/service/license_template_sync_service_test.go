package service_test

import (
	"log/slog"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/redis/nanocache"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
	"anchor/internal/license/service"
)

func TestTemplateSyncRejectsMalformedPayload(t *testing.T) {
	sync := service.NewLicenseTemplateSyncService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nanocache.Cache[license.OrganizationLicense]{},
		nil,
		slog.New(slog.DiscardHandler),
	)
	for _, payload := range []string{
		`{`, `{}`, `{"tenant_id":" ","product_id":"prd_1","template_id":"ltpl_1"}`,
	} {
		err := sync.ProcessQueueJob(t.Context(), queue.Job{Payload: []byte(payload)})
		require.Error(t, err)
		require.True(t, queue.IsNonRetryable(err))
	}
}
