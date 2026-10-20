package service

import (
	"context"
	"errors"
	"slices"

	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/repository"
	coresvc "anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
)

// Evaluator decides whether a subject may act on an organization or a workspace.
//
// It reads Postgres on every call and caches nothing, so a membership, role,
// workspace or API key write applies to the next evaluation without eviction.
type Evaluator interface {
	Evaluate(ctx context.Context, input EvaluateInput) (Decision, error)
}

type evaluator struct {
	productUsers  repository.ProductUserRepository
	memberships   repository.OrganizationMembershipRepository
	organizations repository.OrganizationRepository
	workspaces    repository.WorkspaceRepository
	apiKeys       coresvc.OrganizationAPIKeyService
	logger        zerolog.Logger
}

func NewEvaluator(
	productUsers repository.ProductUserRepository,
	memberships repository.OrganizationMembershipRepository,
	organizations repository.OrganizationRepository,
	workspaces repository.WorkspaceRepository,
	apiKeys coresvc.OrganizationAPIKeyService,
	logger zerolog.Logger,
) Evaluator {
	return &evaluator{
		productUsers:  productUsers,
		memberships:   memberships,
		organizations: organizations,
		workspaces:    workspaces,
		apiKeys:       apiKeys,
		logger:        logger.With().Str("component", "access_evaluator").Logger(),
	}
}

func (e *evaluator) Evaluate(ctx context.Context, input EvaluateInput) (Decision, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return Decision{}, err
	}
	if err := validateResource(input.Subject, input.Resource); err != nil {
		return Decision{}, err
	}

	identity, reason, err := e.resolve(ctx, input)
	if err != nil {
		return Decision{}, err
	}
	if reason != "" {
		return denyAll(input.Actions, reason), nil
	}
	return decide(input.Actions, input.Semantic, identity), nil
}

func validateResource(subject Subject, resource Resource) error {
	if resource.Type == ResourceWorkspace && resource.ID == "" {
		return fault.BadRequest("INVALID_ACCESS_RESOURCE", "A workspace resource needs an id.")
	}
	if subject.Type == SubjectUser && resourceOrganization(resource) == "" {
		return fault.BadRequest(
			"INVALID_ACCESS_RESOURCE",
			"A user subject needs the organization: the organization id, or properties.organization_id for a workspace.",
		)
	}
	return nil
}

// resourceOrganization is the organization the request names, or "" when it names none.
func resourceOrganization(resource Resource) string {
	if resource.Type == ResourceOrganization {
		return resource.ID
	}
	return resource.OrganizationID
}

// resolve returns the subject's identity in the requested organization, or the
// reason the subject cannot reach the resource at all.
func (e *evaluator) resolve(ctx context.Context, input EvaluateInput) (*Identity, DenyReason, error) {
	var identity *Identity
	var reason DenyReason
	var err error
	if input.Subject.Type == SubjectUser {
		identity, reason, err = e.resolveUser(
			ctx,
			input.ProductID,
			input.Subject.ID,
			resourceOrganization(input.Resource),
		)
	} else {
		identity, reason, err = e.resolveAPIKey(
			ctx,
			input.ProductID,
			input.Subject.ID,
			resourceOrganization(input.Resource),
		)
	}
	if err != nil || reason != "" {
		return nil, reason, err
	}
	if input.Resource.Type == ResourceWorkspace {
		found, findErr := e.workspaces.FindByID(ctx, input.ProductID, identity.OrganizationID, input.Resource.ID)
		if findErr != nil {
			e.logger.Error().
				Err(findErr).
				Str("product_id", input.ProductID).
				Msg("access evaluation: workspace lookup failed")
			return nil, "", fault.ErrUnexpected
		}
		if found.IsAbsent() {
			return nil, ReasonWorkspaceNotInOrganization, nil
		}
	}
	return identity, "", nil
}

