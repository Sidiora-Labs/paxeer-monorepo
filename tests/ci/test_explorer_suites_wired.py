"""Keep the complete explorer Rust and frontend suites on the build workflow."""

import json
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKSPACES = {"libs", "smart-contract-verifier", "sig-provider"}


def explorer_workflow():
    return yaml.load(
        (ROOT / ".github/workflows/explorer-build.yml").read_text(),
        Loader=yaml.BaseLoader,
    )


def required_job(workflow, name):
    job = workflow["jobs"][name]
    assert "if" not in job
    assert "needs" not in job
    assert "continue-on-error" not in job
    for step in job["steps"]:
        assert "continue-on-error" not in step
    checkout = next(
        step
        for step in job["steps"]
        if step.get("uses", "").startswith("actions/checkout@")
    )
    assert checkout["with"]["persist-credentials"] == "false"
    return job


def test_explorer_build_runs_cargo_test():
    workflow = explorer_workflow()
    job = required_job(workflow, "explorer-services")
    assert job["strategy"]["fail-fast"] == "false"
    assert set(job["strategy"]["matrix"]["service"]) == WORKSPACES
    steps = job["steps"]
    run = next(step for step in steps if step["name"] == "Test service workspace")
    assert "if" not in run
    assert run["working-directory"] == "services/explorer/services/${{ matrix.service }}"
    assert shlex.split(run["run"]) == ["cargo", "test", "--locked", "--workspace"]
    assert not any("cargo check" in step.get("run", "") for step in steps)
    assert any(step.get("uses", "").startswith("dtolnay/rust-toolchain@") for step in steps)
    dependencies = next(
        step["run"] for step in steps if step["name"] == "Install build dependencies"
    )
    assert {"protobuf-compiler", "pkg-config", "libssl-dev"} <= set(shlex.split(dependencies))
    assert any("protoc-gen-openapiv2@v" in step.get("run", "") for step in steps)
    assert job["env"]["DATABASE_URL"] == "postgres://postgres:postgres@localhost:5432"
    postgres = job["services"]["postgres"]
    assert postgres["image"] == "postgres:15"
    assert postgres["env"]["POSTGRES_PASSWORD"] == "postgres"
    assert "5432:5432" in postgres["ports"]
    assert "--health-cmd pg_isready" in postgres["options"]
    for workspace in WORKSPACES:
        directory = ROOT / "services/explorer/services" / workspace
        manifest = tomllib.loads((directory / "Cargo.toml").read_text())
        assert manifest["workspace"]["members"], workspace
        assert "exclude" not in manifest["workspace"], workspace
        assert manifest["profile"]["test"]["debug"] == 0, workspace
        assert (directory / "Cargo.lock").is_file(), workspace
    gate = (ROOT / "scripts/explorer/gate-test.sh").read_text()
    loop = re.search(
        r'for service in ([^;]+); do\s+'
        r'run_leg "\$service" "\$service workspace tests" '
        r'"\$repo_root/services/explorer/services/\$service" \\\n'
        r'\s+cargo test --locked --workspace\s+done',
        gate,
    )
    assert loop, "The local gate must execute the same complete Rust workspace suites"
    assert set(shlex.split(loop[1])) == WORKSPACES


def test_explorer_build_runs_frontend_vitest():
    workflow = explorer_workflow()
    paths = workflow["on"]["pull_request"]["paths"]
    assert {
        "apps/explorer/**",
        "services/explorer/**",
        "scripts/explorer/**",
        ".github/workflows/explorer-build.yml",
        "tests/ci/test_explorer_suites_wired.py",
    } <= set(paths)
    job = required_job(workflow, "explorer-frontend-test")
    assert job["env"]["HUSKY"] == "0"
    steps = job["steps"]
    setup = next(
        step for step in steps
        if step.get("uses", "").startswith("actions/setup-node@")
    )
    assert setup["with"]["node-version-file"] == "apps/explorer/.nvmrc"
    corepack = next(step for step in steps if step.get("run") == "corepack enable")
    install = next(step for step in steps if step["name"] == "Install frontend dependencies")
    run = next(step for step in steps if step["name"] == "Run frontend Vitest suite")
    assert steps.index(setup) < steps.index(corepack) < steps.index(install) < steps.index(run)
    for step in (setup, corepack, install, run):
        assert "if" not in step
    assert install["working-directory"] == run["working-directory"] == "apps/explorer"
    assert shlex.split(install["run"]) == ["yarn", "install", "--frozen-lockfile"]
    assert shlex.split(run["run"]) == ["yarn", "test:vitest:ci"]
    package = json.loads((ROOT / "apps/explorer/package.json").read_text())
    assert package["packageManager"].startswith("yarn@1.22.22")
    assert package["scripts"]["test:vitest:ci"] == "vitest run"
    assert "vitest" in package["devDependencies"]
    assert (ROOT / "apps/explorer/yarn.lock").is_file()
    gate = (ROOT / "scripts/explorer/gate-test.sh").read_text()
    assert re.search(
        r'run_leg frontend-vitest \'frontend vitest\' "\$frontend_dir" \\\n'
        r'\s+env HUSKY=0 yarn test:vitest:ci\s',
        gate,
    ), "The local gate must invoke the same unfiltered, non-watching frontend suite"
