package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// Postgres-named constraints on organization_licenses. The unique index is the
// real guard against one Organization holding two licenses. The composite
// foreign key refuses an Organization that does not exist and one belonging to
// another Product with the same failure, which is correct — from this Product's
// side the two are the same thing.
const (
	organizationLicenseUniqueConstraint       = "organization_licenses_organization_id_key"
	organizationLicenseOrganizationConstraint = "fk_organization_licenses_organization_product"
)

var (
	ErrOrganizationLicenseNotFound = fault.NotFound(
		"ORGANIZATION_LICENSE_NOT_FOUND",
		"This organization has no license",
	)
	ErrOrganizationLicenseAlreadyExists = fault.Conflict(
		"ORGANIZATION_LICENSE_EXISTS",
		"This organization already has a license; adjust it instead",
	)
	ErrLicenseOrganizationNotFound = fault.NotFound(
		"ORGANIZATION_NOT_FOUND",
		"This product has no organization with that identifier",
	)
)

// OrganizationLicenseService owns one Organization's license: its own copy of a
// [license.Template]'s values. See
// docs/adr/0004-license-schema-template-and-copy.md.
//
// As with a template, whether a set of values satisfies the declaration is the
// schema's question, answered by [LicenseSchemaService.ValidateValues]. This
// service holds no schema repository, so an adjustment cannot drift from what a
// template write is held to.
type OrganizationLicenseService interface {
	// Instantiate joins the caller's transaction when ctx carries one, so an
	// Organization and its first license can be written as one unit.
	Instantiate(
		ctx context.Context, in license.InstantiateLicenseInput,
	) (license.OrganizationLicense, error)
	// GetLicense returns the Organization's effective license — its own copy
	// of a template's values, plus every limit field's latest usage and
	// derived status — or nil when it has never been instantiated.
	GetLicense(
		ctx context.Context, in license.GetLicenseInput,
	) (*license.OrganizationLicenseRead, error)
	// ListByOrganizations reads many Organizations' licenses at once, keyed by
	// organization ID, for a caller composing them onto something else. It
	// carries no usage and does not go through the per-organization cache — one
	// statement rather than a cache lookup per organization.
	ListByOrganizations(
		ctx context.Context, in license.ListLicensesByOrganizationsInput,
	) (map[string]license.OrganizationLicense, error)
	AdjustValues(
		ctx context.Context, in license.AdjustLicenseInput,
	) (license.OrganizationLicense, error)
	DiffAgainstTemplate(
		ctx context.Context, in license.GetLicenseInput,
	) (license.OrganizationLicenseDiff, error)
	// Search reads a page of the Product's customer book: each Organization and
	// the license it holds. Usage is not derived — see
	// [OrganizationLicenseService.GetLicense] for one Organization's full read.
	Search(
		ctx context.Context, in license.SearchOrganizationLicensesInput,
	) (search.Result[license.OrganizationLicenseSummary], error)
}

type organizationLicenseService struct {
	licenseRepo licenserepo.OrganizationLicenseRepository
	templates   LicenseTemplateService
	schemas     LicenseSchemaService
	usage       licenserepo.UsageObservationRepository
	changes     licenserepo.OrganizationLicenseChangeRepository
	transactor  transactor.Transactor
	licenses    *organizationLicenseCache
	logger      zerolog.Logger
}

func NewOrganizationLicenseService(
	licenseRepo licenserepo.OrganizationLicenseRepository,
	templates LicenseTemplateService,
	schemas LicenseSchemaService,
	usage licenserepo.UsageObservationRepository,
	changes licenserepo.OrganizationLicenseChangeRepository,
	tx transactor.Transactor,
	cacheStore cache.Store,
	logger zerolog.Logger,
) OrganizationLicenseService {
	return &organizationLicenseService{
		licenseRepo: licenseRepo,
		templates:   templates,
		schemas:     schemas,
		usage:       usage,
		changes:     changes,
		transactor:  tx,
		licenses:    newOrganizationLicenseCache(cacheStore, logger),
		logger:      logger.With().Str("component", "organization_license_service").Logger(),
	}
}

