// Package searchcontains pins how searchexec.Contains, which every free-text
// filter now uses, differs from the Contains that anchor wrote in #339.
package searchcontains_test

import (
	"strings"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/stretchr/testify/assert"

	"anchor/internal/db/gen/anchor/public/table"
)

func renderedContains(term string) (string, []any) {
	return postgres.SELECT(table.Products.ID).
		FROM(table.Products).
		WHERE(searchexec.Contains(term, table.Products.Name)).
		Sql()
}

// legacyPattern is the pattern the #339 Contains bound: no trim, backslash as
// the escape character, and the empty term still a LIKE on '%%'.
func legacyPattern(term string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
	return "%" + escaped + "%"
}

func TestContainsUsesCaseFoldedLikeWithABangEscape(t *testing.T) {
	t.Parallel()

	query, args := renderedContains("Atlas")

	assert.Contains(t, query, `products.name ILIKE $1 ESCAPE '!'`)
	assert.Equal(t, []any{"%Atlas%"}, args)
}

func TestContainsTrimsTheTerm(t *testing.T) {
	t.Parallel()

	_, args := renderedContains("  Atlas \t")

	assert.Equal(t, []any{"%Atlas%"}, args)
	assert.Equal(t, "%  Atlas \t%", legacyPattern("  Atlas \t"), "the #339 Contains kept the spaces")
}

func TestContainsTreatsABlankTermAsNoFilter(t *testing.T) {
	t.Parallel()

	for _, term := range []string{"", "   ", "\t\n"} {
		query, args := renderedContains(term)

		assert.Contains(t, query, "WHERE TRUE", "term %q", term)
		assert.Empty(t, args, "term %q", term)
	}
	assert.Equal(t, "%%", legacyPattern(""), "the #339 Contains still built a LIKE, which drops NULL columns")
}

func TestContainsEscapesWithABangNotABackslash(t *testing.T) {
	t.Parallel()

	for term, pattern := range map[string]string{
		"100%":     `%100!%%`,
		"seat_cap": `%seat!_cap%`,
		"a!b":      `%a!!b%`,
		`C:\Temp`:  `%C:\Temp%`,
	} {
		_, args := renderedContains(term)
		assert.Equal(t, []any{pattern}, args, "term %q", term)
	}
}

func TestContainsAnswersFalseForATermNoTextColumnCanHold(t *testing.T) {
	t.Parallel()

	for _, term := range []string{"a\x00b", "a\xffb"} {
		query, args := renderedContains(term)

		assert.Contains(t, query, "WHERE FALSE", "term %q", term)
		assert.Empty(t, args, "term %q", term)
	}
}

func TestContainsMatchesAnyColumn(t *testing.T) {
	t.Parallel()

	query, args := postgres.SELECT(table.Products.ID).
		FROM(table.Products).
		WHERE(searchexec.Contains("atlas", table.Products.Name, table.Products.Description)).
		Sql()

	assert.Contains(t, query, "products.name ILIKE $1 ESCAPE '!'")
	assert.Contains(t, query, "products.description ILIKE $2 ESCAPE '!'")
	assert.Contains(t, query, "OR")
	assert.Equal(t, []any{"%atlas%", "%atlas%"}, args)
}
