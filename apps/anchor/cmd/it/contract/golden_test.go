//go:build contract

package contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
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

// capturedHeaders are the response headers a golden records first, in this
// order. Every other header except unstableHeaders is recorded with its value
// after them, sorted by name, so a new header or a new value always shows up.
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
	timestampRegexp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
	uuidPattern     = regexp.MustCompile(
		`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
	)
	jwtPattern      = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	cookieToken     = regexp.MustCompile(`(refresh_token=)[^;]+`)
	callerPattern   = regexp.MustCompile(`^[^:\s]+\.go:\d+$`)
	unixSeconds     = regexp.MustCompile(`^\d{10}$`)
	webhookSig      = regexp.MustCompile(`\b(v1,)[A-Za-z0-9+/]+={0,2}`)
	hexPattern      = regexp.MustCompile(`^[0-9a-f]+$`)
	summaryDuration = regexp.MustCompile(`\(([0-9][^()]*)\)$`)
	httpDatePattern = regexp.MustCompile(
		`(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT`,
	)
	obfuscatedKey = regexp.MustCompile(`(\*\*\*_)[0-9a-f]{8}\b`)
	// One-time API key values (<prefix>_<kind>_apikey_<secret>[_<checksum>]):
	// the prefix, the kind and the checksum's presence stay.
	apiKeySecret = regexp.MustCompile(`\b([a-z0-9]+_(?:org|prd)_apikey_)[A-Za-z0-9]{16,}(_[0-9a-f]{8})?\b`)
	// Webhook signing secrets returned once at creation.
	signingSecret = regexp.MustCompile(`\b(whsec_)[A-Za-z0-9+/]{20,}={0,2}`)
	// The SMTP integration's connection check runs in the background against
	// smtp.example.com, so a read shows either the not-configured text or the
	// connection failure, depending on timing and on the network.
	smtpCheckError = regexp.MustCompile(`"last_error":"(?:Not configured yet\.[^"]*|smtp: [^"]*)"`)
	// Opaque pagination cursors are base64url JSON (ids and sort values).
	cursorPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}={0,2}`)
	// Random 128-bit hex tokens (invitation codes).
	hex32Pattern = regexp.MustCompile(`\b[0-9a-f]{32}\b`)
	// Masked secrets keep their prefix and the number of stars.
	maskedTail = regexp.MustCompile(`(\*{6,})[A-Za-z0-9+/=]+`)
	// A timestamp sent URL-encoded in a query string.
	urlTimestamp = regexp.MustCompile(
		`\d{4}-\d{2}-\d{2}T\d{2}%3A\d{2}%3A\d{2}(?:\.\d+)?(?:Z|%2B\d{2}%3A\d{2})`,
	)
	// The product webhook's events list is built from a Go map, so its order
	// changes between calls; it is sorted before comparison.
	eventsArray = regexp.MustCompile(`"events":\[((?:"[^"]*",?)*)\]`)
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
	s = cursorPattern.ReplaceAllString(s, "<cursor>")
	s = apiKeySecret.ReplaceAllStringFunc(s, func(m string) string {
		sub := apiKeySecret.FindStringSubmatch(m)
		if sub[2] != "" {
			return sub[1] + "<secret>_<hex8>"
		}
		return sub[1] + "<secret>"
	})
	s = hex32Pattern.ReplaceAllString(s, "<hex32>")
	s = signingSecret.ReplaceAllString(s, "${1}<secret>")
	s = smtpCheckError.ReplaceAllString(s, `"last_error":"<smtp check: not configured or connection failed>"`)
	s = maskedTail.ReplaceAllString(s, "${1}<tail>")
	s = urlTimestamp.ReplaceAllString(s, "<timestamp-urlencoded>")
	s = eventsArray.ReplaceAllStringFunc(s, sortEventsArray)
	s = httpDatePattern.ReplaceAllStringFunc(s, func(m string) string {
		if m == epochHTTPDate {
			return m
		}
		return "<http-date>"
	})
	s = obfuscatedKey.ReplaceAllString(s, "${1}<hex>")
	s = timestampRegexp.ReplaceAllStringFunc(s, timestampShape)
	s = uuidPattern.ReplaceAllString(s, "<uuid>")
	s = ksuidIDPattern.ReplaceAllString(s, "${1}_<id>")
	return s
}

