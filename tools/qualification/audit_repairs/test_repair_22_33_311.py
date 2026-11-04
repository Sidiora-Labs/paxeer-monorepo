"""Prove copy divergence coverage without invoking Docker, psql or a toolchain.

The shell syntax gate runs bash -n. These tests inspect its guarded scenarios
and drive the production CopyRun/PsqlSession over committed protocol fixtures.
"""

import importlib.util
import json
from pathlib import Path
import re
import shlex

import pytest


ROOT = Path(__file__).resolve().parents[3]
TOOLS = ROOT / "services/explorer/deploy/tools"
SCRIPT = TOOLS / "tests/copy-blockscout-test.sh"
SPEC = importlib.util.spec_from_file_location(
    "copy_protocol_support_311", TOOLS / "test_repair_22_33_310.py"
)
SUPPORT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SUPPORT)
COPY = SUPPORT.copy
FIXTURE = SUPPORT.TRANSCRIPT


def shell_function(name):
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n(.*?)^\}",
                      SCRIPT.read_text(), re.MULTILINE | re.DOTALL)
    assert match, f"missing shell function {name}"
    return match.group(1).replace("\\\n", "")


def assert_ordered(body, *commands):
    offset = 0
    for command in commands:
        found = body.find(command, offset)
        assert found >= 0, f"missing or out-of-order command: {command}"
        offset = found + len(command)


def test_source_mutation_scenario_checks_retained_content_and_progress():
    body = shell_function("scenario_recovery_content_divergence")
    assert_ordered(
        body,
        "reset_destination", "install_interruption",
        "run_tool content-interrupted BLOCK_BATCH=2",
        "assert_rc 2 'content interruption'",
        "SELECT value FROM transactions WHERE block_number = 1' 1000",
        "retained_transaction=$(dst_sql",
        "remove_interruption",
        "UPDATE transactions SET value = 999 WHERE block_number = 1",
        "SELECT value FROM transactions WHERE block_number = 1' 999",
        "run_tool changed-source BLOCK_BATCH=2",
        "assert_rc 2 'changed source after interruption'",
        "source keys or content changed since the interrupted snapshot",
        "SELECT value FROM transactions WHERE block_number = 1' 1000",
        'assert_value "$retained_transaction"',
        "SELECT highest_key FROM paxeer_x_copy_progress WHERE table_name = 'logs'",
        "UPDATE transactions SET value = 1000 WHERE block_number = 1",
        "run_tool restored-source BLOCK_BATCH=2",
        "assert_rc 0 'recovery after explicit source reconciliation'",
        'assert_value "$retained_transaction"',
        "every processed table matches under its cut",
    )
    tokens = shlex.split(body, comments=True)
    assert body.count("SELECT row_to_json(t)::text FROM transactions AS t WHERE block_number = 1") == 3
    assert "SELECT count(*) FROM transactions" in tokens
    assert body.count("WHERE table_name = 'logs'\" 3") == 2


def test_wrong_key_scenario_checks_identity_and_complete_retained_row():
    body = shell_function("scenario_equal_count_wrong_key")
    assert_ordered(
        body,
        "SELECT count(*) FROM logs' 18 'source count before key substitution'",
        "SELECT count(*) FROM logs' 18 'destination count before key substitution'",
        'UPDATE logs SET "index" = 99 WHERE block_number = 3 AND "index" = 0',
        "retained_log=$(dst_sql",
        "run_tool wrong-key", "assert_rc 3 'equal-count wrong-key destination'",
        "MISMATCH (keys or content differ)",
        "SELECT count(*) FROM logs' 18 'destination log count remains unchanged'",
        "SELECT count(*) FROM logs' 18 'source log count equals the destination count'",
        '''SELECT count(*) FROM logs WHERE block_number = 3 AND "index" = 0' 0''',
        '''SELECT count(*) FROM logs WHERE block_number = 3 AND "index" = 0' 1''',
        'assert_value "$retained_log"',
    )
    tokens = shlex.split(body, comments=True)
    assert body.count('SELECT row_to_json(l)::text FROM logs AS l WHERE block_number = 3 AND "index" = 99') == 2
    assert 'SELECT count(*) FROM logs WHERE "index" = 99' in tokens


