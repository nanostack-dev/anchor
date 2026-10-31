package service_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/product"
	"anchor/internal/repository"
	"anchor/internal/service"
)

type protectedProducts struct {
	repository.ProductRepository
}

func (protectedProducts) LockByID(context.Context, string, string) error { return nil }

func (protectedProducts) FindByID(
	_ context.Context, _, id string,
) (functional.Option[product.Product], error) {
	return functional.Some(product.Product{
		ID: id, Config: product.Config{Protected: true},
	}), nil
}

type inlineTransactor struct{}

func (inlineTransactor) InTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// A refused delete is a 409 the caller must fix, so it must not log at error.
func TestDeleteProtectedProductDoesNotLogAtError(t *testing.T) {
	var buf bytes.Buffer
	svc := service.NewProductService(
		protectedProducts{}, nil, nil, nil, nil, inlineTransactor{}, zerolog.New(&buf),
	)

	err := svc.Delete(context.Background(), product.DeleteProductInput{TenantID: "tenant", ProductID: "product"})

	require.ErrorIs(t, err, product.ErrProductProtected)
	assert.Contains(t, buf.String(), `"message":"failed to delete product"`)
	assert.Contains(t, buf.String(), `"status":409`)
	assert.NotContains(t, buf.String(), `"level":"error"`)
}
