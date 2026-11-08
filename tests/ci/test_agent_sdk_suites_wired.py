"""Keep both SDK test trees and standalone agent crates reachable from CI."""

import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys

import yaml


ROOT = Path(__file__).resolve().parents[2]
PYTHON_TREES = ("tests/agent/sdk/python", "services/agent/sdk/python/tests")
STANDALONE_CRATES = (
    "tests/parity",
    "tests/qualify",
    "tests/boundary",
    "qualification/pay2",
    "tests/isolation",
    "tools/sdk-gen",
    "tools/policy-harness",
    "tools/doc-check",
)


def workflow():
    return yaml.load(
        (ROOT / ".github/workflows/agent.yml").read_text(), Loader=yaml.BaseLoader
    )


def make_recipe(target):
    source = (ROOT / "Makefile").read_text()
    match = re.search(rf"^{re.escape(target)}:\n((?:\t[^\n]*\n)+)", source, re.M)
    assert match, f"Missing runnable Make target: {target}"
    return match[1]


def required_step(job, command):
    assert "if" not in job, "SDK suites must run on every agent workflow event"
    assert job.get("continue-on-error", "false") == "false"
    steps = [step for step in job["steps"] if step.get("run") == command]
    assert len(steps) == 1, f"Missing required workflow command: {command}"
    assert "if" not in steps[0]
    assert steps[0].get("continue-on-error", "false") == "false"
    assert steps[0].get("working-directory", ".") == "."
    return steps[0]


def test_agent_sdk_python_collects_cleanly():
    environment = dict(os.environ)
    environment["PYTHONPATH"] = str(ROOT / "services/agent/sdk/python")
    result = subprocess.run(
        [
            sys.executable, "-m", "pytest", "-q", "--collect-only",
            "--rootdir", str(ROOT), *PYTHON_TREES,
        ],
        cwd=ROOT,
        env=environment,
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    for tree in PYTHON_TREES:
        assert re.search(rf"^{re.escape(tree)}/test_.*::", output, re.M), output
    assert "ERROR" not in output, output
    collected = {
        line for line in output.splitlines()
        if "::" in line and any(line.startswith(tree + "/test_") for tree in PYTHON_TREES)
    }
    baseline = json.loads((ROOT / "tests/ci/data/agent_sdk_python_baseline.json").read_text())
    assert isinstance(baseline, list)
    nodeids = set()
    for entry in baseline:
        assert set(entry) == {"nodeid", "first_error_line", "retiring_task"}
        assert entry["nodeid"] in collected, entry
        assert entry["nodeid"] not in nodeids, entry
        nodeids.add(entry["nodeid"])
        assert isinstance(entry["first_error_line"], str) and entry["first_error_line"].strip()
        assert "\n" not in entry["first_error_line"]
        assert entry["retiring_task"] in {"22.39.119", "22.39.120"}


def test_agent_workflow_runs_sdk_python():
    job = workflow()["jobs"]["sdk-python"]
    required_step(job, "make agent-test-sdk-py")
    required_step(job, "python3 -m pytest -q tests/ci/test_agent_sdk_suites_wired.py")
    recipe = make_recipe("agent-test-sdk-py")
    commands = [shlex.split(line.strip()) for line in recipe.splitlines()]
    invocation = next(command for command in commands if "pytest" in command)
    assert invocation == [
        "PYTHONPATH=services/agent/sdk/python",
        "python3", "-m", "pytest", "-q", *PYTHON_TREES,
    ]
    dependencies = "\n".join(step.get("run", "") for step in job["steps"])
    for dependency in ("pytest", "cryptography", "PyYAML"):
        assert dependency in dependencies
    foundry = next(
        step for step in job["steps"]
        if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    )
    pinned = next(
        step for step in workflow()["jobs"]["build-lint-test"]["steps"]
        if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    )
    assert foundry["uses"] == pinned["uses"]
    assert foundry["with"]["version"] == pinned["with"]["version"] == "v1.8.1"
    assert "if" not in foundry
    assert foundry.get("continue-on-error", "false") == "false"
    assert job["steps"].index(foundry) < job["steps"].index(required_step(job, "make agent-test-sdk-py"))


def test_agent_workflow_runs_sdk_typescript():
    job = workflow()["jobs"]["sdk-typescript"]
    required_step(job, "make agent-test-sdk-ts")
    assert "cd services/agent/sdk/typescript && npm test" in make_recipe("agent-test-sdk-ts")
    manifest = json.loads((ROOT / "services/agent/sdk/typescript/package.json").read_text())
    assert manifest["scripts"]["test"] == "npm run test:ci"
    assert manifest["scripts"]["test:ci"] == "npm run build && node scripts/run-tests.mjs ci"
    runner = (ROOT / "services/agent/sdk/typescript/scripts/run-tests.mjs").read_text()
    assert 'const discovered = discoverTests(sourceRoot)' in runner
    assert 'case "ci":' in runner
    assert 'for (const target of compiled)' in runner
    assert 'process.exitCode = failures === 0 ? 0 : 1' in runner
    assert any(step.get("uses", "").startswith("foundry-rs/foundry-toolchain@") for step in job["steps"])
    install = next(step for step in job["steps"] if step.get("run", "").startswith("npm ci "))
    assert install["working-directory"] == "services/agent/sdk/typescript"


def test_agent_workflow_runs_unwired_crates():
    job = workflow()["jobs"]["build-lint-test"]
    steps = [step for step in job["steps"] if step.get("name") == "Test standalone agent crates"]
    assert len(steps) == 1
    step = required_step(job, steps[0]["run"])
    script = step["run"]
    loop = re.search(r"for crate in ([^;]+); do\n\s*(.+)\n\s*done", script)
    assert loop, "Every standalone crate must run an unfiltered cargo test"
    assert set(shlex.split(loop[1])) == set(STANDALONE_CRATES)
    assert shlex.split(loop[2]) == [
        "sh", "tools/runtime/run-with-clock.sh", "cargo", "+1.91.1",
        "test", "--locked", "--manifest-path", "services/agent/$crate/Cargo.toml",
    ]
    for crate in STANDALONE_CRATES:
        assert (ROOT / "services/agent" / crate / "Cargo.toml").is_file()
    assert step["env"]["CARGO_TARGET_DIR"] == "${{ github.workspace }}/.lane-target"
