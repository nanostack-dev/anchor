package email

import (
	"anchor/internal/email/renderer"
	"anchor/internal/email/repository"
	"anchor/internal/email/service"

	"go.uber.org/fx"
)

// NewModule wires the email subsystem (templates, send dispatch).
func NewModule() fx.Option {
	return fx.Module(
		"email",
		fx.Provide(
			renderer.New,
			repository.NewTemplateRepository,
			repository.NewTemplateVersionRepository,
			repository.NewSendRecordRepository,
			service.NewEmailService,
		),
	)
}
