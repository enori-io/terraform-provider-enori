// Terraform provider for Enori — entry point.
//
// STATUS: pre-alpha (2026-07-24). Compiles + `go vet`s clean on go1.26. Remaining before v0.1.0:
// flesh enori_monitor out to the full CreateMonitorRequest field set (docs/DESIGN.md §2), add
// acceptance tests, then publish to the Terraform Registry (docs/DESIGN.md §5).
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
