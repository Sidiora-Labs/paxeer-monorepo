"""Offline checks for the consolidated documentation index and its links."""

from html import unescape
from html.parser import HTMLParser
from pathlib import Path
import re
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
INDEX = ROOT / "docs/README.md"
TREES = (
    "hyperfide/",
    "platform/",
    "wallet/",
    "explorer/",
    "launchpad/",
    "perps/",
    "agent/",
    "interop/",
)
DESTINATION = r"(?:<([^>\n]*)>|((?:\\.|[^\s()\\]|\((?:\\.|[^()\\])*\))*))"
INLINE_LINK = re.compile(r"\]\(\s*" + DESTINATION)
REFERENCE_LINK = re.compile(r"^ {0,3}\[[^\]\n]+\]:\s*" + DESTINATION, re.M)


class HTMLLinks(HTMLParser):
    def __init__(self):
        super().__init__()
        self.targets = []

    def handle_starttag(self, tag, attrs):
        for name, value in attrs:
            if name in {"href", "src"} and value is not None:
                self.targets.append(value)


def markdown_targets(source):
    """Read inline, image, reference and HTML targets outside fenced examples."""
    lines = []
    fence = None
    for line in source.splitlines():
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if marker:
            ticks, suffix = marker.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None:
            lines.append(line)
    prose = "\n".join(lines)
    for pattern in (INLINE_LINK, REFERENCE_LINK):
        for match in pattern.finditer(prose):
            yield match[1] if match[1] is not None else match[2]
    html = HTMLLinks()
    html.feed(prose)
    yield from html.targets


def index_rows():
    rows = []
    for line in INDEX.read_text(encoding="utf-8").splitlines():
        if line.startswith("|") and line.endswith("|"):
            cells = [cell.strip() for cell in line[1:-1].split("|")]
            if len(cells) == 5 and cells[0] not in {"System", "---"}:
                rows.append(cells)
    return rows


def documentation_directory_section():
    text = INDEX.read_text(encoding="utf-8")
    return text.split("Other documentation directories:", 1)[1].split(
        "The [monorepo layout]", 1
    )[0]


@pytest.mark.parametrize("tree", TREES)
def test_consolidated_tree_has_one_described_index_row(tree):
    section = documentation_directory_section()
    targets = list(markdown_targets(section))
    assert targets.count(tree) == 1, f"Expected exactly one index row for docs/{tree}"
    assert (INDEX.parent / tree).is_dir(), f"Missing documentation tree: {tree}"
    assert re.search(r"\[[^]]+\]\(" + re.escape(tree) + r"\)", section)


def test_wallet_row_links_to_app_and_service_documentation():
    text = INDEX.read_text(encoding="utf-8")
    assert list(markdown_targets(documentation_directory_section())).count("wallet/") == 1
    for target in ("wallet/app/operations/runbooks.md", "wallet/service/cutover.md"):
        assert f"]({target})" in text, f"Wallet documentation link is missing: {target}"
        assert (INDEX.parent / target).is_file(), f"Missing wallet page: {target}"


def test_every_relative_index_link_resolves():
    targets = list(markdown_targets(INDEX.read_text(encoding="utf-8")))
    assert targets, "The documentation index has no links"
    relative_targets = []
    for destination in targets:
        destination = re.sub(
            r"\\([!\"#$%&'()*+,\-./:;<=>?@\[\]\\^_`{|}~])",
            r"\1",
            unescape(destination),
        )
        url = urlsplit(destination)
        if url.scheme or url.netloc:
            continue
        assert not url.path.startswith("/"), f"Index link must be relative: {destination}"
        relative_targets.append(destination)
        target = INDEX.parent / unquote(url.path) if url.path else INDEX
        assert target.exists(), f"Broken relative index link: {destination} -> {target}"
    assert relative_targets, "The documentation index has no relative links"
