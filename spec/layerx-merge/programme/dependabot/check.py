#!/usr/bin/env python3
"""check.py <slug>: validate reports/<slug>.kvx against input/<slug>.json."""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", "..", "..", ".."))
VERDICTS = {"upgraded", "pinned", "no-fix"}
KEYS = ["package", "manifest", "verdict", "version_before", "version_after", "pinned_by", "revision"]
LOCKS = {"package.json": ["package-lock.json", "pnpm-lock.yaml", "yarn.lock"], "go.mod": ["go.sum"], "Cargo.toml": ["Cargo.lock"], "pom.xml": []}


def parse(text):
    blocks = {}
    cur = None
    for line in text.splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        m = re.match(r"^\[alert\.([A-Za-z0-9-]+)\]$", line)
        if m:
            cur = m.group(1)
            blocks[cur] = {}
            continue
        m = re.match(r'^([a-z_]+) = "(.*)"$', line)
        if not m or cur is None:
            return None, "unparsed line: %s" % line
        blocks[cur][m.group(1)] = m.group(2)
    return blocks, None


def vtuple(v):
    parts = re.findall(r"\d+", v.split("-")[0].split("+")[0])
    return tuple(int(p) for p in parts) if parts else None


def lock_files(manifest):
    base = os.path.basename(manifest)
    d = os.path.dirname(manifest)
    names = [manifest] + [os.path.join(d, x) for x in LOCKS.get(base, [])]
    return names


def main():
    if len(sys.argv) != 2:
        print("usage: check.py <slug>")
        return 2
    slug = sys.argv[1]
    inp = json.load(open(os.path.join(HERE, "input", slug + ".json")))
    path = os.path.join(HERE, "reports", slug + ".kvx")
    errors = []
    if not os.path.isfile(path):
        print("%s: missing (1 errors)" % path)
        return 1
    blocks, err = parse(open(path).read())
    if err:
        print("%s: %s (1 errors)" % (path, err))
        return 1
    want = {a["ghsa"]: a for a in inp["alerts"]}
    for ghsa in want:
        if ghsa not in blocks:
            errors.append("no block for %s" % ghsa)
    for ghsa, b in blocks.items():
        if ghsa not in want:
            errors.append("%s is not an alert of this group" % ghsa)
            continue
        a = want[ghsa]
        if list(b.keys()) != KEYS:
            errors.append("%s: keys must be %s" % (ghsa, ", ".join(KEYS)))
            continue
        if b["package"] != a["package"]:
            errors.append("%s: package %s, alert says %s" % (ghsa, b["package"], a["package"]))
        if b["manifest"] != a["manifest"]:
            errors.append("%s: manifest %s, alert says %s" % (ghsa, b["manifest"], a["manifest"]))
        if b["verdict"] not in VERDICTS:
            errors.append("%s: verdict %s" % (ghsa, b["verdict"]))
            continue
        if not re.fullmatch(r"[0-9a-f]{40}", b["revision"]):
            errors.append("%s: revision is not a 40-hex commit" % ghsa)
        if not b["version_before"]:
            errors.append("%s: version_before is empty" % ghsa)
        if b["verdict"] == "upgraded":
            if not a["patched"]:
                errors.append("%s: upgraded, but the advisory names no patched version" % ghsa)
            after, patched = vtuple(b["version_after"]), vtuple(a["patched"] or "")
            if after is None or patched is None or after < patched:
                errors.append("%s: version_after %s is below the patched %s" % (ghsa, b["version_after"], a["patched"]))
            found = False
            for f in lock_files(a["manifest"]):
                p = os.path.join(ROOT, f)
                if os.path.isfile(p) and b["version_after"] in open(p, errors="replace").read():
                    found = True
                    break
            if not found:
                errors.append("%s: version_after %s does not appear in %s or its lock" % (ghsa, b["version_after"], a["manifest"]))
        else:
            if b["verdict"] == "no-fix" and a["patched"]:
                errors.append("%s: no-fix, but the advisory names %s" % (ghsa, a["patched"]))
            if b["verdict"] == "pinned" and not a["patched"]:
                errors.append("%s: pinned needs a patched version in the advisory" % ghsa)
            if len(b["pinned_by"]) < 8:
                errors.append("%s: pinned_by must name the dependency path" % ghsa)
    for e in errors:
        print(e)
    print("%s: %d alerts (%d errors)" % (path, len(want), len(errors)))
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
