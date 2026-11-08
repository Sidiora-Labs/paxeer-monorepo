"""Keep the platform middleware, framework and example suites reachable in CI."""

import json
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[2]
PYTHON = "$(PLATFORM_PYTHON_ENV)/bin/python"


def recipe(target):
    makefile = (ROOT / "services/platform/Makefile.inc").read_text()
    match = re.search(rf"^{re.escape(target)}:([^\n]*)\n((?:\t[^\n]*\n)+)", makefile, re.M)
    assert match is not None, target
    commands = match[2].replace("\\\n", " ").splitlines()
    assert all(not line.lstrip().startswith(("-", "@-")) for line in commands)
    return shlex.split(match[1]), [shlex.split(line) for line in commands]


def middleware_commands():
    workflow = yaml.load(
        (ROOT / ".github/workflows/platform.yml").read_text(), Loader=yaml.BaseLoader
    )
    assert {"push", "pull_request"} <= workflow["on"].keys()
    assert workflow["on"]["push"]["branches"] == ["main"]
    job = workflow["jobs"]["build-lint-test"]
    assert not {"if", "continue-on-error"} & job.keys()
    steps = job["steps"]
    run = next(step for step in steps if step.get("run") == "make platform-test-middleware")
    assert not {"if", "continue-on-error"} & run.keys()
    for command in ("make platform-js-install", "make platform-dependencies-install"):
        install = next(step for step in steps if step.get("run") == command)
        assert not {"if", "continue-on-error"} & install.keys()
        assert steps.index(install) < steps.index(run)
    prerequisites, commands = recipe("platform-test-middleware")
    assert "platform-sample-dependencies-ready" in prerequisites
    assert "include services/platform/Makefile.inc" in (ROOT / "Makefile").read_text()
    return commands


def assert_package_test(commands, directory):
    package = json.loads((ROOT / directory / "package.json").read_text())
    workspaces = json.loads((ROOT / "package.json").read_text())["workspaces"]
    assert directory in workspaces
    assert ["npm", "test", "--workspace", package["name"]] in commands
    scripts = package["scripts"]
    assert scripts["pretest"] == "npm run build"
    assert scripts["prebuild"].endswith("run build:dependencies -- " + package["name"])
    assert "--test" in shlex.split(scripts["test"])
    assert "test/*.test.mjs" in shlex.split(scripts["test"])
    assert list((ROOT / directory / "test").glob("*.test.mjs"))
    assert "--test-skip-pattern" not in scripts["test"]
    assert "||" not in scripts["test"]
    return scripts


def test_platform_middleware_runs_package_tests():
    commands = middleware_commands()
    for name in ("seller", "agent", "buyer", "merchant", "conformance"):
        scripts = assert_package_test(commands, f"services/platform/middleware/{name}")
        if name == "conformance":
            assert shlex.split(scripts["test"]) == [
                "node", "--test", "test/*.test.mjs", "pay6/*.test.mjs",
                "../examples/test/*.test.mjs",
            ]
    assert [
        "npm", "run", "conformance", "--workspace", "@sidiora/layerx-middleware-conformance"
    ] in commands
    assert [
        "python3", "-m", "pytest", "-q", "tests/ci/test_platform_middleware_suites_wired.py",
    ] in commands
    assert [
        PYTHON, "-m", "pytest", "-q", "services/platform/middleware/conformance/pay6",
        "services/platform/middleware/examples",
    ] in commands
    for directory in ("conformance/pay6", "examples"):
        assert list((ROOT / "services/platform/middleware" / directory).glob("test_*.py"))


def test_platform_integrations_tests_wired():
    commands = middleware_commands()
    for name in ("agents", "express", "next"):
        scripts = assert_package_test(commands, f"sdk/integrations/{name}")
        if name in ("express", "next"):
            assert scripts["test"].endswith("&& npm --prefix example test")
    assert [
        "PYTHONPATH=services/agent/sdk/python:sdk/integrations/fastapi",
        PYTHON, "-m", "pytest", "-q", "sdk/integrations/fastapi",
    ] in commands
    marketplace = json.loads(
        (ROOT / "services/platform/examples/marketplace/package.json").read_text()
    )
    assert ["npm", "test", "--workspace", marketplace["name"]] in commands
    assert marketplace["scripts"]["pretest"].endswith(
        "run build:dependencies -- " + marketplace["name"]
    )
    assert shlex.split(marketplace["scripts"]["test"]) == [
        "node", "--check", "index.mjs", "&&", "node", "test/deploy-payload.test.mjs",
        "&&", "node", "test/receipt-outcome.test.mjs",
    ]


def test_fastapi_integration_declares_test_deps():
    integration = ROOT / "sdk/integrations/fastapi"
    project = tomllib.loads((integration / "pyproject.toml").read_text())["project"]
    dependencies = project["optional-dependencies"]["test"]
    assert set(dependencies) == {
        line.strip() for line in (integration / "tests/requirements.txt").read_text().splitlines()
        if line.strip()
    }
    assert {re.split(r"[<>=!~\[]", value)[0] for value in dependencies} == {
        "pytest", "fastapi", "httpx", "cryptography",
    }
    prerequisites, _ = recipe("platform-dependencies-install")
    assert "platform-sample-preflight" in prerequisites
    _, setup = recipe("platform-dependencies-install")
    assert ["$(MAKE)", "platform-sample-install"] in setup
    _, install = recipe("platform-sample-install")
    assert [
        PYTHON, "-m", "pip", "install", "-r", "sdk/integrations/fastapi/tests/requirements.txt",
    ] in install
    assert install.index(["python3", "-m", "venv", "$(PLATFORM_PYTHON_ENV)"]) < install.index([
        PYTHON, "-m", "pip", "install", "-r", "sdk/integrations/fastapi/tests/requirements.txt",
    ])
