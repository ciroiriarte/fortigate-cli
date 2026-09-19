#!/usr/bin/env python3
"""Extract the per-object field inventory (and cross-version diff) from FortiOS
CLI Reference PDFs. This is the reproducible pipeline behind docs/api/.

The CLI Reference `config <path>` compact blocks are the authoritative,
version-exact source of the cmdb object model (cmdb REST paths map 1:1 to CLI
config trees). This tool turns them into machine-readable JSON and a diff.

Usage:
  # 1. Convert the reference PDFs (see references/, gitignored) to text:
  pdftotext -q FortiOS-8.0.1-CLI_Reference.pdf 8.0.1.txt
  # 2. Extract + diff:
  python3 tools/fortios-cli-extract.py \
      --version 7.4.12=7.4.12.txt --version 7.6.7=7.6.7.txt --version 8.0.1=8.0.1.txt \
      --object "firewall address" --object "firewall policy" \
      --json docs/api/field-inventory.json --diff

  # With no --object flags, a built-in curated set is used.

Handles PDF-to-text artifacts: doubled object headers, page-break lines
('FortiOS X.Y.Z CLI Reference' / 'Fortinet Inc.' / bare page numbers), nested
config subtables. Field values (types/enums/defaults) are NOT parsed here — grep
the CLI Reference text or use `GET /api/v2/cmdb/<path>?action=schema` on a live
unit for those (the authoritative per-build source; see docs/api/rest-conventions.md).

Why pdftotext and not an ML PDF-to-markdown tool: the CLI Reference object
definitions are monospaced config trees (`set <field> {type}` / `config <sub>`),
not visual tables — pdftotext extracts field names and nesting near-perfectly and
in seconds. ML converters (e.g. datalab-to/marker) only add value on the per-object
"Parameter | Description | Type | Size" tables (types/enums/defaults), and for that
`?action=schema` on a live unit is strictly better (authoritative JSON per build).
Reach for marker only as an offline fallback when no device is reachable.
"""
import argparse
import json
import re
import sys
from pathlib import Path

SET_RE = re.compile(r"^set ([A-Za-z0-9_.\-]+)\b")
CONFIG_RE = re.compile(r"^config ([A-Za-z0-9_.\- ]+)$")
NOISE_RE = re.compile(r"^(FortiOS .*CLI Reference|Fortinet Inc\.|\d+)\s*$")

DEFAULT_OBJECTS = [
    "firewall address", "firewall addrgrp", "firewall service custom",
    "firewall service group", "firewall policy", "system interface",
    "system admin", "system dns", "system global", "router static",
    "router policy", "switch-controller managed-switch",
    "switch-controller lldp-settings", "switch-controller lldp-profile",
    "vpn ipsec phase1-interface", "vpn ipsec phase2-interface", "system api-user",
]


def clean(text):
    out = []
    for ln in text.splitlines():
        s = ln.rstrip()
        if s and not NOISE_RE.match(s):
            out.append(s)
    return out


def extract_block(lines, obj):
    """Return {'fields': [...], 'child_tables': [...]} or None."""
    header = f"config {obj}"
    start = None
    for i, ln in enumerate(lines):
        if ln == header and i + 1 < len(lines) and lines[i + 1].startswith("Description:"):
            start = i
            break
    if start is None:
        return None
    top, subs = set(), set()
    depth, started = 0, False
    for ln in lines[start:]:
        cm = CONFIG_RE.match(ln)
        if cm:
            if started and depth == 1:
                subs.add(cm.group(1).strip())
            depth += 1
            started = True
            continue
        if ln == "end":
            depth -= 1
            if started and depth == 0:
                break
            continue
        sm = SET_RE.match(ln)
        if sm and depth == 1:
            top.add(sm.group(1))
    return {"fields": sorted(top), "child_tables": sorted(subs)}


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--version", action="append", metavar="LABEL=FILE", required=True,
                    help="version label and its CLI-Reference text file (repeatable)")
    ap.add_argument("--object", action="append", metavar='"cli path"', default=None,
                    help="config object to extract (repeatable); default = curated set")
    ap.add_argument("--json", metavar="PATH", help="write the inventory JSON here")
    ap.add_argument("--diff", action="store_true", help="print cross-version field deltas")
    args = ap.parse_args(argv)

    versions = {}
    for spec in args.version:
        label, _, path = spec.partition("=")
        versions[label] = clean(Path(path).read_text(errors="replace"))
    objects = args.object or DEFAULT_OBJECTS

    inv = {}
    for obj in objects:
        inv[obj] = {v: extract_block(lines, obj) for v, lines in versions.items()}

    if args.json:
        Path(args.json).write_text(json.dumps(inv, indent=1))
        print(f"wrote {args.json}: {len(inv)} objects x {len(versions)} versions", file=sys.stderr)

    if args.diff or not args.json:
        vers = list(versions)
        for obj in objects:
            print(f"\n### config {obj}")
            for v in vers:
                r = inv[obj][v]
                print(f"  {v}: " + ("NOT FOUND" if r is None else
                                     f"{len(r['fields'])} fields, {len(r['child_tables'])} child-tables"))
            for a, b in zip(vers, vers[1:]):
                ra, rb = inv[obj][a], inv[obj][b]
                if not (ra and rb):
                    continue
                fa, fb = set(ra["fields"]), set(rb["fields"])
                ta, tb = set(ra["child_tables"]), set(rb["child_tables"])
                for label, s in (("+field", fb - fa), ("-field", fa - fb),
                                 ("+table", tb - ta), ("-table", ta - tb)):
                    if s:
                        print(f"    {label} {a}->{b}: {', '.join(sorted(s))}")


if __name__ == "__main__":
    main()
