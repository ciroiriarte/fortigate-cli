# FortiOS API knowledge base

Reference material for implementing `fgt`, distilled from Fortinet's own
FortiOS documentation (7.4.12, 7.6.7, 8.0.0/8.0.1 Administration Guides and CLI
References) and cross-checked against the `fortinet.fortios` Ansible collection
and the `fortinetdev/fortios` Terraform provider.

## Why these docs exist

A feature-complete FortiOS CLI has to wrap a large, versioned object model. This
folder captures the parts the implementation actually needs, per FortiOS
version, so command authors don't re-derive them each time.

## Contents

| File | What it covers |
|---|---|
| [`rest-conventions.md`](rest-conventions.md) | REST mechanics: base URL, `cmdb` vs `monitor`, auth (Bearer token + session/CSRF), response envelope, query params (`filter`, `format`, `vdom`, **`action=schema`**), status/error codes, gotchas, and **auth version deltas** (7.4/7.6/8.0). |
| [`objects-firewall.md`](objects-firewall.md) | `firewall/address`, `addrgrp`, `service/custom`, `service/group`, `policy` — field tables, mkeys, child-tables, version deltas. |
| [`objects-system-routing.md`](objects-system-routing.md) | `system/interface`, `system/admin`, `system/dns`, `router/static`, `router/policy` + the key `monitor/*` endpoints. |
| [`objects-switch-vpn.md`](objects-switch-vpn.md) | `switch-controller/managed-switch` (+ `ports`), `lldp-*`, VLAN modeling, `vpn.ipsec/phase1-interface`, `phase2-interface` + monitor status paths. |
| [`version-matrix.md`](version-matrix.md) | **Authoritative** per-version field counts and add/remove deltas across 7.4.12 / 7.6.7 / 8.0.1, extracted directly from the CLI References. |
| [`field-inventory.json`](field-inventory.json) | Machine-readable: every target object's top-level fields + child-tables, per version. Feeds M5 codegen. |

## Ground-truth hierarchy (most to least authoritative)

1. **`GET /api/v2/cmdb/<path>?action=schema` on a live unit of the exact FortiOS
   build.** FortiOS self-describes its object model per build — field names,
   types, enums, defaults, and which field is the mkey. This is the single
   source of truth and matches `fgt`'s own raw/escape-hatch philosophy. Use it
   before hard-coding anything version-specific.
2. **The FortiOS CLI Reference PDF for that version** (in `references/`, gitignored).
   The `config <path>` trees map 1:1 to `cmdb/<path>` objects, so the CLI
   Reference *is* the cmdb object model. `version-matrix.md` and
   `field-inventory.json` are extracted from these.
3. **`fortinet.fortios` Ansible module docs / `fortinetdev/fortios` Terraform
   docs** — good field enumerations with types/enums, but they track "current"
   FortiOS and don't reliably timestamp per-version deltas.
4. Community posts / blogs — used only for mechanics not published elsewhere
   (rate limits, error-code tables); flagged inline where relied upon.

## Reproducing the extraction

The field inventory and version matrix are generated, not hand-typed:

```sh
# Convert the reference PDFs (references/, gitignored) to text:
for v in 7.4.12 7.6.7 8.0.1; do
  pdftotext -q "references/FortiOS-$v-CLI_Reference.pdf" "/tmp/$v.txt"
done
# Extract + diff + regenerate the committed inventory:
python3 tools/fortios-cli-extract.py \
  --version 7.4.12=/tmp/7.4.12.txt \
  --version 7.6.7=/tmp/7.6.7.txt \
  --version 8.0.1=/tmp/8.0.1.txt \
  --json docs/api/field-inventory.json --diff
```

The tool parses only field/child-table *names* (authoritative and stable to
extract). Field *types, enums, and defaults* in the object docs come from the
Ansible/Terraform cross-check and should be confirmed against `action=schema`
before being encoded into validation logic.

## Caveats

- The three PDF versions (7.4.12, 7.6.7, 8.0.1) are specific point releases;
  minor-version field drift within a train exists but is small.
- Field *values* (types/enums/defaults) in the object-reference tables are
  cross-checked from third-party docs and marked where unverified — treat
  `action=schema` as the tiebreaker.
