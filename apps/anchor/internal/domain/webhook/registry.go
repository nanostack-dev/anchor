// Package webhook defines the generic outbound webhook system: product-scoped
// endpoints, their signing secrets, the event outbox, and the delivery log.
//
// Nothing in this package is license-specific. `license.*` is simply the first
// registered event group; adding a group is one constant plus an emit call at
// the business site.
package webhook

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// APIVersion is the envelope contract version stamped on every event. It is
// pinned per event row so a future envelope change cannot rewrite the meaning
// of events already in flight.
const APIVersion = "2026-07-23"

// Event types registered in v1. The grammar is `<group>.<event>`, past tense.
// `ping` is the one deliberate exception: it is a transport-level probe rather
// than a business event, so it carries no group.
const (
	EventTypeLicenseCreated = "license.created"
	EventTypeLicenseUpdated = "license.updated"
	EventTypeLicenseRevoked = "license.revoked"
	EventTypePlanUpdated    = "plan.updated"
	EventTypePing           = "ping"
)

// WildcardSuffix subscribes an endpoint to every event of a group.
const WildcardSuffix = ".*"

// Event groups. A group is the first segment of an event type and the unit a
// `<group>.*` subscription covers.
const (
	GroupLicense = "license"
	GroupPlan    = "plan"
	GroupPing    = "ping"
)

// EventTypeDescriptor is one catalog entry: what the type is called, which
// group it belongs to, and what it means. The OpenAPI enum and the admin UI's
// picker both derive from this catalog.
type EventTypeDescriptor struct {
	Type        string
	Group       string
	Description string
}

// eventTypeCatalog is the ordered registry of every emittable event type.
//
// It is built per call rather than held in a package variable so no caller can
// mutate the catalog that every other caller reads.
func eventTypeCatalog() []EventTypeDescriptor {
	return []EventTypeDescriptor{
		{
			Type:        EventTypeLicenseCreated,
			Group:       GroupLicense,
			Description: "A license was assigned to an organization for the first time.",
		},
		{
			Type:  EventTypeLicenseUpdated,
			Group: GroupLicense,
			Description: "An organization's license changed: plan, expiry, grace, " +
				"entitlement overrides, or a suspend/reinstate transition.",
		},
		{
			Type:        EventTypeLicenseRevoked,
			Group:       GroupLicense,
			Description: "An organization's license was revoked and stops resolving entitlements.",
		},
		{
			Type:  EventTypePlanUpdated,
			Group: GroupPlan,
			Description: "A plan definition changed. Emitted once at product scope, " +
				"never once per licensed organization.",
		},
		{
			Type:        EventTypePing,
			Group:       GroupPing,
			Description: "Synthetic delivery used to verify an endpoint's URL and signature setup.",
		},
	}
}

// eventTypePattern is the grammar every registered business event type obeys.
func eventTypePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)
}

// EventTypeCatalog returns the registry, ordered for presentation.
func EventTypeCatalog() []EventTypeDescriptor {
	return eventTypeCatalog()
}

// EventTypes returns every registered event type.
func EventTypes() []string {
	catalog := eventTypeCatalog()
	types := make([]string, 0, len(catalog))
	for _, descriptor := range catalog {
		types = append(types, descriptor.Type)
	}

	return types
}

// IsRegisteredEventType reports whether eventType exists in the registry.
func IsRegisteredEventType(eventType string) bool {
	for _, descriptor := range eventTypeCatalog() {
		if descriptor.Type == eventType {
			return true
		}
	}

	return false
}

// Validate reports whether eventType may be emitted. Membership in the registry
// is the real rule; the grammar check only guards against a registry entry that
// was added without following the naming convention.
func Validate(eventType string) error {
	if !IsRegisteredEventType(eventType) {
		return fmt.Errorf("unknown event type %q", eventType)
	}
	if eventType != EventTypePing && !eventTypePattern().MatchString(eventType) {
		return fmt.Errorf("event type %q does not match the <group>.<event> grammar", eventType)
	}

	return nil
}

// IsRegisteredGroup reports whether at least one registered event type belongs
// to the given group.
func IsRegisteredGroup(group string) bool {
	for _, descriptor := range eventTypeCatalog() {
		if descriptor.Group == group {
			return true
		}
	}

	return false
}

// ValidateSubscription reports whether a subscription entry is usable: either an
// exact registered event type, or a `<group>.*` wildcard over a registered group.
func ValidateSubscription(subscription string) error {
	if subscription == "" {
		return errors.New("subscription entry must not be empty")
	}

	if group, ok := strings.CutSuffix(subscription, WildcardSuffix); ok {
		if !IsRegisteredGroup(group) {
			return fmt.Errorf("unknown event group %q in subscription %q", group, subscription)
		}

		return nil
	}

	if !IsRegisteredEventType(subscription) {
		return fmt.Errorf("unknown event type %q in subscription", subscription)
	}

	return nil
}

// MatchesSubscription reports whether an endpoint subscribed to `subscribed`
// should receive eventType. Exact matches and `<group>.*` wildcards both count;
// nothing else does, so an endpoint can never be accidentally subscribed to
// everything.
func MatchesSubscription(subscribed []string, eventType string) bool {
	for _, subscription := range subscribed {
		if subscription == eventType {
			return true
		}
		if group, ok := strings.CutSuffix(subscription, WildcardSuffix); ok {
			if strings.HasPrefix(eventType, group+".") {
				return true
			}
		}
	}

	return false
}
