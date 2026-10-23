package service

import (
	"context"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	"anchor/internal/authz/model"
)

// Authorizer answers access questions from the relationship store, after
// replaying any write the answer depends on. Ask fluently:
//
//	access, err := authorizer.Product(productID).
//	    User(productUserID).
//	    InWorkspace(organizationID, workspaceID).
//	    Resolve(ctx)
type Authorizer struct {
	syncer *Syncer
}

func NewAuthorizer(syncer *Syncer) *Authorizer {
	return &Authorizer{syncer: syncer}
}

// Access is what a subject holds on an organization or a workspace.
type Access struct {
	OrganizationExists bool
	// IsMember is false for a product user outside the organization. An API key
	// always belongs to the organization it is asked about.
	IsMember bool
	// WorkspaceInOrganization is false when a workspace resource is unknown or
	// belongs to another organization; it is true for an organization resource.
	WorkspaceInOrganization bool
	// The role IDs the subject holds; a product user holds at most one of each.
	OrganizationRoleID functional.Option[string]
	WorkspaceRoleID    functional.Option[string]
	// Permissions are every permission held on the resource, sorted.
	Permissions []string
}

// Holds reports whether the subject holds a permission on the resource.
func (a Access) Holds(permission string) bool {
	_, found := slices.BinarySearch(a.Permissions, permission)
	return found
}

// Reaches reports whether the subject gets to the resource at all.
func (a Access) Reaches() bool {
	return a.OrganizationExists && a.IsMember && a.WorkspaceInOrganization
}

type ProductQuestion struct {
	authorizer *Authorizer
	productID  string
}

func (a *Authorizer) Product(productID string) ProductQuestion {
	return ProductQuestion{authorizer: a, productID: productID}
}

type SubjectQuestion struct {
	product ProductQuestion
	subject string
	isKey   bool
}

// User asks about a product user by its Anchor ID.
func (p ProductQuestion) User(productUserID string) SubjectQuestion {
	return SubjectQuestion{product: p, subject: model.User(productUserID)}
}

// APIKey asks about an organization API key by its ID, never its value.
func (p ProductQuestion) APIKey(apiKeyID string) SubjectQuestion {
	return SubjectQuestion{product: p, subject: model.APIKey(apiKeyID), isKey: true}
}

type Question struct {
	SubjectQuestion
	organizationID string
	workspaceID    string
}

func (s SubjectQuestion) InOrganization(organizationID string) Question {
	return Question{SubjectQuestion: s, organizationID: organizationID}
}

func (s SubjectQuestion) InWorkspace(organizationID, workspaceID string) Question {
	return Question{SubjectQuestion: s, organizationID: organizationID, workspaceID: workspaceID}
}

// Resolve reads what the subject holds, stopping at the first thing it lacks:
// an organization that does not exist holds no member, and a non-member is not
// asked about permissions.
func (q Question) Resolve(ctx context.Context) (Access, error) {
	current, err := q.product.authorizer.syncer.fresh(ctx, q.product.productID, q.organizationID)
	if err != nil {
		return Access{}, err
	}
	organization := model.Organization(q.organizationID)
	access := Access{WorkspaceInOrganization: q.workspaceID == ""}

	access.OrganizationExists, err = q.has(ctx, current, organization,
		model.RelationProduct, model.Product(q.product.productID))
	if err != nil || !access.OrganizationExists {
		return access, err
	}
	access.IsMember, access.OrganizationRoleID, err = q.membership(ctx, current, organization)
	if err != nil || !access.IsMember {
		return access, err
	}

	resource := organization
	if q.workspaceID != "" {
		resource = model.Workspace(q.workspaceID)
		access.WorkspaceInOrganization, err = q.has(ctx, current, resource, model.RelationOrganization, organization)
		if err != nil || !access.WorkspaceInOrganization {
			return access, err
		}
		if !q.isKey {
			if access.WorkspaceRoleID, err = q.role(ctx, current, resource); err != nil {
				return access, err
			}
		}
	}

	access.Permissions, err = q.permissions(ctx, current, resource)
	return access, err
}

// membership reads the subject's tuples on the organization: "member" and the
// relation of its organization role. An API key holds neither and is a member
// of its own organization by construction.
func (q Question) membership(
	ctx context.Context, current view, organization string,
) (bool, functional.Option[string], error) {
	if q.isKey {
		return true, functional.None[string](), nil
	}
	tuples, err := current.store.read(ctx, &openfgav1.ReadRequestTupleKey{Object: organization, User: q.subject})
	if err != nil {
		return false, functional.None[string](), err
	}
	isMember := slices.ContainsFunc(tuples, func(t model.Tuple) bool { return t.Relation == model.RelationMember })
	return isMember, roleIn(current.catalog, tuples), nil
}

func (q Question) role(ctx context.Context, current view, object string) (functional.Option[string], error) {
	tuples, err := current.store.read(ctx, &openfgav1.ReadRequestTupleKey{Object: object, User: q.subject})
	if err != nil {
		return functional.None[string](), err
	}
	return roleIn(current.catalog, tuples), nil
}

// roleIn returns the role the tuples assign. Postgres allows one role per
// member and scope, so at most one tuple names a role the catalog defines; the
// lowest ID wins should a stale one linger until the next sync.
func roleIn(catalog model.Catalog, tuples []model.Tuple) functional.Option[string] {
	var roleIDs []string
	for _, tuple := range tuples {
		if role, ok := catalog.RoleAssignedBy(tuple.Relation).Get(); ok {
			roleIDs = append(roleIDs, role.ID)
		}
	}
	if len(roleIDs) == 0 {
		return functional.None[string]()
	}
	return functional.Some(slices.Min(roleIDs))
}

func (q Question) has(ctx context.Context, current view, object, relation, user string) (bool, error) {
	tuples, err := current.store.read(
		ctx,
		&openfgav1.ReadRequestTupleKey{Object: object, Relation: relation, User: user},
	)
	return len(tuples) > 0, err
}

// permissions checks every permission of the catalog on the resource in one
// batch; the model computes each from roles, the organization and API keys.
func (q Question) permissions(ctx context.Context, current view, resource string) ([]string, error) {
	relations := make([]string, len(current.catalog.Permissions))
	for i, permission := range current.catalog.Permissions {
		relations[i] = model.PermissionRelation(permission)
	}
	held, err := current.store.holds(ctx, current.modelID, q.subject, resource, relations)
	if err != nil {
		return nil, err
	}
	permissions := []string{}
	for i, permission := range current.catalog.Permissions {
		if held[relations[i]] {
			permissions = append(permissions, permission)
		}
	}
	slices.Sort(permissions)
	return permissions, nil
}
