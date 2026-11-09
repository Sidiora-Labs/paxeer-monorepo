from pathlib import Path
import re

import yaml
from yaml.nodes import MappingNode, ScalarNode, SequenceNode


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
RUNNER_LABEL = re.compile(
    r"(?<![\w-])(?:ubuntu|macos)-(?:latest|[0-9][\w.-]*)(?![\w-])"
)
LINUX_FALLBACK = re.compile(r"vars\.CI_LINUX_RUNNER\s*\|\|\s*(['\"])(.*?)\1")


def _scalars(node):
    if isinstance(node, ScalarNode):
        yield node
    elif isinstance(node, MappingNode):
        for _, value in node.value:
            yield from _scalars(value)
    elif isinstance(node, SequenceNode):
        for value in node.value:
            yield from _scalars(value)


def _workflow_scalars():
    paths = sorted(
        path
        for path in WORKFLOWS.rglob("*")
        if path.is_file() and path.suffix in {".yml", ".yaml"}
    )
    assert paths, "No GitHub workflows found"
    for path in paths:
        workflow = yaml.compose(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
        assert isinstance(workflow, MappingNode), path
        for node in _scalars(workflow):
            location = f"{path.relative_to(ROOT)}:{node.start_mark.line + 1}"
            yield location, node.value


def test_no_latest_runner_labels():
    labels = set()
    for location, value in _workflow_scalars():
        assert not re.search(r"\b(?:ubuntu|macos|windows)-latest\b", value), (
            location,
            value,
        )
        for label in RUNNER_LABEL.findall(value):
            assert label in {"ubuntu-24.04", "ubuntu-24.04-arm", "macos-15"}, (
                location,
                label,
            )
            labels.add(label)
    assert {"ubuntu-24.04", "macos-15"} <= labels, labels


def test_ci_linux_runner_fallbacks_agree():
    fallbacks = []
    for location, value in _workflow_scalars():
        if "CI_LINUX_RUNNER" not in value:
            continue
        matches = LINUX_FALLBACK.findall(value)
        assert len(matches) == value.count("CI_LINUX_RUNNER"), (
            location,
            "Every CI_LINUX_RUNNER reference must have an explicit fallback",
            value,
        )
        for _, fallback in matches:
            assert fallback == "ubuntu-24.04", (location, fallback)
            fallbacks.append(fallback)
        labels = RUNNER_LABEL.findall(value)
        assert labels and all(label == "ubuntu-24.04" for label in labels), (
            location,
            "Fork pull requests and the default must use the same Ubuntu image",
            value,
        )
    assert fallbacks, "No CI_LINUX_RUNNER fallbacks were checked"
