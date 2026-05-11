//go:build tools

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=codegen.yaml ./cmd/http/openapi.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=client-types-codegen.yaml ./cmd/http/openapi-cleaned.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=client-codegen.yaml ./cmd/http/openapi-cleaned.yaml
package main

import (
	"fmt"

	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)

func main() {
	fmt.Println("OAPI codegen tool in use!")
}
