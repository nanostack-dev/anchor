package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
)

func memberIDs(list ct.ResourceMemberListResponse) []string {
	found := make([]string, len(list.Items))
	for i, item := range list.Items {
		found[i] = item.ProductUserId
	}
	return found
}

func TestSearchResourceMembers(t *testing.T) {
	t.Parallel()

	t.Run("A search lists the resource's members with their roles, oldest first", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		second := w.newMember(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Put(second, w.viewer.Id)
		w.projectMembers().Put(w.newMember(t), w.projectAdmin.Id)

		found := w.folderMembers().Search(ct.SearchResourceMembersJSONRequestBody{})

		assert.Equal(t, []string{w.member, second}, memberIDs(found))
		assert.EqualValues(t, 2, found.Total)
		assert.Equal(t, []string{w.viewer.Id}, roleIDs(found.Items[1]))
	})

	t.Run("A role filter keeps the members holding it", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		second := w.newMember(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Put(second, w.viewer.Id)

		found := w.folderMembers().Search(ct.SearchResourceMembersJSONRequestBody{
			Filter: &ct.ResourceMemberFilter{RoleIds: []string{w.viewer.Id}},
		})

		assert.Equal(t, []string{second}, memberIDs(found))
	})

	t.Run("A product user filter keeps those members", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		second := w.newMember(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Put(second, w.editor.Id)

		found := w.folderMembers().Search(ct.SearchResourceMembersJSONRequestBody{
			Filter: &ct.ResourceMemberFilter{ProductUserIds: []string{second, ids.MustNew("pusr")}},
		})

		assert.Equal(t, []string{second}, memberIDs(found))
	})

	t.Run("Sorting by product user ID descending reverses the order", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		second := w.newMember(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Put(second, w.editor.Id)
		sortBy, direction := ct.ResourceMemberSortField("product_user_id"), ct.DESC

		found := w.folderMembers().Search(ct.SearchResourceMembersJSONRequestBody{
			SortBy: &sortBy, SortDirection: &direction,
		})

		want := []string{w.member, second}
		if second > w.member {
			want = []string{second, w.member}
		}
		assert.Equal(t, want, memberIDs(found))
	})

	t.Run("A filter of 51 roles answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		roles := make([]string, 51)
		for i := range roles {
			roles[i] = ids.MustNew("role")
		}

		resp := w.folderMembers().SearchRaw(ct.SearchResourceMembersJSONRequestBody{
			Filter: &ct.ResourceMemberFilter{RoleIds: roles},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("A filter of 101 product users answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		users := make([]string, 101)
		for i := range users {
			users[i] = ids.MustNew("pusr")
		}

		resp := w.folderMembers().SearchRaw(ct.SearchResourceMembersJSONRequestBody{
			Filter: &ct.ResourceMemberFilter{ProductUserIds: users},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("An unregistered resource answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.product.ResourceMembers(w.org.Id, "folder", resourceID()).
			SearchRaw(ct.SearchResourceMembersJSONRequestBody{})

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})
}
