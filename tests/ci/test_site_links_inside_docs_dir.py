"""Keep MkDocs' relative page links inside its tracked documentation tree."""

from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs/site/docs"
INLINE_LINK = re.compile(r"\]\(\s*(?:<([^>\n]*)>|([^\s)]+))")
REFERENCE_LINK = re.compile(
    r"^[ \t]{0,3}\[[^\]\n]+\]:[ \t]*(?:<([^>\n]*)>|(\S+))", re.MULTILINE
)
HTML_LINK = re.compile(
    r"<(?:a|img)\b[^>]*?\b(?:href|src)\s*=\s*"
    r"(?:\"([^\"]*)\"|'([^']*)'|([^\s>]+))",
    re.IGNORECASE,
)


def _relative_links():
    pages = sorted(DOCS.rglob("*.md"))
    assert pages, "No MkDocs pages found"
    links = []
    for page in pages:
        text = page.read_text(encoding="utf-8")
        for pattern in (INLINE_LINK, REFERENCE_LINK, HTML_LINK):
            for match in pattern.finditer(text):
                target = next(value for value in match.groups() if value is not None)
                url = urlsplit(target)
                if url.scheme or url.netloc or url.path.startswith("/"):
                    continue
                destination = (
                    page.parent / unquote(url.path) if url.path else page
                ).resolve()
                line = text.count("\n", 0, match.start()) + 1
                source = f"{page.relative_to(ROOT)}:{line}: {target}"
                links.append((source, destination))
    assert links, "No relative MkDocs links checked"
    return links


def test_site_relative_links_stay_inside_docs_dir():
    outside = [
        source
        for source, destination in _relative_links()
        if not destination.is_relative_to(DOCS.resolve())
    ]
    assert not outside, "Relative links outside the MkDocs docs directory:\n" + "\n".join(
        outside
    )


def test_site_relative_links_resolve():
    result = subprocess.run(
        ["git", "ls-files", "--full-name", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    tracked = {ROOT / path for path in result.stdout.split("\0") if path}
    assert tracked, "Git reported no tracked files"
    unresolved = [
        source
        for source, destination in _relative_links()
        if not destination.is_file() or destination not in tracked
    ]
    assert not unresolved, "Relative links without a tracked file target:\n" + "\n".join(
        unresolved
    )
