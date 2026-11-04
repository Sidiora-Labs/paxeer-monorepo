"""Prove the in-scope SDK workflow contract; root discovery needs owner wiring.

Parse production YAML and Cargo metadata without invoking Cargo or GitHub.
The root workflow placement is explicitly outside this task's scope.
"""

from fnmatch import fnmatchcase
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
SDK = "sdk/chainflow/rust"
WORKFLOW = f"{SDK}/.github/workflows/sdk-ci.yml"
ROOT_WORKFLOW = ".github/workflows/chainflow-rust-sdk.yml"
RECORD = Path(__file__).with_name("repair_22_33_1332.kvx")
COMMANDS = [
    ["cargo", "build", "--all-features"],
    ["cargo", "clippy", "--all-targets", "--all-features", "--", "-D", "warnings"],
    ["cargo", "test", "--all-features"],
    ["cargo", "fmt", "--check"],
]


def workflow():
    # BaseLoader preserves GitHub's YAML 1.2 event name `on` as a string;
    # PyYAML's YAML 1.1 implicit boolean resolver would turn it into True.
    return yaml.load((ROOT / WORKFLOW).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def record():
    return tomllib.loads(RECORD.read_text(encoding="utf-8"))


def test_sdk_push_and_pull_request_paths_reach_verification():
    config = workflow()
    assert set(config["on"]) == {"push", "pull_request"}
    for event in config["on"].values():
        assert set(event) == {"paths"}, "No branch, type or ignore filter may hide SDK changes"
        assert event["paths"] == [f"{SDK}/**", ROOT_WORKFLOW]
        for changed in (
            f"{SDK}/src/client.rs",
            f"{SDK}/tests/mock.rs",
            f"{SDK}/Cargo.toml",
            f"{SDK}/Cargo.lock",
            WORKFLOW,
            ROOT_WORKFLOW,
        ):
            assert any(fnmatchcase(changed, pattern) for pattern in event["paths"]), changed
        assert not any(fnmatchcase("docs/unrelated.md", pattern) for pattern in event["paths"])
    assert set(config["jobs"]) == {"verify"}
    job = config["jobs"]["verify"]
    assert job["runs-on"] == "ubuntu-latest"
    assert "if" not in job
    assert "needs" not in job
    assert "continue-on-error" not in job


def test_checkout_and_toolchain_feed_every_cargo_check_in_sdk_directory():
    config = workflow()
    assert config["permissions"] == {"contents": "read"}
    job = config["jobs"]["verify"]
    assert job["defaults"]["run"]["working-directory"] == SDK
    steps = job["steps"]
    assert len(steps) == 6
    assert steps[0] == {"uses": "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"}
    assert steps[1] == {
        "uses": "dtolnay/rust-toolchain@29eef336d9b2848a0b548edc03f92a220660cdb8",
        "with": {"toolchain": "stable", "components": "rustfmt, clippy"},
    }
    for step, expected in zip(steps[2:], COMMANDS, strict=True):
        assert set(step) == {"run"}, "Checks must run unconditionally and propagate failures"
        assert shlex.split(step["run"]) == expected
        # The default resolves from actions/checkout's monorepo root.
        directory = ROOT / job["defaults"]["run"]["working-directory"]
        manifest = tomllib.loads((directory / "Cargo.toml").read_text(encoding="utf-8"))
        assert manifest["package"]["name"] == "chainflow-data-gateway"


def test_all_features_keeps_required_mock_target_and_gated_sdk_surfaces():
    manifest = tomllib.loads((ROOT / SDK / "Cargo.toml").read_text(encoding="utf-8"))
    mock_targets = [target for target in manifest["test"] if target["name"] == "mock"]
    assert len(mock_targets) == 1
    assert mock_targets[0]["required-features"] == ["mock", "tokio"]
    assert set(mock_targets[0]["required-features"]) <= manifest["features"].keys()
    assert "websocket" in manifest["features"]
    assert (ROOT / SDK / "tests/mock.rs").is_file()
    commands = [shlex.split(step["run"]) for step in workflow()["jobs"]["verify"]["steps"] if "run" in step]
    assert commands == COMMANDS
    for command in commands[:3]:
        assert "--all-features" in command
    assert "--all-targets" in commands[1]
    assert commands[2] == ["cargo", "test", "--all-features"], "Keep doctests and integration tests enabled"


def test_sdk_ci_finding_maps_to_proofs_and_explicit_root_wiring_request():
    data = record()
    task = data["task"]
    assert task["id"] == "22.33.1332"
    finding = data["finding"]["sdk-ci-not-discovered"]
    assert finding["file"] == WORKFLOW
    assert finding["audit"] == "22.30.72"
    assert finding["disposition"] == "in-scope-repaired; root-discovery-pending"
    assert set(finding["tests"]) == {
        "test_sdk_push_and_pull_request_paths_reach_verification",
        "test_checkout_and_toolchain_feed_every_cargo_check_in_sdk_directory",
        "test_all_features_keeps_required_mock_target_and_gated_sdk_surfaces",
        "test_sdk_ci_finding_maps_to_proofs_and_explicit_root_wiring_request",
    }
    assert len(task["wiring_requests"]) == 1
    request = task["wiring_requests"][0]
    for required in (ROOT_WORKFLOW, "domain 15", WORKFLOW, SDK, "push", "pull_request"):
        assert required in request
    assert "not executed" in task["baseline"]
    assert "GitHub discovery" in task["review_only"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert re.fullmatch(r"[0-9a-f]{40}", task["inspected_revision"])
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1332.py"
