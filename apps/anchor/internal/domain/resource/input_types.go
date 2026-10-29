package resource

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/searchx"
)

// PutInput creates, renames or moves one resource. Parent is always named:
// the Organization is a parent like any other, never a default.
type PutInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Type           string `validate:"required,max=49"`
	ID             string `validate:"required,max=200"`
	Name           string `validate:"required,notblank,max=200"`
	Parent         Ref
}

func (in PutInput) Validate() error {
	if !ValidID(in.ID) {
		return NewInvalidIDError(in.ID)
	}
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}
	if !in.Parent.IsOrganization() && !ValidID(in.Parent.ID) {
		return NewInvalidIDError(in.Parent.ID)
	}
	return nil
}

type Filter struct {
	ResourceTypes []string `validate:"omitempty,max=50,dive,required,max=49"`
	Parent        *Ref
	IDs           []string `validate:"omitempty,max=100,dive,required,max=200"`
	NameContains  *string  `validate:"omitempty,min=1,max=200"`
}

type SortField string

const (
	SortFieldCreatedAt SortField = "created_at"
	SortFieldName      SortField = "name"
)

type SearchInput struct {
	ProductID      string                            `validate:"required,notblank"`
	OrganizationID string                            `validate:"required,notblank"`
	Request        search.Request[Filter, SortField] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request, SortFieldCreatedAt, SortFieldName)
}
