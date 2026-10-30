package nanoclient

// Organization is the built-in access resource type. The contract leaves
// AccessResourceType open, since it also takes every resource type slug a
// product declares, so the generator writes no constant for it. A constant the
// generator writes later under the same name, such as an enum value
// "organization" of another schema, would collide with it and fail the build.
const Organization AccessResourceType = "organization"
