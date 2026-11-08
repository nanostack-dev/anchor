//go:build contract

package contract_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const gateFlowDir = "testdata/gateflows"

// gateSkips lists the gate-flow nodes the in-process suite cannot reproduce,
// keyed "<flow-slug>/<node-id>" (or "<flow-slug>/*" for a whole flow), with
// the reason. Their assertion failures are logged, not failed, and README.md
// lists them as not covered.
var gateSkips = map[string]string{
	"access-evaluation/beta":                             envFixtureReason,
	"access-evaluation/ep-key":                           envFixtureReason,
	"access-evaluation-user-writes-apply-at-once/beta":   envFixtureReason,
	"access-evaluation-user-writes-apply-at-once/ep-key": envFixtureReason,
	"email-templates/preview-optional-missing": "needs the Echopoint workspace variable emailOptionalVarBody, " +
		"which the flow export does not carry",
	"email-templates/preview-optional-given": "needs the Echopoint workspace variable emailOptionalVarBody, " +
		"which the flow export does not carry",
}

const envFixtureReason = "needs Echopoint's own dev product, organization, member and workspace " +
	"(nanostackProductId, echopointOrgId, echopointWorkspaceId); every node after it is skipped"

func skipReason(slug, failure string) (string, bool) {
	if reason, ok := gateSkips[slug+"/*"]; ok {
		return reason, true
	}
	for key, reason := range gateSkips {
		flow, node, _ := strings.Cut(key, "/")
		if flow == slug && strings.Contains(failure, "["+node+"]") {
			return reason, true
		}
	}
	return "", false
}

func gateFlowPaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(gateFlowDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	return paths
}

// TestGateFlows replays every production gate flow of the Anchor workspace
// against the in-process server: every assertion must hold, and every refusal
// (status >= 400, error envelope, asserted header) must match its golden.
func TestGateFlows(t *testing.T) {
	for _, path := range gateFlowPaths(t) {
		slug := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(slug, func(t *testing.T) {
			flow, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			run := newFlowRun(t, flow, slug)
			run.run(flow.Definition)
			for _, failure := range run.failures {
				if reason, skipped := skipReason(slug, failure); skipped {
					t.Logf("skipped (%s): %s", reason, failure)
					continue
				}
				t.Error(failure)
			}
		})
	}
}
