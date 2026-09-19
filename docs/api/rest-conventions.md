# FortiOS REST API conventions

How the FortiOS REST API works, for the `fgt` transport/auth/protocol layers.
Targets FortiOS 8.0 with 7.4/7.6 deltas called out. Primary source: Fortinet
Administration Guide "Using APIs" (doc id `940602`, same slug across 7.4/7.6/8.0)
and the `config system api-user` CLI Reference; corroborated by the
`fortinet.fortios` Ansible collection, the `fortiosapi` reference client, and
Fortinet community troubleshooting KBs (cited inline).

## 1. Base URL and paths

- Base: `https://<host>[:port]` — HTTPS only for token auth (§2).
- Every call is prefixed `/api/v2/`. Two trees:
  - **`cmdb`** — the configuration database (read + write). Path mirrors the CLI
    config tree: `cmdb/<path>/<object>[/<mkey>]`, e.g. `cmdb/firewall/address`,
    `cmdb/firewall/address/web`, `cmdb/system/interface`.
  - **`monitor`** — operational status (mostly read) plus a few action endpoints
    (backup, reboot, execute): `monitor/<category>/<sub>[/<action>]`, e.g.
    `monitor/system/interface`, `monitor/system/status`.
- `fgt`'s `api` command takes the `/api/v2/`-relative path directly:
  `fgt api GET cmdb/firewall/address`, `fgt api GET monitor/system/status`.
- **Dotted categories**: a few cmdb categories use a literal `.` where the CLI
  uses a space — notably `cmdb/vpn.ipsec/phase1-interface` (CLI: `config vpn
  ipsec phase1-interface`) and `cmdb/firewall.service/custom` (CLI: `config
  firewall service custom`). The transport must not "correct" these dots.
- The `/api/v2` root has been stable from FortiOS 6.0 through 8.0.

## 2. Authentication

### API token / Bearer (what `fgt` uses)

- Header: `Authorization: Bearer <API-TOKEN>`.
- Create via GUI **System > Administrators > Create New > REST API Admin**, or CLI:
  ```
  config system api-user
      edit "fgt"
          set accprofile "super_admin"     # scopes CRUD like any admin profile
          config trusthost
              edit 1
                  set ipv4-trusthost <cli-runner-ip>/32
              next
          end
          set vdom "root"                   # VDOM(s) this token may touch
      next
  end
  ```
  The token is shown **once** at creation and can only be regenerated, never
  re-read. Source: Fortinet CLI Reference `config system api-user`;
  community KB "About REST API"
  (community.fortinet.com/.../Technical-Tip-About-REST-API/ta-p/195425).
- **Trusted host** is enforced at the network layer *before* auth — a valid
  token from a non-trusted source IP is refused. `fgt` runs from arbitrary
  hosts/CI, so document that the runner IP must be in the api-user's trusthost.
- **HTTPS enforced**: token auth over plaintext HTTP is refused by FortiOS.
  `fgt`'s `transport.EnsureSecureURL` mirrors this (loopback exempt for tunnels).
- **FIPS-CC mode**: REST API tokens are unavailable when the FortiGate is in
  FIPS-CC mode (the REST API admin option is hidden). Surface this as a clear
  error if a token call 403s on such a box.
- `?access_token=<token>` URL-param auth also exists but is **disabled by
  default since 7.4.5 / 7.6.1** (re-enable with `set rest-api-key-url-query
  enable` under `config system global`); its status in 8.0 is unconfirmed. `fgt`
  standardizes on the Bearer header and should not add the query-param form.

### Session auth (`/logincheck`) — deferred to a later `fgt` milestone

- `POST /logincheck` (form-encoded `username=&secretkey=&ajax=1`) → sets
  `APSCOOKIE_<hash>` (session) and `ccsrftoken` (CSRF) cookies.
- Writes (POST/PUT/DELETE) then require the `X-CSRFTOKEN` header set to the CSRF
  cookie value. `GET` does not. `POST /logout` ends the session.
- **Version break**: in **FortiOS 7.6.3+**, `/logincheck` no longer returns a
  usable session cookie; the replacement is `POST /api/v2/authentication` (JSON
  creds), returning `session_key_443_<hash>` + `ccsrf_token_443_<hash>`. A
  session backend must branch on version (pre-7.6.3 vs 7.6.3+). Token/Bearer auth
  needs no CSRF and is unaffected. Source: community KB
  ".../fortios-v7-6-3-logincheck-no-longer-returns-a-session-cookie-227837".

## 3. Request / response format

### Envelope

Every response is a JSON object; the payload is under `results`:

```json
{
  "http_method": "GET",
  "results": [ /* array for a list GET, single object for a mkey GET */ ],
  "vdom": "root",
  "path": "firewall",
  "name": "address",
  "status": "success",
  "http_status": 200,
  "serial": "FGT...",
  "version": "v8.0.1",
  "build": 1234,
  "revision": "54.0",
  "mkey": "web"        // on single-object + write responses
}
```

`fgt`'s `protocol.DecodeData` unwraps `results` (falling back to the whole body).
A **single-object GET** returns `results` as an object, not a 1-element array —
a common unmarshaling gotcha; some monitor/`with_meta` combos still return arrays.

### Writes

- **Create**: `POST cmdb/<path>` with a JSON body of the object's fields
  (`Content-Type: application/json`). The mkey may be in the body.
- **Update**: `PUT cmdb/<path>/<mkey>` — merge/patch semantics (send only the
  changed fields).
