package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"anchor/internal/domain/email"
	domainintegration "anchor/internal/domain/integration"
	"anchor/internal/email/renderer"
	emailrepo "anchor/internal/email/repository"
	"anchor/internal/integration/provider"
	intrepo "anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
)

const emailSendDedupeConstraint = "idx_email_send_records_dedupe"

// emailStatusPersistTimeout bounds the terminal send-status write that runs on
// a context detached from the caller's request, so the write cannot hang.
const emailStatusPersistTimeout = 5 * time.Second

var (
	ErrEmailIntegrationNotConfigured = fault.Conflict(
		"EMAIL_INTEGRATION_NOT_CONFIGURED",
		"No email integration is configured for this product",
	)
	ErrEmailIntegrationInactive = fault.Conflict(
		"EMAIL_INTEGRATION_INACTIVE",
		"The email integration is not in an ACTIVE state",
	)
	ErrEmailTemplateNotFound = fault.NotFound(
		"EMAIL_TEMPLATE_NOT_FOUND",
		"Email template not found",
	)
	ErrEmailTemplateNotPublished = fault.Conflict(
		"EMAIL_TEMPLATE_NOT_PUBLISHED",
		"This template has no published version. Publish the template before you send with it.",
	)
	ErrEmailTemplateNoDraft = fault.Conflict(
		"EMAIL_TEMPLATE_NO_DRAFT",
		"This template has no draft version available",
	)
	ErrEmailTemplateSlugTaken = fault.Conflict(
		"EMAIL_TEMPLATE_SLUG_TAKEN",
		"An email template with this slug already exists for the product",
	)
	ErrEmailRateLimitExceeded = fault.TooManyRequests(
		"EMAIL_RATE_LIMIT_EXCEEDED",
		"Sends for this product exceeded the configured rate limit",
	)
	ErrEmailMailerCapabilityMissing = fault.Conflict(
		"EMAIL_MAILER_CAPABILITY_MISSING",
		"The configured integration does not support outbound email",
	)
	ErrEmailTemplateSelectorMissing = fault.BadRequest(
		"EMAIL_TEMPLATE_SELECTOR_MISSING",
		"Either template_id or template_slug must be supplied",
	)
	// ErrEmailDeliveryFailed models a failure to hand a message off to the
	// configured email integration (SMTP relay rejection, auth failure, dial
	// error, or an undeliverable recipient/sender). Delivery failure is a known
	// operational outcome, not a "truly unexpected" bug, so it is a modelled 500:
	// the boundary handler logs it as an intentional internal error with a stable
	// code instead of the generic "unhandled error" safety net, and the client
	// gets EMAIL_DELIVERY_FAILED rather than an opaque UNEXPECTED. The transport
	// cause is wrapped for server-side logs but never serialized to the client.
	ErrEmailDeliveryFailed = fault.Internal(
		"EMAIL_DELIVERY_FAILED",
		"Failed to deliver the email through the configured integration",
	)
	// ErrEmailDeliveryRejected models a permanent rejection of the message by the
	// configured relay (unauthorized sender identity, undeliverable recipient, or
	// rejected content). Unlike ErrEmailDeliveryFailed this is a caller-input
	// problem, not a server fault: retrying the same request will not help.
	ErrEmailDeliveryRejected = fault.BadRequest(
		"EMAIL_DELIVERY_REJECTED",
		"The email integration rejected the message. Check that the sender "+
			"identity is authorized and the recipient address is valid.",
	)
)

// classifyDispatchError maps a mailer dispatch failure to the API error the
// caller should see. A permanent rejection by the relay is the caller's problem
// and surfaces as a 422 EMAIL_DELIVERY_REJECTED; every other transport failure
// is a modelled 500 EMAIL_DELIVERY_FAILED. A cancelled context means the caller
// (or shutdown) went away mid-send, not a delivery fault, so it is returned
// unchanged to keep the context-cancellation logging policy (pkg/log) from
// surfacing it on error dashboards and alerts.
func classifyDispatchError(err error) error {
	if log.IsContextError(err) {
		return err
	}
	if errors.Is(err, provider.ErrMessageRejected) {
		return ErrEmailDeliveryRejected.Wrap(err)
	}
	return ErrEmailDeliveryFailed.Wrap(err)
}

