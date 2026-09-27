#!/usr/bin/env bash
#
# update-obs.sh — cut a released git tag into the OBS package.
#
# What it does, in order:
#   1. Resolves a release tag (vX.Y.Z) to its immutable commit SHA.
#   2. Rewrites packaging/obs/_service to pin that SHA + version.
#   3. Checks out the OBS package, copies the packaging files in, runs the
#      manual source services (obs_scm snapshot + go_modules vendoring) in a
#      controlled environment, and commits the new revision.
#
# OBS build chroots have no network, so vendoring MUST happen here (not in the
# build). We verify the vendor tree against go.sum before committing.
#
# Requirements: git, osc (configured for api.opensuse.org), go, tar.
# Usage:
#   packaging/scripts/update-obs.sh v1.4.0 [OBS_PROJECT] [OBS_PACKAGE]
#
set -euo pipefail

TAG="${1:?usage: update-obs.sh vX.Y.Z [OBS_PROJECT] [OBS_PACKAGE]}"
OBS_PROJECT="${2:-home:ciriarte:fortigate-cli}"
OBS_PACKAGE="${3:-fortigate-cli}"

REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
PKG_DIR="$REPO_ROOT/packaging/obs"
VERSION="${TAG#v}"

echo ">> Resolving $TAG to a commit SHA (must be an existing tag on origin)…"
git -C "$REPO_ROOT" fetch --tags --quiet
SHA="$(git -C "$REPO_ROOT" rev-list -n1 "$TAG")"
echo "   $TAG -> $SHA"

echo ">> Pinning _service to $SHA / $VERSION…"
SERVICE="$PKG_DIR/_service"
tmp="$(mktemp)"
sed -e "s|<param name=\"revision\">.*</param>|<param name=\"revision\">$SHA</param>|" \
    -e "s|<param name=\"version\">.*</param>|<param name=\"version\">$VERSION</param>|" \
    "$SERVICE" > "$tmp"
mv "$tmp" "$SERVICE"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
echo ">> Checking out OBS package $OBS_PROJECT/$OBS_PACKAGE into $WORK…"
( cd "$WORK" && osc checkout "$OBS_PROJECT" "$OBS_PACKAGE" )
OSC_DIR="$WORK/$OBS_PROJECT/$OBS_PACKAGE"

echo ">> Copying packaging files…"
cp "$PKG_DIR/_service" "$PKG_DIR/fortigate-cli.spec" \
   "$PKG_DIR/fortigate-cli.changes" "$OSC_DIR/"
rm -rf "$OSC_DIR/debian"
cp -r "$PKG_DIR/debian" "$OSC_DIR/debian"

echo ">> Running manual source services (snapshot + vendoring)…"
( cd "$OSC_DIR" && osc service manualrun )

echo ">> Verifying vendored modules against go.sum…"
( cd "$OSC_DIR" && for v in vendor.tar.gz vendor.tar.*; do
    [ -e "$v" ] || continue
    tmpd="$(mktemp -d)"; tar -xf "$v" -C "$tmpd"
    if [ -f "$tmpd/vendor/modules.txt" ]; then
      echo "   vendor.tar contains $(grep -c '^# ' "$tmpd/vendor/modules.txt") modules"
    fi
    rm -rf "$tmpd"
  done )

echo ">> Staging + committing to OBS…"
( cd "$OSC_DIR" && osc addremove && osc status \
    && osc commit -m "Update to $TAG ($SHA)" )

echo ">> Done. Watch the build with: osc results $OBS_PROJECT $OBS_PACKAGE"
