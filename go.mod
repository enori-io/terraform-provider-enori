module github.com/hpatsev/terraform-provider-enori

go 1.21

require (
	github.com/hashicorp/terraform-plugin-framework v1.11.0
	github.com/hashicorp/terraform-plugin-framework-validators v0.13.0
	github.com/hashicorp/terraform-plugin-go v0.23.0
	github.com/hashicorp/terraform-plugin-log v0.9.0
)

// NOTE: run `go mod tidy` once Go is installed to resolve the full transitive set +
// generate go.sum. Versions above are the last-known-good set for the framework line.
