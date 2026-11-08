"""Run the retained Python suites with their recorded dependency and repair lists."""

from pathlib import Path, PurePosixPath
import posixpath
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
EXCLUSIONS = ROOT / "tests/ci/data/python_suite_exclusions.txt"
BASELINE = ROOT / "tests/ci/data/python_suite_baseline.txt"
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


def read_entries(source, fields):
    entries = {}
    for number, line in enumerate(source.read_text().splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        values = tuple(value.strip() for value in line.split("|"))
        if len(values) != fields or not all(values):
            raise ValueError(f"{source}:{number}: expected {fields} nonempty fields")
        path = values[0]
        if (
            path in entries
            or PurePosixPath(path).is_absolute()
            or ".." in PurePosixPath(path).parts
            or any(character in path for character in "*?[]")
            or posixpath.normpath(path) != path
            or not (ROOT / path).exists()
        ):
            raise ValueError(
                f"{source}:{number}: duplicate, invalid or missing path: {path}"
            )
        entries[path] = values[1:]
    return entries


def read_exclusions():
    return {path: reason for path, (reason,) in read_entries(EXCLUSIONS, 2).items()}


def read_baseline():
    entries = read_entries(BASELINE, 3)
    for path, (task, _reason) in entries.items():
        if not (ROOT / path).is_file() or not path.endswith(".py"):
            raise ValueError(f"Baseline entry must name a Python file: {path}")
        if not re.fullmatch(r"\d+(?:\.\d+)+", task):
            raise ValueError(f"Baseline entry must name its retiring task: {path}")
    return entries


def beneath(path, root):
    return root == "." or path == root or path.startswith(root.rstrip("/") + "/")


def pytest_command():
    excluded, baseline = read_exclusions(), read_baseline()
    for path in baseline:
        if any(beneath(path, root) for root in excluded):
            raise ValueError(f"Baseline entry also appears in exclusions: {path}")
        if not any(beneath(path, root) for root in SUITE_ROOTS):
            raise ValueError(f"Baseline entry is outside the suite roots: {path}")
    return [
        sys.executable,
        "-m",
        "pytest",
        "-q",
        *(root for root in SUITE_ROOTS if root not in baseline),
        *(f"--ignore={path}" for path in excluded),
        *(f"--ignore={path}" for path in baseline),
    ]


def main():
    command = pytest_command()
    for path, (task, reason) in read_baseline().items():
        print(f"Python suite baseline: {path} | {task} | {reason}", flush=True)
    return subprocess.run(command, cwd=ROOT, check=False).returncode


if __name__ == "__main__":
    raise SystemExit(main())
