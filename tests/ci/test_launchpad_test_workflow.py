"""Keep the launchpad test matrix aligned with the real package inventory."""

import json
from pathlib import Path
import re
import shlex

import pytest
import yaml

from test_pnpm_workspace_roots import (
    expand_matrix,
    matrix_rows,
    pnpm_commands,
    pnpm_directory,
    run_directory,
)
from test_workflow_runs_on_routed import ROUTED
from test_launchpad_package_job_environment import package_condition_matches


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/launchpad-test.yml"
WORKSPACE = ROOT / "apps/launchpad"


def launchpad_workflow():
    # BaseLoader preserves GitHub's `on` key instead of treating it as YAML 1.1 true.
    return yaml.load((ROOT / WORKFLOW).read_text(), Loader=yaml.BaseLoader)


@pytest.mark.parametrize(
    ("workflow_name", "job_name", "expected_runner"),
    (
        ("paxeer-go-test.yml", "tagged-tests", ROUTED),
        ("paxeer-go-test.yml", "autobahn-integration", "ubuntu-24.04"),
        ("paxeer-go-test.yml", "hpx-registry", ROUTED),
        ("launchpad-test.yml", "launchpad-test", ROUTED),
    ),
)
def test_workflow_runner_contracts_match_routed_jobs(
    workflow_name, job_name, expected_runner
):
    workflow = yaml.load(
        (ROOT / ".github/workflows" / workflow_name).read_text(),
        Loader=yaml.BaseLoader,
    )
    assert workflow["jobs"][job_name]["runs-on"] == expected_runner, (
        f"{workflow_name}:{job_name}: expected the current runner expression"
    )


def test_launchpad_workflow_covers_every_package_with_a_test_script():
    manifests = list((ROOT / "sdk/launchpad/packages").rglob("package.json"))
    manifests.extend(
        ROOT / "apps/launchpad" / app / "package.json"
        for app in ("web", "mobile")
    )
    packages = {}
    for manifest in manifests:
        if {"node_modules", "dist"}.intersection(manifest.relative_to(ROOT).parts):
            continue
        package = json.loads(manifest.read_text())
        if "test" in package.get("scripts", {}):
            assert package["scripts"]["test"].strip(), manifest
            packages[manifest.parent.relative_to(ROOT).as_posix()] = package
    assert packages, "The launchpad test inventory must not be empty"
    assert {"apps/launchpad/web", "apps/launchpad/mobile"} <= packages.keys()

    workflow = launchpad_workflow()
    job = workflow["jobs"]["launchpad-test"]
    matrix = job["strategy"]["matrix"]["package"]
    assert set(matrix) == packages.keys(), {
        "missing": sorted(packages.keys() - set(matrix)),
        "unexpected": sorted(set(matrix) - packages.keys()),
    }
    assert len(matrix) == len(set(matrix)), "Each package must run exactly once"
    assert "exclude" not in job["strategy"]["matrix"]
    assert job["strategy"]["fail-fast"] == "false"
    assert job["runs-on"] == ROUTED
    assert "if" not in job
    assert "continue-on-error" not in job
    assert job["defaults"]["run"]["working-directory"] == "apps/launchpad"
    assert workflow["permissions"] == {"contents": "read"}

    steps = job["steps"]
    conditional_packages = {
        "Set up Go for launchpad services": {
            "apps/launchpad/web",
            *(f"sdk/launchpad/packages/{name}" for name in (
                "candles-sdk", "indexer-sdk", "metadata-sdk", "ranking-algo-sdk",
                "sdk", "stats-sdk", "users-sdk",
            )),
        },
        "Create the pool test database": {
            f"sdk/launchpad/packages/{name}" for name in (
                "indexer-sdk", "metadata-sdk", "sdk", "stats-sdk", "users-sdk",
            )
        },
        "Start object storage fixture": {
            "sdk/launchpad/packages/metadata-sdk", "sdk/launchpad/packages/users-sdk",
        },
        "Install the users scanner": {"sdk/launchpad/packages/users-sdk"},
        "Install PostgreSQL for the candles producer": {
            "sdk/launchpad/packages/candles-sdk",
        },
    }
    for step in steps:
        assert "continue-on-error" not in step
        if "if" in step:
            if step["name"] in conditional_packages:
                assert {
                    package for package in matrix
                    if package_condition_matches(step, package)
                } == conditional_packages[step["name"]]
            else:
                assert step["if"] in {
                    "matrix.package == 'apps/launchpad/web'",
                    "matrix.package != 'apps/launchpad/web'",
                }
        if "uses" in step:
            assert re.fullmatch(r"[\w-]+/[\w-]+@[0-9a-f]{40}", step["uses"])
    (checkout,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/checkout@")
    ]
    (setup,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/setup-node@")
    ]
    assert checkout["with"]["persist-credentials"] == "false"
    assert setup["with"]["node-version"] == "22"
    assert setup["with"]["package-manager-cache"] == "false"
    commands = {step["name"]: step for step in steps if "run" in step}
    manager = commands["Install the workspace package manager"]
    install = commands["Install workspace dependencies"]
    build = commands["Build SDK packages"]
    test = commands["Test package"]
    assert manager["run"] == (
        'npm install --global "$(node -p '
        "'require(\"./package.json\").packageManager')\""
    )
    assert install["run"] == "pnpm install --frozen-lockfile"
    assert build["run"] == "pnpm run build:packages"
    assert test["working-directory"] == "apps/launchpad"
    for row in matrix_rows(job):
        directory = run_directory(workflow, job, test, row)
        assert directory == WORKSPACE
        (command,) = list(pnpm_commands(expand_matrix(test["run"], row), directory))
        cwd, args = command
        assert cwd == WORKSPACE
        assert args[:1] == ["--filter"]
        assert args[2:] == ["--fail-if-no-match", "run", "test"]
        assert pnpm_directory(cwd, args) == WORKSPACE
        selector = args[1]
        assert selector.startswith("./"), row
        assert (WORKSPACE / selector).resolve() == (
            ROOT / row["package"]
        ).resolve(), row
    assert steps.index(checkout) < steps.index(setup) < steps.index(manager)
    assert (
        steps.index(manager) < steps.index(install)
        < steps.index(build) < steps.index(test)
    )

    workspace_package = json.loads((WORKSPACE / "package.json").read_text())
    manager_version = workspace_package["packageManager"]
    assert re.fullmatch(r"pnpm@\d+\.\d+\.\d+", manager_version)
    assert workspace_package["scripts"]["build:packages"] == (
        "pnpm -r --filter '../../sdk/launchpad/packages/*' build"
    )
    workspace_globs = yaml.safe_load(
        (WORKSPACE / "pnpm-workspace.yaml").read_text()
    )["packages"]
    workspace_members = {
        path.resolve() for pattern in workspace_globs for path in WORKSPACE.glob(pattern)
    }
    lock = yaml.safe_load((WORKSPACE / "pnpm-lock.yaml").read_text())
    locked_members = {
        (WORKSPACE / importer).resolve() for importer in lock["importers"]
    }
    for directory, package in packages.items():
        assert (ROOT / directory).resolve() in workspace_members, directory
        assert (ROOT / directory).resolve() in locked_members, directory
        if "packageManager" in package:
            assert package["packageManager"] == manager_version, directory


