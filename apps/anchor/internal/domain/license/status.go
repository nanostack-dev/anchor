package license

import (
	"time"

	"anchor/internal/license/rules"
)

// UsageStatus is one limit field's derived state against its latest reported
// usage. Computed on every license read; never stored. See CONTEXT.md's
// licensing glossary and docs/adr/0012-license-status-derived-on-read.md.
//
// Anchor advises, never gates: this value is information a consuming product
// uses to make its own decision, never a reason Anchor itself refuses or
// blocks anything.
type UsageStatus string

const (
	// UsageWithinLimit is the latest reported usage, strictly under the limit.
	UsageWithinLimit UsageStatus = "within_limit"
	// UsageAtLimit is the latest reported usage, exactly at the limit.
	UsageAtLimit UsageStatus = "at_limit"
	// UsageExceeded is the latest reported usage, past the limit. UsageService
	// stores a report past the limit as-is — see docs/adr/0003 — so this is
	// always reachable.
	UsageExceeded UsageStatus = "exceeded"
	// UsageStale means the latest observation cannot be trusted as current:
	// either nothing has ever been reported for this field, or the latest
	// report is older than the field's declared expected reporting interval.
	UsageStale UsageStatus = "stale"
)

// FieldUsage is one limit field's latest reported usage and derived status, as
// carried by an organization's license read. Computed fresh from the latest
// observation on record every time; never stored.
type FieldUsage struct {
	Field string
	// Limit is the organization's current value for this field, mirroring
	// OrganizationLicense.Values[Field].
	Limit float64
	// Usage is the latest reported value, or nil when this field has never
	// been reported against.
	Usage  *float64
	Status UsageStatus
	// LastReportedAt is when the latest observation was recorded, nil exactly
	// when Usage is nil.
	LastReportedAt *time.Time
}

// OrganizationLicenseRead is an organization's effective license: its own copy
// of a template's values, plus every limit field's latest usage and derived
// status. This is what the license read hands back — embedding usage saves a
// consumer a second call on the hot path. See
// docs/adr/0012-license-status-derived-on-read.md.
type OrganizationLicenseRead struct {
	OrganizationLicense
	// Usage holds one entry per limit field the product's license schema
	// declares, keyed by field name. A field that is not a limit never
	// appears here.
	Usage map[string]FieldUsage
}

// DeriveUsage computes the Usage entries a license read carries: one per limit
// field the schema declares, given the license's own values and the latest
// observation on record for each key.
//
// It is pure — schema fields, license values, latest observations and the
// clock all arrive as arguments — so status derivation is unit-tested without
// a database. now is a parameter rather than read from the clock so a test can
// pin the boundary between "reported recently" and "stale" exactly.
func DeriveUsage(
	fields []Field, values TemplateValues, latestByKey map[string]UsageObservation, now time.Time,
) map[string]FieldUsage {
	usage := make(map[string]FieldUsage)
	for _, field := range fields {
		if field.Type != rules.Limit {
			continue
		}

		// A limit field's value is validated as numeric by rules.ValidateValue
		// before it is ever stored, so this is unreachable in practice — a
		// defensive skip rather than a failure of the whole read.
		limit, ok := asFloat(values[field.Name])
		if !ok {
			continue
		}

		var latest *UsageObservation
		if observation, found := latestByKey[field.Name]; found {
			latest = &observation
		}

		usage[field.Name] = deriveFieldUsage(field.Name, limit, latest, field.ExpectedReportingInterval, now)
	}
	return usage
}

// deriveFieldUsage is DeriveUsage's per-field rule.
//
// A limit that has never reported is stale: there is no current number to
// trust, which is the strongest form of staleness there is. A limit whose
// latest observation is older than its declared expected reporting interval is
// stale for the same reason — and only reachable when that interval is
// declared, since Anchor declares what it expects and never pulls a report on
// its own. Otherwise the latest observation decides: past the limit is
// exceeded, equal to it is at_limit, under it is within_limit.
func deriveFieldUsage(
	field string, limit float64, latest *UsageObservation,
	expectedReportingInterval *time.Duration, now time.Time,
) FieldUsage {
	result := FieldUsage{Field: field, Limit: limit, Status: UsageStale}
	if latest == nil {
		return result
	}

	usage := latest.Value
	result.Usage = &usage
	lastReportedAt := latest.ObservedAt
	result.LastReportedAt = &lastReportedAt

	if expectedReportingInterval != nil && now.Sub(latest.ObservedAt) > *expectedReportingInterval {
		return result
	}

	switch {
	case usage > limit:
		result.Status = UsageExceeded
	case usage == limit:
		result.Status = UsageAtLimit
	default:
		result.Status = UsageWithinLimit
	}
	return result
}
