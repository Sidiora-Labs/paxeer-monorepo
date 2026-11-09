"""Prove the vendored tree relocation and its documentation links on disk."""

import hashlib
from html import unescape
from html.parser import HTMLParser
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
PROJECT = ROOT / "tools/hyperfide"
DOCS = ROOT / "docs/hyperfide"
BASE = "2e0a668a793a7b5c0469b44950808547467f017e"
# Published source-control and issue work has advanced the relocated code.
LIVE_SOURCE = "2910889b640274b1b6f2bd14a2fba25019093bb2"
REMOVED = {
    "CHANGELOG.md",
    "CODEMAP.md",
    "WHY.md",
    "MAINTAINERS.md",
    "CODE_OF_CONDUCT.md",
    "CONTRIBUTING.md",
    "SECURITY.md",
    "CITATION.cff",
}
MARKDOWN = sorted(DOCS.rglob("*.md")) + [
    PROJECT / "README.md",
    PROJECT / "AGENTS.md",
]
DESTINATION = r"(?:<([^>\n]*)>|((?:\\.|[^\s()\\]|\((?:\\.|[^()\\])*\))*))"
INLINE_LINK = re.compile(r"\]\(\s*" + DESTINATION)
REFERENCE_LINK = re.compile(r"^ {0,3}\[[^\]\n]+\]:\s*" + DESTINATION, re.M)


def git(*args):
    return subprocess.run(
        ["git", "-C", str(ROOT), *args], check=True, capture_output=True
    ).stdout


class HTMLLinks(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        for name, value in attrs:
            if name in {"href", "src"} and value is not None:
                self.links.append(value)


def markdown_links(text):
    """Read inline/image links, reference destinations and embedded HTML links."""
    lines = []
    fence = None
    for line in text.splitlines(keepends=True):
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if fence is None and marker:
            fence = marker[1]
        elif fence is not None:
            closing = r" {0,3}" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}\s*"
            if re.fullmatch(closing, line):
                fence = None
        else:
            lines.append(line)
    prose = "".join(lines)
    for pattern in (INLINE_LINK, REFERENCE_LINK):
        for match in pattern.finditer(prose):
            yield match[1] if match[1] is not None else match[2]
    html = HTMLLinks()
    html.feed(prose)
    yield from html.links


def test_vendored_project_has_the_required_layout():
    assert not (ROOT / "addiqate").exists()
    assert PROJECT.is_dir()
    for name in (
        "README.md",
        "AGENTS.md",
        "LICENSE",
        "GOTCHA.kvx",
        "Makefile",
        "package.json",
        "pnpm-workspace.yaml",
    ):
        assert (PROJECT / name).is_file(), name
    assert (PROJECT / "licenses").is_dir()
    assert not (PROJECT / "docs").exists()
    for name in REMOVED:
        assert not (PROJECT / name).exists(), name


def test_documentation_keeps_all_104_files():
    original = git(
        "ls-tree", "-r", "--name-only", BASE, "--", "addiqate/hyperfide/docs/"
    )
    expected = {
        path.removeprefix("addiqate/hyperfide/docs/")
        for path in original.decode().splitlines()
    }
    actual = {p.relative_to(DOCS).as_posix() for p in DOCS.rglob("*") if p.is_file()}
    assert len(expected) == 104
    assert actual == expected


def test_code_specs_dot_directories_and_assets_are_preserved():
    inventory = git("ls-tree", "-rz", BASE, "--", "addiqate/hyperfide/")
    live_inventory = git("ls-tree", "-rz", LIVE_SOURCE, "--", "tools/hyperfide/")
    live = {}
    for entry in live_inventory.split(b"\0"):
        if entry:
            metadata, path = entry.split(b"\t", 1)
            live[path.decode().removeprefix("tools/hyperfide/")] = metadata.decode().split()
    checked = 0
    for entry in inventory.split(b"\0"):
        if not entry:
            continue
        metadata, path = entry.split(b"\t", 1)
        mode, kind, oid = metadata.decode().split()
        relative = path.decode().removeprefix("addiqate/hyperfide/")
        if relative.startswith("docs/") or relative in REMOVED:
            continue
        if relative in live:
            mode, kind, oid = live[relative]
        # An absent relocation artifact still has to match its original bytes;
        # the newer source snapshot never authorizes dropping it.
        moved = PROJECT / relative
        assert moved.is_file() or moved.is_symlink(), relative
        if relative in {"README.md", "AGENTS.md"}:
            continue
        assert kind == "blob", relative
        if mode == "120000":
            content = moved.readlink().as_posix().encode()
            assert moved.is_symlink(), relative
        else:
            content = moved.read_bytes()
            assert bool(moved.stat().st_mode & 0o111) == (mode == "100755"), relative
        if relative == ".gitignore":
            assert b".codex\n!.codex/\n.codex/*\n!.codex/hooks.json\n" in content
        actual = hashlib.sha1(
            b"blob " + str(len(content)).encode() + b"\0" + content
        ).hexdigest()
        assert actual == oid, relative
        checked += 1
    assert checked > 10_000


def test_restored_codex_hook_is_kept_by_ordinary_adds():
    path = "tools/hyperfide/.codex/hooks.json"
    assert (ROOT / path).read_bytes() == git(
        "show", "ebcb190287:addiqate/hyperfide/.codex/hooks.json"
    )
    result = subprocess.run(
        ["git", "-C", str(ROOT), "check-ignore", "--no-index", "--quiet", "--", path],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1, result.stderr


@pytest.mark.parametrize(
    "document", MARKDOWN, ids=lambda p: p.relative_to(ROOT).as_posix()
)
def test_relative_markdown_links_resolve(document):
    assert document.is_file()
    broken = []
    for link in markdown_links(document.read_text()):
        link = re.sub(
            r"\\([!\"#$%&'()*+,\-./:;<=>?@\[\]\\^_`{|}~])", r"\1", unescape(link)
        )
        target = urlsplit(link)
        if target.scheme or target.netloc or not target.path:
            continue
        resolved = document.parent / unquote(target.path)
        if not resolved.exists():
            broken.append(f"{link} -> {resolved}")
    assert not broken, "\n".join(broken)


def test_tools_index_contains_the_project_and_documentation_row():
    rows = (ROOT / "tools/README.md").read_text().splitlines()
    row = next(line for line in rows if line.startswith("| [`hyperfide/`]"))
    assert "(hyperfide/README.md)" in row
    assert "(../docs/hyperfide/DOCUMENTATION.md)" in row
    for link in markdown_links(row):
        assert (ROOT / "tools" / link).exists(), link


def test_tracked_candidate_has_no_old_directory_references():
    # The index is the publishable candidate. Runner-owned working-tree bookkeeping
    # (including its temporary .cgignore) is deliberately not published with it.
    result = subprocess.run(
        [
            "git", "-C", str(ROOT), "grep", "--cached", "-l", "-F",
            "addiqate/", "--", ".",
            ":(exclude)spec/**",
            ":(exclude)**/spec/**",
            ":(exclude)GOTCHA.kvx",
            ":(exclude)**/GOTCHA.kvx",
            ":(exclude)tools/qualification/audit_repairs/test_repair_22_36_1.py",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode in (0, 1), result.stderr
    assert result.returncode == 1, result.stdout
