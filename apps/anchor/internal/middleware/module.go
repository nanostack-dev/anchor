package middleware

import (
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"middleware",
		fx.Provide(
			NewAuthMiddleware,
			// Republishes request_id/method/path onto any logger rebuilt by
			// log.Bind, and the authenticated caller alongside it.
			fx.Annotate(requestlog.NewLogEnricher, fx.ResultTags(logging.EnricherGroup)),
			fx.Annotate(security.NewLogEnricher, fx.ResultTags(logging.EnricherGroup)),
		),
	)
}
