package ct_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cursor-search indexes are one migration file each, with one CONCURRENTLY
// statement. golang-migrate sends a file to the server as a single query and
// opens no transaction of its own, so a lone statement builds without blocking
// writes; two statements in one file would run in an implicit transaction,
// where CONCURRENTLY is refused and a plain build locks the table.
var cursorSearchIndexMigration = regexp.MustCompile(`^0000(4[3-9]|5[0-3])_.*\.(up|down)\.sql$`)

func TestCursorSearchIndexMigrationsAreSingleConcurrentStatements(t *testing.T) {
	t.Parallel()

	t.Run("Cursor search index migrations are single concurrent statements", func(t *testing.T) {
		t.Parallel()

		files, err := filepath.Glob("../../../migrations/0000[45]*.sql")
		require.NoError(t, err)

		checked := 0
		for _, file := range files {
			name := filepath.Base(file)
			if !cursorSearchIndexMigration.MatchString(name) {
				continue
			}
			checked++
			raw, readErr := os.ReadFile(file)
			require.NoError(t, readErr)

			statements := splitStatements(string(raw))
			require.Len(t, statements, 1, "%s must hold exactly one statement", name)
			if strings.HasSuffix(name, ".up.sql") {
				assert.Regexp(t, `^CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_\w+ ON \w+ \(`, statements[0], name)
			} else {
				assert.Regexp(t, `^DROP INDEX CONCURRENTLY IF EXISTS idx_\w+$`, statements[0], name)
			}
		}
		assert.Equal(t, 22, checked, "eleven indexes, an up and a down file each")
	})
}

func TestCursorSearchIndexesAreValid(t *testing.T) {
	t.Parallel()

	t.Run("Cursor search indexes are valid", func(t *testing.T) {
		t.Parallel()

		// A concurrent build that fails leaves an INVALID index behind, and the
		// IF NOT EXISTS of a rerun would skip it: validity is part of the contract.
		var valid, total int
		require.NoError(t, testDB.QueryRowContext(t.Context(), `
		SELECT count(*) FILTER (WHERE i.indisvalid AND i.indisready), count(*)
		FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname ~ '^idx_.*_created_(id|user)$'`).Scan(&valid, &total))

		// Ten built concurrently on existing tables (000073 dropped the workspaces
		// one), plus the resource type
		// search index, built with its table in 000054.
		assert.Equal(t, 11, total)
		assert.Equal(t, 11, valid)
	})
}

func splitStatements(sql string) []string {
	var lines []string
	for line := range strings.SplitSeq(sql, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			lines = append(lines, trimmed)
		}
	}
	var statements []string
	for statement := range strings.SplitSeq(strings.Join(lines, " "), ";") {
		if strings.TrimSpace(statement) != "" {
			statements = append(statements, strings.TrimSpace(statement))
		}
	}
	return statements
}
