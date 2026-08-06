package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createOrgMembership creates a role, a product user, and an organization
// membership linking them, returning both IDs for assertions.
//
//nolint:nonamedreturns // named per the required helper signature; both IDs are otherwise identical strings.
func createOrgMembership(t *testing.T, productID, organizationID string) (productUserID, roleID string) {
	t.Helper()

	roleID = createProductRole(t, productID)
	productUserID = createProductUser(t, productID)

	_, err := repoCtx.OrganizationMembershipRepository.Create(
		context.Background(), productID, organizationID, productUserID, roleID,
	)
	require.NoError(t, err)

	return productUserID, roleID
}

// createOrgMembershipWithUser creates a role and a membership for an
// explicitly-constructed product user, returning the product user's ID.
func createOrgMembershipWithUser(
	t *testing.T, productID, organizationID string, entity user.ProductUser,
) string {
	t.Helper()

	entity.ProductID = productID
	if entity.Status == "" {
		entity.Status = user.ProductUserStatusActive
	}
	entity.GenerateID()

	created, err := repoCtx.ProductUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	roleID := createProductRole(t, productID)

	_, err = repoCtx.OrganizationMembershipRepository.Create(
		context.Background(), productID, organizationID, created.ID, roleID,
	)
	require.NoError(t, err)

	return created.ID
}

