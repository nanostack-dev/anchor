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

// stopsAfter reports whether the semantic skips the actions after this one.
func (s Semantic) stopsAfter(item ItemDecision) bool {
	return (s == DenyOnFirstDeny && !item.Allowed) || (s == PermitOnFirstPermit && item.Allowed)
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
