# Switch-controller (FortiLink) & VPN IPsec objects

FortiLink-managed FortiSwitch units are administered **through** the FortiGate's
`switch-controller`, never contacted directly — so `fgt switch` wraps the
FortiGate's `cmdb/switch-controller/*` and `monitor/switch-controller/*`. VPN
IPsec uses the dotted category `cmdb/vpn.ipsec/*`. Field names + deltas from the
CLI References (7.4.12/7.6.7/8.0.1); types/enums cross-checked from Ansible.

---

## `cmdb/switch-controller/managed-switch` — mkey `switch-id`

The FortiSwitch serial is the `switch-id`. ~43–45 top-level fields and a large
set of child tables (19→24 across versions); the `ports` child table is the big
one.

Key top-level fields:

| Field | Type / enum | Notes |
|---|---|---|
| `switch-id` | string | mkey (serial) |
| `name` / `description` | string | |
| `switch-profile` | ref | |
| `fsw-wan1-admin` | `discovered`\|`disable`\|`enable` | uplink authorization |
| `fsw-wan1-peer` | ref | FortiGate FortiLink interface |
| `poe-pre-standard-detection` | `enable`\|`disable` | |
| `firmware-provision(-version)` | `enable`\|`disable` / string | auto-provision firmware |
| `port-selection-criteria` | `src-mac`\|`dst-mac`\|`src-dst-mac`\|`src-ip`\|… | trunk hash (8.0) |
| `owner-vdom` | ref | |

`ports` child table (per-port; ~90 sub-fields) — the ones a CLI user sets:

| Group | Fields |
|---|---|
| identity | `port-name` (key), `port-number`, `type` (`physical`\|`trunk`), `status`, `description` |
| VLAN | `vlan` (native/untagged), `allowed-vlans`, `allowed-vlans-all`, `untagged-vlans`, `discard-mode` |
| LAG/LACP | `mode` (`static`\|`lacp-passive`\|`lacp-active`), `lacp-speed`, `members`, `min/max-bundle` |
| PoE | `poe-status`, `poe-max-power`, `poe-port-mode` (`ieee802-3af`\|`at`\|`bt`), `poe-port-priority`, `poe-standard` |
| security | `access-mode` (`dynamic`\|`nac`\|`static`\|`normal`), `port-security-policy` (802.1X), `edge-port` |
| STP | `stp-state`, `stp-bpdu-guard`, `stp-root-guard`, `loop-guard` |
| DHCP/ARP | `dhcp-snooping` (`untrusted`\|`trusted`), `arp-inspection-trust`, `ip-source-guard` |
| IGMP | `igmp-snooping`, `igmp-snooping-flood-reports` |
| LLDP | `lldp-status` (`disable`\|`rx-only`\|`tx-only`\|`tx-rx`), `lldp-profile` (ref) |
| MAC | `learning-limit` (0=unlimited), `sticky-mac` |

**Version deltas** (authoritative): 7.4→7.6 `+max-poe-budget` and `+router-static,
router-vrf, system-dhcp-server, system-interface` child tables (L3/route-offload).
7.6→8.0 `+port-selection-criteria` and `+components` child table (stack/chassis
members). `purdue-level` and PTP fields exist in current schemas but their exact
7.4 presence is unverified — check `?action=schema`.

---

## `cmdb/switch-controller/lldp-settings` (singleton) & `lldp-profile` (mkey `name`)

`lldp-settings` (5 fields, stable across versions): `status`, `device-detection`,
`management-interface` (`internal`\|`mgmt`), `tx-interval`, `tx-hold`.

`lldp-profile` (mkey `name`, 13 fields + 3 child tables `custom-tlvs`,
`med-network-policy`, `med-location-service`): `med-tlvs`, `802.1-tlvs`,
`802.3-tlvs`, `auto-isl*`, `auto-mclag-icl`. Referenced by `ports[].lldp-profile`.

---

## VLAN modeling for FortiLink ports

There is **no single VLAN object** — three surfaces combine:

1. **`cmdb/system/interface` with `type=vlan`**, bound to the FortiLink interface
   (`interface=<fortilink>`, `vlanid=<n>`). These are the L3 VLANs referenced by
   `ports[].vlan` / `allowed-vlans`. This is the primary mechanism.
2. **`cmdb/switch-controller/vlan`** (mkey `name`) — adds captive-portal/802.1X
   auth semantics (`security`, `auth`, `usergroup`, `radius-server`) on top of the
   interface VLAN. Optional; only for authenticated switch-port VLANs.
3. **`cmdb/switch-controller/vlan-policy`** + `dynamic-port-policy` — dynamic,
   rule-matched VLAN/port assignment (e.g. LLDP device-type → VLAN).

