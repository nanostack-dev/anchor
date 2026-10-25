package service

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/repository"
	coresvc "anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
)

// driftLogBurst caps the drift lines logged a minute.
const driftLogBurst = 20

// Evaluator decides whether a subject may act on an organization or a workspace.
//
// Postgres resolves who the subject is: the product user and its membership,
// or the API key through introspection, and the workspace's organization.
// Which actions it holds comes from OpenFGA, which a write that changes access
// usually reaches before its response, and otherwise within seconds, when the
// sweeper replays its mark (ADR-0022 E5). Nothing is cached.
type Evaluator interface {
	Evaluate(ctx context.Context, request Request) (Decision, error)
}

type evaluator struct {
	productUsers  repository.ProductUserRepository
	memberships   repository.OrganizationMembershipRepository
	organizations repository.OrganizationRepository
	workspaces    repository.WorkspaceRepository
	apiKeys       coresvc.OrganizationAPIKeyService
	permissions   PermissionChecker
	logger        zerolog.Logger
	driftSampler  zerolog.Sampler
}

func NewEvaluator(
	productUsers repository.ProductUserRepository,
	memberships repository.OrganizationMembershipRepository,
	organizations repository.OrganizationRepository,
	workspaces repository.WorkspaceRepository,
	apiKeys coresvc.OrganizationAPIKeyService,
	permissions PermissionChecker,
	logger zerolog.Logger,
) Evaluator {
	return &evaluator{
		productUsers:  productUsers,
		memberships:   memberships,
		organizations: organizations,
		workspaces:    workspaces,
		apiKeys:       apiKeys,
		permissions:   permissions,
		logger:        logger.With().Str("component", "access_evaluator").Logger(),
		driftSampler:  &zerolog.BurstSampler{Burst: driftLogBurst, Period: time.Minute},
	}
}

// Evaluate walks the subject toward the resource, organization first, then the
// workspace, and decides the actions with whatever identity it reached.
func (e *evaluator) Evaluate(ctx context.Context, request Request) (Decision, error) {
	if err := validate.ValidateStruct(request); err != nil {
		return Decision{}, err
	}
	if err := request.checkResource(); err != nil {
		return Decision{}, err
	}
	subject, err := e.reachOrganization(ctx, request)
	if err != nil {
		return Decision{}, err
	}
	if subject, err = e.reachWorkspace(ctx, request, subject); err != nil {
		return Decision{}, err
	}
	if subject.stopped != "" {
		return subject.decide(request.Actions, request.Semantic, nil), nil
	}
	identity := subject.identity
	held, err := e.permissions.Holds(ctx, PermissionCheck{
		ProductID:      request.ProductID,
		OrganizationID: identity.OrganizationID,
		ProductUserID:  identity.ProductUserID,
		APIKeyID:       identity.APIKeyID,
		Actions:        request.Actions,
	})
	switch {
	case errors.Is(err, context.Canceled):
		return Decision{}, err
	case errors.Is(err, ErrPermissionStoreUnavailable):
		log.Ctx(ctx).Warn().Err(err).Str("product_id", request.ProductID).
			Str("organization_id", identity.OrganizationID).Msg("access decisions unavailable")
		return Decision{}, ErrDecisionsUnavailable
	case err != nil:
		return Decision{}, e.unexpected(err, request, "permissions")
	}
	e.reportDrift(ctx, request, identity, held)
	return subject.decide(request.Actions, request.Semantic, func(action string) bool { return held[action] }), nil
}

// reportDrift logs, sampled, each action where OpenFGA and the permissions
// Anchor's tables list for the identity disagree. A write's own replay window
// can show one; a lasting one is a sync defect.
func (e *evaluator) reportDrift(ctx context.Context, request Request, identity *Identity, held map[string]bool) {
	logger := log.Ctx(ctx).Sample(e.driftSampler)
	for _, action := range request.Actions {
		listed := slices.Contains(identity.Permissions, action)
		if listed == held[action] {
			continue
		}
		logger.Warn().
			Str("event", "authz.decision_drift").
			Str("product_id", request.ProductID).
			Str("organization_id", identity.OrganizationID).
			Str("product_user_id", identity.ProductUserID).
			Str("api_key_id", identity.APIKeyID).
			Str("permission", action).
			Bool("tables", listed).
			Bool("openfga", held[action]).
			Msg("authz.decision_drift")
	}
}