// errEmailRequiredVariablesMissing builds the validation error returned when a
// send omits variables the template version marks required. The missing names
// are surfaced both in the message and as machine-readable metadata so clients
// can highlight the offending fields.
func errEmailRequiredVariablesMissing(missing []string) *fault.Error {
	return fault.BadRequest(
		"EMAIL_REQUIRED_VARIABLES_MISSING",
		fmt.Sprintf("Missing required template variable(s): %s", strings.Join(missing, ", ")),
	).Metadata(map[string]any{"missing_variables": missing})
}

// EmailService is the product-facing entry point for managing templates
// and dispatching transactional email.
//
// Templates are versioned: every Template owns at most one DRAFT and one
// PUBLISHED version. CreateTemplate seeds a DRAFT only; senders must
// publish before the template is usable from the public API. Send always
// uses the PUBLISHED version unless SendInput.UseDraft is set (admin path).
type EmailService interface {
	CreateTemplate(ctx context.Context, in email.CreateTemplateInput) (email.Template, error)
	UpdateTemplate(ctx context.Context, in email.UpdateTemplateInput) (email.Template, error)
	UpdateTemplateDraft(ctx context.Context, in email.UpdateTemplateDraftInput) (email.TemplateVersion, error)
	GetTemplateDraft(ctx context.Context, in email.GetTemplateDraftInput) (*email.TemplateVersion, error)
	PublishTemplate(ctx context.Context, in email.PublishTemplateInput) (email.TemplateVersion, error)
	GetTemplate(ctx context.Context, in email.GetTemplateInput) (*email.Template, error)
	ListTemplates(ctx context.Context, in email.ListTemplatesInput) ([]email.Template, error)
	DeleteTemplate(ctx context.Context, in email.DeleteTemplateInput) error

	SaveTemplateExamples(ctx context.Context, in email.SaveTemplateExamplesInput) ([]email.TemplateExample, error)

	Preview(ctx context.Context, in email.PreviewInput) (email.PreviewResult, error)
	Send(ctx context.Context, in email.SendInput) (email.SendRecord, error)
	TestSend(ctx context.Context, in email.TestSendInput) (email.SendRecord, error)

	ListSends(ctx context.Context, in email.ListSendsInput) ([]email.SendRecord, error)
}

type emailService struct {
	transactor   transactor.Transactor
	templateRepo emailrepo.TemplateRepository
	versionRepo  emailrepo.TemplateVersionRepository
	sendRepo     emailrepo.SendRecordRepository
	instanceRepo intrepo.IntegrationInstanceRepository
	registry     *provider.Registry
	renderer     *renderer.Renderer
	rateLimits   []email.RateLimitWindow
	logger       zerolog.Logger
}

// NewEmailService constructs the email orchestration service. The rate-limit
// windows are a hard-coded default for v1; switch to per-product config when
// real customers ask.
func NewEmailService(
	transactor transactor.Transactor,
	templateRepo emailrepo.TemplateRepository,
	versionRepo emailrepo.TemplateVersionRepository,
	sendRepo emailrepo.SendRecordRepository,
	instanceRepo intrepo.IntegrationInstanceRepository,
	registry *provider.Registry,
	r *renderer.Renderer,
	logger zerolog.Logger,
) EmailService {
	return &emailService{
		transactor:   transactor,
		templateRepo: templateRepo,
		versionRepo:  versionRepo,
		sendRepo:     sendRepo,
		instanceRepo: instanceRepo,
		registry:     registry,
		renderer:     r,
		rateLimits:   email.DefaultRateLimits,
		logger:       logger.With().Str("component", "email_service").Logger(),
	}
}

