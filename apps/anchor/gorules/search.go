//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// searchUsesContains reports a hand-built LIKE outside searchx. A raw LIKE is
// case-sensitive and reads % and _ in the user's term as wildcards; every free
// text filter goes through searchx.Contains instead.
func searchUsesContains(m dsl.Matcher) {
	m.Import("github.com/nanostack-dev/nanostack-framework/db/postgres")
	m.Import("github.com/nanostack-dev/nanostack-framework/pkg/jetx")

	m.Match(`$x.LIKE($_)`, `$x.NOT_LIKE($_)`).
		Where(m["x"].Type.Implements(`postgres.StringExpression`) &&
			!m.File().PkgPath.Matches(`/internal/searchx$`)).
		Report(`use searchx.Contains: a raw LIKE is case-sensitive and treats % and _ in the term as wildcards`)

	m.Match(`jetx.BuildSubstringFilter($*_)`).
		Report(`use searchx.Contains: jetx.BuildSubstringFilter is case-sensitive and treats % and _ as wildcards`)
}
