package service

import (
	"context"
	"errors"

	authz "anchor/internal/authz/service"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/repository"
	coresvc "anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
)

// Evaluator decides whether a subject may act on an organization or a workspace.
//
// Every decision comes from the product's OpenFGA store: organization
// existence, membership, the workspace's organization, roles and permissions.
// Postgres only resolves who is asking (a product user's external ID, an API
// key's credential) and the display names in the answer. The store replays any
// pending write before it answers, so a change applies to the next evaluation.
type Evaluator interface {
	Evaluate(ctx context.Context, request Request) (Decision, error)
}

type evaluator struct {
	productUsers  repository.ProductUserRepository
	organizations repository.OrganizationRepository
	apiKeys       coresvc.OrganizationAPIKeyService
	authorizer    *authz.Authorizer
	logger        zerolog.Logger
}

func NewEvaluator(
	productUsers repository.ProductUserRepository,
	organizations repository.OrganizationRepository,
	apiKeys coresvc.OrganizationAPIKeyService,
	authorizer *authz.Authorizer,
	logger zerolog.Logger,
) Evaluator {
	return &evaluator{
		productUsers:  productUsers,
		organizations: organizations,
		apiKeys:       apiKeys,
		authorizer:    authorizer,
		logger:        logger.With().Str("component", "access_evaluator").Logger(),
	}
}

// Evaluate identifies the subject, asks the store what it holds on the
// resource, and decides the actions with that.
func (e *evaluator) Evaluate(ctx context.Context, request Request) (Decision, error) {
	if err := validate.ValidateStruct(request); err != nil {
		return Decision{}, err
	}
	if err := request.checkResource(); err != nil {
		return Decision{}, err
	}
	subject, err := e.identify(ctx, request)
	if err != nil {
		return Decision{}, err
	}
	if subject.stopped != "" {
		return subject.decide(request.Actions, request.Semantic), nil
	}
	access, err := subject.question.Resolve(ctx)
	if err != nil {
		return Decision{}, e.unexpected(err, request, "relationship store")
	}
	reached, err := e.reach(ctx, request, subject.identity, access)
	if err != nil {
		return Decision{}, err
	}
	return reached.decide(request.Actions, request.Semantic), nil
}

// identified is a subject Anchor recognized, with the question to ask the store
// about it, or the reason it could not be recognized.
type identified struct {
	reach
	question authz.Question
}

func (e *evaluator) identify(ctx context.Context, request Request) (identified, error) {
	if request.Subject.Type == SubjectOrganizationAPIKey {
		return e.identifyAPIKey(ctx, request)
	}
	return e.identifyUser(ctx, request)
}

func (e *evaluator) identifyUser(ctx context.Context, request Request) (identified, error) {
	found, err := e.productUsers.FindByExternalID(ctx, request.ProductID, request.Subject.ID)
	if err != nil {
		return identified{}, e.unexpected(err, request, "product user")
	}
	user, synced := found.Get()
	if !synced {
		return identified{reach: stoppedBy(ReasonSubjectNotSynced)}, nil
	}
	return identified{
		reach: reached(&Identity{
			OrganizationID:  request.Organization(),
			ProductUserID:   user.ID,
			ProductUserName: user.Name,
		}),
		question: on(e.authorizer.Product(request.ProductID).User(user.ID), request, request.Organization()),
	}, nil
}

func (e *evaluator) identifyAPIKey(ctx context.Context, request Request) (identified, error) {
	key, err := e.apiKeys.IntrospectAPIKey(ctx, orgapikey.IntrospectOrganizationAPIKeyInput{
		ProductID:   request.ProductID,
		APIKeyValue: request.Subject.ID,
	})
	switch {
	case errors.Is(err, orgapikey.ErrOrganizationAPIKeyCredentialNotFound), errors.Is(err, coresvc.ErrInvalidAPIKey):
		return identified{reach: stoppedBy(ReasonAPIKeyInvalid)}, nil
	case err != nil:
		return identified{}, err
	case key.Inactive:
		return identified{reach: stoppedBy(ReasonAPIKeyInactive)}, nil
	case request.Organization() != "" && request.Organization() != key.APIKey.OrganizationID:
		return identified{reach: stoppedBy(ReasonNotAMember)}, nil
	}
	organizationID := key.APIKey.OrganizationID
	return identified{
		reach: reached(&Identity{OrganizationID: organizationID, APIKeyID: key.APIKey.ID}),
		question: on(
			e.authorizer.Product(request.ProductID).APIKey(key.APIKey.ID), request, organizationID,
		),
	}, nil
}

func on(subject authz.SubjectQuestion, request Request, organizationID string) authz.Question {
	if request.Resource.Type == ResourceWorkspace {
		return subject.InWorkspace(organizationID, request.Resource.ID)
	}
	return subject.InOrganization(organizationID)
}

// reach turns what the store answered into how far the subject got, filling in
// the names a decision reports.
func (e *evaluator) reach(
	ctx context.Context,
	request Request,
	identity *Identity,
	access authz.Access,
) (reach, error) {
	switch {
	case !access.OrganizationExists:
		return stoppedBy(ReasonOrganizationNotFound), nil
	case !access.IsMember:
		return stoppedBy(ReasonNotAMember), nil
	case !access.WorkspaceInOrganization:
		return stoppedBy(ReasonWorkspaceNotInOrganization), nil
	}
	organization, err := e.organizations.FindByID(ctx, request.ProductID, identity.OrganizationID)
	if err != nil {
		return reach{}, e.unexpected(err, request, "organization")
	}
	if found, ok := organization.Get(); ok {
		identity.OrganizationName = found.Name
	}
	if role, ok := access.OrganizationRole.Get(); ok {
		identity.RoleID, identity.RoleName = role.ID, role.Name
	}
	if role, ok := access.WorkspaceRole.Get(); ok {
		identity.WorkspaceRoleID, identity.WorkspaceRoleName = role.ID, role.Name
	}
	identity.Permissions = access.Permissions
	return reached(identity), nil
}

func (e *evaluator) unexpected(err error, request Request, lookup string) error {
	e.logger.Error().Err(err).Str("product_id", request.ProductID).Str("lookup", lookup).
		Msg("access evaluation lookup failed")
	return fault.ErrUnexpected
}
