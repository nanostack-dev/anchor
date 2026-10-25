// Package service resolves, lists and updates the roles each Organization
// owns. Copies of role templates are made when first needed: when an
// Organization is created, when a template is resolved inside one, before its
// roles are listed, and by the startup backfill.
package service

import (
	"context"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	"anchor/internal/organizationrole/repository"
	corerepository "anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"
	"anchor/internal/searchx"
)

// copyBatch bounds one InsertCopies call, so a backfill over many
// Organizations writes in steps.
const copyBatch = 500

type Service interface {
	// Validate checks, without writing, that a request body's role_id names a
	// role of the Organization or a role template of its Product.
	Validate(ctx context.Context, productID, organizationID, roleID string) error
	// ResolveTemplate finds the role template a new Organization's founding
	// member gets: the one named, or else the Product's default template.
	ResolveTemplate(ctx context.Context, productID, roleID string) (organizationrole.Role, error)
	// Resolve finds the role a request body named inside an Organization,
	// copying the template into it first when the copy is missing.
	Resolve(txCtx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error)
	// ResolveDefault finds the Organization's default role: its own choice,
	// or else its copy of the Product's default template.
	ResolveDefault(txCtx context.Context, productID, organizationID string) (organizationrole.Role, error)
	// EnsureCopies copies every template the Organization lacks and moves its
	// memberships that still hold a template onto the copy.
	EnsureCopies(txCtx context.Context, productID, organizationID string) error
	// SyncTemplate brings every copy of a template in step with it.
	SyncTemplate(txCtx context.Context, productID, templateID string) error

	// Read reads an Organization's role with its scope and effective default.
	Read(txCtx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error)
	// ApplyDefault applies the update's is_default inside the caller's
	// transaction.
	ApplyDefault(txCtx context.Context, input organizationrole.UpdateInput) error

	Get(ctx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error)
	Search(ctx context.Context, input organizationrole.SearchInput) (search.Result[organizationrole.Role], error)
	Update(ctx context.Context, input organizationrole.UpdateInput) (organizationrole.Role, error)
}

type service struct {
	repo          repository.Repository
	organizations corerepository.OrganizationRepository
	resourceTypes rtservice.Service
	transactor    transactor.Transactor
	events        events.Emitter
	logger        zerolog.Logger
}

func NewService(
	repo repository.Repository,
	organizations corerepository.OrganizationRepository,
	resourceTypes rtservice.Service,
	tx transactor.Transactor,
	emitter events.Emitter,
	logger zerolog.Logger,
) Service {
	return &service{
		repo:          repo,
		organizations: organizations,
		resourceTypes: resourceTypes,
		transactor:    tx,
		events:        emitter,
		logger:        logger.With().Str("component", "organization_role_service").Logger(),
	}
}

func (s *service) Validate(ctx context.Context, productID, organizationID, roleID string) error {
	found, err := s.repo.Resolve(ctx, productID, organizationID, roleID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organizationrole.NewBodyRoleNotFoundError(roleID)
	}
	return nil
}

