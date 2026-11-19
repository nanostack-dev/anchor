package service_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/product/user"
	"anchor/internal/repository"
	"anchor/internal/service"
)

// productUsersByEmail embeds a nil repository, so any method it does not
// override (a scan of every product user included) panics.
type productUsersByEmail struct {
	repository.ProductUserRepository

	existing user.ProductUser
}

func (r productUsersByEmail) FindByProductAndEmail(
	_ context.Context, productID, email string,
) (functional.Option[user.ProductUser], error) {
	if productID == r.existing.ProductID && email == r.existing.Email {
		return functional.Some(r.existing), nil
	}
	return functional.None[user.ProductUser](), nil
}

func (r productUsersByEmail) FindByExternalID(
	_ context.Context, productID, externalID string,
) (functional.Option[user.ProductUser], error) {
	if r.existing.ExternalID != nil && productID == r.existing.ProductID && externalID == *r.existing.ExternalID {
		return functional.Some(r.existing), nil
	}
	return functional.None[user.ProductUser](), nil
}

// Create looks the email up on its unique index instead of loading every user
// of the product, and an email conflict still wins over an external ID one.
func TestCreateProductUserLooksUpTheEmailWithoutScanningTheProduct(t *testing.T) {
	externalID := "ext-1"
	existing := user.ProductUser{
		ID: "pu_1", ProductID: "product", Email: "garbage+dup@nanostack.dev",
		ExternalID: &externalID, Status: user.ProductUserStatusActive,
	}
	svc := service.NewProductUserService(
		productUsersByEmail{existing: existing}, nil, nil, inlineTransactor{}, nil,
		slog.New(slog.DiscardHandler),
	)

	_, err := svc.Create(context.Background(), user.CreateProductUserInput{
		ProductID: "product", Email: existing.Email, ExternalID: &externalID,
		Status: user.ProductUserStatusActive,
	})

	require.ErrorIs(t, err, service.ErrProductUserEmailAlreadyExists)
}
