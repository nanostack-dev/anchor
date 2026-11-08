//go:build contract

package contract_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

const gateFlowDir = "testdata/gateflows"

// gateSkips lists the gate-flow nodes the in-process suite cannot reproduce,
// keyed "<flow-slug>/<node-id>" (or "<flow-slug>/*" for a whole flow), with
// the reason. Their assertion failures are logged, not failed, and README.md
// lists them as not covered.
var gateSkips = map[string]string{
	"email-templates/preview-optional-missing": "needs the Echopoint workspace variable emailOptionalVarBody, " +
		"which the flow export does not carry",
	"email-templates/preview-optional-given": "needs the Echopoint workspace variable emailOptionalVarBody, " +
		"which the flow export does not carry",
}

// echopointEnv seeds, once per process, the stand-in for the Echopoint dev data
// the access-evaluation flows read (testdata/fixtures/echopoint-dev-product.json).
var echopointEnv struct {
	once   sync.Once
	inputs map[string]string
	err    error
}

func echopointInputs(t *testing.T) (map[string]string, error) {
	t.Helper()
	echopointEnv.once.Do(func() {
		flow, err := loadFlow(filepath.Join("testdata", "fixtures", "echopoint-dev-product.json"))
		if err != nil {
			echopointEnv.err = err
			return
		}
		run := newFlowRun(t, flow, "fixture-echopoint-dev-product")
		run.run(flow.Definition)
		if len(run.failures) > 0 {
			echopointEnv.err = fmt.Errorf("echopoint fixture: %s", strings.Join(run.failures, "; "))
			return
		}
		value := func(node, output string) string {
			outputs, _ := run.scope[node].(map[string]any)
			s, _ := outputs[output].(string)
			return s
		}
		echopointEnv.inputs = map[string]string{
			"nanostackProductId":   value("create-product", "productId"),
			"echopointOrgId":       value("create-org", "orgId"),
			"echopointWorkspaceId": value("create-ws", "wsId"),
		}
	})
	return echopointEnv.inputs, echopointEnv.err
}

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
			if raw, _ := os.ReadFile(path); strings.Contains(string(raw), "nanostackProductId") {
				inputs, envErr := echopointInputs(t)
				if envErr != nil {
					t.Fatal(envErr)
				}
				for name, value := range inputs {
					run.inputs[name] = value
					run.norm.register(value, "{{"+name+"}}")
				}
			}
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
