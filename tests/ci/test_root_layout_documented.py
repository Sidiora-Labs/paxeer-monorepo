"""Keep the root layout inventory and package-manager lockfile documented."""

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
    document = (ROOT / "docs/MONOREPO.md").read_text(encoding="utf-8")
    assert document.count(LAYOUT_HEADING) == 1, "Expected one root layout section"
    section = document.split(LAYOUT_HEADING, 1)[1].split("\n## ", 1)[0]
    rows = re.findall(r"^\| (.+) \| ([^\n|]+) \|$", section, re.MULTILINE)
    assert rows[0] == ("Path", "Contents")
    paths = []
    for links, purpose in rows[1:]:
        if re.fullmatch(r"[-: ]+", links):
            continue
        assert purpose.strip(), links
        targets = re.findall(r"\[[^\]]+\]\(([^)]+)\)", links)
        assert targets, links
        for target in targets:
            path = (ROOT / "docs" / target).resolve()
            assert path.is_dir(), target
            relative = path.relative_to(ROOT).as_posix()
            assert any(name.startswith(relative + "/") for name in tracked), target
            paths.append(relative)
    assert len(paths) == len(set(paths))
    assert {
        "src", "include", "node", "programs", "services", "sdk", "apps", "tests"
    } <= set(paths)
    reference = ROOT / "docs/readme/NETWORK.md"
    network = reference.read_text(encoding="utf-8")
    link, = re.findall(r"\[[^\]\n]+\]\(([^)]+MONOREPO\.md#root-layout)\)", network)
    assert (
        (reference.parent / link.split("#", 1)[0]).resolve()
        == ROOT / "docs/MONOREPO.md"
    )
    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    assert re.search(r"\[[^\]\n]+\]\(docs/readme/NETWORK\.md\)", readme)


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
