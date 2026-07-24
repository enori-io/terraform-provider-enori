# Terraform Provider — Design & Build Plan (v1)

**Thread:** T3 (chief-engineer). **Status:** DESIGN — build blocked on operator prerequisites (§4). **2026-07-24.**

Companion to the existing implementation blueprint **`docs/terraform-provider-prompt.md`** (1314 lines — the phase-by-phase build instructions). This doc is the reconciliation of that prompt against the **current** Enori API + the MVP-scope + operator-prereq decisions. Read this first, then the prompt.

---

## 1. What it is (one paragraph)

A **standalone Go binary** (`terraform-provider-enori`, its own repo — NOT the .NET monorepo) built on HashiCorp's `terraform-plugin-framework` v1.5+, published to the **Terraform Registry** (`registry.terraform.io/providers/<namespace>/enori`). It lets customers manage Enori resources as Infrastructure-as-Code: `terraform apply` maps declarative HCL to `POST/PUT/DELETE` calls on the Enori public REST API, `terraform plan` shows the diff, `terraform destroy` tears down. Authenticates with an **Enori API key** (generated in Settings → API Keys) — which is why there is no "Terraform tab": the provider is a CLI plugin, not a UI surface.

```hcl
terraform {
  required_providers { enori = { source = "<namespace>/enori" } }
}
provider "enori" { api_key = var.enori_api_key }   # X-Api-Key header

resource "enori_monitor" "api" {
  name             = "Production API"
  type             = "website"
  url              = "https://api.example.com/health"
  interval_seconds = 60
}
```

## 2. MVP scope (RATIFY)

Two resources, matching `docs/terraform-provider-prompt.md`:
- **`enori_monitor`** — the flagship. CRUD against `/api/monitors`. Attributes map to `CreateMonitorRequest`/`MonitorDto` (`src/UpNest.Api/DTOs/MonitorDto.cs`): `name`, `group_name?`, `url`, `type` (website/browser/apiflow/ping/port/dns/domain/job), `interval_seconds`, `timeout_seconds`, `grace_period_seconds`, `schedule_description?`, `cron_expression?`, alerting flags, … (full field map in the prompt — reconcile against the DTO at build time, it is the source of truth).
- **`enori_alert_channel`** — CRUD against `/api/alert-channels`.

Expansion later (own phases): `enori_status_page`, `enori_slo`, `enori_escalation_policy`, `enori_maintenance_window`. Recommended start: **both MVP resources** (the prompt already scaffolds both) — `enori_monitor` alone is the smaller option if we want to prove the pipeline fastest.

## 3. Premise-verify errata (reconcile the prompt against CURRENT code before building)

The prompt predates recent API changes — the architect premise-grep found:

