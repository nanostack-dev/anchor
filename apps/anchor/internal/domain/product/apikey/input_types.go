package apikey

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

// CreateProductAPIKeyInput represents input for creating a product API key.
type CreateProductAPIKeyInput struct {
	ProductID   string  `validate:"required,notblank"`
	Name        string  `validate:"required,notblank,max=100"`
	Description *string `validate:"omitempty,max=500"`
	Mutable     bool
	Permissions []string `validate:"required,dive,notblank,min=1"`
}

type GetProductAPIKeyInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type UpdateProductAPIKeyInput struct {
	ProductID   string    `validate:"required,notblank"`
	ID          string    `validate:"required,notblank"`
	Name        *string   `validate:"omitempty,notblank,max=100"`
	Description *string   `validate:"omitempty,max=500"`
	Permissions *[]string `validate:"omitempty,dive,notblank"`
}

type DeleteProductAPIKeyInput struct {
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

type SortFieldProductAPIKey string

const (
	SortFieldProductAPIKeyCreatedAt SortFieldProductAPIKey = "created_at"
	SortFieldProductAPIKeyUpdatedAt SortFieldProductAPIKey = "updated_at"
	SortFieldProductAPIKeyName      SortFieldProductAPIKey = "name"
	SortFieldProductAPIKeyStatus    SortFieldProductAPIKey = "status"
	SortFieldProductAPIKeyLastUsed  SortFieldProductAPIKey = "last_used_at"
)

type SearchProductAPIKeysInput struct {
	ProductID string `validate:"required,notblank"`
	Request   search.Request[SearchProductAPIKeyFilter, SortFieldProductAPIKey]
}

type SearchProductAPIKeyFilter struct {
	ProductAPIKeyIDs []string `validate:"omitempty,dive"`
	Names            []string `validate:"omitempty,dive,min=1"`
	Status           []string `validate:"omitempty,dive,oneof=ACTIVE INACTIVE"`
	LastUsedBefore   *time.Time
	LastUsedAfter    *time.Time
}

type ValidateAPIKeyScopesInput struct {
	ProductID   string   `validate:"required,notblank"`
	Scopes      []string `validate:"required,dive,notblank"`
	APIKeyValue string   `validate:"required,notblank"`
}
