package main

import (
	"context"
	"log"

	"terraform-provider-nubes/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	version string = "0.0.1"
)

func main() {
	opts := providerserver.ServeOpts{
		Address: "terra.k8c.ru/nubes/nubes",
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err)
	}
}
