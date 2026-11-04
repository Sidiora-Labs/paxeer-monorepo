#!/usr/bin/env python3
"""Validate spec/layerx-merge/programme/parity-stale.kvx: every stale selector is replaced by an existing test or retired with a reason."""
import os, re, sys

def parse(path):
    entries, cur = [], None
    for line in open(path, encoding="utf-8"):
        line = line.rstrip("\n")
        m = re.match(r"^\[stale\.(\d+)\]$", line)
        if m:
            cur = {"n": int(m.group(1))}; entries.append(cur); continue
        if cur is None or " = " not in line or line.startswith("#"):
            continue
        k, v = line.split(" = ", 1)
        cur[k] = v.strip().strip('"')
    return entries

def main():
    path = sys.argv[1] if len(sys.argv) > 1 else "spec/layerx-merge/programme/parity-stale.kvx"
    root = os.getcwd()
    entries = parse(path)
    errors = []
    if len(entries) != 136:
        errors.append("expected 136 entries, found %d" % len(entries))
    for e in entries:
        d = e.get("disposition", "")
        if d == "replacement":
            rep = e.get("replacement", "")
            if "::" not in rep:
                errors.append("stale.%d replacement must be path::TestName" % e["n"]); continue
            f, name = rep.split("::", 1)
            full = os.path.join(root, f)
            if not os.path.isfile(full):
                errors.append("stale.%d replacement file missing: %s" % (e["n"], f)); continue
            if name not in open(full, encoding="utf-8", errors="replace").read():
                errors.append("stale.%d test %s not found in %s" % (e["n"], name, f))
        elif d == "retired":
            if len(e.get("reason", "").split()) < 20:
                errors.append("stale.%d retired without a reason of twenty words" % e["n"])
        else:
            errors.append("stale.%d still %s" % (e["n"], d or "undecided"))
    for err in errors:
        print(err)
    print("entries %d errors %d" % (len(entries), len(errors)))
    return 1 if errors else 0

if __name__ == "__main__":
    sys.exit(main())
