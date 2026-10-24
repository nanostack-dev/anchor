package workspacemember

import (
	"go.uber.org/fx"

	"anchor/internal/workspacemember/repository"
	"anchor/internal/workspacemember/service"
)

// NewModule wires the workspace membership subsystem: the roles organization
// members hold in a workspace.
func NewModule() fx.Option {
	return fx.Module(
		"workspace_member",
		fx.Provide(
			repository.NewRepository,
			service.NewWorkspaceMemberService,
		),
	)
}
