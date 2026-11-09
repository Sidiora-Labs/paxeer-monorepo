import importlib.util
import itertools
import json
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
LAUNCHPAD = ROOT / "apps/launchpad"
MATRIX_VALUE = re.compile(r"\$\{\{\s*matrix\.(\w+)\s*\}\}")
RUNNER = ROOT / "tools/ci/run_wallet_vitest.py"


def wallet_runner():
    spec = importlib.util.spec_from_file_location("run_wallet_vitest", RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_yaml(path):
    return yaml.load(path.read_text(), Loader=yaml.BaseLoader)


def matrix_rows(job):
    matrix = job.get("strategy", {}).get("matrix", {})
    assert isinstance(matrix, dict), "pnpm jobs need a statically inspectable matrix"
    axes = {
        key: values for key, values in matrix.items()
        if key not in {"include", "exclude"}
    }
    assert all(isinstance(values, list) and values for values in axes.values())
    rows = [dict(zip(axes, values)) for values in itertools.product(*axes.values())]
    rows = [
        row for row in rows
        if not any(
            all(row.get(key) == value for key, value in excluded.items())
            for excluded in matrix.get("exclude", [])
        )
    ]
    for included in matrix.get("include", []):
        matches = [
            row for row in rows
            if all(
                key not in axes or row[key] == value
                for key, value in included.items()
            )
        ]
        if matches:
            for row in matches:
                row.update(included)
        else:
            rows.append(included)
    assert rows, "pnpm job matrix must execute at least once"
    return rows


def expand_matrix(value, row):
    return MATRIX_VALUE.sub(lambda match: str(row[match.group(1)]), value)


def run_directory(workflow, job, step, row):
    defaults = workflow.get("defaults", {}).get("run", {})
    defaults = {**defaults, **job.get("defaults", {}).get("run", {})}
    directory = expand_matrix(
        step.get("working-directory", defaults.get("working-directory", ".")), row
    )
    assert "${{" not in directory, directory
    path = (ROOT / directory).resolve()
    assert path.is_dir(), directory
    assert path.is_relative_to(ROOT), directory
    return path


def pnpm_commands(script, directory):
    for line in script.splitlines():
        lexer = shlex.shlex(line, posix=True, punctuation_chars=";&|")
        lexer.whitespace_split = True
        commands = [[]]
        for token in lexer:
            if token in {";", "&&", "||", "|", "&"}:
                commands.append([])
            else:
                commands[-1].append(token)
        for tokens in commands:
            if not tokens:
                continue
            if tokens[0] == "cd":
                assert len(tokens) == 2, tokens
                directory = (directory / tokens[1]).resolve()
            if "pnpm" in tokens:
                yield directory, tokens[tokens.index("pnpm") + 1:]


def pnpm_directory(directory, args):
    for index, arg in enumerate(args):
        if arg in {"exec", "run"}:
            break
        if arg in {"--dir", "-C"}:
            return (directory / args[index + 1]).resolve()
        if arg.startswith("--dir="):
            return (directory / arg.split("=", 1)[1]).resolve()
    return directory


def nearest_workspace(directory):
    assert directory.is_relative_to(ROOT), directory
    for parent in (directory, *directory.parents):
        workspace = parent / "pnpm-workspace.yaml"
        if workspace.is_file():
            return workspace
        manifest = parent / "package.json"
        if manifest.is_file() and "workspaces" in json.loads(manifest.read_text()):
            return manifest
        if parent == ROOT:
            break
    raise AssertionError(f"No workspace found above {directory.relative_to(ROOT)}")


def test_pnpm_steps_resolve_a_pnpm_workspace_root():
    checked = 0
    workflows = sorted((ROOT / ".github/workflows").glob("*.yml"))
    workflows += sorted((ROOT / ".github/workflows").glob("*.yaml"))
    for path in workflows:
        workflow = load_yaml(path)
        for job_name, job in workflow.get("jobs", {}).items():
            steps = [
                step for step in job.get("steps", [])
                if "pnpm" in step.get("run", "")
                or step.get("run") == "${{ matrix.command }}"
            ]
            if not steps:
                continue
            for row in matrix_rows(job):
                for step in steps:
                    script = expand_matrix(step["run"], row)
                    directory = run_directory(workflow, job, step, row)
                    for cwd, args in pnpm_commands(script, directory):
                        context = (path.name, job_name, step.get("name"), row, args)
                        checked += 1
                        options = args[:args.index("exec")] if "exec" in args else args
                        options = (
                            options[:options.index("run")]
                            if "run" in options else options
                        )
                        if "--ignore-workspace" in options:
                            continue
                        workspace = nearest_workspace(pnpm_directory(cwd, args))
                        assert workspace.name == "pnpm-workspace.yaml", context
                        manifest = json.loads(
                            (workspace.parent / "package.json").read_text()
                        )
                        assert manifest["packageManager"].startswith("pnpm@"), context
    assert checked > 0, "No pnpm workflow commands were inspected"


def test_launchpad_matrix_runs_from_launchpad_workspace():
    workflow = load_yaml(ROOT / ".github/workflows/launchpad-test.yml")
    job = workflow["jobs"]["launchpad-test"]
    steps = job["steps"]
    test_step, = [step for step in steps if step.get("name") == "Test package"]
    assert "if" not in job and "if" not in test_step
    assert "continue-on-error" not in job and "continue-on-error" not in test_step
    assert job["strategy"]["fail-fast"] == "false"
    workspace = load_yaml(LAUNCHPAD / "pnpm-workspace.yaml")
    members = {
        path.resolve()
        for pattern in workspace["packages"]
        for path in LAUNCHPAD.glob(pattern)
        if (path / "package.json").is_file()
    }
    expected = {
        member for member in members
        if "test" in json.loads((member / "package.json").read_text()).get("scripts", {})
    }
    assert {LAUNCHPAD / "web", LAUNCHPAD / "mobile"} <= expected
    assert any(member.is_relative_to(ROOT / "sdk/launchpad") for member in expected)
    packages = job["strategy"]["matrix"]["package"]
    assert len(packages) == len(set(packages))
    assert {(ROOT / package).resolve() for package in packages} == expected
    lock = load_yaml(LAUNCHPAD / "pnpm-lock.yaml")
    locked = {(LAUNCHPAD / importer).resolve() for importer in lock["importers"]}
    assert members <= locked
    for row in matrix_rows(job):
        directory = run_directory(workflow, job, test_step, row)
        command, = list(pnpm_commands(expand_matrix(test_step["run"], row), directory))
        cwd, args = command
        assert cwd == ROOT
        assert args[:2] == ["--dir", "apps/launchpad"]
        assert pnpm_directory(cwd, args) == LAUNCHPAD
        assert args[-2:] == ["run", "test"]
        assert "--fail-if-no-match" in args
        assert args.count("--filter") == 1
        selector = args[args.index("--filter") + 1]
        assert selector.startswith("./")
        assert (LAUNCHPAD / selector).resolve() == (ROOT / row["package"]).resolve()
        assert nearest_workspace(LAUNCHPAD) == LAUNCHPAD / "pnpm-workspace.yaml"


def test_sdk_wallet_vitest_ignores_root_workspace():
    workflow = load_yaml(ROOT / ".github/workflows/wallet-test.yml")
    job = workflow["jobs"]["vitest"]
    assert "sdk/wallet" in job["strategy"]["matrix"]["directory"]
    suite, = [step for step in job["steps"] if step.get("name") == "Run Vitest suite"]
    assert "if" not in job and "if" not in suite
    assert "continue-on-error" not in job and "continue-on-error" not in suite
    assert (
        run_directory(workflow, job, suite, {"directory": "sdk/wallet"})
        == ROOT / "sdk/wallet"
    )
    row, = [row for row in matrix_rows(job) if row["directory"] == "sdk/wallet"]
    assert shlex.split(expand_matrix(suite["run"], row)) == [
        "python3", "../../tools/ci/run_wallet_vitest.py"
    ]
    runner = wallet_runner()
    assert runner.WALLET == ROOT / "sdk/wallet"
    assert runner.wallet_vitest_command(runner.read_baseline())[:5] == [
        "pnpm", "--ignore-workspace", "exec", "vitest", "run"
    ]
    manifest = json.loads((ROOT / "sdk/wallet/package.json").read_text())
    assert manifest["packageManager"].startswith("pnpm@")
    for script in ("build", "test", "typecheck"):
        commands = list(pnpm_commands(manifest["scripts"][script], ROOT / "sdk/wallet"))
        assert commands, script
        for _, args in commands:
            assert args[:3] == ["--ignore-workspace", "run", "build:agent-sdk"], script
    assert manifest["scripts"]["build:agent-sdk"] == (
        "npm --prefix ../../services/agent/sdk/typescript ci --workspaces=false --no-audit --no-fund"
        " && npm --prefix ../../services/agent/sdk/typescript run build --workspaces=false"
    )


def test_sdk_wallet_workspace_install_resolves_from_services_wallet():
    workflow = load_yaml(ROOT / ".github/workflows/wallet-test.yml")
    job = workflow["jobs"]["vitest"]
    steps = job["steps"]
    install, = [
        step for step in steps
        if step.get("name") == "Install wallet workspace dependencies"
    ]
    assert install["if"] == "matrix.directory == 'sdk/wallet'"
    row, = [row for row in matrix_rows(job) if row["directory"] == "sdk/wallet"]
    directory = run_directory(workflow, job, install, row)
    assert directory == ROOT / "services/wallet"
    command, = list(pnpm_commands(expand_matrix(install["run"], row), directory))
    cwd, args = command
    assert args == ["install", "--frozen-lockfile"]
    assert nearest_workspace(pnpm_directory(cwd, args)) == directory / "pnpm-workspace.yaml"
    sdk, = [step for step in steps if step.get("name") == "Install wallet SDK dependencies"]
    suite, = [step for step in steps if step.get("name") == "Run Vitest suite"]
    assert steps.index(sdk) < steps.index(install) < steps.index(suite)


def test_sdk_wallet_job_runs_the_baseline_script():
    workflow = load_yaml(ROOT / ".github/workflows/wallet-test.yml")
    job = workflow["jobs"]["vitest"]
    suite, = [step for step in job["steps"] if step.get("name") == "Run Vitest suite"]
    rows = matrix_rows(job)
    assert {row["directory"] for row in rows} == {"sdk/wallet", "apps/wallet"}
    for row in rows:
        directory = run_directory(workflow, job, suite, row)
        command = shlex.split(expand_matrix(suite["run"], row))
        if row["directory"] == "sdk/wallet":
            assert command == ["python3", "../../tools/ci/run_wallet_vitest.py"]
            assert (directory / command[1]).resolve() == RUNNER
        else:
            assert command == ["pnpm", "exec", "vitest", "run"]

    runner = wallet_runner()
    entries = runner.read_baseline()
    assert entries == json.loads(
        (ROOT / "tests/ci/data/wallet_vitest_baseline.json").read_text()
    )
    pattern = re.compile(runner.baseline_name_pattern(entries))
    excluded_names = []
    other_names = []
    for entry in entries:
        source = (ROOT / entry["file"]).read_text()
        *suites, name = entry["full_name"].split(" > ")
        for parent in suites:
            assert re.search(r"describe\(\s*(['\"])" + re.escape(parent) + r"\1", source)
        literal = r"(?:it|test)\(\s*(['\"])" + re.escape(name) + r"\1\s*,"
        if not re.search(literal, source):
            assert re.search(r"\bfunction\s+" + re.escape(name) + r"\s*\(", source)
            registration = r"(?:it|test)\(\s*" + re.escape(name) + r"(?:\.name)?\s*[,)]"
            if not re.search(registration, source):
                # receipt.test.ts registers each named function in its array.
                assert re.search(
                    r"for \(const run of \[[\s\S]*\b" + re.escape(name)
                    + r"\b[\s\S]*\]\) \{\s*test\(run.name, run\);", source
                )
        name_in_runner = entry["full_name"].replace(" > ", " ")
        excluded_names.append(name_in_runner)
        assert not pattern.fullmatch(name_in_runner), entry
        for other in (
            name_in_runner + " extra",
            "extra " + name_in_runner,
            "Other suite " + name_in_runner,
        ):
            other_names.append(other)
            assert pattern.fullmatch(other), (entry, other)
    javascript_check = subprocess.run(
        ["node", "--input-type=module", "-e", """
import assert from 'node:assert/strict';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const { pattern, excluded, others } = JSON.parse(input);
const filter = new RegExp(pattern);
for (const name of excluded) assert.equal(filter.test(name), false, name);
for (const name of others) assert.equal(filter.test(name), true, name);
"""],
        input=json.dumps({
            "pattern": runner.baseline_name_pattern(entries),
            "excluded": excluded_names,
            "others": other_names,
        }),
        text=True,
        capture_output=True,
    )
    assert javascript_check.returncode == 0, javascript_check.stderr
    assert pattern.fullmatch("test/future.test.ts an unrecorded case")
    assert re.fullmatch(runner.baseline_name_pattern([]), "test/new.test.ts any case")
    arguments = ["test/provider.test.ts", "--reporter=verbose"]
    assert runner.wallet_vitest_command(entries, arguments) == [
        "pnpm", "--ignore-workspace", "exec", "vitest", "run", *arguments,
        "--testNamePattern", runner.baseline_name_pattern(entries),
    ]
