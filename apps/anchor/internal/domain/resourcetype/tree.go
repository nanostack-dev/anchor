package resourcetype

import (
	"cmp"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// Tree is a product's resource types, keyed by slug. The type graph may be a
// DAG (a type lists several parent types), but each resource keeps one parent.
type Tree struct {
	bySlug map[string]ResourceType
	byID   map[string]ResourceType
}

func NewTree(types []ResourceType) Tree {
	return Tree{
		bySlug: functional.Slice(types).ToMap(func(t ResourceType) string { return t.Slug }),
		byID:   functional.Slice(types).ToMap(func(t ResourceType) string { return t.ID }),
	}
}

func (t Tree) BySlug(slug string) functional.Option[ResourceType] {
	found, ok := t.bySlug[slug]
	return functional.OptionOf(found, ok)
}

func (t Tree) ByID(id string) functional.Option[ResourceType] {
	found, ok := t.byID[id]
	return functional.OptionOf(found, ok)
}

// Organization is the built-in root. Every stored tree has one.
func (t Tree) Organization() functional.Option[ResourceType] {
	return t.BySlug(OrganizationSlug)
}

// Types lists the tree with organization first, then by creation time. Types
// created in one transaction tie on time and follow by slug.
func (t Tree) Types() []ResourceType {
	types := make([]ResourceType, 0, len(t.bySlug))
	for _, rt := range t.bySlug {
		types = append(types, rt)
	}
	slices.SortFunc(types, func(a, b ResourceType) int {
		if a.BuiltIn() != b.BuiltIn() {
			if a.BuiltIn() {
				return -1
			}
			return 1
		}
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.Slug, b.Slug)
	})
	return types
}

// ScopeFilter is a role or permission search filter on scopes, resolved to
// type IDs.
type ScopeFilter struct {
	IDs []string
}

// Active reports whether the filter restricts anything.
func (f ScopeFilter) Active() bool {
	return len(f.IDs) > 0
}

// ResolveScopes turns slugs into a ScopeFilter. A slug the product lacks is
// kept as an ID no type has, so it matches nothing instead of widening the
// search.
func (t Tree) ResolveScopes(slugs []string) ScopeFilter {
	var filter ScopeFilter
	for _, slug := range slugs {
		found, ok := t.BySlug(slug).Get()
		if !ok {
			filter.IDs = append(filter.IDs, "")
			continue
		}
		filter.IDs = append(filter.IDs, found.ID)
	}
	return filter
}

// Missing returns the slugs the tree does not declare, in request order.
func (t Tree) Missing(slugs []string) []string {
	return functional.Slice(slugs).Filter(func(slug string) bool {
		_, ok := t.bySlug[slug]
		return !ok
	})
}

// Children lists the types that name slug among their parent types.
func (t Tree) Children(slug string) []string {
	var children []string
	for _, rt := range t.bySlug {
		if slices.Contains(rt.ParentTypes, slug) {
			children = append(children, rt.Slug)
		}
	}
	slices.Sort(children)
	return children
}

// Below is slug and every type reachable downward from it: the types a role
// scoped to slug may hold permissions of (ADR-0022 D1).
func (t Tree) Below(slug string) map[string]bool {
	below := map[string]bool{}
	pending := []string{slug}
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		if below[next] {
			continue
		}
		below[next] = true
		pending = append(pending, t.Children(next)...)
	}
	return below
}

// Validate checks the rules every stored tree keeps: organization is the only
// root, parents exist and never include the type itself, no two types are
// each other's ancestors, and no type sits deeper than MaxDepth.
func (t Tree) Validate() error {
	for _, rt := range t.Types() {
		if rt.BuiltIn() {
			continue
		}
		if slices.Contains(rt.ParentTypes, rt.Slug) {
			return NewSelfParentError(rt.Slug)
		}
		if missing := t.Missing(rt.ParentTypes); len(missing) > 0 {
			return NewUnknownError(missing)
		}
	}
	if cycle := t.cycle(); len(cycle) > 0 {
		return NewCycleError(cycle)
	}
	depths := map[string]int{}
	for _, rt := range t.Types() {
		if depth := t.depth(rt.Slug, depths); depth > MaxDepth {
			return NewTooDeepError(rt.Slug, depth)
		}
	}
	return nil
}

// cycle returns a chain of slugs that leads back to its first one, walking
// parent edges, or nil when the graph is acyclic.
func (t Tree) cycle() []string {
	const (
		unvisited = iota
		inPath
		done
	)
	state := map[string]int{}
	var path []string
	var visit func(slug string) []string
	visit = func(slug string) []string {
		switch state[slug] {
		case inPath:
			start := slices.Index(path, slug)
			return append(slices.Clone(path[start:]), slug)
		case done:
			return nil
		}
		state[slug] = inPath
		path = append(path, slug)
		parents := slices.Clone(t.bySlug[slug].ParentTypes)
		slices.Sort(parents)
		for _, parent := range parents {
			if _, ok := t.bySlug[parent]; !ok {
				continue
			}
			if found := visit(parent); found != nil {
				return found
			}
		}
		path = path[:len(path)-1]
		state[slug] = done
		return nil
	}
	for _, rt := range t.Types() {
		if found := visit(rt.Slug); found != nil {
			return found
		}
	}
	return nil
}

// depth is the type's level counting organization as 1, through its deepest
// parent. Call it only on an acyclic tree.
func (t Tree) depth(slug string, memo map[string]int) int {
	if depth, ok := memo[slug]; ok {
		return depth
	}
	deepest := 0
	for _, parent := range t.bySlug[slug].ParentTypes {
		deepest = max(deepest, t.depth(parent, memo))
	}
	memo[slug] = deepest + 1
	return deepest + 1
}
