"""Keep the Bridge wiki aligned with the public bridge API."""

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
BRIDGE_PAGE = ROOT / "docs/wiki/Bridge.md"


def test_wiki_bridge_names_only_existing_lxp_symbols():
    text = BRIDGE_PAGE.read_text(encoding="utf-8")
    assert (
        "The retained library declarations in" in text
        and "[`lxp_bridge.h`](../../include/layerx/lxp_bridge.h)" in text
    )
    # The current page names the declaration header rather than individual
    # helper symbols; source paths are documentation links, not API names.
    prose = re.sub(r"\[[^\]]+\]\([^)]+\)", "", text)
    prose = re.sub(r"`[^`\n]+\.(?:h|c)`", "", prose)
    names = set(re.findall(r"\blxp_[A-Za-z0-9_]+\b", prose))

    headers = sorted((ROOT / "include").rglob("*.h"))
    assert headers, "Public headers must be present"
    declarations = "\n".join(
        re.sub(
            r"/\*.*?\*/|//[^\n]*",
            "",
            header.read_text(encoding="utf-8"),
            flags=re.DOTALL,
        )
        for header in headers
    )
    missing = sorted(
        name
        for name in names
        if not re.search(
            r"\b" + re.escape(name) + r"\s*\([^;{}]*\)\s*;", declarations
        )
    )
    assert not missing, f"Bridge wiki names APIs without public declarations: {missing}"


def test_wiki_bridge_does_not_link_guarantor():
    text = BRIDGE_PAGE.read_text(encoding="utf-8")
    assert not re.search(r"\bGuarantor\.md\b", text, re.IGNORECASE), (
        "Bridge wiki must not link to the retired Guarantor page"
    )
