"""Check one pre-upgrade audit record against its package.

usage: python3 spec/layerx-merge/programme/audit_check.py <package kvx> <record kvx>

The package (spec/layerx-merge/programme/audit/packages/<task>.kvx) names the files a task
reviews, the requirement ids, the interface-registry rows it produces and consumes and the
documentation pages that describe it. The record (spec/layerx-merge/programme/audit/records/
<task>.kvx) is the task's only deliverable. The grammar is the programme kvx grammar
(spec/layerx-merge/programme/kvx.py): one key = value per line, strings double quoted,
lists on one line.

Required blocks:

[audit.<task>]      task, revision (40 hex), files_reviewed, findings, docs_reviewed (ints
                    equal to the block counts below), upgrade_ready (bool), blocking (list of
                    finding block ids; every high finding and nothing else)
[file.<k>]          one per package member: path, lines (the file's line count in the tree
                    being audited), verdict (clean | issues | generated | vendored), note
                    (required for generated and vendored, at least 20 characters)
[finding.<k>]       file (a package member, or an existing documentation path for doc-drift),
                    line (1-based; 0 only when the finding is that something is missing),
                    class (interface-mismatch | wiring-gap | retired-reference | runtime-risk |
                    restart-recovery | config-drift | doc-drift | test-gap | dead-code |
                    security), severity (high | medium | low), observed, expected, proposal
                    (each at least 40 characters), owner_domain (1..17), interface (required
                    for interface-mismatch: an id present in spec/layerx-merge/interfaces.kvx)
[doc.<k>]           one per package doc plus any other page read: path (exists), status
                    (accurate | stale | misleading | missing), notes (at least 20 characters)

Every file with verdict issues has at least one finding and every finding on a member names a
file whose verdict is issues. Exit 0 when every check passes, 1 with one line per failure
otherwise. Run from the repository root.
"""
from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from kvx import KvxError, load  # noqa: E402

CLASSES = (
    "interface-mismatch", "wiring-gap", "retired-reference", "runtime-risk",
    "restart-recovery", "config-drift", "doc-drift", "test-gap", "dead-code", "security",
)
SEVERITIES = ("high", "medium", "low")
VERDICTS = ("clean", "issues", "generated", "vendored")
DOC_STATUSES = ("accurate", "stale", "misleading", "missing")
INTERFACES = "spec/layerx-merge/interfaces.kvx"


def line_count(path: str) -> int:
    with open(path, "rb") as handle:
        data = handle.read()
    if not data:
        return 0
    return data.count(b"\n") + (0 if data.endswith(b"\n") else 1)


