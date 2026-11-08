"""Operator documentation uses native commands and describes observable checks."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]
RELEASE_DOCS = (
    "docs/release/7.0.0-readiness-checklist.md",
    "docs/release/7.0.0-service-operator-runbook.md",
    "docs/release/7.0.0-upgrade-plan.md",
)
PRODUCT_DOCS = (
    "docs/platform/content/beta.md",
    "storage/state_db/bench/README.md",
    "tools/hpx/README.md",
    "tools/launchpad/production/dispatcher/README.md",
    "tools/launchpad/production/load/README.md",
) + RELEASE_DOCS
OPERATOR_SCRIPTS = (
    "tools/hpx/fleet-keygen.sh",
    "tools/hpx/stake-fleet.sh",
)
SPEC_CLI_COMMAND = re.compile(
    r"\b(?:cg|hyperfide|codify)\s+spec\b|\bhf-(?:build|done)\b",
    re.IGNORECASE,
)
PROGRAMME_GATE_ID = re.compile(
    r"\b(?:17|22)\.\d+(?:\.\d+)+\b"
    r"|\b(?:tasks?|gates?)[\s`*#:_-]+\d+(?:\.\d+)+\b",
    re.IGNORECASE,
)
SERVER_HOME_PATH = re.compile(r"/(?:root|home/[^\s/`\"']+)(?:/|\b)")


def _references(paths, pattern):
    references = []
    for name in paths:
        path = ROOT / name
        assert path.is_file(), f"Required documentation is missing: {name}"
        content = path.read_text(encoding="utf-8")
        assert content.strip(), f"Required documentation is empty: {name}"
        # Join shell continuations without discarding prose, comments or code spans.
        content = re.sub(r"\\\r?\n", " \n", content)
        for match in pattern.finditer(content):
            line = content.count("\n", 0, match.start()) + 1
            references.append(f"{name}:{line}: {match.group()}")
    return references


def test_docs_have_no_spec_cli_commands():
    references = _references(PRODUCT_DOCS + OPERATOR_SCRIPTS, SPEC_CLI_COMMAND)
    assert not references, "Engineering process commands in operator docs:\n" + "\n".join(
        references
    )


def test_release_docs_have_no_programme_gate_ids():
    references = _references(RELEASE_DOCS, PROGRAMME_GATE_ID)
    assert not references, "Internal gate IDs in release docs:\n" + "\n".join(references)


def test_operator_scripts_have_no_server_home_paths():
    references = _references(OPERATOR_SCRIPTS, SERVER_HOME_PATH)
    assert not references, "Server home paths in operator scripts:\n" + "\n".join(references)
