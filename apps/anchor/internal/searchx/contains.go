package searchx

import (
	"strings"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

// likeEscape is the escape character every search pattern uses. It is
// PostgreSQL's default, but Contains names it in an ESCAPE clause so the
// pattern does not depend on that default.
const likeEscape = `\`

// Contains matches term as a literal, case-insensitive substring of any of the
// columns. It is the one way a search filters on free text.
//
// The term is literal: % and _ match themselves, so a customer named
// "100% Renewable" is found by typing its name, and "seat_cap" does not also
// find "seatXcap". Case is ignored because both sides go through LOWER: a person
// typing into a search box does not capitalise the way the record does. A NULL
// column does not match.
//
// The predicate is built for every term, the empty one included, which matches
// every row whose column is not NULL. A caller that treats an empty term as "no
// filter" checks for it before calling.
func Contains(
	term string, column postgres.StringExpression, more ...postgres.StringExpression,
) postgres.BoolExpression {
	escaped := strings.NewReplacer(
		likeEscape, likeEscape+likeEscape, `%`, likeEscape+`%`, `_`, likeEscape+`_`,
	).Replace(term)
	pattern := postgres.LOWER(postgres.String("%" + escaped + "%"))
	match := containsPattern(column, pattern)
	for _, other := range more {
		match = match.OR(containsPattern(other, pattern))
	}
	return match
}

func containsPattern(column, pattern postgres.StringExpression) postgres.BoolExpression {
	return postgres.BoolExp(postgres.CustomExpression(
		postgres.LOWER(column), postgres.Token("LIKE"), pattern,
		postgres.Token("ESCAPE"), postgres.String(likeEscape),
	))
}
