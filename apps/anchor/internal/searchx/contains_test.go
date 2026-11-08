package searchx_test

import (
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/stretchr/testify/assert"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/searchx"
)

// containsSQL renders the predicate Contains builds inside a minimal statement,
// so the test reads the SQL and the bound pattern the database receives.
func containsSQL(term string, more ...postgres.StringExpression) (string, []any) {
	return postgres.SELECT(table.Products.ID).
		FROM(table.Products).
		WHERE(searchx.Contains(term, table.Products.Name, more...)).
		Sql()
}

func TestContainsLowersBothSidesAndNamesTheEscape(t *testing.T) {
	t.Parallel()

	query, args := containsSQL("Atlas")

	assert.Contains(t, query, `LOWER(products.name) LIKE LOWER($1::text) ESCAPE $2::text`)
	assert.Equal(t, []any{"%Atlas%", `\`}, args)
}

func TestContainsEscapesLikeMetacharacters(t *testing.T) {
	t.Parallel()

	for term, pattern := range map[string]string{
		"100%":       `%100\%%`,
		"seat_cap":   `%seat\_cap%`,
		`C:\Temp`:    `%C:\\Temp%`,
		`\%_`:        `%\\\%\_%`,
		"Mixed Case": "%Mixed Case%",
		"":           "%%",
	} {
		_, args := containsSQL(term)
		assert.Equal(t, pattern, args[0], "term %q", term)
	}
}

func TestContainsMatchesAnyColumn(t *testing.T) {
	t.Parallel()

	query, args := containsSQL("atlas", table.Products.Description)

	assert.Contains(t, query, `(LOWER(products.name) LIKE LOWER($1::text) ESCAPE $2::text) OR `+
		`(LOWER(products.description) LIKE LOWER($3::text) ESCAPE $4::text)`)
	assert.Equal(t, []any{"%atlas%", `\`, "%atlas%", `\`}, args)
}
