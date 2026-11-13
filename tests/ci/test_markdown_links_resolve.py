"""Resolve tracked Markdown links without relying on untracked checkout files."""

from bisect import bisect_right
from dataclasses import dataclass
from functools import lru_cache
from html import unescape
from html.parser import HTMLParser
from pathlib import Path, PurePosixPath
import posixpath
import re
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
DRIFT_FIXTURES = "tools/jev/tests/fixtures/"
# Imported Hyperfide sources and documentation retain their upstream link policy.
VENDORED_PROJECTS = ("tools/hyperfide/", "docs/hyperfide/")
MARKDOWN_SUFFIXES = {".md", ".markdown", ".mdown", ".mkd", ".mdx"}
GO_IMPORTS = (
    "github.com/Sidiora-Labs/Paxeer-X-Network",
    "github.com/Sidiora-Labs/LayerX-Network",
)
FENCE = re.compile(r"^[ \t]*(?:>[ \t]*)*(`{3,}|~{3,})(.*)$")
DEFINITION = re.compile(r"^[ ]{0,3}\[([^\]\n]+)\]:[ \t]*(?:\n[ \t]*)?", re.M)
BARE_URL = re.compile(r"(?:https?://|//github\.com/|github\.com/)[^\s<>\"']+")
REPOSITORY_NAMES = {"paxeer-monorepo", "paxeer-x-network", "layerx-network"}


@dataclass(frozen=True)
class Link:
    line: int
    target: str


def _blank(text):
    return re.sub(r"[^\n]", " ", text)


def _prose(text):
    """Mask non-link Markdown while preserving source offsets and line numbers."""
    result = []
    fence = None
    for line in text.splitlines(keepends=True):
        match = FENCE.match(line)
        if fence is not None:
            result.append(_blank(line))
            if (
                match
                and match[1][0] == fence[0]
                and len(match[1]) >= len(fence)
                and not match[2].strip()
            ):
                fence = None
        elif match and not (match[1][0] == "`" and "`" in match[2]):
            fence = match[1]
            result.append(_blank(line))
        else:
            result.append(line)
    text = "".join(result)
    text = re.sub(r"<!--[\s\S]*?(?:-->|\Z)", lambda m: _blank(m[0]), text)
    runs = list(re.finditer(r"`+", text))
    next_run = {}
    following = {}
    for index in range(len(runs) - 1, -1, -1):
        size = len(runs[index][0])
        if size in following:
            next_run[index] = following[size]
        following[size] = index
    pieces = []
    start = 0
    index = 0
    while index < len(runs):
        if index not in next_run:
            index += 1
            continue
        closing = next_run[index]
        pieces.append(text[start : runs[index].start()])
        start = runs[closing].end()
        pieces.append(_blank(text[runs[index].start() : start]))
        index = closing + 1
    pieces.append(text[start:])
    return "".join(pieces)


def _closing(text, start, opening, closing):
    depth = 1
    index = start + 1
    while index < len(text):
        if text[index] == "\\":
            index += 2
            continue
        if text[index] == opening:
            depth += 1
        elif text[index] == closing:
            depth -= 1
            if depth == 0:
                return index
        index += 1
    return None


def _bracket_pairs(text):
    stack = []
    pairs = {}
    index = 0
    while index < len(text):
        if text[index] == "\\":
            index += 2
            continue
        if text[index] == "[":
            stack.append(index)
        elif text[index] == "]" and stack:
            pairs[stack.pop()] = index
        index += 1
    return pairs


def _inline_end(text, start):
    index = start
    while index < len(text) and text[index].isspace():
        index += 1
    if index == len(text):
        return False
    if text[index] == ")":
        return True
    quote = text[index]
    if quote not in {"'", '"', "("}:
        return False
    if quote == "(":
        closing = _closing(text, index, "(", ")")
        if closing is None:
            return False
        index = closing + 1
    else:
        index += 1
        while index < len(text) and text[index] != quote:
            index += 2 if text[index] == "\\" else 1
        if index == len(text):
            return False
        index += 1
    while index < len(text) and text[index].isspace():
        index += 1
    return index < len(text) and text[index] == ")"


def _destination(text, start):
    index = start
    while index < len(text) and text[index].isspace():
        index += 1
    if index < len(text) and text[index] == "<":
        end = index + 1
        while end < len(text) and text[end] not in ">\n":
            end += 2 if text[end] == "\\" else 1
        if end < len(text) and text[end] == ">":
            return text[index + 1 : end], end + 1
        return None
    begin = index
    depth = 0
    while index < len(text):
        char = text[index]
        if char == "\\":
            index += 2
            continue
        if char.isspace() or (char == ")" and depth == 0):
            break
        if char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
        index += 1
    if depth:
        return None
    return text[begin:index], index


def _label(text):
    return " ".join(text.split()).casefold()


def _decode(target):
    return unescape(
        re.sub(r"\\([!\"#$%&'()*+,\-./:;<=>?@\[\]\\^_`{|}~])", r"\1", target)
    )


