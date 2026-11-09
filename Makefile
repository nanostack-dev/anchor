.PHONY: generate-client contract-test contract-update contract-bench lint lint-fix

generate-client:
	@echo "Generating Anchor Go client..."
	./scripts/client-spec.sh apps/anchor/cmd/http/openapi.yaml apps/anchor/cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go tool oapi-codegen \
		--config=client-types-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd apps/anchor && go tool oapi-codegen \
		--config=client-codegen.yaml ./cmd/http/openapi-cleaned.yaml
	cd clients/go && go mod tidy
	rm -f apps/anchor/cmd/http/openapi-cleaned.yaml

# Golden contract suite (apps/anchor/cmd/it/contract, build tag contract).
# Needs Docker. See apps/anchor/cmd/it/contract/README.md.
contract-test:
	cd apps/anchor && go test -tags contract -count=1 -timeout 30m ./cmd/it/contract/

contract-update:
	cd apps/anchor && go test -tags contract -count=1 -timeout 30m ./cmd/it/contract/ -update

contract-bench:
	cd apps/anchor && go test -tags contract -run '^$$' -bench . -benchmem -count=10 -timeout 60m ./cmd/it/contract/

# Full backend lint with the golangci-lint pinned in
# apps/anchor/tools/golangci-lint/go.mod (built by `go tool`, as CI runs it).
lint:
	./scripts/lint.sh backend

lint-fix:
	./scripts/lint.sh backend --fix
