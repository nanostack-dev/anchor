package service

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	intrepo "anchor/internal/repository"
)

var (
	// ErrLicenseMigrationSelectionInvalid refuses a run that names both
	// selections or neither. The two are alternatives, not filters that
	// compose: an explicit list already says which Organizations to move, and
	// narrowing it by their current template would only ever remove entries the
	// caller asked for by name.
	ErrLicenseMigrationSelectionInvalid = fault.BadRequest(
		"LICENSE_MIGRATION_SELECTION_INVALID",
		"Supply exactly one of organization_ids or from_template_id",
	)
	// ErrLicenseMigrationSourceTemplateNotFound reports a from_template_id that
	// names nothing. Distinct from the target's own 404, because a caller who
	// mistyped it would otherwise read an empty run as "nobody was on that
	// tier".
	ErrLicenseMigrationSourceTemplateNotFound = fault.NewWithStatus(
		"LICENSE_MIGRATION_SOURCE_TEMPLATE_NOT_FOUND",
		"This product has no license template with that identifier to migrate from",
		http.StatusNotFound,
	)
)

// errLicenseMigrationTooLarge refuses a selection past the cap rather than
// covering part of it. See
// docs/adr/0014-organization-licenses-are-migrated-in-bulk.md.
func errLicenseMigrationTooLarge(matched int) *fault.Error {
	return fault.NewWithStatus(
		"LICENSE_MIGRATION_TOO_LARGE",
		fmt.Sprintf(
			"This selection matches %d organizations; at most %d can be migrated in one run",
			matched, license.MaxMigrationOrganizations,
		),
		http.StatusBadRequest,
	)
}

// LicenseMigrationService moves a set of Organizations onto one license
// template: a fresh copy of its values, and the provenance restamped to say
// which tier the customer is on now.
//
// This is the only operation that changes an Organization's `template_id`.
// [OrganizationLicenseService.AdjustValues] deliberately cannot, so a bespoke
// change can never be mistaken for a tier change, and a tier change can never
// be mistaken for a run of bespoke ones. See
// docs/adr/0014-organization-licenses-are-migrated-in-bulk.md.
type LicenseMigrationService interface {
	Migrate(
		ctx context.Context, in license.MigrateLicensesInput,
	) (license.Migration, error)
}

type licenseMigrationService struct {
	licenseRepo   licenserepo.OrganizationLicenseRepository
	templates     LicenseTemplateService
	changes       licenserepo.OrganizationLicenseChangeRepository
	organizations intrepo.OrganizationRepository
	transactor    transactor.Transactor
	licenses      *organizationLicenseCache
	logger        zerolog.Logger
}

func NewLicenseMigrationService(
	licenseRepo licenserepo.OrganizationLicenseRepository,
	templates LicenseTemplateService,
	changes licenserepo.OrganizationLicenseChangeRepository,
	organizations intrepo.OrganizationRepository,
	tx transactor.Transactor,
	cacheStore cache.Store,
	logger zerolog.Logger,
) LicenseMigrationService {
	return &licenseMigrationService{
		licenseRepo:   licenseRepo,
		templates:     templates,
		changes:       changes,
		organizations: organizations,
		transactor:    tx,
		licenses:      newOrganizationLicenseCache(cacheStore, logger),
		logger:        logger.With().Str("component", "license_migration_service").Logger(),
	}
}

// migrationRun carries what every Organization in one run is decided against:
// the request, the target, the shared clock, and the templates already read.
// Templates are memoised because carrying a difference forward needs the values
// of the tier the Organization is on today, and a cohort shares a handful of
// tiers between hundreds of Organizations.
type migrationRun struct {
	input      license.MigrateLicensesInput
	target     license.Template
	migratedAt time.Time
	templates  map[string]*license.Template
}

func (r *migrationRun) policy() license.DifferencePolicy {
	if r.input.OnDifference == license.DiscardDifferences {
		return license.DiscardDifferences
	}
	return license.CarryForwardDifferences
}

