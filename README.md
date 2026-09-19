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
- **Auth**: FortiOS REST **API token** (`Authorization: Bearer`), HTTPS enforced.
  Secrets come from the OS keyring or an env var, never a flag by default.
  Session (`/logincheck`) auth is a later phase.
- **Output**: `table` (default), `json`, `yaml`, `csv`, `value`.

## Install / build

Requires Go 1.22+.

```sh
git clone https://github.com/ciroiriarte/fortigate-cli
cd fortigate-cli
go mod tidy      # first build resolves the dependency graph
make build       # produces ./fgt
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

# read-only status (monitor surface):
fgt system interface list          # live interface status
fgt switch list                    # FortiLink-managed FortiSwitch units
fgt config current                 # resolved settings (secret redacted)

# escape hatch — reach ANY endpoint on ANY FortiOS version:
fgt api GET  cmdb/firewall/policy
fgt api GET  "cmdb/firewall/address?action=schema"
fgt api DELETE cmdb/firewall/address/web
```

Global flags: `--server`, `--vdom`, `--token`, `-o/--format`, `-c/--column`,
`--sort`, `--no-headers`, `--insecure`, `--tls-fingerprint`, `--debug`, `-y/--yes`.

## Status

- **M1** ✅ — transport, auth, config/keyring, output, the `api` escape hatch.
- **M2** 🚧 — curated CRUD (list/show/create/set/delete) via a declarative
  resource framework: `firewall address`/`addrgrp`/`service custom`/`service
  group`/`policy`, `router static`, `system admin`/`dns`, plus monitor reads
  (`system interface`, `switch`). Adding an object is a ~15-line declaration.

See [`docs/DESIGN.md`](docs/DESIGN.md) for the roadmap and
[`docs/api/`](docs/api/) for the FortiOS REST + per-version object-model reference
that the commands are built from.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

[osc]: https://docs.openstack.org/python-openstackclient/latest/
[pvecli]: https://github.com/ciroiriarte/pve-cli
