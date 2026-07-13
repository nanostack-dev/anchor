package security

import "context"

type actorKey string

const requestActorKey actorKey = "request_actor"

// ActorType identifies the kind of authenticated principal on a request.
// Values match audit.ActorType so the audit service can cast directly.
type ActorType string

const (
	ActorTypePlatformUser ActorType = "PLATFORM_USER"
	//nolint:gosec // actor type label, not a credential
	ActorTypeProductAPIKey ActorType = "PRODUCT_API_KEY"
)

// Actor describes the authenticated principal performing the current request.
// Set by the auth middleware; consumed by the audit log service.
type Actor struct {
	Type ActorType
	ID   string
	Name string
}

// SetActor stores the request actor in the context.
func SetActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, requestActorKey, actor)
}

// GetActor retrieves the request actor from the context.
func GetActor(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(requestActorKey).(Actor)
	return actor, ok
}