// resolveMailer looks up the product's email integration, type-asserts the
// Mailer capability, and returns both the instance and the typed provider.
func (s *emailService) resolveMailer(
	ctx context.Context,
	tenantID, productID string,
) (provider.Mailer, *domainintegration.Instance, error) {
	found, err := s.instanceRepo.FindByProductAndProvider(
		ctx, tenantID, productID, string(domainintegration.ProviderTypeSMTP),
	)
	if err != nil {
		return nil, nil, err
	}
	if found.IsAbsent() {
		return nil, nil, ErrEmailIntegrationNotConfigured
	}
	instance := found.ToPtr()
	if instance.Status != domainintegration.StatusActive || !instance.IsEnabled {
		return nil, nil, ErrEmailIntegrationInactive
	}

	prov, err := s.registry.GetProvider(string(instance.ProviderType))
	if err != nil {
		return nil, instance, err
	}
	mailer, ok := prov.(provider.Mailer)
	if !ok {
		return nil, instance, ErrEmailMailerCapabilityMissing
	}
	return mailer, instance, nil
}

func (s *emailService) resolveTemplate(
	ctx context.Context, tenantID, productID string, idPtr, slugPtr *string,
) (*email.Template, error) {
	if idPtr != nil && strings.TrimSpace(*idPtr) != "" {
		found, err := s.templateRepo.FindByID(ctx, tenantID, productID, *idPtr)
		if err != nil {
			return nil, err
		}
		if found.IsAbsent() {
			return nil, ErrEmailTemplateNotFound
		}
		return found.ToPtr(), nil
	}
	if slugPtr != nil && strings.TrimSpace(*slugPtr) != "" {
		found, err := s.templateRepo.FindBySlug(ctx, tenantID, productID, *slugPtr)
		if err != nil {
			return nil, err
		}
		if found.IsAbsent() {
			return nil, ErrEmailTemplateNotFound
		}
		return found.ToPtr(), nil
	}
	return nil, ErrEmailTemplateSelectorMissing
}

