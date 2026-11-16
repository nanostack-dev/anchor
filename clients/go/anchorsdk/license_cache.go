package anchorsdk

import (
	"context"
	"errors"
	"sync"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// defaultLicenseCacheTTL bounds how long [LicenseCache.Get] serves a cached
// read before attempting a live refresh. 30s keeps a hot-path, permission-style
// check from adding a network round trip to every request, while keeping a
// template swap or an out-of-band Adjust visible reasonably quickly.
const defaultLicenseCacheTTL = 30 * time.Second

// LicenseCachePolicy controls how a [LicenseCache] serves a license read. The
// zero value uses the defaults: a 30s TTL, and fail-open on a stale entry.
//
// The cache is explicit. [License.Get] never reads it, and no write made
// through the SDK updates it. Call [LicenseCache.Invalidate] after a change.
type LicenseCachePolicy struct {
	// TTL bounds how long a cached read is served without a live refresh.
	// Zero uses the default.
	TTL time.Duration

	// Strict disables fail-open. By default, once TTL has elapsed, a live
	// refresh that fails with a transient error (a transport failure, a 5xx,
	// or 429 with retries exhausted) still serves the last known value —
	// marked Stale on [LicenseSnapshot] — rather than returning an error, so
	// an Anchor outage does not block a paying customer; see ADR-0001. A
	// permanent failure (any other 4xx: a bad request, a revoked key, an
	// organization that no longer exists) is never papered over regardless of
	// Strict, because unlike an outage it will not resolve on its own.
	//
	// Set Strict to opt into fail-closed: a failed refresh past TTL always
	// returns the error, even one that fail-open would otherwise absorb.
	Strict bool
}

// ttl resolves the effective TTL, applying the default for the zero value.
func (p LicenseCachePolicy) ttl() time.Duration {
	if p.TTL <= 0 {
		return defaultLicenseCacheTTL
	}
	return p.TTL
}

// CachedLicense is one organization's last known license read, as a
// [LicenseCacheStore] keeps it. It marshals to JSON so a store can keep it in
// a shared system such as Redis.
type CachedLicense struct {
	License   *nanoclient.OrganizationLicenseResponse `json:"license"`
	FetchedAt time.Time                               `json:"fetched_at"`
}

// LicenseCacheRead is the result of [LicenseCacheStore.Load].
type LicenseCacheRead struct {
	// Entry is the stored read, or nil on a miss.
	Entry *CachedLicense
	// Generation is the store's version for this organization. Pass it back to
	// [LicenseCacheStore.Store].
	Generation int64
}

// LicenseCacheStore keeps cached license reads, one per organization. A store
// shared by several processes lets one process's [LicenseCache.Invalidate]
// reach the others. Implementations must be safe for concurrent use.
type LicenseCacheStore interface {
	// Load returns the organization's entry, if any, and its current generation.
	Load(ctx context.Context, organizationID string) (LicenseCacheRead, error)
	// Store saves entry only if the organization's generation still equals generation,
	// that is, no Invalidate ran since the Load that returned it. A skipped write is not an error.
	Store(ctx context.Context, organizationID string, entry CachedLicense, generation int64) error
	// Invalidate drops the entry and advances the generation.
	Invalidate(ctx context.Context, organizationID string) error
}

// memoryLicenseCacheStore is the in-process [LicenseCacheStore].
type memoryLicenseCacheStore struct {
	mu          sync.Mutex
	entries     map[string]CachedLicense
	generations map[string]int64
}

// NewMemoryLicenseCacheStore returns a store that keeps entries in this
// process's memory. Entries never expire on their own: [LicenseCache] applies
// the TTL. Each process has its own view, so an Invalidate in one process does
// not reach another.
func NewMemoryLicenseCacheStore() LicenseCacheStore {
	return &memoryLicenseCacheStore{
		entries:     make(map[string]CachedLicense),
		generations: make(map[string]int64),
	}
}

func (s *memoryLicenseCacheStore) Load(_ context.Context, organizationID string) (LicenseCacheRead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	read := LicenseCacheRead{Generation: s.generations[organizationID]}
	if entry, ok := s.entries[organizationID]; ok {
		read.Entry = &entry
	}
	return read, nil
}

func (s *memoryLicenseCacheStore) Store(
	_ context.Context, organizationID string, entry CachedLicense, generation int64,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generations[organizationID] != generation {
		return nil
	}
	s.entries[organizationID] = entry
	return nil
}

func (s *memoryLicenseCacheStore) Invalidate(_ context.Context, organizationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, organizationID)
	s.generations[organizationID]++
	return nil
}

// LicenseCache serves license reads from a [LicenseCacheStore], going to
// Anchor when the entry is missing or older than the TTL. It is opt-in:
// [License.Get] never uses it. Build one with [NewLicenseCache] and share it.
// It is safe for concurrent use when its store is.
//
//	cache := anchorsdk.NewLicenseCache(client, nil, anchorsdk.LicenseCachePolicy{})
//	snapshot, err := cache.Get(ctx, orgID)
//
// Nothing written through the SDK updates the cache. Call
// [LicenseCache.Invalidate] after [License.Instantiate], [License.Adjust],
// [Licensing.Migrate] or a usage report when the next read must see it.
type LicenseCache struct {
	client *Client
	store  LicenseCacheStore
	policy LicenseCachePolicy
}

// NewLicenseCache builds a cache that reads through client. A nil store uses
// [NewMemoryLicenseCacheStore].
func NewLicenseCache(client *Client, store LicenseCacheStore, policy LicenseCachePolicy) *LicenseCache {
	if store == nil {
		store = NewMemoryLicenseCacheStore()
	}
	return &LicenseCache{client: client, store: store, policy: policy}
}

// Get returns the organization's license, from the store while the entry is
// younger than the TTL and from Anchor otherwise. A live read is written back
// to the store, unless an Invalidate ran while it was in flight.
//
// When the live read fails with a transient error and the policy is not
// Strict, Get returns the last stored entry with Stale set. A permanent error
// is always returned. A store failure never reaches the caller: a failed Load
// counts as a miss, and a failed Store is ignored.
func (c *LicenseCache) Get(ctx context.Context, organizationID string) (*LicenseSnapshot, error) {
	read, loadErr := c.store.Load(ctx, organizationID)
	if loadErr != nil {
		read = LicenseCacheRead{}
	}

	if read.Entry != nil && time.Since(read.Entry.FetchedAt) < c.policy.ttl() {
		return snapshotOf(read.Entry, false), nil
	}

	snap, err := c.client.Organization(organizationID).License().Get(ctx)
	if err != nil {
		if !c.policy.Strict && !errors.Is(err, ErrPermanent) && read.Entry != nil {
			return snapshotOf(read.Entry, true), nil
		}
		return nil, err
	}

	if loadErr == nil {
		entry := CachedLicense{License: snap.OrganizationLicenseResponse, FetchedAt: snap.FetchedAt}
		_ = c.store.Store(ctx, organizationID, entry, read.Generation)
	}

	return snap, nil
}

// Invalidate drops the organization's entry, so the next Get reads from
// Anchor, and stops a live read that is already in flight from storing its
// result. With a shared store this reaches every process that uses it.
func (c *LicenseCache) Invalidate(ctx context.Context, organizationID string) error {
	return c.store.Invalidate(ctx, organizationID)
}

func snapshotOf(entry *CachedLicense, stale bool) *LicenseSnapshot {
	return &LicenseSnapshot{
		OrganizationLicenseResponse: entry.License,
		Stale:                       stale,
		FetchedAt:                   entry.FetchedAt,
	}
}
