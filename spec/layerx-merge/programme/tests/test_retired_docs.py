from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DOCUMENTS = (
    "docs/site/docs/agents/sdk-verification.md",
    "docs/site/docs/programs/workspace.md",
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
    retired_page = ROOT / "docs/site/docs/platform/relay-archive.md"
    assert not retired_page.exists(), "the relay/archive docs page must be retired"
    navigation = (ROOT / "docs/site/mkdocs.yml").read_text(encoding="utf-8")
    assert "relay-archive" not in navigation, "retired page remains in navigation"

    for name in DOCUMENTS:
        document = (ROOT / name).read_text(encoding="utf-8")
        for retired in ("terminal-v4.test.py", "receipt-programs"):
            assert retired not in document, f"{name}: retired reference {retired}"
        assert "daemon/committed-fixture/vectors/" in document, (
            f"{name}: missing sequencer vector source"
        )
        for vector in VECTORS:
            assert vector in document, f"{name}: missing sequencer vector {vector}"
