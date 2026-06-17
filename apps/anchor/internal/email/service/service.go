package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"anchor/internal/domain/email"
	domainintegration "anchor/internal/domain/integration"
	"anchor/internal/email/renderer"
	emailrepo "anchor/internal/email/repository"
	"anchor/internal/integration/provider"
	intrepo "anchor/internal/repository"

	"github.com/lib/pq"
	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/rs/zerolog"
)

const emailSendDedupeConstraint = "idx_email_send_records_dedupe"

var (
	ErrEmailIntegrationNotFound = apierror.NewWithStatus(
		"EMAIL_INTEGRATION_NOT_FOUND",
		"No email integration is configured for this product",
		http.StatusFailedDependency,
	)
	ErrEmailIntegrationInactive = apierror.NewBadRequest(
		"EMAIL_INTEGRATION_INACTIVE",
		"The email integration is not in an ACTIVE state",
	)
	ErrEmailTemplateNotFound = apierror.NewWithStatus(
		"EMAIL_TEMPLATE_NOT_FOUND",
		"Email template not found",
		http.StatusNotFound,
	)
	ErrEmailTemplateNotPublished = apierror.NewBadRequest(
		"EMAIL_TEMPLATE_NOT_PUBLISHED",
		"This template has no published version; publish it before sending",
	)
	ErrEmailTemplateNoDraft = apierror.NewBadRequest(
		"EMAIL_TEMPLATE_NO_DRAFT",
		"This template has no draft version available",
	)
	ErrEmailTemplateSlugTaken = apierror.NewBadRequest(
		"EMAIL_TEMPLATE_SLUG_TAKEN",
		"An email template with this slug already exists for the product",
	)
	ErrEmailRateLimitExceeded = apierror.NewWithStatus(
		"EMAIL_RATE_LIMIT_EXCEEDED",
		"Sends for this product exceeded the configured rate limit",
		http.StatusTooManyRequests,
	)
	ErrEmailMailerCapabilityMissing = apierror.NewBadRequest(
		"EMAIL_MAILER_CAPABILITY_MISSING",
		"The configured integration does not support outbound email",
	)
	ErrEmailTemplateSelectorMissing = apierror.NewBadRequest(
		"EMAIL_TEMPLATE_SELECTOR_MISSING",
		"Either template_id or template_slug must be supplied",
	)
	ErrEmailContentMissing = apierror.NewBadRequest(
		"EMAIL_CONTENT_MISSING",
		"A raw send requires subject and at least one of body_html or body_text",
	)
)

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
	instance, err := s.instanceRepo.FindByProductAndProvider(
		ctx, tenantID, productID, string(domainintegration.ProviderTypeSMTP),
	)
	if err != nil {
		return nil, nil, err
	}
	if instance == nil {
		return nil, nil, ErrEmailIntegrationNotFound
	}
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
		t, err := s.templateRepo.FindByID(ctx, tenantID, productID, *idPtr)
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, ErrEmailTemplateNotFound
		}
		return t, nil
	}
	if slugPtr != nil && strings.TrimSpace(*slugPtr) != "" {
		t, err := s.templateRepo.FindBySlug(ctx, tenantID, productID, *slugPtr)
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, ErrEmailTemplateNotFound
		}
		return t, nil
	}
	return nil, ErrEmailTemplateSelectorMissing
}

// resolveSendVersion is the version-resolution rule for Send.
func (s *emailService) resolveSendVersion(
	ctx context.Context, templateID string, useDraft bool,
) (*email.TemplateVersion, error) {
	if useDraft {
		v, err := s.versionRepo.FindCurrentDraft(ctx, templateID)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, ErrEmailTemplateNoDraft
		}
		return v, nil
	}
	v, err := s.versionRepo.FindCurrentPublished(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrEmailTemplateNotPublished
	}
	return v, nil
}

// sendContent is the resolved subject/body to dispatch plus, for template
// sends, the template/version IDs recorded on the audit row. For raw sends the
// ID pointers are nil.
type sendContent struct {
	subject    string
	bodyHTML   string
	bodyText   string
	templateID *string
	versionID  *string
}