def interface_exists(interface_id: str) -> bool:
    needle = f"[interface.{interface_id}]"
    try:
        with open(INTERFACES, encoding="utf-8") as handle:
            return any(line.rstrip("\n") == needle for line in handle)
    except OSError:
        return False


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(__doc__.strip().splitlines()[2])
        return 2
    package_path, record_path = argv[1], argv[2]
    errors: list[str] = []
    try:
        package = load(package_path)
    except (OSError, KvxError) as error:
        print(f"package: {error}")
        return 1
    try:
        record = load(record_path)
    except (OSError, KvxError) as error:
        print(f"record: {error}")
        return 1
    heads = [name for name in package if name.startswith("package.")]
    if len(heads) != 1:
        print("package: expected exactly one [package.<task>] block")
        return 1
    task = heads[0][len("package."):]
    pkg = package[heads[0]]
    members = list(pkg.get("members", []))
    docs = list(pkg.get("docs", []))

    audit = record.get(f"audit.{task}")
    if audit is None:
        errors.append(f"record: missing [audit.{task}]")
        audit = {}
    if audit.get("task") != task:
        errors.append(f"[audit.{task}] task must equal {task!r}")
    revision = audit.get("revision")
    if not isinstance(revision, str) or not re.fullmatch(r"[0-9a-f]{40}", revision):
        errors.append(f"[audit.{task}] revision must be 40 lowercase hex characters")
    if not isinstance(audit.get("upgrade_ready"), bool):
        errors.append(f"[audit.{task}] upgrade_ready must be true or false")

    files = {k: v for k, v in record.items() if k.startswith("file.")}
    findings = {k: v for k, v in record.items() if k.startswith("finding.")}
    doc_blocks = {k: v for k, v in record.items() if k.startswith("doc.")}
    for key, blocks in (("files_reviewed", files), ("findings", findings), ("docs_reviewed", doc_blocks)):
        if audit.get(key) != len(blocks):
            errors.append(f"[audit.{task}] {key} must equal {len(blocks)}")

    verdicts: dict[str, str] = {}
    for name, block in files.items():
        path = block.get("path")
        if not isinstance(path, str) or path not in members:
            errors.append(f"[{name}] path must be a package member")
            continue
        if path in verdicts:
            errors.append(f"[{name}] duplicate file entry for {path}")
        verdict = block.get("verdict")
        if verdict not in VERDICTS:
            errors.append(f"[{name}] verdict must be one of {', '.join(VERDICTS)}")
        verdicts[path] = str(verdict)
        lines = block.get("lines")
        if not isinstance(lines, int) or isinstance(lines, bool):
            errors.append(f"[{name}] lines must be an integer")
        elif os.path.isfile(path):
            actual = line_count(path)
            if lines != actual:
                errors.append(f"[{name}] lines is {lines} but {path} has {actual} lines")
        else:
            errors.append(f"[{name}] {path} is not a file in this tree")
        if verdict in ("generated", "vendored"):
            note = block.get("note")
            if not isinstance(note, str) or len(note) < 20:
                errors.append(f"[{name}] a {verdict} verdict needs a note of at least 20 characters")
    for path in members:
        if path not in verdicts:
            errors.append(f"record: no [file.*] entry for package member {path}")

    issue_files_with_findings: set[str] = set()
    blocking_expected: list[str] = []
    for name, block in findings.items():
        path = block.get("file")
        is_doc_finding = block.get("class") == "doc-drift"
        if not isinstance(path, str) or not (path in members or (is_doc_finding and os.path.exists(path))):
            errors.append(f"[{name}] file must be a package member (or an existing documentation path for doc-drift)")
        elif path in members:
            if verdicts.get(path) != "issues":
                errors.append(f"[{name}] {path} carries a finding but its verdict is not issues")
            issue_files_with_findings.add(path)
        line = block.get("line")
        if not isinstance(line, int) or isinstance(line, bool) or line < 0:
            errors.append(f"[{name}] line must be a non-negative integer")
        elif isinstance(path, str) and line > 0 and os.path.isfile(path) and line > line_count(path):
            errors.append(f"[{name}] line {line} is past the end of {path}")
        if block.get("class") not in CLASSES:
            errors.append(f"[{name}] class must be one of {', '.join(CLASSES)}")
        severity = block.get("severity")
        if severity not in SEVERITIES:
            errors.append(f"[{name}] severity must be one of {', '.join(SEVERITIES)}")
        elif severity == "high":
            blocking_expected.append(name[len("finding."):])
        for key in ("observed", "expected", "proposal"):
            value = block.get(key)
            if not isinstance(value, str) or len(value) < 40:
                errors.append(f"[{name}] {key} must be a string of at least 40 characters")
        owner = block.get("owner_domain")
        if not isinstance(owner, int) or isinstance(owner, bool) or not 1 <= owner <= 17:
            errors.append(f"[{name}] owner_domain must be an integer from 1 to 17")
        interface = block.get("interface")
        if block.get("class") == "interface-mismatch" and not isinstance(interface, str):
            errors.append(f"[{name}] an interface-mismatch finding names its interface id")
        if isinstance(interface, str) and not interface_exists(interface):
            errors.append(f"[{name}] interface {interface} is not a row of {INTERFACES}")
    for path, verdict in verdicts.items():
        if verdict == "issues" and path not in issue_files_with_findings:
            errors.append(f"record: {path} has verdict issues but no finding names it")

    blocking = audit.get("blocking")
    if not isinstance(blocking, list):
        errors.append(f"[audit.{task}] blocking must be a list of finding ids")
    elif sorted(blocking) != sorted(blocking_expected):
        errors.append(f"[audit.{task}] blocking must list exactly the high findings: {sorted(blocking_expected)}")
    elif audit.get("upgrade_ready") is True and blocking_expected:
        errors.append(f"[audit.{task}] upgrade_ready cannot be true while high findings exist")

    seen_docs: set[str] = set()
    for name, block in doc_blocks.items():
        path = block.get("path")
        if not isinstance(path, str) or not path:
            errors.append(f"[{name}] path must be a string")
            continue
        status = block.get("status")
        if status not in DOC_STATUSES:
            errors.append(f"[{name}] status must be one of {', '.join(DOC_STATUSES)}")
        if status != "missing" and not os.path.exists(path):
            errors.append(f"[{name}] {path} does not exist in this tree")
        notes = block.get("notes")
        if not isinstance(notes, str) or len(notes) < 20:
            errors.append(f"[{name}] notes must be a string of at least 20 characters")
        seen_docs.add(path)
    for path in docs:
        if path not in seen_docs:
            errors.append(f"record: no [doc.*] entry for package doc {path}")

    for line in errors:
        print(line)
    if errors:
        return 1
    print(
        f"audit {task}: {len(files)} files, {len(findings)} findings "
        f"({len(blocking_expected)} high), {len(doc_blocks)} docs, upgrade_ready={audit.get('upgrade_ready')}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
