package email

import (
	"anchor/internal/email/renderer"
	"anchor/internal/email/repository"
	"anchor/internal/email/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module wires the email subsystem (templates, send dispatch).
var Module = nano.Module("email", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		renderer.New,
		repository.NewTemplateRepository,
		repository.NewTemplateVersionRepository,
		repository.NewSendRecordRepository,
		service.NewEmailService,
	),
)