def test_scenarios_invoke_real_tool_and_propagate_assertion_failures():
    assert 'readonly TOOL="${HERE}/../copy-blockscout-11-to-10.sh"' in SCRIPT.read_text()
    invocation = shell_function("run_tool")
    assert 'env SRC_DATABASE_URL="$SRC_URL" DST_DATABASE_URL="$DST_URL" "$@"' in invocation
    assert '"$TOOL" > "$LAST_LOG" 2>&1 || LAST_RC=$?' in invocation
    assert '"$expected" = "$LAST_RC"' in shell_function("assert_rc")
    assert '"$expected" = "$actual"' in shell_function("assert_value")
    assert 'actual=$("${side}_sql" "$query")' in shell_function("assert_count")
    assert 'FAILURES=$((FAILURES + 1))' in shell_function("fail")
    assert_ordered(
        shell_function("main"),
        "  scenario_recovery_content_divergence\n",
        "  scenario_equal_count_wrong_key\n",
        'if [ "$FAILURES" -ne 0 ]; then', "exit 1",
    )


def commit_sql(high_key):
    return (
        'WITH inserted AS (INSERT INTO public."balances" (' + FIXTURE["columns"] + ') '
        'SELECT ' + FIXTURE["columns"] + ' FROM public."paxeer_x_copy_stage" '
        'ON CONFLICT DO NOTHING RETURNING 1), '
        'counted AS (SELECT count(*)::bigint AS row_count FROM inserted), '
        'recorded AS (INSERT INTO public."paxeer_x_copy_progress" AS p '
        '(table_name, ceiling, highest_key, rows_copied, updated_at) '
        f"SELECT 'balances', 6, {high_key}, counted.row_count, now() FROM counted "
        'ON CONFLICT (table_name) DO UPDATE SET ceiling = EXCLUDED.ceiling, '
        'highest_key = GREATEST(p.highest_key, EXCLUDED.highest_key), '
        'rows_copied = p.rows_copied + EXCLUDED.rows_copied, '
        'updated_at = now()) SELECT row_count FROM counted'
    )


def copied_rows(peer, index):
    """Read the actual COPY payload emitted to the committed transcript peer."""
    lines = peer.seen[index].splitlines(keepends=True)
    assert lines[0] == FIXTURE["copy_from_sql"] + ";\n"
    assert lines[-1] == "\\.\n"
    return lines[1:-1]


def json_rows(rows):
    result = []
    for row in rows:
        key, block, balance = row.rstrip("\n").split("\t")
        result.append(json.dumps(dict(hash=key, block_number=int(block), balance=int(balance)),
                                 separators=(",", ":")) + "\n")
    return sorted(result)


def interrupt_after_first_commit():
    source, destination = SUPPORT.acquire_steps()
    source += SUPPORT.probes() + SUPPORT.probes()
    source += [(FIXTURE["fingerprint_sql"], "".join(FIXTURE["rows"]))]
    destination += SUPPORT.boundary_steps() + SUPPORT.prepare_steps()
    first_sql = FIXTURE["copy_sql"].replace('<= 6', '<= 3')
    rest_sql = FIXTURE["copy_sql"].replace('<= 6', '> 3 AND "block_number" <= 6')
    source += SUPPORT.probes() + [(first_sql, FIXTURE["copy_rows"][0])]
    destination += SUPPORT.probes() + [(FIXTURE["copy_from_sql"], "", FIXTURE["copy_rows"][:1])]
    source += SUPPORT.probes()
    destination += SUPPORT.probes() + [(commit_sql(3), "1\n")]
    source += SUPPORT.probes() + [(rest_sql, FIXTURE["copy_rows"][1])]
    destination += SUPPORT.probes() + [(FIXTURE["copy_from_sql"], None, FIXTURE["copy_rows"][1:])]
    with SUPPORT.peers(source, destination) as (run, _, store):
        SUPPORT.begin(run)
        manifest = json.dumps(SUPPORT.prepare(run), sort_keys=True)
        persisted = dict(run.boundary)
        assert COPY.literal(json.dumps(persisted, sort_keys=True)) in store.seen[4]
        assert COPY.literal(manifest) in store.seen[7]
        run.copy("balances", FIXTURE["columns"], FIXTURE["columns"], 'WHERE "block_number" <= 3')
        assert run.commit("balances", FIXTURE["columns"], "6", "3") == "1"
        retained = copied_rows(store, 9)
        with pytest.raises(COPY.CopyError, match="session lost"):
            run.copy("balances", FIXTURE["columns"], FIXTURE["columns"],
                     'WHERE "block_number" > 3 AND "block_number" <= 6')
        assert run.source.failed and run.destination.failed
        with pytest.raises(COPY.CopyError, match="lost its database session"):
            run.commit("balances", FIXTURE["columns"], "6", "6")
        assert store.seen[-2] == "SELECT 1;\n"
        assert retained == ["a\t2\t100\n"]
    return persisted, manifest, retained