func (s *service) ResolveTemplate(ctx context.Context, productID, roleID string) (organizationrole.Role, error) {
	if roleID == "" {
		templateID, err := s.repo.FindDefaultTemplateID(ctx, productID)
		if err != nil {
			return organizationrole.Role{}, err
		}
		if templateID.IsAbsent() {
			return organizationrole.Role{}, organizationrole.ErrRoleRequired
		}
		roleID = templateID.Value()
	}
	found, err := s.repo.Resolve(ctx, productID, "", roleID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	return found.ToResult(organizationrole.NewBodyRoleNotFoundError(roleID)).Value()
}

func (s *service) Resolve(
	txCtx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	if roleID == "" {
		return organizationrole.Role{}, organizationrole.NewBodyRoleNotFoundError(roleID)
	}
	found, err := s.repo.Find(txCtx, productID, organizationID, roleID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	if found.IsAbsent() {
		if err = s.copyTemplate(txCtx, productID, organizationID, roleID); err != nil {
			return organizationrole.Role{}, err
		}
		if found, err = s.repo.Find(txCtx, productID, organizationID, roleID); err != nil {
			return organizationrole.Role{}, err
		}
	}
	role, err := found.ToResult(organizationrole.NewBodyRoleNotFoundError(roleID)).Value()
	if err != nil || role.TemplateID == nil {
		return role, err
	}
	// The write that follows refers to the copy; holding its template keeps a
	// delete of the template from removing the copy under it.
	held, err := s.repo.ShareTemplate(txCtx, productID, *role.TemplateID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	if !held {
		return organizationrole.Role{}, organizationrole.NewBodyRoleNotFoundError(roleID)
	}
	return role, nil
}

func (s *service) ResolveDefault(
	txCtx context.Context, productID, organizationID string,
) (organizationrole.Role, error) {
	chosen, err := s.repo.FindChosenDefault(txCtx, productID, organizationID)
	if err != nil || chosen.IsPresent() {
		return chosen.OrElse(organizationrole.Role{}), err
	}
	templateID, err := s.repo.FindDefaultTemplateID(txCtx, productID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	if templateID.IsAbsent() {
		return organizationrole.Role{}, organizationrole.ErrRoleRequired
	}
	return s.Resolve(txCtx, productID, organizationID, templateID.Value())
}

func (s *service) EnsureCopies(txCtx context.Context, productID, organizationID string) error {
	for {
		pairs, err := s.repo.MissingCopies(txCtx, productID, organizationID, "", copyBatch)
		if err != nil {
			return err
		}
		if len(pairs) == 0 {
			break
		}
		inserted, err := s.repo.InsertCopies(txCtx, productID, pairs)
		if err != nil {
			return err
		}
		if inserted == 0 {
			break
		}
	}
	_, err := s.repo.RepointMemberships(txCtx, productID, organizationID)
	return err
}

func (s *service) copyTemplate(txCtx context.Context, productID, organizationID, templateID string) error {
	pairs, err := s.repo.MissingCopies(txCtx, productID, organizationID, templateID, 1)
	if err != nil {
		return err
	}
	_, err = s.repo.InsertCopies(txCtx, productID, pairs)
	return err
}

func (s *service) SyncTemplate(txCtx context.Context, productID, templateID string) error {
	return s.repo.SyncCopies(txCtx, productID, templateID)
}

func (s *service) Get(ctx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error) {
	if productID == "" || organizationID == "" || roleID == "" {
		return organizationrole.Role{}, organizationrole.ErrNotFound
	}
	var role organizationrole.Role
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.requireOrganization(txCtx, productID, organizationID); err != nil {
			return err
		}
		if err := s.EnsureCopies(txCtx, productID, organizationID); err != nil {
			return err
		}
		found, err := s.repo.Find(txCtx, productID, organizationID, roleID)
		if err != nil {
			return err
		}
		role, err = found.ToResult(organizationrole.ErrNotFound).Value()
		if err != nil {
			return err
		}
		role, err = s.markDefault(txCtx, productID, organizationID, role)
		return err
	})
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, productID, "get")
	}
	return s.withScope(ctx, productID, role)
}

func (s *service) Search(
	ctx context.Context, input organizationrole.SearchInput,
) (search.Result[organizationrole.Role], error) {
	if err := input.Validate(); err != nil {
		return search.Result[organizationrole.Role]{}, err
	}
	tree, err := s.resourceTypes.Tree(ctx, input.ProductID)
	if err != nil {
		return search.Result[organizationrole.Role]{}, s.unexpected(err, input.ProductID, "search")
	}
	request := input.Request
	if request.Filter != nil {
		resolved := request.Filter.WithScopes(tree)
		request.Filter = &resolved
	}
	var result search.Result[organizationrole.Role]
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if txErr := s.requireOrganization(txCtx, input.ProductID, input.OrganizationID); txErr != nil {
			return txErr
		}
		if txErr := s.EnsureCopies(txCtx, input.ProductID, input.OrganizationID); txErr != nil {
			return txErr
		}
		var searchErr error
		result, searchErr = s.repo.Search(txCtx, input.ProductID, input.OrganizationID, request)
		if searchErr != nil {
			return searchErr
		}
		defaultRowID, defaultErr := s.effectiveDefault(txCtx, input.ProductID, input.OrganizationID)
		result.Items = functional.Slice(result.Items).Map(func(role organizationrole.Role) organizationrole.Role {
			role.IsDefault = role.RowID == defaultRowID
			return role
		})
		return defaultErr
	})
	if err != nil {
		return search.Result[organizationrole.Role]{}, s.unexpected(err, input.ProductID, "search")
	}
	result.Items = functional.Slice(result.Items).Map(withScope(tree))
	return result, nil
}

func (s *service) Update(
	ctx context.Context, input organizationrole.UpdateInput,
) (organizationrole.Role, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationrole.Role{}, err
	}
	var updated organizationrole.Role
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.requireOrganization(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		if err := s.EnsureCopies(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		if err := s.ApplyDefault(txCtx, input); err != nil {
			return err
		}
		var err error
		updated, err = s.Read(txCtx, input.ProductID, input.OrganizationID, input.RoleID)
		return err
	})
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, input.ProductID, "update")
	}
	return updated, nil
}

