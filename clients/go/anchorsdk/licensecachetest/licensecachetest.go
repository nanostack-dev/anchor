// Package licensecachetest is a conformance suite for [anchorsdk.LicenseCacheStore]
// implementations. Run it from the store's own tests.
package licensecachetest

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

// maxFlows is the limit value every test entry carries.
const maxFlows = 50

// RunStoreTests checks that the stores newStore returns keep the
// [anchorsdk.LicenseCacheStore] contract. Each subtest gets a fresh store and
// organization IDs no other subtest or run has used, so a store that keeps its
// keys between runs, such as a shared Redis, never collides with itself.
func RunStoreTests(t *testing.T, newStore func(t *testing.T) anchorsdk.LicenseCacheStore) {
	t.Helper()

	tests := map[string]func(t *testing.T, store anchorsdk.LicenseCacheStore){
		"load on an empty store misses":                                testEmptyLoad,
		"store then load round-trips the entry":                        testRoundTrip,
		"store with a generation from before an invalidate is skipped": testStaleGenerationSkipped,
		"invalidate drops the entry":                                   testInvalidateDrops,
		"organizations are independent":                                testIndependentOrganizations,
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			run(t, newStore(t))
		})
	}
}

func testEmptyLoad(t *testing.T, store anchorsdk.LicenseCacheStore) {
	read := load(t, store, uniqueOrgID())
	if read.Entry != nil {
		t.Errorf("Entry = %+v, want nil", read.Entry)
	}
}

func testRoundTrip(t *testing.T, store anchorsdk.LicenseCacheStore) {
	org := uniqueOrgID()
	want := entry("tpl_1")

	save(t, store, org, want, load(t, store, org).Generation)

	read := load(t, store, org)
	if read.Entry == nil || read.Entry.License == nil {
		t.Fatalf("Entry = %+v, want the stored entry", read.Entry)
	}
	if got := read.Entry.License.TemplateId; got != "tpl_1" {
		t.Errorf("TemplateId = %q, want tpl_1", got)
	}
	// A store that keeps JSON returns numbers as float64.
	if got := read.Entry.License.Values["max_flows"]; got != float64(maxFlows) && got != maxFlows {
		t.Errorf("Values[max_flows] = %v, want %d", got, maxFlows)
	}
	if !read.Entry.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", read.Entry.FetchedAt, want.FetchedAt)
	}
}

func testStaleGenerationSkipped(t *testing.T, store anchorsdk.LicenseCacheStore) {
	org := uniqueOrgID()

	before := load(t, store, org)
	invalidate(t, store, org)
	// A skipped write is not an error, so save fails the test only on an error.
	save(t, store, org, entry("tpl_1"), before.Generation)

	read := load(t, store, org)
	if read.Entry != nil {
		t.Errorf("Entry = %+v, want nil: the stale write must be skipped", read.Entry)
	}
	if read.Generation == before.Generation {
		t.Errorf("Generation = %d, want it advanced by Invalidate", read.Generation)
	}
}

func testInvalidateDrops(t *testing.T, store anchorsdk.LicenseCacheStore) {
	org := uniqueOrgID()

	save(t, store, org, entry("tpl_1"), load(t, store, org).Generation)
	invalidate(t, store, org)

	if read := load(t, store, org); read.Entry != nil {
		t.Errorf("Entry = %+v, want nil after Invalidate", read.Entry)
	}
}

func testIndependentOrganizations(t *testing.T, store anchorsdk.LicenseCacheStore) {
	orgA, orgB := uniqueOrgID(), uniqueOrgID()

	save(t, store, orgA, entry("tpl_a"), load(t, store, orgA).Generation)
	save(t, store, orgB, entry("tpl_b"), load(t, store, orgB).Generation)
	invalidate(t, store, orgA)

	if read := load(t, store, orgA); read.Entry != nil {
		t.Errorf("first organization Entry = %+v, want nil", read.Entry)
	}
	read := load(t, store, orgB)
	if read.Entry == nil || read.Entry.License.TemplateId != "tpl_b" {
		t.Errorf("second organization Entry = %+v, want tpl_b untouched by the first one's Invalidate", read.Entry)
	}
}

func load(t *testing.T, store anchorsdk.LicenseCacheStore, org string) anchorsdk.LicenseCacheRead {
	t.Helper()

	read, err := store.Load(context.Background(), org)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return read
}

func save(t *testing.T, store anchorsdk.LicenseCacheStore, org string, e anchorsdk.CachedLicense, generation int64) {
	t.Helper()

	if err := store.Store(context.Background(), org, e, generation); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

func invalidate(t *testing.T, store anchorsdk.LicenseCacheStore, org string) {
	t.Helper()

	if err := store.Invalidate(context.Background(), org); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
}

// uniqueOrgID returns an organization ID no other call or run has used.
func uniqueOrgID() string {
	return "org_" + rand.Text()
}

func entry(templateID string) anchorsdk.CachedLicense {
	return anchorsdk.CachedLicense{
		License: &nanoclient.OrganizationLicenseResponse{
			TemplateId: templateID,
			Values:     nanoclient.LicenseTemplateValues{"max_flows": maxFlows},
		},
		FetchedAt: time.Date(2026, 11, 16, 12, 0, 0, 123, time.UTC),
	}
}
