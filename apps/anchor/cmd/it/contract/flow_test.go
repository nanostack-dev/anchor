//go:build contract

package contract_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The gate flows are exported Echopoint flow definitions (see
// testdata/gateflows/MANIFEST.md). The interpreter below runs them the way
// echopoint-runner does, sequentially and in a stable order, so the goldens do
// not depend on scheduling.

type flowFile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Definition  flowDef  `json:"flow_definition"`
}

type flowDef struct {
	Nodes []flowNode `json:"nodes"`
	Edges []flowEdge `json:"edges"`
}

type flowEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type flowNode struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	RunWhen     string          `json:"run_when"`
	DisplayName string          `json:"display_name"`
	Data        json.RawMessage `json:"data"`
	Assertions  []assertion     `json:"assertions"`
	Outputs     []flowOutput    `json:"outputs"`
}

type assertion struct {
	ExtractorType string `json:"extractor_type"`
	ExtractorData struct {
		Path       string `json:"path"`
		HeaderName string `json:"header_name"`
	} `json:"extractor_data"`
	OperatorType string `json:"operator_type"`
	OperatorData struct {
		Value any `json:"value"`
	} `json:"operator_data"`
}

type flowOutput struct {
	Name      string `json:"name"`
	Extractor struct {
		Type string `json:"type"`
		Path string `json:"path"`
	} `json:"extractor"`
}

type requestData struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

type moduleData struct {
	FlowID         string            `json:"flow_id"`
	OutputBindings map[string]string `json:"output_bindings"`
	Inputs         map[string]any    `json:"inputs"`
}

type loopData struct {
	Body struct {
		Nodes []flowNode `json:"nodes"`
		Edges []flowEdge `json:"edges"`
	} `json:"body"`
	Items           any    `json:"items"`
	ItemVar         string `json:"item_var"`
	IndexVar        string `json:"index_var"`
	MaxIterations   int    `json:"max_iterations"`
	ContinueOnError bool   `json:"continue_on_error"`
}

type nodeState int

const (
	statePending nodeState = iota
	statePassed
	stateFailed
	stateSkipped
)

const (
	runWhenAlways = "always"
	nodeRequest   = "request"
)

// flowRun is one execution of one flow.
type flowRun struct {
	t        *testing.T
	ctx      context.Context
	slug     string
	prefix   string // request-id and golden path prefix (module runs nest)
	flow     *flowFile
	inputs   map[string]any
	scope    map[string]any // node id or loop variable -> outputs
	norm     *normalizer
	dyn      *dynamicVars
	states   map[string]nodeState
	failures []string
	goldens  []string
	webhook  *hookInbox
}

func loadFlow(path string) (*flowFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var flow flowFile
	if unmarshalErr := json.Unmarshal(raw, &flow); unmarshalErr != nil {
		return nil, fmt.Errorf("%s: %w", path, unmarshalErr)
	}
	return &flow, nil
}

func newFlowRun(t *testing.T, flow *flowFile, slug string) *flowRun {
	t.Helper()
	dyn := newDynamicVars()
	norm := &normalizer{}
	norm.register(dyn.runID, "{{$runId}}")
	inbox := hooks.inbox(slug + "-" + dyn.runID)
	run := &flowRun{
		t:      t,
		ctx:    t.Context(),
		slug:   slug,
		prefix: slug,
		flow:   flow,
		inputs: map[string]any{
			"nanostackBaseUrl": suite.baseURL,
			"email":            ownerEmail,
			"password":         ownerPassword,
			"webhook.url":      inbox.url,
		},
		scope:   map[string]any{},
		norm:    norm,
		dyn:     dyn,
		states:  map[string]nodeState{},
		webhook: inbox,
	}
	norm.register(inbox.url, "{{webhook.url}}")
	return run
}