func TestOrganizationMembershipRepositorySearchByOrgIDEmpty(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	result, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDPaginationBoundaries(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		productUserID, _ := createOrgMembership(t, productID, organizationID)
		created = append(created, productUserID)
	}

	sortReq := []search.Sort[organization.SortFieldMember]{
		{Field: organization.SortFieldMemberJoinedAt, Direction: search.SortAscending},
	}

	// Full page in one shot.
	result, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort:       sortReq,
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, created[i], item.ProductUserID)
	}

	// First page of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort:       sortReq,
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].ProductUserID)
	assert.Equal(t, created[1], page1.Items[1].ProductUserID)

	// Second page of 2.
	page2, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort:       sortReq,
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].ProductUserID)
	assert.Equal(t, created[3], page2.Items[1].ProductUserID)

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort:       sortReq,
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].ProductUserID)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDSortDirections(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	// Emails chosen so ascending/descending order is unambiguous.
	emails := []string{"alpha@example.com", "bravo@example.com", "charlie@example.com"}
	productUserIDs := make(map[string]string, len(emails))
	for _, email := range emails {
		id := createOrgMembershipWithUser(t, productID, organizationID, user.ProductUser{
			Email: email,
			Name:  uniqueName(t, "Test User"),
		})
		productUserIDs[email] = id
	}

	// --- JoinedAt ---

	joinedAsc, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort: []search.Sort[organization.SortFieldMember]{
				{Field: organization.SortFieldMemberJoinedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, joinedAsc.Items, 3)
	assert.Equal(
		t,
		[]string{
			productUserIDs["alpha@example.com"],
			productUserIDs["bravo@example.com"],
			productUserIDs["charlie@example.com"],
		},
		[]string{joinedAsc.Items[0].ProductUserID, joinedAsc.Items[1].ProductUserID, joinedAsc.Items[2].ProductUserID},
	)

	joinedDesc, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort: []search.Sort[organization.SortFieldMember]{
				{Field: organization.SortFieldMemberJoinedAt, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, joinedDesc.Items, 3)
	assert.Equal(
		t,
		[]string{
			productUserIDs["charlie@example.com"],
			productUserIDs["bravo@example.com"],
			productUserIDs["alpha@example.com"],
		},
		[]string{
			joinedDesc.Items[0].ProductUserID,
			joinedDesc.Items[1].ProductUserID,
			joinedDesc.Items[2].ProductUserID,
		},
	)

	// --- Email ---

	emailAsc, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort: []search.Sort[organization.SortFieldMember]{
				{Field: organization.SortFieldMemberEmail, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, emailAsc.Items, 3)
	assert.Equal(t, []string{"alpha@example.com", "bravo@example.com", "charlie@example.com"},
		[]string{emailAsc.Items[0].UserEmail, emailAsc.Items[1].UserEmail, emailAsc.Items[2].UserEmail})

	emailDesc, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Sort: []search.Sort[organization.SortFieldMember]{
				{Field: organization.SortFieldMemberEmail, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, emailDesc.Items, 3)
	assert.Equal(t, []string{"charlie@example.com", "bravo@example.com", "alpha@example.com"},
		[]string{emailDesc.Items[0].UserEmail, emailDesc.Items[1].UserEmail, emailDesc.Items[2].UserEmail})
}

func TestOrganizationMembershipRepositorySearchByOrgIDFilterByProductUserIDs(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	first, _ := createOrgMembership(t, productID, organizationID)
	second, _ := createOrgMembership(t, productID, organizationID)
	_, _ = createOrgMembership(t, productID, organizationID) // unrelated third membership, must not match

	byID, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{ProductUserIDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ProductUserID, byID.Items[1].ProductUserID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{ProductUserIDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDFilterByRoleIDs(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	matchingProductUserID, matchingRoleID := createOrgMembership(t, productID, organizationID)
	_, _ = createOrgMembership(t, productID, organizationID) // different role, must not match

	byRole, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{RoleIDs: []string{matchingRoleID}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byRole.Items, 1)
	assert.Equal(t, matchingProductUserID, byRole.Items[0].ProductUserID)
	assert.Equal(t, matchingRoleID, byRole.Items[0].RoleID)

	none, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{RoleIDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDFilterByEmails(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	matchingID := createOrgMembershipWithUser(t, productID, organizationID, user.ProductUser{
		Email: "findme@example.com",
		Name:  uniqueName(t, "Test User"),
	})
	_, _ = createOrgMembership(t, productID, organizationID) // unrelated email, must not match

	byEmail, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{Emails: []string{"findme@example.com"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byEmail.Items, 1)
	assert.Equal(t, matchingID, byEmail.Items[0].ProductUserID)

	none, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{Emails: []string{"nobody@example.com"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDFilterByExternalIDs(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	externalID := "clerk_" + uniqueName(t, "ext")
	matchingID := createOrgMembershipWithUser(t, productID, organizationID, user.ProductUser{
		Email:      uniqueName(t, "user") + "@example.com",
		Name:       uniqueName(t, "Test User"),
		ExternalID: &externalID,
	})
	_, _ = createOrgMembership(t, productID, organizationID) // no external ID, must not match

	byExternalID, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{ExternalIDs: []string{externalID}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byExternalID.Items, 1)
	assert.Equal(t, matchingID, byExternalID.Items[0].ProductUserID)

	none, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Filter:     &organization.SearchMembersFilter{ExternalIDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationMembershipRepositorySearchByOrgIDFullTextSearch(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	matchingID := createOrgMembershipWithUser(t, productID, organizationID, user.ProductUser{
		Email: uniqueName(t, "user") + "@example.com",
		Name:  "Atlas Migration User",
	})
	_ = createOrgMembershipWithUser(t, productID, organizationID, user.ProductUser{
		Email: uniqueName(t, "user") + "@example.com",
		Name:  "Billing Console User",
	})

	// The filter is a plain SQL LIKE (see jetx.BuildFullTextSearchFilter), not
	// tsvector — it is case-sensitive, so the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matchingID, result.Items[0].ProductUserID)
}

func TestOrganizationMembershipRepositorySearchByOrgIDScopesToOrganization(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)
	otherProductID, otherOrganizationID := tenantProductOrgChain(t)

	inScopeID, _ := createOrgMembership(t, productID, organizationID)
	_, _ = createOrgMembership(t, otherProductID, otherOrganizationID) // must never leak into the first org's results

	result, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), productID, organizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScopeID, result.Items[0].ProductUserID)

	otherResult, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(
		context.Background(), otherProductID, otherOrganizationID,
		search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}
