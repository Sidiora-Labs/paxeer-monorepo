"""Exercise the production session owner over committed psql/COPY transcripts.

The SQL peers replay committed protocol results over real sockets. No PostgreSQL,
psql, Go, environment configuration or subprocess is needed by this gate.
"""

import contextlib
import hashlib
import importlib.util
import json
from pathlib import Path
import socket
import threading
from types import SimpleNamespace

import pytest


HERE = Path(__file__).resolve().parent
TRANSCRIPT = json.loads((HERE / "fixtures/copy-m310-transcripts.json").read_text())


def load_module(name, filename):
    spec = importlib.util.spec_from_file_location(name, HERE / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


copy = load_module("copy_session_m310", "copy_session.py")


class TranscriptPeer:
    """A loopback psql protocol peer with exact SQL and COPY input assertions."""

    def __init__(self, steps):
        client, self.server = socket.socketpair()
        client.settimeout(5)
        self.server.settimeout(5)
        self.client = client
        self.session = copy.PsqlSession(
            client.makefile("r", encoding="utf-8"),
            client.makefile("w", encoding="utf-8"),
        )
        self.steps = steps
        self.errors = []
        self.seen = []
        self.pause_sql = None
        self.paused = threading.Event()
        self.resume = threading.Event()
        self.thread = threading.Thread(target=self.replay, daemon=True)
        self.thread.start()

    def replay(self):
        try:
            with self.server, self.server.makefile("r", encoding="utf-8") as reader, \
                    self.server.makefile("w", encoding="utf-8") as writer:
                for step in self.steps:
                    lines = []
                    while True:
                        line = reader.readline()
                        assert line, "session closed before the transcript was consumed"
                        if line.startswith("\\echo paxeer_copy_"):
                            marker = line[len("\\echo "):]
                            break
                        lines.append(line)
                    actual = "".join(lines)
                    expected = step[0].rstrip().rstrip(";") + ";\n"
                    if len(step) == 3:
                        expected += "".join(step[2]) + "\\.\n"
                    assert actual == expected
                    self.seen.append(actual)
                    if step[0] == self.pause_sql:
                        self.paused.set()
                        assert self.resume.wait(5), "owner operation was never released"
                    if step[1] is None:
                        return  # A lost database connection, before the marker.
                    writer.write(step[1])
                    writer.write(marker)
                    writer.flush()
                assert reader.readline() == "", "unexpected command after the transcript"
        except Exception as error:
            self.errors.append(error)

    def close(self):
        self.session.close()
        self.client.close()
        self.thread.join(6)
        assert not self.thread.is_alive(), "transcript peer did not stop"
        assert not self.errors, self.errors
        assert len(self.seen) == len(self.steps)


@contextlib.contextmanager
def peers(source_steps, destination_steps):
    source = TranscriptPeer(source_steps)
    destination = TranscriptPeer(destination_steps)
    run = copy.CopyRun(source.session, destination.session, "source-fixture")
    try:
        yield run, source, destination
    finally:
        source.close()
        destination.close()


def acquire_steps(snapshot=None, lock="t"):
    source = []
    if lock == "t":
        source = [
            ("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY", ""),
            ("SELECT pg_export_snapshot()", (snapshot or TRANSCRIPT["snapshot_first"]) + "\n"),
        ]
    return source, [(copy.LOCK_SQL, lock + "\n")]


def probes():
    return [("SELECT 1", "1\n")]


def boundary(snapshot=None):
    return dict(ceiling="6", timestamp="2026-01-01 00:00:00.000000", epoch="1767225600.000000",
                tables="balances", source="source-fixture",
                snapshot=snapshot or TRANSCRIPT["snapshot_first"])


def boundary_steps(previous=None, snapshot=None):
    current = previous or boundary(snapshot)
    steps = probes() + [
        ('CREATE TABLE IF NOT EXISTS public."paxeer_x_copy_run" ('
         'singleton boolean PRIMARY KEY CHECK (singleton), '
         'boundary text NOT NULL, completed boolean NOT NULL DEFAULT false)', ""),
        ('SELECT boundary FROM public."paxeer_x_copy_run" WHERE singleton AND NOT completed',
         json.dumps(previous, sort_keys=True) + "\n" if previous else ""),
    ]
    if previous is None:
        steps.append((
            'INSERT INTO public."paxeer_x_copy_run" (singleton, boundary, completed) VALUES '
            f"(true, {copy.literal(json.dumps(current, sort_keys=True))}, false) "
            'ON CONFLICT (singleton) DO UPDATE SET boundary = EXCLUDED.boundary, completed = false', ""))
    return steps


def manifest(rows=None):
    rows = rows if rows is not None else TRANSCRIPT["rows"]
    return json.dumps(dict(count=len(rows), digest=hashlib.sha256("".join(rows).encode()).hexdigest(),
                           columns=TRANSCRIPT["columns"]), sort_keys=True)


def prepare_steps(previous="", snapshot=None, refuse=False):
    snapshot = snapshot or TRANSCRIPT["snapshot_first"]
    steps = probes() + [(
        'SELECT source_fingerprint FROM public."paxeer_x_copy_progress" '
        f"WHERE table_name = 'balances' AND snapshot_id = '{snapshot}'", previous + "\n" if previous else "")]
    if not refuse:
        steps.append((
            'INSERT INTO public."paxeer_x_copy_progress" '
            '(table_name, ceiling, snapshot_id, source_fingerprint) '
            f"VALUES ('balances', 6, '{snapshot}', {copy.literal(manifest())}) "
            'ON CONFLICT (table_name) DO UPDATE SET snapshot_id = EXCLUDED.snapshot_id, '
            'source_fingerprint = EXCLUDED.source_fingerprint', ""))
    return steps


def begin(run):
    run.acquire()
    result = run.boundary_at("6", "2026-01-01 00:00:00.000000", "1767225600.000000", "balances")
    assert result == boundary()


def prepare(run):
    return run.prepare("balances", TRANSCRIPT["columns"], TRANSCRIPT["columns"], TRANSCRIPT["predicate"])


@pytest.mark.parametrize("phase", ["stage_creation", "copy", "progress_commit"])
def test_TestRepairM_310_copy_stage_concurrency(phase):
    source, destination = acquire_steps()
    stage = 'CREATE UNLOGGED TABLE public."paxeer_x_copy_stage" AS SELECT "hash", "block_number", "balance" FROM public."balances" WITH NO DATA'
    commit = ('WITH inserted AS (INSERT INTO public."balances" ("hash", "block_number", "balance") '
              'SELECT "hash", "block_number", "balance" FROM public."paxeer_x_copy_stage" '
              'ON CONFLICT DO NOTHING RETURNING 1), counted AS (SELECT count(*)::bigint AS row_count FROM inserted), '
              'recorded AS (INSERT INTO public."paxeer_x_copy_progress" AS p '
              '(table_name, ceiling, highest_key, rows_copied, updated_at) '
              "SELECT 'balances', 6, 6, counted.row_count, now() FROM counted "
              'ON CONFLICT (table_name) DO UPDATE SET ceiling = EXCLUDED.ceiling, '
              'highest_key = GREATEST(p.highest_key, EXCLUDED.highest_key), '
              'rows_copied = p.rows_copied + EXCLUDED.rows_copied, updated_at = now()) SELECT row_count FROM counted')
    source += probes() + probes() + [(TRANSCRIPT["copy_sql"], "".join(TRANSCRIPT["copy_rows"]))] + probes()
    destination += probes() + [(stage, "")] + probes()
    destination += [(TRANSCRIPT["copy_from_sql"], "", TRANSCRIPT["copy_rows"])]
    destination += probes() + [(commit, "2\n")]
    with peers(source, destination) as (owner, source_peer, destination_peer):
        owner.acquire()
        destination_peer.pause_sql = dict(stage_creation=stage, copy=TRANSCRIPT["copy_from_sql"],
                                          progress_commit=commit)[phase]
        owner_errors = []
        result = []

        def copy_and_commit():
            try:
                owner.dispatch("dst", [stage])
                owner.dispatch("copy", ["balances", TRANSCRIPT["columns"], TRANSCRIPT["columns"], TRANSCRIPT["predicate"]])
                result.append(owner.dispatch("commit", ["balances", TRANSCRIPT["columns"], "6", "6"]))
            except Exception as error:
                owner_errors.append(error)

        worker = threading.Thread(target=copy_and_commit, daemon=True)
        worker.start()
        try:
            assert destination_peer.paused.wait(5), "owner never reached the contested operation"
            assert worker.is_alive()
            # The competing connection is refused while the owner is inside
            # stage creation, COPY completion or the atomic progress commit.
            contender_source, contender_destination = acquire_steps(lock="f")
            with peers(contender_source, contender_destination) as (contender, _, rejected):
                with pytest.raises(copy.CopyError, match="another copy run owns this destination"):
                    contender.acquire()
                assert not contender.owned
                assert len(rejected.seen) == 1
            assert owner.owned
        finally:
            destination_peer.resume.set()
            worker.join(6)
        assert not worker.is_alive()
        assert not owner_errors
        assert result == ["2"]
        assert destination_peer.seen[0] == copy.LOCK_SQL + ";\n"
        assert source_peer.seen[0].startswith("BEGIN ISOLATION LEVEL REPEATABLE READ")
    # Closing the owner ends its connection. A new owner must acquire a fresh
    # lock and start a fresh source snapshot rather than silently reconnecting.
    source, destination = acquire_steps(TRANSCRIPT["snapshot_reopened"])
    with peers(source, destination) as (reopened, _, __):
        reopened.acquire()
        assert reopened.snapshot == TRANSCRIPT["snapshot_reopened"]


@pytest.mark.parametrize("destination_rows", ["rows", "same_key_changed", "wrong_key"])
def test_TestRepairM_310_restart_recovers_snapshot_and_content(destination_rows):
    source, destination = acquire_steps()
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))]
    destination += boundary_steps() + prepare_steps()
    with peers(source, destination) as (run, _, saved):
        begin(run)
        expected = prepare(run)
        assert expected["count"] == 2
        assert copy.literal(manifest()) in saved.seen[-1]
        persisted_boundary = dict(run.boundary)
        persisted_manifest = json.dumps(expected, sort_keys=True)
    # Reopen both real session/transport components, retaining the progress
    # emitted by the production producer above as the store transcript result.
    source, destination = acquire_steps(TRANSCRIPT["snapshot_reopened"])
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))] + probes()
    destination += boundary_steps(persisted_boundary) + prepare_steps(persisted_manifest) + probes()
    destination += [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT[destination_rows]))]
    with peers(source, destination) as (run, _, __):
        run.acquire()
        recovered = run.boundary_at("9", "later", "9999999999", "balances")
        assert recovered == persisted_boundary
        assert run.snapshot != recovered["snapshot"]
        assert prepare(run) == expected
        assert run.verify("balances", TRANSCRIPT["columns"], TRANSCRIPT["predicate"]) == dict(source=2, target=2, matches=destination_rows == "rows")


