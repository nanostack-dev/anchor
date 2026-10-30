package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	documentView  = "document:view"
	documentEdit  = "document:edit"
	projectManage = "project:manage"
)

// resourceAccessWorld is a product with the document tree of ADR-0022 and two
// Organizations. In the first, project › folder › sub-folder, with a document
// registered under the folder and one under the sub-folder, and a member
// holding folder-viewer on the folder and nothing in the Organization.
type resourceAccessWorld struct {
	product   *itdsl.ProductContext
	access    itdsl.AccessClient
	org       ct.ProductOrganizationResponse
	other     ct.ProductOrganizationResponse
	resources itdsl.ResourceClient
	// The product's identifiers of the first Organization's resources.
	project, folder, subfolder, document, subDocument string
	// The roles the product manages.
	orgMember, docReader, folderViewer, documentViewer ct.ProductRoleResponse
	// member holds orgMember in the Organization and folderViewer on folder.
	member, memberExternalID string
}

func givenResourceAccessWorld(t *testing.T) resourceAccessWorld {
	t.Helper()
	product := createTestProductContext(t)
	declareDocumentTree(t, product)
	product.CreateScopedPermissions(t, "document", documentView, documentEdit)
	product.CreateScopedPermissions(t, "project", projectManage)
	w := resourceAccessWorld{
		product: product,
		access:  product.Access(),
		orgMember: createManagedRole(
			t, product, ct.CreateProductRoleJSONRequestBody{Name: "Member " + ids.MustNew("ct")},
		),
		docReader: createManagedRole(t, product, ct.CreateProductRoleJSONRequestBody{
			Name: "Doc reader " + ids.MustNew("ct"), Permissions: []string{documentView},
		}),
		folderViewer:   createManagedRole(t, product, rolePermitting("folder", "Folder viewer", documentView)),
		documentViewer: createManagedRole(t, product, rolePermitting("document", "Document viewer", documentView)),
	}
	w.org = product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
	w.other = product.CreateOrganization(t, "Other-"+ids.MustNew("ct"), nil)
	w.resources = product.Resources(w.org.Id)
	w.project, w.folder, w.subfolder = resourceID(), resourceID(), resourceID()
	w.document, w.subDocument = resourceID(), resourceID()
	w.resources.Put("project", w.project, "Website redesign", w.resources.Top())
	w.resources.Put("folder", w.folder, "Design", itdsl.Ref("project", w.project))
	w.resources.Put("folder", w.subfolder, "Specs", itdsl.Ref("folder", w.folder))
	w.resources.Put("document", w.document, "Brief", itdsl.Ref("folder", w.folder))
	w.resources.Put("document", w.subDocument, "Pricing page", itdsl.Ref("folder", w.subfolder))
	w.member, w.memberExternalID = w.newMember(t, w.orgMember.Id)
	w.members("folder", w.folder).Put(w.member, w.folderViewer.Id)
	return w
}

func rolePermitting(resourceType, name string, permissions ...string) ct.CreateProductRoleJSONRequestBody {
	body := roleOn(resourceType, name)
	body.Permissions = permissions
	return body
}

// newMember adds a product user with an external ID to the first
// Organization with the given role.
func (w resourceAccessWorld) newMember(t *testing.T, roleID string) (string, string) {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	externalID := "user_" + ids.MustNew("ext")
	setDSLProductUserExternalID(t, w.product, user.ID, externalID)
	addMember(t, w.product, w.org.Id, user.ID, roleID)
	return user.ID, externalID
}

func (w resourceAccessWorld) members(resourceType, id string) itdsl.ResourceMemberClient {
	return w.product.ResourceMembers(w.org.Id, resourceType, id)
}

// asMember asks about the world's member on a resource of the first
// Organization.
func (w resourceAccessWorld) asMember(resourceType, id string) itdsl.AccessQuestion {
	return w.access.User(w.memberExternalID).OnResource(w.org.Id, resourceType, id)
}

