"""Keep the documentation index aligned with the merged network and release."""

from pathlib import Path
import re


DOCS = Path(__file__).resolve().parents[2] / "docs"


def _index_row(page):
    content = (DOCS / "README.md").read_text(encoding="utf-8")
    rows = [
        line
        for line in content.splitlines()
        if line.startswith("|") and f"]({page})" in line.split("|")[1]
    ]
    assert len(rows) == 1, f"Expected one documentation index row for {page}"
    assert (DOCS / page).is_file(), f"Index target does not exist: {page}"
    return rows[0]


def test_docs_readme_does_not_call_network_overview_historical():
    row = _index_row("paxeer-network.md")
    assert "historical" not in row.lower()
    assert re.search(r"current.*network overview", row, re.IGNORECASE)
    assert "chain ID `125`" in row

    content = (DOCS / "README.md").read_text(encoding="utf-8")
    introduction = " ".join(content.split("| Path |", 1)[0].split())
    assert "current documentation and historical references" in introduction
    assert "Rows marked historical" in introduction
    assert "The historical references below" not in introduction


def test_docs_readme_links_release_plan():
    row = _index_row("release/7.0.0-upgrade-plan.md")
    assert "7.0.0 release plan" in row
    for page in (
        "release/7.0.0-validator-runbook.md",
        "release/7.0.0-service-operator-runbook.md",
        "release/7.0.0-readiness-checklist.md",
    ):
        assert re.search(rf"\[[^\]]+\]\({re.escape(page)}\)", row), (
            f"Release plan row must link its companion document: {page}"
        )
        assert (DOCS / page).is_file(), f"Release document does not exist: {page}"
