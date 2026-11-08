//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// searchUsesContains reports a hand-built LIKE outside searchx. A raw LIKE is
// case-sensitive and reads % and _ in the user's term as wildcards; every free
// text filter goes through searchx.Contains instead.
//
// The rule matches on syntax only. It does not check that the receiver is a jet
// expression, and it does not check the file's imports: a file can call LIKE on
// a generated table column without importing the jet package. Resolving the jet
// types (m.Import with Type.Implements) made ruleguard type-check the jet
// package again for every file, and took the lint run from about one minute to
// more than five.
func searchUsesContains(m dsl.Matcher) {
	m.Match(`$x.LIKE($_)`, `$x.NOT_LIKE($_)`).
		Where(!m.File().PkgPath.Matches(`/internal/searchx$`)).
		Report(`use searchx.Contains: a raw LIKE is case-sensitive and treats % and _ in the term as wildcards`)

	m.Match(`$_.BuildSubstringFilter($*_)`).
		Report(`use searchx.Contains: jetx.BuildSubstringFilter is case-sensitive and treats % and _ as wildcards`)
}
