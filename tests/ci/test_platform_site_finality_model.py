"""Keep platform guidance aligned with Paxeer X Network and block finality."""

from pathlib import Path
import re

import pytest


REPO_ROOT = Path(__file__).resolve().parents[2]
CONTENT = REPO_ROOT / "docs/platform/content"
RECEIPT_PAGES = (
    "concepts/receipts.md",
    "guide/receipts.md",
    "concepts/paying.md",
    "quickstart/rust.md",
    "framework/spring.md",
    "environments/emulator.md",
)
COMMITTED_EVIDENCE = (
    "https://docs.paxeer.app/checkpoints/"
)


def test_platform_hand_written_pages_do_not_name_layerx_network():
    pages = sorted(
        page
        for page in CONTENT.rglob("*.md")
        if page.relative_to(CONTENT).parts[0] != "reference"
    )
    assert pages, "No hand-written platform pages found"
    offending_pages = [
        str(page.relative_to(CONTENT))
        for page in pages
        if "LayerX Network" in page.read_text(encoding="utf-8")
    ]
    assert not offending_pages, (
        "Hand-written platform pages name LayerX Network: "
        + ", ".join(offending_pages)
    )


@pytest.mark.parametrize("page", RECEIPT_PAGES)
def test_platform_receipt_pages_do_not_present_checkpoint_finality(page):
    text = (CONTENT / page).read_text(encoding="utf-8")
    for obsolete in (
        "SettlementAnchored",
        "CheckpointFinalised",
        "SequencerSigned",
        "BatchIncluded",
        "StateProven",
        "The sequencer did not authorise this batch",
        "verifies against an authorised batch header using only those two inputs",
        "The batch is under a finalised checkpoint",
        "The checkpoint is anchored externally",
        "additionally covered by a finalised checkpoint proof",
        "a checkpoint you follow, a mirror publication on another chain",
        "Differential conformance against the hosted beta network is what closes that gap",
    ):
        assert obsolete not in text, f"Obsolete finality claim in {page}: {obsolete}"

    rung_page = (
        "concepts/receipts.md"
        if page in {"guide/receipts.md", "quickstart/rust.md", "environments/emulator.md"}
        else page
    )
    rungs = (CONTENT / rung_page).read_text(encoding="utf-8")
    assert (
        "`settlement-anchored` and `paxeer-finalised` are never reached "
        "on the merged network."
    ) in rungs, rung_page
    for line in text.splitlines():
        if line.startswith("|"):
            assert not re.search(
                r"`(?:checkpoint-finalised|settlement-anchored|paxeer-finalised)`",
                line,
            ), f"Retired ladder row in {page}: {line}"


@pytest.mark.parametrize("page", RECEIPT_PAGES)
def test_platform_receipt_pages_name_committed_as_final(page):
    text = (CONTENT / page).read_text(encoding="utf-8")
    finality_page = "concepts/receipts.md" if page == "environments/emulator.md" else page
    finality = (CONTENT / finality_page).read_text(encoding="utf-8")
    assert "A receipt is final once its block is committed." in finality, finality_page
    assert f"]({COMMITTED_EVIDENCE})" in text, page
    assert "CometBFT commit" in text, page
    assert "inclusion" in text and "height" in text, page

    if page == "framework/spring.md":
        assert "Signed receipt verification authenticates execution" in text, page
        assert "independently trusted receipt keys" in text, page
    elif page == "environments/emulator.md":
        assert (
            "The emulator does not participate in consensus or establish block commitment."
        ) in " ".join(text.split()), page
        assert "Verify receipts against independently pinned keys." in text, page
    elif page != "concepts/paying.md":
        phrase = (
            "local compatibility verification" if page == "guide/receipts.md"
            else "local compatibility verifier"
        )
        assert phrase in text, page
        assert "caller-supplied batch facts" in text, page
    if page != "concepts/paying.md":
        rungs = finality if page == "environments/emulator.md" else text
        for rung in ("submitted", "receipted", "committed"):
            assert f"`{rung}`" in rungs, page

    if page in ("concepts/receipts.md", "guide/receipts.md"):
        rows = re.findall(r"^\| `([^`]+)` \| (.+) \|$", text, re.MULTILINE)
        rung_rows = [
            (name, meaning)
            for name, meaning in rows
            if name in ("submitted", "receipted", "committed")
        ]
        assert rung_rows == [
            ("submitted", "A known transaction without a signed execution receipt"),
            ("receipted", "A signed execution receipt awaiting block commitment"),
            (
                "committed",
                "The committed block covers the receipt; block commit is finality",
            ),
        ], page
        aliases = (CONTENT / "concepts/receipts.md").read_text(encoding="utf-8")
        assert (
            "`checkpoint-finalised` and `done-finalised` are retained API names "
            "for the `committed` rung."
        ) in aliases


def test_platform_paying_done_finalised_is_committed_rung():
    text = (CONTENT / "concepts/paying.md").read_text(encoding="utf-8")
    rows = [
        line
        for line in text.splitlines()
        if line.startswith("| `done-finalised` |")
    ]
    assert rows == [
        "| `done-finalised` | Retained API name for an outcome backed by the "
        "`committed` rung; block commit is finality |"
    ]
    assert (
        "`checkpoint-finalised` is also a retained API name for the `committed` rung; "
        "neither label adds another checkpoint or settlement step."
    ) in text
    assert "A receipt is final once its block is committed." in text
