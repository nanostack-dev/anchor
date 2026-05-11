package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	providerpkg "github.com/nanostack-dev/anchor/terraform/provider/internal/provider"
)

func main() {
		err := providerserver.Serve(context.Background(), providerpkg.New, providerserver.ServeOpts{
			Address: "registry.terraform.io/nanostack-dev/anchor",
		})
	if err != nil {
		log.Fatal(err.Error())
	}
}
