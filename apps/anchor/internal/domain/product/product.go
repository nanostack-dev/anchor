package product

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Product struct {
	ID               string
	PlatformTenantID string
	Name             string
	Description      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GenerateID sets the product's ID to a new prefixed KSUID.
func (p *Product) GenerateID() {
	p.ID = ids.MustNew("prd")
}