// ErrDecisionsUnavailable answers a decision the relationship store can't
// make right now; the caller retries.
var ErrDecisionsUnavailable = fault.NewWithStatus(
	"ACCESS_DECISIONS_UNAVAILABLE",
	"Access decisions are unavailable for a moment. Try again.",
	http.StatusServiceUnavailable,
)

func (e *evaluator) reachOrganization(ctx context.Context, request Request) (reach, error) {
	if request.Subject.Type == SubjectOrganizationAPIKey {
		return e.reachAsAPIKey(ctx, request)
	}
	return e.reachAsUser(ctx, request)
}

func (e *evaluator) reachAsUser(ctx context.Context, request Request) (reach, error) {
	found, err := e.productUsers.FindByExternalID(ctx, request.ProductID, request.Subject.ID)
	if err != nil {
		return reach{}, e.unexpected(err, request, "product user")
	}
	user, synced := found.Get()
	if !synced {
		return stoppedBy(ReasonSubjectNotSynced), nil
	}
	membership, err := e.memberships.FindByProductUserIDAndOrgID(
		ctx, request.ProductID, user.ID, request.Organization(), true)
	if err != nil {
		return reach{}, e.unexpected(err, request, "membership")
	}
	member, isMember := membership.Get()
	if !isMember {
		return e.whyNotAMember(ctx, request)
	}
	return reached(&Identity{
		OrganizationID:   member.OrganizationID,
		OrganizationName: member.OrganizationName,
		RoleID:           member.RoleID,
		RoleName:         member.RoleName,
		ProductUserID:    user.ID,
		ProductUserName:  user.Name,
		ProductUserEmail: user.Email,
		Permissions:      member.RolePermissions,
	}), nil
}

// whyNotAMember tells a missing organization from a missing membership; it
// costs a query only on that deny path.
func (e *evaluator) whyNotAMember(ctx context.Context, request Request) (reach, error) {
	organization, err := e.organizations.FindByID(ctx, request.ProductID, request.Organization())
	if err != nil {
		return reach{}, e.unexpected(err, request, "organization")
	}
	if organization.IsAbsent() {
		return stoppedBy(ReasonOrganizationNotFound), nil
	}
	return stoppedBy(ReasonNotAMember), nil
}

func (e *evaluator) reachAsAPIKey(ctx context.Context, request Request) (reach, error) {
	key, err := e.apiKeys.IntrospectAPIKey(ctx, orgapikey.IntrospectOrganizationAPIKeyInput{
		ProductID:   request.ProductID,
		APIKeyValue: request.Subject.ID,
	})
	switch {
	case errors.Is(err, orgapikey.ErrOrganizationAPIKeyCredentialNotFound), errors.Is(err, coresvc.ErrInvalidAPIKey):
		return stoppedBy(ReasonAPIKeyInvalid), nil
	case err != nil:
		return reach{}, err
	case key.Inactive:
		return stoppedBy(ReasonAPIKeyInactive), nil
	case request.Organization() != "" && request.Organization() != key.APIKey.OrganizationID:
		return stoppedBy(ReasonNotAMember), nil
	}
	return reached(&Identity{
		OrganizationID: key.APIKey.OrganizationID,
		APIKeyID:       key.APIKey.ID,
		APIKeyName:     key.APIKey.Name,
		Permissions:    key.Permissions,
	}), nil
}

// reachWorkspace checks that a requested workspace belongs to the organization
// the subject reached. Unknown and foreign workspaces stop the same way.
func (e *evaluator) reachWorkspace(ctx context.Context, request Request, subject reach) (reach, error) {
	if request.Resource.Type != ResourceWorkspace || subject.stopped != "" {
		return subject, nil
	}
	found, err := e.workspaces.FindByID(ctx, request.ProductID, subject.identity.OrganizationID, request.Resource.ID)
	if err != nil {
		return reach{}, e.unexpected(err, request, "workspace")
	}
	if found.IsAbsent() {
		return stoppedBy(ReasonWorkspaceNotInOrganization), nil
	}
	return subject, nil
}

func (e *evaluator) unexpected(err error, request Request, lookup string) error {
	e.logger.Error().Err(err).Str("product_id", request.ProductID).Str("lookup", lookup).
		Msg("access evaluation lookup failed")
	return fault.ErrUnexpected
}
