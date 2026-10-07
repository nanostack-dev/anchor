package workflow

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
	"go.uber.org/fx"

	emailsvc "anchor/internal/email/service"
	"anchor/internal/events"
	invitationsvc "anchor/internal/invitation/service"
	licensesvc "anchor/internal/license/service"
	anchorservice "anchor/internal/service"
	serviceconfig "anchor/internal/service/config"
	"anchor/internal/workflow/engine"
	"anchor/internal/workflow/repository"
	"anchor/internal/workflow/service"
)

// NewModule wires product workflows: the definitions a Product saves, the
// engine that runs their steps against its resources, and the worker that
// starts a run for every matching product event.
func NewModule() fx.Option {
	return fx.Module(
		"product_workflow",
		fx.Provide(
			repository.NewRepository,
			newEngine,
			service.NewRunner,
			service.NewWorkflowService,
			events.AsListener(service.Listener),
		),
		fx.Invoke(service.RegisterWorker),
	)
}

type engineParams struct {
	fx.In
	Catalog       events.Catalog
	Organizations anchorservice.OrganizationService
	Workspaces    anchorservice.WorkspaceService
	Memberships   anchorservice.OrganizationMembershipService
	ProductUsers  anchorservice.ProductUserService
	Invitations   invitationsvc.OrganizationInvitationService
	Licenses      licensesvc.OrganizationLicenseService
	Migrations    licensesvc.LicenseMigrationService
	Email         emailsvc.EmailService
	Endpoints     events.EndpointService
	Queue         *queue.Client
	Transactor    transactor.Transactor
	Core          *serviceconfig.CoreConfig
}

func newEngine(p engineParams) *engine.Engine {
	return engine.New(engine.Services{
		Organizations: p.Organizations,
		Workspaces:    p.Workspaces,
		Memberships:   p.Memberships,
		ProductUsers:  p.ProductUsers,
		Invitations:   p.Invitations,
		Licenses:      p.Licenses,
		Migrations:    p.Migrations,
		Email:         p.Email,
		CustomEvents:  engine.NewCustomEventSender(p.Queue, p.Transactor),
		Caller:        engine.NewHTTPCaller(p.Endpoints, p.Core.IsProduction()),
	}, p.Catalog)
}
