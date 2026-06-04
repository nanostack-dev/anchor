.PHONY: generate-client

generate-client:
	@echo "Generating Anchor Go client..."
	cd apps/anchor && sed -e '/x-go-type: string/!{/x-go-type:/d;}' \
		-e '/x-go-type-import:/d' \
		-e '/^[[:space:]]*path:[[:space:]]/d' \
		./cmd/http/openapi.yaml > ./cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=client-types-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=client-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd clients/go && go mod tidy
	rm -f apps/anchor/cmd/http/openapi-cleaned.yaml