func (s *licenseMigrationService) Migrate(
	ctx context.Context, in license.MigrateLicensesInput,
) (license.Migration, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Migration{}, err
	}
	if (len(in.OrganizationIDs) == 0) == (in.FromTemplateID == "") {
		return license.Migration{}, ErrLicenseMigrationSelectionInvalid
	}

	target, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: in.TemplateID,
	})
	if err != nil {
		return license.Migration{}, err
	}
	if target == nil {
		return license.Migration{}, ErrLicenseTemplateNotFound
	}
	if target.IsArchived() {
		return license.Migration{}, ErrLicenseTemplateArchived
	}

	organizationIDs, err := s.selectOrganizations(ctx, in)
	if err != nil {
		return license.Migration{}, err
	}
	if len(organizationIDs) > license.MaxMigrationOrganizations {
		return license.Migration{}, errLicenseMigrationTooLarge(len(organizationIDs))
	}

	run := &migrationRun{
		input:      in,
		target:     *target,
		migratedAt: time.Now(),
		templates:  map[string]*license.Template{target.ID: target},
	}

	migration := license.Migration{
		TemplateID: target.ID,
		MigratedAt: run.migratedAt,
		Results:    make([]license.OrganizationMigrationResult, 0, len(organizationIDs)),
	}
	for _, organizationID := range organizationIDs {
		migration.Results = append(migration.Results, s.migrateOne(ctx, run, organizationID))
	}

	tally := migration.Tally()
	s.logger.Info().
		Str("product_id", in.ProductID).
		Str("license_template_id", target.ID).
		Str("on_difference", string(run.policy())).
		Int("considered", len(migration.Results)).
		Int("changed", tally.Changed).
		Int("unchanged", tally.Unchanged).
		Int("failed", tally.Failed).
		Msg("organization license migration run finished")

	return migration, nil
}

// selectOrganizations resolves the run's selection, sorted and deduplicated so
// results come back in a stable order and an Organization named twice is moved
// once.
func (s *licenseMigrationService) selectOrganizations(
	ctx context.Context, in license.MigrateLicensesInput,
) ([]string, error) {
	if in.FromTemplateID == "" {
		// Bounded before the duplicates come out, not after. The contract bounds
		// organization_ids, and nothing validates request bodies at the edge
		// (see ExcludeRequestBody in cmd/http/server.go), so refusing the list
		// as sent is what makes the documented cap real — a caller naming a
		// hundred thousand organizations is answered rather than deduplicated.
		if len(in.OrganizationIDs) > license.MaxMigrationOrganizations {
			return nil, errLicenseMigrationTooLarge(len(in.OrganizationIDs))
		}

		unique := make(map[string]struct{}, len(in.OrganizationIDs))
		for _, organizationID := range in.OrganizationIDs {
			unique[organizationID] = struct{}{}
		}
		return slices.Sorted(maps.Keys(unique)), nil
	}

	source, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: in.FromTemplateID,
	})
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrLicenseMigrationSourceTemplateNotFound
	}

	return s.licenseRepo.ListOrganizationIDsForTemplate(
		ctx, in.TenantID, in.ProductID, in.FromTemplateID,
	)
}

// migrateOne decides and applies one Organization's move, in its own
// transaction. A failure is reported against that Organization and the batch
// continues: no invariant spans two Organizations' licenses, so a batch-wide
// transaction would only mean one bad row discarding several hundred good ones.
func (s *licenseMigrationService) migrateOne(
	ctx context.Context, run *migrationRun, organizationID string,
) license.OrganizationMigrationResult {
	var result license.OrganizationMigrationResult
	wrote := false

	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.licenseRepo.FindByOrganizationForUpdate(
			txCtx, run.input.TenantID, run.input.ProductID, organizationID,
		)
		if findErr != nil {
			return findErr
		}

		decided, changed, decideErr := s.decide(txCtx, run, organizationID, existing)
		if decideErr != nil {
			return decideErr
		}
		result = decided

		if decided.Outcome != license.OutcomeChanged {
			return nil
		}

		var previousValues license.TemplateValues
		var written license.OrganizationLicense
		var writeErr error
		if existing == nil {
			written, writeErr = s.licenseRepo.Create(txCtx, changed)
		} else {
			previousValues = existing.Values
			written, writeErr = s.licenseRepo.Restamp(txCtx, run.input.TenantID, changed)
		}
		if writeErr != nil {
			return writeErr
		}
		wrote = true

		return s.changes.Append(txCtx, []license.OrganizationLicenseChange{
			license.NewMigrationChange(
				written, decided.PreviousTemplateID, previousValues, run.migratedAt,
			),
		})
	}); txErr != nil {
		s.logger.Warn().
			Str("product_id", run.input.ProductID).
			Str("organization_id", organizationID).
			Err(txErr).
			Msg("organization left behind by a license migration")
		return license.OrganizationMigrationResult{
			OrganizationID: organizationID,
			Outcome:        license.OutcomeFailed,
			Error:          txErr,
		}
	}

	if wrote {
		s.licenses.evict(ctx, run.input.ProductID, organizationID)
	}
	return result
}

