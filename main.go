// Terraform provider for Enori — entry point.
//
// STATUS: first-draft scaffold (2026-07-24). Authored without a local Go toolchain, so it is
// NOT yet compiled — the first build task once Go is installed is `go mod tidy && go build ./... &&
// go vet ./...`, then fix anything the compiler flags. See docs/DESIGN.md §5 (P1).
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hpatsev/terraform-provider-enori/internal/provider"
)

// version is set by the release build (GoReleaser ldflags); "dev" for local runs.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		// Must match the Terraform Registry source address: registry.terraform.io/hpatsev/enori
		Address: "registry.terraform.io/hpatsev/enori",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