// run executes the flow graph: on_success nodes first, each once its
// predecessors passed; then the always (cleanup) nodes once their
// predecessors finished, in node order.
func (r *flowRun) run(def flowDef) {
	preds := map[string][]string{}
	for _, e := range def.Edges {
		preds[e.Target] = append(preds[e.Target], e.Source)
	}
	for _, n := range def.Nodes {
		r.states[n.ID] = statePending
	}
	alwaysPhase := false
	for {
		progressed := false
		for i := range def.Nodes {
			node := &def.Nodes[i]
			if r.states[node.ID] != statePending || !r.predsFinished(preds[node.ID]) {
				continue
			}
			switch {
			case node.RunWhen == runWhenAlways && !alwaysPhase:
				continue
			case node.RunWhen == runWhenAlways:
				r.states[node.ID] = r.execute(node, true)
			case r.predsPassed(preds[node.ID]):
				r.states[node.ID] = r.execute(node, false)
			default:
				r.states[node.ID] = stateSkipped
			}
			progressed = true
		}
		if progressed {
			continue
		}
		if alwaysPhase {
			return
		}
		alwaysPhase = true
	}
}

func (r *flowRun) predsFinished(ids []string) bool {
	for _, id := range ids {
		if r.states[id] == statePending {
			return false
		}
	}
	return true
}

func (r *flowRun) predsPassed(ids []string) bool {
	for _, id := range ids {
		if r.states[id] != statePassed {
			return false
		}
	}
	return true
}

func (r *flowRun) execute(node *flowNode, cleanup bool) nodeState {
	switch node.Type {
	case nodeRequest:
		return r.executeRequest(node, cleanup)
	case "module":
		return r.executeModule(node)
	case "delay":
		var data struct {
			Duration int `json:"duration"`
		}
		_ = json.Unmarshal(node.Data, &data)
		time.Sleep(time.Duration(data.Duration) * time.Millisecond)
		return statePassed
	case "set_variable":
		var data struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(node.Data, &data); err != nil {
			return r.fail(node, "set_variable data: "+err.Error())
		}
		r.scope[node.ID] = r.resolveDeep(data.Variables)
		return statePassed
	case "loop":
		return r.executeLoop(node)
	case "webhook_wait":
		return r.executeWebhookWait(node)
	default:
		return r.fail(node, "unsupported node type "+node.Type)
	}
}

func (r *flowRun) fail(node *flowNode, reason string) nodeState {
	msg := fmt.Sprintf("%s [%s] %q: %s", r.prefix, node.ID, node.DisplayName, reason)
	r.failures = append(r.failures, msg)
	return stateFailed
}

func (r *flowRun) executeRequest(node *flowNode, cleanup bool) nodeState {
	var data requestData
	if err := json.Unmarshal(node.Data, &data); err != nil {
		return r.fail(node, "request data: "+err.Error())
	}
	body := ""
	if len(data.Body) > 0 && string(data.Body) != "null" {
		var s string
		if json.Unmarshal(data.Body, &s) == nil {
			body = s
		} else {
			body = string(data.Body)
		}
	}
	url := r.resolve(data.URL)
	body = r.resolve(body)
	headers := make(map[string]string, len(data.Headers))
	for k, v := range data.Headers {
		headers[k] = r.resolve(v)
	}
	if cleanup && r.hasMissingNodeRef(data.URL+body) {
		return stateSkipped
	}
	requestID := "ct/" + r.prefix + "/" + node.ID
	method := strings.ToUpper(data.Method)
	if method == "" {
		method = "GET"
	}
	resp, err := rawDo(r.ctx, method, url, headers, body, requestID)
	if err != nil {
		return r.fail(node, "request: "+err.Error())
	}
	r.extractOutputs(node, resp)
	failed := r.checkAssertions(node.Assertions, resp)
	if goldenWorthy(node) {
		lines, logErr := suite.logs.linesFor(requestID)
		if logErr != nil {
			failed = append(failed, logErr.Error())
		}
		ex := exchange{
			method:      method,
			path:        strings.TrimPrefix(url, suite.baseURL),
			reqHeaders:  headers,
			reqBody:     body,
			resp:        resp,
			logs:        lines,
			requestID:   requestID,
			description: r.flow.Name + " / " + node.DisplayName,
		}
		rel := filepath.Join("gate", r.prefix, safeName(node.ID)+".golden")
		checkGolden(r.t, rel, r.norm.render(ex))
		r.goldens = append(r.goldens, rel)
	}
	if len(failed) > 0 {
		return r.fail(
			node,
			fmt.Sprintf("status %d: %s; body: %.400s", resp.status, strings.Join(failed, "; "), resp.body),
		)
	}
	return statePassed
}

