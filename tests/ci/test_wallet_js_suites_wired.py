"""Keep the wallet service suites and their real prerequisites reachable in CI."""

import json
from pathlib import Path
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/wallet-test.yml"


def service_job(name):
    workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
    assert {"pull_request", "push", "workflow_dispatch"} <= workflow["on"].keys()
    assert workflow["on"]["push"]["branches"] == ["main"]
    for event in ("pull_request", "push"):
        settings = workflow["on"][event]
        if isinstance(settings, dict):
            assert not {"paths", "paths-ignore", "branches-ignore"} & settings.keys()
    job = workflow["jobs"][name]
    assert not {"if", "needs", "continue-on-error"} & job.keys()
    assert "ubuntu-24.04" in job["runs-on"]
    steps = job["steps"]
    for step in steps:
        assert not {"if", "continue-on-error"} & step.keys(), step
    checkout = next(s for s in steps if s.get("uses", "").startswith("actions/checkout@"))
    assert checkout["with"]["persist-credentials"] == "false"
    node = next(s for s in steps if s.get("uses", "").startswith("actions/setup-node@"))
    assert node["with"]["node-version"] == "24.22.0"
    pnpm = command_step(steps, "npm install --global pnpm@10.34.9")
    assert steps.index(checkout) < steps.index(node) < steps.index(pnpm)
    return steps


def command_step(steps, command, directory=None):
    matches = [
        step for step in steps
        if shlex.split(step.get("run", "")) == shlex.split(command)
        and step.get("working-directory") == directory
    ]
    assert len(matches) == 1, (command, directory, matches)
    return matches[0]


def package_test(steps, directory):
    package = json.loads((ROOT / directory / "package.json").read_text())
    assert package["packageManager"] == "pnpm@10.34.9"
    assert package["scripts"]["test"] == "vitest run"
    assert "vitest" in package["devDependencies"]
    assert (ROOT / directory / "vitest.config.ts").is_file()
    run = command_step(steps, "pnpm test", directory)
    install = command_step(steps, "pnpm install --frozen-lockfile", "services/wallet")
    pnpm = command_step(steps, "npm install --global pnpm@10.34.9")
    assert steps.index(pnpm) < steps.index(install) < steps.index(run)
    return run


def agent_sdk_prerequisites(steps, run):
    install = command_step(
        steps, "npm ci --workspaces=false --no-audit --no-fund",
        "services/agent/sdk/typescript",
    )
    build = command_step(
        steps, "npm run build --workspaces=false", "services/agent/sdk/typescript",
    )
    sdk = command_step(
        steps, "pnpm install --ignore-workspace --frozen-lockfile", "sdk/wallet",
    )
    assert steps.index(install) < steps.index(build) < steps.index(sdk) < steps.index(run)
    assert (ROOT / "sdk/wallet/pnpm-lock.yaml").is_file()


def postgres_prerequisites(steps, run):
    postgres = next(s for s in steps if s["name"] == "Install PostgreSQL fixture binaries")
    lines = postgres["run"].splitlines()
    assert lines[0] == "sudo apt-get update -q"
    assert shlex.split(lines[1])[:6] == [
        "sudo", "apt-get", "install", "-y", "--no-install-recommends", "postgresql",
    ]
    assert lines[2] == (
        'echo "PG_BIN_DIR=$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)" '
        '>> "$GITHUB_ENV"'
    )
    assert steps.index(postgres) < steps.index(run)


def assert_workspace_package(directory):
    workspace = ROOT / "services/wallet"
    config = yaml.safe_load((workspace / "pnpm-workspace.yaml").read_text())
    members = {(workspace / member).resolve() for member in config["packages"]}
    assert (ROOT / directory).resolve() in members
    lock = yaml.safe_load((workspace / "pnpm-lock.yaml").read_text())
    importer = (ROOT / directory).relative_to(workspace).as_posix()
    package = json.loads((ROOT / directory / "package.json").read_text())
    for group in ("dependencies", "devDependencies"):
        assert {
            dependency: entry["specifier"]
            for dependency, entry in lock["importers"][importer][group].items()
        } == package[group]


def test_wallet_gateway_vitest_wired():
    steps = service_job("gateway")
    run = package_test(steps, "services/wallet/gateway")
    assert_workspace_package("services/wallet/gateway")
    agent_sdk_prerequisites(steps, run)
    postgres_prerequisites(steps, run)
    go = next(s for s in steps if s.get("uses", "").startswith("actions/setup-go@"))
    assert go["with"]["go-version-file"] == "services/wallet/attestor/go.mod"
    assert go["with"]["cache-dependency-path"] == "services/wallet/attestor/go.sum"
    anvil = next(
        s for s in steps if s.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    )
    assert anvil["with"]["version"] == "v1.8.1"
    assert steps.index(go) < steps.index(run)
    assert steps.index(anvil) < steps.index(run)
    assert list((ROOT / "services/wallet/gateway/test").rglob("*.test.ts"))


def test_wallet_ceremony_import_tests_wired():
    directory = "services/wallet/ceremony/legacy-import"
    steps = service_job("ceremony-import")
    run = package_test(steps, directory)
    postgres_prerequisites(steps, run)
    install = command_step(
        steps, "pnpm install --ignore-workspace --frozen-lockfile", directory,
    )
    gateway = command_step(steps, "pnpm install --frozen-lockfile", "services/wallet")
    assert steps.index(gateway) < steps.index(install) < steps.index(run)
    importer = ROOT / directory
    workspace = yaml.safe_load((importer / "pnpm-workspace.yaml").read_text())
    assert workspace["packages"] == ["."]
    lock = yaml.safe_load((importer / "pnpm-lock.yaml").read_text())
    package = json.loads((importer / "package.json").read_text())
    for group in ("dependencies", "devDependencies"):
        assert {
            dependency: entry["specifier"]
            for dependency, entry in lock["importers"]["."][group].items()
        } == package[group]
    python = next(
        s for s in steps if s.get("uses", "").startswith("actions/setup-python@")
    )
    assert python["with"]["python-version"] == "3.12"
    pytest_install = command_step(steps, "python3 -m pip install pytest")
    pytest_run = command_step(steps, "python3 -m pytest -q test", directory)
    assert steps.index(python) < steps.index(pytest_install) < steps.index(pytest_run)
    assert steps.index(run) < steps.index(pytest_run)
    assert (importer / "test/test_lockfile.py").is_file()
    assert list((importer / "test").glob("*.test.ts"))


def test_wallet_demo_tests_wired():
    steps = service_job("demo")
    run = package_test(steps, "services/wallet/demo")
    assert_workspace_package("services/wallet/demo")
    agent_sdk_prerequisites(steps, run)
    package = json.loads((ROOT / "services/wallet/demo/package.json").read_text())
    assert package["dependencies"]["@paxeer/wallet"] == "workspace:*"
    assert "jsdom" in package["devDependencies"]
    assert list((ROOT / "services/wallet/demo/src").rglob("*.test.tsx"))