1. **API base host — `api.enori.io`, NOT `app.enori.io`.** The prompt (§ base URL) says `https://app.enori.io`; that is the frontend/dashboard. The public REST API the frontend + MCP clients call is **`https://api.enori.io`** (X-Api-Key auth confirmed in `tools/enori-appinsights-mcp`). The provider MUST target `api.enori.io`.
2. **Required API-key scopes (NEW — from T1, #931).** `AlertChannelsController` is now gated under **`alerts:read`/`alerts:write`** (added in the C10 settings-polish thread), and `MonitorsController` under `monitors:read`/`monitors:write`. So a provider API key needs **`monitors:read/write` + `alerts:read/write`**. Those four scopes are user-requestable as of #931 — document them in the provider's README ("create a key with these scopes"). The provider is only fully functional once #931 is merged (it is).
3. **`MonitorType` enum values** — reconcile the HCL `type` string ↔ `MonitorType` (Website=10/Browser=11/ApiFlow=12/Ping=3/Port=2/Dns=4/Domain=6/Job=8; legacy aliases auto-migrate). The provider should accept the human names and map to the API's expected representation (verify whether the API takes the string or the int at build time).
4. **Duplicate-monitor 409** (V024 unique index on `user_id,url,type`) — the provider's Create must surface this as a clean Terraform error, and Read must support import so an already-existing monitor can be adopted.

## 4. Operator prerequisites — BUILD IS BLOCKED until these are done (Hristo)

None block this design; **all block the Go build.** Surfaced, not actioned (chief boundary — new repo / toolchain / external account are operator decisions):

- [ ] **Install Go** on the build machine (`brew install go`, ≥ 1.21). Not currently installed on this Mac.
- [ ] **Create the GitHub repo** `terraform-provider-enori` (public — the Terraform Registry only lists public repos). I can `gh repo create` on your explicit OK, or you create it.
- [ ] **Terraform Registry account + namespace.** Sign in to registry.terraform.io with the GitHub org/user that will own the `<namespace>/enori` provider; connect the repo.
- [ ] **GPG signing key** for releases (the Registry requires GPG-signed release artifacts; GoReleaser + a `GPG_PRIVATE_KEY` GitHub Actions secret is the standard pipeline).
- [ ] **A test API key** with `monitors:read/write` + `alerts:read/write` for the acceptance tests (create in Settings → API Keys).

## 5. Phased build plan (once §4 is unblocked)

Follows `docs/terraform-provider-prompt.md`; each phase = its own PR on the new repo:

- **P1 — Scaffold + provider config + `enori_monitor` CRUD.** Repo skeleton, `terraform-plugin-framework` wiring, provider `api_key` config, the HTTP client (X-Api-Key → `api.enori.io`), `enori_monitor` Create/Read/Update/Delete + Import. Acceptance tests (against a real key).
- **P2 — `enori_alert_channel`** CRUD + import + tests.
- **P3 — Registry publishing pipeline.** GoReleaser config, GitHub Actions release workflow (GPG-signed), `docs/` for the registry, `terraform-registry-manifest.json`. First tagged release → Registry listing.
- **P4 — Usage guide** in the Enori API docs (`content/api-docs.ts` or a `content/docs/terraform.md`) — "configure the provider + example HCL + the scopes your key needs". This is the only in-product surface.
- **P5+ (backlog)** — `enori_status_page`, `enori_slo`, `enori_escalation_policy`, `enori_maintenance_window`.

## 6. Marketing reconciliation

The public site advertises a Terraform provider in **4 places** (FeaturesSection, PublicFooter, PricingTable, landing). Building this makes the claim honest — no marketing change needed once P1–P3 ship and the Registry listing is live. (The alternative — pulling the claims — was rejected by Hristo.)

## 7. Known limitations (v0.1.0) — deferred, with reasons

- **Advanced type-specific config not modeled.** `enori_monitor` covers the common cross-type +
  HTTP/website + alerting core (see README argument reference). Browser steps, ApiFlow steps, DNS
  record matching / routing, device emulation, and encrypted variables are **not** exposed. Reason:
  they are large, type-specific sub-objects; shipping a clean core first is more valuable than a
  half-modelled everything. Roadmapped for later versions.
- **`type` restricted to the 6 basic types** (`website`, `ping`, `port`, `dns`, `domain`, `job`) via a
  `OneOf` validator. The five deprecated aliases the API may still return for old monitors
  (`http`/`api`/`ssl`/`https`/`reputation`) are **folded to `website`** on read (they are unified into
  Website behaviourally), so importing a legacy-type monitor works cleanly. Browser/ApiFlow are excluded
  because they require step definitions the provider does not model yet — importing one populates state
  but its `type` cannot be expressed in config until those types are supported (a `terraform import` of a
  Browser/ApiFlow monitor is therefore not recommended in v0.1.0).
- **Partial-update can't-clear caveat.** The Enori update endpoint is a partial merge (a null field =
  "no change"), so *removing* an optional argument from config keeps its last value. Set the argument
  explicitly (e.g. `expected_keyword = ""`, `tags = []`) to change/clear it. Documented in the README.
- **`Update` on a 404 returns a generic error** rather than dropping the resource from state (the way
  `Read`/`Delete` do). Reason (deferred P2 from the 2026-07-24 code review): this only occurs when a
  monitor is deleted out-of-band *within a single apply window* (a rare race), and erroring rather than
  silently recreating is a defensible, common provider choice. Revisit if it surfaces in practice.