// resolveSendVersion is the version-resolution rule for Send.
func (s *emailService) resolveSendVersion(
	ctx context.Context, templateID string, useDraft bool,
) (*email.TemplateVersion, error) {
	if useDraft {
		found, err := s.versionRepo.FindCurrentDraft(ctx, templateID)
		if err != nil {
			return nil, err
		}
		if found.IsAbsent() {
			return nil, ErrEmailTemplateNoDraft
		}
		return found.ToPtr(), nil
	}
	found, err := s.versionRepo.FindCurrentPublished(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if found.IsAbsent() {
		return nil, ErrEmailTemplateNotPublished
	}
	return found.ToPtr(), nil
}

func (s *emailService) CreateTemplate(
	ctx context.Context, in email.CreateTemplateInput,
) (email.Template, error) {
	if err := validateStruct(in); err != nil {
		return email.Template{}, err
	}

	existing, err := s.templateRepo.FindBySlug(ctx, in.TenantID, in.ProductID, in.Slug)
	if err != nil {
		return email.Template{}, err
	}
	if existing.IsPresent() {
		return email.Template{}, ErrEmailTemplateSlugTaken
	}

	tpl := email.Template{
		PlatformTenantID: in.TenantID,
		ProductID:        in.ProductID,
		Slug:             in.Slug,
		Name:             in.Name,
		Description:      in.Description,
		IsActive:         true,
	}
	tpl.GenerateID()
	created, err := s.templateRepo.Create(ctx, tpl)
	if err != nil {
		return email.Template{}, err
	}

	ver := email.TemplateVersion{
		TemplateID:    created.ID,
		VersionNumber: 1,
		Subject:       in.Subject,
		BodyHTML:      in.BodyHTML,
		BodyText:      in.BodyText,
		Variables:     in.Variables,
		Status:        email.TemplateVersionStatusDraft,
	}
	ver.GenerateID()
	draft, err := s.versionRepo.Create(ctx, ver)
	if err != nil {
		return email.Template{}, err
	}

	if err = s.templateRepo.SetVersionPointers(
		ctx, in.TenantID, created.ID, &draft.ID, nil,
	); err != nil {
		return email.Template{}, err
	}
	created.DraftVersionID = &draft.ID
	return created, nil
}

func (s *emailService) UpdateTemplate(
	ctx context.Context, in email.UpdateTemplateInput,
) (email.Template, error) {
	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	if err != nil {
		return email.Template{}, err
	}
	if found.IsAbsent() {
		return email.Template{}, ErrEmailTemplateNotFound
	}
	existing := found.Value()
	if in.Name != nil {
		existing.Name = *in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	return s.templateRepo.Update(ctx, in.TenantID, existing)
}

func (s *emailService) UpdateTemplateDraft(
	ctx context.Context, in email.UpdateTemplateDraftInput,
) (email.TemplateVersion, error) {
	foundTpl, findErr := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if findErr != nil {
		return email.TemplateVersion{}, findErr
	}
	if foundTpl.IsAbsent() {
		return email.TemplateVersion{}, ErrEmailTemplateNotFound
	}
	tpl := foundTpl.ToPtr()

	var draft *email.TemplateVersion
	if tpl.DraftVersionID != nil {
		foundDraft, err := s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
		if err != nil {
			return email.TemplateVersion{}, err
		}
		// The template envelope points at a draft version id the caller never
		// named. If that row is not there, the envelope and the version table
		// disagree — a server invariant, not a "no draft" state the caller
		// could reach by never publishing one.
		if foundDraft.IsAbsent() {
			return email.TemplateVersion{}, fmt.Errorf(
				"update draft: draft version %s missing for template %s", *tpl.DraftVersionID, tpl.ID,
			)
		}
		draft = foundDraft.ToPtr()
	}
	//nolint:nestif // template state machine with three branches
	if draft == nil {
		if tpl.PublishedVersionID == nil {
			return email.TemplateVersion{}, ErrEmailTemplateNoDraft
		}
		foundPub, err := s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
		if err != nil {
			return email.TemplateVersion{}, err
		}
		if foundPub.IsAbsent() {
			return email.TemplateVersion{}, fmt.Errorf(
				"update draft: published version %s missing for template %s", *tpl.PublishedVersionID, tpl.ID,
			)
		}
		pub := foundPub.ToPtr()
		next, err := s.versionRepo.NextVersionNumber(ctx, tpl.ID)
		if err != nil {
			return email.TemplateVersion{}, err
		}
		newDraft := email.TemplateVersion{
			TemplateID:    tpl.ID,
			VersionNumber: next,
			Subject:       pub.Subject,
			BodyHTML:      pub.BodyHTML,
			BodyText:      pub.BodyText,
			Variables:     pub.Variables,
			Status:        email.TemplateVersionStatusDraft,
		}
		newDraft.GenerateID()
		var createdDraft email.TemplateVersion
		createdDraft, err = s.versionRepo.Create(ctx, newDraft)
		if err != nil {
			return email.TemplateVersion{}, err
		}
		if err = s.templateRepo.SetVersionPointers(
			ctx, in.TenantID, tpl.ID, &createdDraft.ID, tpl.PublishedVersionID,
		); err != nil {
			return email.TemplateVersion{}, err
		}
		draft = &createdDraft
	}

	if in.Subject != nil {
		draft.Subject = *in.Subject
	}
	if in.BodyHTML != nil {
		draft.BodyHTML = *in.BodyHTML
	}
	if in.BodyText != nil {
		draft.BodyText = *in.BodyText
	}
	if in.Variables != nil {
		draft.Variables = *in.Variables
	}
	return s.versionRepo.Update(ctx, *draft)
}

// GetTemplateDraft returns the current DRAFT version of a template. If no
// draft exists but a published version is present, a new draft is created
// from the published content and the template envelope is updated accordingly.
// Returns nil when the template has neither a draft nor a published version.
func (s *emailService) GetTemplateDraft(
	ctx context.Context, in email.GetTemplateDraftInput,
) (*email.TemplateVersion, error) {
	foundTpl, findErr := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if findErr != nil {
		return nil, findErr
	}
	if foundTpl.IsAbsent() {
		return nil, ErrEmailTemplateNotFound
	}
	tpl := foundTpl.ToPtr()

	if tpl.DraftVersionID != nil {
		foundDraft, err := s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
		if err != nil {
			return nil, err
		}
		if foundDraft.IsAbsent() {
			return nil, fmt.Errorf(
				"get draft: draft version %s missing for template %s", *tpl.DraftVersionID, tpl.ID,
			)
		}
		return foundDraft.ToPtr(), nil
	}

	if tpl.PublishedVersionID == nil {
		return nil, nil
	}

	foundPub, err := s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
	if err != nil {
		return nil, err
	}
	if foundPub.IsAbsent() {
		return nil, fmt.Errorf(
			"get draft: published version %s missing for template %s", *tpl.PublishedVersionID, tpl.ID,
		)
	}
	pub := foundPub.ToPtr()

	next, err := s.versionRepo.NextVersionNumber(ctx, tpl.ID)
	if err != nil {
		return nil, err
	}
	newDraft := email.TemplateVersion{
		TemplateID:    tpl.ID,
		VersionNumber: next,
		Subject:       pub.Subject,
		BodyHTML:      pub.BodyHTML,
		BodyText:      pub.BodyText,
		Variables:     pub.Variables,
		Status:        email.TemplateVersionStatusDraft,
	}
	newDraft.GenerateID()
	created, err := s.versionRepo.Create(ctx, newDraft)
	if err != nil {
		return nil, err
	}
	if err = s.templateRepo.SetVersionPointers(
		ctx, in.TenantID, tpl.ID, &created.ID, tpl.PublishedVersionID,
	); err != nil {
		return nil, err
	}
	return &created, nil
}

// PublishTemplate atomically swaps the template's draft into the published
// slot. Wrapped in a single DB transaction so a partial failure cannot leave
// the envelope pointing at conflicting versions.
func (s *emailService) PublishTemplate(
	ctx context.Context, in email.PublishTemplateInput,
) (email.TemplateVersion, error) {
	foundTpl, findErr := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if findErr != nil {
		return email.TemplateVersion{}, findErr
	}
	if foundTpl.IsAbsent() {
		return email.TemplateVersion{}, ErrEmailTemplateNotFound
	}
	tpl := foundTpl.ToPtr()
	if tpl.DraftVersionID == nil {
		return email.TemplateVersion{}, ErrEmailTemplateNoDraft
	}

	publishedID := *tpl.DraftVersionID
	publishedAt := time.Now()
	prevPublishedID := tpl.PublishedVersionID

	if err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if prevPublishedID != nil {
			if err := s.versionRepo.UpdateStatus(
				txCtx, *prevPublishedID, email.TemplateVersionStatusArchived, nil,
			); err != nil {
				return err
			}
		}
		if err := s.versionRepo.UpdateStatus(
			txCtx, publishedID, email.TemplateVersionStatusPublished, &publishedAt,
		); err != nil {
			return err
		}
		return s.templateRepo.SetVersionPointers(
			txCtx, in.TenantID, tpl.ID, nil, &publishedID,
		)
	}); err != nil {
		return email.TemplateVersion{}, err
	}

	foundPublished, err := s.versionRepo.FindByID(ctx, publishedID)
	if err != nil {
		return email.TemplateVersion{}, err
	}
	// The transaction above just committed this row. The caller never named
	// publishedID, so its absence here is a server invariant break, not a
	// 404 the caller could act on — a bare wrapped error becomes the
	// generic 500 the boundary handler logs, instead of a misleading
	// EMAIL_TEMPLATE_NOT_FOUND on data that was just written successfully.
	if foundPublished.IsAbsent() {
		return email.TemplateVersion{}, fmt.Errorf(
			"publish: version %s missing after commit", publishedID,
		)
	}
	published := foundPublished.Value()

	// Open a fresh DRAFT cloned from the just-published row so further edits
	// do not mutate the live version. Best-effort post-tx; failure here does
	// not roll back the publish.
	next, err := s.versionRepo.NextVersionNumber(ctx, tpl.ID)
	if err != nil {
		s.logger.Warn().Err(err).Str("template_id", tpl.ID).Msg("publish: next version number")
		return published, nil
	}
	newDraft := email.TemplateVersion{
		TemplateID:    tpl.ID,
		VersionNumber: next,
		Subject:       published.Subject,
		BodyHTML:      published.BodyHTML,
		BodyText:      published.BodyText,
		Variables:     published.Variables,
		Status:        email.TemplateVersionStatusDraft,
	}
	newDraft.GenerateID()
	createdDraft, err := s.versionRepo.Create(ctx, newDraft)
	if err != nil {
		s.logger.Warn().Err(err).Str("template_id", tpl.ID).Msg("publish: clone fresh draft")
		return published, nil
	}
	if err = s.templateRepo.SetVersionPointers(
		ctx, in.TenantID, tpl.ID, &createdDraft.ID, &publishedID,
	); err != nil {
		s.logger.Warn().Err(err).Str("template_id", tpl.ID).Msg("publish: re-point draft")
	}
	return published, nil
}

