package dslfactory

import (
	"context"
	"net/http"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

func NewNoAuthClient(
	t require.TestingT, serverURL string,
) *nanostackClient.ClientWithResponses {
	client, err := nanostackClient.NewClientWithResponses(serverURL)
	require.NoError(t, err)

	return client
}

func NewBearerClient(
	t require.TestingT, serverURL string, accessToken string,
) *nanostackClient.ClientWithResponses {
	client, err := nanostackClient.NewClientWithResponses(
		serverURL,
		nanostackClient.WithRequestEditorFn(
			func(_ context.Context, req *http.Request) error {
				req.Header.Add("Authorization", "Bearer "+accessToken)
				return nil
			},
		),
	)
	require.NoError(t, err)

	return client
}
