"""Resolve documentation routes and public assets within the static site."""

from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "docs/site/paxeer-docs"
DOCS = SITE / "src/app"
PUBLIC = SITE / "public"
LINK = re.compile(r'''\b(href|src)\s*(?:=|:)\s*["']([^"']+)["']''')


def _relative_links():
    pages = sorted(DOCS.rglob("page.tsx"))
    assert pages, "No documentation pages found"
    navigation = SITE / "src/components/nav.ts"
    assert navigation.is_file(), "Site navigation must exist"
    links = []
    for page in [*pages, navigation]:
        text = page.read_text(encoding="utf-8")
        for match in LINK.finditer(text):
            attribute, target = match.groups()
            url = urlsplit(target)
            if url.scheme or url.netloc or not url.path:
                continue
            path = unquote(url.path)
            if attribute == "href":
                route = path.lstrip("/") if path.startswith("/") else str(
                    (page.parent / path).relative_to(DOCS)
                )
                destination = (DOCS / route / "page.tsx").resolve()
                if Path(path).suffix:
                    destination = (PUBLIC / path.lstrip("/")).resolve()
            else:
                destination = (PUBLIC / path.lstrip("/")).resolve()
            line = text.count("\n", 0, match.start()) + 1
            source = f"{page.relative_to(ROOT)}:{line}: {target}"
            links.append((source, destination))
    assert links, "No documentation links checked"
    return links


def test_site_relative_links_stay_inside_docs_dir():
    outside = [
        source
        for source, destination in _relative_links()
        if not (
            destination.is_relative_to(DOCS.resolve())
            or destination.is_relative_to(PUBLIC.resolve())
        )
    ]
    assert not outside, "Links outside the static site:\n" + "\n".join(outside)


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
    assert not unresolved, "Links without a tracked file target:\n" + "\n".join(unresolved)