func (s *organizationLicenseService) Instantiate(
	ctx context.Context, in license.InstantiateLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	var created license.OrganizationLicense
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		template, templateErr := s.templates.GetTemplate(txCtx, license.GetTemplateInput{
			TenantID:   in.TenantID,
			ProductID:  in.ProductID,
			TemplateID: in.TemplateID,
		})
		if templateErr != nil {
			return templateErr
		}
		if template == nil {
			return ErrLicenseTemplateNotFoundInRequest(in.TemplateID)
		}
		// A withdrawn tier cannot be sold to anyone else. The organizations
		// already on it keep what they hold.
		if template.IsArchived() {
			return ErrLicenseTemplateArchived
		}

		// The copied values are deliberately not re-validated: a schema tightened
		// since the template was last written would otherwise block onboarding onto
		// a tier still on sale, and Anchor validates but never gates. See
		// docs/adr/0009-every-license-field-is-mandatory.md.
		instantiatedAt := time.Now()
		instantiated := license.OrganizationLicense{
			PlatformTenantID: in.TenantID,
			ProductID:        in.ProductID,
			OrganizationID:   in.OrganizationID,
			TemplateID:       template.ID,
			InstantiatedAt:   instantiatedAt,
			Values:           template.Values,
		}
		instantiated.GenerateID()

		var createErr error
		created, createErr = s.licenseRepo.Create(txCtx, instantiated)
		if createErr != nil {
			// Checked by the foreign key rather than by a read before the write:
			// one round trip, and no window in which the Organization is deleted
			// between the check and the insert.
			if pgerr.IsForeignKeyViolation(createErr, organizationLicenseOrganizationConstraint) {
				return ErrLicenseOrganizationNotFound
			}
			if pgerr.IsUniqueViolation(createErr, organizationLicenseUniqueConstraint) {
				return ErrOrganizationLicenseAlreadyExists
			}
			return createErr
		}

		return s.changes.Append(txCtx, []license.OrganizationLicenseChange{
			license.NewInstantiationChange(created, instantiatedAt),
		})
	}); txErr != nil {
		return license.OrganizationLicense{}, txErr
	}

	// Nothing can be cached before the license exists — a not-found read is
	// never cached, see cache.Cache.GetOrElse — so this is a no-op today. It
	// stays here so every license write follows the same eviction discipline,
	// rather than relying on that invariant to hold forever. Inside a caller's
	// transaction it runs before their commit, the harmless direction: a read
	// racing them cannot see the row and so caches nothing.
	s.licenses.evict(ctx, in.ProductID, in.OrganizationID)

	return created, nil
}

// GetLicense reads the Organization's license record from cache — evicted on
// every write, see organizationLicenseCache — and derives usage against it
// fresh on every call. Usage itself is never cached: were it folded into the
// cached value, a report arriving between evictions would stay invisible until
// the TTL expired, which is not the contract. See
// docs/adr/0012-license-status-derived-on-read.md.
func (s *organizationLicenseService) GetLicense(
	ctx context.Context, in license.GetLicenseInput,
) (*license.OrganizationLicenseRead, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	base, err := s.licenses.key(in.ProductID, in.OrganizationID).GetOrElse(
		ctx, func() (*license.OrganizationLicense, error) {
			found, err := s.licenseRepo.FindByOrganization(ctx, in.TenantID, in.ProductID, in.OrganizationID)
			if err != nil {
				return nil, err
			}
			return found.ToPtr(), nil
		},
	)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, nil //nolint:nilnil // absence is not an error; the handler maps it to 404
	}

	read, err := s.withUsage(ctx, in.TenantID, in.ProductID, *base)
	if err != nil {
		return nil, err
	}
	return &read, nil
}

func (s *organizationLicenseService) ListByOrganizations(
	ctx context.Context, in license.ListLicensesByOrganizationsInput,
) (map[string]license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	return s.licenseRepo.FindByOrganizations(ctx, in.TenantID, in.ProductID, in.OrganizationIDs)
}

