"""Require every tracked Python test root to have CI coverage or an explanation."""

from pathlib import Path, PurePosixPath
import os
import posixpath
import re
import shlex
import subprocess
import sys

import yaml

from test_js_suites_have_runner import expand_matrix, matrix_rows
from test_make_recipe_paths_exist import make_rules
import run_python_suites as python_suites


ROOT = Path(__file__).resolve().parents[2]
EXCLUSIONS = ROOT / "tests/ci/data/python_suite_exclusions.txt"
SUITE_ROOTS = (
    "cmd/layerx-genesis",
    "cmd/layerx-module-registry",
    "tests/daemon",
    "tests/explorer",
    "tests/conformance",
    "tests/integration/scripts",
    "tests/explorer_fixture_test.py",
    "tests/qualification/test_mcp_binding_receipt_keys.py",
    "services/platform/hosted/node",
    "services/platform/hosted/testnet",
    "services/platform/hosted/tests",
    "services/platform/hosted/gateway/tests",
    "services/platform/tests",
)


def tracked_tests():
    paths = subprocess.check_output(
        ["git", "ls-files", "-z", "--", "*.py"], cwd=ROOT
    ).decode().split("\0")
    return {
        path
        for path in paths
        if path
        and not path.startswith("tests/ci/")
        and (
            PurePosixPath(path).name.startswith("test_")
            or PurePosixPath(path).name.endswith("_test.py")
        )
    }


def _tests_beneath(path, tests):
    return {
        test for test in tests
        if test == path or test.startswith(path.rstrip("/") + "/") or path == "."
    }