// apiKey creates an API key of the first Organization holding the
// permissions.
func (w resourceAccessWorld) apiKey(t *testing.T, permissions ...string) (string, string) {
	t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationAPIKeyWithResponse(
		t.Context(), w.product.ProductID, w.org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{Name: "Key-" + ids.MustNew("ct"), Permissions: permissions},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return resp.JSON201.Id, resp.JSON201.Value
}

func requireAllowed(t *testing.T, response ct.AccessEvaluationsResponse) {
	t.Helper()
	assert.True(t, response.Decision, "context: %+v", response.Context)
	assert.Nil(t, response.Context.Reason)
}

func roleGrant(role ct.ProductRoleResponse, resourceType, id string) ct.AccessGrant {
	return ct.AccessGrant{
		Role:     &ct.AccessIdentityRef{Id: role.Id, Name: &role.Name},
		Resource: ct.AccessGrantResource{Type: resourceType, Id: id},
	}
}

func TestAccessEvaluationOnResources(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)
	unregistered := resourceID()

	t.Run("A role on a folder allows a document registered under it", func(t *testing.T) {
		requireAllowed(t, w.asMember("document", w.document).Can(documentView).Decide())
	})

	t.Run("A role on a folder allows a document registered under its sub-folder", func(t *testing.T) {
		requireAllowed(t, w.asMember("document", w.subDocument).Can(documentView).Decide())
	})

	t.Run("A role on a folder allows an unregistered document named by its parent folder", func(t *testing.T) {
		requireAllowed(t, w.asMember("document", unregistered).Under("folder", w.folder).Can(documentView).Decide())
		requireAllowed(t, w.asMember("document", unregistered).Under("folder", w.subfolder).Can(documentView).Decide())
	})

	t.Run("A role on a folder allows the folder itself and its sub-folder", func(t *testing.T) {
		requireAllowed(t, w.asMember("folder", w.folder).Can(documentView).Decide())
		requireAllowed(t, w.asMember("folder", w.subfolder).Can(documentView).Decide())
	})

	t.Run("A role on a folder grants nothing on its parent project", func(t *testing.T) {
		requireDenied(t, w.asMember("project", w.project).Can(documentView).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A role on a folder grants nothing on a document under the parent project", func(t *testing.T) {
		response := w.asMember("document", unregistered).Under("project", w.project).Can(documentView).Decide()
		requireDenied(t, response, ct.MISSINGPERMISSION)
	})

	t.Run("A role on a folder grants nothing on the organization", func(t *testing.T) {
		response := w.access.User(w.memberExternalID).InOrganization(w.org.Id).Can(documentView).Decide()
		requireDenied(t, response, ct.MISSINGPERMISSION)
	})

	t.Run("A role on a folder grants only the permissions its role holds", func(t *testing.T) {
		response := w.asMember("document", w.document).Can(documentEdit).Decide()
		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, []string{documentEdit}, *response.Context.MissingPermissions)
	})

	t.Run("A resource decision carries the member's organization identity", func(t *testing.T) {
		response := w.asMember("document", w.document).Can(documentView).Decide()

		assert.Equal(t, w.org.Id, response.Context.Organization.Id)
		assert.Equal(t, w.orgMember.Id, response.Context.Role.Id)
		assert.Equal(t, w.member, response.Context.ProductUser.Id)
		assert.Empty(t, *response.Context.Permissions)
		assert.Nil(t, response.Context.GrantedBy, "no explanation was asked for")
	})

	t.Run("The single evaluation decides a resource the same way", func(t *testing.T) {
		resp := w.asMember("document", unregistered).Under("folder", w.folder).Can(documentView).Single()

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.True(t, resp.JSON200.Decision)
	})

	t.Run("The SDK decides a resource the same way as the API", func(t *testing.T) {
		decision, err := w.product.SDKClient().AccessForUser(w.memberExternalID).
			OnResource(w.org.Id, "document", unregistered).
			Under("folder", w.subfolder).
			Permissions(documentView).
			Explain().
			Do(t.Context())

		require.NoError(t, err)
		assert.True(t, decision.Decision)
		require.Len(t, decision.Evaluations, 1)
		require.NotNil(t, decision.Evaluations[0].Context)
		assert.Equal(t, []ct.AccessGrant{roleGrant(w.folderViewer, "folder", w.folder)},
			*decision.Evaluations[0].Context.GrantedBy)
	})
}

