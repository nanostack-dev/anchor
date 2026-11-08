//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

const goldenDir = "testdata/golden"

// capturedHeaders are the response headers a golden records with their value.
// Every other header is recorded by name only, so a new header still shows up.
var capturedHeaders = []string{
	"Access-Control-Allow-Credentials",
	"Access-Control-Allow-Origin",
	"Allow",
	"Cache-Control",
	"Content-Type",
	"Deprecation",
	"Link",
	"Location",
	"Retry-After",
	"Set-Cookie",
	"Vary",
	"Www-Authenticate",
	"X-Content-Type-Options",
	"X-Request-Id",
}

// unstableHeaders vary between runs for the same response and are left out of
// the header-name list (Content-Length follows body length; chunking follows
// size).
var unstableHeaders = []string{"Content-Length", "Date", "Transfer-Encoding"}

var (
	ksuidIDPattern  = regexp.MustCompile(`\b([a-z][a-z0-9]*(?:_[a-z0-9]+)*)_[0-9A-Za-z]{27}\b`)
	timestampRegexp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	uuidPattern     = regexp.MustCompile(
		`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
	)
	jwtPattern      = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	cookieToken     = regexp.MustCompile(`(refresh_token=)[^;]+`)
	callerPattern   = regexp.MustCompile(`^[^:\s]+\.go:\d+$`)
	hexPattern      = regexp.MustCompile(`^[0-9a-f]+$`)
	summaryDuration = regexp.MustCompile(`\(([0-9][^()]*)\)$`)
	httpDatePattern = regexp.MustCompile(
		`(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT`,
	)
	obfuscatedKey = regexp.MustCompile(`(\*\*\*_)[0-9a-f]{8}\b`)
)

// epochHTTPDate is the Expires value of a cleared cookie; it is part of the
// contract (the gate asserts it) and is never normalised.
const epochHTTPDate = "Thu, 01 Jan 1970 00:00:00 GMT"

// normalizer replaces run-specific values (ids, tokens, run ids, timestamps)
// with stable placeholders. Registered values win over the regex fallbacks.
type normalizer struct {
	values []replacement
}

type replacement struct {
	value       string
	placeholder string
}

// register makes every later occurrence of value render as placeholder.
// Short or numeric values are ignored so a count or a common word is never
// rewritten by accident.
func (n *normalizer) register(value, placeholder string) {
	const minLen = 6
	value = strings.TrimSpace(value)
	if len(value) < minLen || isNumeric(value) || value == placeholder {
		return
	}
	// Two JWTs minted in the same second for the same subject are identical
	// (no jti), so a captured token can reappear in another response by
	// timing alone. JWTs always render as <jwt>.
	if jwtPattern.MatchString(value) {
		return
	}
	for _, existing := range n.values {
		if existing.value == value {
			return
		}
	}
	n.values = append(n.values, replacement{value: value, placeholder: placeholder})
	sort.SliceStable(n.values, func(i, j int) bool { return len(n.values[i].value) > len(n.values[j].value) })
}

func (n *normalizer) string(s string) string {
	for _, r := range n.values {
		s = strings.ReplaceAll(s, r.value, r.placeholder)
	}
	s = jwtPattern.ReplaceAllString(s, "<jwt>")
	s = httpDatePattern.ReplaceAllStringFunc(s, func(m string) string {
		if m == epochHTTPDate {
			return m
		}
		return "<http-date>"
	})
	s = obfuscatedKey.ReplaceAllString(s, "${1}<hex>")
	s = timestampRegexp.ReplaceAllString(s, "<timestamp>")
	s = uuidPattern.ReplaceAllString(s, "<uuid>")
	s = ksuidIDPattern.ReplaceAllString(s, "${1}_<id>")
	return s
}

func isNumeric(s string) bool {
	_, err := json.Number(s).Float64()
	return err == nil
}

// exchange is one request and its observed response and log lines.
type exchange struct {
	method      string
	path        string
	reqHeaders  map[string]string
	reqBody     string
	resp        *response
	logs        []map[string]any
	requestID   string
	description string
}

// render turns the exchange into the golden text.
func (n *normalizer) render(ex exchange) string {
	var b strings.Builder
	if ex.description != "" {
		fmt.Fprintf(&b, "# %s\n", ex.description)
	}
	fmt.Fprintf(&b, "request: %s %s\n", ex.method, n.string(ex.path))
	names := make([]string, 0, len(ex.reqHeaders))
	for name := range ex.reqHeaders {
		names = append(names, http.CanonicalHeaderKey(name))
	}
	slices.Sort(names)
	fmt.Fprintf(&b, "request-headers: %s\n", strings.Join(names, ", "))
	if ex.reqBody != "" {
		fmt.Fprintf(&b, "request-body: %s\n", n.string(ex.reqBody))
	}
	fmt.Fprintf(&b, "\nstatus: %d\n", ex.resp.status)
	for _, name := range capturedHeaders {
		for _, value := range ex.resp.header.Values(name) {
			value = cookieToken.ReplaceAllStringFunc(value, func(m string) string {
				if m == "refresh_token=" {
					return m
				}
				return "refresh_token=<token>"
			})
			fmt.Fprintf(&b, "header %s: %s\n", name, n.string(value))
		}
	}
	present := make([]string, 0, len(ex.resp.header))
	for name := range ex.resp.header {
		if !slices.Contains(unstableHeaders, name) {
			present = append(present, name)
		}
	}
	slices.Sort(present)
	fmt.Fprintf(&b, "header-names: %s\n", strings.Join(present, ", "))
	b.WriteString("\nbody:\n")
	b.WriteString(n.string(string(ex.resp.body)))
	if len(ex.resp.body) > 0 && !bytes.HasSuffix(ex.resp.body, []byte("\n")) {
		b.WriteString("\n<no trailing newline>\n")
	}
	fmt.Fprintf(&b, "\nlogs: %d line(s)\n", len(ex.logs))
	for _, line := range ex.logs {
		b.WriteString(n.logLine(line))
		b.WriteString("\n")
	}
	return b.String()
}

// logLine normalises one JSON log line into a stable, key-sorted JSON object.
// Values that change per run keep their type and format: caller stays
// file:line, duration keeps its unit, ids keep their prefix.
func (n *normalizer) logLine(line map[string]any) string {
	out := make(map[string]any, len(line))
	message, _ := line["message"].(string)
	for key, value := range line {
		switch key {
		case "time":
			out[key] = typeMarker(value, "<time>")
		case "request_body":
			// Present only because the harness sets LOG_REQUEST_BODY=true, and
			// it would carry the passwords of login and register bodies.
			out[key] = "<present>"
		case "caller":
			out[key] = callerMarker(value)
		case "duration":
			out[key] = durationMarker(value, message)
		case "client_ip":
			out[key] = typeMarker(value, "<ip>")
		case "bytes_out":
			out[key] = typeMarker(value, "<int>")
		case "trace_id", "span_id":
			s, _ := value.(string)
			if hexPattern.MatchString(s) {
				out[key] = fmt.Sprintf("<hex%d>", len(s))
			} else {
				out[key] = n.value(value)
			}
		case "message":
			out[key] = n.string(summaryDuration.ReplaceAllString(message, "(<duration>)"))
		default:
			out[key] = n.value(value)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Sprintf("<unencodable log line: %v>", err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func (n *normalizer) value(v any) any {
	switch t := v.(type) {
	case string:
		return n.string(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = n.value(item)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = n.value(item)
		}
		return out
	default:
		return v
	}
}

// typeMarker keeps the JSON type of a run-specific value.
func typeMarker(v any, marker string) string {
	switch v.(type) {
	case string:
		return marker
	case json.Number:
		return marker + ":number"
	default:
		return fmt.Sprintf("%s:%T", marker, v)
	}
}

func callerMarker(v any) string {
	s, ok := v.(string)
	if !ok {
		return typeMarker(v, "<caller-not-a-string>")
	}
	if callerPattern.MatchString(s) {
		return "<file>.go:<line>"
	}
	return "<caller-unexpected-format>"
}

// durationMarker records the unit of the duration field, read against the
// rounded duration printed in the summary message. The slow-request alert
// filters duration:>1000 and so depends on milliseconds.
func durationMarker(v any, message string) string {
	num, ok := v.(json.Number)
	if !ok {
		return typeMarker(v, "<duration>")
	}
	value, err := num.Float64()
	if err != nil {
		return "<duration:not-a-number>"
	}
	match := summaryDuration.FindStringSubmatch(message)
	if match == nil {
		return "<duration:number>"
	}
	printed, err := time.ParseDuration(match[1])
	if err != nil {
		return "<duration:number>"
	}
	const tolerance = float64(time.Microsecond)
	ns := float64(printed.Nanoseconds())
	units := []struct {
		name  string
		scale float64
	}{
		{"ms", float64(time.Millisecond)},
		{"ns", 1},
		{"us", float64(time.Microsecond)},
		{"s", float64(time.Second)},
	}
	for _, unit := range units {
		if math.Abs(value*unit.scale-ns) <= tolerance {
			return "<duration:number:" + unit.name + ">"
		}
	}
	return "<duration:number:unknown-unit>"
}

// checkGolden compares got with the golden file at rel (under testdata/golden),
// or rewrites it with -update.
func checkGolden(t *testing.T, rel, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, rel)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("golden %s missing (run with -update to create it): %v\n--- got ---\n%s", rel, err, got)
		return
	}
	if string(want) != got {
		t.Errorf("golden %s differs (run with -update to accept):\n%s", rel, lineDiff(string(want), got))
	}
}

// lineDiff lists the lines that differ, with their line numbers.
func lineDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	for i := range max(len(wantLines), len(gotLines)) {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			fmt.Fprintf(&b, "line %d\n  - %s\n  + %s\n", i+1, w, g)
		}
	}
	return b.String()
}
