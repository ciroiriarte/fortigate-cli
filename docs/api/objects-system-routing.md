# System & routing objects (cmdb + monitor)

Field reference for the system/routing objects `fgt` wraps, plus the read-only
`monitor/*` endpoints the curated read commands use. Field names + deltas from
the CLI References (7.4.12/7.6.7/8.0.1); types/enums cross-checked from Ansible /
Terraform — confirm against `?action=schema`.

---

## `cmdb/system/interface` — mkey `name`

The largest common object (~278–295 fields). Child tables include `secondaryip`,
`vrrp`, `member` (aggregate/redundant), `tagging`, `ipv6` (nested block), and
DHCP sub-tables.

| Field | Type / enum | Notes |
|---|---|---|
| `name` | string | mkey |
| `vdom` | ref | owning VDOM |
| `type` | `physical`\|`vlan`\|`aggregate`\|`redundant`\|`loopback`\|`tunnel`\|`switch`\|`hard-switch`\|`emac-vlan`\|`vxlan`\|… | |
| `mode` | `static`\|`dhcp`\|`pppoe` | IPv4 addressing |
| `ip` | `"<ip> <mask>"` / `<ip>/<pfx>` | |
| `allowaccess` | space list: `ping https ssh snmp http telnet fgfm radius-acct probe-response fabric ftm speed-test scim capwap` | mgmt access |
| `role` | `lan`\|`wan`\|`dmz`\|`undefined` | |
| `status` | `up`\|`down` | admin status |
| `alias` / `description` | string | |
| `vlanid` | 1–4094 | for `type=vlan` |
| `interface` | ref | parent (VLAN/aggregate) |
| `member` | child `[{interface-name}]` | aggregate/redundant members |
| `speed` | `auto`\|`1000full`\|… | |
| `mtu` / `mtu-override` | int / `enable`\|`disable` | |
| `secondary-IP` + `secondaryip[]` | toggle + child table | extra IPs |
| `dhcp-relay-service` / `dhcp-relay-ip` | `enable`\|`disable` / list | |
| `vrrp[]` | child table (`vrid, vrip, priority, …`) | |
| `ipv6` | nested block (`ip6-address, ip6-mode, ip6-allowaccess, vrrp6[], …`) | |

**Version deltas**: 7.4→7.6 `+dhcp-relay-vrf-select, exclude-signatures, mrru,
multilink, netflow-*, security-ip-auth-bypass, telemetry-discover, virtual-mac`;
`-drop-overlapped-fragment, gi-gk`. 7.6→8.0 `+arp-egress-cos, captive-portal-api-server,
dhcp-egress-cos, in/outbandwidth-source, ipam-conflicts, remote-management-ip,
stp-root-guard, switch-controller-fortilink-settings, tx-queue-len`.
8.0 also adds a manual `ip6-link-local` under `ipv6` (setting it removes the
auto-generated link-local).

---

## `cmdb/system/admin` — mkey `name`

Administrator accounts. Child table: `vdom` (`[{name}]`), plus GUI dashboard lists.

| Field | Type / enum | Notes |
|---|---|---|
| `name` | string | mkey |
| `password` | secret | required on create |
| `accprofile` | ref `system.accprofile` | permission scope |
| `vdom` | child `[{name}]` | accessible VDOMs |
| `trusthost1`…`trusthost10` | `"<ip> <mask>"` | source restriction (default `0.0.0.0 0.0.0.0`) |
| `ip6-trusthost1`…`10` | ipv6+pfx | |
| `two-factor` | `disable`\|`fortitoken`\|`fortitoken-cloud`\|`email`\|`sms` | |
| `fortitoken` / `email-to` / `sms-*` | | 2FA details |
| `ssh-public-key1..3` | string | key-based SSH login |
| `force-password-change` | `enable`\|`disable` | |
| `remote-auth` / `remote-group` | `enable`\|`disable` / ref | RADIUS/LDAP/TACACS+ |

**Version deltas**: no change 7.4→7.6. 7.6→8.0 `+disallowed-login-methods,
gui-custom-table-menu-style, gui-custom-theme, gui-llm-provider, gui-theme(+type),
openai-api-key(+part2), openai-model, openai-org-id, openai-project-id` — i.e. the
8.0 FortiAI/LLM-assist fields.

---

## `cmdb/system/dns` — singleton (GET/PUT, no mkey)

Child tables: `domain` (search suffixes), `server-hostname`.