@pytest.mark.parametrize("rows", ["same_key_changed", "wrong_key"])
def test_TestRepairM_310_restart_refuses_changed_source(rows):
    source, destination = acquire_steps()
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))]
    destination += boundary_steps() + prepare_steps()
    with peers(source, destination) as (run, _, __):
        begin(run)
        persisted_manifest = json.dumps(prepare(run), sort_keys=True)
        persisted_boundary = dict(run.boundary)
    source, destination = acquire_steps(TRANSCRIPT["snapshot_reopened"])
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT[rows]))]
    destination += boundary_steps(persisted_boundary) + prepare_steps(persisted_manifest, refuse=True)
    with peers(source, destination) as (run, _, store):
        run.acquire()
        run.boundary_at("9", "later", "9999999999", "balances")
        with pytest.raises(copy.CopyError, match="source keys or content changed since the interrupted snapshot"):
            prepare(run)
        assert store.seen[-1].startswith("SELECT source_fingerprint")


@pytest.mark.parametrize("rows", ["same_key_changed", "wrong_key"])
def test_TestRepairM_310_equal_counts_do_not_verify_divergence(rows):
    source, destination = acquire_steps()
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))] + probes()
    destination += boundary_steps() + prepare_steps() + probes()
    destination += [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT[rows]))]
    with peers(source, destination) as (run, _, __):
        begin(run)
        prepare(run)
        result = run.verify("balances", TRANSCRIPT["columns"], TRANSCRIPT["predicate"])
        assert result == dict(source=2, target=2, matches=False)


