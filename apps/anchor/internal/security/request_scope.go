package security

import "context"

type requestScopeKey string

const productScopeKey requestScopeKey = "product_scope"

type ProductScope struct {
	ProductID string
}

func SetProductScope(ctx context.Context, scope ProductScope) context.Context {
	return context.WithValue(ctx, productScopeKey, scope)
}

func GetProductScope(ctx context.Context) (ProductScope, bool) {
	scope, ok := ctx.Value(productScopeKey).(ProductScope)
	return scope, ok
}
