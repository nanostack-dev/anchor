//nolint:testpackage // tests the unexported validation wrappers
package api

import (
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organizationinvitation"
	"anchor/internal/domain/product"
)

func detailOf(t *testing.T, err error) fault.Detail {
	t.Helper()

	var faulted *fault.Error
	require.ErrorAs(t, err, &faulted, "got %v", err)
	require.NotEmpty(t, faulted.Details)

	return faulted.Details[0]
}

// The generated searches answer their own errors, so these tests pin that a
// request Anchor refused before the move is still refused with the same body.

func TestUnknownSortOnAProductSearchIsAValidationErrorNamingTheOffers(t *testing.T) {
	t.Parallel()

	bogus := product.SortFieldProduct("bogus")
	err := validateProductsSearch("tenant", &SearchProductsRequestObject{
		Body: &SearchProductsJSONRequestBody{SortBy: &bogus, Pagination: &search.Pagination{Limit: 10}},
	})

	detail := detailOf(t, err)
	assert.Equal(t, "VALIDATION_ERROR", detail.Code)
	assert.Equal(t, "sort_by must be one of: created_at, updated_at, name", detail.Message)
	assert.Equal(t, "sort_by", detail.Metadata["field"])
	assert.Equal(t, "oneof", detail.Metadata["rule"])
}

func TestAnOfferedSortPassesValidation(t *testing.T) {
	t.Parallel()

	for _, field := range []product.SortFieldProduct{"created_at", "updated_at", "name"} {
		err := validateProductsSearch("tenant", &SearchProductsRequestObject{
			Body: &SearchProductsJSONRequestBody{SortBy: &field, Pagination: &search.Pagination{Limit: 10}},
		})
		assert.NoError(t, err, "sort_by %s", field)
	}
}

func TestBlankNamesStillFailValidation(t *testing.T) {
	t.Parallel()

	err := validateProductOrganizationsSearch("tenant", "product", nil, &SearchProductOrganizationsJSONRequestBody{
		Filter: &OrganizationFilter{Names: []string{" "}}, Pagination: &search.Pagination{Limit: 10},
	})

	detail := detailOf(t, err)
	assert.Equal(t, "VALIDATION_ERROR", detail.Code)
	assert.Equal(t, "Names[0]", detail.Metadata["field"])
}

func TestAnInvalidEmailFilterStillFailsValidation(t *testing.T) {
	t.Parallel()

	err := validatePlatformInvitationsSearch("tenant", &SearchPlatformInvitationsJSONRequestBody{
		Filter:     &PlatformInvitationFilter{Emails: []string{"not-an-email"}},
		Pagination: &search.Pagination{Limit: 10},
	})

	detail := detailOf(t, err)
	assert.Equal(t, "VALIDATION_ERROR", detail.Code)
	assert.Equal(t, "Emails[0]", detail.Metadata["field"])
	assert.Equal(t, "email", detail.Metadata["rule"])
}

func TestAnUnknownInvitationStatusKeepsItsOriginalMessage(t *testing.T) {
	t.Parallel()

	archived := organizationinvitation.Status("archived")
	err := validateProductInvitationsSearch("product", SearchOrganizationInvitationsJSONRequestBody{
		Filter:     &OrganizationInvitationFilter{Statuses: &[]organizationinvitation.Status{archived}},
		Pagination: &search.Pagination{Limit: 10},
	})

	detail := detailOf(t, err)
	assert.Equal(t, "Statuses[0] must be one of [pending accepted expired]", detail.Message)
	assert.Equal(t, "Statuses[0]", detail.Metadata["field"])
}
