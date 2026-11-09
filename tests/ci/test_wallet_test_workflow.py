"""Keep wallet CI on the real browser SDK and native test environment APIs."""

import json
from pathlib import Path
import re

import yaml


ROOT = Path(__file__).resolve().parents[2]
WALLET = ROOT / "apps/wallet"


def wallet_workflow():
    return yaml.load(
        (ROOT / ".github/workflows/wallet-test.yml").read_text(),
        Loader=yaml.BaseLoader,
    )


def test_sdk_wallet_declares_pnpm():
    sdk = ROOT / "sdk/wallet"
    package = json.loads((sdk / "package.json").read_text())
    app = json.loads((WALLET / "package.json").read_text())
    assert package["packageManager"] == app["packageManager"]
    assert re.fullmatch(r"pnpm@10\.34\.9\+sha512\.[a-f0-9]{128}", package["packageManager"])
    assert (sdk / "pnpm-lock.yaml").is_file()
    lock = yaml.safe_load((sdk / "pnpm-lock.yaml").read_text())
    assert str(lock["lockfileVersion"]) == "9.0"
    assert "." in lock["importers"]
    steps = wallet_workflow()["jobs"]["vitest"]["steps"]
    install = next(step for step in steps if step["name"] == "Install wallet SDK dependencies")
    assert install["working-directory"] == "sdk/wallet"
    assert install["run"] == "pnpm install --ignore-workspace --frozen-lockfile"
    run = next(step for step in steps if step["name"] == "Run Vitest suite")
    assert run["working-directory"] == "${{ matrix.directory }}"
    assert run["run"] == "${{ matrix.command }}"


def test_wallet_vitest_aliases_node_sqlite():
    config = (WALLET / "vitest.config.ts").read_text()
    assert re.search(r"'@sidiora/layerx-sdk':\s*layerxSdkBrowser\b", config)
    assert re.search(r"'@sidiora/layerx-sdk/browser':\s*layerxSdkBrowser\b", config)
    match = re.search(
        r"const layerxSdkBrowser = path.resolve\(\s*__dirname,\s*'([^']+)'\s*,?\s*\)",
        config,
    )
    assert match, "Both aliases must resolve to the real browser entry"
    entry = (WALLET / match[1]).resolve()
    assert entry == ROOT / "services/agent/sdk/typescript/src/browser.ts"
    pending = [entry]
    visited = set()
    while pending:
        source = pending.pop()
        if source in visited:
            continue
        visited.add(source)
        text = source.read_text()
        assert not re.search(r"['\"](?:node:)?sqlite['\"]", text), source
        for specifier in re.findall(r"(?:from\s*|import\s*\()(['\"])([^'\"]+)\1", text):
            relative = specifier[1]
            if relative.startswith("."):
                dependency = (source.parent / relative).resolve()
                if dependency.suffix == ".js":
                    dependency = dependency.with_suffix(".ts")
                assert dependency.is_file(), dependency
                pending.append(dependency)
    assert len(visited) > 1, "Inspect the browser entry's dependencies too"
    for name in (
        "BridgeView.test.tsx", "ExchangeView.test.tsx", "FeeChoice.test.tsx",
        "LaunchpadView.test.tsx", "WebDataView.test.tsx", "repair-22-33-205.test.ts",
    ):
        text = (WALLET / "src/surfaces" / name).read_text()
        assert "from '@sidiora/layerx-sdk/browser'" in text, name
        assert "from '@sidiora/layerx-sdk'" not in text, name


def test_wallet_setup_provides_web_locks():
    steps = wallet_workflow()["jobs"]["vitest"]["steps"]
    setup = next(step for step in steps if step.get("uses", "").startswith("actions/setup-node@"))
    assert setup["with"]["node-version"] == "24.22.0"
    for name in ("pns/repair-22-33-138.test.ts", "swap/repair-22-31-239.test.ts"):
        source = (WALLET / "src/widgets" / name).read_text()
        assert "import * as workerThreads from 'node:worker_threads'" in source, name
        assert "const locks: unknown = Reflect.get(workerThreads, 'locks')" in source, name
        setup_body = source.split("beforeAll(async () => {", 1)[1]
        assert "Object.defineProperty(navigator, 'locks', { value: locks, configurable: true })" in setup_body, name
        assert "expect(navigator.locks).toBe(locks)" in setup_body, name
        assert "expect(typeof navigator.locks.request).toBe('function')" in setup_body, name
    worker = (WALLET / "src/pwa/worker.test.ts").read_text()
    assert "import process from 'node:process'" in worker
    assert re.search(r"vm\.runInNewContext\(source,\s*\{[^}]*\bprocess\b", worker)


def test_wallet_workflow_matrix_dirs_exist():
    workflow = wallet_workflow()
    assert {"pull_request", "push", "workflow_dispatch"} <= workflow["on"].keys()
    job = workflow["jobs"]["vitest"]
    matrix = job["strategy"]["matrix"]
    assert matrix == {
        "directory": ["apps/wallet", "sdk/wallet"],
        "include": [
            {"directory": "apps/wallet", "command": "pnpm exec vitest run"},
            {
                "directory": "sdk/wallet",
                "command": "python3 ../../tools/ci/run_wallet_vitest.py",
            },
        ],
    }
    assert job["strategy"]["fail-fast"] == "false"
    assert "if" not in job
    assert "continue-on-error" not in job
    for directory in matrix["directory"]:
        package = ROOT / directory
        assert package.is_dir(), directory
        manifest = json.loads((package / "package.json").read_text())
        assert "vitest" in manifest["devDependencies"], directory
        assert (package / "pnpm-lock.yaml").is_file(), directory
    for step in job["steps"]:
        assert "continue-on-error" not in step
        directory = step.get("working-directory")
        if directory and directory != "${{ matrix.directory }}":
            assert (ROOT / directory).is_dir(), directory
    run = next(step for step in job["steps"] if step["name"] == "Run Vitest suite")
    assert "if" not in run
    assert run["run"] == "${{ matrix.command }}"


def test_sdk_wallet_installs_gateway_dependencies_and_fixture_tools():
    workflow = wallet_workflow()
    steps = workflow["jobs"]["vitest"]["steps"]
    gateway_steps = workflow["jobs"]["gateway"]["steps"]
    sdk = next(step for step in steps if step["name"] == "Install wallet SDK dependencies")
    install = next(step for step in steps if step["name"] == "Install wallet workspace dependencies")
    gateway_install = next(
        step for step in gateway_steps
        if step["name"] == "Install wallet workspace dependencies"
    )
    suite = next(step for step in steps if step["name"] == "Run Vitest suite")
    assert install["if"] == "matrix.directory == 'sdk/wallet'"
    assert {key: value for key, value in install.items() if key != "if"} == gateway_install
    assert install["working-directory"] == "services/wallet"
    assert install["run"] == "pnpm install --frozen-lockfile"
    assert steps.index(sdk) < steps.index(install) < steps.index(suite)
    for name in ("Set up Go for attestor fixtures", "Install PostgreSQL fixture binaries"):
        setup = next(step for step in steps if step["name"] == name)
        gateway_setup = next(step for step in gateway_steps if step["name"] == name)
        assert setup["if"] == "matrix.directory == 'sdk/wallet'"
        assert {key: value for key, value in setup.items() if key != "if"} == gateway_setup
        assert steps.index(setup) < steps.index(suite)
