from pathlib import Path
import re
import shlex

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
WORKFLOWS = ("ci.yml", "wallet-test.yml", "explorer-images.yml")


def workflow(name):
    return yaml.load(
        (ROOT / ".github/workflows" / name).read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )


def required_run(job, command):
    assert "if" not in job
    assert job.get("continue-on-error", "false") == "false"
    steps = [
        step
        for step in job["steps"]
        if shlex.split(step.get("run", "")) == command
    ]
    assert len(steps) == 1
    step = steps[0]
    assert "if" not in step
    assert step.get("continue-on-error", "false") == "false"
    assert step.get("working-directory", ".") == "."
    return step


@pytest.mark.parametrize("name", WORKFLOWS)
def test_workflows_parse_as_yaml_and_use_pinned_actions(name):
    document = workflow(name)
    assert isinstance(document, dict)
    assert document["name"]
    assert document["permissions"] == {"contents": "read"}
    assert document["jobs"]
    for job in document["jobs"].values():
        assert job["runs-on"]
        timeouts = (
            [entry["timeout"] for entry in job["strategy"]["matrix"]["include"]]
            if job["timeout-minutes"] == "${{ matrix.timeout }}"
            else [job["timeout-minutes"]]
        )
        assert timeouts
        assert all(int(timeout) > 0 for timeout in timeouts)
        assert job["steps"]
        checkout = [
            step
            for step in job["steps"]
            if step.get("uses", "").startswith("actions/checkout@")
        ]
        assert len(checkout) == 1
        assert checkout[0]["with"]["persist-credentials"] == "false"
        for step in job["steps"]:
            if "uses" in step:
                assert re.fullmatch(r"[\w-]+/[\w-]+@[0-9a-f]{40}", step["uses"])


@pytest.mark.parametrize("name", WORKFLOWS)
def test_workflows_run_on_pull_requests_and_pushes_to_main(name):
    events = workflow(name)["on"]
    assert "pull_request" in events
    assert events["push"]["branches"] == ["main"]
    if name == "explorer-images.yml":
        for event in ("pull_request", "push"):
            assert "services/explorer/**" in events[event]["paths"]
            assert "docker/explorer-*/**" in events[event]["paths"]
            assert "precompiles/**" in events[event]["paths"]
            assert ".github/workflows/explorer-images.yml" in events[event]["paths"]


@pytest.mark.parametrize("target", ["test-tests-python", "programs-reference-fixtures"])
def test_ci_runs_both_make_targets_in_separate_jobs(target):
    jobs = workflow("ci.yml")["jobs"]
    owners = {
        name
        for name, job in jobs.items()
        if any(
            shlex.split(step.get("run", "")) == ["make", target]
            for step in job["steps"]
        )
    }
    assert len(owners) == 1
    job = jobs[owners.pop()]
    required_run(job, ["make", target])
    other = (
        "programs-reference-fixtures"
        if target == "test-tests-python"
        else "test-tests-python"
    )
    assert all(
        shlex.split(step.get("run", "")) != ["make", other]
        for step in job["steps"]
    )
    commands = [step.get("run", "") for step in job["steps"]]
    if target == "test-tests-python":
        assert any("pip install pytest cryptography PyYAML" in run for run in commands)
    else:
        assert (
            "rustup toolchain install 1.91.1 --profile minimal --target wasm32-unknown-unknown"
            in commands
        )


def test_wallet_runs_unfiltered_vitest_in_both_directories():
    job = workflow("wallet-test.yml")["jobs"]["vitest"]
    assert "if" not in job
    assert job.get("continue-on-error", "false") == "false"
    assert job["strategy"]["fail-fast"] == "false"
    assert job["strategy"]["matrix"]["directory"] == ["apps/wallet", "sdk/wallet"]
    steps = job["steps"]
    test = steps[-1]
    assert test["working-directory"] == "${{ matrix.directory }}"
    assert shlex.split(test["run"]) == ["pnpm", "exec", "vitest", "run"]
    assert "if" not in test
    assert test.get("continue-on-error", "false") == "false"
    commands = [step.get("run", "") for step in steps]
    agent_install = commands.index("npm ci --workspaces=false --no-audit --no-fund")
    agent_build = commands.index("npm run build --workspaces=false")
    sdk_install = commands.index("pnpm install --ignore-workspace --frozen-lockfile")
    app_install = commands.index("pnpm install --frozen-lockfile")
    assert agent_install < agent_build < sdk_install < app_install < len(steps) - 1
    assert steps[agent_install]["working-directory"] == "services/agent/sdk/typescript"
    assert steps[agent_build]["working-directory"] == "services/agent/sdk/typescript"
    assert steps[sdk_install]["working-directory"] == "sdk/wallet"
    assert steps[app_install]["working-directory"] == "apps/wallet"
    assert steps[app_install]["if"] == "matrix.directory == 'apps/wallet'"
    assert "npm install --global pnpm@10.34.9" in commands
    node = next(
        step
        for step in steps
        if step.get("uses", "").startswith("actions/setup-node@")
    )
    assert node["with"]["node-version"] == "22"


def test_explorer_runs_mix_test_through_the_builder_without_publishing_on_prs():
    jobs = workflow("explorer-images.yml")["jobs"]
    required_run(
        jobs["explorer-backend-test"],
        ["services/explorer/deploy/tools/mix-in-builder.sh", "test"],
    )
    assert jobs["publish"]["if"] == "github.event_name != 'pull_request'"
