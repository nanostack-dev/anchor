package service

import "slices"

// Decide exposes the decision step without repositories or OpenFGA: the
// identity holds the permissions it lists.
func Decide(identity *Identity, semantic Semantic, actions ...string) Decision {
	return reached(identity).decide(actions, semantic, func(action string) bool {
		return slices.Contains(identity.Permissions, action)
	})
}

// Deny exposes a subject stopped before the resource.
func Deny(reason DenyReason, actions ...string) Decision {
	return stoppedBy(reason).decide(actions, ExecuteAll, func(string) bool { return false })
}

// CheckResource exposes the rules tags cannot express.
func CheckResource(request Request) error { return request.checkResource() }
