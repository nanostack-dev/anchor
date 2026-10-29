package fgasync

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"anchor/internal/events"
)

// Marker marks what each event makes stale, inside the event's transaction, so
// the mark and the write commit together and no committed change goes
// unsynced. It runs inside every write that emits an event, so it depends on
// the sources' Stale and the model's changes only: nothing it needs may
// itself emit events.
type Marker struct {
	repo         Repository
	sources      []Source
	modelChanges ModelChanges
}

func NewMarker(repo Repository, sources []Source, modelChanges ModelChanges) (*Marker, error) {
	if _, err := sourcesByName(sources); err != nil {
		return nil, err
	}
	return &Marker{repo: repo, sources: sources, modelChanges: modelChanges}, nil
}

// ObserveTx marks the scopes the event makes stale; a model change marks the
// whole Product. When the request collects marks (CollectMarks), it also hands
// them to the request.
func (m *Marker) ObserveTx(ctx context.Context, event events.Event) error {
	scopes, err := m.staleScopes(event)
	if err != nil {
		return err
	}
	collector := collectorFrom(ctx)
	for _, scope := range scopes {
		if err = m.repo.Mark(ctx, scope); err != nil {
			return err
		}
		if collector != nil {
			collector.add(scope)
		}
	}
	return nil
}

func (m *Marker) staleScopes(event events.Event) ([]Scope, error) {
	if m.modelChanges(event) {
		return []Scope{WholeProduct(event.ProductID)}, nil
	}
	var scopes []Scope
	for _, p := range m.sources {
		keys, err := p.Stale(event)
		if err != nil {
			return nil, fmt.Errorf("authz source %s: %w", p.Name(), err)
		}
		for _, key := range keys {
			if key == "" {
				return nil, fmt.Errorf(
					"%w: source %s marked an empty key for %s",
					ErrInvalidScope,
					p.Name(),
					event.Type,
				)
			}
			scope := Scope{ProductID: event.ProductID, Source: p.Name(), Key: key}
			if !slices.Contains(scopes, scope) {
				scopes = append(scopes, scope)
			}
		}
	}
	if len(scopes) > 0 && event.ProductID == "" {
		return nil, fmt.Errorf("%w: %s event without a product", ErrInvalidScope, event.Type)
	}
	return scopes, nil
}

// markCollector collects the scopes a request's writes marked.
type markCollector struct {
	mu     sync.Mutex
	scopes []Scope
}

type collectorKey struct{}

// CollectMarks returns a context in which marks are also collected, and the
// function that lists them once the request's transaction has committed.
func CollectMarks(ctx context.Context) (context.Context, func() []Scope) {
	collector := &markCollector{}
	return context.WithValue(ctx, collectorKey{}, collector), collector.list
}

func collectorFrom(ctx context.Context) *markCollector {
	collector, _ := ctx.Value(collectorKey{}).(*markCollector)
	return collector
}

func (c *markCollector) add(scope Scope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !slices.Contains(c.scopes, scope) {
		c.scopes = append(c.scopes, scope)
	}
}

func (c *markCollector) list() []Scope {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.scopes)
}
