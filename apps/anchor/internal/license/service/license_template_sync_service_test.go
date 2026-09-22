package service_test

import (
	"testing"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"anchor/internal/license/service"
)

func TestTemplateSyncRejectsMalformedPayload(t *testing.T) {
	sync := service.NewLicenseTemplateSyncService(nil, nil, nil, nil, nil, nil, nil, nil, zerolog.Nop())
	for _, payload := range []string{
		`{`, `{}`, `{"tenant_id":" ","product_id":"prd_1","template_id":"ltpl_1"}`,
	} {
		err := sync.ProcessQueueJob(t.Context(), queue.Job{Payload: []byte(payload)})
		require.Error(t, err)
		require.True(t, queue.IsNonRetryable(err))
	}
}
