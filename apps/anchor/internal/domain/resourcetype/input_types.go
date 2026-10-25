package resourcetype

import (
	"regexp"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/searchx"
)

type CreateInput struct {
	ProductID     string   `validate:"required,notblank"`
	Slug          string   `validate:"required,max=49"`
	Name          string   `validate:"required,notblank,max=100"`
	Description   *string  `validate:"omitempty,max=500"`
	ParentTypes   []string `validate:"required,min=1,max=10,unique,dive,required"`
	NestsInItself bool
}

type UpdateInput struct {
	ProductID     string   `validate:"required,notblank"`
	ID            string   `validate:"required,notblank"`
	Name          *string  `validate:"omitempty,notblank,max=100"`
	Description   *string  `validate:"omitempty,max=500"`
	ParentTypes   []string `validate:"omitempty,min=1,max=10,unique,dive,required"`
	NestsInItself *bool
}

// TreeItem is one type of a whole-tree replacement, matched by slug.
type TreeItem struct {
	Slug          string   `validate:"required,max=49"`
	Name          string   `validate:"required,notblank,max=100"`
	Description   *string  `validate:"omitempty,max=500"`
	ParentTypes   []string `validate:"required,min=1,max=10,unique,dive,required"`
	NestsInItself bool
}

type ReplaceTreeInput struct {
	ProductID string     `validate:"required,notblank"`
	Types     []TreeItem `validate:"max=50,unique=Slug,dive"`
}

type Filter struct {
	IDs   []string `validate:"omitempty,max=100,dive,notblank"`
	Slugs []string `validate:"omitempty,max=100,dive,notblank"`
}

type SortField string

const (
	SortFieldCreatedAt SortField = "created_at"
	SortFieldUpdatedAt SortField = "updated_at"
	SortFieldSlug      SortField = "slug"
	SortFieldName      SortField = "name"
)

type SearchInput struct {
	ProductID string                            `validate:"required,notblank"`
	Request   search.Request[Filter, SortField] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldCreatedAt, SortFieldUpdatedAt, SortFieldSlug, SortFieldName)
}

// slugPattern keeps a slug a valid OpenFGA type name (ADR-0022 A1).
var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,48}$`)

func NewSlugInvalidError(slug string) *fault.Error {
	return fault.BadRequest(
		"RESOURCE_TYPE_SLUG_INVALID",
		"A resource type slug starts with a lowercase letter and holds only lowercase letters, digits and underscores, 49 characters at most.",
	).Metadata(map[string]any{metadataSlug: slug})
}

func validateSlugs(slugs ...string) error {
	for _, slug := range slugs {
		if !slugPattern.MatchString(slug) {
			return NewSlugInvalidError(slug)
		}
	}
	return nil
}

func (in CreateInput) Validate() error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}
	return validateSlugs(append([]string{in.Slug}, in.ParentTypes...)...)
}

func (in UpdateInput) Validate() error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}
	if in.ParentTypes != nil && len(in.ParentTypes) == 0 {
		return ErrParentTypesRequired
	}
	return validateSlugs(in.ParentTypes...)
}

func (in ReplaceTreeInput) Validate() error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}
	for _, item := range in.Types {
		if err := validateSlugs(append([]string{item.Slug}, item.ParentTypes...)...); err != nil {
			return err
		}
		if item.Slug == OrganizationSlug {
			return ErrBuiltIn
		}
	}
	return nil
}
