# fortigate-cli — design & roadmap

`fgt` is a remote-first CLI for FortiOS, modeled on [`pve-cli`][pvecli]. This
doc records the architecture and the roadmap; the "why" is backed by research
into the existing tool landscape (see the bottom).

## Architecture (mirrors pve-cli)

```
cmd/fgt/main.go            → thin entrypoint, calls cli.Execute()
internal/
  cli/                     → cobra command tree (curated UX) + `api` escape hatch
  provider/                → backend interface (Provider); New() is the one entry
    fortigate/             → FortiOS REST implementation (registers via init())
  transport/               → base HTTPS client: auth, TLS, retries, rate limit, VDOM
  auth/                    → pluggable auth (token now; session later)
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
| Auth | `Authorization: Bearer <token>`; HTTPS enforced (transport refuses plaintext to non-loopback) |
| Multi-tenancy | `--vdom` / `FGT_CLI_VDOM` → `?vdom=` on every call |
| Self-signed certs | pin SHA-256 fingerprint (preferred) or `--insecure` |
| Write bodies | JSON objects (`--body`/`--data`), not form-encoding |
| Response shape | payload under `results`; `protocol.DecodeData` unwraps it |

## Roadmap

- **M1 (this scaffold)** — transport/auth/config/output/protocol, `api` escape
  hatch, first curated slice: `system interface`, `firewall address`, `switch`.
- **M2 — curated CRUD** for high-value objects: firewall `policy`/`address`/
  `service`/`addrgrp`, `system interface`/`admin`/`dns`, `router static`, `vpn`.
  Add `create`/`show`/`set`/`delete` verbs on top of the generic client.
- **M3 — FortiSwitch depth**: `switch-controller.*` — managed-switch port config,
  VLAN assignment, PoE, stacking/tier, firmware, plus port status from monitor.
- **M4 — session auth** (`POST /logincheck`, cookie + `X-CSRFTOKEN`), so
  password admins work where tokens aren't provisioned.
- **M5 — schema-driven scaffolding**: generate command/flag trees from FortiOS
  per-object cmdb schemas (each object exposes its own field schema) or from the
  Terraform provider's resource map, promoting raw endpoints into typed commands.
- **M6 — FortiManager backend**: a second `Provider` targeting FortiManager's
  JSON-RPC API for fleet-wide management.
- **Cross-cutting**: config backup/restore (`/api/v2/backup`), man pages +
  shell completions + `docs` target, GoReleaser `.deb`/`.rpm`, OBS packaging
  (all as in pve-cli).

## Open questions

1. Is the per-object cmdb field schema stable/machine-readable enough across
   7.2/7.4/7.6/8.0 to drive codegen (M5), or is the raw passthrough the only
   maintainable route to "full coverage"?
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