def exclusions():
    recorded = {}
    for number, line in enumerate(EXCLUSIONS.read_text().splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        path, separator, reason = line.partition("|")
        path, reason = path.strip(), reason.strip()
        assert separator and len(reason.split()) >= 8, (
            f"{EXCLUSIONS.name}:{number}: explain the exclusion"
        )
        assert path not in recorded, f"Duplicate Python exclusion: {path}"
        assert path and not any(character in path for character in "*?[]"), path
        assert not PurePosixPath(path).is_absolute(), path
        assert ".." not in PurePosixPath(path).parts, path
        assert posixpath.normpath(path) == path, path
        recorded[path] = reason
    return recorded


def command_tests(command, directory, tests, rules, seen=None):
    """Follow executable pytest/unittest invocations and Make prerequisites."""
    seen = set() if seen is None else seen
    covered = set()
    for line in command.replace("\\\n", " ").splitlines():
        line = line.lstrip()
        if line.startswith("-"):
            continue
        line = line.lstrip("@+")
        if not re.match(
            r"^(?:[A-Za-z_]\w*=(?:\"[^\"]*\"|'[^']*'|\S+)\s+)*"
            r"(?:python3?|pytest|make|\$\(MAKE\)|cd)(?:\s|$)", line
        ):
            continue
        lexer = shlex.shlex(line, posix=True, punctuation_chars=";&|")
        lexer.whitespace_split = True
        words = list(lexer)
        if "||" in words or "|" in words or "&" in words:
            continue
        segments, segment = [], []
        for word in words + [";"]:
            if word in {";", "&&"}:
                segments.append(segment)
                segment = []
            else:
                segment.append(word)
        current_directory = directory
        for words in segments:
            while words and re.match(r"^[A-Za-z_]\w*=", words[0]):
                words = words[1:]
            if not words:
                continue
            if words[0] == "cd" and len(words) == 2:
                current_directory = posixpath.normpath(
                    posixpath.join(current_directory, words[1])
                )
                continue
            executable = PurePosixPath(words[0]).name
            if executable in {"make", "$(MAKE)"}:
                if any(word in {"-n", "--just-print", "--dry-run", "--touch", "-t"}
                       for word in words[1:]):
                    continue
                if current_directory != ".":
                    continue
                for target in words[1:]:
                    if target.startswith("-") or "=" in target or target not in rules:
                        continue
                    if target in seen:
                        continue
                    seen.add(target)
                    rule = rules[target]
                    covered.update(command_tests(
                        "make " + " ".join(rule.prerequisites), ".", tests, rules, seen
                    ))
                    for recipe in rule.recipe:
                        covered.update(command_tests(recipe, ".", tests, rules, seen))
                continue
            if executable in {"python", "python3"} and words[1:3] == ["-m", "pytest"]:
                arguments = words[3:]
            elif (
                executable in {"python", "python3"}
                and words[1:] == ["tests/ci/run_python_suites.py"]
                and current_directory == "."
            ):
                ignored = (
                    set(python_suites.read_exclusions())
                    | set(python_suites.read_baseline())
                )
                covered.update(
                    test for test in tests
                    if any(
                        python_suites.beneath(test, root)
                        for root in python_suites.SUITE_ROOTS
                    )
                    and not any(
                        python_suites.beneath(test, root) for root in ignored
                    )
                )
                continue
            elif executable == "pytest":
                arguments = words[1:]
            elif executable in {"python", "python3"} and words[1:3] == ["-m", "unittest"]:
                arguments = words[3:]
                if "discover" in arguments:
                    if "-s" not in arguments or "-p" in arguments or "--pattern" in arguments:
                        continue
                    start = arguments.index("-s") + 1
                    if start < len(arguments):
                        covered.update(_tests_beneath(
                            posixpath.normpath(posixpath.join(current_directory, arguments[start])),
                            tests,
                        ))
                else:
                    for module in arguments:
                        covered.update(_tests_beneath(module.replace(".", "/") + ".py", tests))
                continue
            else:
                continue
            # A filtered, collection-only, or failure-masking invocation does not
            # establish coverage for an entire file or directory.
            if any(
                argument.startswith("-")
                and argument not in {"-q", "-v", "-vv", "-s", "--import-mode=importlib"}
                for argument in arguments
            ) or any("::" in argument for argument in arguments):
                continue
            paths = [argument for argument in arguments if not argument.startswith("-")]
            for path in paths:
                covered.update(_tests_beneath(
                    posixpath.normpath(posixpath.join(current_directory, path)), tests
                ))
    return covered


def workflow_coverage(tests, rules):
    covered = set()
    for path in sorted((ROOT / ".github/workflows").glob("*.y*ml")):
        workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
        if not workflow or "on" not in workflow:
            continue
        workflow_directory = workflow.get("defaults", {}).get("run", {}).get(
            "working-directory", "."
        )
        for job in workflow.get("jobs", {}).values():
            if job.get("if") in {"false", "${{ false }}"} or job.get("continue-on-error") == "true":
                continue
            job_directory = job.get("defaults", {}).get("run", {}).get(
                "working-directory", workflow_directory
            )
            for row in matrix_rows(job):
                for step in job.get("steps", []):
                    if "if" in step or step.get("continue-on-error") == "true":
                        continue
                    covered.update(command_tests(
                        expand_matrix(step.get("run", ""), row),
                        expand_matrix(step.get("working-directory", job_directory), row),
                        tests,
                        rules,
                    ))
    return covered


def test_python_suite_roots_have_runner():
    tests = tracked_tests()
    assert tests, "No tracked Python test files found"
    rules = make_rules()
    covered = workflow_coverage(tests, rules)
    excluded = set().union(*(_tests_beneath(path, tests) for path in exclusions()))
    missing = tests - covered - excluded - python_suites.read_baseline().keys()
    assert not missing, (
        "Python tests without a workflow-reached runner or reasoned exclusion:\n"
        + "\n".join(sorted(missing))
    )
    for command in (
        'echo "python3 -m pytest tests/daemon"',
        "# python3 -m pytest tests/daemon",
        "python3 -m pytest --collect-only tests/daemon",
        "python3 -m pytest -k one_case tests/daemon",
        "python3 -m pytest tests/daemon || true",
        "make -n test-python-suites",
    ):
        assert not command_tests(command, ".", tests, rules), command
    one_file = "tests/daemon/test_serve_commits_programs_call.py"
    assert command_tests(f"python3 -m pytest {one_file}", ".", tests, rules) == {one_file}


def test_ci_runs_test_python_suites():
    workflow = yaml.load((ROOT / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader)
    assert "main" in workflow["on"]["push"]["branches"]
    assert "pull_request" in workflow["on"]
    tests, rules = tracked_tests(), make_rules()
    assert SUITE_ROOTS == python_suites.SUITE_ROOTS
    all_suites = set().union(*(_tests_beneath(path, tests) for path in SUITE_ROOTS))
    expected = all_suites - python_suites.read_baseline().keys()
    for path in SUITE_ROOTS:
        assert _tests_beneath(path, tests), f"Stale Python suite root: {path}"
    target_coverage = command_tests("make test-python-suites", ".", tests, rules)
    assert expected <= target_coverage, sorted(expected - target_coverage)
    job = workflow["jobs"]["tests-python"]
    assert "if" not in job and job.get("continue-on-error", "false") == "false"
    steps = [step for step in job["steps"] if step.get("run", "").strip() == "make test-python-suites"]
    assert len(steps) == 1, "Main CI must execute make test-python-suites"
    assert "if" not in steps[0] and steps[0].get("continue-on-error", "false") == "false"
    assert expected <= command_tests(steps[0]["run"], ".", tests, rules)
    assert not any(_tests_beneath(path, all_suites) for path in exclusions()), (
        "The suites provisioned by this job must run without exclusions"
    )


def test_exclusion_list_entries_exist():
    tests = tracked_tests()
    roots = {str(PurePosixPath(path).parent) for path in tests}
    recorded = exclusions()
    assert recorded == python_suites.read_exclusions()
    assert recorded, "External suite dependencies must be recorded explicitly"
    for path in recorded:
        assert path in tests or path in roots, f"Not an exact tracked test root or file: {path}"
        assert (ROOT / path).exists(), f"Stale Python exclusion: {path}"
        assert _tests_beneath(path, tests), f"Exclusion contains no tracked tests: {path}"


def test_baseline_entries_are_collectable_files_with_retiring_tasks():
    baseline = python_suites.read_baseline()
    tests, excluded = tracked_tests(), exclusions()
    task_blocks = set(
        re.findall(
            r"^\[task\.([^\]]+)\]$",
            (ROOT / "spec/layerx-merge/spec.kvx").read_text(),
            re.MULTILINE,
        )
    )
    for path, (task, reason) in baseline.items():
        assert path in tests, f"Baseline is not a tracked Python test file: {path}"
        assert (ROOT / path).is_file(), f"Baseline must never name a directory: {path}"
        assert task in task_blocks, f"Missing retiring task: {task}"
        assert len(reason.split()) >= 8, f"Explain the baseline failure: {path}"
        assert any(python_suites.beneath(path, root) for root in SUITE_ROOTS), path
        assert not any(python_suites.beneath(path, root) for root in excluded), path
    if baseline:
        environment = os.environ.copy()
        environment["PYTHONPATH"] = str(ROOT / "services/agent/sdk/python")
        result = subprocess.run(
            [sys.executable, "-m", "pytest", "--collect-only", "-q", *baseline],
            cwd=ROOT,
            env=environment,
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
        assert result.returncode == 0, result.stdout + result.stderr
        collected = {
            line.split("::", 1)[0]
            for line in result.stdout.splitlines()
            if "::" in line
        }
        assert set(baseline) <= collected, sorted(set(baseline) - collected)


def test_python_suite_command_uses_exact_recorded_ignores():
    command = python_suites.pytest_command()
    assert command[:4] == [sys.executable, "-m", "pytest", "-q"]
    baseline = python_suites.read_baseline()
    roots = [root for root in SUITE_ROOTS if root not in baseline]
    assert command[4 : 4 + len(roots)] == roots
    expected = list(exclusions()) + list(baseline)
    assert command[4 + len(roots) :] == [
        f"--ignore={path}" for path in expected
    ]
    environment = os.environ.copy()
    environment["PYTHONPATH"] = str(ROOT / "services/agent/sdk/python")
    result = subprocess.run(
        [*command, "--collect-only"],
        cwd=ROOT,
        env=environment,
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    collected = {
        line.split("::", 1)[0]
        for line in result.stdout.splitlines()
        if "::" in line
    }
    assert collected, "The real suite command must collect active tests"
    assert not collected.intersection(baseline), sorted(collected.intersection(baseline))
    assert not any(
        python_suites.beneath(path, root)
        for path in collected
        for root in exclusions()
    )