- **Delete**: `DELETE cmdb/<path>/<mkey>`.
- Success → `status:"success"`, echoed `mkey`, bumped `revision`.
- **PUT-vs-POST ambiguity**: a PUT against a not-yet-existing mkey can return
  404/405/**500** depending on version; Fortinet's own client (`fortiosapi`)
  retries such a PUT as POST. Worth mirroring defensively rather than surfacing a
  raw 500.

### Status + error codes

| HTTP | Meaning |
|---|---|
| 200 | success |
| 400 | bad request (malformed body/params) |
| 401 | unauthorized (missing/invalid auth) |
| 403 | missing CSRF token, **insufficient accprofile permission**, or **VDOM-scope mismatch** in multi-VDOM |
| 404 | resource not found |
| 405 | method not allowed on that path/state |
| 424 | failed dependency — usually missing/invalid required params |
| 429 | rate-limited (§5) |
| 500 | internal error |

The body also carries a fine-grained numeric **`error`** field on failures
(~300+ codes, roughly `-1`..`-10002`; e.g. `-3` "Entry not found", `-14`
"Permission denied"), and a **`cli_error`** string surfacing the underlying CLI
validation message. `fgt`'s `protocol.DecodeError` maps HTTP status → error
`Kind` and prefers `cli_error` for the message. Source: community KB
".../troubleshooting-tip-rest-api-response-error-codes-101747".

## 4. cmdb query parameters

Appended to the URL; combine with `&`.

| Param | Purpose |
|---|---|
| `vdom=<name>` | scope to one VDOM. `global=1` targets global-scope objects; `vdom=<a>&vdom=<b>` (or the api-user's `vdom` list) for several. |
| `filter=<field><op><value>` | server-side filtering. Operators: `==`, `!=`, `=@` (contains), `!@` (not-contains), `<`, `<=`, `>`, `>=`. `&`=AND, `,`=OR. e.g. `filter=action==accept&status==enable`. Advanced filtering needs FortiOS 6.4.2+ (fine for 7.4+). |
| `format=<f1>\|<f2>` | select/limit returned fields, e.g. `format=name\|subnet`. |
| `sort=<field>[,asc\|,dsc]` | server-side sort. |
| `start=<n>` / `count=<n>` | pagination (0-based offset, page size). |
| `datasource=1` | annotate linked-object references. |
| `with_meta=1` | include per-object metadata (type id, references). |
| **`action=schema`** | **return the object's schema** (field names, types, enums, defaults, mkey) instead of data — the basis for M5 schema-driven codegen. |
| `action=default` | return a default-valued object (useful to pre-fill a create). |

### `action=schema` — the key to version-proof coverage

`GET cmdb/<path>?action=schema` returns FortiOS's own description of the object
for that exact build. Because cmdb field sets shift per release (see
[`version-matrix.md`](version-matrix.md)), this is the only maintainable way to
stay correct across 7.4/7.6/8.0 without hand-maintaining field tables. Capture it
per target version; do not cache one snapshot as universal. `fgt`'s `raw`/`api`
escape hatch already reaches it: `fgt api GET "cmdb/firewall/address?action=schema"`.

> The exact `action=schema` response shape (key names like `help`, `category`,
> `multiple_values`, `options`) was not captured from a live unit in this
> research pass — confirm against a lab box before writing the M5 parser.

## 5. Gotchas relevant to the CLI

- **Rate limiting (429)**: FortiOS enforces built-in, non-configurable per-source
  limits (community-reported ~100 GET/s, ~30 write/s; treat as order-of-magnitude).
  `fgt`'s transport already has client-side rate limiting + backoff — keep it.
- **Child tables vs scalars**: many fields are arrays of `{name: <ref>}` objects
  (e.g. policy `srcaddr`/`dstaddr`/`srcintf`/`service`), not string arrays.
  Writes must send the array-of-objects shape. Some fields are nested objects
  (e.g. interface `ipv6`, VPN sub-blocks).
- **Global vs VDOM scope**: mirrors the CLI's `config global` vs `config vdom`
  split. In multi-VDOM mode a global object without `global=1` (or a VDOM object
  with `global=1`) typically 403/404s. Drive this from `--vdom` like the CLI does.
- **CSRF is session-auth-only** — never send `X-CSRFTOKEN` on Bearer-token calls.
- **accprofile scoping** — a read-only REST admin gets 403 on writes and on some
  monitor actions (reboot/backup); distinguish this from an auth failure in error
  messages (both are HTTP 403 but differ in `error`/`cli_error`).

## Auth/version particularities (7.4 / 7.6 / 8.0)

| Area | 7.4 | 7.6 | 8.0 |
|---|---|---|---|
| `/api/v2/` cmdb+monitor prefix | stable | stable | stable |
| `?access_token=` URL param | off by default from **7.4.5** | off by default (7.6.1) | unconfirmed; use Bearer header |
| Session login | `POST /logincheck` | `POST /logincheck` (**broken 7.6.3+**) | `POST /api/v2/authentication` |
| Session cookie | `APSCOOKIE_<hash>` | `session_key_443_<hash>` (7.6.3+) | `session_key_443_<hash>` |
| `action=schema` / `action=default` | present | present | present (schema *contents* differ per build) |

**Implication for `fgt`**: M1–M4 (Bearer token) are unaffected by the session-auth
churn — it only matters when a session backend is added, which must version-branch
on 7.6.3. The `/api/v2` request shape itself is stable across all three versions.

## Sources

- Fortinet Admin Guide "Using APIs" (doc 940602), 8.0.0 / 7.6 / 7.4 — page bodies
  are JS-rendered; relied on the offline **CLI Reference PDFs** in `references/`
  plus search snippets.
- Fortinet CLI Reference `config system api-user` (8.0.1, in `references/`).
- Fortinet community KBs: About REST API (195425), REST-API error codes (101747),
  7.6.3 /logincheck change (227837), access_token URL param (188971),
  multi-VDOM 403 (344460).
- `fortiosapi` reference client (github.com/fortinet-solutions-cse/fortiosapi).
