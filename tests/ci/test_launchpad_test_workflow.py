"""Keep the launchpad test matrix aligned with the real package inventory."""

import json
from pathlib import Path
import re

import yaml

from test_pnpm_workspace_roots import (
    expand_matrix,
    matrix_rows,
    pnpm_commands,
    pnpm_directory,
    run_directory,
)


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/launchpad-test.yml"
WORKSPACE = ROOT / "apps/launchpad"


def launchpad_workflow():
    # BaseLoader preserves GitHub's `on` key instead of treating it as YAML 1.1 true.
    return yaml.load((ROOT / WORKFLOW).read_text(), Loader=yaml.BaseLoader)


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
    assert job["runs-on"] == (
        "${{ (github.event_name == 'pull_request' && "
        "github.event.pull_request.head.repo.full_name != github.repository) "
        "&& 'ubuntu-24.04' || vars.CI_LINUX_RUNNER || 'ubuntu-24.04' }}"
    )
    assert "if" not in job
    assert "continue-on-error" not in job
    assert job["defaults"]["run"]["working-directory"] == "apps/launchpad"
    assert workflow["permissions"] == {"contents": "read"}

    steps = job["steps"]
    for step in steps:
        assert "if" not in step
        assert "continue-on-error" not in step
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
    commands = [step for step in steps if "run" in step]
    manager, install, build, test = commands
    assert manager["run"] == (
        'npm install --global "$(node -p '
        "'require(\"./package.json\").packageManager')\""
    )
    assert install["run"] == "pnpm install --frozen-lockfile"
    assert build["run"] == "pnpm run build:packages"
    assert test["working-directory"] == "."
    for row in matrix_rows(job):
        directory = run_directory(workflow, job, test, row)
        assert directory == ROOT
        (command,) = list(pnpm_commands(expand_matrix(test["run"], row), directory))
        cwd, args = command
        assert cwd == ROOT
        assert args[:3] == ["--dir", "apps/launchpad", "--filter"]
        assert args[4:] == ["--fail-if-no-match", "run", "test"]
        assert pnpm_directory(cwd, args) == WORKSPACE
        selector = args[3]
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
