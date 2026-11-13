"""Keep retired settlement pages separate from current block finality."""

import re
from html import unescape
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
FINALITY_PAGES = (
    ROOT / "docs/wiki/Finality.md",
    ROOT / "docs/site/paxeer-docs/src/app/finality/page.tsx",
)
LEGACY_TERMS = re.compile(
    r"\b(?:L[0-4]|ladder|guarantors?|quorum|checkpoint|bonded|bonds|"
    r"challenge window|withdrawal limits)\b",
    re.IGNORECASE,
)


def document_text(path):
    text = path.read_text(encoding="utf-8")
    if path.suffix != ".tsx":
        return text
    text = re.sub(
        r"<h([1-6])(?:\s[^>]*)?>(.*?)</h\1>",
        lambda match: "\n\n" + "#" * int(match[1]) + " " + match[2] + "\n\n",
        text,
        flags=re.DOTALL,
    )
    text = re.sub(r"</?code(?:\s[^>]*)?>", "`", text)
    text = re.sub(
        r"<(?:Link|a)\b[^>]*href=[\"']([^\"']+)[\"'][^>]*>(.*?)</(?:Link|a)>",
        lambda match: "[" + match[2] + "](" + match[1] + ")",
        text,
        flags=re.DOTALL,
    )
    text = re.sub(r"</(?:p|div|section|ul|ol)>", "\n\n", text)
    text = re.sub(r"<[^>]+>", "", text)
    return unescape(text)


def assert_retired_notice(page):
    text = document_text(page)
    heading_match = re.search(r"(?m)^# (.+)$", text)
    assert heading_match, page
    heading = "# " + heading_match[1].lower()
    assert heading.startswith("# finality"), page
    assert "retired" in heading, page
    prose = " ".join(text.split())
    assert re.search(r"CometBFT block finality", prose), page
    assert re.search(r"chain (?:ID )?`?125\b`?", prose), page
    assert re.search(r"[Tt]he proposer (?:executes|is the executor)", prose), page
    assert re.search(r"receipt is final when its block is committed", prose), page
    links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", text)
    network_guide = ROOT / "docs/paxeer-network.md"
    assert network_guide.is_file()
    assert any((page.parent / link).resolve() == network_guide for link in links), page


def test_wiki_finality_page_marked_retired():
    assert_retired_notice(FINALITY_PAGES[0])
    home = (ROOT / "docs/wiki/Home.md").read_text(encoding="utf-8")
    labels = re.findall(r"\[([^\]]+)\]\(Finality\.md\)", home)
    assert labels, "The wiki home must retain a link to the retirement notice"
    assert all("retired" in label.lower() for label in labels), labels


def test_site_finality_page_marked_retired():
    assert_retired_notice(FINALITY_PAGES[1])
    navigation = (ROOT / "docs/site/paxeer-docs/src/components/nav.ts").read_text(encoding="utf-8")
    assert "href: '/finality'" in navigation


@pytest.mark.parametrize("page", FINALITY_PAGES, ids=("wiki", "site"))
def test_finality_pages_present_no_ladder_as_current(page):
    text = document_text(page)
    statements = re.split(r"[.!?]\s+|\n\s*\n", text)
    retirement_notices = []
    for statement in statements:
        if LEGACY_TERMS.search(statement):
            assert re.search(r"\b(?:is|are|was|were) retired\b", statement), (
                f"{page.relative_to(ROOT)} presents retired settlement as current: "
                f"{statement}"
            )
            retirement_notices.append(statement)
    assert retirement_notices, f"{page.relative_to(ROOT)} must identify what retired"
    assert not re.search(r"(?m)^\s*\|\s*L[0-4]\s*\|", text), page
