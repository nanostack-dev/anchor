package roleassignment

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/searchx"
)

// Target names one member on one registered resource, as the path does.
type Target struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ResourceType   string `validate:"required,max=49"`
	ResourceID     string `validate:"required,max=200"`
	ProductUserID  string `validate:"required,notblank,max=255"`
}

// Empty reports whether the path left an identifier out.
func (t Target) Empty() bool {
	return t.ProductID == "" || t.OrganizationID == "" || t.ResourceType == "" || t.ResourceID == "" ||
		t.ProductUserID == ""
}

// PutInput replaces the roles a member holds on a resource.
type PutInput struct {
	Target
	RoleIDs []string `validate:"required,min=1,max=10,dive,required,notblank,max=255"`
}

func (in PutInput) Validate() error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}
	if len(in.RoleIDs) > MaxRolesPerResource {
		return NewSingleRoleError(len(in.RoleIDs))
	}
	return nil
}

type Filter struct {
	ProductUserIDs []string `validate:"omitempty,max=100,dive,required,max=255"`
	RoleIDs        []string `validate:"omitempty,max=50,dive,required,max=255"`
}

type SortField string

const (
	SortFieldCreatedAt     SortField = "created_at"
	SortFieldProductUserID SortField = "product_user_id"
)

type SearchInput struct {
	ProductID      string                            `validate:"required,notblank"`
	OrganizationID string                            `validate:"required,notblank"`
	ResourceType   string                            `validate:"required,max=49"`
	ResourceID     string                            `validate:"required,max=200"`
	Request        search.Request[Filter, SortField] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request, SortFieldCreatedAt, SortFieldProductUserID)
}
