package provider

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"go.uber.org/fx"
)

// Registry holds all registered integration providers and allows lookup by type.
type Registry struct {
	providers map[string]Provider
}

// RegistryParams is used by Uber FX to inject all providers via group tagging.
type RegistryParams struct {
	fx.In
	Providers []Provider `group:"integration_providers"`
}

// NewRegistry creates a provider registry from all FX-injected providers.
func NewRegistry(p RegistryParams) *Registry {
	return &Registry{
		providers: functional.Slice(p.Providers).ToMap(func(prov Provider) string {
			return prov.Type()
		}),
	}
}

// GetProvider returns the provider for the given type, or an error if not registered.
func (r *Registry) GetProvider(providerType string) (Provider, error) {
	p, ok := r.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider type %q not registered", providerType)
	}
	return p, nil
}

// ProviderTypes returns all registered provider type names.
func (r *Registry) ProviderTypes() []string {
	types := make([]string, 0, len(r.providers))
	for t := range r.providers {
		types = append(types, t)
	}
	return types
}

// AsProviderResult wraps a constructor to tag its result for FX group injection.
func AsProviderResult(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(Provider)),
		fx.ResultTags(`group:"integration_providers"`),
	)
}