| Field | Type / enum | Notes |
|---|---|---|
| `primary`/`secondary` | ipv4 | |
| `ip6-primary`/`ip6-secondary` | ipv6 | |
| `domain[]` | child `[{domain}]` | search suffixes |
| `protocol` | list `cleartext`\|`dot`\|`doh` | transport |
| `dns-over-tls` | `disable`\|`enable`\|`enforce` | |
| `server-select-method` | `least-rtt`\|`failover` | |
| `interface-select-method` | `auto`\|`sdwan`\|`specify` | |
| `source-ip` / `interface` | | egress |

**Version deltas**: 7.4→7.6 `+hostname-limit, hostname-ttl, root-servers,
source-ip-interface, vrf-select`; 7.6→8.0 `+cert-verify, ocsp-stapling-request`.

---

## `cmdb/router/static` — mkey `seq-num` (auto-assigned if omitted)

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `seq-num` | int | — | mkey |
| `status` | `enable`\|`disable` | `enable` | |
| `dst` | `<ip> <mask>` | `0.0.0.0/0` | destination |
| `gateway` | ipv4 | — | next hop |
| `device` | iface ref | — | egress interface |
| `distance` | 1–255 | 10 | admin distance |
| `priority` | 0–4294967295 | 0 | |
| `weight` | 0–255 | 0 | ECMP tie-break |
| `blackhole` | `enable`\|`disable` | `disable` | route to null0 |
| `dynamic-gateway` | `enable`\|`disable` | `disable` | gw from DHCP/PPP |
| `sdwan-zone[]` | child `[{name}]` | — | SD-WAN egress (since 7.0.1; prefer over legacy `sdwan`) |
| `dstaddr` | addr ref | — | address instead of `dst` |
| `vrf` | 0–251 | 0 | |
| `bfd` | `enable`\|`disable` | `disable` | |
| `comment` | string | — | |

**Version deltas**: 7.4→7.6 `+internet-service-fortiguard`. No change 7.6→8.0.

---

## `cmdb/router/policy` — mkey `seq-num`

Policy-based routing. Most match fields are child tables (`input-device`, `src`,
`dst`, `srcaddr`, `dstaddr`, `internet-service-*`).

| Field | Type / enum | Notes |
|---|---|---|
| `seq-num` | int | mkey |
| `status` | `enable`\|`disable` | |
| `action` | `deny`\|`permit` | (default `permit`) |
| `input-device[]` | child `[{name}]` | ingress match |
| `src[]`/`dst[]` | child `[{subnet}]` | IP match |
| `srcaddr[]`/`dstaddr[]` | child `[{name}]` | address match |
| `gateway` | ipv4 | next hop |
| `output-device` | iface ref | egress |
| `protocol` | 0–255 | |
| `start-port`/`end-port` | 0–65535 | |
| `comments` | string | |

**Version deltas**: 7.4→7.6 `+groups, internet-service-fortiguard, users` (these
were flagged uncertain by web docs — CLI Reference confirms they land in 7.6 and
persist in 8.0). No change 7.6→8.0.

---

## Monitor endpoints (read surface)

Same envelope as cmdb (payload under `results`).

| Endpoint | Returns |
|---|---|
| `monitor/system/interface` | Live status **keyed by interface name** (object, not array): link state, IPv4/IPv6, speed/duplex, VDOM, counters. Params: `include_vlan`, `vdom`. |
| `monitor/system/status` | `serial`, `version` (e.g. `v8.0.1`), `build`, `hostname`, model, HA/VDOM state — the "what am I talking to" probe. |
| `monitor/system/resource/usage` | CPU/mem/disk/session utilization; params `resource=`, `scope=`, `interval=`. |
| `monitor/router/ipv4` | Active IPv4 RIB (dst, gateway, interface, type, distance/metric); paginated + filterable. `monitor/router/ipv6` for v6. |
| `monitor/system/ha-statistics` | Per HA-cluster-member stats (serial, hostname, and per-build utilization/session counters). Backs `fgt system ha status`. **Field set is per-build and not statically modeled** — the command renders whatever the box returns; confirm exact keys against a live HA pair. Standalone units return a single member. |

> `monitor/system/interface` returning an object keyed by name is why `fgt`'s
> provider unmarshals it into a `map[string]…` and flattens to a slice.

## Sources

- FortiOS CLI Reference 7.4.12/7.6.7/8.0.1 (`references/`) via `tools/fortios-cli-extract.py`.
- Ansible `fortios_system_{interface,admin,dns}`, `fortios_router_{static,policy}`.
- Terraform `fortinetdev/fortios` (raw GitHub markdown).
- `sdwan-zone` since 7.0.1: FortiOS 7.4 admin guide doc 270527.
- `ip6-link-local` / FortiAI fields: FortiOS 8.0.0 new-features.