func (s *emailService) GetTemplate(
	ctx context.Context, in email.GetTemplateInput,
) (*email.Template, error) {
	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	if err != nil {
		return nil, err
	}
	return found.ToPtr(), nil
}

func (s *emailService) ListTemplates(
	ctx context.Context, in email.ListTemplatesInput,
) ([]email.Template, error) {
	return s.templateRepo.List(ctx, in.TenantID, in.ProductID, in.Limit, in.Offset)
}

func (s *emailService) DeleteTemplate(
	ctx context.Context, in email.DeleteTemplateInput,
) error {
	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return ErrEmailTemplateNotFound
	}

	return s.templateRepo.DeleteByID(ctx, in.TenantID, in.ProductID, in.ID)
}

func (s *emailService) SaveTemplateExamples(
	ctx context.Context, in email.SaveTemplateExamplesInput,
) ([]email.TemplateExample, error) {
	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if found.IsAbsent() {
		return nil, ErrEmailTemplateNotFound
	}
	if err = s.templateRepo.SaveExamples(ctx, in.TenantID, in.ProductID, in.TemplateID, in.Examples); err != nil {
		return nil, err
	}
	return in.Examples, nil
}

func (s *emailService) Preview(
	ctx context.Context, in email.PreviewInput,
) (email.PreviewResult, error) {
	foundTpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return email.PreviewResult{}, err
	}
	if foundTpl.IsAbsent() {
		return email.PreviewResult{}, ErrEmailTemplateNotFound
	}
	tpl := foundTpl.ToPtr()
	var foundVersion functional.Option[email.TemplateVersion]
	if in.UsePublished {
		if tpl.PublishedVersionID == nil {
			return email.PreviewResult{}, ErrEmailTemplateNotPublished
		}
		foundVersion, err = s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
	} else {
		if tpl.DraftVersionID == nil {
			return email.PreviewResult{}, ErrEmailTemplateNoDraft
		}
		foundVersion, err = s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
	}
	if err != nil {
		return email.PreviewResult{}, err
	}
	if foundVersion.IsAbsent() {
		return email.PreviewResult{}, fmt.Errorf(
			"preview: template %s version missing", tpl.ID,
		)
	}
	v := foundVersion.ToPtr()
	res, err := s.renderer.Render(v, in.Variables)
	if err != nil {
		return email.PreviewResult{}, fault.BadRequest(
			"EMAIL_TEMPLATE_RENDER_ERROR",
			err.Error(),
		)
	}
	return email.PreviewResult{
		Subject:  res.Subject,
		BodyHTML: res.BodyHTML,
		BodyText: res.BodyText,
		Warnings: res.Warnings,
	}, nil
}