func TestAccessEvaluationOnResourcesThroughTheOrganization(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)
	_, reader := w.newMember(t, w.docReader.Id)

	t.Run("An organization role reaches every resource of the organization", func(t *testing.T) {
		as := w.access.User(reader)
		requireAllowed(t, as.InOrganization(w.org.Id).Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "project", w.project).Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "folder", w.subfolder).Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "document", w.subDocument).Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "document", resourceID()).Under("project", w.project).
			Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "project", resourceID()).Under("organization", w.org.Id).
			Can(documentView).Decide())
	})

	t.Run("An organization API key reaches the resources of its organization", func(t *testing.T) {
		_, key := w.apiKey(t, documentView)
		as := w.access.APIKey(key)

		requireAllowed(t, as.OnResource("", "document", w.subDocument).Can(documentView).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "document", resourceID()).Under("folder", w.folder).
			Can(documentView).Decide())
		requireDenied(t, as.OnResource("", "document", w.document).Can(documentEdit).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("An organization API key naming another organization's resource is not a member", func(t *testing.T) {
		_, key := w.apiKey(t, documentView)
		other := w.product.Resources(w.other.Id)
		project := resourceID()
		other.Put("project", project, "Elsewhere", other.Top())

		response := w.access.APIKey(key).OnResource(w.other.Id, "project", project).Can(documentView).Decide()
		requireDenied(t, response, ct.NOTAMEMBER)
	})

	t.Run("A permission the resource's type does not hold is missing, not an error", func(t *testing.T) {
		response := w.access.User(reader).OnResource(w.org.Id, "document", w.document).Can(projectManage).Decide()
		requireDenied(t, response, ct.MISSINGPERMISSION)
	})

	t.Run("An organization-scoped permission asked on a document is answered by its organization", func(t *testing.T) {
		billing := "billing:read"
		w.product.CreateScopedPermissions(t, "organization", billing)
		admin := createManagedRole(t, w.product, ct.CreateProductRoleJSONRequestBody{
			Name: "Billing " + ids.MustNew("ct"), Permissions: []string{billing},
		})
		_, billingReader := w.newMember(t, admin.Id)
		as := w.access.User(billingReader)

		requireAllowed(t, as.InOrganization(w.org.Id).Can(billing).Decide())
		requireAllowed(t, as.OnResource(w.org.Id, "document", w.document).Can(billing).Decide())
		requireDenied(t, w.access.User(w.memberExternalID).OnResource(w.org.Id, "document", w.document).
			Can(billing).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("An organization API key is refused another organization's resources", func(t *testing.T) {
		_, key := w.apiKey(t, documentView)
		other := w.product.Resources(w.other.Id)
		project, folder := resourceID(), resourceID()
		other.Put("project", project, "Elsewhere", other.Top())
		other.Put("folder", folder, "Elsewhere", itdsl.Ref("project", project))
		as := w.access.APIKey(key)

		requireDenied(t, as.OnResource("", "folder", folder).Can(documentView).Decide(), ct.RESOURCENOTFOUND)
		requireDenied(t, as.OnResource("", "document", resourceID()).Under("folder", folder).
			Can(documentView).Decide(), ct.RESOURCENOTINORGANIZATION)
	})
}

func TestAccessEvaluationResourceDenyReasons(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)
	other := w.product.Resources(w.other.Id)
	otherFolder, otherProject := resourceID(), resourceID()
	other.Put("project", otherProject, "Elsewhere", other.Top())
	other.Put("folder", otherFolder, "Elsewhere", itdsl.Ref("project", otherProject))

	cases := []struct {
		name     string
		question itdsl.AccessQuestion
		reason   ct.AccessDenyReason
	}{
		{
			name:     "A resource type the product has not declared is denied as not found",
			question: w.asMember("dashboard", resourceID()),
			reason:   ct.RESOURCETYPENOTFOUND,
		},
		{
			name:     "An unregistered resource that names no parent is denied as not found",
			question: w.asMember("document", resourceID()),
			reason:   ct.RESOURCENOTFOUND,
		},
		{
			name:     "An unregistered resource whose parent is not registered is denied as not found",
			question: w.asMember("document", resourceID()).Under("folder", resourceID()),
			reason:   ct.RESOURCENOTFOUND,
		},
		{
			name:     "A resource registered only in another organization is denied as not found",
			question: w.asMember("folder", otherFolder),
			reason:   ct.RESOURCENOTFOUND,
		},
		{
			name:     "A parent registered only in another organization is denied as not in the organization",
			question: w.asMember("document", resourceID()).Under("folder", otherFolder),
			reason:   ct.RESOURCENOTINORGANIZATION,
		},
		{
			name:     "Another organization as the parent is denied as not in the organization",
			question: w.asMember("project", resourceID()).Under("organization", w.other.Id),
			reason:   ct.RESOURCENOTINORGANIZATION,
		},
		{
			name:     "A parent of a type the resource's type can't sit under is denied as not allowed",
			question: w.asMember("document", resourceID()).Under("organization", w.org.Id),
			reason:   ct.PARENTTYPENOTALLOWED,
		},
		{
			name:     "A parent of a type the product has not declared is denied as not allowed",
			question: w.asMember("document", resourceID()).Under("dashboard", resourceID()),
			reason:   ct.PARENTTYPENOTALLOWED,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			response := c.question.Can(documentView, documentEdit).Decide()

			requireDenied(t, response, c.reason)
			assert.Nil(t, response.Context.Organization, "a refusal before the permission check carries no identity")
			require.Len(t, response.Evaluations, 2)
			for _, item := range response.Evaluations {
				assert.Equal(t, c.reason, *item.Context.Reason)
			}
		})
	}

	t.Run("A user who is not a member is refused before the resource is looked up", func(t *testing.T) {
		response := w.access.User(w.memberExternalID).OnResource(w.other.Id, "dashboard", resourceID()).
			Can(documentView).Decide()
		requireDenied(t, response, ct.NOTAMEMBER)
	})
}

func TestAccessEvaluationResourceParents(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)

	t.Run("A registered resource is checked under its stored parent, not the one named", func(t *testing.T) {
		response := w.asMember("document", w.document).Under("project", w.project).Can(documentView).Decide()
		requireAllowed(t, response)
	})

	t.Run("Naming a parent the member can reach gives nothing on a resource stored elsewhere", func(t *testing.T) {
		underProject := resourceID()
		w.resources.Put("document", underProject, "Roadmap", itdsl.Ref("project", w.project))

		response := w.asMember("document", underProject).Under("folder", w.folder).Can(documentView).Decide()
		requireDenied(t, response, ct.MISSINGPERMISSION)
	})

	t.Run("An organization resource takes no parent", func(t *testing.T) {
		as := w.access.User(w.memberExternalID)
		for _, question := range []itdsl.AccessQuestion{
			as.InOrganization(w.org.Id).Under("project", w.project),
		} {
			resp := question.Can(documentView).Send()

			requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
			require.NotNil(t, resp.JSON400)
			require.NotEmpty(t, resp.JSON400.Errors)
			assert.Equal(t, "INVALID_ACCESS_RESOURCE", resp.JSON400.Errors[0].Code)
		}
	})

	t.Run("A resource without an id is a bad request", func(t *testing.T) {
		resp := w.asMember("document", "").Can(documentView).Send()
		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})

	t.Run("A user on a resource without an organization is a bad request", func(t *testing.T) {
		resp := w.access.User(w.memberExternalID).OnResource("", "document", w.document).Can(documentView).Send()
		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})
}

func TestAccessEvaluationExplainsGrants(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)
	reader, readerExternalID := w.newMember(t, w.docReader.Id)
	w.members("folder", w.folder).Put(reader, w.folderViewer.Id)
	w.members("document", w.document).Put(reader, w.documentViewer.Id)
	asReader := func(resourceType, id string) itdsl.AccessQuestion {
		return w.access.User(readerExternalID).OnResource(w.org.Id, resourceType, id)
	}
	organizationGrant := roleGrant(w.docReader, "organization", w.org.Id)

	t.Run("Every grant is listed nearest first, the organization role last", func(t *testing.T) {
		response := asReader("document", w.document).Can(documentView).Explain().Decide()

		requireAllowed(t, response)
		require.NotNil(t, response.Evaluations[0].Context)
		assert.Equal(t, []ct.AccessGrant{
			roleGrant(w.documentViewer, "document", w.document),
			roleGrant(w.folderViewer, "folder", w.folder),
			organizationGrant,
		}, *response.Evaluations[0].Context.GrantedBy)
	})

	t.Run("An unregistered resource lists the grants from its parent up", func(t *testing.T) {
		response := asReader("document", resourceID()).Under("folder", w.subfolder).Can(documentView).Explain().Decide()

		assert.Equal(t, []ct.AccessGrant{roleGrant(w.folderViewer, "folder", w.folder), organizationGrant},
			*response.Evaluations[0].Context.GrantedBy)
	})

	t.Run("The organization role alone grants a decision on the organization", func(t *testing.T) {
		response := w.access.User(readerExternalID).InOrganization(w.org.Id).Can(documentView).Explain().Decide()

		assert.Equal(t, []ct.AccessGrant{organizationGrant}, *response.Evaluations[0].Context.GrantedBy)
	})

	t.Run("An API key's own permission is its grant on its organization", func(t *testing.T) {
		keyID, key := w.apiKey(t, documentView)
		response := w.access.APIKey(key).OnResource("", "document", w.document).Can(documentView).Explain().Decide()

		grants := *response.Evaluations[0].Context.GrantedBy
		require.Len(t, grants, 1)
		assert.Nil(t, grants[0].Role)
		require.NotNil(t, grants[0].ApiKey)
		assert.Equal(t, keyID, grants[0].ApiKey.Id)
		assert.Equal(t, ct.AccessGrantResource{Type: "organization", Id: w.org.Id}, grants[0].Resource)
	})

	t.Run("Only allowed items carry their grants", func(t *testing.T) {
		response := w.asMember("document", w.subDocument).Can(documentEdit, documentView).Explain().Decide()

		require.Len(t, response.Evaluations, 2)
		assert.Nil(t, response.Evaluations[0].Context.GrantedBy)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Evaluations[0].Context.Reason)
		assert.Equal(t, []ct.AccessGrant{roleGrant(w.folderViewer, "folder", w.folder)},
			*response.Evaluations[1].Context.GrantedBy)
		assert.Nil(t, response.Context.GrantedBy, "the batch context explains no single item")
	})

	t.Run("The single evaluation explains its decision in its context", func(t *testing.T) {
		resp := asReader("document", w.subDocument).Can(documentView).Explain().Single()

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, []ct.AccessGrant{roleGrant(w.folderViewer, "folder", w.folder), organizationGrant},
			*resp.JSON200.Context.GrantedBy)
	})

	t.Run("Without explain, an allowed item carries no context", func(t *testing.T) {
		response := asReader("document", w.document).Can(documentView).Decide()

		assert.Nil(t, response.Evaluations[0].Context)
	})
}

