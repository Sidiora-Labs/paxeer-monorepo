"""Keep the wiki's retired commitment contract aligned with the site guide."""

from html import unescape
from pathlib import Path
import re


REPO_ROOT = Path(__file__).resolve().parents[2]
SITE_PATH = REPO_ROOT / "docs/site/paxeer-docs/src/app/finality/page.tsx"
WIKI_PATH = REPO_ROOT / "docs/wiki/CommitmentLevels.md"
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
    retired_text = (
        "The former `executed`, `batched` and `finalised` submission contract "
        "is retired. `lx_sendActivity`, `lx_getActivityStatus`, `lx_getReceipt` "
        "and the old `lx_` proof/status methods are retired; the node and "
        "gateway return `-32601` (Method not found) for them. "
        "The separate sequencer service and guarantor finality model are retired."
    )

    for path, text in ((SITE_PATH, site), (WIKI_PATH, wiki)):
        paragraphs = [" ".join(part.split()) for part in text.split("\n\n")]
        assert retired_text in paragraphs, f"Missing shared retirement notice: {path}"

    assert wiki.startswith("# Commitment levels (retired)\n")
    assert "[Commitment levels](../site/paxeer-docs/src/app/finality/page.tsx)" in wiki
    labels = re.findall(r"\[([^\]]+)\]\(CommitmentLevels\.md\)", home)
    assert labels == ["Commitment levels (retired)"] * 2


def test_wiki_commitment_levels_names_no_live_lx_method():
    wiki = WIKI_PATH.read_text(encoding="utf-8")
    retired_methods = {"lx_sendActivity", "lx_getActivityStatus", "lx_getReceipt"}
    named_methods = set(re.findall(r"\blx_[A-Za-z][A-Za-z0-9_]*\b", wiki))
    assert named_methods == retired_methods

    for paragraph in wiki.split("\n\n"):
        if re.search(r"\blx_", paragraph):
            assert "methods are retired" in paragraph
            assert "`-32601`" in paragraph
            assert "Method not found" in paragraph
            assert "recover with" not in paragraph.lower()

    for obsolete in (
        "## Evidence at each level",
        "## Pending and refusal behavior",
        "## x402 binding",
        "Requested commitment unavailable",
        "`-32001`",
        "`executed` is the default",
        "```",
    ):
        assert obsolete not in wiki, f"Obsolete commitment guidance remains: {obsolete}"