class _HTMLLinks(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.links = []

    def handle_starttag(self, tag, attrs):
        for name, value in attrs:
            if name in {"href", "src"} and value is not None:
                raw = self.get_starttag_text()
                match = re.search(r"\b" + name + r"\s*=", raw, re.I)
                line = self.getpos()[0] + raw.count("\n", 0, match.start())
                self.links.append(Link(line, value))


def _links(text):
    text = _prose(text)
    newlines = [match.start() for match in re.finditer("\n", text)]
    links = []
    definitions = {}
    masked = list(text)
    for match in DEFINITION.finditer(text):
        destination = _destination(text, match.end())
        if destination is None:
            continue
        target, end = destination
        definitions.setdefault(_label(match[1]), target)
        links.append(Link(bisect_right(newlines, match.start()) + 1, _decode(target)))
        masked[match.start() : end] = _blank(text[match.start() : end])
    text = "".join(masked)
    pairs = _bracket_pairs(text)
    index = 0
    while index < len(text):
        if text[index] == "\\":
            index += 2
            continue
        if text[index] != "[":
            index += 1
            continue
        end = pairs.get(index)
        if end is None:
            index += 1
            continue
        following = end + 1
        target = None
        if following < len(text) and text[following] == "(":
            destination = _destination(text, following + 1)
            if destination is not None:
                value, after = destination
                if _inline_end(text, after):
                    target = value
        else:
            label = text[index + 1 : end]
            if following < len(text) and text[following] == "[":
                ref_end = pairs.get(following)
                if ref_end is not None:
                    label = text[following + 1 : ref_end] or label
            target = definitions.get(_label(label))
        if target is not None:
            links.append(Link(bisect_right(newlines, index) + 1, _decode(target)))
        # Advance through the label too: linked images contain another link.
        index += 1
    html = _HTMLLinks()
    html.feed(text)
    links.extend(html.links)
    return sorted(set(links), key=lambda link: (link.line, link.target))


@lru_cache(maxsize=1)
def _inventory():
    result = subprocess.run(
        ["git", "ls-files", "-z", "--full-name"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    tracked = frozenset(name for name in result.stdout.split("\0") if name)
    assert "README.md" in tracked, "Tracked inventory lacks the root README"
    directories = {"."}
    for name in tracked:
        directories.update(str(parent) for parent in PurePosixPath(name).parents)
    return tracked, frozenset(directories)


def _first_party_page(name):
    path = PurePosixPath(name)
    return (
        path.suffix.lower() in MARKDOWN_SUFFIXES
        and path.parts[0] != "spec"
        and "vendor" not in path.parts[:-1]
        and not name.startswith((DRIFT_FIXTURES, *VENDORED_PROJECTS))
    )


@lru_cache(maxsize=1)
def _pages():
    tracked, _ = _inventory()
    return sorted(name for name in tracked if _first_party_page(name))


def _go_import(target):
    return any(
        target == prefix or target.startswith(prefix + "/") for prefix in GO_IMPORTS
    )


def _unresolved(page, links):
    tracked, directories = _inventory()
    failures = []
    for link in links:
        target = link.target
        if _go_import(target):
            continue
        try:
            url = urlsplit(target)
        except ValueError as error:
            failures.append(f"{page}:{link.line}: {target} (invalid URL: {error})")
            continue
        if url.scheme or url.netloc:
            continue
        path = unquote(url.path)
        if not path:
            destination = page
        elif path.startswith("/"):
            destination = posixpath.normpath(path.lstrip("/"))
        else:
            destination = posixpath.normpath(
                posixpath.join(posixpath.dirname(page), path)
            )
        if (
            destination not in tracked and destination not in directories
        ) or not (ROOT / destination).exists():
            failures.append(f"{page}:{link.line}: {target} -> {destination}")
    return failures


def _noncanonical(page, links):
    failures = []
    for link in links:
        if _go_import(link.target):
            continue
        target = unquote(link.target)
        if target.startswith("github.com/"):
            target = "https://" + target
        try:
            url = urlsplit(target)
        except ValueError:
            # Invalid relative URLs are reported by the resolution check.
            continue
        if url.hostname != "github.com":
            continue
        parts = url.path.strip("/").split("/")
        if len(parts) < 2 or parts[0].casefold() != "sidiora-labs":
            continue
        repository = re.sub(r"\.git$", "", parts[1], flags=re.I)
        if repository.casefold() in REPOSITORY_NAMES and (
            parts[0] != "Sidiora-Labs" or repository != "paxeer-monorepo"
        ):
            failures.append(f"{page}:{link.line}: {link.target}")
    return failures


def test_tracked_markdown_relative_links_resolve(capsys):
    failures = []
    checked = 0
    for page in _pages():
        links = _links((ROOT / page).read_text(encoding="utf-8"))
        checked += len(links)
        failures.extend(_unresolved(page, links))
    with capsys.disabled():
        print(f"Scanned {len(_pages())} tracked Markdown pages ({checked} links)")
    assert checked, "No links found in tracked Markdown pages"
    assert not failures, "Unresolved tracked Markdown links:\n" + "\n".join(failures)


def test_tracked_markdown_repository_urls_are_canonical():
    failures = []
    checked = 0
    for page in _pages():
        text = (ROOT / page).read_text(encoding="utf-8")
        links = _links(text)
        prose = _prose(text)
        newlines = [match.start() for match in re.finditer("\n", prose)]
        links.extend(
            Link(
                bisect_right(newlines, match.start()) + 1,
                match[0].rstrip(".,);]"),
            )
            for match in BARE_URL.finditer(prose)
        )
        checked += len(links)
        failures.extend(_noncanonical(page, links))
    assert checked, "No links checked for canonical repository URLs"
    assert not failures, "Noncanonical repository URLs:\n" + "\n".join(
        sorted(set(failures))
    )


def test_markdown_link_guard_reads_committed_syntax_examples():
    page = DRIFT_FIXTURES + "markdown_links/README.md"
    links = _links((ROOT / page).read_text(encoding="utf-8"))
    assert links == [
        Link(3, "../../../../../README.md#readme"),
        Link(4, "/README.md"),
        Link(5, "../../../../../docs/"),
        Link(6, "../../../../../README.md?view=1#readme"),
        Link(7, "../../../../../README.md"),
        Link(8, "../../../../../README.md"),
        Link(9, "../../../../../README.md"),
        Link(10, "../../../../../README.md"),
        Link(11, "../../../../../README.md"),
        Link(12, "#local-anchor"),
        Link(14, "../../../../../README.md"),
        Link(15, "#"),
        Link(26, "https://github.com/Sidiora-Labs/paxeer-monorepo"),
        Link(27, "https://img.shields.io/github/license/Sidiora-Labs/paxeer-monorepo"),
        Link(29, "/README.md?view=1&plain=1#readme"),
        Link(31, "/tools/jev/tests/fixtures/markdown_links/Link%20target.svg"),
        Link(32, "/tools/jev/tests/fixtures/markdown_links/Link target.svg"),
    ]
    assert not _unresolved(page, links)
    assert not _noncanonical(page, links)
    assert page not in _pages()


def test_markdown_link_guard_detects_existing_drift():
    page = DRIFT_FIXTURES + "repair_22_33_705/link-drift.md"
    links = _links((ROOT / page).read_text(encoding="utf-8"))
    assert links == [Link(11, "agent/sdk/python")]
    assert _unresolved(page, links) == [
        f"{page}:11: agent/sdk/python -> {DRIFT_FIXTURES}repair_22_33_705/agent/sdk/python"
    ]


def test_markdown_link_guard_detects_repository_and_badge_drift():
    page = DRIFT_FIXTURES + "markdown_links/repository-drift.md"
    links = _links((ROOT / page).read_text(encoding="utf-8"))
    assert links == [
        Link(3, "https://github.com/Sidiora-Labs/Paxeer-X-Network"),
        Link(
            4,
            "https://img.shields.io/github/actions/workflow/status/"
            "Sidiora-Labs/LayerX-Network/ci.yml",
        ),
        Link(5, "https://github.com/Sidiora-Labs/hyperfide"),
        Link(7, "github.com/Sidiora-Labs/Paxeer-X-Network/core"),
        Link(8, "github.com/Sidiora-Labs/LayerX-Network/sdk"),
        Link(10, "https://github.com/Sidiora-Labs/LayerX-Network/blob/main/README.md"),
        Link(11, "https://github.com/Sidiora-Labs/Paxeer-Monorepo.git"),
        Link(12, "https://github.com/sidiora-labs/paxeer-monorepo/issues"),
        Link(13, "https://github.com/Sidiora-Labs/paxeer-monorepo-tools"),
        Link(14, "https://example.org/github.com/Sidiora-Labs/Paxeer-X-Network"),
    ]
    assert _noncanonical(page, links) == [
        f"{page}:3: https://github.com/Sidiora-Labs/Paxeer-X-Network",
        f"{page}:10: https://github.com/Sidiora-Labs/LayerX-Network/blob/main/README.md",
        f"{page}:11: https://github.com/Sidiora-Labs/Paxeer-Monorepo.git",
        f"{page}:12: https://github.com/sidiora-labs/paxeer-monorepo/issues",
    ]
    assert not _unresolved(page, links)


def test_markdown_link_guard_inventory_covers_first_party_pages():
    tracked, _ = _inventory()
    pages = set(_pages())
    assert pages
    assert {
        "README.md",
        "consensus/spec/README.md",
        "docs/site/paxeer-docs/README.md",
        "docs/platform/content/install.md",
        "services/interop/README.md",
    } <= pages
    excluded = ("spec/", "programs/vendor/", "tools/hyperfide/", "docs/hyperfide/")
    for prefix in excluded:
        candidates = {
            name
            for name in tracked
            if name.startswith(prefix)
            and PurePosixPath(name).suffix.lower() in MARKDOWN_SUFFIXES
        }
        assert candidates, f"No tracked Markdown found under {prefix}"
        assert not pages.intersection(candidates)
    assert not _first_party_page("nested/vendor/package/README.md")
    assert _first_party_page("tools/wasm-tools/README.md")
    assert _first_party_page("docs/site/paxeer-docs/README.md")
