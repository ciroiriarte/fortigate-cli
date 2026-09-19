# Firewall objects (cmdb)

Field reference for the firewall cmdb objects `fgt` wraps first (M2). Field
*names* and version deltas are extracted from the FortiOS CLI References
(7.4.12 / 7.6.7 / 8.0.1); field *types/enums/defaults* are cross-checked from the
`fortinet.fortios` Ansible modules and `fortinetdev/fortios` Terraform provider —
confirm those against `?action=schema` before encoding validation. See
[`version-matrix.md`](version-matrix.md) for the raw per-version diffs and
[`field-inventory.json`](field-inventory.json) for the full name lists.

> Child tables are arrays of objects on the wire, usually `[{"name": "<ref>"}]`.

---

## `firewall/address` — mkey `name`

Configure IPv4 addresses. Child tables: `list` (IP list), `tagging`, and (8.0+)
`addr-8021x`.

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `name` | string (≤79) | — | mkey |
| `type` | `ipmask`\|`iprange`\|`fqdn`\|`geography`\|`wildcard`\|`dynamic`\|`interface-subnet`\|`mac`\|`wildcard-fqdn`\|`route-tag` | `ipmask` | selects which value field applies |
| `subnet` | `"<ip> <mask>"` or CIDR | `0.0.0.0 0.0.0.0` | when `type=ipmask` |
| `start-ip`/`end-ip` | ipv4 | — | when `type=iprange` |
| `fqdn` | string (≤255) | — | when `type=fqdn` |
| `country` | ISO code | — | when `type=geography` |
| `wildcard` | `"<ip> <wildcard>"` | — | when `type=wildcard` |
| `interface` | iface ref | — | when `type=interface-subnet` |
| `sub-type` | `sdn`\|`clearpass-spt`\|`fsso`\|`ems-tag`\|… | `sdn` | for `type=dynamic` |
| `associated-interface` | iface ref | — | display/policy hint |
| `allow-routing` | `enable`\|`disable` | `disable` | usable as static-route dst |
| `fabric-object` | `enable`\|`disable` | `disable` | Security Fabric global object |
| `comment` | var-string (≤255) | — | |
| `uuid` | uuid | auto | immutable |

**Version deltas** (authoritative, from CLI Reference):
- 7.4→7.6: `+agent-id, obsolete, passive-fqdn-learning, sso-attribute-value`.
- 7.6→8.0: `+custom-tags, display-with, fabric-force-sync, fabric-object-source,
  hw-version, ipam-allocate-unique, managed-subnetwork-size`; **`hw-model` →
  renamed `hw-version`**; `+addr-8021x` child table.

---

## `firewall/addrgrp` — mkey `name`

Address group. Child table: `member` (**required**, ≥1), plus `tagging`.

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `name` | string (≤79) | — | mkey |
| `type` | `default`\|`folder` | `default` | |
| `category` | `default`\|`ztna-ems-tag`\|`ztna-geo-tag` | `default` | what members may be |
| `member` | child `[{name}]` → address/addrgrp | — | **required** |
| `exclude` | `enable`\|`disable` | `disable` | |
| `exclude-member` | child `[{name}]` | — | when `exclude=enable` |
| `allow-routing` | `enable`\|`disable` | `disable` | |
| `fabric-object` | `enable`\|`disable` | `disable` | |
| `comment` | var-string | — | |

**Version deltas**: 7.6→8.0 `+custom-tags, display-with, fabric-force-sync`.
No change 7.4→7.6.

---

## `firewall/service/custom` — mkey `name` (REST path `cmdb/firewall.service/custom`)

Custom service. No child tables.

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `name` | string (≤79) | — | mkey |
| `protocol` | `TCP/UDP/SCTP`\|`ICMP`\|`ICMP6`\|`IP`\|`HTTP`\|`FTP`\|… | `TCP/UDP/SCTP` | |
| `protocol-number` | 0–254 | 0 | when `protocol=IP` |
| `tcp-portrange` | `"[src:]dst[-dst] …"` | — | e.g. `443` or `1024-65535:80` |
| `udp-portrange` | same syntax | — | |
| `sctp-portrange` | same syntax | — | |
| `icmptype`/`icmpcode` | 0–255 | any | ICMP/ICMP6 |
| `iprange` | `<start>-<end>` | — | restrict dst IP |
| `fqdn` | string | — | restrict dst FQDN |
| `session-ttl` | 0 or 300–2764800 | 0 | override |
| `helper` | `auto`\|`disable`\|`ftp`\|`sip`\|… | `auto` | ALG binding |
| `category` | ref | — | GUI grouping |
| `comment` | var-string | — | |

