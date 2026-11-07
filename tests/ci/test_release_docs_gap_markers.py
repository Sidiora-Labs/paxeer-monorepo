"""Keep v7 release gap markers aligned with implemented release support."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]
RELEASE_DOCS = ROOT / "docs/release"
CLOSED_IDS = {"D1", "D2", "D3", "D4", "D7", "D9", "D12", "D14", "D16"}


def _read_doc(name):
    return (RELEASE_DOCS / f"7.0.0-{name}.md").read_text(encoding="utf-8")


def _section(text, heading):
    return text.split(f"## {heading}\n", 1)[1].split("\n## ", 1)[0]


def _table_rows(text):
    rows = []
    for line in text.splitlines():
        if not line.startswith("|"):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if not all(re.fullmatch(r"[-: ]+", cell) for cell in cells):
            rows.append(cells)
    assert rows, "Expected a non-empty documentation table"
    return rows


def _assert_no_closed_gap_markers(text):
    for paragraph in re.split(r"\n\s*\n", text):
        if re.search(r"\*\*Gaps?\b", paragraph):
            ids = set(re.findall(r"\bD\d+\b", paragraph))
            assert not ids & CLOSED_IDS, paragraph


def test_upgrade_plan_has_no_gap_for_version_json():
    text = _read_doc("upgrade-plan")
    _assert_no_closed_gap_markers(text)
    identity = next(
        row for row in _table_rows(text) if row[0] == "Release identity file"
    )
    assert "`version` `v7.0.0`" in identity[1]
    assert "`upgrade` `v7.0`" in identity[1]
    assert "Gap" not in " ".join(identity)
    assert "v6.12.0" not in " ".join(identity)
    assert "tools/release/version.json:2" in identity[2]
    scheduling = text.split("### Scheduling\n", 1)[1].split("### Heights\n", 1)[0]
    assert "**Gap" not in scheduling
    for marker in (
        "plan-info --check",
        "node --plan-info",
        "swap.sh --plan-info",
        "tools/release/upgrade_info.py:343",
        "tools/release/swap.sh:37",
    ):
        assert marker in scheduling
    assert "writes that file without `info`" not in text


def test_gap_table_lists_only_open_ids():
    text = _read_doc("upgrade-plan")
    gaps = _table_rows(_section(text, "Known gaps"))
    assert gaps[0] == ["ID", "Path", "Gap"]
    assert [row[0] for row in gaps[1:]] == ["D5", "D6"]
    assert gaps[1][1] == "spec/layerx-merge/spec.kvx"
    assert "17.0.9" in gaps[1][2]
    assert "pending" in gaps[1][2]
    assert gaps[2][1] == "tools/release/rehearse-v6120.sh"
    assert "v6.12.0" in gaps[2][2]
    assert "rollback" in gaps[2][2]

    support = _table_rows(_section(text, "Implemented release support"))
    assert support[0] == ["ID", "Present support", "Source"]
    assert {row[0] for row in support[1:]} == CLOSED_IDS
    assert len(support[1:]) == len(CLOSED_IDS)
    sources = {
        "D1": "tools/release/version.json:2",
        "D2": ".github/workflows/paxeer-release-check.yml:65",
        "D3": "tools/release/upgrade_info.py:343",
        "D4": "tools/release/swap.sh:37",
        "D7": "sdk/wallet/src/endpoint.ts:66",
        "D9": "services/explorer/backend/config/runtime.exs:315",
        "D12": "docs/release/consensus-commits.md:1",
        "D14": "node/upgrades.go:222",
        "D16": ".github/workflows/paxeer-docker-publish.yml:82",
    }
    for gap_id, fact, source in support[1:]:
        assert "Gap" not in fact
        assert sources[gap_id] in source
    _assert_no_closed_gap_markers(text)


def test_validator_runbook_has_no_closed_gaps():
    text = _read_doc("validator-runbook")
    _assert_no_closed_gap_markers(text)
    assert "**Gap" not in text
    assert "identity reads `v7.0.0` / `v7.0`" in text
    assert "tools/release/version.json:2" in text
    assert "tools/release/upgrade_info.py:27" in text
    assert "still reads `v6.12.0`" not in text
    assert "refuses\na two-part plan name" not in text
    deletion = _section(text, "9. Later: anchor-store deletion")
    assert "positive applied merge height strictly below the current" in deletion
    assert "node/upgrades.go:222" in deletion
    assert "does not\nenforce that order" not in text
    assert ".github/workflows/paxeer-docker-publish.yml:82" in text
    assert "image is built separately" not in text


def test_checklist_has_no_closed_gaps():
    text = _read_doc("readiness-checklist")
    _assert_no_closed_gap_markers(text)
    gap_items = [
        line for line in text.splitlines() if "**Gap" in line and "- [ ]" in line
    ]
    assert len(gap_items) == 4
    assert [re.findall(r"\bD\d+\b", line) for line in gap_items] == [
        ["D5"],
        ["D5"],
        ["D6"],
        [],
    ]
    assert "17.0.9" in gap_items[0]
    assert "rehearse-v6120.sh" in gap_items[2]
    assert "check-attestor-manifest.py" in gap_items[3]
    for source in (
        "tools/release/version.json:2",
        "tools/release/upgrade_info.py:27",
        ".github/workflows/paxeer-release-check.yml:65",
        "tools/release/upgrade_info.py:343",
        "docs/release/consensus-commits.md:1",
        "node/upgrades.go:222",
        ".github/workflows/paxeer-docker-publish.yml:82",
        "services/platform/hosted/gateway/deployment.yaml:97",
        "services/explorer/backend/config/runtime.exs:315",
        "sdk/wallet/src/endpoint.ts:66",
    ):
        assert source in text
    assert "plan-info --check --plan-height <H>" in text
    assert "equals SHA-256 of its `public_key`" not in text
    assert "wallet keeps `NEXT_PUBLIC_PAXEER_HOSTED_URL` set through" not in text
    assert "only the release binary" in text


def test_matrix_current_version_is_v7():
    text = _read_doc("readiness-matrix")
    rows = _table_rows(_section(text, "Versions"))
    assert rows[0] == ["Artifact", "Current version", "Expected for the 7.0.0 upgrade"]
    for field, version in (("version", "v7.0.0"), ("upgrade", "v7.0")):
        row = next(
            row
            for row in rows[1:]
            if row[0].startswith(f"`tools/release/version.json` `{field}`")
        )
        assert row[1:] == [f"`{version}`", f"`{version}`"]
        assert "tools/release/version.json:" in row[0]
    ledger = next(
        row
        for row in rows[1:]
        if row[0].startswith("`docs/release/consensus-commits.md`")
    )
    assert "**v7.0.0**" in ledger[1]
    assert "**v7.0**" in ledger[1]
    assert "v6.12.0" not in ledger[1]
    assert ledger[2] == "7.0.0"


def test_service_operator_runbook_has_only_open_gaps():
    text = _read_doc("service-operator-runbook")
    gaps = _table_rows(_section(text, "Gaps in this runbook"))
    assert gaps[0] == ["Path", "Gap"]
    assert [row[0] for row in gaps[1:]] == [
        "services/bridge/deploy/attestors.json",
        "spec/layerx-merge/spec.kvx",
    ]
    assert "pending governance" in gaps[1][1]
    assert "Agent daemon tasks still pending" in gaps[2][1]
    marked = [line for line in text.splitlines() if "**Gap" in line]
    assert len(marked) == 3  # Definition, pending agent tasks, attestor manifest.
    support = _table_rows(_section(text, "Implemented service support"))
    assert len(support[1:]) == 5
    for path in (
        "services/platform/hosted/gateway/deployment.yaml:97",
        "services/explorer/backend/config/runtime.exs:315",
        "sdk/wallet/src/endpoint.ts:66",
        "apps/explorer/lib/hooks/useNavItems.tsx:445",
        "services/platform/hosted/gateway/README.md:9",
    ):
        assert any(path in row[0] for row in support[1:])
    gateway = _section(text, "Hosted gateway")
    assert "`api1` to `api14`" in gateway
    assert "decision 2521" in gateway
    assert "https://api1.mainnet-beta.paxeer.network/" in gateway
    assert "https://api8.mainnet-beta.paxeer.network/" in gateway
    assert "one internal" not in text
    assert "keyId` must equal SHA-256" not in text
    assert "only `px_register` and `px_requestFunds`" in text
    assert "`Kernel archive`" in text
    assert "Text still says public nodes do not serve" not in text
