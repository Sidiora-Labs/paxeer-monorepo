import importlib.util
import json
from pathlib import Path

import pytest


CI = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "consensus_commit_gate_22_33_731", CI / "consensus_commit_gate.py"
)
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)
RECORDING = json.loads(
    (Path(__file__).parent / "fixtures/consensus-history-22-33-731.json").read_text(
        encoding="utf-8"
    )
)
RULE = "node/sequencer/rule.go"


def recorded_gate(name):
    assert RECORDING["producer"].startswith("git version ")
    assert tuple(RECORDING["history_args"]) == GATE.HISTORY_ARGS
    case = RECORDING["cases"][name]
    return case, GATE.unlisted_from_history(case["ledger"], case["history"])


@pytest.mark.parametrize(
    "name",
    ["merge_only", "resolved_merge", "merge_deletion", "octopus_merge"],
)
def test_finding_010_merge_result_requires_its_own_evidence(name):
    case, unlisted = recorded_gate(name)
    assert unlisted == {case["ids"]["merge"]: [RULE]}
    assert case["ids"]["base"] not in unlisted


@pytest.mark.parametrize("name", ["merge_listed", "resolved_merge_listed"])
def test_finding_010_merge_evidence_closes_the_refusal(name):
    case, unlisted = recorded_gate(name)
    assert case["ids"]["merge"] in GATE._table_commits(case["ledger"])
    assert unlisted == {}


@pytest.mark.parametrize("name", ["clean_merge", "inherited_resolution"])
def test_finding_010_parent_covered_changes_need_no_merge_entry(name):
    case, unlisted = recorded_gate(name)
    listed = GATE._table_commits(case["ledger"])
    assert case["ids"]["left"] in listed
    assert case["ids"]["right"] in listed
    assert case["ids"]["merge"] not in listed
    assert GATE.consensus_changes_from_history(case["history"])[
        case["ids"]["merge"]
    ] == []
    assert unlisted == {}


def test_finding_010_merge_approval_does_not_cover_an_unlisted_parent():
    case, unlisted = recorded_gate("unlisted_parent")
    assert case["ids"]["merge"] in GATE._table_commits(case["ledger"])
    assert unlisted == {case["ids"]["left"]: [RULE]}


def test_finding_010_clean_merge_keeps_parent_evidence_requirements():
    case, unlisted = recorded_gate("unlisted_clean_parents")
    assert unlisted == {
        case["ids"]["left"]: ["modules/perps/keeper/orders.go"],
        case["ids"]["right"]: ["precompiles/perps/positions.go"],
    }
    assert case["ids"]["merge"] not in unlisted


def test_finding_010_uses_the_ledger_from_the_recorded_head():
    before, refused = recorded_gate("merge_only")
    after, accepted = recorded_gate("merge_listed")
    assert before["ids"]["merge"] == after["ids"]["merge"]
    assert refused == {before["ids"]["merge"]: [RULE]}
    assert accepted == {}
    assert GATE.unlisted_from_history(before["ledger"], before["history"]) == refused


def test_finding_010_nul_codec_preserves_consensus_filenames():
    case, unlisted = recorded_gate("unusual_filename")
    assert unlisted == {
        case["ids"]["commit"]: ["node/sequencer/rule\nwith\ttab.go"]
    }


def test_finding_010_test_only_merge_is_not_a_consensus_change():
    case, unlisted = recorded_gate("test_only_merge")
    assert case["ids"]["merge"] in GATE.consensus_changes_from_history(
        case["history"]
    )
    assert unlisted == {}


def test_finding_010_root_consensus_commit_still_needs_evidence():
    case, unlisted = recorded_gate("root_commit")
    assert unlisted == {case["ids"]["commit"]: [RULE]}


def test_finding_010_incomplete_recording_fails_closed():
    case = RECORDING["cases"]["root_commit"]
    fields = case["history"].split("\0")
    with pytest.raises(ValueError, match="Missing raw consensus history path"):
        GATE.unlisted_from_history(case["ledger"], "\0".join(fields[:-2]))
