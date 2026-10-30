.PHONY: generate-client

generate-client:
	@echo "Generating Anchor Go client..."
	./scripts/client-spec.sh apps/anchor/cmd/http/openapi.yaml apps/anchor/cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=client-types-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=client-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd clients/go && go mod tidy
	rm -f apps/anchor/cmd/http/openapi-cleaned.yaml
