// Package model compiles a Product's catalog into its OpenFGA authorization
// model, and an Organization's roles, members and API keys into the tuples
// that model reads (ADR-0022 E3). Roles are data: the model names types and
// permissions only, so a role change never changes it.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Fixed object types of every Product's model.
const (
	TypeUser           = "user"
	TypeAPIKey         = "api_key"
	TypeRole           = "role"
	TypeRoleAssignment = "role_assignment"
	TypeOrganization   = "organization"
)

// Relations every model defines.
const (
	RelationRole       = "role"
	RelationAssignee   = "assignee"
	RelationAssignment = "assignment"
	RelationParent     = "parent"
)

const (
	// OpenFGA refuses relation names longer than this.
	maxRelationLength = 50
	grantsPrefix      = "grants"
	canPrefix         = "can"
	// Enough of a SHA-256 to keep two names apart once they are truncated or
	// reduced to the same safe characters ("doc:read" and "doc_read").
	relationHashLength = 12
	// AllUsers is the wildcard subject of a role's grants.
	AllUsers = TypeUser + ":*"
)

// ResourceType names a resource type in the model. A Product's own types carry
// a prefix, so no valid slug can shadow a fixed type, collide with another, or
// be a word of OpenFGA's language ("define", "and", "self").
func ResourceType(slug string) string {
	if slug == TypeOrganization {
		return slug
	}
	return resourceTypePrefix + slug
}

const resourceTypePrefix = "rt_"

// CanRelation answers whether a subject holds a permission on an object.
func CanRelation(permission string) string {
	return relationName(canPrefix, permission)
}

// GrantsRelation is a role's bundling of a permission.
func GrantsRelation(permission string) string {
	return relationName(grantsPrefix, permission)
}

// relationName keeps the readable part of a permission for debugging and
// appends a hash of the original, because permission names hold characters
// OpenFGA does not accept in a relation (":" and "*" among them).
func relationName(prefix, permission string) string {
	sum := sha256.Sum256([]byte(permission))
	suffix := hex.EncodeToString(sum[:])[:relationHashLength]
	readable := sanitize(permission)
	room := maxRelationLength - len(prefix+"_") - len("_"+suffix)
	if len(readable) > room {
		readable = readable[:room]
	}
	return prefix + "_" + readable + "_" + suffix
}

func sanitize(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Object references, formatted the way OpenFGA expects ("type:id").

func User(productUserID string) string { return TypeUser + ":" + productUserID }

func APIKey(apiKeyID string) string { return TypeAPIKey + ":" + apiKeyID }

func Role(roleRowID string) string { return TypeRole + ":" + roleRowID }

func Organization(organizationID string) string { return TypeOrganization + ":" + organizationID }

// ResourceObject names a registered resource by its row ID. A product's own
// identifier is unique only in its Organization and type, so two
// Organizations would share an object in the Product's one store.
func ResourceObject(slug, rowID string) string { return ResourceType(slug) + ":" + rowID }

// Assignment names the assignment of one role on one object: an
// Organization, by its ID, or a registered resource, by its row ID. Every
// member holding the role there is one of its assignees, so a check walks the
// object's roles, a handful, not its members. The ID holds the role, so its
// role tuple never changes. Organization and resource row IDs are prefixed
// KSUIDs, so the two kinds never share an ID.
func Assignment(objectID, roleRowID string) string {
	return TypeRoleAssignment + ":" + objectID + "." + roleRowID
}

// AssignmentParts is what an assignment's ID holds.
type AssignmentParts struct {
	// ObjectID is the Organization's ID or the resource's row ID.
	ObjectID  string
	RoleRowID string
}

// ParseAssignment splits an assignment reference back into its parts.
func ParseAssignment(object string) (AssignmentParts, bool) {
	id, found := strings.CutPrefix(object, TypeRoleAssignment+":")
	if !found {
		return AssignmentParts{}, false
	}
	objectID, roleRowID, found := strings.Cut(id, ".")
	if !found || objectID == "" || roleRowID == "" || strings.Contains(roleRowID, ".") {
		return AssignmentParts{}, false
	}
	return AssignmentParts{ObjectID: objectID, RoleRowID: roleRowID}, true
}

// ObjectID returns the ID part of an object reference.
func ObjectID(object string) string {
	_, id, _ := strings.Cut(object, ":")
	return id
}

// unregisteredID names the object of a resource the registry lacks. It exists
// for one check only, under the contextual parent tuple the check gives it, and
// can't meet a stored object: those are named by prefixed row IDs.
const unregisteredID = "unregistered"

// UnregisteredResource names, for one check, a resource of the type that the
// registry lacks.
func UnregisteredResource(slug string) string { return ResourceType(slug) + ":" + unregisteredID }
