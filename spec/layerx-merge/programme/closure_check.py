"""Check closure disposition files against their finding list.

usage: python3 spec/layerx-merge/programme/closure_check.py <ids file> <disposition kvx>...

The ids file lists finding ids, one per line. Every id must appear exactly once as a
[disposition.<id>] block across the given files, in the file of its domain (dN-k.kvx for a
finding N.x.y), with the keys finding (equal to the id), status (resolved, superseded,
environment or open) and evidence (product facts, at least 60 characters); an open
disposition also names a follow_up task that exists in spec/layerx-merge/spec.kvx. Every
listed id must exist as a [finding.<id>] block in spec/layerx-merge/qualification.kvx.
Exit 0 when every check passes, 1 with one line per failure otherwise. Run from the repository root.
"""
from __future__ import annotations

import os
import re
import sys

STATUSES = ("resolved", "superseded", "environment", "open")


def blocks(text: str) -> dict[str, dict[str, str]]:
    out: dict[str, dict[str, str]] = {}
    cur = None
    for line in text.splitlines():
        m = re.match(r"^\[disposition\.([^\]]+)\]$", line)
        if m:
            cur = m.group(1)
            if cur in out:
                out[cur]["__dup__"] = "1"
            else:
                out[cur] = {}
            continue
        if line.startswith("[") and line.endswith("]"):
            cur = None
            continue
        m = re.match(r'^([a-z_]+) = "(.*)"$', line)
        if m and cur is not None:
            out[cur][m.group(1)] = m.group(2)
    return out


def main(argv: list[str]) -> int:
    if len(argv) < 3:
        print(__doc__)
        return 2
    ids = [l.strip() for l in open(argv[1]) if l.strip()]
    spec = open("spec/layerx-merge/spec.kvx").read()
    qual = open("spec/layerx-merge/qualification.kvx").read()
    findings = set(re.findall(r"^\[finding\.([^\]]+)\]", qual, re.M))
    failures: list[str] = []
    where: dict[str, str] = {}
    data: dict[str, dict[str, str]] = {}
    for path in argv[2:]:
        if not os.path.isfile(path):
            failures.append("%s: missing" % path)
            continue
        m = re.match(r"d(\d+)-\d+\.kvx$", os.path.basename(path))
        if not m:
            failures.append("%s: name is not d<domain>-<k>.kvx" % path)
            continue
        for fid, b in blocks(open(path).read()).items():
            if "__dup__" in b:
                failures.append("%s: [disposition.%s] appears twice" % (path, fid))
            if fid in where:
                failures.append("%s: [disposition.%s] also in %s" % (path, fid, where[fid]))
            where[fid] = path
            data[fid] = b
            if fid.split(".")[0] != m.group(1):
                failures.append("%s: [disposition.%s] belongs to d%s" % (path, fid, fid.split(".")[0]))
    for fid in ids:
        if fid not in findings:
            failures.append("%s: no [finding.%s] in qualification.kvx" % (fid, fid))
        b = data.get(fid)
        if b is None:
            failures.append("%s: no disposition" % fid)
            continue
        if b.get("finding") != fid:
            failures.append("%s: finding key is %r" % (fid, b.get("finding")))
        status = b.get("status")
        if status not in STATUSES:
            failures.append("%s: status %r" % (fid, status))
        evidence = b.get("evidence", "")
        if len(evidence) < 60:
            failures.append("%s: evidence shorter than 60 characters" % fid)
        if status == "open":
            fu = b.get("follow_up", "")
            if not fu or ("[task.%s]" % fu) not in spec:
                failures.append("%s: open without an existing follow_up task (%r)" % (fid, fu))
    extra = [fid for fid in data if fid not in ids]
    for fid in extra:
        failures.append("%s: not in the ids list" % fid)
    for line in failures:
        print(line)
    if not failures:
        print("closure_check: %d dispositions ok" % len(ids))
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
