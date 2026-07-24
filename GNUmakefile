# Developer tasks for terraform-provider-enori.
# `make` with no target runs build + unit tests.

NAME    := enori
BINARY  := terraform-provider-$(NAME)

default: build test

# Compile the provider binary.
build:
	go build -v ./...

# Vet + unit tests (no network; safe in CI).
test:
	go vet ./...
	go test -v -cover ./...

# Acceptance tests — hit the REAL Enori API and create/destroy real monitors.
# Requires: TF_ACC=1 and ENORI_API_KEY set to a key with monitors:read/write.
testacc:
	TF_ACC=1 go test -v -count=1 -timeout 30m ./internal/provider/

# Regenerate the Terraform Registry documentation under docs/ from the schema + examples/.
# Requires tfplugindocs: go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
docs:
	tfplugindocs generate --provider-name $(NAME)

# gofmt the tree.
fmt:
	gofmt -w -s .

# Install the provider into the local Terraform plugin dir for manual testing
# (pair with a dev_overrides block in ~/.terraformrc — see README).
install:
	go install .

.PHONY: default build test testacc docs fmt install
