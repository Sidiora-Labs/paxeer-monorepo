"""Keep v7 release gap markers aligned with implemented release support."""

from pathlib import Path
import re
import tomllib


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
        cells = [
            cell.strip().replace(r"\|", "|")
            for cell in re.split(r"(?<!\\)\|", line.strip("|"))
        ]
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
    assert "The release tools and node provide these operations." in _section(
        text, "Release support"
    )
    support = _table_rows(_section(text, "Release support"))
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
    release_checks = _section(text, "Release checks and rehearsal evidence")
    assert (
        "- [ ] Programme release qualification passes at the candidate revision."
    ) in release_checks
    assert (
        "Use the acceptance criteria in the "
        "[merge specification](../../spec/layerx-merge/spec.kvx)."
    ) in release_checks
    services = _section(text, "Services")
    assert "The bridge signer set is governed, or the bridge stays paused." in services
    assert "check-attestor-manifest.py" in services
    assert "7.0.0-candidate-report.md" in text
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
    assert "plan-info --check <plan-info.json> --plan-height <H>" in text
    assert "equals SHA-256 of its `public_key`" not in text
    assert "wallet keeps `NEXT_PUBLIC_PAXEER_HOSTED_URL` set through" not in text
    assert "only the release binary" in text


def test_matrix_current_version_is_v7():
    text = _read_doc("readiness-matrix")
    rows = _table_rows(_section(text, "Versions"))
    assert rows[0] == ["Artifact", "Declared value", "Source"]
    release = next(row for row in rows[1:] if row[0] == "Node release")
    assert release[1:] == ["`v7.0.0`", "`tools/release/version.json`"]
    merge = next(row for row in rows[1:] if row[0] == "Merge plan")
    assert merge[1:] == [
        "`v7.0`", "`precompiles/common/precompiles.go`, `tools/release/version.json`"
    ]
    ledger = (RELEASE_DOCS / "consensus-commits.md").read_text(encoding="utf-8")
    assert "v7.0.0" in ledger and "v7.0" in ledger


def test_service_operator_runbook_has_only_open_gaps():
    text = _read_doc("service-operator-runbook")
    _assert_no_closed_gap_markers(text)
    support = _table_rows(_section(text, "Implemented service support"))
    assert len(support[1:]) == 6
    for path in (
        "services/platform/hosted/gateway/deployment.yaml:97",
        "services/explorer/backend/config/runtime.exs:315",
        "sdk/wallet/src/endpoint.ts:66",
        "apps/explorer/lib/hooks/useNavItems.tsx:445",
        "services/platform/hosted/gateway/README.md:9",
    ):
        assert any(path in row[0] for row in support[1:])
    manifest = next(row for row in support[1:] if "attestors.json" in row[0])
    assert "threshold 1" in manifest[1]
    assert "the checker compares all chain configurations" in manifest[1]
    bridge = _section(text, "Bridge")
    assert "threshold `1`" in bridge
    assert "exits 0 on the committed manifest" in bridge
    assert "**Gap" not in bridge
    assert (
        "Supply the chain owners, Solana program ID and deployment environment"
    ) in bridge
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
    assert (
        "Review halt, binary swap, first sequenced block and recovery"
    ) in " ".join(timeline)
    assert "paxeer-x-fork.md#4-rehearsal-must-pass-before-go" in " ".join(timeline)
    for evidence in (go, checklist):
        assert "candidate" in evidence
        positions = [evidence.index(name) for name in ORDERED_REHEARSAL_TESTS]
        assert positions == sorted(positions)
    for evidence in (checklist, _read_doc("candidate-report")):
        assert PROGRAMME_REHEARSAL_TEST in evidence
        assert "spec/layerx-merge/programme/tests/test_rehearsal_run.py::" in evidence
    assert "candidate report](7.0.0-candidate-report.md)" in go
    assert "Aggregate rehearsal checks also passed." in go
    assert "Review halt, swap, first sequenced block and recovery behavior" in checklist
    assert (
        "the presence of a check in this list does not establish that it passed."
    ) in checklist


def test_candidate_report_names_the_proven_checks_and_open_evidence():
    report = _read_doc("candidate-report")
    checklist = _section(
        _read_doc("readiness-checklist"), "Release checks and rehearsal evidence"
    )
    rows = _table_rows(_section(report, "Release checks"))
    assert rows[0] == ["Coverage", "Command"]
    expected = {
        (
            "First sequenced block, export/import, activation, store deletion, "
            "bond returns and constructor refusal"
        ): (
            *ORDERED_REHEARSAL_TESTS,
            "TestMergePlanReturnsGuarantorBonds",
            "TestMergePlanRefusesMissingMergeConstructors",
        ),
        "Retired anchor writes and selector refusals": (
            "TestRetiredWritesRevertAfterMerge", "TestEveryAnchorSelectorRetired",
        ),
        "Retired custody balance release": (
            "TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority",
        ),
        "Retired-caller inventory": ("tools/paxeer-x/tests/retired_callers_test.py",),
        "ABI selectors, calldata, decoded results and refusal encodings": (
            "tools/release/tests/test_programme.py",
        ),
        "Ordered merge rehearsal": (PROGRAMME_REHEARSAL_TEST,),
        "Pool-parity comparison rules": ("case_pool_parity_units",),
        "Consensus-commit coverage": ("tools/ci/consensus_commit_gate.py",),
    }
    assert [row[0] for row in rows[1:]] == list(expected)
    for coverage, command in rows[1:]:
        assert command.startswith("`") and command.endswith("`")
        for symbol in expected[coverage]:
            assert symbol in command
    for fact in (
        "after preparing the native dependencies with `make paxd-native-libs`",
        "Pool-parity unit checks cover pool counts, reserve caps and mismatch refusals",
        "service readiness and network acceptance are deployment checks.",
    ):
        assert fact in report
    for item in (
        "Candidate release evidence", "Golden-vector checks", "The retired-caller scan",
        "The four ordered rehearsal tests", "Bond return and custody release tests",
        "Retired anchor writes", "Retired custody balances", "Refusal tests",
        "The programme rehearsal", "Pool-parity comparison rules",
    ):
        assert sum(
            line.startswith(f"- [ ] {item}") for line in checklist.splitlines()
        ) == 1


def test_candidate_record_identifies_every_ticked_line():
    record = tomllib.loads(
        (ROOT / "spec/layerx-merge/programme/records/docs/release/repair_22_39_182.kvx").read_text(
            encoding="utf-8"
        )
    )
    lines = _read_doc("readiness-checklist").splitlines()
    recorded = record["checklist"]
    assert len(recorded["ticked_lines"]) == len(recorded["ticked_items"]) == 11
    assert recorded["ticked_lines"] == sorted(set(recorded["ticked_lines"]))
    for item in recorded["ticked_items"]:
        assert sum(line.startswith(f"- [ ] {item}") for line in lines) == 1, item
    assert (
        record["release_candidate"]["report"]
        == "docs/release/7.0.0-candidate-report.md"
    )
    assert (ROOT / record["release_candidate"]["report"]).is_file()


def test_agent_daemon_qualification_is_after_s_acceptance():
    agent = _section(_read_doc("service-operator-runbook"), "Agent daemon")
    acceptance = agent.split("**Acceptance after `S`.**", 1)[1]
    for evidence in (
        "exact deployed\nrevision",
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
    assert "for the deployed release" in after_s
    assert "agent daemon passes" not in _section(checklist, "Services")
