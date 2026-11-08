"""Keep the public node gate's package coverage and timeout explicit."""

import json
import shlex
from pathlib import Path

import yaml


WORKFLOW = Path(__file__).resolve().parents[2] / ".github/workflows/node-build.yml"
BASELINE = Path(__file__).resolve().parent / "data/node_go_test_baseline.json"
BASELINE_TESTS = {
    "TestMergePlanReturnsGuarantorBonds": "22.39.116",
    "TestHostedKernelProgramsABIFollowsTheMergeUpgrade": "22.39.116",
    "TestHostedKernelProgramsABISurvivesRestart": "22.39.116",
    "TestEVMServerServesSequencerReceiptOption": "22.39.155",
    "TestStandbyReplicatesRecorderGroupsThroughOpenAccount": "22.39.155",
    "TestV640ToV66UpgradeServesTheExchangeBridgeAndLaunchpadPrecompiles": "22.39.155",
}


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
    assert [entry["name"] for entry in baseline["tests"]] == list(BASELINE_TESTS)
    for entry in baseline["tests"]:
        assert entry["retiring_task"] == BASELINE_TESTS[entry["name"]]
        assert entry["reason"].strip()
    assert command.count("-skip") == 1
    assert command[command.index("-skip") + 1] == "|".join(BASELINE_TESTS)
    assert command == [
        "go",
        "test",
        "-timeout",
        "15m",
        "-count=1",
        "-skip",
        "|".join(BASELINE_TESTS),
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
