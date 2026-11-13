from html import unescape
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[4]
DOCUMENTS = (
    "docs/site/paxeer-docs/src/app/receipts/page.tsx",
    "docs/site/paxeer-docs/src/app/program-workspace/page.tsx",
    "docs/wiki/SdkTerminalVerification.md",
    "docs/wiki/ProgramsWorkspace.md",
)
VECTORS = (
    "vector-v1-receipted-positive.json",
    "vector-v2-receipted-revert.json",
    "vector-v3-receipt-refusals.json",
    "vector-v4-committed.json",
)


def test_docs_name_no_retired_paths() -> None:
    retired_page = ROOT / "docs/site/paxeer-docs/src/app/relay-archive/page.tsx"
    assert retired_page.is_file(), "the relay/archive route must retain its retirement notice"
    assert "Relay/archive is retired" in retired_page.read_text(encoding="utf-8")
    navigation = (ROOT / "docs/site/paxeer-docs/src/components/nav.ts").read_text(encoding="utf-8")
    assert "Relay archive (retired)" in navigation, "relay/archive navigation must identify its retirement"

    for name in DOCUMENTS:
        document = (ROOT / name).read_text(encoding="utf-8")
        for retired in ("terminal-v4.test.py", "receipt-programs"):
            assert retired not in document, f"{name}: retired reference {retired}"
        assert "daemon/committed-fixture/vectors/" in document, (
            f"{name}: missing sequencer vector source"
        )
        for vector in VECTORS:
            assert vector in document, f"{name}: missing sequencer vector {vector}"


def test_no_docs_page_links_retired_relay_archive() -> None:
    pages = sorted((ROOT / "docs/site/paxeer-docs/src/app").rglob("page.tsx"))
    assert pages, "no documentation pages found"
    offending_pages = [
        str(page.relative_to(ROOT))
        for page in pages
        if re.search(r"href=[\"\'](?:[^\"\']*/)?relay-archive\.md(?:[?#][^\"\']*)?[\"\']", page.read_text(encoding="utf-8"))
    ]
    assert not offending_pages, (
        f"retired relay/archive link target remains in: {', '.join(offending_pages)}"
    )


def test_inbound_pages_describe_sequencer_receipt_verification() -> None:
    pages = (
        "docs/wiki/PortableVerifier.md",
        "docs/site/paxeer-docs/src/app/proofs/page.tsx",
        "docs/site/paxeer-docs/src/app/agents/page.tsx",
        "docs/site/paxeer-docs/src/app/sdk/page.tsx",
        "docs/site/paxeer-docs/src/app/programs/page.tsx",
    )
    retired_descriptions = (
        "CALL terminal decoder",
        "Program CALL terminal",
        "SdkTerminalVerification.md:17-19",
        "SdkTerminalVerification.md:50-62",
        "SdkTerminalVerification.md:57-58",
    )
    for name in pages:
        document = " ".join(unescape(re.sub(r"<[^>]+>", "", (ROOT / name).read_text(encoding="utf-8"))).split())
        assert "SDK sequencer receipt verification" in document, (
            f"{name}: missing sequencer receipt verification description"
        )
        for retired in retired_descriptions:
            assert retired not in document, f"{name}: retired reference {retired}"


def test_custody_credit_docs_are_retired() -> None:
    pages = (
        "docs/wiki/Bridge.md",
        "docs/wiki/Programs.md",
        "docs/site/paxeer-docs/src/app/programs/page.tsx",
        "docs/wiki/Custody.md",
        "docs/site/paxeer-docs/src/app/custody/page.tsx",
    )
    retired_references = (
        "lxp_bridge_credit",
        "lxp_bridge_light",
        "lxp_ctx_bridge_credit",
        "LXP_BRIDGE_CREDIT",
        "custody-credit-profile/v1",
        "custody-issued:",
    )
    for name in pages:
        document = " ".join(unescape(re.sub(r"<[^>]+>", "", (ROOT / name).read_text(encoding="utf-8"))).split())
        assert (
            "kernel custody credit activity and its light client are retired" in document
        ), (
            f"{name}: missing kernel custody credit and light client retirement notice"
        )
        for retired in retired_references:
            assert retired not in document, f"{name}: retired reference {retired}"
