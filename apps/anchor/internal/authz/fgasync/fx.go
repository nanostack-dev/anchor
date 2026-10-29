package fgasync

import "go.uber.org/fx"

// AsSource registers a source's constructor with the fgasync.
func AsSource(constructor any) any {
	return fx.Annotate(constructor, fx.As(new(Source)), fx.ResultTags(`group:"authz_sources"`))
}

// Params is what fx hands New: every source registered with AsSource.
type Params struct {
	fx.In
	Engines    Engines
	Stores     Stores
	Repository Repository
	Locker     Locker
	Models     ModelSource
	Sources    []Source `group:"authz_sources"`
}

// MarkerParams is what fx hands NewMarker.
type MarkerParams struct {
	fx.In
	Sources []Source `group:"authz_sources"`
}
