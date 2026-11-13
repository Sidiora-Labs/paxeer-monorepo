"""Keep Python bytecode out of the repository's tracked inventory."""

from pathlib import Path, PurePosixPath
import subprocess


ROOT = Path(__file__).resolve().parents[3]


def test_tracked_inventory_contains_no_python_cache_directory():
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    paths = result.stdout.decode("utf-8", errors="surrogateescape").split("\0")
    caches = [path for path in paths if "__pycache__" in PurePosixPath(path).parts]
    assert not caches, f"Tracked Python bytecode caches: {caches}"


def test_root_gitignore_contains_and_applies_python_bytecode_rules():
    lines = (ROOT / ".gitignore").read_text(encoding="utf-8").splitlines()
    assert "__pycache__/" in lines
    assert "*.pyc" in lines

    paths = [
        "__pycache__/root.pyc",
        "tools/qualification/audit_repairs/__pycache__/nested.pyc",
        "root.pyc",
        "tools/qualification/audit_repairs/nested.pyc",
    ]
    result = subprocess.run(
        ["git", "check-ignore", "--no-index", "--stdin"],
        cwd=ROOT,
        input="\n".join(paths) + "\n",
        check=True,
        capture_output=True,
        text=True,
    )
    assert result.stdout.splitlines() == paths
