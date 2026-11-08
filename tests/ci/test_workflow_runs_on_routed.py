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
    ("launchpad-test.yml", "launchpad-test"),
    ("paxeer-go-test.yml", "tagged-tests"),
    ("paxeer-go-test.yml", "autobahn-integration"),
    ("paxeer-go-test.yml", "hpx-registry"),
)
HOSTED_JOBS = (
    ("ci.yml", "c17", "sanitizer", "ubuntu-24.04"),
    ("explorer-images.yml", "explorer-backend-test", "docker-command", "ubuntu-24.04"),
)


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


def test_repository_runner_routing_check_passes():
    assert_runner_routing()


@pytest.mark.parametrize("name", WORKFLOWS)
def test_every_non_excluded_job_keeps_routed_runs_on(name):
    assert_workflow_runs_on_routed(name, load_workflow(name))


@pytest.mark.parametrize(("name", "job_name"), RESTORED_JOBS)
def test_restored_job_keeps_routed_runs_on(name, job_name):
    jobs = load_workflow(name)["jobs"]
    assert job_name in jobs, f"{name}:{job_name}: restored job disappeared"
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


def test_explorer_backend_exclusion_retains_container_builder():
    job = load_workflow("explorer-images.yml")["jobs"]["explorer-backend-test"]
    assert any(
        step.get("run") == "services/explorer/deploy/tools/mix-in-builder.sh test"
        for step in job["steps"]
    ), "Explorer backend's docker-command exclusion must retain its container builder"
