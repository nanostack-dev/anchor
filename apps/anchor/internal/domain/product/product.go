package product

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
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
	p.ID = toolkit.NewID("prd")
}
