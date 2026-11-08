//go:build contract

package contract_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// hookServer stands in for Echopoint's webhook receiver ({{webhook.url}}): it
// records every product event Anchor delivers, per flow run.
type hookServer struct {
	once    sync.Once
	server  *httptest.Server
	mu      sync.Mutex
	inboxes map[string]*hookInbox
}

type hookInbox struct {
	url        string
	mu         sync.Mutex
	deliveries []*response
}

var hooks = &hookServer{inboxes: map[string]*hookInbox{}}

func (h *hookServer) inbox(key string) *hookInbox {
	h.once.Do(func() {
		h.server = httptest.NewServer(http.HandlerFunc(h.serve))
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	inbox := &hookInbox{url: h.server.URL + "/webhook/" + key}
	h.inboxes[key] = inbox
	return inbox
}

func (h *hookServer) serve(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/webhook/")
	h.mu.Lock()
	inbox := h.inboxes[key]
	h.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	if inbox == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	delivery := &response{status: http.StatusOK, header: r.Header.Clone(), body: raw}
	var parsed any
	if json.Unmarshal(raw, &parsed) == nil {
		delivery.parsed = parsed
		delivery.isJSON = true
	}
	inbox.mu.Lock()
	inbox.deliveries = append(inbox.deliveries, delivery)
	inbox.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (i *hookInbox) snapshot() []*response {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]*response(nil), i.deliveries...)
}

type hookExpectation struct {
	Name       string      `json:"name"`
	Min        *int        `json:"min"`
	Max        *int        `json:"max"`
	Assertions []assertion `json:"assertions"`
}

type webhookWaitData struct {
	Expect    []hookExpectation `json:"expect"`
	SettleMs  int               `json:"settle_ms"`
	TimeoutMs int               `json:"timeout_ms"`
}

// executeWebhookWait follows echopoint-runner's webhook_wait: with
// expectations, wait until every minimum is met, let the settle time pass,
// then check every count against its [min, max]; without, wait for one
// delivery that passes the node's own assertions.
func (r *flowRun) executeWebhookWait(node *flowNode) nodeState {
	var data webhookWaitData
	if err := json.Unmarshal(node.Data, &data); err != nil {
		return r.fail(node, "webhook_wait data: "+err.Error())
	}
	const pollEvery = 100 * time.Millisecond
	deadline := time.Now().Add(time.Duration(data.TimeoutMs) * time.Millisecond)
	if len(data.Expect) == 0 {
		for {
			for _, d := range r.webhook.snapshot() {
				if len(r.checkAssertions(node.Assertions, d)) == 0 {
					return statePassed
				}
			}
			if time.Now().After(deadline) {
				return r.fail(node, fmt.Sprintf("no delivery matched within %dms (%d received)",
					data.TimeoutMs, len(r.webhook.snapshot())))
			}
			time.Sleep(pollEvery)
		}
	}
	for {
		counts := r.countExpectations(node, data.Expect)
		met := true
		for i, e := range data.Expect {
			if counts[i] < minimum(e) {
				met = false
			}
		}
		if met || time.Now().After(deadline) {
			break
		}
		time.Sleep(pollEvery)
	}
	time.Sleep(time.Duration(data.SettleMs) * time.Millisecond)
	counts := r.countExpectations(node, data.Expect)
	var problems []string
	for i, e := range data.Expect {
		if counts[i] < minimum(e) || (e.Max != nil && counts[i] > *e.Max) {
			problems = append(problems, fmt.Sprintf("%q received %d", e.Name, counts[i]))
		}
	}
	if len(problems) > 0 {
		return r.fail(node, "event expectations: "+strings.Join(problems, "; "))
	}
	return statePassed
}

func minimum(e hookExpectation) int {
	if e.Min == nil {
		return 1
	}
	return *e.Min
}

func (r *flowRun) countExpectations(node *flowNode, expect []hookExpectation) []int {
	counts := make([]int, len(expect))
	for _, d := range r.webhook.snapshot() {
		if len(r.checkAssertions(node.Assertions, d)) > 0 {
			continue
		}
		for i, e := range expect {
			if len(r.checkAssertions(e.Assertions, d)) == 0 {
				counts[i]++
			}
		}
	}
	return counts
}
