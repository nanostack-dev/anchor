// Package audit contains the domain types for the general audit log:
// an append-only, tenant-scoped record of management-plane mutations.
// Design doc: docs/audit-logs.md
package audit

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Log is a single immutable audit log entry.
type Log struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   *string
	Action           Action
	Outcome          Outcome
	ActorType        ActorType
	ActorID          *string
	ActorName        *string
	TargetType       string
	TargetID         *string
	TargetName       *string
	RequestID        *string
	MetadataJSON     json.RawMessage
	CreatedAt        time.Time
}

// GenerateID sets the audit log's ID to a new prefixed KSUID.
func (l *Log) GenerateID() { l.ID = ids.MustNew("alog") }

// MetadataKeyPrevious is the metadata key holding the pre-mutation field
// values on *.updated events (sparse delta, not a full snapshot).
const MetadataKeyPrevious = "previous"

// Metadata marshals values into the MetadataJSON payload of a Log entry.
// Returns nil (no metadata) if marshalling fails — audit writes are
// best-effort and must not fail the calling mutation.
func Metadata(values map[string]any) json.RawMessage {
	payload, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return payload
}

// ActorType identifies what kind of principal performed the action.
type ActorType string

const (
	ActorTypePlatformUser ActorType = "PLATFORM_USER"
	//nolint:gosec // actor type label, not a credential
	ActorTypeProductAPIKey ActorType = "PRODUCT_API_KEY"
	ActorTypeSystem        ActorType = "SYSTEM"
)

// Outcome records whether the audited action succeeded.
type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailure Outcome = "FAILURE"
)

// Action is a dotted resource.verb_past_tense event name, e.g. "organization.created".
// All actions must be declared here; call sites never use free strings.
type Action string

const (
	ActionOrganizationCreated Action = "organization.created"
	ActionOrganizationUpdated Action = "organization.updated"
	ActionOrganizationDeleted Action = "organization.deleted"

	ActionOrganizationMemberAdded       Action = "organization.member_added"
	ActionOrganizationMemberRemoved     Action = "organization.member_removed"
	ActionOrganizationMemberRoleUpdated Action = "organization.member_role_updated"

	ActionWorkspaceCreated Action = "workspace.created"
	ActionWorkspaceUpdated Action = "workspace.updated"
	ActionWorkspaceDeleted Action = "workspace.deleted"

	ActionProductCreated Action = "product.created"
	ActionProductUpdated Action = "product.updated"
	ActionProductDeleted Action = "product.deleted"

	ActionProductUserCreated Action = "product_user.created"
	ActionProductUserDeleted Action = "product_user.deleted"

	ActionPermissionCreated Action = "permission.created"
	ActionPermissionUpdated Action = "permission.updated"
	ActionPermissionDeleted Action = "permission.deleted"

	ActionResourcePermissionCreated Action = "resource_permission.created"
	ActionResourcePermissionUpdated Action = "resource_permission.updated"
	ActionResourcePermissionDeleted Action = "resource_permission.deleted"

	ActionOrganizationAPIKeyCreated Action = "organization_api_key.created"
	ActionOrganizationAPIKeyUpdated Action = "organization_api_key.updated"
	ActionOrganizationAPIKeyDeleted Action = "organization_api_key.deleted"

	ActionProductAPIKeyCreated Action = "product_api_key.created"
	ActionProductAPIKeyUpdated Action = "product_api_key.updated"
	ActionProductAPIKeyDeleted Action = "product_api_key.deleted"

	ActionRoleCreated              Action = "role.created"
	ActionRoleUpdated              Action = "role.updated"
	ActionRoleDeleted              Action = "role.deleted"
	ActionRolePermissionAssigned   Action = "role.permission_assigned"
	ActionRolePermissionUnassigned Action = "role.permission_unassigned"
)

// Target types referenced by audit entries.
const (
	TargetTypeOrganization       = "organization"
	TargetTypeWorkspace          = "workspace"
	TargetTypeMembership         = "membership"
	TargetTypeOrganizationAPIKey = "organization_api_key"
	TargetTypeProductAPIKey      = "product_api_key"
	TargetTypeRole               = "role"
	TargetTypeProduct            = "product"
	TargetTypeProductUser        = "product_user"
	TargetTypePermission         = "permission"
	TargetTypeResourcePermission = "resource_permission"
)