// resolveSendContent produces the message content for a send. With a template
// selector it resolves the version and renders with variables; otherwise it
// uses the raw subject/body verbatim, requiring a subject and at least one body.
func (s *emailService) resolveSendContent(
	ctx context.Context, in email.SendInput,
) (sendContent, error) {
	if !in.HasTemplateSelector() {
		if strings.TrimSpace(in.Subject) == "" ||
			(strings.TrimSpace(in.BodyHTML) == "" && strings.TrimSpace(in.BodyText) == "") {
			return sendContent{}, ErrEmailContentMissing
		}
		return sendContent{
			subject:  in.Subject,
			bodyHTML: in.BodyHTML,
			bodyText: in.BodyText,
		}, nil
	}

	tpl, err := s.resolveTemplate(ctx, in.TenantID, in.ProductID, in.TemplateID, in.TemplateSlug)
	if err != nil {
		return sendContent{}, err
	}
	version, err := s.resolveSendVersion(ctx, tpl.ID, in.UseDraft)
	if err != nil {
		return sendContent{}, err
	}
	rendered, err := s.renderer.Render(version, in.Variables)
	if err != nil {
		return sendContent{}, err
	}
	return sendContent{
		subject:    rendered.Subject,
		bodyHTML:   rendered.BodyHTML,
		bodyText:   rendered.BodyText,
		templateID: &tpl.ID,
		versionID:  &version.ID,
	}, nil
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
	if existing != nil {
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
	existing, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	if err != nil {
		return email.Template{}, err
	}
	if existing == nil {
		return email.Template{}, ErrEmailTemplateNotFound
	}
	if in.Name != nil {
		existing.Name = *in.Name
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	return s.templateRepo.Update(ctx, in.TenantID, *existing)
}

func (s *emailService) UpdateTemplateDraft(
	ctx context.Context, in email.UpdateTemplateDraftInput,
) (email.TemplateVersion, error) {
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return email.TemplateVersion{}, err
	}
	if tpl == nil {
		return email.TemplateVersion{}, ErrEmailTemplateNotFound
	}

	var draft *email.TemplateVersion
	if tpl.DraftVersionID != nil {
		draft, err = s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
		if err != nil {
			return email.TemplateVersion{}, err
		}
	}
	//nolint:nestif // template state machine with three branches
	if draft == nil {
		if tpl.PublishedVersionID == nil {
			return email.TemplateVersion{}, ErrEmailTemplateNoDraft
		}
		var pub *email.TemplateVersion
		pub, err = s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
		if err != nil {
			return email.TemplateVersion{}, err
		}
		if pub == nil {
			return email.TemplateVersion{}, ErrEmailTemplateNoDraft
		}
		var next int32
		next, err = s.versionRepo.NextVersionNumber(ctx, tpl.ID)
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
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, ErrEmailTemplateNotFound
	}

	if tpl.DraftVersionID != nil {
		var draft *email.TemplateVersion
		draft, err = s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
		if err != nil {
			return nil, err
		}
		return draft, nil
	}

	if tpl.PublishedVersionID == nil {
		return nil, nil
	}

	pub, err := s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, nil
	}

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
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return email.TemplateVersion{}, err
	}
	if tpl == nil {
		return email.TemplateVersion{}, ErrEmailTemplateNotFound
	}
	if tpl.DraftVersionID == nil {
		return email.TemplateVersion{}, ErrEmailTemplateNoDraft
	}

	publishedID := *tpl.DraftVersionID
	publishedAt := time.Now()
	prevPublishedID := tpl.PublishedVersionID

	if err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if prevPublishedID != nil {
			if err = s.versionRepo.UpdateStatus(
				txCtx, *prevPublishedID, email.TemplateVersionStatusArchived, nil,
			); err != nil {
				return err
			}
		}
		if err = s.versionRepo.UpdateStatus(
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

	published, err := s.versionRepo.FindByID(ctx, publishedID)
	if err != nil || published == nil {
		return email.TemplateVersion{}, err
	}

	// Open a fresh DRAFT cloned from the just-published row so further edits
	// do not mutate the live version. Best-effort post-tx; failure here does
	// not roll back the publish.
	next, err := s.versionRepo.NextVersionNumber(ctx, tpl.ID)
	if err != nil {
		s.logger.Warn().Err(err).Str("template_id", tpl.ID).Msg("publish: next version number")
		return *published, nil //nolint:nilerr // best-effort post-tx; does not roll back publish
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
		return *published, nil //nolint:nilerr // best-effort post-tx; does not roll back publish
	}
	if err = s.templateRepo.SetVersionPointers(
		ctx, in.TenantID, tpl.ID, &createdDraft.ID, &publishedID,
	); err != nil {
		s.logger.Warn().Err(err).Str("template_id", tpl.ID).Msg("publish: re-point draft")
	}
	return *published, nil
}

func (s *emailService) GetTemplate(
	ctx context.Context, in email.GetTemplateInput,
) (*email.Template, error) {
	return s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
}

func (s *emailService) ListTemplates(
	ctx context.Context, in email.ListTemplatesInput,
) ([]email.Template, error) {
	return s.templateRepo.List(ctx, in.TenantID, in.ProductID, in.Limit, in.Offset)
}

