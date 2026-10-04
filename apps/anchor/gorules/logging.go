//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// errorLevelForReturnedError reports an error logged at a fixed Error level in
// the request path. A 4xx fault there is the API answering correctly, so
// log.Event picks the level from the error instead. Background workers and
// startup jobs have no client to blame and are left out.
func errorLevelForReturnedError(m dsl.Matcher) {
	m.Import("github.com/rs/zerolog")
	m.Match(
		`$l.Error().Err($err)`,
		`$l.Error().$_($*_).Err($err)`,
		`$l.Error().$_($*_).$_($*_).Err($err)`,
		`$l.Error().$_($*_).$_($*_).$_($*_).Err($err)`,
	).
		Where((m["l"].Type.Is(`zerolog.Logger`) || m["l"].Type.Is(`*zerolog.Logger`)) &&
			m.File().PkgPath.Matches(`/internal/(service|repository|license/service)$`) &&
			!m.File().Name.Matches(`(_worker|_startup_sync)\.go$`)).
		Report(`a 4xx fault logged at Error pages for a correct answer; use log.Event(&$l, $err) from nanostack-framework/pkg/log`)
}