// Send dispatches a transactional email through the product's configured
// integration. Honours dedupe key and rate limits.
//
// Order matters: we look up dedupe before doing any other work so a repeated
// submission is fast and cheap; we persist the SendRecord before dialing the
// SMTP server so a connection failure leaves a FAILED audit row.
func (s *emailService) Send(
	ctx context.Context, in email.SendInput,
) (email.SendRecord, error) {
	// retryRecord is a prior send for this dedupe key that FAILED — we re-dispatch
	// it (reusing the row) instead of returning the stale failure. Any other
	// status (queued/sending/sent) is a genuine duplicate and short-circuits.
	var retryRecord *email.SendRecord
	if in.DedupeKey != nil && strings.TrimSpace(*in.DedupeKey) != "" {
		found, err := s.sendRepo.FindByDedupeKey(ctx, in.TenantID, in.ProductID, *in.DedupeKey)
		if err != nil {
			return email.SendRecord{}, err
		}
		if found.IsPresent() {
			existing := found.ToPtr()
			if existing.Status != email.SendStatusFailed {
				return *existing, nil
			}
			retryRecord = existing
		}
	}

	mailer, instance, err := s.resolveMailer(ctx, in.TenantID, in.ProductID)
	if err != nil {
		return email.SendRecord{}, err
	}

	tpl, err := s.resolveTemplate(ctx, in.TenantID, in.ProductID, in.TemplateID, in.TemplateSlug)
	if err != nil {
		return email.SendRecord{}, err
	}
	version, err := s.resolveSendVersion(ctx, tpl.ID, in.UseDraft)
	if err != nil {
		return email.SendRecord{}, err
	}

	if missing := email.MissingRequiredVariables(version.Variables, in.Variables); len(missing) > 0 {
		return email.SendRecord{}, errEmailRequiredVariablesMissing(missing)
	}

	sender, err := mailer.SenderIdentity(ctx, instance)
	if err != nil {
		// SenderIdentity decrypts and validates the integration's stored config
		// before we touch the relay, so its failure modes (an undecryptable
		// credential, a missing encryption key) are the same class of integration
		// failure as a dispatch error. Classify it the same way: otherwise it
		// reaches the strict boundary unmodelled and is logged as the generic
		// "unhandled error" 500 safety net instead of a stable, intentional
		// EMAIL_DELIVERY_FAILED. No SendRecord exists yet, so there is nothing to
		// mark FAILED here.
		return email.SendRecord{}, classifyDispatchError(err)
	}

	rendered, err := s.renderer.Render(version, in.Variables)
	if err != nil {
		return email.SendRecord{}, err
	}

	varsJSON := json.RawMessage("{}")
	if len(in.Variables) > 0 {
		var raw []byte
		raw, err = json.Marshal(in.Variables)
		if err != nil {
			return email.SendRecord{}, err
		}
		varsJSON = raw
	}

	for _, w := range s.rateLimits {
		since := time.Now().Add(-w.WindowDuration)
		var count int64
		count, err = s.sendRepo.CountSince(ctx, in.TenantID, in.ProductID, since)
		if err != nil {
			return email.SendRecord{}, err
		}
		if count >= w.MaxSends {
			return email.SendRecord{}, ErrEmailRateLimitExceeded
		}
	}

	var persisted email.SendRecord
	var messageID string
	if retryRecord != nil {
		// Re-dispatch a previously FAILED send, reusing its row (the dedupe unique
		// index forbids a second row for the same key).
		persisted = *retryRecord
		messageID = persisted.MessageID
	} else {
		rec := email.SendRecord{
			PlatformTenantID:      in.TenantID,
			ProductID:             in.ProductID,
			IntegrationInstanceID: instance.ID,
			TemplateID:            &tpl.ID,
			TemplateVersionID:     &version.ID,
			DedupeKey:             in.DedupeKey,
			ToAddress:             in.ToAddress,
			ToName:                in.ToName,
			FromAddress:           sender.Email,
			FromName:              sender.Name,
			Subject:               rendered.Subject,
			BodyHTML:              rendered.BodyHTML,
			BodyText:              rendered.BodyText,
			VariablesJSON:         varsJSON,
			Status:                email.SendStatusQueued,
		}
		rec.GenerateID()
		messageID = ids.MustNew("emid")
		rec.MessageID = messageID

		var createdNew bool
		persisted, createdNew, err = s.createSendRecord(ctx, in, rec)
		if err != nil {
			return email.SendRecord{}, err
		}
		if !createdNew {
			return persisted, nil
		}
	}

	msg := provider.OutboundMessage{
		From:      sender,
		To:        []provider.Address{{Email: in.ToAddress, Name: in.ToName}},
		Subject:   rendered.Subject,
		HTML:      rendered.BodyHTML,
		Text:      rendered.BodyText,
		MessageID: messageID,
	}

	dispatchErr := mailer.Send(ctx, instance, msg)

	// mailer.Send has already had its outbound side effect — the message was
	// handed to the provider or definitively rejected — so the terminal status
	// write must survive cancellation of the caller's request context, or a
	// delivered email can stay recorded as unsent. Detaching drops only
	// cancellation while keeping request-scoped values (trace correlation); the
	// timeout keeps the detached write bounded.
	persistCtx, cancelPersist := context.WithTimeout(context.WithoutCancel(ctx), emailStatusPersistTimeout)
	defer cancelPersist()

	if dispatchErr != nil {
		errMsg := dispatchErr.Error()
		if updErr := s.sendRepo.UpdateStatus(
			persistCtx, in.TenantID, persisted.ID, email.SendStatusFailed, &errMsg, nil,
		); updErr != nil {
			log.Event(&s.logger, updErr).Str("send_id", persisted.ID).Msg("update FAILED status")
		}
		persisted.Status = email.SendStatusFailed
		persisted.LastError = &errMsg
		return persisted, classifyDispatchError(dispatchErr)
	}

	now := time.Now()
	if err = s.sendRepo.UpdateStatus(
		persistCtx, in.TenantID, persisted.ID, email.SendStatusSent, nil, &now,
	); err != nil {
		log.Event(&s.logger, err).Str("send_id", persisted.ID).Msg("update SENT status")
	}
	persisted.Status = email.SendStatusSent
	persisted.SentAt = &now
	return persisted, nil
}