func (s *emailService) DeleteTemplate(
	ctx context.Context, in email.DeleteTemplateInput,
) error {
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	if err != nil {
		return err
	}
	if tpl == nil {
		return ErrEmailTemplateNotFound
	}

	return s.templateRepo.DeleteByID(ctx, in.TenantID, in.ProductID, in.ID)
}

func (s *emailService) SaveTemplateExamples(
	ctx context.Context, in email.SaveTemplateExamplesInput,
) ([]email.TemplateExample, error) {
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
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
	tpl, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return email.PreviewResult{}, err
	}
	if tpl == nil {
		return email.PreviewResult{}, ErrEmailTemplateNotFound
	}
	var v *email.TemplateVersion
	if in.UsePublished {
		if tpl.PublishedVersionID == nil {
			return email.PreviewResult{}, ErrEmailTemplateNotPublished
		}
		v, err = s.versionRepo.FindByID(ctx, *tpl.PublishedVersionID)
	} else {
		if tpl.DraftVersionID == nil {
			return email.PreviewResult{}, ErrEmailTemplateNoDraft
		}
		v, err = s.versionRepo.FindByID(ctx, *tpl.DraftVersionID)
	}
	if err != nil {
		return email.PreviewResult{}, err
	}
	if v == nil {
		return email.PreviewResult{}, ErrEmailTemplateNotFound
	}
	res, err := s.renderer.Render(v, in.Variables)
	if err != nil {
		return email.PreviewResult{}, apierror.NewBadRequest(
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
	if in.DedupeKey != nil && strings.TrimSpace(*in.DedupeKey) != "" {
		existing, err := s.sendRepo.FindByDedupeKey(ctx, in.TenantID, in.ProductID, *in.DedupeKey)
		if err != nil {
			return email.SendRecord{}, err
		}
		if existing != nil {
			return *existing, nil
		}
	}

	mailer, instance, err := s.resolveMailer(ctx, in.TenantID, in.ProductID)
	if err != nil {
		return email.SendRecord{}, err
	}

	content, err := s.resolveSendContent(ctx, in)
	if err != nil {
		return email.SendRecord{}, err
	}

	sender, err := mailer.SenderIdentity(ctx, instance)
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

	rec := email.SendRecord{
		PlatformTenantID:      in.TenantID,
		ProductID:             in.ProductID,
		IntegrationInstanceID: instance.ID,
		TemplateID:            content.templateID,
		TemplateVersionID:     content.versionID,
		DedupeKey:             in.DedupeKey,
		ToAddress:             in.ToAddress,
		ToName:                in.ToName,
		FromAddress:           sender.Email,
		FromName:              sender.Name,
		Subject:               content.subject,
		BodyHTML:              content.bodyHTML,
		BodyText:              content.bodyText,
		VariablesJSON:         varsJSON,
		Status:                email.SendStatusQueued,
		Attempts:              0,
	}
	rec.GenerateID()

	messageID := ids.MustNew("emid")
	rec.MessageID = messageID

	persisted, createdNew, err := s.createSendRecord(ctx, in, rec)
	if err != nil {
		return email.SendRecord{}, err
	}
	if !createdNew {
		return persisted, nil
	}

	msg := provider.OutboundMessage{
		From:      sender,
		To:        []provider.Address{{Email: in.ToAddress, Name: in.ToName}},
		Subject:   content.subject,
		HTML:      content.bodyHTML,
		Text:      content.bodyText,
		MessageID: messageID,
	}

	if dispatchErr := mailer.Send(ctx, instance, msg); dispatchErr != nil {
		errMsg := dispatchErr.Error()
		if updErr := s.sendRepo.UpdateStatus(
			ctx, in.TenantID, persisted.ID, email.SendStatusFailed, &errMsg, nil,
		); updErr != nil {
			s.logger.Error().Err(updErr).Str("send_id", persisted.ID).Msg("update FAILED status")
		}
		persisted.Status = email.SendStatusFailed
		persisted.LastError = &errMsg
		return persisted, dispatchErr
	}

	now := time.Now()
	if err = s.sendRepo.UpdateStatus(
		ctx, in.TenantID, persisted.ID, email.SendStatusSent, nil, &now,
	); err != nil {
		s.logger.Error().Err(err).Str("send_id", persisted.ID).Msg("update SENT status")
	}
	persisted.Status = email.SendStatusSent
	persisted.SentAt = &now
	return persisted, nil
}

func isEmailSendDedupeConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == emailSendDedupeConstraint
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

	existing, findErr := s.sendRepo.FindByDedupeKey(ctx, in.TenantID, in.ProductID, *in.DedupeKey)
	if findErr != nil {
		return email.SendRecord{}, false, findErr
	}
	if existing != nil {
		return *existing, false, nil
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
