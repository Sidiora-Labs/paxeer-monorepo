"""Keep v7 release gap markers aligned with implemented release support."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]
RELEASE_DOCS = ROOT / "docs/release"
CLOSED_IDS = {"D1", "D2", "D3", "D4", "D6", "D7", "D9", "D12", "D14", "D16"}


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
    assert [row[0] for row in gaps[1:]] == ["D5"]
    assert gaps[1][1] == "spec/layerx-merge/spec.kvx"
    assert "17.0.9" in gaps[1][2]
    assert "pending" in gaps[1][2]

    support = _table_rows(_section(text, "Implemented release support"))
    assert support[0] == ["ID", "Present support", "Source"]
    assert {row[0] for row in support[1:]} == CLOSED_IDS
    assert len(support[1:]) == len(CLOSED_IDS)
    sources = {
        "D1": "tools/release/version.json:2",
        "D2": ".github/workflows/paxeer-release-check.yml:65",
        "D3": "tools/release/upgrade_info.py:343",
        "D4": "tools/release/swap.sh:37",
        "D6": "tools/release/rehearse-v6120.sh",
        "D7": "sdk/wallet/src/endpoint.ts:66",
        "D9": "services/explorer/backend/config/runtime.exs:315",
        "D12": "docs/release/consensus-commits.md:1",
        "D14": "node/upgrades.go:222",
        "D16": ".github/workflows/paxeer-docker-publish.yml:82",
    }
    for gap_id, fact, source in support[1:]:
        assert "Gap" not in fact
        assert sources[gap_id] in source
    rehearsal = next(row for row in support[1:] if row[0] == "D6")
    for fact in (
        "tools/release/version.json", "--plan-info", "v7.0", "node upgrade files"
    ):
        assert fact in rehearsal[1]
    assert "tests/ci/test_release_merge_rehearsal.py" in rehearsal[2]
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
    assert len(gap_items) == 3
    assert [re.findall(r"\bD\d+\b", line) for line in gap_items] == [
        ["D5"],
        ["D5"],
        [],
    ]
    assert "17.0.9" in gap_items[0]
    assert "check-attestor-manifest.py" in gap_items[2]
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
    ]
    assert "pending governance" in gaps[1][1]
    marked = [line for line in text.splitlines() if "**Gap" in line]
    assert len(marked) == 2  # Definition and attestor manifest.
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
    assert (
        "upgraded public node root URLs" in gateway
        and "the node root serves the `px_` methods" in text
        and "there is no `/rpc` path and no separate router" in text
    )
    assert "https://api1.mainnet-beta.paxeer.network/" in gateway
    assert "https://api8.mainnet-beta.paxeer.network/" in gateway
    assert "one internal" not in text
    assert "keyId` must equal SHA-256" not in text
    assert "only `px_register` and `px_requestFunds`" in text
    assert "`Kernel archive`" in text
    assert "Text still says public nodes do not serve" not in text


ORDERED_REHEARSAL_TESTS = (
    "TestMergeRehearsalFirstSequencedBlock",
    "TestExportAfterMergeKeepsSequencerState",
    "TestMergePlanActivatesGatedPrecompiles",
    "TestAnchorStoreDeletedAtStep",
)
PROGRAMME_REHEARSAL_TEST = "test_rehearsal_passes_on_merged_revision"


def test_exported_state_rehearsal_waiver_retains_candidate_evidence():
    plan = _read_doc("upgrade-plan")
    timeline = next(row for row in _table_rows(plan) if row[0] == "T−16 d")
    go = _section(plan, "5. Go / no-go criteria")
    checklist = _section(
        _read_doc("readiness-checklist"), "Release checks and rehearsal evidence"
    )
    for evidence in (" ".join(timeline), go, checklist):
        assert "release owner" in evidence
        assert "decision 3020" in evidence
        assert "waiv" in evidence
        assert "exported-state rehearsal" in evidence.lower()
        assert "halt and swap evidence" in evidence.lower()
        assert "candidate release report" in evidence
        assert PROGRAMME_REHEARSAL_TEST in evidence
        positions = [evidence.index(name) for name in ORDERED_REHEARSAL_TESTS]
        assert positions == sorted(positions)
    assert "spec/layerx-merge/programme/tests/test_rehearsal_run.py::" in go
    waiver_items = [
        line
        for line in checklist.splitlines()
        if "- [ ]" in line and "waiv" in line
    ]
    assert len(waiver_items) == 2
    for item in waiver_items:
        assert "decision 3020" in item
        assert "candidate release report" in item
        assert PROGRAMME_REHEARSAL_TEST in item
        assert "**Gap" not in item
    assert 'UPGRADE "v7.0" NEEDED' in waiver_items[1]


def test_agent_daemon_qualification_is_after_s_acceptance():
    agent = _section(_read_doc("service-operator-runbook"), "Agent daemon")
    acceptance = agent.split("**Acceptance after `S` (decision 3020).**", 1)[1]
    for evidence in (
        "exact deployed revision",
        "before opening writes",
        "only after `px_getReceiptKeys` returns the plan's keys",
        "readiness result and receipt-key response",
        "A scheduled job settles",
        "settlement evidence",
        "Node logs show no `agent end block failed`",
        "Retain the node logs",
    ):
        assert evidence in acceptance
    assert "**Gap" not in agent
    checklist = _read_doc("readiness-checklist")
    after_s = _section(checklist, "After `S` (acceptance)")
    assert "agent daemon passes release qualification after `S`" in after_s
    assert "exact deployed revision" in after_s
    assert "agent daemon passes" not in _section(checklist, "Services")
