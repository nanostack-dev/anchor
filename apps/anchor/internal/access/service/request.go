package service

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

// Request is one access question: may this subject perform these actions on
// this organization, workspace or resource? Build it fluently:
//
//	accesssvc.Ask(productID).
//	    User(externalUserID).
//	    OnResource(organizationID, "document", documentID).
//	    Under("folder", folderID).
//	    Can("document:view")
//
// Each step returns a new Request, so a partial one can be reused.
type Request struct {
	ProductID string   `validate:"required,notblank"`
	Subject   Subject  `validate:"required"`
	Resource  Resource `validate:"required"`
	Actions   []string `validate:"dive,required,notblank"`
	Semantic  Semantic `validate:"omitempty,oneof=execute_all deny_on_first_deny permit_on_first_permit"`
	// Explain asks what granted each allowed action (Anchor's extension of
	// AuthZEN). It costs two more reads per decision, none without it.
	Explain bool
}

type Subject struct {
	Type SubjectType `validate:"required,oneof=user organization_api_key"`
	// ID is the external user ID or the raw API key. Never log it for a key.
	ID string `validate:"required,notblank"`
}

type Resource struct {
	Type ResourceType `validate:"required,notblank"`
	// ID is the organization or workspace ID, or the product's own ID of a
	// resource. Empty only for an organization resource with an API key
	// subject, meaning the key's organization.
	ID string
	// OrganizationID is the organization of anything but an organization; a
	// user subject needs it.
	OrganizationID string
	// Parent names the parent of a resource the registry lacks. A registered
	// resource's stored parent wins over it.
	Parent *ParentRef `validate:"omitempty"`
}

// ParentRef names a parent the way the product does: a resource type's slug
// and the product's ID, or organization and the Organization's ID.
type ParentRef struct {
	Type string `validate:"required,notblank"`
	ID   string `validate:"required,notblank"`
}

// Ask starts a question about the product's subjects, in the key's own
// organization until InOrganization or InWorkspace names one.
func Ask(productID string) Request {
	return Request{ProductID: productID, Resource: Resource{Type: ResourceOrganization}}
}

// User asks about a product user named by its external (identity provider) ID.
func (r Request) User(externalUserID string) Request {
	return r.As(SubjectUser, externalUserID)
}

// APIKey asks about a raw organization API key.
func (r Request) APIKey(rawKey string) Request {
	return r.As(SubjectOrganizationAPIKey, rawKey)
}

// As names the subject by type, for callers that map a wire value.
func (r Request) As(subjectType SubjectType, id string) Request {
	r.Subject = Subject{Type: subjectType, ID: id}
	return r
}

// InOrganization targets an organization; "" means the API key's own.
func (r Request) InOrganization(organizationID string) Request {
	return r.On(ResourceOrganization, organizationID, "")
}

// InWorkspace targets a workspace; organizationID may be "" for an API key.
func (r Request) InWorkspace(organizationID, workspaceID string) Request {
	return r.On(ResourceWorkspace, workspaceID, organizationID)
}

// OnResource targets a resource of one of the Product's types, by the
// product's own ID; organizationID may be "" for an API key.
func (r Request) OnResource(organizationID, resourceType, id string) Request {
	return r.On(ResourceType(resourceType), id, organizationID)
}

// On names the resource by type, for callers that map a wire value. A parent
// named before is kept, so Under works before or after it.
func (r Request) On(resourceType ResourceType, id, organizationID string) Request {
	r.Resource = Resource{Type: resourceType, ID: id, OrganizationID: organizationID, Parent: r.Resource.Parent}
	return r
}

// Under names the parent of a resource the registry lacks.
func (r Request) Under(parentType, parentID string) Request {
	r.Resource.Parent = &ParentRef{Type: parentType, ID: parentID}
	return r
}

// Explaining asks what granted each allowed action.
func (r Request) Explaining() Request {
	r.Explain = true
	return r
}

// Can adds the actions to decide. With none, the request asks only whether the
// subject reaches the resource.
func (r Request) Can(actions ...string) Request {
	r.Actions = append(append([]string{}, r.Actions...), actions...)
	return r
}

// Stopping sets how a batch short-circuits; the default evaluates every action.
func (r Request) Stopping(semantic Semantic) Request {
	r.Semantic = semantic
	return r
}

// Organization is the organization the request names, or "" when it names none.
func (r Request) Organization() string {
	if r.Resource.Type == ResourceOrganization {
		return r.Resource.ID
	}
	return r.Resource.OrganizationID
}

// checkResource enforces what tags cannot express: a workspace or a resource
// needs its ID, only a resource of the Product's own types takes a parent,
// and a user needs the organization it acts in.
func (r Request) checkResource() error {
	if r.Resource.Type == ResourceWorkspace && r.Resource.ID == "" {
		return fault.BadRequest("INVALID_ACCESS_RESOURCE", "A workspace resource needs an id.")
	}
	if !r.Resource.Type.builtIn() && r.Resource.ID == "" {
		return fault.BadRequest("INVALID_ACCESS_RESOURCE", "A resource needs its id.")
	}
	if r.Resource.Type.builtIn() && r.Resource.Parent != nil {
		return fault.BadRequest(
			"INVALID_ACCESS_RESOURCE",
			"Only a resource of the product's own types takes properties.parent; organization and workspace take none.",
		)
	}
	if r.Subject.Type == SubjectUser && r.Organization() == "" {
		return fault.BadRequest(
			"INVALID_ACCESS_RESOURCE",
			"A user subject needs the organization: the organization id, or properties.organization_id for any "+
				"other resource.",
		)
	}
	return nil
}
