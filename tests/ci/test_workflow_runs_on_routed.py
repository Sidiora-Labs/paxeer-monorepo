from pathlib import Path
import re
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
CHECK = ROOT / "scripts/ci/runner-routing-check.sh"
WORKFLOWS = ("human.yml", "platform.yml")
ROUTED = (
    "${{ (github.event_name == 'pull_request' && "
    "github.event.pull_request.head.repo.full_name != github.repository) && "
    "'ubuntu-24.04' || vars.CI_LINUX_RUNNER || 'ubuntu-24.04' }}"
)
RESTORED_JOBS = (
    ("human.yml", "build-lint-test"),
    ("human.yml", "intent-fuzz"),
    ("platform.yml", "android-application-artifact"),
    ("platform.yml", "programs-acceptance"),
    ("platform.yml", "agent-sanitizers"),
    ("platform.yml", "agent-fuzz-corpus"),
    ("platform.yml", "release-promotion"),
    ("ci.yml", "js-suite-inventory"),
    ("ci.yml", "js-node-tests"),
    ("ci.yml", "js-npm-tests"),
    ("ci.yml", "js-workspace-tests"),
    ("ci.yml", "js-pnpm-tests"),
    ("ci.yml", "js-wallet-gateway-tests"),
    ("ci.yml", "js-explorer-env-tests"),
    ("explorer-images.yml", "explorer-backend-test"),
    ("launchpad-test.yml", "launchpad-test"),
    ("paxeer-go-test.yml", "tagged-tests"),
    ("paxeer-go-test.yml", "hpx-registry"),
)
HOSTED_JOBS = (
    ("ci.yml", "c17", "sanitizer", "ubuntu-24.04"),
    ("paxeer-go-test.yml", "autobahn-integration", "docker-command", "ubuntu-24.04"),
)
HOSTED_RUNNER_LABELS = {
    runner for _, _, _, runner in HOSTED_JOBS
} | {"ubuntu-24.04-arm", "macos-15", "windows-2022"}


