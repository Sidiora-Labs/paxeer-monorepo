"""Keep the wiki's retired commitment contract aligned with the site guide."""

from html import unescape
from pathlib import Path
import re


REPO_ROOT = Path(__file__).resolve().parents[2]
SITE_PATH = REPO_ROOT / "docs/site/paxeer-docs/src/app/finality/page.tsx"
WIKI_PATH = REPO_ROOT / "docs/wiki/CommitmentLevels.md"
API_PATH = REPO_ROOT / "docs/wiki/PublicAPI.md"
HOME_PATH = REPO_ROOT / "docs/wiki/Home.md"


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


def test_wiki_commitment_levels_matches_site_retired_text():
    site = document_text(SITE_PATH)
    wiki = WIKI_PATH.read_text(encoding="utf-8")
    home = HOME_PATH.read_text(encoding="utf-8")
    for path, text in ((SITE_PATH, site), (WIKI_PATH, wiki)):
        compact = " ".join(text.split())
        for rung in ("submitted", "receipted", "committed"):
            assert f"`{rung}`" in compact, f"Current rung missing from {path}"

    assert wiki.startswith("# Commitment levels\n")
    assert "[Commitment levels](../site/paxeer-docs/src/app/finality/page.tsx)" in wiki
    labels = re.findall(r"\[([^\]]+)\]\(CommitmentLevels\.md\)", home)
    assert labels == ["Commitment levels (retired)"] * 2


def test_wiki_commitment_levels_names_no_live_lx_method():
    api = API_PATH.read_text(encoding="utf-8")
    retired_methods = {"lx_sendActivity", "lx_getReceipt", "lx_subscribe"}
    named_methods = set(re.findall(r"\blx_[A-Za-z][A-Za-z0-9_]*\b", api))
    assert named_methods == retired_methods

    for paragraph in api.split("\n\n"):
        if re.search(r"\blx_", paragraph):
            assert "API" in paragraph and "retired" in paragraph
            assert "`-32601`" in paragraph
            assert "gateway" in paragraph
