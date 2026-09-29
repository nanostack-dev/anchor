package service

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var (
	errInvitationNotFound = fault.NotFound(
		"ORGANIZATION_INVITATION_NOT_FOUND",
		"This organization has no invitation with that identifier.",
	)

	errTokenNotFound = fault.BadRequest(
		"ORGANIZATION_INVITATION_TOKEN_NOT_FOUND",
		"This product has no invitation with that token.",
	)

	errPendingInvitationExists = fault.Conflict(
		"ORGANIZATION_INVITATION_ALREADY_PENDING",
		"A pending invitation already exists for this email address in this organization.",
	)

	errEmailBelongsToMember = fault.Conflict(
		"ORGANIZATION_INVITATION_EMAIL_IS_MEMBER",
		"This email address belongs to a member of this organization already.",
	)

	errInvitationAlreadyAccepted = fault.Conflict(
		"ORGANIZATION_INVITATION_ALREADY_ACCEPTED",
		"This invitation is accepted already.",
	)

	errInvitationExpired = fault.Conflict(
		"ORGANIZATION_INVITATION_EXPIRED",
		"This invitation is expired. Create a new invitation.",
	)

	errExpiryNotInFuture = fault.BadRequest(
		"ORGANIZATION_INVITATION_EXPIRY_NOT_IN_FUTURE",
		"The expiry of an invitation must be in the future.",
	)
)