func (e *evaluator) resolveUser(
	ctx context.Context, productID, externalID, organizationID string,
) (*Identity, DenyReason, error) {
	productUser, err := e.productUsers.FindByExternalID(ctx, productID, externalID)
	if err != nil {
		e.logger.Error().Err(err).Str("product_id", productID).Msg("access evaluation: product user lookup failed")
		return nil, "", fault.ErrUnexpected
	}
	user, synced := productUser.Get()
	if !synced {
		return nil, ReasonSubjectNotSynced, nil
	}
	membership, err := e.memberships.FindByProductUserIDAndOrgID(ctx, productID, user.ID, organizationID, true)
	if err != nil {
		e.logger.Error().Err(err).Str("product_id", productID).Msg("access evaluation: membership lookup failed")
		return nil, "", fault.ErrUnexpected
	}
	member, isMember := membership.Get()
	if !isMember {
		return e.absentMembershipReason(ctx, productID, organizationID)
	}
	return &Identity{
		OrganizationID:   member.OrganizationID,
		OrganizationName: member.OrganizationName,
		RoleID:           member.RoleID,
		RoleName:         member.RoleName,
		ProductUserID:    user.ID,
		ProductUserName:  user.Name,
		Permissions:      member.RolePermissions,
	}, "", nil
}

// absentMembershipReason tells a missing organization from a missing membership;
// it costs a query only on that deny path.
func (e *evaluator) absentMembershipReason(
	ctx context.Context,
	productID, organizationID string,
) (*Identity, DenyReason, error) {
	organization, err := e.organizations.FindByID(ctx, productID, organizationID)
	if err != nil {
		e.logger.Error().Err(err).Str("product_id", productID).Msg("access evaluation: organization lookup failed")
		return nil, "", fault.ErrUnexpected
	}
	if organization.IsAbsent() {
		return nil, ReasonOrganizationNotFound, nil
	}
	return nil, ReasonNotAMember, nil
}

func (e *evaluator) resolveAPIKey(
	ctx context.Context, productID, rawKey, organizationID string,
) (*Identity, DenyReason, error) {
	result, err := e.apiKeys.IntrospectAPIKey(ctx, orgapikey.IntrospectOrganizationAPIKeyInput{
		ProductID:   productID,
		APIKeyValue: rawKey,
	})
	if errors.Is(err, orgapikey.ErrOrganizationAPIKeyCredentialNotFound) || errors.Is(err, coresvc.ErrInvalidAPIKey) {
		return nil, ReasonAPIKeyInvalid, nil
	}
	if err != nil {
		return nil, "", err
	}
	if result.Inactive {
		return nil, ReasonAPIKeyInactive, nil
	}
	if organizationID != "" && organizationID != result.APIKey.OrganizationID {
		return nil, ReasonNotAMember, nil
	}
	return &Identity{
		OrganizationID: result.APIKey.OrganizationID,
		APIKeyID:       result.APIKey.ID,
		Permissions:    result.Permissions,
	}, "", nil
}

func denyAll(actions []string, reason DenyReason) Decision {
	items := make([]ItemDecision, len(actions))
	for i := range items {
		items[i] = ItemDecision{Reason: reason}
	}
	return Decision{Reason: reason, Items: items}
}

func decide(actions []string, semantic Semantic, identity *Identity) Decision {
	decision := Decision{Allowed: true, Identity: identity, Items: make([]ItemDecision, len(actions))}
	stopped := false
	for i, action := range actions {
		if stopped {
			decision.Items[i] = ItemDecision{Reason: ReasonNotEvaluated}
			decision.Allowed = false
			continue
		}
		if slices.Contains(identity.Permissions, action) {
			decision.Items[i] = ItemDecision{Allowed: true}
			stopped = semantic == PermitOnFirstPermit
			continue
		}
		decision.Items[i] = ItemDecision{Reason: ReasonMissingPermission}
		decision.Allowed = false
		decision.MissingPermissions = append(decision.MissingPermissions, action)
		stopped = semantic == DenyOnFirstDeny
	}
	if !decision.Allowed {
		decision.Reason = ReasonMissingPermission
		if len(decision.MissingPermissions) == 0 {
			// Only NOT_EVALUATED items denied: permit_on_first_permit skipped the rest.
			decision.Reason = ReasonNotEvaluated
		}
	}
	return decision
}
