"""Keep retired node cases in the full public package gate with its bounded timeout."""

import json
import shlex
from pathlib import Path

import yaml


WORKFLOW = Path(__file__).resolve().parents[2] / ".github/workflows/node-build.yml"
BASELINE = Path(__file__).resolve().parent / "data/node_go_test_baseline.json"


def node_test_command():
    workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
    job = workflow["jobs"]["node-tests"]
    assert "if" not in job
    assert "continue-on-error" not in job
    (step,) = [step for step in job["steps"] if step.get("name") == "Test node packages"]
    assert "if" not in step
    assert "continue-on-error" not in step
    assert step.get("working-directory", ".") == "."
    command = shlex.split(step["run"])
    assert command[:2] == ["go", "test"]
    assert not {"-run", "||", ";", "&"}.intersection(command)
    baseline = json.loads(BASELINE.read_text())
    assert baseline["package"] == "./node/"
    assert baseline["tests"] == []
    assert "-skip" not in command
    assert command == [
        "go",
        "test",
        "-timeout",
        "15m",
        "-count=1",
        "./node/...",
    ]
    assert (
        "# Known failures and retiring task: tests/ci/data/node_go_test_baseline.json"
        in WORKFLOW.read_text()
    )
    return command


def test_node_build_runs_node_packages():
    command = node_test_command()
    assert "./node/..." in command or "./node/" in command


def test_node_build_runs_sequencer_package():
    command = node_test_command()
    assert "./node/..." in command or "./node/sequencer/" in command


def test_node_build_sets_go_test_timeout():
    command = node_test_command()
    assert command.count("-timeout") == 1
    assert command[command.index("-timeout") + 1] == "15m"
    assert "-count=1" in command
