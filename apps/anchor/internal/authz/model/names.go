// Package model turns a product's roles and permissions into an OpenFGA
// authorization model, and organizations, members, workspaces and API keys into
// the relationship tuples that model reads.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Object types of the authorization model.
const (
	TypeUser         = "user"
	TypeAPIKey       = "api_key"
	TypeProduct      = "product"
	TypeOrganization = "organization"
	TypeWorkspace    = "workspace"
)

// Relations every product model defines, whatever its catalog holds.
const (
	RelationProduct      = "product"
	RelationMember       = "member"
	RelationOrganization = "organization"
)

const (
	// OpenFGA refuses relation names longer than this.
	maxRelationLength        = 50
	roleRelationPrefix       = "role"
	permissionRelationPrefix = "can"
	// Enough of a SHA-256 to keep two names apart once both are truncated or
	// reduced to the same safe characters ("flows:read" and "flows_read").
	relationHashLength = 12
)

// RoleRelation names the relation that assigns a product role, on an
// organization or a workspace.
func RoleRelation(roleID string) string {
	return relationName(roleRelationPrefix, roleID)
}

// PermissionRelation names the relation that answers whether a subject holds a
// product resource permission such as "flows:read".
func PermissionRelation(permission string) string {
	return relationName(permissionRelationPrefix, permission)
}

// relationName keeps the readable part of a name for debugging and appends a
// hash of the original, because permission names may hold characters OpenFGA
// does not accept in a relation (":" and "*" among them).
func relationName(prefix, raw string) string {
	sum := sha256.Sum256([]byte(raw))
	suffix := hex.EncodeToString(sum[:])[:relationHashLength]
	readable := sanitize(raw)
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

func Product(productID string) string { return TypeProduct + ":" + productID }

func Organization(organizationID string) string { return TypeOrganization + ":" + organizationID }

func Workspace(workspaceID string) string { return TypeWorkspace + ":" + workspaceID }

// ObjectID returns the ID part of an object reference.
func ObjectID(object string) string {
	_, id, _ := strings.Cut(object, ":")
	return id
}
