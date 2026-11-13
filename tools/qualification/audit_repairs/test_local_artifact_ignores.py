"""Keep personal permissions and installed dependencies out of the Git index."""

from pathlib import Path
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[3]
TARGETS = (
    (".claude/settings.local.json", ".claude/.gitignore", "/settings.local.json"),
    (
        "apps/explorer/deploy/tools/envs-validator/node_modules/",
        "apps/explorer/deploy/tools/envs-validator/.gitignore",
        "/node_modules/",
    ),
    (
        "programs/sdk/assemblyscript/node_modules/",
        "programs/sdk/assemblyscript/.gitignore",
        "/node_modules/",
    ),
)


@pytest.mark.parametrize("target,ignore_file,pattern", TARGETS)
def test_personal_settings_and_package_trees_are_untracked(target, ignore_file, pattern):
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", target.rstrip("/")],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    assert tracked.stdout == b"", f"Git still tracks paths under {target}"


@pytest.mark.parametrize("target,ignore_file,pattern", TARGETS)
def test_directory_ignore_file_names_and_ignores_target(target, ignore_file, pattern):
    rules = (ROOT / ignore_file).read_text(encoding="utf-8").splitlines()
    assert pattern in rules
    ignored = subprocess.run(
        ["git", "check-ignore", "--no-index", "--verbose", "--", target],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    source, matched_path = ignored.stdout.rstrip("\n").split("\t")
    source_file, line_number, matched_pattern = source.rsplit(":", 2)
    assert source_file == ignore_file
    assert int(line_number) > 0
    assert matched_pattern == pattern
    assert matched_path == target
