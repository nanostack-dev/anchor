//go:build contract

package contract_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"anchor/internal/domain/session"
)

// A gate flow waits for a server-side duration to run out with a fixed delay
// node sized for production: 61 s for a 60 s expiry, 12 s for the 10 s
// refresh-token reuse grace. In-process the suite makes that duration short
// and waits only until it has run out, plus the margin the flow keeps (the
// delay minus the duration). The server sees the same requests in the same
// order, and the flow observes the same state before and after the wait.
const (
	// shortExpiryLead is how far ahead a shortened expiry lies when the flow
	// first uses it: whole seconds, so 2-3 s from the request that sets it,
	// far longer than the create request that asserts the resource is still
	// pending or active.
	shortExpiryLead = 3 * time.Second
	// contractReuseGrace replaces session.ReuseGrace in this process
	// (main_test.go).
	contractReuseGrace = time.Second
)

// shortWait says what a gate delay node waits out.
type shortWait struct {
	// expiry is the {{$isoTimestamp:<offset>}} argument of the expiry the
	// delay waits out; in this flow it resolves to a short expiry.
	expiry string
	// reuseGrace marks a delay that waits out session.ReuseGrace.
	reuseGrace bool
}

// gateShortWaits lists the delay nodes that wait out a server-side duration,
// keyed "<flow-slug>/<node-id>". Every other delay node sleeps its duration
// (product-event-delivery/settle: 1 s for an asynchronous write).
var gateShortWaits = map[string]shortWait{
	// invite-x is created with expires_at {{$isoTimestamp:+60s}} (status
	// pending), then read, searched, accepted and updated as expired.
	"invitations/wait-x-expiry": {expiry: "+60s"},
	// create-expiring-key gets expires_at {{$isoTimestamp:+60s}} (ACTIVE), then
	// validate, introspect, reactivate and get see it expired and INACTIVE.
	"organization-api-key/wait-expiry": {expiry: "+60s"},
	// refresh-rotates rotates the login token; replay-rotated-token presents
	// it again past the grace and revokes the session.
	"auth/wait-past-reuse-grace": {reuseGrace: true},
}

// registerShortExpiries makes the flow's dynamic variables resolve the
// expiries its delay nodes wait out to a short expiry.
func registerShortExpiries(slug string, dyn *dynamicVars) {
	for key, wait := range gateShortWaits {
		if flow, _, _ := strings.Cut(key, "/"); flow == slug && wait.expiry != "" {
			dyn.shortExpiries[wait.expiry] = time.Time{}
		}
	}
}

// checkShortWaits fails when gateShortWaits names a node of this flow that is
// not a delay node outlasting the duration it waits out, so a re-exported flow
// cannot silently change what a shortened wait stands for.
func checkShortWaits(t *testing.T, slug string, flow *flowFile) {
	t.Helper()
	for key, wait := range gateShortWaits {
		flowSlug, nodeID, _ := strings.Cut(key, "/")
		if flowSlug != slug {
			continue
		}
		var delay time.Duration
		found := false
		for _, node := range flow.Definition.Nodes {
			if node.ID == nodeID && node.Type == "delay" {
				delay, found = delayDuration(node), true
			}
		}
		if !found {
			t.Fatalf("gateShortWaits: %s is not a delay node of the flow", key)
		}
		if _, err := wait.margin(delay); err != nil {
			t.Fatalf("gateShortWaits: %s: %v", key, err)
		}
	}
}

func delayDuration(node flowNode) time.Duration {
	var data struct {
		Duration int `json:"duration"`
	}
	_ = json.Unmarshal(node.Data, &data)
	return time.Duration(data.Duration) * time.Millisecond
}

// margin is how long the flow's delay outlasts the production duration it
// waits out.
func (w shortWait) margin(delay time.Duration) (time.Duration, error) {
	waited := session.ReuseGrace
	if w.expiry != "" {
		offset, err := time.ParseDuration(strings.TrimPrefix(w.expiry, "+"))
		if err != nil {
			return 0, err
		}
		waited = offset
	}
	if delay <= waited {
		return 0, fmt.Errorf("the %s delay does not outlast the %s it waits out", delay, waited)
	}
	return delay - waited, nil
}

// executeDelay sleeps a delay node: its fixed duration, or for a node in
// gateShortWaits until the shortened duration has run out plus the flow's
// margin.
func (r *flowRun) executeDelay(node *flowNode, delay time.Duration) nodeState {
	wait, ok := gateShortWaits[r.prefix+"/"+node.ID]
	if !ok {
		time.Sleep(delay)
		return statePassed
	}
	margin, err := wait.margin(delay)
	if err != nil {
		return r.fail(node, err.Error())
	}
	until := time.Now().Add(contractReuseGrace + margin)
	if wait.expiry != "" {
		expiresAt := r.dyn.shortExpiries[wait.expiry]
		if expiresAt.IsZero() {
			return r.fail(node, fmt.Sprintf("no request used {{$isoTimestamp:%s}} before the delay", wait.expiry))
		}
		until = expiresAt.Add(margin)
	}
	time.Sleep(time.Until(until))
	return statePassed
}
