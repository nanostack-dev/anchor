package license_ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summaryFor looks one organization up in a page of search results.
func summaryFor(
	t *testing.T, page ct.OrganizationLicenseSearchResponse, organizationID string,
) ct.OrganizationLicenseSummaryResponse {
	t.Helper()
	for _, item := range page.Items {
		if item.OrganizationId == organizationID {
			return item
		}
	}
	require.FailNowf(t, "organization not in the page", "no result for %q", organizationID)
	return ct.OrganizationLicenseSummaryResponse{}
}

func summaryOrganizationIDs(page ct.OrganizationLicenseSearchResponse) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.OrganizationId)
	}
	return ids
}

func TestSearchOrganizationLicenses(t *testing.T) {
	t.Run("lists an organization with the license it holds", func(t *testing.T) {
		w := newLicensedWorld(t)

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{})

		summary := summaryFor(t, page, w.OrganizationID())
		assert.NotEmpty(t, summary.OrganizationName)
		require.NotNil(t, summary.License)
		assert.Equal(t, w.TemplateID(), summary.License.TemplateId)
		assertValues(t, summary.License.Values, validTemplateValues())
	})

	t.Run("lists an organization holding no license", func(t *testing.T) {
		w := newLicensedWorld(t)
		unlicensed := w.NewOrganization()

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{})

		// A row with no license, not a missing row: "who is on no tier" is half
		// of what this search is asked.
		assert.Contains(t, summaryOrganizationIDs(page), unlicensed)
		assert.Nil(t, summaryFor(t, page, unlicensed).License)
	})

	t.Run("does not derive usage", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge(worldLimitKey, 42))

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{})

		// Deriving it would cost one usage read per row. The single-organization
		// license read is where usage lives.
		require.NotNil(t, summaryFor(t, page, w.OrganizationID()).License)
		assert.Nil(t, summaryFor(t, page, w.OrganizationID()).License.Usage)
	})

	t.Run("keeps only the organizations on the named tiers", func(t *testing.T) {
		w := newLicensedWorld(t)
		onPro := w.NewOrganization()
		unlicensed := w.NewOrganization()
		pro := w.NewTemplate(proValues())
		w.License().For(onPro).Instantiate(pro.Id)

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			Filter: &ct.OrganizationLicenseFilter{LicenseTemplateIds: []string{pro.Id}},
		})

		assert.Equal(t, []string{onPro}, summaryOrganizationIDs(page))
		assert.NotContains(t, summaryOrganizationIDs(page), unlicensed)
	})

	t.Run("finds the customers still on a withdrawn tier", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Template().Archive()

		// The reason an archived template stays listed at all: this is how an
		// operator finds who has to be moved off it.
		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			Filter: &ct.OrganizationLicenseFilter{LicenseTemplateIds: []string{w.TemplateID()}},
		})

		assert.Equal(t, []string{w.OrganizationID()}, summaryOrganizationIDs(page))
	})

	t.Run("narrows to organizations that hold a license, or that hold none", func(t *testing.T) {
		w := newLicensedWorld(t)
		unlicensed := w.NewOrganization()

		licensed := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			Filter: &ct.OrganizationLicenseFilter{Licensed: new(true)},
		})
		none := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			Filter: &ct.OrganizationLicenseFilter{Licensed: new(false)},
		})

		assert.Equal(t, []string{w.OrganizationID()}, summaryOrganizationIDs(licensed))
		assert.Equal(t, []string{unlicensed}, summaryOrganizationIDs(none))
	})

	t.Run("matches the organization name", func(t *testing.T) {
		w := newLicensedWorld(t)
		named := w.NewNamedOrganization("Northwind Trading")
		w.NewNamedOrganization("Globex")

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			FullTextSearch: new("northwind"),
		})

		assert.Equal(t, []string{named}, summaryOrganizationIDs(page))
	})

	t.Run("pages, and reports the total beyond the page", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.NewOrganization()
		w.NewOrganization()

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			Pagination: &ct.PaginationRequest{Limit: new(int32(2)), Offset: new(int32(0))},
		})

		assert.Len(t, page.Items, 2)
		assert.Equal(t, 2, page.Count)
		assert.Equal(t, int64(3), page.Total)
	})

	t.Run("sorts by organization name", func(t *testing.T) {
		w := newLicenseWorld(t)
		w.NewNamedOrganization("Zeta Industries")
		w.NewNamedOrganization("Alpha Holdings")

		page := w.Search().Run(ct.OrganizationLicenseSearchRequest{
			SortBy:         new(ct.OrganizationName),
			SortDirection:  new(ct.ASC),
			FullTextSearch: new("Holdings"),
		})

		require.NotEmpty(t, page.Items)
		assert.Equal(t, "Alpha Holdings", page.Items[0].OrganizationName)
	})

	t.Run("another product's organizations are not reachable", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicensedWorld(t)

		page := first.Search().Run(ct.OrganizationLicenseSearchRequest{})

		assert.NotContains(t, summaryOrganizationIDs(page), second.OrganizationID())
	})

	t.Run("the read scope is what reaches it", func(t *testing.T) {
		w := newLicensedWorld(t)
		writeOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:update", "organization_license:migrate"},
		)

		resp := w.Search().As(writeOnly).RunRaw(ct.OrganizationLicenseSearchRequest{})
		assert.Equal(t, http.StatusForbidden, resp.StatusCode())
	})
}
