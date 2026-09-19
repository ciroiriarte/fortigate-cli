# AGENTS.md — fortigate-cli

Guidance for AI agents (and humans) working in this repo.

## What this is

`fgt`: an **unofficial**, remote-first CLI for FortiGate/FortiOS over the REST
API, modeled on [`pve-cli`](https://github.com/ciroiriarte/pve-cli). Go +
spf13/cobra, single static binary. See `docs/DESIGN.md` for architecture/roadmap.

**FortiOS API reference**: `docs/api/` documents the REST conventions and the
per-version (7.4/7.6/8.0) object model this CLI wraps — read it before adding
commands. When in doubt about a field/enum/default for a specific build, run
`fgt api GET "cmdb/<path>?action=schema"` against that box: FortiOS self-describes
its schema per build, which is more authoritative than any static doc.

## Layout

- `cmd/fgt/` — entrypoint only.
- `internal/cli/` — cobra tree (curated UX) + `api` escape hatch. Commands are
  `resource action` (e.g. `firewall address list`).
- `internal/provider/` — backend interface; `fortigate/` implements FortiOS.
  New backends (e.g. FortiManager) register via `init()`; never import HTTP from
  the cli layer.
- `internal/transport/` — the only place that speaks HTTP. Owns auth/TLS/retry/
  rate-limit/VDOM. Paths are relative to `/api/v2/`.
- `internal/{auth,config,protocol,output,domain,version}/` — supporting packages.

## Conventions

- **No secrets in flags by default** — resolve from keyring (`secret_ref:
  keyring://…`) or env (`FGT_CLI_TOKEN`). Never log the token; `--debug` prints
  metadata only.
- **HTTPS enforced** — `transport.EnsureSecureURL` refuses plaintext to
  non-loopback hosts. Don't weaken it.
- **cmdb = config (writes), monitor = status (reads).** Keep the split visible
  in command help.
- **json/yaml are the stable scripting contract**; table layout is not
  guaranteed. Every list command sets `Tabular.Raw` for structured output.
- **Mutating `api` calls confirm** unless `-y/--yes`.
- Env var prefix is `FGT_CLI_`.

## Build & gates

```sh
go mod tidy      # first time / after dep changes
make check       # fmtcheck + vet + test + build  — run before committing
```

The binary is `fgt`. Version metadata is injected via `-ldflags` (see Makefile).

## Adding a curated command

1. Add domain type(s) in `internal/domain/` if needed.
2. Add a provider method to the `Provider` interface + `fortigate` impl (use
   `cl.Do` with a `cmdb/...` or `monitor/...` path).
3. Add the cobra command under the right resource in `internal/cli/`, render via
   `a.render(output.Tabular{...})`, and set `Raw` for json/yaml.
4. Add a test. Keep authoring and review as separate passes.
