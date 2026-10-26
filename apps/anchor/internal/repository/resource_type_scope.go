package repository

import (
	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/domain/resourcetype"
)

func scopeCondition(column postgres.ColumnString, scopes resourcetype.ScopeFilter) postgres.BoolExpression {
	scoped := column.IN(jetx.ToStringExpressions(scopes.IDs)...)
	if scopes.IncludeUnscoped {
		return scoped.OR(column.IS_NULL())
	}
	return scoped
}
