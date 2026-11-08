import ast
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]


def _test_target_commands():
    makefile = (ROOT / "Makefile").read_text().replace("\\\n", " ")
    match = re.search(r"^test-tests-python:[^\n]*\n((?:\t[^\n]*\n)+)", makefile, re.M)
    assert match is not None, "test-tests-python must have an executable recipe"
    return [shlex.split(line.strip()) for line in match[1].splitlines()]


def pytest_paths():
    commands = [
        command for command in _test_target_commands()
        if command[:3] == ["python3", "-m", "pytest"]
    ]
    assert len(commands) == 1, "run the Python suites in one pytest invocation"
    assert all(not arg.startswith("-") for arg in commands[0][3:]), commands[0]
    paths = [(ROOT / arg).resolve() for arg in commands[0][3:]]
    assert paths and all(path.exists() for path in paths), commands[0]
    return paths


def test_every_tools_ci_module_is_collected():
    suites = pytest_paths()
    tools_tests = ROOT / "tools/ci/tests"
    assert tools_tests in suites, "the public target must collect tools/ci/tests"
    assert ROOT / "tests/ci" in suites
    modules = sorted(tools_tests.rglob("test_*.py"))
    assert modules, "the tools CI suite must contain test modules"
    for module in modules:
        assert any(suite == module or suite in module.parents for suite in suites), module
    assert not (ROOT / "scripts/ci/test_repair_22_31_317.py").exists()
    completion = ast.parse((tools_tests / "test_completion_gate.py").read_text())
    moved_tests = {
        node.name for node in completion.body if isinstance(node, ast.FunctionDef)
    }
    assert {
        "test_release_record_is_accepted_unchanged",
        "test_repeated_release_retains_both_source_sidecars",
        "test_command_failure_is_bound_to_source_and_rejected",
        "test_dirty_source_is_retained_but_not_release_eligible",
        "test_transient_source_mutation_cannot_receive_gate_credit",
        "test_command_must_match_selected_task_before_execution",
        "test_consumer_rejects_missing_source_or_changed_log",
        "test_legacy_unbound_cli_is_rejected_without_source_tree_evidence",
    } <= moved_tests
    workflow = yaml.safe_load((ROOT / ".github/workflows/ci.yml").read_text())
    assert any(
        step.get("run") == "make test-tests-python"
        for step in workflow["jobs"]["tests-python"]["steps"]
    )


def test_completion_gate_planned_paths_exist():
    module = ast.parse((ROOT / "tools/ci/tests/test_completion_gate.py").read_text())
    assignments = [
        node.value for node in module.body
        if isinstance(node, ast.Assign)
        and any(isinstance(name, ast.Name) and name.id == "PLANNED_PATHS" for name in node.targets)
    ]
    assert len(assignments) == 1
    planned = ast.literal_eval(assignments[0])
    assert planned
    for task, relative_path in planned:
        assert (ROOT / relative_path).is_file(), f"task {task}: missing {relative_path}"
    assert ("16.97.3", "tests/ci/test_retired_requested_retirements_absent.py") in planned


def test_no_unreferenced_ci_shell_tests():
    commands = _test_target_commands()
    shell_tests = sorted((ROOT / "tools/ci/tests").rglob("*.sh"))
    assert shell_tests
    for script in shell_tests:
        command = ["bash", script.relative_to(ROOT).as_posix()]
        assert commands.count(command) == 1, f"{script}: must run once in test-tests-python"
        result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=60)
        assert result.returncode == 0, result.stdout + result.stderr


def test_test_tests_python_has_no_duplicate_paths():
    paths = pytest_paths()
    assert len(paths) == len(set(paths)), (
        f"pytest repeats path tokens: {[path.relative_to(ROOT).as_posix() for path in paths]}"
    )
