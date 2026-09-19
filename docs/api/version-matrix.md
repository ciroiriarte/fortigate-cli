# Version matrix (7.4.12 / 7.6.7 / 8.0.1)

Authoritative per-version field counts and add/remove deltas for the cmdb objects
`fgt` targets, extracted directly from the FortiOS CLI Reference PDFs via
`tools/fortios-cli-extract.py`. This is the primary-source answer to the
`docs/DESIGN.md` open question "is the cmdb schema stable across versions?" —
**the schema is stable in shape but the field set drifts every release**, so
version-specific validation must come from `?action=schema` on the target build,
not a hard-coded table.

Counts are top-level `set` fields (child-table sub-fields excluded).

| Object | 7.4.12 | 7.6.7 | 8.0.1 | child-tables (8.0.1) |
|---|---:|---:|---:|---:|
| `firewall/address` | 40 | 44 | 50 | 3 |
| `firewall/addrgrp` | 10 | 10 | 13 | 1 |
| `firewall/service/custom` | 26 | 27 | 28 | 0 |
| `firewall/service/group` | 6 | 6 | 7 | 0 |
| `firewall/policy` | 178 | 185 | 192 | 1 |
| `system/interface` | 278 | 285 | 295 | 12 |
| `system/admin` | 50 | 50 | 62 | 0 |
| `system/dns` | 23 | 28 | 30 | 0 |
| `system/global` | 259 | 285 | 289 | 0 |
| `router/static` | 20 | 21 | 21 | 0 |
| `router/policy` | 22 | 25 | 25 | 0 |
| `switch-controller/managed-switch` | 43 | 44 | 45 | 24 |
| `switch-controller/lldp-settings` | 5 | 5 | 5 | 0 |
| `switch-controller/lldp-profile` | 13 | 13 | 13 | 3 |
| `vpn.ipsec/phase1-interface` | 181 | 198 | 198 | 2 |
| `vpn.ipsec/phase2-interface` | 45 | 51 | 51 | 0 |
| `system/api-user` | 8 | 8 | 8 | 0 |

## Notable deltas

### firewall/address
- 7.4→7.6: **+** `agent-id, obsolete, passive-fqdn-learning, sso-attribute-value`
- 7.6→8.0: **+** `custom-tags, display-with, fabric-force-sync, fabric-object-source, hw-version, ipam-allocate-unique, managed-subnetwork-size`; **rename** `hw-model → hw-version`; **+child** `addr-8021x`

### firewall/policy
- 7.4→7.6: **+** `app-monitor, internet-service-fortiguard (+src/6/6-src), log-http-transaction, port-random, radius-ip-auth-bypass, telemetry-profile, ztna-ems-tag-negate`; **−** `cifs-profile, gtp-profile, pfcp-profile`
- 7.6→8.0: **+** `custom-tags, fabric-force-sync, fabric-object, skip-vrf-match, ztna-destination, ztna-ems-tag6 (+negate)`; **+child** `fabric-policy`

### system/interface
- 7.4→7.6: **+** `dhcp-relay-vrf-select, exclude-signatures, mrru, multilink, netflow-sample-rate, netflow-sampler-id, security-ip-auth-bypass, telemetry-discover, virtual-mac`; **−** `drop-overlapped-fragment, gi-gk`
- 7.6→8.0: **+** `arp-egress-cos, captive-portal-api-server, dhcp-egress-cos, inbandwidth-source, ipam-conflicts, outbandwidth-source, remote-management-ip, stp-root-guard, switch-controller-fortilink-settings, tx-queue-len`

### system/admin
- 7.6→8.0: **+** `disallowed-login-methods, gui-custom-table-menu-style, gui-custom-theme, gui-llm-provider, gui-table-menu-style-type, gui-theme, gui-theme-type, openai-api-key, openai-api-key-part2, openai-model, openai-org-id, openai-project-id` (the 8.0 FortiAI/LLM-assist surface)

### system/dns
- 7.4→7.6: **+** `hostname-limit, hostname-ttl, root-servers, source-ip-interface, vrf-select`
- 7.6→8.0: **+** `cert-verify, ocsp-stapling-request`

### router/static
- 7.4→7.6: **+** `internet-service-fortiguard`

### router/policy
- 7.4→7.6: **+** `groups, internet-service-fortiguard, users`

### switch-controller/managed-switch
- 7.4→7.6: **+** `max-poe-budget`; **+child** `router-static, router-vrf, system-dhcp-server, system-interface`
- 7.6→8.0: **+** `port-selection-criteria`; **+child** `components`

### vpn.ipsec/phase1-interface
- 7.4→7.6: **+** `addke1…addke7, auto-discovery-dialup-placeholder, auto-transport-threshold, dns-suffix-search, ipv6-auto-linklocal, multipath, peer-egress-shaping (+value), qkd-hybrid, remote-gw-ztna-tags, shared-idle-timeout, ztna-cert-scim-authorization`; **−** `fallback-tcp-threshold`
- 7.6→8.0: **+** `fec-separate-redundant-tunnel`; **−** `vni`

### vpn.ipsec/phase2-interface
- 7.4→7.6: **+** `addke1…addke7`; **−** `ipv4-df`

## Reading this for implementation

- **Curated commands (M2)** should target the intersection of fields present
  across 7.4/7.6/8.0 for a stable core UX, and gate version-specific fields
  behind a detected version (from `monitor/system/status`) or simply expose them
  through the `api` escape hatch.
- **Post-quantum IPsec (`addke*`), FortiAI admin fields, and ZTNA policy fields**
  are the clearest "don't assume on 7.4" cases.
- The `api`/`raw` escape hatch already covers every field on every version with no
  code change — curated commands are ergonomics on top, not a coverage gate.

_Regenerate this file's data with `tools/fortios-cli-extract.py --diff` (see
[`README.md`](README.md))._
