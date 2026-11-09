//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
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

// bodyMode says how a gate node's response body is recorded when its bytes
// are not stable between runs for a reason that is not a contract change.
type bodyMode struct {
	mode   string // bodyUnordered or bodyShape
	reason string
}

const (
	// bodyUnordered re-encodes the JSON body with object keys sorted and
	// every array sorted, so item order does not count.
	bodyUnordered = "unordered"
	// bodyShape records only the JSON shape: keys, value types and array
	// lengths.
	bodyShape = "shape"
)

const nameTieReason = "every item has the same name, so the order (and the page an item lands on) " +
	"follows the random id tie-break"

const catalogReason = "the catalog is built from a Go map"

const orgLicenseTieReason = "the licences belong to organizations that share one name, so the default " +
	"order follows the random id tie-break"

// gateBodyModes lists the gate nodes, keyed "<flow-slug>/<node-id>", whose
// body is recorded with a bodyMode instead of byte for byte. Status, headers
// and log lines stay exact.
var gateBodyModes = map[string]bodyMode{
	"product-event-delivery/catalog":                                     {bodyUnordered, catalogReason},
	"cursor-searches-email-licensing-integrations/templates-by-name-ref": {bodyShape, nameTieReason},
	"cursor-searches-email-licensing-integrations/templates-by-name-p1":  {bodyShape, nameTieReason},
	"cursor-searches-email-licensing-integrations/templates-by-name-p2":  {bodyShape, nameTieReason},
	"cursor-searches-email-licensing-integrations/org-licenses-ref":      {bodyShape, orgLicenseTieReason},
	"cursor-searches-email-licensing-integrations/org-licenses-p1":       {bodyShape, orgLicenseTieReason},
	"cursor-searches-email-licensing-integrations/org-licenses-p2":       {bodyShape, orgLicenseTieReason},
	"cursor-searches-organizations-members/orgs-by-name-ref":             {bodyShape, nameTieReason},
	"cursor-searches-organizations-members/orgs-by-name-p1":              {bodyShape, nameTieReason},
	"cursor-searches-organizations-members/orgs-by-name-p2":              {bodyShape, nameTieReason},
	"cursor-searches-organizations-members/orgs-by-name-p3":              {bodyShape, nameTieReason},
}

// applyBodyMode returns the body to record for a node with a bodyMode.
func applyBodyMode(mode bodyMode, body []byte) []byte {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var v any
	if err := decoder.Decode(&v); err != nil {
		return fmt.Appendf(nil, "<%s body is not JSON: %v>\n%s", mode.mode, err, body)
	}
	if mode.mode == bodyShape {
		v = jsonShape(v)
	} else {
		v = sortArrays(v)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return fmt.Appendf(nil, "<%s body: %v>", mode.mode, err)
	}
	return fmt.Appendf(nil, "<%s: %s>\n%s\n", mode.mode, mode.reason, out)
}

func sortArrays(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, item := range t {
			t[k] = sortArrays(item)
		}
		return t
	case []any:
		keys := make([]string, len(t))
		for i, item := range t {
			t[i] = sortArrays(item)
			raw, _ := json.Marshal(t[i])
			keys[i] = string(raw)
		}
		idx := make([]int, len(t))
		for i := range idx {
			idx[i] = i
		}
		slices.SortStableFunc(idx, func(a, b int) int { return strings.Compare(keys[a], keys[b]) })
		out := make([]any, len(t))
		for i, j := range idx {
			out[i] = t[j]
		}
		return out
	default:
		return v
	}
}

func jsonShape(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = jsonShape(item)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = jsonShape(item)
		}
		return out
	case string:
		return "<string>"
	case json.Number:
		return "<number>"
	case bool:
		return "<bool>"
	default:
		return v
	}
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
			if reason := backendDown(); reason != "" {
				t.Fatal("not run: " + reason)
			}
			flow, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			checkShortWaits(t, slug, flow)
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
