package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/fx"

	"anchor/internal/stripeprototype"
)

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml openapi.yaml

//go:embed openapi.yaml
var specification []byte

func main() {
	configPath := flag.String("config", "", "private local prototype configuration file")
	flag.Parse()
	config, err := stripeprototype.LoadRuntimeConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fx.New(stripeprototype.Module(config, specification), fx.NopLogger, fx.StartTimeout(time.Minute)).Run()
}
