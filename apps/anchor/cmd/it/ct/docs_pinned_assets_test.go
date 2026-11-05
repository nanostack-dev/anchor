package ct_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	caseExternalScriptTag = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']https?://[^"']+["'][^>]*>`)
	caseExternalStyleTag  = regexp.MustCompile(`(?is)<link\b[^>]*\bhref\s*=\s*["']https?://[^"']+["'][^>]*>`)
	caseTagURL            = regexp.MustCompile(`(?is)\b(?:src|href)\s*=\s*["'](https?://[^"']+)["']`)
	caseIntegrityAttr     = regexp.MustCompile(`(?is)\bintegrity\s*=\s*["']sha(?:256|384|512)-[^"']+["']`)
	casePinnedVersion     = regexp.MustCompile(`@\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?/`)
)

// caseCSPConfinesScripts reports whether a CSP restricts script sources to
// something that does not include unpkg (or any wildcard host).
func caseCSPConfinesScripts(csp string) bool {
	if csp == "" {
		return false
	}
	directives := map[string]string{}
	for part := range strings.SplitSeq(csp, ";") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		directives[strings.ToLower(fields[0])] = strings.ToLower(strings.Join(fields[1:], " "))
	}
	sources, ok := directives["script-src"]
	if !ok {
		sources, ok = directives["default-src"]
	}
	if !ok {
		return false
	}
	return !strings.Contains(sources, "unpkg.com") &&
		!strings.Contains(sources, "*") &&
		!strings.Contains(sources, "https:")
}

func TestDocsPinsItsExternalAssets(t *testing.T) {
	t.Parallel()

	// 1. /docs on the API origin must not run a floating, integrity-free third-party script.
	docsResp, err := http.Get(itshared.ServerURL + "/docs")
	require.NoError(t, err)
	docsBody, err := io.ReadAll(docsResp.Body)
	_ = docsResp.Body.Close()
	require.NoError(t, err)
	csp := docsResp.Header.Get("Content-Security-Policy")
	t.Logf("GET /docs status=%d Content-Security-Policy=%q", docsResp.StatusCode, csp)

	if docsResp.StatusCode == http.StatusOK {
		cspConfines := caseCSPConfinesScripts(csp)
		tags := append(
			caseExternalScriptTag.FindAllString(string(docsBody), -1),
			caseExternalStyleTag.FindAllString(string(docsBody), -1)...,
		)
		for _, tag := range tags {
			m := caseTagURL.FindStringSubmatch(tag)
			require.Len(t, m, 2)
			url := m[1]
			hasIntegrity := caseIntegrityAttr.MatchString(tag)
			pinned := casePinnedVersion.MatchString(url)
			isScript := strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "<script")
			if isScript && cspConfines {
				continue
			}
			assert.True(t, hasIntegrity && pinned, "%s needs a pinned version and an integrity hash", url)
		}
	}

	// 2. Minimum boundary result: a same-origin request that carries only the
	// HttpOnly refresh cookie mints an access token for the dummy admin.
}