def assert_runner_routing():
    result = subprocess.run(
        ["bash", "scripts/ci/runner-routing-check.sh"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def routing_exclusions():
    table = re.search(
        r'^EXCLUDED = """\n(.*?)\n"""',
        CHECK.read_text(encoding="utf-8"),
        re.MULTILINE | re.DOTALL,
    )
    assert table is not None, "Routing script exclusion table was not found"
    return {
        (name, job): (reason, runner)
        for name, job, reason, runner in (
            line.split(None, 3) for line in table.group(1).splitlines()
        )
    }


def assert_workflow_runs_on_routed(name, workflow):
    excluded = routing_exclusions()
    source = (ROOT / ".github/workflows" / name).read_text(encoding="utf-8")
    checked = set()
    for job_name, job in workflow["jobs"].items():
        if "runs-on" not in job or (name, job_name) in excluded:
            continue
        assert job["runs-on"] == ROUTED, (
            f"{name}:{job_name}: expected routed runs-on, got {job['runs-on']!r}"
        )
        block = re.search(
            rf"^  {re.escape(job_name)}:\n(.*?)(?=^  \S|\Z)",
            source,
            re.MULTILINE | re.DOTALL,
        )
        assert block is not None, f"{name}:{job_name}: job source was not found"
        assert f"    runs-on: {ROUTED}" in block.group(1).splitlines(), (
            f"{name}:{job_name}: runs-on must use the exact routed line"
        )
        checked.add(job_name)
    assert checked, f"{name}: no routed jobs were inspected"


def load_workflow(name):
    return yaml.load(
        (ROOT / ".github/workflows" / name).read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )


def assert_restored_job_runs_on_routed(name, job_name, job, callers=()):
    identity = (name, job_name)
    assert identity not in callers, f"{name}:{job_name}: recursive workflow call"
    excluded = routing_exclusions()
    if "uses" in job:
        assert "runs-on" not in job, f"{name}:{job_name}: caller declares a runner"
        assert identity not in excluded, f"{name}:{job_name}: caller is excluded"
        workflow_path = ROOT / job["uses"]
        assert job["uses"].startswith("./.github/workflows/"), (
            f"{name}:{job_name}: expected a local reusable workflow"
        )
        assert workflow_path.resolve().parent == ROOT / ".github/workflows", (
            f"{name}:{job_name}: called workflow must be under .github/workflows"
        )
        called_name = workflow_path.name
        called_jobs = load_workflow(called_name)["jobs"]
        assert called_jobs, f"{called_name}: called workflow has no jobs"
        for called_job_name, called_job in called_jobs.items():
            assert_restored_job_runs_on_routed(
                called_name, called_job_name, called_job, (*callers, identity)
            )
    elif job["runs-on"] == ROUTED:
        assert identity not in excluded, (
            f"{name}:{job_name}: restored job must not be excluded from routing"
        )
    else:
        runner = job["runs-on"]
        reference = re.fullmatch(r"\$\{\{ matrix\.([A-Za-z0-9_-]+) \}\}", runner)
        assert reference is not None, (
            f"{name}:{job_name}: restored job must keep the routed runner"
        )
        axis = reference.group(1)
        matrix = job["strategy"]["matrix"]
        runners = list(matrix.get(axis, []))
        for entry in matrix.get("include", []):
            assert axis in entry or axis in matrix, (
                f"{name}:{job_name}: matrix entry has no runner value"
            )
            if axis in entry:
                runners.append(entry[axis])
        assert runners, f"{name}:{job_name}: runner matrix must not be empty"
        assert all(
            value == ROUTED or value in HOSTED_RUNNER_LABELS for value in runners
        ), (
            f"{name}:{job_name}: matrix contains an unsupported runner: {runners!r}"
        )
        if any(value != ROUTED for value in runners):
            assert excluded[identity] == ("matrix-runner", runner)
        else:
            assert identity not in excluded


def test_repository_runner_routing_check_passes():
    assert_runner_routing()


@pytest.mark.parametrize("name", WORKFLOWS)
def test_every_non_excluded_job_keeps_routed_runs_on(name):
    assert_workflow_runs_on_routed(name, load_workflow(name))


@pytest.mark.parametrize(("name", "job_name"), RESTORED_JOBS)
def test_restored_job_keeps_routed_runs_on(name, job_name):
    jobs = load_workflow(name)["jobs"]
    assert job_name in jobs, f"{name}:{job_name}: restored job disappeared"
    if "uses" in jobs[job_name]:
        assert_restored_job_runs_on_routed(name, job_name, jobs[job_name])
    else:
        assert jobs[job_name]["runs-on"] == ROUTED, (
            f"{name}:{job_name}: restored job must keep the routed runner"
        )
    assert (name, job_name) not in routing_exclusions(), (
        f"{name}:{job_name}: restored job must not be excluded from routing"
    )
    assert_workflow_runs_on_routed(name, load_workflow(name))


@pytest.mark.parametrize(("name", "job_name", "reason", "runner"), HOSTED_JOBS)
def test_hosted_job_keeps_runner_and_exclusion_reason(name, job_name, reason, runner):
    jobs = load_workflow(name)["jobs"]
    assert job_name in jobs, f"{name}:{job_name}: hosted job disappeared"
    assert jobs[job_name]["runs-on"] == runner, (
        f"{name}:{job_name}: {reason} job must retain its hosted runner"
    )
    assert routing_exclusions()[(name, job_name)] == (reason, runner)


def test_explorer_backend_routed_job_retains_container_builder():
    job = load_workflow("explorer-images.yml")["jobs"]["explorer-backend-test"]
    assert job["runs-on"] == ROUTED, "Explorer backend must keep the routed runner"
    assert ("explorer-images.yml", "explorer-backend-test") not in routing_exclusions(), (
        "Explorer backend must not be excluded from routing"
    )
    assert any(
        step.get("run") == "services/explorer/deploy/tools/mix-in-builder.sh test"
        for step in job["steps"]
    ), "Explorer backend's routed job must retain its container builder"


def test_autobahn_hosted_runner_preserves_cluster_entrypoint():
    workflow = load_workflow("paxeer-go-test.yml")
    job = workflow["jobs"]["autobahn-integration"]
    test_hosted_job_keeps_runner_and_exclusion_reason(
        "paxeer-go-test.yml", "autobahn-integration", "docker-command", "ubuntu-24.04"
    )
    assert "pull_request" in workflow["on"]
    assert "if" not in job
    assert "continue-on-error" not in job
    assert job["timeout-minutes"] == "60"
    assert workflow["env"]["GO_TEST_TIMEOUT"] == "30m"
    assert job["env"] == {
        "DOCKER_PLATFORM": "linux/amd64",
        "GIGA_EXECUTOR": "true",
        "GIGA_STORAGE": "true",
        "GIGA_OCC": "true",
    }
    steps = job["steps"]
    assert len(steps) == 3
    assert all("if" not in step and "continue-on-error" not in step for step in steps)
    assert steps[0]["uses"].startswith("actions/checkout@")
    assert steps[1]["uses"].startswith("actions/setup-go@")
    assert steps[1]["with"] == {
        "go-version": "${{ env.GO_VERSION }}",
        "cache": "false",
    }
    assert steps[2] == {
        "name": "Autobahn integration tests",
        "run": (
            "make -f chain.mk autobahn-integration-test "
            "AUTOBAHN_TEST_TIMEOUT=${{ env.GO_TEST_TIMEOUT }}"
        ),
    }


def test_autobahn_hosted_runner_preserves_multinode_entrypoint():
    test_autobahn_hosted_runner_preserves_cluster_entrypoint()