func TestAccessEvaluationBatchOnResources(t *testing.T) {
	t.Parallel()
	w := givenResourceAccessWorld(t)

	t.Run("Each action is decided on the resource and the batch is denied by a missing one", func(t *testing.T) {
		response := w.asMember("document", w.document).Can(documentView, documentEdit, projectManage).Decide()

		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, []string{documentEdit, projectManage}, *response.Context.MissingPermissions)
		require.Len(t, response.Evaluations, 3)
		assert.True(t, response.Evaluations[0].Decision)
		assert.False(t, response.Evaluations[1].Decision)
		assert.False(t, response.Evaluations[2].Decision)
	})

	t.Run("Deny on first deny leaves the remaining actions on the resource not evaluated", func(t *testing.T) {
		response := w.asMember("document", resourceID()).Under("folder", w.folder).
			Can(documentEdit, documentView).DenyOnFirstDeny().Decide()

		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, ct.NOTEVALUATED, *response.Evaluations[1].Context.Reason)
	})

	t.Run("No action asks only whether the subject reaches the resource", func(t *testing.T) {
		requireAllowed(t, w.asMember("document", w.document).Decide())
		requireDenied(t, w.asMember("document", resourceID()).Decide(), ct.RESOURCENOTFOUND)
	})
}

func TestAccessEvaluationOnResourcesSeesWritesOnTheNextCall(t *testing.T) {
	t.Parallel()

	t.Run("A removed role assignment is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		question := w.asMember("document", resourceID()).Under("folder", w.subfolder).Can(documentView)
		requireAllowed(t, question.Decide())

		w.members("folder", w.folder).Delete(w.member)

		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A role assigned on a document applies on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		elsewhere := resourceID()
		w.resources.Put("document", elsewhere, "Roadmap", itdsl.Ref("project", w.project))
		question := w.asMember("document", elsewhere).Can(documentView)
		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)

		w.members("document", elsewhere).Put(w.member, w.documentViewer.Id)

		requireAllowed(t, question.Decide())
	})

	t.Run("A document moved out of the folder is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		question := w.asMember("document", w.document).Can(documentView)
		requireAllowed(t, question.Decide())

		w.resources.Put("document", w.document, "Brief", itdsl.Ref("project", w.project))

		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)
	})
}
