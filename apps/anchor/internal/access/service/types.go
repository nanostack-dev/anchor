package service

// SubjectType names who asks for access.
type SubjectType string

const (
	// SubjectUser is a product user named by its external (identity provider) user ID.
	SubjectUser SubjectType = "user"
	// SubjectOrganizationAPIKey is a raw organization API key value.
	SubjectOrganizationAPIKey SubjectType = "organization_api_key"
)

// ResourceType names what access is asked for.
type ResourceType string

const (
	ResourceOrganization ResourceType = "organization"
	ResourceWorkspace    ResourceType = "workspace"
)

// DenyReason says why access is denied.
type DenyReason string

const (
	ReasonSubjectNotSynced           DenyReason = "SUBJECT_NOT_SYNCED"
	ReasonAPIKeyInvalid              DenyReason = "API_KEY_INVALID"  //nolint:gosec // a reason code, not a credential
	ReasonAPIKeyInactive             DenyReason = "API_KEY_INACTIVE" //nolint:gosec // a reason code, not a credential
	ReasonOrganizationNotFound       DenyReason = "ORGANIZATION_NOT_FOUND"
	ReasonNotAMember                 DenyReason = "NOT_A_MEMBER"
	ReasonWorkspaceNotInOrganization DenyReason = "WORKSPACE_NOT_IN_ORGANIZATION"
	ReasonMissingPermission          DenyReason = "MISSING_PERMISSION"
	ReasonNotEvaluated               DenyReason = "NOT_EVALUATED"
)

// Semantic controls how a batch short-circuits, as AuthZEN's evaluations_semantic does.
type Semantic string

const (
	ExecuteAll          Semantic = "execute_all"
	DenyOnFirstDeny     Semantic = "deny_on_first_deny"
	PermitOnFirstPermit Semantic = "permit_on_first_permit"
)

type Subject struct {
	Type SubjectType `validate:"required,oneof=user organization_api_key"`
	// ID is the external user ID or the raw API key. Never log it for a key.
	ID string `validate:"required,notblank"`
}

type Resource struct {
	Type ResourceType `validate:"required,oneof=organization workspace"`
	// ID is the organization or workspace ID. Empty only for an organization
	// resource with an API key subject, meaning the key's organization.
	ID string
	// OrganizationID is the workspace's organization; required for a user on a workspace.
	OrganizationID string
}

type EvaluateInput struct {
	ProductID string   `validate:"required,notblank"`
	Subject   Subject  `validate:"required"`
	Resource  Resource `validate:"required"`
	Actions   []string `validate:"dive,required,notblank"`
	Semantic  Semantic `validate:"omitempty,oneof=execute_all deny_on_first_deny permit_on_first_permit"`
}

// Identity is who the subject resolved to in the organization.
type Identity struct {
	OrganizationID   string
	OrganizationName string
	RoleID           string
	RoleName         string
	ProductUserID    string
	ProductUserName  string
	APIKeyID         string
	Permissions      []string
}

type ItemDecision struct {
	Allowed bool
	Reason  DenyReason
}

type Decision struct {
	Allowed            bool
	Reason             DenyReason
	MissingPermissions []string
	// Identity is set when the subject reached the resource, allowed or not.
	Identity *Identity
	Items    []ItemDecision
}
