#!/usr/bin/env python3
"""Gate for a documentation rewrite manifest (spec/layerx-merge/programme/docs/rewrite/<task>.kvx).

Every [page.N] path exists and is non-empty, a Markdown page has a heading, and no page line carries a
retired reference (router URL, /rpc paths, a separate LayerX interface, a sequencer service, the layerxd
binary, guarantor finality, node.hyperpaxeer.com as an RPC endpoint) unless the same sentence states
that the reference is retired. Exit 0 only when every page passes.
"""
import os, re, sys

RETIRED = [
    (re.compile(r"paxeer\.network/rpc\b", re.I), "router URL"),
    (re.compile(r"(^|[^\w.])/rpc(/ws|/schema)?(\b|$)"), "/rpc path"),
    (re.compile(r"\bLayerX interface\b", re.I), "separate LayerX interface"),
    (re.compile(r"\bsequencer service\b", re.I), "sequencer service"),
    (re.compile(r"\blayerxd\b"), "layerxd binary"),
    (re.compile(r"\bguarantor finality\b", re.I), "guarantor finality"),
    (re.compile(r"node\.hyperpaxeer\.com", re.I), "node.hyperpaxeer.com"),
]
EXEMPT = re.compile(r"retired|no longer|there is no|is not an? |never |removed|pre-merge|replaced|mirror", re.I)

def parse(path):
    pages, cur = [], None
    for line in open(path, encoding="utf-8"):
        line = line.rstrip("\n")
        if re.match(r"^\[page\.\d+\]$", line):
            cur = {}; pages.append(cur); continue
        if cur is not None and " = " in line and not line.startswith("#"):
            k, v = line.split(" = ", 1); cur[k] = v.strip().strip('"')
    return pages

def check_page(path):
    errors = []
    if not os.path.isfile(path):
        return ["missing: %s" % path]
    text = open(path, encoding="utf-8", errors="replace").read()
    if not text.strip():
        return ["empty: %s" % path]
    if path.endswith((".md", ".mdx")) and not re.search(r"^#{1,6} \S", text, re.M):
        errors.append("no heading: %s" % path)
    for n, line in enumerate(text.splitlines(), 1):
        for rx, label in RETIRED:
            if rx.search(line) and not EXEMPT.search(line):
                errors.append("%s:%d retired reference (%s)" % (path, n, label))
                break
    return errors

def main():
    if len(sys.argv) != 2:
        print("usage: docs_check.py <manifest.kvx>"); return 2
    pages = parse(sys.argv[1])
    if not pages:
        print("no pages in manifest"); return 1
    errors = []
    for p in pages:
        errors += check_page(p["path"])
    for e in errors:
        print(e)
    print("pages %d errors %d" % (len(pages), len(errors)))
    return 1 if errors else 0

if __name__ == "__main__":
    sys.exit(main())
