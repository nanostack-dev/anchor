package resource

import (
	"fmt"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

var (
	ErrNotFound = fault.NotFound(
		"RESOURCE_NOT_FOUND",
		"This organization has no registered resource of that type and identifier.",
	)

	// ErrTypeNotRegistrable answers a path naming organization: the root of
	// every chain is managed by the organization routes, so no resource of
	// that type exists.
	ErrTypeNotRegistrable = fault.NotFound(
		"RESOURCE_TYPE_NOT_REGISTRABLE",
		"Organizations are managed through the organization routes and are never registered as resources.",
	)
)

func NewTypeNotFoundError(slug string) *fault.Error {
	return fault.NotFound(
		"RESOURCE_TYPE_NOT_FOUND",
		"This product has no resource type with that slug.",
	).Metadata(map[string]any{"resource_type": slug})
}

func NewInvalidIDError(id string) *fault.Error {
	return fault.BadRequest(
		"INVALID_RESOURCE_ID",
		fmt.Sprintf(
			"A resource ID holds 1 to %d letters, digits, '.', '_', '~' or '-', and is not '.' or '..'.",
			MaxIDLength,
		),
	).Metadata(map[string]any{"id": id})
}

func NewInvalidParentError(resourceType, parentType string, allowed []string) *fault.Error {
	return fault.BadRequest(
		"INVALID_RESOURCE_PARENT",
		fmt.Sprintf("A resource of type %s can't sit under one of type %s. Its parent is of type %s.",
			resourceType, parentType, strings.Join(allowed, " or ")),
	).Metadata(map[string]any{
		"resource_type": resourceType, "parent_type": parentType, "allowed_parent_types": allowed,
	})
}

func NewParentNotRegisteredError(parent Ref) *fault.Error {
	return fault.Conflict(
		"PARENT_NOT_REGISTERED",
		"Register the parent before its children.",
	).Metadata(map[string]any{"parent": map[string]string{"type": parent.Type, "id": parent.ID}})
}

// NewCycleError refuses a move under the resource itself or one of its
// descendants. chain runs from the new parent up to the resource.
func NewCycleError(chain []Ref) *fault.Error {
	refs := make([]string, len(chain))
	for i, ref := range chain {
		refs[i] = ref.String()
	}
	return fault.Conflict(
		"RESOURCE_CYCLE",
		"The new parent is inside this resource, so the resource can't move under it.",
	).Metadata(map[string]any{"chain": refs})
}

func NewTooDeepError(depth int) *fault.Error {
	return fault.Conflict(
		"RESOURCE_TOO_DEEP",
		fmt.Sprintf(
			"A resource chain may be at most %d levels deep, counting the organization. This one would be %d.",
			MaxDepth, depth,
		),
	).Metadata(map[string]any{"max_depth": MaxDepth, "depth": depth})
}

// NewWorkspaceMirrorError refuses deleting a workspace through the resource
// routes while its workspace exists: the workspace routes delete both.
func NewWorkspaceMirrorError(workspaceID string) *fault.Error {
	return fault.Conflict(
		"RESOURCE_IS_WORKSPACE",
		fmt.Sprintf("Workspace %s is deleted with DELETE /workspaces/%s while the workspace routes exist.",
			workspaceID, workspaceID),
	).Metadata(map[string]any{"workspace_id": workspaceID})
}

// NewInUseError refuses to delete a resource that still has registered
// children or role assignments without cascade.
func NewInUseError(children, roleAssignments int) *fault.Error {
	return fault.Conflict(
		"RESOURCE_IN_USE",
		fmt.Sprintf(
			"This resource still has %s and %s. Send cascade=true to delete them too.",
			plural(children, "registered child", "registered children"),
			plural(roleAssignments, "role assignment", "role assignments"),
		),
	).Metadata(map[string]any{"children": children, "role_assignments": roleAssignments})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
