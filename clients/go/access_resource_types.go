package nanoclient

// The built-in access resource types. They replace the enum values the
// generator wrote while AccessResourceType was an enum of these two; the
// contract now leaves it open, since it also takes every resource type slug a
// product declares, so the generator writes no constants for it. Existing
// callers use these names, so they stay. A constant the generator writes
// later under the same name, such as an enum value "organization" or
// "workspace" of another schema, would collide with them and fail the build.
const (
	Organization AccessResourceType = "organization"
	Workspace    AccessResourceType = "workspace"
)