def test_TestRepairM_310_success_allows_a_new_boundary():
    source, destination = acquire_steps()
    source += probes() + probes() + [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))] + probes() + probes()
    destination += boundary_steps() + prepare_steps() + probes()
    destination += [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))] + probes()
    destination += [('UPDATE public."paxeer_x_copy_run" SET completed = true WHERE singleton', "")]
    with peers(source, destination) as (run, _, store):
        begin(run)
        prepare(run)
        assert run.verify("balances", TRANSCRIPT["columns"], TRANSCRIPT["predicate"])["matches"]
        run.complete()
        assert store.seen[-1] == 'UPDATE public."paxeer_x_copy_run" SET completed = true WHERE singleton;\n'
    source, destination = acquire_steps(TRANSCRIPT["snapshot_reopened"])
    source += probes()
    destination += boundary_steps(snapshot=TRANSCRIPT["snapshot_reopened"])
    with peers(source, destination) as (run, _, __):
        run.acquire()
        advanced = run.boundary_at("6", "2026-01-01 00:00:00.000000", "1767225600.000000", "balances")
        assert advanced == boundary(TRANSCRIPT["snapshot_reopened"])


@pytest.mark.parametrize("field,value", [("source", "different-source"), ("tables", "different-table")])
def test_TestRepairM_310_restart_refuses_a_different_owner_boundary(field, value):
    previous = boundary()
    previous[field] = value
    source, destination = acquire_steps(TRANSCRIPT["snapshot_reopened"])
    source += probes()
    destination += boundary_steps(previous)
    with peers(source, destination) as (run, _, store):
        run.acquire()
        with pytest.raises(copy.CopyError, match="different source or table list"):
            run.boundary_at("6", "2026-01-01 00:00:00.000000", "1767225600.000000", "balances")
        assert store.seen[-1].startswith("SELECT boundary")


