# fortigate-cli — design & roadmap

`fgt` is a remote-first CLI for FortiOS, modeled on [`pve-cli`][pvecli]. This
doc records the architecture and the roadmap; the "why" is backed by research
into the existing tool landscape (see the bottom).

> **API knowledge base**: the FortiOS REST mechanics and per-version object model
> that the implementation wraps are documented in [`docs/api/`](api/) —
> [`rest-conventions.md`](api/rest-conventions.md) (auth, cmdb/monitor, envelope,
> `action=schema`, error codes), the `objects-*.md` field references, and
> [`version-matrix.md`](api/version-matrix.md) (authoritative 7.4/7.6/8.0 field
> deltas, extracted from the CLI References via `tools/fortios-cli-extract.py`).

## Architecture (mirrors pve-cli)

```
cmd/fgt/main.go            → thin entrypoint, calls cli.Execute()
internal/
  cli/                     → cobra command tree (curated UX) + `api` escape hatch
  provider/                → backend interface (Provider); New() is the one entry
    fortigate/             → FortiOS REST implementation (registers via init())
  transport/               → base HTTPS client: auth, TLS, retries, rate limit, VDOM
  auth/                    → pluggable auth (API token + session/logincheck)
  config/                  → kubeconfig-style profiles+contexts; precedence resolve
  protocol/                → FortiOS response envelope + error decoding
  output/                  → table/json/yaml/csv/value renderer
  domain/                  → backend-agnostic value types the CLI renders
  version/                 → build metadata + supported-FortiOS matrix
```

Key parallels to pve-cli: same Go + spf13/cobra stack; a `Provider` interface so
a **FortiManager** backend can be added without touching the CLI layer (the way
pve-cli abstracts PVE vs PDM); OS-keyring secrets with env-var bypass for CI;
and the **curated commands + raw escape hatch** so full API coverage doesn't
block on hand-writing hundreds of cmdb tables.

## FortiOS specifics the design accounts for

| Concern | Decision |
|---|---|
| Config plane | `GET/POST/PUT/DELETE /api/v2/cmdb/<path>` |
| Status plane | `GET /api/v2/monitor/<path>` |
| Auth | API token (`Authorization: Bearer <token>`) or session (`/logincheck` cookie + `X-CSRFTOKEN`); HTTPS enforced (transport refuses plaintext to non-loopback) |
| Multi-tenancy | `--vdom` / `FGT_CLI_VDOM` → `?vdom=` on every call |
| Self-signed certs | pin SHA-256 fingerprint (preferred) or `--insecure` |
| Write bodies | JSON objects (`--body`/`--data`), not form-encoding |
| Response shape | payload under `results`; `protocol.DecodeData` unwraps it |

## Roadmap

- **M1** ✅ — transport/auth/config/output/protocol, `api` escape hatch, monitor
  reads (`system interface`, `switch`).
- **M2** ✅ — curated CRUD via a **declarative resource framework**
  (`internal/cli/resource.go`): generic cmdb `List/Get/Create/Update/Delete` on
  the provider, and each object is a data declaration (path, mkey, columns,
  fields). `--set key=value` reaches any un-modeled field; ref child-tables
  (`srcaddr`, `member`, …) take comma lists. Shipped:
  - `firewall address`/`addrgrp`/`service custom`/`service group`/`policy`.
  - `router static`, and **dynamic routing** `router bgp`/`ospf` (singletons;
    neighbor/area/network child-tables via `--set`) plus `route-map`/
    `prefix-list`/`access-list` (thin wrappers — `rule` child-tables via `--set`
    until M5 codegen).
  - `system admin`, `system dns` (singleton), `system interface` (cmdb
    show/create/set/delete; `list` stays on the monitor surface for live
    status/IP), **VDOM administration** `system vdom` CRUD, and **HA clustering**
    `system ha` config (singleton show/set) + `system ha status`
    (`monitor/system/ha-statistics`). VDOM *scoping* (`--vdom`) shipped in M1.
  - Deferred to later milestones: inter-VDOM links / `npu-vlink` (see NPU
    bullet), richer per-object columns, and promoting the child-table-heavy
    objects (bgp neighbors, route-map rules) into typed sub-commands (M5).
- **Hardware acceleration** (NPU) — `system/npu`, `npu-vlink` accelerated
  inter-VDOM links, and per-interface/per-policy offload knobs (`auto-asic-offload`).
  Advanced/appliance-focused; reachable via `api` today, curated later.
- **M3 — FortiSwitch depth**: `switch-controller.*` — managed-switch port config,
  VLAN assignment, PoE, stacking/tier, firmware, plus port status from monitor.
- **M4** ✅ — session auth (`POST /logincheck`, shared cookie jar +
  `X-CSRFTOKEN` on writes), so password admins work where tokens aren't
  provisioned. `auth.type: session` with `user` + a password (`secret_ref`,
  `FGT_CLI_PASSWORD`, or `--password`); login is lazy and reused per invocation.
  Token auth remains the default and automation-safe path.
- **M5 — schema-driven scaffolding**: generate command/flag trees from FortiOS
  per-object cmdb schemas (each object exposes its own field schema) or from the
  Terraform provider's resource map, promoting raw endpoints into typed commands.
- **M6 — FortiManager backend**: a second `Provider` targeting FortiManager's
  JSON-RPC API for fleet-wide management.
- **Cross-cutting**: config backup/restore (`/api/v2/backup`), man pages +
  shell completions + `docs` target, GoReleaser `.deb`/`.rpm`, OBS packaging
  (all as in pve-cli).

## Open questions

1. ~~Is the per-object cmdb field schema stable across 7.2/7.4/7.6/8.0?~~
   **Answered** (see [`api/version-matrix.md`](api/version-matrix.md)): the schema
   shape is stable but the field set drifts every release (e.g. post-quantum
   `addke*` added in 7.6; FortiAI admin fields and ZTNA policy fields in 8.0;
   `hw-model`→`hw-version` rename in 8.0). So M5 codegen must read `?action=schema`
   from the target build rather than bake a fixed table; the `api` escape hatch
   already gives full coverage on every version with no code change.
2. Should FortiManager be a first-class backend (M6) given it fronts both
   FortiGates and FortiSwitches?
3. Which `switch-controller.*` endpoints make a genuinely useful FortiSwitch
   surface (M3) beyond status reads?

## Research basis (2026-09-18)

No feature-complete FortiGate CLI exists. Candidates split into stale read-only
CLIs (e.g. `malwan23/FG-CLI` — GET-only, last push 2024-03) and importable
Python libraries (`fortiosapi` — archived 2026-04-15; `vladimirs-git/fortigate-api`;
`DavidChayla/FortigateApi` — FortiOS 5.2/5.4 era; `pyFGT`; `gatepy`;
`pyFortinetAPI`). Fortinet's official automation is the `fortinet.fortios`
Ansible collection plus a Terraform provider — broad coverage, **no CLI binary**.
The FortiOS REST API is a clean, versioned HTTPS surface, making it a good target
for a `pve-cli`-style client.

[pvecli]: https://github.com/ciroiriarte/pve-cli