def test_launchpad_service_health_commands_use_container_option_quoting():
    workflow = launchpad_workflow()
    health_commands = {
        "postgres": "pg_isready -U kindle_fixture -d kindle_fixture",
        "redis": "redis-cli ping",
    }
    checked = set()
    for job_name, job in workflow["jobs"].items():
        for service_name, service in job.get("services", {}).items():
            options = service["options"]
            assert "'" not in options, (job_name, service_name, options)
            assert shlex.split(options) == [
                "--health-cmd", health_commands[service_name],
                "--health-interval", "10s",
                "--health-timeout", "5s", "--health-retries", "5",
            ], (job_name, service_name)
            checked.add((job_name, service_name))
    assert checked == {
        (job_name, service_name)
        for job_name in ("launchpad-test", "launchpad-go-test", "launchpad-go-ci")
        for service_name in health_commands
    }


def test_launchpad_web_fixtures_use_the_go_jobs_version():
    workflow = launchpad_workflow()
    job = workflow["jobs"]["launchpad-test"]
    steps = job["steps"]
    (setup,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/setup-go@")
    ]
    assert setup["name"] == "Set up Go for launchpad services"
    expected_packages = {
        "apps/launchpad/web",
        *(f"sdk/launchpad/packages/{name}" for name in (
            "candles-sdk", "indexer-sdk", "metadata-sdk", "ranking-algo-sdk",
            "sdk", "stats-sdk", "users-sdk",
        )),
    }
    for package in job["strategy"]["matrix"]["package"]:
        assert package_condition_matches(setup, package) == (package in expected_packages)
    assert setup["with"] == {"go-version": "1.27.3", "cache": "false"}
    assert job["env"]["GOTOOLCHAIN"] == "go" + setup["with"]["go-version"]
    for name in ("launchpad-go-test", "launchpad-go-ci", "launchpad-go-wiring"):
        go_job = workflow["jobs"][name]
        (go_setup,) = [
            step for step in go_job["steps"]
            if step.get("uses", "").startswith("actions/setup-go@")
        ]
        assert setup["uses"] == go_setup["uses"]
        assert setup["with"]["go-version"] == go_setup["with"]["go-version"]
        assert job["env"]["GOTOOLCHAIN"] == go_job["env"]["GOTOOLCHAIN"]
    for name in (
        "Install the joined event broker", "Prepare chain fixtures and run web Vitest",
    ):
        (consumer,) = [step for step in steps if step.get("name") == name]
        assert steps.index(setup) < steps.index(consumer)
        assert consumer["if"] == "matrix.package == 'apps/launchpad/web'"
        assert "GOTOOLCHAIN" not in consumer.get("env", {})