// isEmailSendDedupeConflict reports whether err is the unique violation raised
// when a concurrent send races on the same dedupe key.
func isEmailSendDedupeConflict(err error) bool {
	return pgerr.IsUniqueViolation(err, emailSendDedupeConstraint)
}

func (s *emailService) createSendRecord(
	ctx context.Context,
	in email.SendInput,
	rec email.SendRecord,
) (email.SendRecord, bool, error) {
	persisted, err := s.sendRepo.Create(ctx, in.TenantID, in.ProductID, rec)
	if err == nil {
		return persisted, true, nil
	}
	if in.DedupeKey == nil || strings.TrimSpace(*in.DedupeKey) == "" || !isEmailSendDedupeConflict(err) {
		return email.SendRecord{}, false, err
	}

	found, findErr := s.sendRepo.FindByDedupeKey(ctx, in.TenantID, in.ProductID, *in.DedupeKey)
	if findErr != nil {
		return email.SendRecord{}, false, findErr
	}
	if found.IsPresent() {
		return found.Value(), false, nil
	}

	return email.SendRecord{}, false, err
}

func (s *emailService) TestSend(
	ctx context.Context, in email.TestSendInput,
) (email.SendRecord, error) {
	tplID := in.TemplateID
	return s.Send(ctx, email.SendInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: &tplID,
		ToAddress:  in.ToAddress,
		Variables:  in.Variables,
		UseDraft:   true,
		IsTestSend: true,
	})
}

func (s *emailService) ListSends(
	ctx context.Context, in email.ListSendsInput,
) ([]email.SendRecord, error) {
	return s.sendRepo.List(ctx, in)
}
