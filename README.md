# fortigate-cli (`fgt`)

> **Unofficial** remote-first CLI for **FortiGate / FortiOS** — and the
> FortiLink-managed **FortiSwitch** units behind it — driven entirely over the
> FortiOS REST API. Nothing is installed on the device.
>
> Not affiliated with or endorsed by Fortinet, Inc. FortiGate, FortiOS,
> FortiSwitch and FortiLink are trademarks of Fortinet, Inc.

`fgt` is an [OpenStack-Client][osc]-inspired `resource action` CLI in the spirit
of [`pve-cli`][pvecli]: a single static Go binary that talks to a FortiGate's
`/api/v2/` REST surface — `cmdb/...` for configuration, `monitor/...` for status.

## Why

Every existing open-source option is either a **read-only/stale CLI** or an
**importable Python library** (fortiosapi — now archived, fortigate-api, pyFGT,
…). Fortinet's own automation ships as an Ansible collection and a Terraform
provider, but there is **no standalone CLI binary**. `fgt` fills that gap.

## Design in one breath

- **Remote-first**: pure REST client, no agent on the box.
- **Hybrid coverage**: curated, ergonomic commands for common objects **plus**
  an `fgt api` / raw escape hatch so **every** `cmdb`/`monitor` endpoint is
  reachable without waiting for a hand-written wrapper.
- **cmdb vs monitor** split is explicit; `--vdom` scopes any call.
- **Auth**: FortiOS REST **API token** (`Authorization: Bearer`) **or session
  login** (`/logincheck` + `X-CSRFTOKEN`, for password admins without a token),
  HTTPS enforced. Secrets come from the OS keyring or an env var, never a flag by
  default.
- **Output**: `table` (default; wide columns truncate, `--wide` to disable),
  `json`, `yaml`, `csv`, `value`.
- **Schema-aware**: `fgt schema <path>` shows any object's per-build field schema
  and can scaffold new curated commands (`--gen`).

## Install / build

Requires Go 1.22+.

```sh
git clone https://github.com/ciroiriarte/fortigate-cli
cd fortigate-cli
go mod tidy      # first build resolves the dependency graph
make build       # produces ./fgt
make check       # fmtcheck + vet + test + build (run before committing)
```

### Man pages & shell completions

Pre-generated man pages live in [`docs/man/`](docs/man/) and completion scripts
in [`contrib/completions/`](contrib/completions/) (regenerate with `make docs`):

```sh
# man pages
sudo cp docs/man/*.1 /usr/local/share/man/man1/ && man fgt

# bash completion (persistent)
sudo cp contrib/completions/fgt.bash /etc/bash_completion.d/fgt
# …or per-shell, no install:
source <(fgt completion bash)      # also: zsh | fish | powershell
```

## Configure

Create a REST-API admin + token on the FortiGate, then point `fgt` at it. The
fastest path uses environment variables:

```sh
export FGT_CLI_SERVER="https://fw.example.com"
export FGT_CLI_TOKEN="<rest-api-token>"
# self-signed mgmt cert? pin it instead of --insecure:
export FGT_CLI_TLS_FINGERPRINT="aa:bb:cc:..."
```

Or a config file at `~/.config/fortigate-cli/config.yaml` (kubeconfig-style
profiles + contexts):

```yaml
current_context: lab
contexts:
  lab:
    profile: lab-fw
    vdom: root
profiles:
  lab-fw:
    server: https://fw.example.com
    auth:
      type: token
      secret_ref: keyring://fortigate-cli/lab-fw   # or env:FGT_CLI_TOKEN
    tls:
      fingerprint: "aa:bb:cc:..."
```

No REST-API token? Use **session auth** with a normal admin account
(`/logincheck` under the hood) — set `FGT_CLI_USER` + `FGT_CLI_PASSWORD`, or:

```yaml
  lab-session:
    server: https://fw.example.com
    auth:
      type: session
      user: admin
      secret_ref: env:FGT_CLI_PASSWORD   # never store the password in the file
    tls:
      fingerprint: "aa:bb:cc:..."
```

## Use

