//go:build contract

package contract_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Baseline benchmarks through the full in-process stack (loopback HTTP into
// the same server the goldens use). allocs/op and B/op count the client and
// the server together, since both run in the test process; the client side is
// the same before and after a migration step.

type benchFixture struct {
	product string
	key     string
	org     string
	runID   string
}

// bench is built once per process and shared by the benchmarks.
var bench struct {
	once sync.Once
	fix  benchFixture
	err  error
}

func fixture(b *testing.B) benchFixture {
	b.Helper()
	bench.once.Do(func() { bench.fix, bench.err = buildBenchFixture() })
	if bench.err != nil {
		b.Fatal(bench.err)
	}
	suite.logs.setDiscard(true)
	b.Cleanup(func() { suite.logs.setDiscard(false) })
	return bench.fix
}

func buildBenchFixture() (benchFixture, error) {
	ctx := context.Background()
	fix := benchFixture{runID: newDynamicVars().runID}
	owner := map[string]string{"Content-Type": jsonCT, "Authorization": "Bearer " + suite.ownerToken}
	step := func(path, body, field string) (string, error) {
		resp, err := rawDo(ctx, http.MethodPost, suite.baseURL+path, owner, body, "")
		if err != nil {
			return "", err
		}
		if resp.status != http.StatusCreated {
			return "", fmt.Errorf("POST %s: status %d: %s", path, resp.status, resp.body)
		}
		value, _ := resp.json()[field].(string)
		return value, nil
	}
	var err error
	if fix.product, err = step("/v1/products",
		`{"name":"contract-bench-`+fix.runID+`"}`, "id"); err != nil {
		return fix, err
	}
	if fix.key, err = step("/v1/products/"+fix.product+"/api-keys",
		`{"name":"contract-bench","mutable":false,"permissions":["organization:read","organization:create",`+
			`"product_user:read","product_user:create"]}`, "value"); err != nil {
		return fix, err
	}
	owner = map[string]string{"Content-Type": jsonCT, "X-Product-Api-Key": fix.key}
	if fix.org, err = step("/v1/products/"+fix.product+"/organizations",
		`{"name":"contract-bench-org"}`, "id"); err != nil {
		return fix, err
	}
	const seededUsers = 50
	for i := range seededUsers {
		if _, err = step(
			"/v1/products/"+fix.product+"/product-users",
			fmt.Sprintf(
				`{"email":"garbage+bench-%s-seed%d@nanostack.dev","name":"Bench %d"}`,
				fix.runID,
				i,
				i,
			),
			"id",
		); err != nil {
			return fix, err
		}
	}
	return fix, nil
}

func benchRequest(b *testing.B, method, path string, headers map[string]string, body func(i int) string, want int) {
	b.Helper()
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		resp, err := rawDo(ctx, method, suite.baseURL+path, headers, body(i), "")
		if err != nil {
			b.Fatal(err)
		}
		if resp.status != want {
			b.Fatalf("status %d: %s", resp.status, resp.body)
		}
		i++
	}
}

func noBody(int) string { return "" }

func keyHeaders(fix benchFixture) map[string]string {
	return map[string]string{"Content-Type": jsonCT, "X-Product-Api-Key": fix.key}
}

// BenchmarkHealth: GET /health (no auth, no request log, one tenant lookup).
func BenchmarkHealth(b *testing.B) {
	fixture(b)
	benchRequest(b, http.MethodGet, "/health", nil, noBody, http.StatusOK)
}

// BenchmarkReadOrganization: cheap read through product API key auth.
func BenchmarkReadOrganization(b *testing.B) {
	fix := fixture(b)
	benchRequest(b, http.MethodGet, "/v1/products/"+fix.product+"/organizations/"+fix.org,
		keyHeaders(fix), noBody, http.StatusOK)
}

// BenchmarkSearchProductUsers: a 20-item page of a cursor search.
func BenchmarkSearchProductUsers(b *testing.B) {
	fix := fixture(b)
	benchRequest(b, http.MethodPost, "/v1/products/"+fix.product+"/product-users/search", keyHeaders(fix),
		func(int) string { return `{"pagination":{"limit":20},"sort_by":"created_at"}` }, http.StatusOK)
}

// BenchmarkCreateProductUser: a validated write in a transaction with an
// event enqueue.
func BenchmarkCreateProductUser(b *testing.B) {
	fix := fixture(b)
	prefix := newDynamicVars().runID
	benchRequest(b, http.MethodPost, "/v1/products/"+fix.product+"/product-users", keyHeaders(fix),
		func(i int) string {
			return fmt.Sprintf(`{"email":"garbage+bench-%s-%d@nanostack.dev","name":"Bench user"}`, prefix, i)
		}, http.StatusCreated)
}

// BenchmarkValidationError: 400 VALIDATION_ERROR through the service validator
// and the error handler, with its two log lines.
func BenchmarkValidationError(b *testing.B) {
	fix := fixture(b)
	benchRequest(b, http.MethodPost, "/v1/products/"+fix.product+"/product-users", keyHeaders(fix),
		func(int) string { return `{"email":"not-an-email","name":"x"}` }, http.StatusBadRequest)
}