// goldenWorthy picks the nodes whose response is part of the contract the
// suite pins: refusals (status >= 400), error envelopes and asserted headers.
func goldenWorthy(node *flowNode) bool {
	for _, a := range node.Assertions {
		switch a.ExtractorType {
		case "status_code":
			if code, err := strconv.Atoi(fmt.Sprint(a.OperatorData.Value)); err == nil && code >= 400 &&
				a.OperatorType == "equals" {
				return true
			}
		case "header":
			return true
		case "json_path":
			if strings.Contains(a.ExtractorData.Path, "errors") {
				return true
			}
		}
	}
	return false
}

func (r *flowRun) extractOutputs(node *flowNode, resp *response) {
	outputs := map[string]any{}
	if existing, ok := r.scope[node.ID].(map[string]any); ok {
		outputs = existing
	}
	for _, out := range node.Outputs {
		if out.Extractor.Type != "json_path" || !resp.isJSON {
			continue
		}
		value, err := selectPath(out.Extractor.Path, resp.parsed)
		if err != nil {
			continue
		}
		outputs[out.Name] = value
		if s, ok := value.(string); ok {
			r.norm.register(s, "{{"+node.ID+"."+out.Name+"}}")
		}
	}
	r.scope[node.ID] = outputs
}

// checkAssertions evaluates the node's assertions with the runner's
// semantics: values compare as strings, a missing path fails.
func (r *flowRun) checkAssertions(list []assertion, resp *response) []string {
	var failed []string
	for _, a := range list {
		actual, err := extract(a, resp)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s %s: %v", a.ExtractorType, a.ExtractorData.Path, err))
			continue
		}
		expected := a.OperatorData.Value
		if s, ok := expected.(string); ok {
			expected = r.resolve(s)
		}
		ok, cmpErr := compare(a.OperatorType, actual, expected)
		if cmpErr != nil || !ok {
			failed = append(failed, fmt.Sprintf("%s%s %s %v (got %.200s)", a.ExtractorType,
				pathSuffix(a), a.OperatorType, expected, toString(actual)))
		}
	}
	return failed
}

func pathSuffix(a assertion) string {
	if a.ExtractorData.Path != "" {
		return " " + a.ExtractorData.Path
	}
	if a.ExtractorData.HeaderName != "" {
		return " " + a.ExtractorData.HeaderName
	}
	return ""
}

func extract(a assertion, resp *response) (any, error) {
	switch a.ExtractorType {
	case "status_code":
		return resp.status, nil
	case "body":
		return string(resp.body), nil
	case "header":
		values := resp.header.Values(a.ExtractorData.HeaderName)
		if len(values) == 0 {
			return nil, fmt.Errorf("header %s not found", a.ExtractorData.HeaderName)
		}
		return values[0], nil
	case "json_path":
		if !resp.isJSON {
			return nil, fmt.Errorf("body is not JSON: %.120s", resp.body)
		}
		return selectPath(a.ExtractorData.Path, resp.parsed)
	default:
		return nil, fmt.Errorf("unsupported extractor %s", a.ExtractorType)
	}
}

