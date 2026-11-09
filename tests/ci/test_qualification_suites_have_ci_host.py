import ast
from pathlib import Path
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
QUALIFICATION = ROOT / "tools/qualification"


def test_every_qualification_test_dir_is_in_a_make_target():
    result = subprocess.run(
        ["make", "--no-print-directory", "-n", "test-qualification-python"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    commands = result.stdout.splitlines()
    assert len(commands) == 1, result.stdout
    command = shlex.split(commands[0])
    assert command[:4] == ["python3", "-m", "pytest", "-q"]
    assert command[4:] == ["tools/qualification", "-m", "not native"]
    targets = command[4:-2]
    assert targets, "The qualification target must select test directories"
    directories = []
    for target in targets:
        directory = (ROOT / target).resolve()
        assert directory.is_dir(), target
        assert directory == QUALIFICATION or QUALIFICATION in directory.parents, target
        directories.append(directory)

    tests = [
        path
        for path in QUALIFICATION.rglob("*.py")
        if path.name.startswith("test_") or path.name.endswith("_test.py")
    ]
    assert tests, "No qualification tests found"
    uncovered = [
        str(path.relative_to(ROOT))
        for path in tests
        if not any(directory in path.parents for directory in directories)
    ]
    assert not uncovered, "Qualification tests have no Make host: " + ", ".join(uncovered)


def test_ci_workflow_calls_test_qualification_python():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
    )
    jobs = workflow["jobs"]
    assert list(jobs).index("qualification-python") == list(jobs).index("tests-python") + 1
    job = jobs["qualification-python"]
    assert "if" not in job
    assert not job.get("continue-on-error", False)
    steps = job["steps"]
    qualification_steps = [
        index
        for index, step in enumerate(steps)
        if step.get("run", "").strip() == "make test-qualification-python"
    ]
    assert len(qualification_steps) == 1
    index = qualification_steps[0]
    step = steps[index]
    assert shlex.split(step["run"]) == ["make", "test-qualification-python"]
    assert "if" not in step
    assert not step.get("continue-on-error", False)
    commands = [step.get("run", "") for step in steps]
    assert all(
        target not in command
        for command in commands
        for target in ("make test-tests-python", "make test-python-suites")
    )
    installations = [command for command in commands if "-m pip install" in command]
    assert len(installations) == 1
    assert installations[0].splitlines() == [
        'python3 -m venv "$RUNNER_TEMP/qualification-python"',
        '"$RUNNER_TEMP/qualification-python/bin/python3" -m pip install -r tools/qualification/requirements.txt',
        'echo "$RUNNER_TEMP/qualification-python/bin" >> "$GITHUB_PATH"',
    ]
    install_index = next(
        index
        for index, step in enumerate(steps)
        if step.get("run") == installations[0]
    )
    assert install_index < index
    repository_commands = [step.get("run", "") for step in jobs["tests-python"]["steps"]]
    assert any(
        '"$RUNNER_TEMP/tests-python/bin/python3" -m pip install pytest PyYAML -r tests/bridge/requirements.txt'
        in command
        for command in repository_commands
    )
    assert "make test-qualification-python" not in repository_commands


def test_native_qualification_cases_are_marked_and_registered():
    tree = ast.parse((QUALIFICATION / "test_beta_driver.py").read_text())
    case = next(
        node
        for node in ast.walk(tree)
        if isinstance(node, ast.FunctionDef)
        and node.name == "test_multichain_inventory_reports_absent_infrastructure_as_failed"
    )
    assert "pytest.mark.native" in [ast.unparse(mark) for mark in case.decorator_list]
    registration = ast.parse((QUALIFICATION / "conftest.py").read_text())
    configure = next(
        node
        for node in registration.body
        if isinstance(node, ast.FunctionDef) and node.name == "pytest_configure"
    )
    assert any(
        isinstance(node, ast.Call)
        and ast.unparse(node.func) == "config.addinivalue_line"
        and len(node.args) == 2
        and ast.literal_eval(node.args[0]) == "markers"
        and ast.literal_eval(node.args[1]).startswith("native:")
        for node in ast.walk(configure)
    )


def test_node_workflow_runs_native_qualification_after_node_tests():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/node-build.yml").read_text(encoding="utf-8")
    )
    job = workflow["jobs"]["node-tests"]
    assert "if" not in job
    assert not job.get("continue-on-error", False)
    steps = job["steps"]
    node_index = next(
        index
        for index, step in enumerate(steps)
        if step.get("run", "").strip() == "go test -timeout 15m -count=1 ./node/..."
    )
    assert any(
        step.get("run", "").strip() == "make paxd-native-libs"
        for step in steps[:node_index]
    )
    native = steps[node_index + 1]
    assert "if" not in native
    assert not native.get("continue-on-error", False)
    assert native["env"]["CGO_ENABLED"] == "1"
    assert native["run"].splitlines() == [
        'python3 -m venv "$RUNNER_TEMP/qualification-native"',
        'export PATH="$RUNNER_TEMP/qualification-native/bin:$PATH"',
        "python3 -m pip install -r tools/qualification/requirements.txt",
        "python3 -m pytest -q tools/qualification -m native",
    ]
    assert any("python3-venv" in step.get("run", "") for step in steps[:node_index])
