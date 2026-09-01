package events

func (t Type) Known() bool {
	switch t {
	case OrganizationCreated,
		OrganizationUpdated,
		OrganizationDeleted,
		MembershipCreated,
		MembershipUpdated,
		MembershipDeleted,
		WorkspaceCreated,
		WorkspaceUpdated,
		WorkspaceDeleted,
		OrganizationAPIKeyCreated,
		OrganizationAPIKeyUpdated,
		OrganizationAPIKeyDeleted,
		ProductUserCreated,
		ProductUserUpdated,
		ProductUserDeleted,
		OrganizationLicenseUpdated:
		return true
	default:
		return false
	}
}