// withUsage composes the cached license record with every limit field's
// latest usage and derived status, read fresh from the schema and the usage
// observations — never from the license cache.
func (s *organizationLicenseService) withUsage(
	ctx context.Context, tenantID, productID string, base license.OrganizationLicense,
) (license.OrganizationLicenseRead, error) {
	read := license.OrganizationLicenseRead{OrganizationLicense: base, Usage: map[string]license.FieldUsage{}}

	schema, err := s.schemas.GetSchema(ctx, license.GetSchemaInput{TenantID: tenantID, ProductID: productID})
	if err != nil {
		return license.OrganizationLicenseRead{}, err
	}
	if schema == nil {
		return read, nil
	}

	latest, err := s.usage.LatestPerKey(ctx, tenantID, productID, base.OrganizationID)
	if err != nil {
		return license.OrganizationLicenseRead{}, err
	}
	latestByKey := functional.Slice(latest).ToMap(func(o license.UsageObservation) string { return o.Key })

	read.Usage = license.DeriveUsage(schema.Fields, base.Values, latestByKey)
	return read, nil
}

func (s *organizationLicenseService) AdjustValues(
	ctx context.Context, in license.AdjustLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	// One clock for the request. The update row uses the database clock; mixing
	// the two can order a later change before the one it followed.
	changedAt := time.Now()

	var updated license.OrganizationLicense
	wrote := false
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		foundExisting, findErr := s.licenseRepo.FindByOrganizationForUpdate(
			txCtx, in.TenantID, in.ProductID, in.OrganizationID,
		)
		if findErr != nil {
			return findErr
		}
		if foundExisting.IsAbsent() {
			return ErrOrganizationLicenseNotFound
		}
		existing := foundExisting.ToPtr()

		// Merged, not replaced. The merged set is validated whole, so a license
		// that has fallen behind a tightened schema is corrected rather than re-saved.
		previous := existing.Values
		adjusted := existing.AdjustedValues(in.Values)
		if err := s.schemas.ValidateValues(txCtx, in.TenantID, in.ProductID, adjusted); err != nil {
			return err
		}
		existing.Values = adjusted

		changes := license.NewAdjustmentChanges(*existing, previous, changedAt)
		if len(changes) == 0 {
			updated = *existing
			return nil
		}

		// Every field the adjustment moved is pinned: a propagated template
		// update leaves it alone from here on. A field written back at its
		// current value moved nothing, records no history entry, and pins
		// nothing. See docs/adr/0017-license-follows-its-template.md.
		existing.RecordAdjustedFields(functional.Slice(changes).Map(
			func(c license.OrganizationLicenseChange) string { return *c.Field },
		))

		var updateErr error
		updated, updateErr = s.licenseRepo.Update(txCtx, in.TenantID, *existing)
		if updateErr != nil {
			return updateErr
		}
		wrote = true
		return s.changes.Append(txCtx, changes)
	}); txErr != nil {
		return license.OrganizationLicense{}, txErr
	}

	if wrote {
		s.licenses.evict(ctx, in.ProductID, in.OrganizationID)
	}

	return updated, nil
}

// Search does not read through the license cache. The cache holds one
// Organization's record at a time, so a page of results would have to be
// assembled from as many lookups as it has rows, and a miss on any one of them
// would query the database anyway.
func (s *organizationLicenseService) Search(
	ctx context.Context, in license.SearchOrganizationLicensesInput,
) (search.Result[license.OrganizationLicenseSummary], error) {
	if err := validate.ValidateStruct(in); err != nil {
		return search.Result[license.OrganizationLicenseSummary]{}, err
	}
	return s.licenseRepo.Search(ctx, in)
}

func (s *organizationLicenseService) DiffAgainstTemplate(
	ctx context.Context, in license.GetLicenseInput,
) (license.OrganizationLicenseDiff, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicenseDiff{}, err
	}

	found, err := s.licenseRepo.FindByOrganization(
		ctx, in.TenantID, in.ProductID, in.OrganizationID,
	)
	if err != nil {
		return license.OrganizationLicenseDiff{}, err
	}
	if found.IsAbsent() {
		return license.OrganizationLicenseDiff{}, ErrOrganizationLicenseNotFound
	}
	existing := found.Value()

	// The template always resolves, archived or not: it is never deleted, and a
	// foreign key holds the reference. The nil branch below guards a row written
	// before that constraint existed.
	template, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: existing.TemplateID,
	})
	if err != nil {
		return license.OrganizationLicenseDiff{}, err
	}
	if template == nil {
		return license.OrganizationLicenseDiff{}, ErrLicenseTemplateNotFound
	}

	return license.OrganizationLicenseDiff{
		OrganizationID: existing.OrganizationID,
		TemplateID:     template.ID,
		Differences:    license.DiffValues(existing.Values, template.Values),
	}, nil
}
