package service

// Decide exposes the decision step without repositories.
func Decide(identity *Identity, semantic Semantic, actions ...string) Decision {
	return reached(identity).decide(actions, semantic)
}

// Deny exposes a subject stopped before the resource.
func Deny(reason DenyReason, actions ...string) Decision {
	return stoppedBy(reason).decide(actions, ExecuteAll)
}

// CheckResource exposes the rules tags cannot express.
func CheckResource(request Request) error { return request.checkResource() }
