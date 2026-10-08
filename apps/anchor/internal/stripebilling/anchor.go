package stripebilling

import (
	"context"
	"errors"

	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/product"
	licensesvc "anchor/internal/license/service"
	"anchor/internal/service"
	billing "anchor/internal/stripeprototype"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

const organizationPageSize = 100

type anchorGateway struct {
	tenantID, productID string
	products            service.ProductService
	organizations       service.OrganizationService
	templates           licensesvc.LicenseTemplateService
	licenses            licensesvc.OrganizationLicenseService
	migrations          licensesvc.LicenseMigrationService
}

func (g *anchorGateway) Snapshot(ctx context.Context) (billing.AnchorSnapshot, error) {
	p, err := g.products.Get(ctx, product.GetProductInput{TenantID: g.tenantID, ProductID: g.productID})
	if err != nil {
		return billing.AnchorSnapshot{}, err
	}
	templates, err := g.templates.ListTemplates(
		ctx,
		license.ListTemplatesInput{TenantID: g.tenantID, ProductID: g.productID},
	)
	if err != nil {
		return billing.AnchorSnapshot{}, err
	}
	snapshot := billing.AnchorSnapshot{
		Product: billing.Product{ID: p.ID, Name: p.Name},
		Templates: functional.Slice(templates).Map(func(t license.Template) billing.Template {
			return billing.Template{ID: t.ID, Name: t.Name, Values: map[string]any(t.Values), Archived: t.IsArchived()}
		}),
		Organizations: []billing.AnchorOrganization{},
	}
	// Pagination has an early exit once all tenant-scoped organizations are read.
	for offset := int32(0); ; offset += organizationPageSize {
		page, pageErr := g.organizations.Search(
			ctx,
			organization.SearchProductOrganizationsInput{
				TenantID:  g.tenantID,
				ProductID: g.productID,
				Include:   []organization.Include{organization.IncludeLicense},
				Request: search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
					Pagination: search.Pagination{Limit: organizationPageSize, Offset: offset},
					Sort: []search.Sort[organization.SortFieldProductOrganization]{
						{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
					},
				},
			},
		)
		if pageErr != nil {
			return billing.AnchorSnapshot{}, pageErr
		}
		snapshot.Organizations = append(
			snapshot.Organizations,
			functional.Slice(page.Items).Map(func(o organization.Organization) billing.AnchorOrganization {
				result := billing.AnchorOrganization{ID: o.ID, Name: o.Name, LicenseValues: map[string]any{}}
				if o.License != nil {
					result.TemplateID = o.License.TemplateID
					result.LicenseValues = map[string]any(o.License.Values)
				}
				return result
			})...)
		if len(page.Items) == 0 || int64(offset)+int64(len(page.Items)) >= page.Total {
			break
		}
	}
	return snapshot, nil
}

func (g *anchorGateway) ApplyTemplate(ctx context.Context, organizationID, templateID string) error {
	held, err := g.licenses.GetLicense(
		ctx,
		license.GetLicenseInput{TenantID: g.tenantID, ProductID: g.productID, OrganizationID: organizationID},
	)
	if err != nil {
		return err
	}
	if held != nil && held.TemplateID == templateID {
		return nil
	}
	result, err := g.migrations.Migrate(
		ctx,
		license.MigrateLicensesInput{
			TenantID:        g.tenantID,
			ProductID:       g.productID,
			TemplateID:      templateID,
			OrganizationIDs: []string{organizationID},
			OnDifference:    license.CarryForwardDifferences,
		},
	)
	if err != nil {
		return err
	}
	if len(result.Results) != 1 {
		return errors.New("anchor returned an incomplete license migration")
	}
	if result.Results[0].Outcome == license.OutcomeFailed {
		return result.Results[0].Error
	}
	return nil
}