// BenchmarkNotFound: 404 domain error for an unknown organization.
func BenchmarkNotFound(b *testing.B) {
	fix := fixture(b)
	benchRequest(b, http.MethodGet, "/v1/products/"+fix.product+"/organizations/org_000000000000000000000000000",
		keyHeaders(fix), noBody, http.StatusNotFound)
}

// BenchmarkHeapAfterWarmup reports the test process heap after 1,000 mixed
// requests and two GCs. The process also holds the harness (testcontainers
// client, the suite), which a migration does not change.
func BenchmarkHeapAfterWarmup(b *testing.B) {
	fix := fixture(b)
	ctx := b.Context()
	const warmup = 250
	for i := range warmup {
		for _, req := range []struct{ method, path, body string }{
			{http.MethodGet, "/health", ""},
			{http.MethodGet, "/v1/products/" + fix.product + "/organizations/" + fix.org, ""},
			{http.MethodPost, "/v1/products/" + fix.product + "/product-users/search", `{"pagination":{"limit":20}}`},
			{http.MethodPost, "/v1/products/" + fix.product + "/product-users", `{"email":"bad","name":"x"}`},
		} {
			if _, err := rawDo(ctx, req.method, suite.baseURL+req.path, keyHeaders(fix), req.body, ""); err != nil {
				b.Fatalf("warm-up %d: %v", i, err)
			}
		}
	}
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	for b.Loop() {
		runtime.KeepAlive(&stats)
	}
	const mib = 1 << 20
	b.ReportMetric(float64(stats.HeapAlloc)/mib, "heap-alloc-MiB")
	b.ReportMetric(float64(stats.HeapInuse)/mib, "heap-inuse-MiB")
	b.ReportMetric(float64(runtime.NumGoroutine()), "goroutines")
}

// anchorBinary is built once per process (-count=10 reuses it).
var anchorBinary struct {
	once sync.Once
	path string
	err  error
}

// BenchmarkStartupToReady starts the real anchor binary (go build ./cmd)
// against the suite's Postgres and Redis, already migrated, and measures the
// time until GET /health answers 200. It also reports the process RSS when
// ready and after 400 warm-up requests.
func BenchmarkStartupToReady(b *testing.B) {
	fix := fixture(b)
	anchorBinary.once.Do(func() {
		//nolint:usetesting // b.TempDir is removed after each run; the binary outlives -count runs
		anchorBinary.path = filepath.Join(os.TempDir(), "anchor-contract-bench-"+fix.runID)
		build := exec.CommandContext(context.Background(), "go", "build", "-o", anchorBinary.path, "./cmd")
		build.Dir = filepath.Join("..", "..", "..")
		if out, err := build.CombinedOutput(); err != nil {
			anchorBinary.err = fmt.Errorf("go build: %w: %s", err, out)
		}
	})
	if anchorBinary.err != nil {
		b.Fatal(anchorBinary.err)
	}
	configPath, err := filepath.Abs("application.yaml")
	if err != nil {
		b.Fatal(err)
	}
	var readyRSS, warmRSS float64
	for b.Loop() {
		ready, warm := startOnce(b, configPath, fix)
		readyRSS += ready
		warmRSS += warm
	}
	n := float64(b.N)
	b.ReportMetric(readyRSS/n, "rss-ready-MiB")
	b.ReportMetric(warmRSS/n, "rss-warm-MiB")
}

func startOnce(b *testing.B, configPath string, fix benchFixture) (float64, float64) {
	b.Helper()
	port := freePort(b)
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	cmd := exec.CommandContext(context.Background(), anchorBinary.path)
	cmd.Dir = "." // migrations basePath is relative to the suite directory
	cmd.Env = append(os.Environ(), "CONFIG_PATH="+configPath, "SERVER_PORT="+strconv.Itoa(port),
		"LOG_REQUEST_BODY=false")
	if err := cmd.Start(); err != nil {
		b.Fatal(err)
	}
	defer func() {
		b.StopTimer()
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
		}
		b.StartTimer()
	}()
	ctx := context.Background()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		resp, err := rawDo(ctx, http.MethodGet, base+"/health", nil, "", "")
		if err == nil && resp.status == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			b.Fatalf("anchor did not become healthy: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	b.StopTimer() // the shutdown defer restarts it
	ready := rssMiB(b, cmd.Process.Pid)
	const warmup = 200
	for range warmup {
		_, _ = rawDo(ctx, http.MethodGet, base+"/health", nil, "", "")
		_, _ = rawDo(ctx, http.MethodGet, base+"/v1/products/"+fix.product+"/organizations/"+fix.org,
			keyHeaders(fix), "", "")
	}
	return ready, rssMiB(b, cmd.Process.Pid)
}

func rssMiB(b *testing.B, pid int) float64 {
	b.Helper()
	out, err := exec.CommandContext(context.Background(), "ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		b.Fatalf("ps: %v", err)
	}
	kib, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		b.Fatalf("ps output %q: %v", out, err)
	}
	const kibPerMiB = 1024
	return kib / kibPerMiB
}

func freePort(b *testing.B) int {
	b.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		b.Fatalf("unexpected address %T", listener.Addr())
	}
	return addr.Port
}
