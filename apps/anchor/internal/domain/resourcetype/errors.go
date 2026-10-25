package resourcetype

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

const (
	metadataSlug      = "slug"
	metadataBlockedBy = "blocked_by"
)

var (
	ErrNotFound = fault.NotFound(
		"RESOURCE_TYPE_NOT_FOUND",
		"This product has no resource type with that identifier.",
	)

	ErrBuiltIn = fault.Conflict(
		"BUILT_IN_RESOURCE_TYPE",
		"The organization resource type is built in and can't be deleted, renamed or moved.",
	)

	ErrRequired = fault.BadRequest(
		"RESOURCE_TYPE_REQUIRED",
		"Name the resource type this applies to.",
	)

	ErrParentTypesRequired = fault.BadRequest(
		"RESOURCE_TYPE_PARENT_REQUIRED",
		"Every resource type except organization has at least one parent type.",
	)

	ErrLimitReached = fault.Conflict(
		"RESOURCE_TYPE_LIMIT_REACHED",
		fmt.Sprintf("A product may declare at most %d resource types besides organization.", MaxOwnTypes),
	).Metadata(map[string]any{"max_resource_types": MaxOwnTypes})
)

// NewUnknownError reports slugs a request body named that the product has not declared.
func NewUnknownError(slugs []string) *fault.Error {
	return fault.BadRequest(
		"RESOURCE_TYPE_NOT_FOUND",
		"This product has no resource type with that slug.",
	).Metadata(map[string]any{"resource_types": slugs})
}

// CodeSlugDuplicate is the code of NewSlugExistsError.
const CodeSlugDuplicate = "RESOURCE_TYPE_SLUG_DUPLICATE"

func NewSlugExistsError(slug string) *fault.Error {
	return fault.Conflict(
		CodeSlugDuplicate,
		"This product already has a resource type with that slug.",
	).Metadata(map[string]any{metadataSlug: slug})
}

func NewCycleError(chain []string) *fault.Error {
	return fault.BadRequest(
		"RESOURCE_TYPE_CYCLE",
		"Resource types can't be each other's ancestors. Only a type that nests in itself may sit under its own type.",
	).Metadata(map[string]any{"chain": chain})
}

func NewTooDeepError(slug string, depth int) *fault.Error {
	return fault.BadRequest(
		"RESOURCE_TYPE_TOO_DEEP",
		fmt.Sprintf(
			"The resource type tree may be at most %d levels deep, counting organization. %s would be at level %d.",
			MaxDepth,
			slug,
			depth,
		),
	).Metadata(map[string]any{metadataSlug: slug, "depth": depth, "max_depth": MaxDepth})
}

func NewSelfParentError(slug string) *fault.Error {
	return fault.BadRequest(
		"RESOURCE_TYPE_SELF_PARENT",
		"A resource type never lists itself as a parent type. Set nests_in_itself instead.",
	).Metadata(map[string]any{metadataSlug: slug})
}

// Usage counts what keeps a type from being deleted or reshaped. Resources
// is left out when none block, so the counts a client read before resources
// existed keep their shape.
type Usage struct {
	ChildTypes  int `json:"child_types"`
	Roles       int `json:"roles"`
	Permissions int `json:"permissions"`
	Resources   int `json:"resources,omitempty"`
}

func (u Usage) InUse() bool {
	return u.ChildTypes > 0 || u.Roles > 0 || u.Permissions > 0 || u.Resources > 0
}

// NewResourcesUnderParentError refuses a change that would leave registered
// resources under a parent type their type no longer allows: a removed parent
// type, or its own type once nests_in_itself is off.
func NewResourcesUnderParentError(slug, parentType string, count int) *fault.Error {
	change := fmt.Sprintf("removing %s from its parent types", parentType)
	if parentType == slug {
		change = "turning nests_in_itself off"
	}
	sits := fmt.Sprintf("%d registered resources of type %s sit", count, slug)
	if count == 1 {
		sits = fmt.Sprintf("1 registered resource of type %s sits", slug)
	}
	return fault.Conflict(
		"RESOURCE_TYPE_IN_USE",
		fmt.Sprintf("%s under one of type %s. Move them before %s.", sits, parentType, change),
	).Metadata(map[string]any{
		metadataSlug: slug, "parent_type": parentType, metadataBlockedBy: Usage{Resources: count},
	})
}

func NewInUseError(slug string, usage Usage) *fault.Error {
	return fault.Conflict(
		"RESOURCE_TYPE_IN_USE",
		fmt.Sprintf("%s can't be deleted while other records use it.", slug),
	).Metadata(map[string]any{metadataSlug: slug, metadataBlockedBy: usage})
}

// NewRolesOutOfScopeError refuses a tree change that would leave roles
// bundling a permission no longer below their scope.
func NewRolesOutOfScopeError(slug string, roleIDs []string) *fault.Error {
	return fault.Conflict(
		"RESOURCE_TYPE_IN_USE",
		fmt.Sprintf(
			"Roles would bundle permissions outside their scope if %s changed this way. Remove those permissions from the roles first.",
			slug,
		),
	).Metadata(map[string]any{metadataSlug: slug, metadataBlockedBy: map[string]any{"roles": len(roleIDs)}, "role_ids": roleIDs})
}

// NewTreeRolesOutOfScopeError is NewRolesOutOfScopeError for a whole-tree
// replacement, where no single type is to blame.
func NewTreeRolesOutOfScopeError(roleIDs []string) *fault.Error {
	return fault.Conflict(
		"RESOURCE_TYPE_IN_USE",
		"Roles would bundle permissions outside their scope with this tree. Remove those permissions from the roles first.",
	).Metadata(map[string]any{metadataBlockedBy: map[string]any{"roles": len(roleIDs)}, "role_ids": roleIDs})
}
