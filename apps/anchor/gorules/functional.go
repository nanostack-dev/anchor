//go:build ruleguard

// Package gorules holds the ruleguard rules that golangci-lint runs through
// gocritic. The build tag keeps them out of every ordinary build.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

const functionalPkg = "github.com/nanostack-dev/nanostack-framework/pkg/functional"

// ruleguard cannot parse an instantiated generic type, so functional.Seq[$_]
// is not expressible here. The next-best guard is the underlying type: a Seq is
// a defined slice type, and a slice with both a Filter and a Map method is one
// in every practical case.

// fuseFilterMap reports a Filter step followed immediately by a Map step. The
// pair builds an intermediate slice that FilterMap does not.
func fuseFilterMap(m dsl.Matcher) {
	m.Match(`$x.Filter($pred).Map($f)`).
		Where(m["x"].Type.Underlying().Is(`[]$_`) && m.File().Imports(functionalPkg)).
		Report(`Filter().Map() builds an intermediate slice; use FilterMap($pred, $f)`)
}

// findFirstAfterChain reports a scan that runs after the chain has already
// materialized every element it is about to discard.
func findFirstAfterChain(m dsl.Matcher) {
	m.Match(`$x.Map($f).FindFirst($q)`).
		Where(m["x"].Type.Underlying().Is(`[]$_`) && m.File().Imports(functionalPkg)).
		Report(`FindFirst after Map materializes the whole slice; call FindFirst on the source, then Map the Option`)
}
