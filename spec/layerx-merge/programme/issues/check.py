#!/usr/bin/env python3
"""Check one issue triage report against README.md. Usage: check.py <N>."""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(HERE))))
SPEC = os.path.join(ROOT, "spec", "layerx-merge", "spec.kvx")
VERDICTS = {"task", "already-fixed", "not-relevant", "duplicate", "spec-edit", "needs-owner"}
ROLES = {"fm-platform", "fm-agent", "fm-node-runtime", "fm-chain-modules", "fm-tools-ops",
         "fm-web-explorer-bridge", "fm-launchpad", "fm-wallet", "fm-markets", "fm-human",
         "fm-ledger-auth-pay", "fm-kernel", "fm-programs", "fm-interop", "fm-rpc-api",
         "fm-programme", "fm-cosmos-fork", "fm-parity", "fm-docs", "fm-web"}
PROFILES = {"go", "go-cgo", "rust", "node", "python", "solidity", "elixir", "jvm", "mixed"}
PROFILE_KEY = "".join(["sand", "box_profile"])  # the environment profile key of a task block
HEAD_KEYS = ["title", "status", "mode", "wave", "requires", "reqs", "capabilities", "interfaces"]
TAIL_KEYS = ["symbols", "touches", "tests", "planned_paths", "build_cmd", "verify_cmd", "gate_kind",
             PROFILE_KEY, "owner_role", "rollback", "max_builds", "max_verify_runs",
             "max_repair_cycles", "time_limit_minutes", "stop_condition", "path_policy", "evidence_policy"]
TAIL_VALUES = {
    "gate_kind": "planned", "max_builds": 1, "max_verify_runs": 1, "max_repair_cycles": 2,
    "time_limit_minutes": 30,
    "stop_condition": "Stop on the bounded gate result; record exact revision, command, exit code and private evidence location. Do not mark complete without the declared evidence.",
    "path_policy": "Existing paths are modified in place. Planned paths must be created or explicitly mapped to an existing module before implementation; a planned path never authorizes a duplicate implementation tree. A task touches only the paths it declares.",
    "evidence_policy": "Retain credentials, operational inventories and raw qualification records in access-controlled storage; publish only reviewed technical product material.",
}
errors = []


def err(msg):
    errors.append(msg)


def parse(text):
    blocks = {}
    order = []
    cur = None
    for n, line in enumerate(text.splitlines(), 1):
        if not line.strip() or line.startswith("#"):
            continue
        m = re.match(r"^\[([A-Za-z0-9_.]+)\]$", line)
        if m:
            cur = m.group(1)
            if cur in blocks:
                err("line %d: block %s repeated" % (n, cur))
            blocks[cur] = []
            order.append(cur)
            continue
        m = re.match(r"^([A-Za-z_][A-Za-z0-9_]*) = (.*)$", line)
        if not m or cur is None:
            err("line %d: not a block header or key = value" % n)
            continue
        try:
            value = json.loads(m.group(2))
        except ValueError:
            err("line %d: value of %s is not JSON" % (n, m.group(1)))
            continue
        blocks[cur].append((m.group(1), value))
    return blocks, order


def as_dict(items, name):
    d = {}
    for k, v in items:
        if k in d:
            err("%s: key %s repeated" % (name, k))
        d[k] = v
    return d


def text_key(d, name, key, pattern=None):
    v = d.get(key)
    if not isinstance(v, str) or not v.strip():
        err("%s: %s must be a non-empty string" % (name, key))
        return None
    if pattern and not re.fullmatch(pattern, v):
        err("%s: %s does not match %s" % (name, key, pattern))
    return v


def str_list(d, name, key, allow_empty=True):
    v = d.get(key)
    if not isinstance(v, list) or not all(isinstance(x, str) and x for x in v):
        err("%s: %s must be a list of strings" % (name, key))
        return []
    if not v and not allow_empty:
        err("%s: %s must not be empty" % (name, key))
    return v


def spec_task_ids():
    try:
        with open(SPEC) as f:
            return set(re.findall(r"^\[task\.([0-9.]+)\]", f.read(), re.M))
    except OSError:
        err("cannot read %s" % SPEC)
        return set()


