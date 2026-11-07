from collections import defaultdict
from pathlib import Path
import re

import yaml
from yaml.nodes import MappingNode, ScalarNode, SequenceNode


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
PIN = re.compile(r"([^@\s]+)@([0-9a-fA-F]{40})")
VERSION = re.compile(r"v?([0-9]+)(?:\.[0-9]+){0,2}(?:[-+][0-9A-Za-z.-]+)?")
TOOLCHAIN_VERSION = re.compile(r"stable|nightly|1\.[0-9]+")


def _uses_nodes(node):
    if isinstance(node, MappingNode):
        for key, value in node.value:
            if isinstance(key, ScalarNode) and key.value == "uses":
                assert isinstance(value, ScalarNode), "uses must be a scalar"
                yield value
            yield from _uses_nodes(value)
    elif isinstance(node, SequenceNode):
        for value in node.value:
            yield from _uses_nodes(value)


def _workflow_uses():
    paths = sorted(
        path
        for path in WORKFLOWS.rglob("*")
        if path.is_file() and path.suffix in {".yml", ".yaml"}
    )
    assert paths, "No GitHub workflows found"
    uses = []
    for path in paths:
        source = path.read_text(encoding="utf-8")
        lines = source.splitlines()
        workflow = yaml.compose(source, Loader=yaml.BaseLoader)
        for node in _uses_nodes(workflow):
            location = f"{path.relative_to(ROOT)}:{node.start_mark.line + 1}"
            trailing = lines[node.end_mark.line][node.end_mark.column:]
            comment = trailing.partition("#")[2].strip()
            uses.append((location, node.value, comment))
    assert uses, "No action uses found in GitHub workflows"
    return uses


def test_each_action_has_one_pinned_sha():
    pins = defaultdict(lambda: defaultdict(list))
    invalid = []
    for location, reference, _ in _workflow_uses():
        match = PIN.fullmatch(reference)
        if match is None:
            invalid.append(f"{location}: {reference}")
        else:
            action, sha = match.groups()
            pins[action][sha.lower()].append(location)
    conflicts = {action: shas for action, shas in pins.items() if len(shas) != 1}
    assert not invalid and not conflicts, (
        "Each action must use one consistent 40-character SHA across all workflows",
        {"invalid_pins": invalid, "conflicting_pins": conflicts},
    )


def test_codeql_action_steps_share_version():
    steps = set()
    shas = set()
    majors = set()
    for location, reference, comment in _workflow_uses():
        if not reference.startswith("github/codeql-action/"):
            continue
        pin = PIN.fullmatch(reference)
        assert pin is not None, (location, reference)
        version = VERSION.fullmatch(comment)
        assert version is not None, (location, comment)
        steps.add(pin[1].removeprefix("github/codeql-action/"))
        shas.add(pin[2].lower())
        majors.add(version[1])
    assert {"init", "analyze", "upload-sarif"} <= steps, steps
    assert len(shas) == 1, ("CodeQL steps must share one release SHA", shas)
    assert len(majors) == 1, ("CodeQL steps must share one major version", majors)


def test_pins_carry_version_comment():
    invalid = []
    for location, reference, comment in _workflow_uses():
        pin = PIN.fullmatch(reference)
        version = (
            TOOLCHAIN_VERSION
            if pin is not None and pin[1] == "dtolnay/rust-toolchain"
            else VERSION
        )
        if pin is None or version.fullmatch(comment) is None:
            invalid.append(f"{location}: {reference} # {comment}")
    assert not invalid, (
        "Every action pin must carry a release version or toolchain tag comment",
        invalid,
    )