def test_TestRepairM_310_derived_projection_hashes_copied_values():
    projection = '\"hash\", \"block_number\", COALESCE(\"balance\", 0) AS \"balance\"'
    sql = TRANSCRIPT["fingerprint_sql"].replace(TRANSCRIPT["columns"], projection)
    with peers([(sql, "".join(TRANSCRIPT["rows"]))],
               [(TRANSCRIPT["fingerprint_sql"], "".join(TRANSCRIPT["rows"]))]) as (run, _, __):
        source = copy.fingerprint(run.source, "balances", projection, TRANSCRIPT["predicate"])
        destination = copy.fingerprint(run.destination, "balances", TRANSCRIPT["columns"], TRANSCRIPT["predicate"])
        assert source == destination
        assert source["count"] == 2


def test_TestRepairM_310_session_loss_refuses_progress():
    source, destination = acquire_steps()
    source += [("SELECT 1", None)]
    with peers(source, destination) as (run, _, store):
        run.acquire()
        with pytest.raises(copy.CopyError, match="session lost"):
            run.dispatch("dst", ['TRUNCATE public."paxeer_x_copy_stage"'])
        with pytest.raises(copy.CopyError, match="lost its database session"):
            run.dispatch("dst", ['UPDATE public."paxeer_x_copy_progress" SET highest_key = 6'])
        assert len(store.seen) == 1


