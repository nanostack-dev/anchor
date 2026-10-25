package nanoclient

// The built-in access resource types. The contract leaves AccessResourceType
// open, since it also takes every resource type slug a product declares, so
// the generator no longer writes these.
const (
	Organization AccessResourceType = "organization"
	Workspace    AccessResourceType = "workspace"
)