// decide works out what happens to one Organization, and what its license would
// hold afterwards, without writing anything.
//
// Carrying a difference forward is measured against the template the
// Organization is on today, not against the target: what survives the move is a
// value that has come apart from its own tier. Nothing can tell a bespoke
// adjustment from a tier that moved after the copy was taken — see
// [license.DiffValues] — so the reported changes are what an operator reads
// afterwards, and comparing the two templates is what they do beforehand.
//
// An Organization holding no license is granted one on the target, exactly as
// a licensed Organization is moved onto it: on_difference does not apply, since
// there is no held value to carry forward or discard, and DiffValues against a
// nil set reports every field as only_in_template — which is what a first
// grant is. See docs/adr/0015-migrate-grants-a-first-license.md.
func (s *licenseMigrationService) decide(
	ctx context.Context,
	run *migrationRun,
	organizationID string,
	existing *license.OrganizationLicense,
) (license.OrganizationMigrationResult, license.OrganizationLicense, error) {
	result := license.OrganizationMigrationResult{OrganizationID: organizationID}

	if existing == nil {
		organization, err := s.organizations.FindByID(ctx, run.input.ProductID, organizationID)
		if err != nil {
			return result, license.OrganizationLicense{}, err
		}
		if organization == nil {
			return result, license.OrganizationLicense{}, ErrLicenseOrganizationNotFound
		}

		granted := license.OrganizationLicense{
			PlatformTenantID: run.input.TenantID,
			ProductID:        run.input.ProductID,
			OrganizationID:   organizationID,
			TemplateID:       run.target.ID,
			InstantiatedAt:   run.migratedAt,
			Values:           run.target.Values,
		}
		granted.GenerateID()

		result.Outcome = license.OutcomeChanged
		result.Changes = license.DiffValues(nil, granted.Values)
		return result, granted, nil
	}

	current, err := s.templateByID(ctx, run, existing.TemplateID)
	if err != nil {
		return result, license.OrganizationLicense{}, err
	}

	migrated := existing.MigratedTo(run.target, current.Values, run.policy(), run.migratedAt)
	result.PreviousTemplateID = new(existing.TemplateID)
	result.Changes = license.DiffValues(existing.Values, migrated.Values)

	if existing.TemplateID == run.target.ID && len(result.Changes) == 0 {
		result.Outcome = license.OutcomeUnchanged
		return result, license.OrganizationLicense{}, nil
	}

	result.Outcome = license.OutcomeChanged
	return result, migrated, nil
}

// templateByID reads a template once per run. A template a license names always
// resolves — migration 000028 made it a foreign key and nothing deletes the row
// — so a miss here means a row written before that constraint existed, and the
// Organization fails rather than being moved against values that are gone.
func (s *licenseMigrationService) templateByID(
	ctx context.Context, run *migrationRun, templateID string,
) (*license.Template, error) {
	if known, ok := run.templates[templateID]; ok {
		return known, nil
	}

	found, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   run.input.TenantID,
		ProductID:  run.input.ProductID,
		TemplateID: templateID,
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, ErrLicenseTemplateNotFound
	}

	run.templates[templateID] = found
	return found, nil
}
