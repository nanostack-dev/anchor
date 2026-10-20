package access

import (
	"anchor/internal/access/service"

	"go.uber.org/fx"
)

// NewModule wires access evaluation: the per-request decision a product asks
// Anchor for instead of rebuilding memberships, roles and workspaces itself.
func NewModule() fx.Option {
	return fx.Module(
		"access",
		fx.Provide(service.NewEvaluator),
	)
}