```sh
# curated CRUD (list / show / create / set / delete) over cmdb objects:
fgt firewall address list -o json
fgt firewall address create web --type ipmask --subnet "10.0.0.0 255.255.255.0"
fgt firewall address set web --comment "managed by fgt"
fgt firewall addrgrp create webservers --member web,db
fgt firewall policy create --name allow-web --srcintf port1 --dstintf port2 \
    --srcaddr all --dstaddr web --service HTTPS --action accept
fgt firewall policy list
fgt router static create --dst "0.0.0.0 0.0.0.0" --gateway 10.0.0.1 --device port1
fgt system dns set --primary 1.1.1.1 --secondary 8.8.8.8
fgt firewall address delete web        # confirms first (pass -y to skip)

# any field not modeled as a typed flag is still reachable:
fgt firewall policy set 3 --set "comments=updated" --set "nat=enable"

# more curated surfaces (all list/show/create/set/delete):
fgt firewall vip / ippool / central-snat-map / shaper / schedule / shaping-policy
fgt router bgp show ; fgt router static list ; fgt router route-map list
fgt sdwan member list ; fgt sdwan health-check list ; fgt sdwan service list
fgt vpn ipsec phase1-interface list ; fgt vpn ssl portal list
fgt user ldap list ; fgt user group list ; fgt user local list
fgt log syslogd setting show ; fgt system dhcp server list ; fgt system snmp community list

# read-only status (monitor surface):
fgt system interface list          # live interface status
fgt system ha status               # HA cluster members
fgt vpn ssl sessions               # active SSL-VPN sessions
fgt switch list                    # FortiLink-managed FortiSwitch units
fgt config current                 # resolved settings (secret redacted)

# device config backup / restore:
fgt system backup -O amsa.conf     # download running config (privileged)
fgt system restore amsa.conf       # replace config (confirms; usually reboots)

# schema-driven: inspect any object, or scaffold a new curated command:
fgt schema firewall/vip                    # field table for the target build
fgt schema system.snmp/community --gen     # emit a resource{} skeleton

# escape hatch — reach ANY endpoint on ANY FortiOS version:
fgt api GET  cmdb/firewall/policy
fgt api GET  cmdb/firewall/address -d action=schema
fgt api DELETE cmdb/firewall/address/web
```

Global flags: `--server`, `--vdom`, `--token`, `--user`/`--password` (session
auth), `-o/--format`, `--wide`, `-c/--column`, `--sort`, `--no-headers`,
`--insecure`, `--tls-fingerprint`, `--debug`, `-y/--yes`.

## Status

- **M1** ✅ — transport, auth, config/keyring, output, the `api` escape hatch.
- **M2** ✅ — curated CRUD via a declarative resource framework (adding an object
  is a ~15-line declaration). Shipped surfaces:
  - **firewall** — address/addrgrp/service/policy, vip/vip-group/ippool,
    central-snat-map, schedule, shaper/per-ip-shaper, shaping-policy
  - **router** — static/policy, bgp/ospf, route-map/prefix-list/access-list
  - **system** — admin/dns/interface/vdom/ha (+`ha status`), ntp, dhcp server,
    snmp (sysinfo/community/user), **sdwan** (zone/member/health-check/service)
  - **vpn** — ipsec phase1/2, **ssl-vpn** (settings/auth-rule/portal + `sessions`)
  - **user** — local/group/ldap/radius/tacacs+ · **log** — setting/syslogd/faz
  - config **backup**/**restore**
- **M4** ✅ — session auth (`/logincheck`), so password admins work token-free.
- **M5** ✅ — `fgt schema` (schema introspection + `resource{}` scaffolding).

Everything else is reachable today via the `api`/`raw` escape hatch. See
[`docs/api/coverage-map.md`](docs/api/coverage-map.md) for curated-vs-gap status.

See [`docs/DESIGN.md`](docs/DESIGN.md) for the roadmap and
[`docs/api/`](docs/api/) for the FortiOS REST + per-version object-model reference
that the commands are built from.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

[osc]: https://docs.openstack.org/python-openstackclient/latest/
[pvecli]: https://github.com/ciroiriarte/pve-cli
