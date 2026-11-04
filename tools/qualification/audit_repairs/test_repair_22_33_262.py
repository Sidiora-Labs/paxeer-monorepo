"""Offline source proofs; hf-build separately parses the shell with bash -n.

No Rust/native harness or shell subprocess is executed by this gate. The native
producer extension is owned by domain 13 and explicitly recorded as wiring.
"""

import ast
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "programs/tests/ai-market/run-aggregation-runtime.sh"
SUITE = ROOT / "programs/crates/layerx-programs-ai-market/tests/f05_aggregation_runtime.rs"
RECORD = Path(__file__).with_name("repair_22_33_262.kvx")
NATIVE_CASE = "f05_a16_native_fuel_refusal_keeps_committed_progress"
EXISTING_CASES = (
    ("AI.F05-A01", "f05_a01_three_distinct_scores_select_the_median"),
    ("AI.F05-A02", "f05_a02_four_scores_select_the_lower_median"),
    ("AI.F05-A03", "f05_a03_zero_quorum_and_insufficient_quorum_stay_distinct"),
    ("AI.F05-A04", "f05_a04_absent_reports_add_no_observations"),
    ("AI.F05-A05", "f05_a05_arrival_order_never_changes_bytes_and_seal_height_does"),
    ("AI.F05-A06", "f05_a06_canonical_worker_order_and_display_shares"),
    ("AI.F05-A07", "f05_a07_display_truncation_and_weight_settlement"),
    ("AI.F05-A08", "f05_a08_largest_roster_in_four_chunks"),
    ("AI.F05-A09", "f05_a09_no_reports_settles_no_eligible_score"),
    ("AI.F05-A10", "f05_a10_refused_scores_and_tampered_persisted_records"),
    ("AI.F05-A11", "f05_a11_self_assessment_and_unbacked_persisted_votes"),
    ("AI.F05-A12", "f05_a12_settlement_window_and_delayed_completion"),
    ("AI.F05-A13", "f05_a13_cursor_retries_and_once_only_completion"),
    ("AI.F05-A14", "f05_a14_revocation_orderings"),
    ("AI.F05-A15", "f05_a15_worker_replacement_after_freeze"),
    ("AI.F05-A16", "f05_a16_refused_steps_keep_committed_progress"),
    ("AI.F05-A17", "f05_a17_colluding_low_reports_select_zero"),
    ("AI.F05-A18", "f05_a18_settlement_reads_no_evidence"),
    ("AI.F04-A06", "f04_a06_expired_commitments_supply_no_rows"),
    ("AI.F04-A07", "f05_a03_zero_quorum_and_insufficient_quorum_stay_distinct"),
    ("AI.F04-A11", "f04_a11_stable_set_at_reveal_end"),
    ("AI.F04-A12", "f05_a14_revocation_orderings"),
    ("AI.F04-A12", "f05_a12_settlement_window_and_delayed_completion"),
)


def script_parts():
    source = SCRIPT.read_text(encoding="utf-8")
    before, cases, after = re.fullmatch(
        r"(.*?)done <<'CASES'\n(.*?)\nCASES\n(.*)", source, re.DOTALL
    ).groups()
    return before, [tuple(shlex.split(row)) for row in cases.splitlines()], after


def test_required_matrix_retains_every_existing_clause_and_requires_native_fuel():
    _, cases, _ = script_parts()
    assert cases.count(("AI.F05-A16", NATIVE_CASE)) == 1
    assert [row for row in cases if row[1] != NATIVE_CASE] == list(EXISTING_CASES)
    assert len(cases) == 24
    suite = SUITE.read_text(encoding="utf-8")
    for _, name in EXISTING_CASES:
        assert re.search(r"#\[test\]\s*fn " + re.escape(name) + r"\(", suite), name
    assert "let outcome = agg::apply(" in suite
    assert "self.compose(call, at, FULL)?" in suite


def test_command_executes_the_real_suite_and_captures_its_status_immediately():
    before, _, _ = script_parts()
    setup, invocation = before.split('"$programs_cargo" test', 1)
    assert 'programs_cargo=${PROGRAMS_CARGO:-cargo}' in setup
    assert 'cd "$repository_root/programs" || exit 1' in setup
    command, consumer = invocation.split("\ncargo_status=$?\n", 1)
    assert shlex.split('"$programs_cargo" test' + command.replace("\\\n", "")) == [
        "$programs_cargo", "test", "--locked", "-p", "layerx-programs-ai-market",
        "--test", "f05_aggregation_runtime", "--", "--test-threads=1",
        ">$log", "2>&1",
    ]
    assert consumer.startswith("\nfailed=0\nwhile read -r case name; do\n")
    manifest = tomllib.loads((ROOT / "programs/crates/layerx-programs-ai-market/Cargo.toml").read_text())
    assert manifest["package"]["name"] == "layerx-programs-ai-market"
    assert manifest["dev-dependencies"]["layerx-programs-runtime"]["path"] == "../layerx-programs-runtime"


def test_result_consumer_requires_each_named_ok_and_accumulates_missing_or_failed_cases():
    before, _, _ = script_parts()
    loop = before.split("while read -r case name; do\n", 1)[1]
    assert loop == (
        '    if grep -q "^test $name \\.\\.\\. ok\\$" "$log"; then\n'
        '        echo "$case PASS $name"\n'
        '    else\n'
        '        echo "$case FAIL $name"\n'
        '        failed=1\n'
        '    fi\n'
    )
    assert "NOT_RUN" not in SCRIPT.read_text()
    # Every case uses this same consumer, including the new native clause;
    # absent, ignored, filtered, and failed cases cannot supply an exact ok line.
    assert "failed=0" not in loop


def test_exit_zero_is_reachable_only_after_the_complete_matrix_and_both_guards():
    before, _, after = script_parts()
    assert after.strip() == (
        'if [ "$cargo_status" -ne 0 ] || [ "$failed" -ne 0 ]; then\n'
        '    cat "$log"\n'
        '    exit 1\n'
        'fi\n'
        'exit 0'
    )
    assert not re.search(r"\b(?:exit|return)\s+0\b", before)
    assert before.count('trap \'rm -f "$log"\' EXIT') == 1


def test_record_maps_finding_to_source_proofs_and_names_the_external_producer():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["aggregation-runner-incomplete"]
    assert finding["audit"] == "22.30.130"
    assert finding["file"] == str(SCRIPT.relative_to(ROOT))
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 5
    wiring = record["wiring"]["native-fuel"]
    assert wiring["owner_domain"] == 13
    assert wiring["symbol"] == NATIVE_CASE
    assert wiring["paths"] == [
        str(SUITE.relative_to(ROOT)), "tests/programs/test_ai_host_boundary.c"
    ]
    task = record["task"]["22"]["33"]["262"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
