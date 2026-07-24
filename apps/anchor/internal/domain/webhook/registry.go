// Package webhook defines the generic outbound webhook system: product-scoped
// endpoints, their signing secrets, the event outbox, and the delivery log.
//
// Nothing in this package is license-specific. `license.*` is simply the first
// registered event group; adding a group is one constant plus an emit call at
// the business site.
package webhook

import (
	"encoding/json"
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

// PingMessage is the `data.message` of a transport-probe ping.
const PingMessage = "Ping from Anchor"

// Sample identifiers. They are syntactically valid prefixed KSUIDs so a sample
// payload reads like a real one, and they are deliberately not resolvable: a
// sample is illustrative, never a pointer at live data.
const (
	SampleEndpointID     = "whe_2f9Kx7qLmNpRsTvWyZ1aB3cD4eF"
	SampleLicenseID      = "lic_2mQ8dTz5RkYbXhWnJ4pLcVsG7uE"
	SamplePlanID         = "pln_2hR6vBn9WkQdZxMtY3sJfLpC5aU"
	SamplePlanKey        = "pro"
	SamplePlanName       = "Pro"
	SampleLicenseStatus  = "ACTIVE"
	SampleRevokedStatus  = "REVOKED"
	SampleSuspendStatus  = "SUSPENDED"
	SampleChangedFieldID = "status"
)

// EventTypeDescriptor is one catalog entry: what the type is called, which
// group it belongs to, what it means, and what its payload looks like. The
// OpenAPI enum, the admin UI's picker and the test-event sender all derive from
// this catalog.
type EventTypeDescriptor struct {
	Type        string
	Group       string
	Description string
	// Sample is a representative `data` object for the type: what a receiver
	// sees, with illustrative identifiers. It is what a test send transmits and
	// what the admin UI previews before sending.
	//
	// Every registered type must carry one. That requirement is the seam: a new
	// event type added without a sample fails the registry test rather than
	// silently becoming untestable from the admin UI.
	Sample any
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
			Sample: LicenseEventData{
				LicenseID: SampleLicenseID,
				PlanKey:   SamplePlanKey,
				PlanID:    SamplePlanID,
				Status:    SampleLicenseStatus,
			},
		},
		{
			Type:  EventTypeLicenseUpdated,
			Group: GroupLicense,
			Description: "An organization's license changed: plan, expiry, grace, " +
				"entitlement overrides, or a suspend/reinstate transition.",
			Sample: LicenseEventData{
				LicenseID: SampleLicenseID,
				PlanKey:   SamplePlanKey,
				PlanID:    SamplePlanID,
				Status:    SampleSuspendStatus,
				Changes: map[string]LicenseChange{
					SampleChangedFieldID: {
						Previous: SampleLicenseStatus,
						New:      SampleSuspendStatus,
					},
				},
			},
		},
		{
			Type:        EventTypeLicenseRevoked,
			Group:       GroupLicense,
			Description: "An organization's license was revoked and stops resolving entitlements.",
			Sample: LicenseEventData{
				LicenseID: SampleLicenseID,
				PlanKey:   SamplePlanKey,
				PlanID:    SamplePlanID,
				Status:    SampleRevokedStatus,
				Changes: map[string]LicenseChange{
					SampleChangedFieldID: {
						Previous: SampleLicenseStatus,
						New:      SampleRevokedStatus,
					},
				},
			},
		},
		{
			Type:  EventTypePlanUpdated,
			Group: GroupPlan,
			Description: "A plan definition changed. Emitted once at product scope, " +
				"never once per licensed organization.",
			Sample: PlanEventData{
				PlanID:  SamplePlanID,
				PlanKey: SamplePlanKey,
				Name:    SamplePlanName,
			},
		},
		{
			Type:        EventTypePing,
			Group:       GroupPing,
			Description: "Synthetic delivery used to verify an endpoint's URL and signature setup.",
			Sample: PingEventData{
				EndpointID: SampleEndpointID,
				Message:    PingMessage,
			},
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

// SamplePayload returns the registered sample `data` object for eventType.
//
// An unknown type and a type registered without a sample are both errors: the
// caller is either sending something that cannot be emitted, or something the
// registry forgot to describe, and neither should reach a receiver.
func SamplePayload(eventType string) (any, error) {
	for _, descriptor := range eventTypeCatalog() {
		if descriptor.Type != eventType {
			continue
		}
		if descriptor.Sample == nil {
			return nil, fmt.Errorf("event type %q has no sample payload", eventType)
		}

		return descriptor.Sample, nil
	}

	return nil, fmt.Errorf("unknown event type %q", eventType)
}

// SamplePayloadJSON renders an event type's sample as indented JSON. It is what
// the catalog endpoint publishes so the admin UI can show exactly what a test
// send will transmit before it is sent.
func SamplePayloadJSON(eventType string) (string, error) {
	sample, err := SamplePayload(eventType)
	if err != nil {
		return "", err
	}

	encoded, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal sample payload for %q: %w", eventType, err)
	}

	return string(encoded), nil
}

// TestEventData builds the `data` object of a synthetic test send of eventType
// aimed at endpointID.
//
// It is the sample verbatim, with one exception: a ping names the endpoint it
// probes, so the placeholder identifier in the published sample is replaced by
// the real one. Matching on the payload type rather than the event type keeps
// that substitution tied to the shape that actually carries an endpoint id.
func TestEventData(eventType string, endpointID string) (any, error) {
	sample, err := SamplePayload(eventType)
	if err != nil {
		return nil, err
	}

	if probe, ok := sample.(PingEventData); ok {
		probe.EndpointID = endpointID

		return probe, nil
	}

	return sample, nil
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
