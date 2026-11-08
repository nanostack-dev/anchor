package provider

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// Registry holds all registered integration providers and allows lookup by type.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a provider registry from every Provider that a module
// adds to the nano.Collect[Provider] collection.
func NewRegistry(providers []Provider) *Registry {
	return &Registry{
		providers: functional.Slice(providers).ToMap(func(prov Provider) string {
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
