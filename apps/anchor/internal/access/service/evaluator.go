package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"anchor/internal/applog"

	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
	assignmentrepository "anchor/internal/roleassignment/repository"
	coresvc "anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

// logBurst caps the lines each sampled warning logs a minute.
const logBurst = 20

// Evaluator decides whether a subject may act on an organization or a
// resource of the Product's own types (ADR-0022 E2).
//
// Postgres resolves who the subject is: the product user and its membership,
// or the API key through introspection; then what the resource is: the
// registered resource, or the parent an unregistered one names. Which actions it holds comes from OpenFGA, which a
// write that changes access usually reaches before its response, and
// otherwise within seconds, when the sweeper replays its mark (ADR-0022 E5).
// Nothing is cached.
type Evaluator interface {
	Evaluate(ctx context.Context, request Request) (Decision, error)
}

type evaluator struct {
	productUsers  repository.ProductUserRepository
	memberships   repository.OrganizationMembershipRepository
	organizations repository.OrganizationRepository
	apiKeys       coresvc.OrganizationAPIKeyService
	permissions   PermissionChecker
	resourceTypes rtservice.Service
	resources     resourcerepository.Repository
	assignments   assignmentrepository.Repository
	logger        *slog.Logger
	// scoped writes the sampled drift, mismatch and explain lines with the
	// request scope of their context.
	scoped        *slog.Logger
	driftSampler  *applog.BurstSampler
	parentSampler *applog.BurstSampler
}

func NewEvaluator(
	productUsers repository.ProductUserRepository,
	memberships repository.OrganizationMembershipRepository,
	organizations repository.OrganizationRepository,
	apiKeys coresvc.OrganizationAPIKeyService,
	permissions PermissionChecker,
	resourceTypes rtservice.Service,
	resources resourcerepository.Repository,
	assignments assignmentrepository.Repository,
	logger *slog.Logger,
	scoped applog.Scoped,
) Evaluator {
	return &evaluator{
		productUsers:  productUsers,
		memberships:   memberships,
		organizations: organizations,
		apiKeys:       apiKeys,
		permissions:   permissions,
		resourceTypes: resourceTypes,
		resources:     resources,
		assignments:   assignments,
		logger:        logger.With(slog.String("component", "access_evaluator")),
		scoped:        scoped.Logger,
		driftSampler:  &applog.BurstSampler{Burst: logBurst, Period: time.Minute},
		parentSampler: &applog.BurstSampler{Burst: logBurst, Period: time.Minute},
	}
}

// Evaluate walks the subject toward the resource, organization first, then the
// resource, and decides the actions with whatever identity it reached.
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
	target := located{}
	if subject.stopped == "" {
		if subject, target, err = e.locate(ctx, request, subject); err != nil {
			return Decision{}, err
		}
	}
	if subject.stopped != "" {
		return subject.decide(request.Actions, request.Semantic, nil), nil
	}
	identity := subject.identity
	held, err := e.permissions.Holds(ctx, PermissionCheck{
		ProductID:     request.ProductID,
		ProductUserID: identity.ProductUserID,
		APIKeyID:      identity.APIKeyID,
		Object:        target.object,
		Actions:       request.Actions,
	})
	switch {
	case errors.Is(err, context.Canceled):
		return Decision{}, err
	case errors.Is(err, ErrPermissionStoreUnavailable):
		e.scoped.WarnContext(ctx, "access decisions unavailable",
			log.Err(err),
			slog.String("product_id", request.ProductID),
			slog.String("organization_id", identity.OrganizationID),
		)
		return Decision{}, ErrDecisionsUnavailable
	case err != nil:
		return Decision{}, e.unexpected(ctx, err, request, "permissions")
	}
	if target.object.Type == string(ResourceOrganization) {
		e.reportDrift(ctx, request, identity, held)
	}
	decision := subject.decide(request.Actions, request.Semantic, func(action string) bool { return held[action] })
	if request.Explain {
		e.explainOrSkip(ctx, request, identity, target, &decision)
	}
	return decision, nil
}

// reportDrift logs, sampled, each action where OpenFGA and the permissions
// Anchor's tables list for the identity disagree. A write's own replay window
// can show one; a lasting one is a sync defect. Only a decision checked on the
// Organization compares: roles held on a resource grant more than the
// Organization's role lists.
func (e *evaluator) reportDrift(ctx context.Context, request Request, identity *Identity, held map[string]bool) {
	for _, action := range request.Actions {
		listed := slices.Contains(identity.Permissions, action)
		if listed == held[action] || !e.driftSampler.Sample() {
			continue
		}
		e.scoped.WarnContext(ctx, "authz.decision_drift",
			slog.String("event", "authz.decision_drift"),
			slog.String("product_id", request.ProductID),
			slog.String("organization_id", identity.OrganizationID),
			slog.String("product_user_id", identity.ProductUserID),
			slog.String("api_key_id", identity.APIKeyID),
			slog.String("permission", action),
			slog.Bool("tables", listed),
			slog.Bool("openfga", held[action]),
		)
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
		return reach{}, e.unexpected(ctx, err, request, "product user")
	}
	user, synced := found.Get()
	if !synced {
		return stoppedBy(ReasonSubjectNotSynced), nil
	}
	membership, err := e.memberships.FindByProductUserIDAndOrgID(
		ctx, request.ProductID, user.ID, request.Organization(), true)
	if err != nil {
		return reach{}, e.unexpected(ctx, err, request, "membership")
	}
	member, isMember := membership.Get()
	if !isMember {
		return e.whyNotAMember(ctx, request)
	}
	identity := &Identity{
		OrganizationID:   member.OrganizationID,
		OrganizationName: member.OrganizationName,
		ProductUserID:    user.ID,
		ProductUserName:  user.Name,
		ProductUserEmail: user.Email,
		Permissions:      member.RolePermissions,
	}
	// A member with no role reaches the organization and holds no permission.
	if member.Role != nil {
		identity.RoleID, identity.RoleName = member.Role.ID, member.Role.Name
	}
	return reached(identity), nil
}

// whyNotAMember tells a missing organization from a missing membership; it
// costs a query only on that deny path.
func (e *evaluator) whyNotAMember(ctx context.Context, request Request) (reach, error) {
	organization, err := e.organizations.FindByProductAndID(ctx, request.ProductID, request.Organization())
	if err != nil {
		return reach{}, e.unexpected(ctx, err, request, "organization")
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

func (e *evaluator) unexpected(ctx context.Context, err error, request Request, lookup string) error {
	log.ErrorTo(ctx, e.logger, err, "access evaluation lookup failed",
		slog.String("product_id", request.ProductID),
		slog.String("lookup", lookup),
	)
	return fault.ErrUnexpected
}
