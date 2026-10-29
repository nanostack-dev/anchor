package mirror

import "go.uber.org/fx"

// AsProjection registers a projection's constructor with the mirror.
func AsProjection(constructor any) any {
	return fx.Annotate(constructor, fx.As(new(Projection)), fx.ResultTags(`group:"authz_projections"`))
}

// Params is what fx hands New: every projection registered with AsProjection.
type Params struct {
	fx.In
	Engines     Engines
	Stores      Stores
	Repository  Repository
	Locker      Locker
	Models      ModelSource
	Projections []Projection `group:"authz_projections"`
}

// MarkerParams is what fx hands NewMarker.
type MarkerParams struct {
	fx.In
	Projections []Projection `group:"authz_projections"`
}