def test_interrupted_copy_refuses_same_key_mutation_and_recovers_retained_values():
    persisted, manifest, retained = interrupt_after_first_commit()
    original = [json.loads(row) for row in FIXTURE["rows"]]
    changed = [json.loads(row) for row in FIXTURE["same_key_changed"]]
    assert [(row["hash"], row["block_number"]) for row in original] == [
        (row["hash"], row["block_number"]) for row in changed
    ]
    assert original[0]["balance"] == 100 and changed[0]["balance"] == 999
    source, destination = SUPPORT.acquire_steps(FIXTURE["snapshot_reopened"])
    source += SUPPORT.probes() + SUPPORT.probes()
    source += [(FIXTURE["fingerprint_sql"], "".join(FIXTURE["same_key_changed"]))]
    destination += SUPPORT.boundary_steps(persisted) + SUPPORT.prepare_steps(manifest, refuse=True)
    read_retained = 'COPY (SELECT "hash", "block_number", "balance" FROM public."balances") TO STDOUT'
    source += SUPPORT.probes()
    destination += SUPPORT.probes() + [(read_retained, "".join(retained))]
    with SUPPORT.peers(source, destination) as (run, _, store):
        run.acquire()
        assert run.boundary_at("9", "later", "9999999999", "balances") == persisted
        assert run.snapshot != persisted["snapshot"]
        with pytest.raises(COPY.CopyError, match="source keys or content changed since the interrupted snapshot"):
            SUPPORT.prepare(run)
        assert run.expected == {}
        assert run.dispatch("dst", [read_retained]).splitlines() == ["a\t2\t100"]
        assert len(store.seen) == 8  # No COPY, INSERT or progress write after refusal.

    source, destination = SUPPORT.acquire_steps(FIXTURE["snapshot_reopened"])
    source += SUPPORT.probes() + SUPPORT.probes()
    source += [(FIXTURE["fingerprint_sql"], "".join(FIXTURE["rows"]))]
    destination += SUPPORT.boundary_steps(persisted) + SUPPORT.prepare_steps(manifest)
    source += SUPPORT.probes() + [(FIXTURE["copy_sql"], "".join(FIXTURE["copy_rows"]))]
    destination += SUPPORT.probes() + [(FIXTURE["copy_from_sql"], "", FIXTURE["copy_rows"])]
    source += SUPPORT.probes() + SUPPORT.probes()
    destination += SUPPORT.probes() + [(commit_sql(6), "1\n")]
    destination += SUPPORT.probes() + [(FIXTURE["fingerprint_sql"], "".join(json_rows(FIXTURE["copy_rows"])))]
    with SUPPORT.peers(source, destination) as (run, _, store):
        run.acquire()
        assert run.boundary_at("9", "later", "9999999999", "balances") == persisted
        assert json.dumps(SUPPORT.prepare(run), sort_keys=True) == manifest
        run.copy("balances", FIXTURE["columns"], FIXTURE["columns"], FIXTURE["predicate"])
        assert run.commit("balances", FIXTURE["columns"], "6", "6") == "1"
        recovered = copied_rows(store, 8)
        assert recovered[:1] == retained
        assert json_rows(recovered) == FIXTURE["rows"]
        assert run.verify("balances", FIXTURE["columns"], FIXTURE["predicate"]) == dict(
            source=2, target=2, matches=True
        )


def test_equal_count_wrong_key_rejected_by_production_content_verifier():
    source_rows = [json.loads(row) for row in FIXTURE["rows"]]
    wrong_rows = [json.loads(row) for row in FIXTURE["wrong_key"]]
    assert len(source_rows) == len(wrong_rows) == 2
    assert {row["hash"] for row in source_rows} != {row["hash"] for row in wrong_rows}
    source, destination = SUPPORT.acquire_steps()
    source += SUPPORT.probes() + SUPPORT.probes()
    source += [(FIXTURE["fingerprint_sql"], "".join(FIXTURE["rows"]))] + SUPPORT.probes()
    destination += SUPPORT.boundary_steps() + SUPPORT.prepare_steps() + SUPPORT.probes()
    destination += [(FIXTURE["fingerprint_sql"], "".join(FIXTURE["wrong_key"]))]
    with SUPPORT.peers(source, destination) as (run, _, store):
        SUPPORT.begin(run)
        SUPPORT.prepare(run)
        assert run.dispatch("verify", ["balances", FIXTURE["columns"], FIXTURE["predicate"]]) == "2 2 0"
        assert store.seen[-1] == FIXTURE["fingerprint_sql"] + ";\n"
        assert len(store.seen) == 10  # Verification emits a read, no corrective write.