def test_launchpad_workflow_triggers_on_launchpad_paths():
    triggers = launchpad_workflow()["on"]
    assert {"pull_request", "push"} <= triggers.keys()
    assert triggers["push"]["branches"] == ["main"]
    required = {"sdk/launchpad/**", "apps/launchpad/**", WORKFLOW}
    for event in ("pull_request", "push"):
        assert required <= set(triggers[event]["paths"]), event
        assert "tests/ci/test_launchpad_test_workflow.py" in triggers[event]["paths"]
        assert "paths-ignore" not in triggers[event]
        assert "branches-ignore" not in triggers[event]
    assert "branches" not in triggers["pull_request"]
    assert "types" not in triggers["pull_request"]


def test_launchpad_go_jobs_share_every_ci_preparation_step():
    jobs = launchpad_workflow()["jobs"]
    module_job = jobs["launchpad-go-test"]
    ci_job = jobs["launchpad-go-ci"]
    assert module_job["env"] == ci_job["env"]
    assert module_job["services"] == ci_job["services"]
    assert module_job["steps"][-1]["name"] == "Test Go module"
    assert ci_job["steps"][-1]["run"] == "make -C services/launchpad ci"
    preparation = module_job["steps"][:-1]
    previous = -1
    for step in ci_job["steps"][:-1]:
        assert step in preparation, step["name"]
        position = preparation.index(step)
        assert position > previous, step["name"]
        assert step is preparation[position], step["name"]
        previous = position
    for job in (module_job, ci_job):
        assert "continue-on-error" not in job
        for step in job["steps"]:
            assert "continue-on-error" not in step


@pytest.mark.parametrize("job_name", ("launchpad-go-test", "launchpad-go-ci"))
def test_launchpad_object_storage_publishes_its_binary_directory(job_name):
    steps = launchpad_workflow()["jobs"][job_name]["steps"]
    (storage,) = [
        step for step in steps if step["name"] == "Start object storage fixture"
    ]
    command = storage["run"]
    install = (
        'GOBIN="$RUNNER_TEMP/launchpad-fixture-bin" GOFLAGS= '
        'go install github.com/minio/minio@RELEASE.2025-04-22T22-12-26Z'
    )
    publish = 'echo "$RUNNER_TEMP/launchpad-fixture-bin" >> "$GITHUB_PATH"'
    assert install in command.splitlines()
    assert publish in command.splitlines()
    assert command.index(install) < command.index(publish)
    assert steps.index(storage) < len(steps) - 1


@pytest.mark.parametrize("job_name", ("launchpad-go-test", "launchpad-go-ci"))
def test_launchpad_go_jobs_enable_commit_timestamps_after_database_creation(job_name):
    steps = launchpad_workflow()["jobs"][job_name]["steps"]
    (database,) = [
        step for step in steps if step["name"] == "Create the pool test database"
    ]
    timestamps = steps[steps.index(database) + 1]
    assert timestamps["name"] == "Enable database commit timestamps"
    assert timestamps["env"] == database["env"]
    command = timestamps["run"]
    configure = "ALTER SYSTEM SET track_commit_timestamp = on"
    restart = 'docker restart "${{ job.services.postgres.id }}"'
    ready = "pg_isready --host 127.0.0.1 --port 25432 --username kindle_fixture"
    check = "SHOW track_commit_timestamp"
    assert command.index(configure) < command.index(restart)
    assert command.index(restart) < command.index(ready) < command.index(check)
    assert (
        "--tuples-only --no-align --command 'SHOW track_commit_timestamp')\" = on"
        in command
    )
    assert command.rstrip().endswith("exit 1")


def test_launchpad_module_job_has_at_least_sixty_minutes():
    jobs = launchpad_workflow()["jobs"]
    assert set(jobs) == {
        "launchpad-test", "launchpad-go-test", "launchpad-go-ci", "launchpad-go-wiring",
    }
    assert int(jobs["launchpad-go-test"]["timeout-minutes"]) >= 60
    assert jobs["launchpad-test"]["timeout-minutes"] == "30"
    assert jobs["launchpad-go-ci"]["timeout-minutes"] == "360"
    assert "timeout-minutes" not in jobs["launchpad-go-wiring"]