// compare mirrors echopoint-runner pkg/operators.
func compare(op string, actual, expected any) (bool, error) {
	a, e := toString(actual), toString(expected)
	switch op {
	case "equals":
		return a == e, nil
	case "not_equals":
		return a != e, nil
	case "contains":
		return strings.Contains(a, e), nil
	case "not_contains":
		return !strings.Contains(a, e), nil
	case "starts_with":
		return strings.HasPrefix(a, e), nil
	case "ends_with":
		return strings.HasSuffix(a, e), nil
	case "regex":
		return regexp.MatchString(e, a)
	case "empty":
		return isEmpty(actual), nil
	case "not_empty":
		return !isEmpty(actual), nil
	case "greater_than", "less_than", "greater_than_or_equal", "less_than_or_equal":
		af, err1 := strconv.ParseFloat(a, 64)
		ef, err2 := strconv.ParseFloat(e, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("not numeric: %q %q", a, e)
		}
		switch op {
		case "greater_than":
			return af > ef, nil
		case "less_than":
			return af < ef, nil
		case "greater_than_or_equal":
			return af >= ef, nil
		default:
			return af <= ef, nil
		}
	default:
		return false, fmt.Errorf("unsupported operator %s", op)
	}
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return toString(v) == ""
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (r *flowRun) executeModule(node *flowNode) nodeState {
	var data moduleData
	if err := json.Unmarshal(node.Data, &data); err != nil {
		return r.fail(node, "module data: "+err.Error())
	}
	module, err := moduleByID(data.FlowID)
	if err != nil {
		return r.fail(node, err.Error())
	}
	sub := &flowRun{
		t:       r.t,
		ctx:     r.ctx,
		slug:    r.slug,
		prefix:  r.prefix + "/" + safeName(node.ID),
		flow:    module,
		inputs:  map[string]any{},
		scope:   map[string]any{},
		norm:    r.norm,
		dyn:     r.dyn,
		states:  map[string]nodeState{},
		webhook: r.webhook,
	}
	maps.Copy(sub.inputs, r.inputs)
	for k, v := range data.Inputs {
		if s, ok := v.(string); ok {
			v = r.resolve(s)
		}
		sub.inputs[k] = v
	}
	sub.run(module.Definition)
	r.goldens = append(r.goldens, sub.goldens...)
	if len(sub.failures) > 0 {
		r.failures = append(r.failures, sub.failures...)
		return stateFailed
	}
	outputs := map[string]any{}
	for name, ref := range data.OutputBindings {
		nodeID, field, _ := strings.Cut(ref, ".")
		if values, ok := sub.scope[nodeID].(map[string]any); ok {
			outputs[name] = values[field]
			if s, isString := values[field].(string); isString {
				r.norm.register(s, "{{"+node.ID+"."+name+"}}")
			}
		}
	}
	r.scope[node.ID] = outputs
	return statePassed
}

func moduleByID(id string) (*flowFile, error) {
	paths, err := filepath.Glob(filepath.Join(gateFlowDir, "modules", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		flow, loadErr := loadFlow(path)
		if loadErr != nil {
			return nil, loadErr
		}
		if flow.ID == id {
			return flow, nil
		}
	}
	return nil, fmt.Errorf("module flow %s is not in %s/modules", id, gateFlowDir)
}

func (r *flowRun) executeLoop(node *flowNode) nodeState {
	var data loopData
	if err := json.Unmarshal(node.Data, &data); err != nil {
		return r.fail(node, "loop data: "+err.Error())
	}
	items := data.Items
	if s, ok := items.(string); ok {
		items = r.resolveAny(s)
	}
	list, ok := items.([]any)
	if !ok {
		return r.fail(node, fmt.Sprintf("loop items are not a list: %T", items))
	}
	if data.MaxIterations > 0 && len(list) > data.MaxIterations {
		list = list[:data.MaxIterations]
	}
	for i, item := range list {
		r.scope[data.ItemVar] = item
		if data.IndexVar != "" {
			r.scope[data.IndexVar] = i
		}
		body := &flowRun{
			t: r.t, ctx: r.ctx, slug: r.slug, prefix: fmt.Sprintf("%s/%s/%d", r.prefix, safeName(node.ID), i),
			flow: r.flow, inputs: r.inputs, scope: r.scope, norm: r.norm, dyn: r.dyn,
			states: map[string]nodeState{}, webhook: r.webhook,
		}
		body.run(flowDef{Nodes: data.Body.Nodes, Edges: data.Body.Edges})
		if len(body.failures) > 0 && !data.ContinueOnError {
			r.failures = append(r.failures, body.failures...)
			return stateFailed
		}
	}
	return statePassed
}

var templateRef = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// resolve substitutes {{...}} references in s; unknown ones stay verbatim, as
// in echopoint-runner.
func (r *flowRun) resolve(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return templateRef.ReplaceAllStringFunc(s, func(tok string) string {
		ref := templateRef.FindStringSubmatch(tok)[1]
		if v, ok := r.lookup(ref); ok {
			return toString(v)
		}
		return tok
	})
}

// resolveAny returns the raw value when s is exactly one reference.
func (r *flowRun) resolveAny(s string) any {
	m := templateRef.FindStringSubmatch(strings.TrimSpace(s))
	if m != nil && m[0] == strings.TrimSpace(s) {
		if v, ok := r.lookup(m[1]); ok {
			return v
		}
	}
	return r.resolve(s)
}

func (r *flowRun) resolveDeep(v any) any {
	switch t := v.(type) {
	case string:
		return r.resolveAny(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = r.resolveDeep(item)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = r.resolveDeep(item)
		}
		return out
	default:
		return v
	}
}

func (r *flowRun) lookup(ref string) (any, bool) {
	if name, ok := strings.CutPrefix(ref, "$"); ok {
		value, err := r.dyn.resolve(name, r.norm)
		return value, err == nil
	}
	if v, ok := r.inputs[ref]; ok {
		return v, true
	}
	parts := strings.Split(ref, ".")
	current, ok := r.scope[parts[0]]
	if !ok {
		if v, isInput := r.inputs[parts[0]]; isInput && len(parts) == 1 {
			return v, true
		}
		return nil, false
	}
	for _, part := range parts[1:] {
		switch container := current.(type) {
		case map[string]any:
			current, ok = container[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(container) {
				return nil, false
			}
			current = container[index]
		default:
			return nil, false
		}
	}
	return current, true
}

// hasMissingNodeRef reports a {{node.output}} reference whose node never
// produced the output: an always node in that state is skipped, not run.
func (r *flowRun) hasMissingNodeRef(s string) bool {
	for _, m := range templateRef.FindAllStringSubmatch(s, -1) {
		ref := m[1]
		if strings.HasPrefix(ref, "$") {
			continue
		}
		if _, ok := r.lookup(ref); !ok {
			return true
		}
	}
	return false
}

func safeName(s string) string {
	return strings.Map(func(c rune) rune {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			return c
		default:
			return '_'
		}
	}, s)
}

// dynamicVars implements the {{$name}} variables the flows use. Values derive
// from a random run id, so a rerun in the same process never collides, and
// each generated value is registered with the normalizer.
type dynamicVars struct {
	runID   string
	started time.Time
	counter int
}

func newDynamicVars() *dynamicVars {
	const runIDBytes = 6
	buf := make([]byte, runIDBytes)
	_, _ = rand.Read(buf)
	// The first nibble is a letter (a-f): an all-digit run id would be taken
	// for a number, which the normalizer refuses to register, and the raw id
	// would then leak into the goldens (about one flow run in 280).
	const letterNibbles = 6
	buf[0] = (0xa+buf[0]>>4%letterNibbles)<<4 | buf[0]&0x0f
	return &dynamicVars{runID: hex.EncodeToString(buf), started: time.Now().UTC()}
}

var (
	firstNames = []string{"Ada", "Grace", "Alan", "Edsger", "Barbara", "Donald", "Frances", "Ken"}
	lastNames  = []string{"Lovelace", "Hopper", "Turing", "Dijkstra", "Liskov", "Knuth", "Allen", "Thompson"}
	slugWords  = []string{"amber", "basil", "cedar", "delta", "ember", "fjord", "garnet", "harbor", "indigo"}
)

func (d *dynamicVars) resolve(expr string, norm *normalizer) (string, error) {
	name, arg, _ := strings.Cut(expr, ":")
	d.counter++
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", d.runID, d.counter)))
	switch name {
	case "runId":
		return d.runID, nil
	case "uuid":
		h := hex.EncodeToString(sum[:16])
		return fmt.Sprintf("%s-%s-4%s-8%s-%s", h[0:8], h[8:12], h[13:16], h[17:20], h[20:32]), nil
	case "isoTimestamp":
		at := d.started
		if arg != "" {
			offset, err := time.ParseDuration(strings.TrimPrefix(arg, "+"))
			if err != nil {
				return "", err
			}
			at = at.Add(offset)
		}
		value := at.Format(time.RFC3339)
		norm.register(value, "{{$isoTimestamp:"+arg+"}}")
		return value, nil
	case "fullName":
		value := fmt.Sprintf("%s %s %s", firstNames[int(sum[0])%len(firstNames)],
			lastNames[int(sum[1])%len(lastNames)], hex.EncodeToString(sum[2:5]))
		norm.register(value, "{{$fullName}}")
		return value, nil
	case "slug":
		words := 3
		if n, err := strconv.Atoi(arg); err == nil && n > 0 {
			words = n
		}
		parts := make([]string, words)
		for i := range parts {
			parts[i] = slugWords[int(sum[i])%len(slugWords)]
		}
		value := strings.Join(parts, "-") + "-" + hex.EncodeToString(sum[10:13])
		norm.register(value, "{{$slug}}")
		return value, nil
	default:
		return "", fmt.Errorf("unsupported dynamic variable $%s", name)
	}
}
