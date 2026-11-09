from pathlib import Path
import re
import tomllib

import yaml
from yaml.nodes import MappingNode, ScalarNode, SequenceNode


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"


def _mappings(node):
    if isinstance(node, MappingNode):
        yield node
        for _, value in node.value:
            yield from _mappings(value)
    elif isinstance(node, SequenceNode):
        for value in node.value:
            yield from _mappings(value)


def _fields(node):
    assert isinstance(node, MappingNode), "Expected a YAML mapping"
    return {key.value: value for key, value in node.value}


def _workflow_actions(action):
    paths = sorted(
        path
        for path in WORKFLOWS.rglob("*")
        if path.is_file() and path.suffix in {".yml", ".yaml"}
    )
    assert paths, "No GitHub workflows found"
    matches = []
    for path in paths:
        source = path.read_text(encoding="utf-8")
        lines = source.splitlines()
        workflow = yaml.compose(source, Loader=yaml.BaseLoader)
        assert isinstance(workflow, MappingNode), path
        for node in _mappings(workflow):
            fields = _fields(node)
            if "uses" not in fields:
                continue
            uses = fields["uses"]
            assert isinstance(uses, ScalarNode), path
            if uses.value.partition("@")[0] != action:
                continue
            location = f"{path.relative_to(ROOT)}:{uses.start_mark.line + 1}"
            trailing = lines[uses.end_mark.line][uses.end_mark.column:]
            comment = trailing.partition("#")[2].strip()
            matches.append((location, fields, comment))
    assert matches, f"No uses of {action} found"
    return matches


def test_rust_toolchain_action_names_exact_version():
    toolchain = tomllib.loads((ROOT / "rust-toolchain.toml").read_text())
    version = toolchain["toolchain"]["channel"]
    assert version == "1.91.1"
    assert re.fullmatch(r"\d+\.\d+\.\d+", version), version
    for location, fields, comment in _workflow_actions("dtolnay/rust-toolchain"):
        assert "with" in fields, (location, "Missing explicit Rust inputs")
        inputs = _fields(fields["with"])
        assert "toolchain" in inputs, (location, "Missing explicit toolchain")
        channel = inputs["toolchain"]
        assert isinstance(channel, ScalarNode), location
        assert channel.value == version, (location, channel.value, version)
        assert comment == version, (location, comment, version)


def test_foundry_version_is_exact_and_uniform():
    versions = set()
    for location, fields, _ in _workflow_actions("foundry-rs/foundry-toolchain"):
        assert "with" in fields, (location, "Missing explicit Foundry inputs")
        inputs = _fields(fields["with"])
        assert "version" in inputs, (location, "Missing explicit Foundry version")
        version = inputs["version"]
        assert isinstance(version, ScalarNode), location
        assert re.fullmatch(r"v\d+\.\d+\.\d+", version.value), (
            location,
            version.value,
        )
        assert version.value == "v1.8.1", (location, version.value)
        versions.add(version.value)
    assert versions == {"v1.8.1"}, versions