> Practical CLI model: `fgt switch` VLAN assignment writes `ports[].vlan` /
> `allowed-vlans` referencing `system/interface` (`type=vlan`) objects — not
> `switch-controller/vlan`, which is only needed for captive-portal/802.1X.
> (`switch-controller vlan` was not resolvable in the CLI-Reference extractor —
> confirm its field set via `?action=schema`.)

---

## `cmdb/vpn.ipsec/phase1-interface` — mkey `name`

Note the dotted REST category `vpn.ipsec` (CLI: `config vpn ipsec
phase1-interface`). ~181–198 fields.

| Field | Type / enum | Notes |
|---|---|---|
| `name` | string | mkey |
| `type` | `static`\|`dynamic`\|`ddns` | remote-gw type |
| `interface` | iface ref | local egress |
| `ike-version` | `1`\|`2` | |
| `mode` | `aggressive`\|`main` | IKEv1 |
| `peertype` | `any`\|`one`\|`dialup`\|`peer`\|`peergrp` | |
| `authmethod` | `psk`\|`signature` | |
| `psksecret` | secret | PSK |
| `proposal` | list (e.g. `aes256gcm-prfsha384`) | phase1 proposals |
| `dhgrp` | list `1,2,5,14-21,27-32` | DH groups |
| `remote-gw` | ipv4 | static type |
| `nattraversal` | `enable`\|`disable`\|`forced` | |
| `net-device` | `enable`\|`disable` | route-based |
| `mode-cfg` | `enable`\|`disable` | assign VIP to dialup |
| `dpd` | `disable`\|`on-idle`\|`on-demand` | |

**Version deltas** (authoritative): 7.4→7.6 adds post-quantum
`+addke1…addke7`, `qkd-hybrid`, `remote-gw-ztna-tags`, `multipath`,
`peer-egress-shaping(-value)`, `ipv6-auto-linklocal`, `dns-suffix-search`,
`auto-transport-threshold`, `shared-idle-timeout`, `ztna-cert-scim-authorization`;
removes `fallback-tcp-threshold`. 7.6→8.0 `+fec-separate-redundant-tunnel`,
`-vni`. (`fgsp-sync` for per-tunnel FGSP failover is an 8.0 feature — verify
exact field via schema.)

---

## `cmdb/vpn.ipsec/phase2-interface` — mkey `name`

| Field | Type / enum | Notes |
|---|---|---|
| `name` | string | mkey |
| `phase1name` | ref → phase1-interface | parent tunnel |
| `proposal` | list | phase2 proposals |
| `src-subnet`/`dst-subnet` | `<ip>/<mask>` | proxy-IDs |
| `pfs` | `enable`\|`disable` | |
| `dhgrp` | list | PFS groups |
| `keylifeseconds` | 120–172800 | |
| `auto-negotiate` | `enable`\|`disable` | |
| `add-route` | `phase1`\|`enable`\|`disable` | |
| `auto-discovery-sender`/`-forwarder` | `enable`\|`disable` | ADVPN |

**Version deltas**: 7.4→7.6 `+addke1…addke7` (post-quantum), `-ipv4-df`. No change 7.6→8.0.

---

## Monitor endpoints

| Endpoint | Returns |
|---|---|
| `monitor/switch-controller/managed-switch/status` | Per managed switch: `switch-id`, `serial`, `status` (e.g. `Connected`), `state` (e.g. `Authorized`), `os_version`, `connecting_from`, `join_time`, and a nested `ports[]` array (per-port `interface`, `status`, `speed`, `duplex`, `vlan`, `poe_status`, `port_power`, FortiLink/MCLAG/ISL peer info). Port status is nested here, not a separate endpoint. |
| `monitor/vpn/ipsec` | Active tunnels: `name`, `parent`, `incoming_bytes`, `outgoing_bytes`, `rgwy` (remote gw), `tun_id`, `connection_count`, `creation_time`, dialup `username`. Use `?filter=name==<t>` on boxes with many tunnels. |

> `fgt`'s current `ListManagedSwitches` targets
> `monitor/switch-controller/managed-switch/status` — the canonical 7.2+ path.

## Sources

- FortiOS CLI Reference 7.4.12/7.6.7/8.0.1 (`references/`) via `tools/fortios-cli-extract.py`.
- Ansible `fortios_switch_controller_{managed_switch,lldp_settings,lldp_profile,vlan}`,
  `fortios_vpn_ipsec_phase1_interface`, `_phase2_interface`.
- Post-quantum `addke*`: FortiOS 7.6.0/7.6.1 new-features doc 229631.
- FGSP per-tunnel IPsec failover: FortiOS 8.0.0 admin guide doc 892338.
- Managed-switch monitor field shape: `prometheus-community/fortigate_exporter`
  (live-API observation; verify against a lab unit).
