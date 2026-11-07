"""Keep the root layout inventory and package-manager lockfile documented."""

from collections import Counter
import json
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
LAYOUT_HEADING = "## Root layout\n"
LOCKFILES = {
    "npm": "package-lock.json",
    "pnpm": "pnpm-lock.yaml",
    "yarn": "yarn.lock",
    "bun": "bun.lock",
}


def _tracked_paths():
    output = subprocess.run(
        ["git", "ls-files", "--cached", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    ).stdout
    paths = {
        name.decode("utf-8", errors="surrogateescape")
        for name in output.split(b"\0") if name
    }
    assert paths, "Git returned no tracked files"
    return paths


def test_every_top_level_entry_documented():
    tracked = _tracked_paths()
    expected = {
        path.partition("/")[0] + ("/" if "/" in path else "")
        for path in tracked
    }
    document = (ROOT / "docs/MONOREPO.md").read_text(encoding="utf-8")
    assert document.count(LAYOUT_HEADING) == 1, "Expected one root layout section"
    section = document.split(LAYOUT_HEADING, 1)[1].split("\n## ", 1)[0]
    rows = re.findall(
        r"^\| `([^`]+)` \| (Directory|File) \| ([^\n|]+) \|$",
        section,
        flags=re.MULTILINE,
    )
    counts = Counter(entry for entry, _, _ in rows)
    assert set(counts) == expected, (
        f"Undocumented root entries: {sorted(expected - counts.keys())}; "
        f"Stale root entries: {sorted(counts.keys() - expected)}"
    )
    assert all(count == 1 for count in counts.values()), (
        f"Duplicate inventory rows: {dict(counts)}"
    )
    for entry, kind, purpose in rows:
        assert kind == ("Directory" if entry.endswith("/") else "File"), entry
        assert purpose.strip(), f"Missing purpose for {entry}"
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    assert re.search(r"\[[^\]\n]+\]\(docs/MONOREPO\.md#root-layout\)", readme), (
        "README must link to the complete root layout table"
    )


def test_root_package_manager_matches_lockfile():
    tracked = _tracked_paths()
    assert "package.json" in tracked, "Root manifest must be tracked"
    manifest = json.loads((ROOT / "package.json").read_text(encoding="utf-8"))
    declaration = manifest["packageManager"]
    assert isinstance(declaration, str), "packageManager must be a string"
    manager, separator, version = declaration.partition("@")
    assert separator and version.strip(), "Root package manager must be pinned"
    assert manager == "npm", "The root workspace declares npm"
    lockfile = LOCKFILES[manager]
    assert lockfile in tracked, (
        f"Declared package manager {declaration} requires tracked {lockfile}"
    )
    assert (ROOT / lockfile).is_file(), f"Missing root lockfile: {lockfile}"
