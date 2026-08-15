package service

import (
	"context"
	"net/http"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// The organization license read is cached the same way the product API key
// permission cache is: keyed by (product, organization), evicted on every
// write, degrading to a no-op when Redis is unavailable. See
// docs/api-key-permission-cache.md and
// docs/adr/0012-license-status-derived-on-read.md.
//
// Usage is deliberately not part of the cached value — see GetLicense — so a
// usage report becomes visible on the next read without touching this cache.
const (
	organizationLicenseCacheTTL    = 15 * time.Minute
	organizationLicenseCachePrefix = "organization_license"
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
	ErrOrganizationLicenseNotFound = fault.NewWithStatus(
		"ORGANIZATION_LICENSE_NOT_FOUND",
		"This organization has no license",
		http.StatusNotFound,
	)
	ErrOrganizationLicenseAlreadyExists = fault.BadRequest(
		"ORGANIZATION_LICENSE_EXISTS",
		"This organization already has a license; adjust it instead",
	)
	ErrLicenseOrganizationNotFound = fault.NewWithStatus(
		"ORGANIZATION_NOT_FOUND",
		"This product has no organization with that identifier",
		http.StatusNotFound,
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
	Instantiate(
		ctx context.Context, in license.InstantiateLicenseInput,
	) (license.OrganizationLicense, error)
	// GetLicense returns the Organization's effective license — its own copy
	// of a template's values, plus every limit field's latest usage and
	// derived status — or nil when it has never been instantiated.
	GetLicense(
		ctx context.Context, in license.GetLicenseInput,
	) (*license.OrganizationLicenseRead, error)
	AdjustValues(
		ctx context.Context, in license.AdjustLicenseInput,
	) (license.OrganizationLicense, error)
	DiffAgainstTemplate(
		ctx context.Context, in license.GetLicenseInput,
	) (license.OrganizationLicenseDiff, error)
}

type organizationLicenseService struct {
	licenseRepo licenserepo.OrganizationLicenseRepository
	templates   LicenseTemplateService
	schemas     LicenseSchemaService
	usage       licenserepo.UsageObservationRepository
	changes     licenserepo.OrganizationLicenseChangeRepository
	transactor  transactor.Transactor
	licenses    cache.Cache[license.OrganizationLicense]
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
		licenses: cache.New[license.OrganizationLicense](
			cacheStore, organizationLicenseCachePrefix, organizationLicenseCacheTTL, logger,
		),
		logger: logger.With().Str("component", "organization_license_service").Logger(),
	}
}

// licenseCacheKey addresses one Organization's cached license. Product and
// Organization IDs are KSUIDs unique platform-wide, so — as with the API key
// cache — the tenant does not need to be part of the key.
func (s *organizationLicenseService) licenseCacheKey(
	productID, organizationID string,
) cache.Entry[license.OrganizationLicense] {
	return s.licenses.Key(productID, organizationID)
}

// evictLicenseCache logs and swallows a failure, matching ProductAPIKeyService:
// a cache that cannot be evicted should not fail the write that triggered it,
// only degrade the next read.
func (s *organizationLicenseService) evictLicenseCache(ctx context.Context, productID, organizationID string) {
	if err := s.licenseCacheKey(productID, organizationID).Evict(ctx); err != nil {
		s.logger.Error().
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Err(err).
			Msg("failed to evict organization license from cache")
	}
}

func (s *organizationLicenseService) Instantiate(
	ctx context.Context, in license.InstantiateLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	template, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: in.TemplateID,
	})
	if err != nil {
		return license.OrganizationLicense{}, err
	}
	if template == nil {
		return license.OrganizationLicense{}, ErrLicenseTemplateNotFound
	}
	// A withdrawn tier cannot be sold to anyone else. The organizations already
	// on it keep what they hold.
	if template.IsArchived() {
		return license.OrganizationLicense{}, ErrLicenseTemplateArchived
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

	// The license and its first history entry land together or not at all: a
	// license whose instantiation went unrecorded would read as one that was
	// never granted.
	var created license.OrganizationLicense
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
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
	// rather than relying on that invariant to hold forever.
	s.evictLicenseCache(ctx, in.ProductID, in.OrganizationID)

	return created, nil
}

// GetLicense reads the Organization's license record from cache — evicted on
// every write, see evictLicenseCache — and derives usage against it fresh on
// every call. Usage itself is never cached: were it folded into the cached
// value, a report arriving between evictions would stay invisible until the
// TTL expired, which is not the contract. See
// docs/adr/0012-license-status-derived-on-read.md.
func (s *organizationLicenseService) GetLicense(
	ctx context.Context, in license.GetLicenseInput,
) (*license.OrganizationLicenseRead, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	base, err := s.licenseCacheKey(in.ProductID, in.OrganizationID).GetOrElse(
		ctx, func() (*license.OrganizationLicense, error) {
			return s.licenseRepo.FindByOrganization(ctx, in.TenantID, in.ProductID, in.OrganizationID)
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
	latestByKey := make(map[string]license.UsageObservation, len(latest))
	for _, observation := range latest {
		latestByKey[observation.Key] = observation
	}

	read.Usage = license.DeriveUsage(schema.Fields, base.Values, latestByKey)
	return read, nil
}

func (s *organizationLicenseService) AdjustValues(
	ctx context.Context, in license.AdjustLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	existing, err := s.licenseRepo.FindByOrganization(
		ctx, in.TenantID, in.ProductID, in.OrganizationID,
	)
	if err != nil {
		return license.OrganizationLicense{}, err
	}
	if existing == nil {
		return license.OrganizationLicense{}, ErrOrganizationLicenseNotFound
	}

	// Merged, not replaced. The merged set is validated whole, so a license that
	// has fallen behind a tightened schema is corrected rather than re-saved.
	previous := existing.Values
	adjusted := existing.AdjustedValues(in.Values)
	if err = s.schemas.ValidateValues(ctx, in.TenantID, in.ProductID, adjusted); err != nil {
		return license.OrganizationLicense{}, err
	}
	existing.Values = adjusted

	// One reading of the clock for the whole request, and the same clock every
	// other history entry is stamped from. Taking it from the row the update
	// returns would read the database's clock instead, and a history ordered
	// across two clocks can report a change before the change it followed.
	changedAt := time.Now()

	// The adjustment and the entries recording it land together or not at all:
	// a raised limit whose history entry was lost would leave the account
	// looking like it was always on that number.
	var updated license.OrganizationLicense
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.licenseRepo.Update(txCtx, in.TenantID, *existing)
		if updateErr != nil {
			return updateErr
		}
		return s.changes.Append(
			txCtx, license.NewAdjustmentChanges(updated, previous, changedAt),
		)
	}); txErr != nil {
		return license.OrganizationLicense{}, txErr
	}

	// Without this, a limit lowered to make a previously-compliant Organization
	// exceeded would read as compliant until the cache entry's TTL expired.
	s.evictLicenseCache(ctx, in.ProductID, in.OrganizationID)

	return updated, nil
}

func (s *organizationLicenseService) DiffAgainstTemplate(
	ctx context.Context, in license.GetLicenseInput,
) (license.OrganizationLicenseDiff, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicenseDiff{}, err
	}

	existing, err := s.licenseRepo.FindByOrganization(
		ctx, in.TenantID, in.ProductID, in.OrganizationID,
	)
	if err != nil {
		return license.OrganizationLicenseDiff{}, err
	}
	if existing == nil {
		return license.OrganizationLicenseDiff{}, ErrOrganizationLicenseNotFound
	}

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
