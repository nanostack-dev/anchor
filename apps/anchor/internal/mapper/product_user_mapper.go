package mapper

import (
	"fmt"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type ProductUserMapper struct{}

// Removed static interface check

func NewProductUserMapper() *ProductUserMapper { // Return concrete type
	return &ProductUserMapper{}
}

// ToDomain returns an error for a row whose status is outside the contract
// enum, so a bad row fails loudly instead of reaching a response.
func (m *ProductUserMapper) ToDomain(entity model.ProductUsers) (user.ProductUser, error) {
	name := functional.FromPtr(entity.Name).OrElse("")

	status, err := user.ParseProductUserStatus(entity.Status)
	if err != nil {
		return user.ProductUser{}, fmt.Errorf("product user %s: %w", entity.ID, err)
	}

	return user.ProductUser{
		ID:         entity.ID,
		ProductID:  entity.ProductID,
		Email:      entity.Email,
		Name:       name,
		ExternalID: entity.ExternalID,
		Status:     status,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}, nil
}

func (m *ProductUserMapper) ToEntity(domain user.ProductUser) model.ProductUsers {
	name := functional.OptionOf(domain.Name, domain.Name != "").ToPtr()

	return model.ProductUsers{
		ID:         domain.ID,
		ProductID:  domain.ProductID,
		Email:      domain.Email,
		Name:       name,
		ExternalID: domain.ExternalID,
		Status:     string(domain.Status),
		CreatedAt:  domain.CreatedAt,
		UpdatedAt:  domain.UpdatedAt,
	}
}
