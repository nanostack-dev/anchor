package service

import (
	"context"
	"errors"
)

// SubjectType names who asks for access.
type SubjectType string

const (
	// SubjectUser is a product user named by its external (identity provider) user ID.
	SubjectUser SubjectType = "user"
	// SubjectOrganizationAPIKey is a raw organization API key value.
	SubjectOrganizationAPIKey SubjectType = "organization_api_key"
)

// ResourceType names what access is asked for: organization, or any resource
// type slug the Product declares.
type ResourceType string

const ResourceOrganization ResourceType = "organization"

// builtIn reports whether the type keeps its own rules: an organization is no
// registered resource and takes no parent.
func (t ResourceType) builtIn() bool { return t == ResourceOrganization }

// DenyReason says why access is denied.
type DenyReason string

const (
	ReasonSubjectNotSynced          DenyReason = "SUBJECT_NOT_SYNCED"
	ReasonAPIKeyInvalid             DenyReason = "API_KEY_INVALID"  //nolint:gosec // a reason code, not a credential
	ReasonAPIKeyInactive            DenyReason = "API_KEY_INACTIVE" //nolint:gosec // a reason code, not a credential
	ReasonOrganizationNotFound      DenyReason = "ORGANIZATION_NOT_FOUND"
	ReasonNotAMember                DenyReason = "NOT_A_MEMBER"
	ReasonMissingPermission         DenyReason = "MISSING_PERMISSION"
	ReasonNotEvaluated              DenyReason = "NOT_EVALUATED"
	ReasonResourceTypeNotFound      DenyReason = "RESOURCE_TYPE_NOT_FOUND"
	ReasonResourceNotFound          DenyReason = "RESOURCE_NOT_FOUND"
	ReasonResourceNotInOrganization DenyReason = "RESOURCE_NOT_IN_ORGANIZATION"
	ReasonParentTypeNotAllowed      DenyReason = "PARENT_TYPE_NOT_ALLOWED"
)

// Semantic controls how a batch short-circuits, as AuthZEN's evaluations_semantic does.
type Semantic string

const (
	ExecuteAll          Semantic = "execute_all"
	DenyOnFirstDeny     Semantic = "deny_on_first_deny"
	PermitOnFirstPermit Semantic = "permit_on_first_permit"
)

// stopsAfter reports whether the semantic skips the actions after this one.
func (s Semantic) stopsAfter(item ItemDecision) bool {
	return (s == DenyOnFirstDeny && !item.Allowed) || (s == PermitOnFirstPermit && item.Allowed)
}

// PermissionChecker answers which actions an identity holds on an object:
// its Organization or one of its resources. Decisions come from OpenFGA
// (ADR-0021, ADR-0022 G4 PR 7); Postgres still resolves who the subject is
// and which object the resource is.
type PermissionChecker interface {
	Holds(ctx context.Context, check PermissionCheck) (map[string]bool, error)
}

// ErrPermissionStoreUnavailable wraps a failure of the relationship store
// itself: not ready, failing, or too slow. The decision answers 503 for it.
var ErrPermissionStoreUnavailable = errors.New("permission store unavailable")

// PermissionCheck names the identity, the object and the actions to answer
// for it.
type PermissionCheck struct {
	ProductID string
	// ProductUserID is set for a member, APIKeyID for an organization API key.
	ProductUserID string
	APIKeyID      string
	Object        CheckedObject
	// Fallback, when set, is checked for the actions the model doesn't define
	// on Object. Only a workspace sets it, for one release: see
	// workspaceCompatibility.
	Fallback *CheckedObject
	Actions  []string
}

// CheckedObject is an object of the Product's model: the Organization, by its
// ID, or a resource, by its type's slug and its row ID.
type CheckedObject struct {
	Type string
	ID   string
	// Parent is set, and ID left empty, for a resource the registry lacks:
	// the check is given its parent for that check only (ADR-0022 B1).
	Parent *CheckedObject
}

// OrganizationObject is the Organization as a checked object.
func OrganizationObject(organizationID string) CheckedObject {
	return CheckedObject{Type: string(ResourceOrganization), ID: organizationID}
}

// Identity is who the subject resolved to in the organization.
type Identity struct {
	OrganizationID   string
	OrganizationName string
	RoleID           string
	RoleName         string
	ProductUserID    string
	ProductUserName  string
	ProductUserEmail string
	APIKeyID         string
	APIKeyName       string
	// Permissions lists what the role or key grants, read from Anchor's
	// tables for the decision's context; the decision itself comes from
	// OpenFGA.
	Permissions []string
}

type ItemDecision struct {
	Allowed bool
	Reason  DenyReason
	// GrantedBy is set only for an allowed item of a request that asked for
	// an explanation: what granted it, nearest first. It may be empty when
	// Anchor's tables and the store disagree for a moment.
	GrantedBy []Grant
}

// Grant is one role, or an API key's own permission, that grants an action.
type Grant struct {
	// RoleID and RoleName name a role a member holds; APIKeyID and
	// APIKeyName an Organization API key holding the permission itself.
	RoleID     string
	RoleName   string
	APIKeyID   string
	APIKeyName string
	// Resource is where it is held: the Organization or a registered resource.
	Resource GrantResource
}

// GrantResource names where a grant is held, the way the product does.
type GrantResource struct {
	Type string
	ID   string
}

type Decision struct {
	Allowed            bool
	Reason             DenyReason
	MissingPermissions []string
	// Identity is set when the subject reached the resource, allowed or not.
	Identity *Identity
	Items    []ItemDecision
}
