# Changelog

All notable changes to the Enori Terraform provider are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Provider configuration: `api_key` (or `ENORI_API_KEY`) + optional `endpoint` (`ENORI_ENDPOINT`,
  default `https://api.enori.io`).
- `enori_monitor` resource — manage Enori uptime monitors as code. Covers the common cross-type +
  HTTP/website + alerting arguments (name, url, type, group, interval/timeout, HTTP method / expected
  status / keyword / body / user-agent / follow-redirects, port, SSL expiry warning, failure threshold,
  alert on down/recovered, alert channels, tags). Supports create/read/update/delete + `terraform import`.

### Notes

- `type` is immutable (changing it forces a new monitor) and restricted to `website`, `ping`, `port`,
  `dns`, `domain`, `job`. Legacy platform aliases are folded to `website` on read.
- Updates are a partial merge: removing an optional argument keeps its last value — set it explicitly to
  change or clear it (e.g. `tags = []`). See the README for the full argument reference and caveats.

_No released versions yet. `v0.1.0` will be the first tag published to the Terraform Registry._