**Version deltas**: 7.4→7.6 `+udplite-portrange`; 7.6→8.0 `+fabric-force-sync`.

---

## `firewall/service/group` — mkey `name` (REST path `cmdb/firewall.service/group`)

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `name` | string (≤79) | — | mkey |
| `member` | child `[{name}]` → service/custom or group | — | **required** ≥1 |
| `proxy` | `enable`\|`disable` | `disable` | explicit-proxy group |
| `comment` | var-string | — | |

**Version deltas**: 7.6→8.0 `+fabric-force-sync`.

---

## `firewall/policy` — mkey `policyid` (integer; 0/omitted = auto-assign)

The central object. All address/interface/service selectors are **child tables**
of `[{name}]`. ~178–192 fields total; the ones a CLI user sets:

| Field | Type / enum | Default | Notes |
|---|---|---|---|
| `policyid` | int | auto | mkey |
| `name` | string (≤35) | — | display name |
| `srcintf`/`dstintf` | child `[{name}]` (iface/zone) | — | **required** |
| `srcaddr`/`dstaddr` | child `[{name}]` (addr/grp/vip); `all` valid | — | **required** |
| `srcaddr6`/`dstaddr6` | child `[{name}]` | — | IPv6 |
| `service` | child `[{name}]`; `ALL` valid | — | **required** |
| `action` | `accept`\|`deny`\|`ipsec` | `deny` | |
| `status` | `enable`\|`disable` | `enable` | |
| `schedule` | ref | `always` | |
| `nat` | `enable`\|`disable` | `disable` | policy-based SNAT (needs `central-nat=disable`) |
| `ippool` | `enable`\|`disable` | `disable` | |
| `poolname` | child `[{name}]` → ippool | — | when `ippool=enable` |
| `inspection-mode` | `proxy`\|`flow` | `flow` | inert on ≤2 GB-RAM models from 7.4.4 |
| `utm-status` | `enable`\|`disable` | `disable` | master UTM switch |
| `av-profile`/`webfilter-profile`/`dnsfilter-profile`/`ips-sensor`/`application-list` | ref | — | require `utm-status=enable` |
| `ssl-ssh-profile` | ref | — | |
| `logtraffic` | `all`\|`utm`\|`disable` | `utm` | |
| `vpntunnel` | ref phase1 | — | when `action=ipsec` |
| `comments` | var-string (≤1023) | — | |
| `uuid` | uuid | auto | |

**Version deltas** (authoritative):
- 7.4→7.6: `+app-monitor, internet-service-fortiguard (+src/6 variants),
  log-http-transaction, port-random, radius-ip-auth-bypass, telemetry-profile,
  ztna-ems-tag-negate`; **removed** `cifs-profile, gtp-profile, pfcp-profile`.
- 7.6→8.0: `+custom-tags, fabric-force-sync, fabric-object, skip-vrf-match,
  ztna-destination, ztna-ems-tag6(+negate)`; `+fabric-policy` child table.

> NGFW-mode note: `central-nat` (`system settings.central-nat`) decides whether
> policy-level `nat`/`ippool`/`poolname` are honored vs SNAT driven by
> `firewall/central-snat-map`. Surface this in policy-NAT UX.

## Sources

- FortiOS CLI Reference 7.4.12 / 7.6.7 / 8.0.1 (`references/`, gitignored) — field
  names + version deltas, via `tools/fortios-cli-extract.py`.
- Ansible `fortinet.fortios` modules `fortios_firewall_{address,addrgrp,service_custom,service_group,policy}` — types/enums.
- Terraform `fortinetdev/fortios` resource docs (raw GitHub markdown).
- FortiOS 8.0.0 release notes "Changes in CLI" (confirms `hw-model`→`hw-version`).
- 2 GB-RAM proxy restriction: FortiOS 7.4.4 release notes doc 768039.
