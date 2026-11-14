"""Keep the documentation index aligned with the merged network and release."""

from pathlib import Path
import re


DOCS = Path(__file__).resolve().parents[2] / "docs"


def _index_link(page, section):
    content = (DOCS / "README.md").read_text(encoding="utf-8")
    content = content.split(f"### {section}\n", 1)[1].split("\n#", 1)[0]
    rows = [
        line
        for line in content.splitlines()
        if line.startswith("- ") and f"]({page})" in line
    ]
    assert len(rows) == 1, f"Expected one documentation index entry for {page}"
    assert (DOCS / page).is_file(), f"Index target does not exist: {page}"
    return rows[0]


def test_docs_readme_does_not_call_network_overview_historical():
    row = _index_link("paxeer-network.md", "Public endpoints")
    assert "historical" not in row.lower()
    assert "[Network overview]" in row
    assert "describes the node JSON-RPC and tenant-domain views." in row

    content = (DOCS / "README.md").read_text(encoding="utf-8")
    introduction = " ".join(content.split("## Systems", 1)[0].split())
    assert "Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`)." in introduction
    assert "the documentation describes the network, its systems, operations and reference material." in introduction
    overview = (DOCS / "paxeer-network.md").read_text(encoding="utf-8")
    assert "Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`)." in overview


def test_docs_readme_links_release_plan():
    row = _index_link("release/7.0.0-upgrade-plan.md", "Release pages")
    assert "docs/release/7.0.0-upgrade-plan.md" in row
    for page in (
        "release/7.0.0-validator-runbook.md",
        "release/7.0.0-service-operator-runbook.md",
        "release/7.0.0-readiness-checklist.md",
    ):
        entry = _index_link(page, "Release pages")
        assert re.search(rf"\[[^\]]+\]\({re.escape(page)}\)", entry), (
            f"Release documentation must link its companion document: {page}"
        )
        assert (DOCS / page).is_file(), f"Release document does not exist: {page}"