// ApplyDefault applies the update's is_default inside the caller's
// transaction. It compares with the effective default: true on the role
// that already is the default, or false on the copy of the Product's default
// template that the Organization never chose, changes nothing.
func (s *service) ApplyDefault(txCtx context.Context, input organizationrole.UpdateInput) error {
	if input.IsDefault == nil {
		return nil
	}
	found, err := s.repo.Find(txCtx, input.ProductID, input.OrganizationID, input.RoleID)
	if err != nil {
		return err
	}
	current, err := found.ToResult(organizationrole.ErrNotFound).Value()
	if err != nil {
		return err
	}
	effective, err := s.effectiveDefault(txCtx, input.ProductID, input.OrganizationID)
	if err != nil {
		return err
	}
	isEffective := effective == current.RowID
	if *input.IsDefault == isEffective || !*input.IsDefault && !current.IsDefault {
		return nil
	}
	if *input.IsDefault {
		if err = s.requireOrganizationScope(txCtx, input.ProductID, current); err != nil {
			return err
		}
		if err = s.repo.ClearDefault(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
	}
	err = s.repo.SetDefault(txCtx, input.ProductID, input.OrganizationID, current.RowID, *input.IsDefault)
	if errors.Is(err, repository.ErrDefaultExists) {
		return organizationrole.ErrDefaultConflict
	}
	if err != nil {
		return err
	}
	next, err := s.effectiveDefault(txCtx, input.ProductID, input.OrganizationID)
	if err != nil {
		return err
	}
	return s.emitDefaultChanges(txCtx, input.ProductID, input.OrganizationID, effective, next)
}

// Read reads an Organization's role with its scope and effective default.
func (s *service) Read(
	txCtx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	found, err := s.repo.Find(txCtx, productID, organizationID, roleID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	role, err := found.ToResult(organizationrole.ErrNotFound).Value()
	if err != nil {
		return organizationrole.Role{}, err
	}
	if role, err = s.markDefault(txCtx, productID, organizationID, role); err != nil {
		return organizationrole.Role{}, err
	}
	return s.withScope(txCtx, productID, role)
}

// emitDefaultChanges reports the role that stopped being the effective
// default and the one that became it.
func (s *service) emitDefaultChanges(txCtx context.Context, productID, organizationID, previous, next string) error {
	for _, rowID := range functional.Distinct([]string{previous, next}) {
		if rowID == "" {
			continue
		}
		found, err := s.repo.Find(txCtx, productID, organizationID, rowID)
		if err != nil {
			return err
		}
		if found.IsAbsent() {
			continue
		}
		if err = s.events.Emit(txCtx, events.Event{
			Type:      events.OrganizationRoleUpdated,
			ProductID: productID,
			Data: events.Data{
				events.FieldOrganizationID: organizationID,
				events.FieldRoleID:         found.Value().PublicID(),
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) requireOrganizationScope(txCtx context.Context, productID string, role organizationrole.Role) error {
	tree, err := s.resourceTypes.Tree(txCtx, productID)
	if err != nil {
		return err
	}
	if scopeSlug(tree, role.ResourceTypeID) != resourcetype.OrganizationSlug {
		return organizationrole.ErrDefaultNotOrganizationScoped
	}
	return nil
}

// effectiveDefault names the row of the Organization's default role: its own
// choice, or else its copy of the Product's default template. It is empty
// when there is neither.
func (s *service) effectiveDefault(txCtx context.Context, productID, organizationID string) (string, error) {
	chosen, err := s.repo.FindChosenDefault(txCtx, productID, organizationID)
	if err != nil || chosen.IsPresent() {
		return chosen.Map(func(r organizationrole.Role) string { return r.RowID }).OrElse(""), err
	}
	templateID, err := s.repo.FindDefaultTemplateID(txCtx, productID)
	if err != nil || templateID.IsAbsent() {
		return "", err
	}
	copied, err := s.repo.Find(txCtx, productID, organizationID, templateID.Value())
	if err != nil {
		return "", err
	}
	return copied.Map(func(r organizationrole.Role) string { return r.RowID }).OrElse(""), nil
}

func (s *service) markDefault(
	txCtx context.Context, productID, organizationID string, role organizationrole.Role,
) (organizationrole.Role, error) {
	defaultRowID, err := s.effectiveDefault(txCtx, productID, organizationID)
	role.IsDefault = role.RowID == defaultRowID
	return role, err
}

func (s *service) requireOrganization(ctx context.Context, productID, organizationID string) error {
	found, err := s.organizations.FindByID(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (s *service) withScope(
	ctx context.Context,
	productID string,
	role organizationrole.Role,
) (organizationrole.Role, error) {
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, productID, "read scope")
	}
	return withScope(tree)(role), nil
}

func withScope(tree resourcetype.Tree) func(organizationrole.Role) organizationrole.Role {
	return func(role organizationrole.Role) organizationrole.Role {
		role.ResourceType = scopeSlug(tree, role.ResourceTypeID)
		return role
	}
}

// scopeSlug names a role's scope; a null scope counts as organization, as
// the resource type backfill writes.
func scopeSlug(tree resourcetype.Tree, id *string) string {
	if id == nil {
		return resourcetype.OrganizationSlug
	}
	return tree.ByID(*id).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
		OrElse(resourcetype.OrganizationSlug)
}

func (s *service) unexpected(err error, productID, operation string) error {
	if err == nil || searchx.IsClientError(err) {
		return err
	}
	if _, isFault := fault.As(err); isFault {
		return err
	}
	s.logger.Error().Err(err).Str("product_id", productID).Str("operation", operation).
		Msg("organization role operation failed")
	return fault.ErrUnexpected
}
