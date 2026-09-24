package events //nolint:testpackage // exercise the private persistence decoder

import (
	"testing"

	"anchor/internal/db/gen/anchor/public/model"

	"github.com/stretchr/testify/require"
)

func TestEndpointFromModelRejectsInvalidSubscriptions(t *testing.T) {
	t.Parallel()

	for _, encoded := range []string{"null", "{}", `"organization.created"`, "not json"} {
		_, err := endpointFromModel(model.ProductEventEndpointConfigs{ProductID: "prd_test", EventsJSON: encoded})
		require.Error(t, err, encoded)
	}

	endpoint, err := endpointFromModel(model.ProductEventEndpointConfigs{
		ProductID: "prd_test", EventsJSON: `["organization.created"]`,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"organization.created"}, endpoint.Events)
}