def test_TestRepairM_310_failed_copy_cannot_commit_progress():
    source, destination = acquire_steps()
    source += probes() + [(TRANSCRIPT["copy_sql"], "".join(TRANSCRIPT["copy_rows"]))]
    destination += probes() + [(TRANSCRIPT["copy_from_sql"], None, TRANSCRIPT["copy_rows"])]
    with peers(source, destination) as (run, _, store):
        run.acquire()
        with pytest.raises(copy.CopyError, match="session lost"):
            run.copy("balances", TRANSCRIPT["columns"], TRANSCRIPT["columns"], TRANSCRIPT["predicate"])
        assert run.source.failed and run.destination.failed
        with pytest.raises(copy.CopyError, match="lost its database session"):
            run.dispatch("dst", ['UPDATE public."paxeer_x_copy_progress" SET highest_key = 6'])
        assert len(store.seen) == 3


def test_TestRepairM_310_rpc_uses_production_sessions(tmp_path):
    source, destination = acquire_steps()
    source += probes()
    destination += probes() + [("SELECT 42", "42\n")]
    path = str(tmp_path / "copy.sock")
    errors = []
    with peers(source, destination) as (run, _, __), socket.socket(socket.AF_UNIX) as listener:
        run.acquire()
        listener.bind(path)
        listener.listen(1)
        listener.settimeout(5)

        def serve_one():
            try:
                connection, _ = listener.accept()
                with connection, connection.makefile("r", encoding="utf-8") as reader, \
                        connection.makefile("w", encoding="utf-8") as writer:
                    copy.handle_request(run, reader, writer)
            except Exception as error:
                errors.append(error)

        thread = threading.Thread(target=serve_one, daemon=True)
        thread.start()
        assert copy.request(path, "dst", ["SELECT 42"]) == "42"
        thread.join(6)
        assert not thread.is_alive()
        assert not errors


def test_TestRepairM_310_shell_routes_every_write_through_owner():
    script = (HERE / "copy-blockscout-11-to-10.sh").read_text()
    assert 'session_request src "$1"' in script
    assert 'session_request dst "$1"' in script
    assert 'session_request copy "$table" "$column_list" "$select_list" "$predicate"' in script
    assert script.index("  start_sessions\n") < script.index('  src_query "SELECT 1"')
    assert 'session_request prepare "$table" "$column_list" "$select_list" "$source_predicate"' in script
    assert 'session_request verify "$table" "${TABLE_COLUMN_LIST[$table]}" "$dst_predicate"' in script
    assert 'if [ "${TABLE_MATCHES[$table]}" != "1" ]; then' in script
    assert script.index('  session_request complete') > script.index('  if ! print_summary; then')
    assert 'session_request commit "$table" "$column_list" "$CEILING" "$high_key"' in script
    assert "--dbname" not in script  # No independent, unlocked SQL/COPY session.


def test_TestRepairM_310_swagger_retained_surfaces():
    swagger = load_module("swagger_m310", "gen-swagger.py")
    args = SimpleNamespace(chain_type="paxeer_x", include_admin=False, include_web=False,
                           spex=[], version="10.2.6")
    document, _, __, ___ = swagger.build(args)
    # Exercise the same YAML publication codec used by main().
    published = swagger.yaml.safe_load(swagger.yaml.dump(document, Dumper=swagger.Dumper, sort_keys=False))
    description = next(tag["description"] for tag in published["tags"] if tag["name"] == "paxeer-x")
    assert description == "Paxeer X explorer surfaces: node-backed capabilities, price feed, native activity and committed chain receipts"
    for retired in ("custody", "anchor", "settlement status", "sequencer signatures", "kernel receipts"):
        assert retired not in description
    assert "/api/v2/paxeer-x/capabilities" in published["paths"]
    smoke = (HERE / "tests/compose-smoke-test.sh").read_text()
    assert '''expected_capabilities='{"addr": false, "bridge": false, "exchange": false, "launchpad": false}' '''.rstrip() in smoke
    assert "if actual != expected:" in smoke
    assert 'expected["checked_at"] = checked_at' in smoke
