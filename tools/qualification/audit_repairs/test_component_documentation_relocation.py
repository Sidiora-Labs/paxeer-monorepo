"""Offline checks for the four component documents moved into docs/."""

from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
MOVES = {
    "apps/launchpad/production_checklist.md": "docs/launchpad/production-checklist.md",
    "apps/perps/data_api.md": "docs/perps/data-api.md",
    "services/agent/COMPATIBILITY.md": "docs/agent/COMPATIBILITY.md",
    "services/interop/PORTABILITY.md": "docs/interop/PORTABILITY.md",
}


def markdown_targets(source):
    """Read inline/image destinations and reference definitions outside fences."""
    lines = []
    fence = None
    for line in source.splitlines():
        delimiter = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if delimiter:
            ticks, suffix = delimiter.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None:
            lines.append(line)
    prose = "\n".join(lines)
    destination = r"(?:<([^>\n]*)>|((?:[^()\s\\]|\\.|\([^()]*\))+))"
    inline = r"\]\(\s*" + destination + r"(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^()]*\)))?\s*\)"
    reference = r"^ {0,3}\[[^\]]+\]:\s*" + destination
    for pattern in (inline, reference):
        for match in re.finditer(pattern, prose, re.MULTILINE):
            yield match.group(1) if match.group(1) is not None else match.group(2)


@pytest.mark.parametrize("old,new", MOVES.items())
def test_component_document_moved(old, new):
    assert not (ROOT / old).exists(), f"Old document remains: {old}"
    document = ROOT / new
    assert document.is_file(), f"Moved document missing: {new}"
    assert document.read_text(encoding="utf-8").strip(), f"Empty document: {new}"


@pytest.mark.parametrize("new", MOVES.values())
def test_moved_document_relative_markdown_links_resolve(new):
    document = ROOT / new
    for destination in markdown_targets(document.read_text(encoding="utf-8")):
        url = urlsplit(destination)
        if url.scheme or url.netloc or url.path.startswith("/"):
            continue
        target = document.parent / unquote(url.path) if url.path else document
        assert target.exists(), f"{new}: broken relative link {destination}"
        if url.fragment and target.suffix == ".md":
            headings = re.findall(
                r"^ {0,3}#{1,6}\s+(.+?)\s*#*\s*$",
                target.read_text(encoding="utf-8"),
                re.MULTILINE,
            )
            anchors = {
                re.sub(r"[^\w\- ]", "", heading.lower()).replace(" ", "-")
                for heading in headings
            }
            assert unquote(url.fragment) in anchors, (
                f"{new}: missing Markdown heading in {destination}"
            )


def test_no_tracked_references_to_old_document_paths():
    command = ["git", "grep", "--files-with-matches", "--fixed-strings"]
    for old in MOVES:
        command.extend(["-e", old])
    command.extend(
        [
            "--",
            ".",
            ":(exclude)spec/**",
            ":(exclude)GOTCHA.kvx",
            # Decision 2670 preserves historical paths in repair records.
            ":(glob,exclude)**/repair_*.kvx",
            ":(glob,exclude)**/repair-*.kvx",
            ":(glob,exclude)tools/qualification/audit_repairs/**/*.kvx",
            ":(glob,exclude)tools/bringup/**/*.kvx",
            ":(exclude)" + Path(__file__).relative_to(ROOT).as_posix(),
        ]
    )
    result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, check=False)
    assert result.returncode == 1, (
        "Tracked files still contain old document paths, or git grep failed:\n"
        + result.stdout
        + result.stderr
    )
