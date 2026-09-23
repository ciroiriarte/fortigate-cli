# Coverage map

Where `fgt`'s curated commands stand against the FortiOS cmdb object model.
Object counts are top-level `config` objects in the 8.0.1 CLI Reference
(~750 total). **Everything is reachable today via the `api`/`raw` escape hatch**
regardless of status below — this map tracks *curated* coverage only.

Legend: ✅ curated · 🎫 tracked (issue #) · ⭕ gap (escape hatch only)

## By namespace

| Namespace | Objects | Status | Notes |
|---|---:|---|---|
| `firewall` | 92 | partial | address/addrgrp/service×2/policy/vip/vip-group/ippool/central-snat-map/schedule(onetime,recurring)/shaper(traffic,per-ip)/shaping-policy ✅; DoS-policy/proxy-policy ⭕ |
| `system` | 181 | partial | admin/dns/interface/vdom/ha ✅ (ha adds `status` from monitor); sdwan ✅ (`fgt sdwan` settings/zone/member/health-check/service over the child-table sub-paths); npu 🎫#24; dhcp/snmp/ntp/zone/central-mgmt/api-user/automation ⭕ |
| `router` | 29 | partial | static/policy/bgp/ospf/route-map/prefix-list/access-list ✅ (bgp/ospf singletons; child-tables via `--set`); rip/isis/multicast/bfd ⭕ |
| `user` | 26 | ⭕ | **identity & auth** — local/radius/ldap/tacacs+/group/saml/fsso/setting. None curated |
| `vpn` | 24 | partial | ipsec phase1-interface/phase2-interface ✅; **ssl-vpn** (vpn.ssl settings/portal) ⭕; certificate/l2tp/pptp ⭕ |
| `log` | 61 | ⭕ | **logging config** — fortianalyzer/syslogd/disk/memory settings, filters. Whole namespace uncurated |
| `switch-controller` | 52 | partial | managed-switch/ports/lldp 🎫#5–7; vlan/qos/security/stp/dynamic-port-policy ⭕ |
| `wireless-controller` | 43 | ⭕ | FortiAP — vap/wtp/wtp-profile. Only if integrated WiFi (Tier 3) |
| antivirus/webfilter/ips/application/dnsfilter/emailfilter | ~49 | ⭕ | **UTM security profiles** — referenced by every policy's `utm-status`. None curated |
| `dlp`, `file-filter`, `ssh-filter`, `videofilter`, `casb`, `waf` | ~30 | ⭕ | advanced content inspection (Tier 3) |
| `web-proxy`, `ftp-proxy`, `wanopt`, `icap` | ~24 | ⭕ | explicit proxy / WAN opt (Tier 3) |
| `ztna` | 8 | ⭕ | zero-trust access (Tier 3) |
| `certificate` | 5 | ⭕ | cert import/management (Tier 2) |
| `endpoint-control`, `extension-controller`, `telemetry-controller`, `report`, `authentication`, `automation` | ~22 | ⭕ | niche / infra (mixed tiers) |

## Priority tiers (for the un-curated gaps)

**Tier 1 — core, high-frequency**
1. **SD-WAN** — ✅ `system/sdwan` curated (`fgt sdwan` settings + zone/member/health-check/service, via the FortiOS child-table REST sub-paths)
2. **Identity & auth** — `user/{local,radius,ldap,tacacs+,group,saml,fsso,setting}`
3. **SSL-VPN** — `vpn.ssl/settings`, `vpn.ssl.web/portal`
4. **UTM profiles** — `antivirus/profile`, `webfilter/profile`, `ips/sensor`, `application/list`, `dnsfilter/profile`, `firewall/ssl-ssh-profile`, `firewall/profile-protocol-options`
5. **NAT & traffic** — ✅ `firewall/central-snat-map`, `firewall/shaper`+`shaping-policy` curated; still ⭕ `firewall/DoS-policy`, `firewall/proxy-policy`
6. **Logging config** — `log/{fortianalyzer,syslogd,disk,memory} setting`, `log/setting`
7. **DHCP server** — `system/dhcp server`
8. **Schedules** — ✅ `firewall.schedule/{onetime,recurring}` curated

**Tier 2 — ops/infrastructure**
- System services — `system/{snmp,ntp,central-management,fortiguard,zone}`
- Certificates — `certificate/{local,ca,remote}`, `vpn/certificate`
- Automation stitches — `system/automation-{trigger,action,stitch}`
- REST API bootstrap — `system/api-user`; per-VDOM `system/settings`, `system/global`

**Tier 3 — advanced/conditional**
- Wireless controller (FortiAP), ZTNA, WAN-opt/explicit-proxy, DLP, endpoint/extender, CASB, videofilter.

## Design note

Curated coverage is intentionally the *common* objects, not all ~750. The
`api`/`raw` escape hatch guarantees 100% reach on every FortiOS version, and M5
(schema-driven codegen) is the plan to promote the long tail into typed commands
cheaply. This map is the checklist for that promotion.

_See the pinned Roadmap issue and per-domain milestones for the tracked work._