// timestampShape keeps what a client could parse differently: whether the
// value carries a fractional second and how the zone is written. The digits of
// the fraction are dropped: Go trims trailing zeros, so their count varies by
// run (a fraction of exactly zero, about one value in a million, prints none).
func timestampShape(ts string) string {
	m := timestampRegexp.FindStringSubmatch(ts)
	if m == nil {
		return "<timestamp>"
	}
	shape := "<timestamp"
	if m[1] != "" {
		shape += ".frac"
	}
	switch m[2] {
	case "Z":
	case "+00:00":
		shape += "+00:00"
	default:
		shape += "+offset"
	}
	return shape + ">"
}

func sortEventsArray(m string) string {
	inner := eventsArray.FindStringSubmatch(m)[1]
	if inner == "" {
		return m
	}
	items := strings.Split(inner, ",")
	slices.Sort(items)
	return `"events":[` + strings.Join(items, ",") + "]"
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
	// bodyLen is the length of the response body as sent, which bytes_out
	// in the summary log line must equal (resp.body may hold a digest).
	bodyLen int
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
	present := make([]string, 0, len(ex.resp.header))
	for name := range ex.resp.header {
		if !slices.Contains(unstableHeaders, name) {
			present = append(present, name)
		}
	}
	slices.Sort(present)
	ordered := slices.Clone(capturedHeaders)
	for _, name := range present {
		if !slices.Contains(ordered, name) {
			ordered = append(ordered, name)
		}
	}
	for _, name := range ordered {
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
	fmt.Fprintf(&b, "header-names: %s\n", strings.Join(present, ", "))
	b.WriteString("\nbody:\n")
	b.WriteString(n.string(string(ex.resp.body)))
	if len(ex.resp.body) > 0 && !bytes.HasSuffix(ex.resp.body, []byte("\n")) {
		b.WriteString("\n<no trailing newline>\n")
	}
	fmt.Fprintf(&b, "\nlogs: %d line(s)\n", len(ex.logs))
	for _, line := range ex.logs {
		b.WriteString(n.logLine(line, ex.bodyLen))
		b.WriteString("\n")
	}
	return b.String()
}

// logLine normalises one JSON log line into a stable, key-sorted JSON object.
// Values that change per run keep their type and format: caller stays
// file:line, duration keeps its unit, ids keep their prefix.
func (n *normalizer) logLine(line map[string]any, bodyLen int) string {
	out := make(map[string]any, len(line))
	message, _ := line["message"].(string)
	for key, value := range line {
		switch key {
		case "time":
			out[key] = timeMarker(value)
		case "request_body":
			// Present only because the harness sets LOG_REQUEST_BODY=true, and
			// it would carry the passwords of login and register bodies.
			out[key] = "<present>"
		case "caller":
			out[key] = callerMarker(value)
		case "duration":
			out[key] = durationMarker(value, message)
		case "client_ip":
			out[key] = ipMarker(value)
		case "bytes_out":
			out[key] = bytesOutMarker(value, bodyLen)
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
	if !callerPattern.MatchString(s) {
		return "<caller-unexpected-format>"
	}
	if strings.HasPrefix(s, "/") {
		return "/<path>/<file>.go:<line>"
	}
	return "<file>.go:<line>"
}

// timeMarker keeps the type and the layout of the log time field: a switch
// from RFC 3339 to a Unix number, or to fractional seconds, shows up.
func timeMarker(v any) string {
	s, ok := v.(string)
	if !ok {
		return typeMarker(v, "<time>")
	}
	if loc := timestampRegexp.FindStringIndex(s); loc != nil && loc[0] == 0 && loc[1] == len(s) {
		return "<time:" + strings.Trim(timestampShape(s), "<>") + ">"
	}
	return "<time:unexpected-format>"
}

// ipMarker keeps whether client_ip is a bare address, an address with a
// port, or something else.
func ipMarker(v any) string {
	s, ok := v.(string)
	if !ok {
		return typeMarker(v, "<ip>")
	}
	if net.ParseIP(s) != nil {
		return "<ip>"
	}
	if host, _, err := net.SplitHostPort(s); err == nil && net.ParseIP(host) != nil {
		return "<ip:port>"
	}
	return "<ip:unexpected-format>"
}

// bytesOutMarker checks bytes_out against the length of the body the client
// received. The value itself varies with the length of run-specific ids and
// timestamps.
func bytesOutMarker(v any, bodyLen int) string {
	num, ok := v.(json.Number)
	if !ok {
		return typeMarker(v, "<int>")
	}
	got, err := num.Int64()
	if err != nil {
		return "<int:not-an-integer>"
	}
	if got == int64(bodyLen) {
		return "<int:=body-length>"
	}
	return fmt.Sprintf("<int:%d, body is %d bytes>", got, bodyLen)
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
