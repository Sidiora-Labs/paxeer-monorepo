"""Keep the kernel wiki's symbols, activities and source paths current."""

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PAGES = tuple(
    ROOT / "docs/wiki" / name
    for name in ("Asset.md", "Governance.md", "Service.md", "VirtualChains.md")
)
SYMBOL = re.compile(r"\b(?:LXP_|lxp_)[A-Za-z0-9_]+\b")
SOURCE_PATH = re.compile(r"\b(?:include|src)/[A-Za-z0-9_./-]+")
C_COMMENT = re.compile(r"/\*.*?\*/|//[^\n]*", re.DOTALL)


def test_wiki_kernel_symbols_exist_in_source():
    sources = [
        path
        for directory in (ROOT / "include", ROOT / "src")
        for path in directory.rglob("*")
        if path.suffix in {".c", ".h"}
        and not {"test", "tests"}.intersection(path.relative_to(directory).parts)
    ]
    assert sources, "Kernel source files must be present"
    source_symbols = set()
    for path in sources:
        text = C_COMMENT.sub("", path.read_text(encoding="utf-8"))
        source_symbols.update(SYMBOL.findall(text))

    references = {
        page.name: set(SYMBOL.findall(SOURCE_PATH.sub("", page.read_text())))
        for page in PAGES
    }
    assert all(references[name] for name in ("Asset.md", "Governance.md", "Service.md"))
    missing = {
        name: sorted(symbols - source_symbols)
        for name, symbols in references.items()
        if symbols - source_symbols
    }
    assert not missing, f"Wiki symbols absent from kernel source: {missing}"


def test_wiki_governance_lists_registered_ordinals():
    source = C_COMMENT.sub(
        "", (ROOT / "src/modules/governance/lxp_governance.c").read_text()
    )
    header = (ROOT / "include/layerx/lxp_governance.h").read_text()
    constants = {
        name: int(value, 16)
        for name, value in re.findall(
            r"\b(LXP_GOVERNANCE_\w+)\s*=\s*(0x[0-9a-fA-F]+)\b", header
        )
    }
    registration = re.search(
        r"static\s+const\s+uint32_t\s+\w+\[\]\s*=\s*\{([^}]+)\}", source
    )
    assert registration, "Governance must declare its registered activity types"
    registered_names = re.findall(
        r"0x[0-9a-fA-F]+[Uu]?|LXP_GOVERNANCE_\w+", registration[1]
    )
    assert registered_names
    registered = {
        constants[name] if name in constants else int(name.rstrip("Uu"), 16)
        for name in registered_names
    }
    activity = re.search(
        r"bool\s+lxp_governance_activity\([^)]*\)\s*\{([^}]+)\}", source
    )
    assert activity, "Governance must expose its activity acceptance predicate"
    accepted_names = re.findall(
        r"type\s*==\s*(0x[0-9a-fA-F]+[Uu]?|LXP_GOVERNANCE_\w+)", activity[1]
    )
    accepted = {
        constants[name] if name in constants else int(name.rstrip("Uu"), 16)
        for name in accepted_names
    }
    assert registered == accepted

    wiki = (ROOT / "docs/wiki/Governance.md").read_text()
    table = wiki.split("## Registered activities\n", 1)[1].split("\n---", 1)[0]
    rows = re.findall(
        r"^\| `(0x[0-9a-fA-F]+)` \| (\d+) \| ([^\n]+)\|$", table, re.MULTILINE
    )
    assert rows, "Governance wiki must list registered activities"
    documented = [int(value, 16) for value, _, _ in rows]
    assert len(documented) == len(set(documented)), "Duplicate governance rows"
    assert set(documented) == registered
    for value, ordinal, role in rows:
        assert int(ordinal) == int(value, 16) & 0xFFFF
        for name, activity_type in constants.items():
            if activity_type == int(value, 16):
                assert f"`{name}`" in role


def test_wiki_asset_receive_marked_historical():
    wiki = (ROOT / "docs/wiki/Asset.md").read_text()
    sections = re.split(r"^## Historical payloads\s*$", wiki, flags=re.MULTILINE)
    assert len(sections) == 2, "Asset wiki must have one Historical payloads section"
    current, historical = sections
    historical = re.split(r"^## ", historical, maxsplit=1, flags=re.MULTILINE)[0]
    assert (
        "[Historical payment payloads]"
        "(../site/paxeer-docs/src/app/assets/page.tsx)"
    ) in historical
    prose = " ".join(historical.split())
    assert "retained only for decoding archived artifacts" in prose
    assert "do not authorize current grant issuance, receive or submission" in prose
    for heading in ("RECEIVE", "GRANT_ISSUE / GRANT_REVOKE"):
        assert re.search(
            r"^### " + re.escape(heading) + r"$", historical, re.MULTILINE
        )
        assert not re.search(
            r"^#{2,6} " + re.escape(heading) + r"$", current, re.MULTILINE
        )
    assert "0x5201" in historical, "Keep the archived RECEIVE layout identifiable"
    assert "revocation_sequence:u64" in historical, "Keep the archived revoke layout"


def test_wiki_backticked_paths_exist():
    for page in PAGES:
        paths = [
            span
            for span in re.findall(r"(?<!`)`([^`\n]+)`(?!`)", page.read_text())
            if re.fullmatch(r"(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]*", span)
        ]
        assert paths, f"{page.name} must cite repository paths"
        missing = [path for path in paths if not (ROOT / path).exists()]
        assert not missing, f"{page.name} cites missing paths: {missing}"
