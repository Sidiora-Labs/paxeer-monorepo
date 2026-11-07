"""Keep tracked artifacts within the recorded file size budgets."""

import os
import subprocess
from pathlib import Path

import pytest


REPO_ROOT = Path(__file__).resolve().parents[2]
ALLOWLIST_PATH = Path(__file__).parent / "data" / "large_file_allowlist.txt"
MAX_FILE_BYTES = 2_000_000


@pytest.fixture(scope="module")
def tracked_paths() -> set[str]:
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--full-name", "-z"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
    )
    paths = {os.fsdecode(path) for path in result.stdout.split(b"\0") if path}
    assert paths, "Git reported no tracked files"
    return paths


@pytest.fixture(scope="module")
def allowlist() -> dict[str, int]:
    entries = {}
    for line_number, line in enumerate(
        ALLOWLIST_PATH.read_text(encoding="utf-8").splitlines(), start=1
    ):
        if not line.strip() or line.startswith("#"):
            continue
        location = f"{ALLOWLIST_PATH}:{line_number}"
        fields = line.split("\t", 2)
        assert len(fields) == 3, f"{location}: expected path, byte size and reason"
        name, recorded_size, reason = fields
        path = Path(name)
        assert name and not path.is_absolute() and ".." not in path.parts, (
            f"{location}: path must be relative to the repository"
        )
        assert path.as_posix() == name, f"{location}: path must be canonical"
        assert name not in entries, f"{location}: duplicate entry for {name}"
        assert reason.strip(), f"{location}: a reason is required"
        size = int(recorded_size)
        assert size > 0, f"{location}: recorded byte size must be positive"
        entries[name] = size
    assert entries, "The large-file allowlist must not be empty"
    return entries


def test_no_unlisted_tracked_file_over_2mb(
    tracked_paths: set[str], allowlist: dict[str, int]
):
    offenders = []
    for name in sorted(tracked_paths):
        size = (REPO_ROOT / name).lstat().st_size
        if size > MAX_FILE_BYTES and name not in allowlist:
            offenders.append(f"{name}: {size} bytes")
    assert not offenders, (
        f"Tracked files over {MAX_FILE_BYTES} bytes need an allowlist entry "
        "with a recorded byte size and reason:\n" + "\n".join(offenders)
    )


def test_allowlisted_files_have_not_grown(allowlist: dict[str, int]):
    offenders = []
    for name, recorded_size in allowlist.items():
        size = (REPO_ROOT / name).lstat().st_size
        if size * 10 > recorded_size * 11:
            offenders.append(
                f"{name}: {size} bytes; recorded {recorded_size} bytes; "
                f"maximum {recorded_size * 11 // 10} bytes"
            )
    assert not offenders, (
        "Allowlisted files grew by more than ten percent:\n" + "\n".join(offenders)
    )


def test_allowlist_entries_exist(tracked_paths: set[str], allowlist: dict[str, int]):
    offenders = [
        name
        for name in allowlist
        if name not in tracked_paths or not (REPO_ROOT / name).is_file()
    ]
    assert not offenders, (
        "Allowlist entries must name existing tracked files:\n" + "\n".join(offenders)
    )
