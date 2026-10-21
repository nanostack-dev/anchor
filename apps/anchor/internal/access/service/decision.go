package service

import "slices"

// reach is how far a subject got toward the resource: its identity there, or
// the reason it stopped. A stopped subject is denied every action.
type reach struct {
	identity *Identity
	stopped  DenyReason
}

func reached(identity *Identity) reach { return reach{identity: identity} }

func stoppedBy(reason DenyReason) reach { return reach{stopped: reason} }

func (r reach) decide(actions []string, semantic Semantic) Decision {
	if r.stopped != "" {
		return denyAll(actions, r.stopped)
	}
	return r.identity.decide(actions, semantic)
}

func denyAll(actions []string, reason DenyReason) Decision {
	decision := Decision{Reason: reason, Items: []ItemDecision{}}
	for range actions {
		decision.Items = append(decision.Items, ItemDecision{Reason: reason})
	}
	return decision
}

// decide checks each action against the permissions the identity holds.
func (identity *Identity) decide(actions []string, semantic Semantic) Decision {
	decision := Decision{Allowed: true, Identity: identity, Items: []ItemDecision{}}
	skipRest := false
	for _, action := range actions {
		item := ItemDecision{Reason: ReasonNotEvaluated}
		if !skipRest {
			item = identity.check(action)
			skipRest = semantic.stopsAfter(item)
		}
		decision.record(action, item)
	}
	return decision
}

func (identity *Identity) check(action string) ItemDecision {
	if slices.Contains(identity.Permissions, action) {
		return ItemDecision{Allowed: true}
	}
	return ItemDecision{Reason: ReasonMissingPermission}
}

func (d *Decision) record(action string, item ItemDecision) {
	d.Items = append(d.Items, item)
	if item.Allowed {
		return
	}
	d.Allowed = false
	if item.Reason == ReasonMissingPermission {
		d.MissingPermissions = append(d.MissingPermissions, action)
		d.Reason = ReasonMissingPermission
	} else if d.Reason == "" {
		// Only skipped items so far: permit_on_first_permit stopped early.
		d.Reason = ReasonNotEvaluated
	}
}