def check_task(blocks, n, report_ids):
    name = "task.18.%d.1" % n
    if name not in blocks:
        err("missing [%s]" % name)
        return
    items = blocks[name]
    keys = [k for k, _ in items]
    dos = [k for k in keys if re.fullmatch(r"do_[0-9]+", k)]
    expected = HEAD_KEYS + ["do_%d" % i for i in range(1, len(dos) + 1)] + TAIL_KEYS
    if keys != expected:
        err("%s: keys must be exactly, in order: %s" % (name, ", ".join(expected)))
    if not dos:
        err("%s: at least do_1 is required" % name)
    d = as_dict(items, name)
    title = text_key(d, name, "title")
    if title and not title.startswith("Issue #%d: " % n):
        err("%s: title must start with Issue #%d: " % (name, n))
    if d.get("status") != "pending":
        err("%s: status must be pending" % name)
    if d.get("mode") != "BUILD":
        err("%s: mode must be BUILD" % name)
    wave = d.get("wave")
    if not isinstance(wave, int) or wave < 1:
        err("%s: wave must be a positive integer" % name)
    requires = str_list(d, name, "requires")
    known = spec_task_ids() | report_ids
    for r in requires:
        if r not in known:
            err("%s: requires names unknown task %s" % (name, r))
    if requires and wave == 1:
        err("%s: a task with requires has wave 2 or more" % name)
    if d.get("reqs") != ["18%d.1" % n]:
        err("%s: reqs must be [\"18%d.1\"]" % (name, n))
    if d.get("capabilities") != []:
        err("%s: capabilities must be []" % name)
    if d.get("interfaces") != []:
        err("%s: interfaces must be []" % name)
    for k in dos:
        text_key(d, name, k)
    str_list(d, name, "symbols")
    touches = str_list(d, name, "touches", allow_empty=False)
    for t in touches:
        if t.startswith("/") or ".." in t.split("/"):
            err("%s: touches entry %s is not a repository path" % (name, t))
    str_list(d, name, "tests")
    str_list(d, name, "planned_paths")
    for k in ("build_cmd", "verify_cmd"):
        v = text_key(d, name, k)
        if v and '"' in v:
            err("%s: %s contains a double quote" % (name, k))
    if d.get(PROFILE_KEY) not in PROFILES:
        err("%s: the environment profile must be one of %s" % (name, ", ".join(sorted(PROFILES))))
    if d.get("owner_role") not in ROLES:
        err("%s: owner_role must be one of %s" % (name, ", ".join(sorted(ROLES))))
    text_key(d, name, "rollback")
    for k, v in TAIL_VALUES.items():
        if d.get(k) != v:
            err("%s: %s must be the README value" % (name, k))
    rname = "req.18%d" % n
    if rname not in blocks:
        err("missing [%s]" % rname)
    else:
        r = as_dict(blocks[rname], rname)
        text_key(r, rname, "title")
        text_key(r, rname, "ac_1")
        for k in r:
            if k != "title" and not re.fullmatch(r"ac_[0-9]+", k):
                err("%s: unexpected key %s" % (rname, k))
    aname = "acceptance.18.%d.1" % n
    if aname not in blocks:
        err("missing [%s]" % aname)
    else:
        a = as_dict(blocks[aname], aname)
        if [k for k, _ in blocks[aname]] != ["feature", "req", "scenario", "contract", "test", "qualification"]:
            err("%s: keys must be feature, req, scenario, contract, test, qualification" % aname)
        if a.get("feature") != "18":
            err("%s: feature must be 18" % aname)
        if a.get("req") != "18%d.1" % n:
            err("%s: req must be 18%d.1" % (aname, n))
        s = text_key(a, aname, "scenario")
        if s and not s.startswith("Given "):
            err("%s: scenario starts with Given" % aname)
        text_key(a, aname, "contract", r"contracts/[A-Za-z0-9_./-]+\.md#[A-Za-z0-9_.-]+")
        text_key(a, aname, "test")
        if a.get("qualification") != "UNRUN":
            err("%s: qualification must be UNRUN" % aname)


def main():
    if len(sys.argv) != 2 or not sys.argv[1].isdigit():
        print("usage: check.py <issue number>")
        return 2
    n = int(sys.argv[1])
    path = os.path.join(HERE, "%d.kvx" % n)
    if not os.path.isfile(path):
        print("missing report %s" % path)
        return 1
    with open(path) as f:
        blocks, order = parse(f.read())
    hname = "issue.%d" % n
    if not order or order[0] != hname:
        err("the first block must be [%s]" % hname)
    h = as_dict(blocks.get(hname, []), hname)
    if h.get("number") != n:
        err("%s: number must be %d" % (hname, n))
    text_key(h, hname, "title")
    text_key(h, hname, "revision", r"[0-9a-f]{40}")
    verdict = h.get("verdict")
    if verdict not in VERDICTS:
        err("%s: verdict must be one of %s" % (hname, ", ".join(sorted(VERDICTS))))
    text_key(h, hname, "evidence")
    text_key(h, hname, "close_comment")
    for k in h:
        if k not in ("number", "title", "revision", "verdict", "evidence", "close_comment"):
            err("%s: unexpected key %s" % (hname, k))
    others = [b for b in order if b != hname]
    if verdict == "task":
        allowed = {"task.18.%d.1" % n, "req.18%d" % n, "acceptance.18.%d.1" % n}
        for b in others:
            if b not in allowed:
                err("unexpected block [%s]" % b)
        check_task(blocks, n, {"18.%d.1" % n})
    elif others:
        err("a report whose verdict is not task carries only the [%s] block" % hname)
    for e in errors:
        print("error: " + e)
    print("%s: %s (%d errors)" % (path, verdict, len(errors)))
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
