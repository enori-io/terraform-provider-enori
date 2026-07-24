# Terraform Provider for Enori

Manage your [Enori](https://enori.io) uptime monitors as code.

> **⚠️ Status: pre-alpha (2026-07-24).** The repository structure, CI, and a first-draft
> `enori_monitor` resource are in place and **compile + `go vet` clean** on go1.26. Remaining
> before `v0.1.0`: flesh `enori_monitor` out to the full attribute set, add acceptance tests,
> and **publish to the Terraform Registry** (not yet published). See [Building](#building).

## Overview

The Enori provider lets you declare uptime monitors (and, from `v0.2.0`, alert channels) in
HCL and manage them through the Enori public REST API. It is built on the
[terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework).

## Example usage

```hcl
terraform {
  required_providers {
    enori = {
      source  = "hpatsev/enori"
      version = "~> 0.1"
    }
  }
}

provider "enori" {
  # api_key is read from the ENORI_API_KEY environment variable (recommended).
  # endpoint defaults to https://api.enori.io.
}

resource "enori_monitor" "marketing_site" {
  name                 = "Marketing site"
  url                  = "https://www.example.com"
  type                 = "website"
  interval_seconds     = 60
  expected_status_code = 200
  expected_keyword     = "Welcome"
  follow_redirects     = true
  alert_on_down        = true
  tags                 = ["production", "marketing"]
}
```

## Authentication

The provider authenticates with an Enori **API key** (sent as the `X-Api-Key` header).

1. Create a key in the Enori dashboard: **Settings → API Keys**.
2. Grant it the scopes the provider needs:
   - `monitors:read`, `monitors:write` — required for `enori_monitor`.
   - `alerts:read`, `alerts:write` — required for `enori_alert_channel` (from `v0.2.0`).
3. Export it before running Terraform:

   ```bash
   export ENORI_API_KEY="ek_live_..."
   ```

Prefer the environment variable over the `api_key` provider attribute so the key never lands
in a `.tf` file or in state/plan output. When set in config, mark it `sensitive`.

| Setting    | Provider attribute | Environment variable | Default                 |
| ---------- | ------------------ | -------------------- | ----------------------- |
| API key    | `api_key`          | `ENORI_API_KEY`      | — (required)            |
| API base   | `endpoint`         | `ENORI_ENDPOINT`     | `https://api.enori.io`  |

## Resources

| Resource              | Status         | Notes                                              |
| --------------------- | -------------- | -------------------------------------------------- |
| `enori_monitor`       | pre-alpha (P1) | Common cross-type + HTTP/website + alerting core.  |
| `enori_alert_channel` | planned (P2)   | email / slack / discord / webhook channels.        |

### `enori_monitor` — argument reference

**Required**

| Argument | Type   | Notes |
| -------- | ------ | ----- |
| `name`   | string | Monitor name (1–100 chars). |
| `url`    | string | Target URL or host. |
| `type`   | string | One of `website`, `ping`, `port`, `dns`, `domain`, `job` (lowercase). **Immutable** — changing it forces a new monitor. |

**Optional** (the server supplies a default for most; whatever the server stores is read back)

| Argument | Type | Notes |
| -------- | ---- | ----- |
| `group_name` | string | Group the monitor belongs to. |
| `interval_seconds` | number | Check interval (30–31104000). Default 300. |
| `timeout_seconds` | number | Per-check timeout (5–300). Default 30. |
| `http_method` | string | e.g. `GET`, `POST`, `HEAD`. Default `GET`. |
| `expected_status_code` | number | Expected HTTP status (100–599). Default 200. |
| `expected_keyword` | string | Keyword required in the response body. |
| `request_body` | string | Body sent with the check. |
| `custom_user_agent` | string | Custom User-Agent (max 512 chars). |
| `follow_redirects` | bool | Follow HTTP redirects. Default true. |
| `port` | number | Target port (1–65535), for port checks. |
| `ssl_expiry_warning_days` | number | Warn N days before SSL expiry (7/14/30/60). Default 30. |
| `failure_threshold` | number | Consecutive failures before down (0–10). |
| `alert_on_down` | bool | Alert when the monitor goes down. Default true. |
| `alert_on_recovered` | bool | Alert when the monitor recovers. Default true. |
| `alert_channel_ids` | set(string) | Alert channels to notify. |
| `tags` | set(string) | Tags (lowercase alphanumeric + hyphens). |

**Computed:** `id` — the Enori monitor id.

> **Partial-update caveat.** The Enori API applies updates as a partial merge, so **removing** an
> optional argument from your config keeps its last value rather than clearing it — set the argument
> explicitly (e.g. `expected_keyword = ""`) to change it. Advanced type-specific config (browser
> steps, ApiFlow, DNS record matching, device emulation, encrypted variables) is **not** yet exposed;
> see [`docs/DESIGN.md`](docs/DESIGN.md) §2 for the roadmap.

Import an existing monitor by its Enori id:

```bash
terraform import enori_monitor.marketing_site <monitor-id>
```

## Building

This scaffold needs a Go toolchain to compile and a Registry account + GPG key to publish.

```bash
# 1. Install Go 1.21+  (https://go.dev/dl/)
# 2. Resolve dependencies and compile:
go mod tidy
go build ./...
go vet ./...

# 3. Run acceptance tests (requires a real API key against a test account):
#    TF_ACC=1 ENORI_API_KEY=... go test ./... -v
```

**Remaining before `v0.1.0`:**

1. `go mod tidy && go build ./...` — compile the first-draft code and fix anything the
   compiler flags (the code was written without a local Go toolchain).
2. Flesh out `enori_monitor` to the full `CreateMonitorRequest` attribute set (see
   [`docs/DESIGN.md`](docs/DESIGN.md) §2) and add acceptance tests.
3. Create a [Terraform Registry](https://registry.terraform.io) account under `hpatsev` and
   connect this GitHub repo.
4. Generate a GPG signing key, add the **private** key + passphrase as the `GPG_PRIVATE_KEY`
   / `PASSPHRASE` repo secrets, and register the **public** key in the Registry.
5. Tag `v0.1.0` — the release workflow (`.github/workflows/release.yml`) builds the
   cross-platform binaries with GoReleaser and publishes them; the Registry picks up the tag.

## Documentation

- Provider design + roadmap: [`docs/DESIGN.md`](docs/DESIGN.md)
- Enori public API: `https://api.enori.io/scalar/v1`

## License

[MPL-2.0](LICENSE) — the standard license for Terraform providers.
