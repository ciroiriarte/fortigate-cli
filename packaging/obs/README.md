# OBS packaging — the distro-native channel

This directory is the source of truth for the [openSUSE Build Service][obs]
package that builds native `.rpm`/`.deb` for openSUSE, Fedora, Debian and
Ubuntu, and publishes signed `zypper`/`dnf`/`apt` repositories. It is the
**blessed** install route for Linux (see the repo README's Install section).

GitHub Releases (via goreleaser) are the *other* channel: signed universal
binaries plus convenience `.deb`/`.rpm` you download by hand. OBS is what gives
users a real repo with automatic updates and distro-key signing.

## Why vendored, why pinned

OBS build chroots have **no network**. So we:

- pin `_service` to the **immutable commit SHA** of a release tag (never a
  moving branch or "latest tag"), and
- **vendor** the Go modules outside the build (`go_modules` service, `manual`
  mode) and commit `vendor.tar.gz`, so `%build` compiles offline with
  `-mod=vendor`.

`go.mod` pins `toolchain go1.26.6`, so every enabled OBS target must ship Go
1.26.6+ — otherwise the build would try (and fail) to download a toolchain.
Builds export `GOTOOLCHAIN=local` to make that failure explicit rather than a
silent network attempt. Drop a target whenever its Go is too old.

## Files

| File | Purpose |
|------|---------|
| `_service` | obs_scm snapshot (pinned SHA) + set_version + go_modules vendoring |
| `fortigate-cli.spec` | RPM recipe (openSUSE/Fedora) — binary + man + completions |
| `debian/` | Debian/Ubuntu source packaging (`3.0 (quilt)`) |
| `fortigate-cli.changes` | RPM changelog (OBS format) |

One OBS package holds both the `.spec` and `debian/`; OBS picks the right one
per target. No `_multibuild` is needed for the distro matrix.

## Cutting a release into OBS

After a `vX.Y.Z` tag exists on GitHub (so its commit is immutable), run from a
machine with `osc` configured and Go available:

```sh
packaging/scripts/update-obs.sh v1.4.0
# or target a different project/package:
packaging/scripts/update-obs.sh v1.4.0 home:ciriarte:fortigate-cli fortigate-cli
```

The script pins `_service` to the tag's SHA, copies packaging in, runs the
manual source services (snapshot + vendoring), verifies the vendor tree, and
`osc commit`s. Then watch the matrix:

```sh
osc results home:ciriarte:fortigate-cli fortigate-cli
```

## First-time project setup (once)

```sh
# Create the package under your home project
osc meta pkg -e home:ciriarte:fortigate-cli fortigate-cli

# Enable build targets + architectures (x86_64 and aarch64), e.g. via
#   osc meta prj -e home:ciriarte:fortigate-cli
# adding repositories for: openSUSE_Tumbleweed, 15.6 (Leap), Fedora_41,
# Debian_12, xUbuntu_24.04 — each with <arch>x86_64</arch><arch>aarch64</arch>.
```

Publish the repository (Publish flag on in project meta) so
`download.opensuse.org/repositories/home:/ciriarte:/fortigate-cli/` serves the
signed repos consumers add.

## Verifying locally before committing

```sh
cd packaging/obs
osc service manualrun          # generate snapshot + vendor.tar.gz
osc build openSUSE_Tumbleweed x86_64 fortigate-cli.spec   # RPM build
osc build Debian_12 x86_64 fortigate-cli.spec             # deb build
```

[obs]: https://build.opensuse.org
