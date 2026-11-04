"""Exercise production reconciliation with a loopback SQL store and its real codec.

The peer executes the SELECTs from reconciliation.sql using SQLite's relational
engine. Only PostgreSQL session controls, current_epoch's table-function syntax,
and the JSON constructors need dialect adapters; identity queries are unchanged.
No provider process, live database, credentials or external service is required.
"""
from __future__ import annotations

import argparse
import ast
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import re
import shlex
import sqlite3
import sys
import threading
from urllib.request import Request, urlopen

import pytest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
sys.path.insert(0, str(HERE))
from providers import (MAX_PROVIDER_RESPONSE, PostgreSQL, ProviderFailure,
                       RECONCILIATION_STREAMS, bounded_lines, reconciliation_document)

SQL = (HERE / 'reconciliation.sql').read_text()
PSQL_ARGS = ['-X', '--no-password', '-qAt', '-v', 'ON_ERROR_STOP=1']


def json_text(value):
    # PostgreSQL jsonb arrays use these separators and preserve UTF-8 strings.
    return json.dumps(value, ensure_ascii=False, separators=(', ', ': '))


class SQLStore:
    """A real, concurrent SQL peer, with private per-schema WAL stores."""
    def __init__(self, root, rows):
        self.paths = {name: root / (name + '.sqlite') for name in
                      ('indexer', 'pnl', 'candles', 'stats', 'recovery')}
        self.after_summary = None
        self.requests = []
        self.bytes_sent = 0
        self.first_row = None
        self.last_row = None
        with self.connect() as db:
            db.executescript('''
                CREATE TABLE indexer.indexer_raw_logs (
                  chain_id INTEGER, block_hash TEXT, transaction_hash TEXT,
                  log_index INTEGER, height INTEGER, transaction_index INTEGER);
                CREATE TABLE indexer.indexer_ingest_watermarks (
                  chain_id INTEGER, stream_id TEXT, raw_complete_height INTEGER,
                  typed_complete_height INTEGER);
                CREATE TABLE pnl.inventory_l16 (
                  partition TEXT, revision INTEGER, state TEXT, last_identity TEXT,
                  height INTEGER, tx_index INTEGER, log_index INTEGER);
                CREATE TABLE indexer.delivery_outbox (
                  event_id TEXT, payload_hash TEXT, state TEXT);
                CREATE TABLE recovery.current_epoch (epoch INTEGER, region TEXT);
                INSERT INTO recovery.current_epoch VALUES (7, 'region-primary');
                INSERT INTO indexer.indexer_ingest_watermarks
                  VALUES (125, 'raw-chain', 999999, 999999);
            ''')
            for schema in ('indexer', 'candles', 'stats', 'pnl'):
                db.execute(f'CREATE TABLE {schema}.delivery_inbox ('
                           'consumer_id TEXT, projection_generation INTEGER, '
                           'event_id TEXT, envelope_hash TEXT)')
                db.execute(f'INSERT INTO {schema}.delivery_inbox VALUES (?,?,?,?)',
                           (schema + '-consumer', 3, 'event-1', 'b' * 64))
            state = json_text(dict(knownQuantityRaw='9007199254740993',
                                   unknownQuantityRaw='12',
                                   knownRemainingCostUsdlRaw='12345',
                                   knownRealizedSubtotalUsdlRaw='-54321',
                                   retained='nested "state"\nwith Unicode Ω'))
            db.execute('BEGIN')
            db.execute('INSERT INTO pnl.inventory_l16 VALUES (?,?,?,?,?,?,?)',
                       ('wallet:125', 9, state, 'last-identity', 999999, 1, 0))
            db.execute('INSERT INTO indexer.delivery_outbox VALUES (?,?,?)',
                       ('event-1', 'c' * 64, 'pending'))
            for i in range(rows):
                row = (125, '0x' + format(i, '064x'), '0x' + format(i + 1, '064x'), 0, i, 0)
                self.first_row = self.first_row or row
                self.last_row = row
                db.execute('INSERT INTO indexer.indexer_raw_logs VALUES (?,?,?,?,?,?)', row)
            # A different chain is never included in the chain125 commitment.
            db.execute('INSERT INTO indexer.indexer_raw_logs VALUES (1,?,?,0,0,0)',
                       ('foreign-block', 'foreign-transaction'))
            db.execute('COMMIT')

    @contextmanager
    def connect(self):
        db = sqlite3.connect(':memory:', isolation_level=None)
        db.create_function('json_build_object', -1,
                           lambda *values: json_text(dict(zip(values[::2], values[1::2]))))
        db.create_function('jsonb_build_array', -1, lambda *values: json_text(list(values)))
        db.create_function('pg_current_snapshot', 0, lambda: 'fixture:repeatable-read')
        db.create_collation('C', lambda a, b: (a.encode() > b.encode()) - (a.encode() < b.encode()))
        for name, path in self.paths.items():
            db.execute(f'ATTACH DATABASE ? AS {name}', (str(path),))
            db.execute(f'PRAGMA {name}.journal_mode=WAL')
        try:
            yield db
        finally:
            db.close()

    def response(self, request):
        self.requests.append(request)
        assert request['tool'] == 'psql'
        assert request['args'] == PSQL_ARGS
        assert request['timeout'] == 30
        assert request['env']['PGSERVICE'] == 'recovery-peer'
        sql = request['sql']
        auth, body = sql.split('END $identity$;\n', 1)
        assert "session_user <> 'coordinator'" in auth
        assert 'current_user <> session_user' in auth
        assert body == SQL
        assert 'BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;' in body
        assert body.rstrip().endswith('COMMIT;')
        selects = body[body.index('SELECT json_build_object('):body.rindex('COMMIT;')]
        # SQLite exposes the epoch as a one-row table rather than a table function.
        selects = selects.replace('recovery.current_epoch()', 'recovery.current_epoch')
        with self.connect() as db:
            db.execute('BEGIN')
            for i, statement in enumerate(selects.split(';')):
                if not statement.strip():
                    continue
                cursor = db.execute(statement)
                for (value,) in cursor:
                    raw = value.encode() + b'\n'
                    self.bytes_sent += len(raw)
                    yield raw
                if i == 0 and self.after_summary:
                    callback, self.after_summary = self.after_summary, None
                    callback()
            db.execute('COMMIT')


class SQLHandler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.0'

    def do_POST(self):
        request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        self.send_response(200)
        self.send_header('Content-Type', 'application/x-ndjson; charset=utf-8')
        self.end_headers()
        try:
            for row in self.server.store.response(request):
                self.wfile.write(row)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def log_message(self, *args):
        pass


class LoopbackPSQL:
    """Transport the psql invocation contract to an in-process SQL server."""
    def __init__(self, origin):
        self.origin = origin

    def lines(self, tool, args, *, env, data, timeout):
        raw = json.dumps(dict(tool=tool, args=args, env=env, sql=data.decode(), timeout=timeout)).encode()
        request = Request(self.origin, data=raw, headers={'Content-Type': 'application/json'})
        with urlopen(request, timeout=timeout) as response:
            assert response.headers['Content-Type'] == 'application/x-ndjson; charset=utf-8'
            yield from bounded_lines(iter(lambda: response.read(8191), b''))


@contextmanager
def database_peer(tmp_path, rows=8):
    store = SQLStore(tmp_path, rows)
    server = ThreadingHTTPServer(('127.0.0.1', 0), SQLHandler)
    server.store = store
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    service = tmp_path / 'pg_service.conf'
    service.write_text('[recovery-peer]\nuser=coordinator\n')
    service.chmod(0o600)
    tools = LoopbackPSQL('http://127.0.0.1:' + str(server.server_port))
    database = PostgreSQL(tools, str(service), 'recovery-peer', 'coordinator')
    try:
        yield database, store
    finally:
        server.shutdown()
        server.server_close()
        worker.join()


def expected_raw_digest(rows):
    digest = hashlib.sha256(b'recovery-reconciliation-v1\0raw_chain_identities\0')
    # Independent commitment over every retained source identity, including the end.
    for i in range(rows):
        value = [125, '0x' + format(i, '064x'), '0x' + format(i + 1, '064x'), 0]
        digest.update(json_text(value).encode() + b'\n')
    return digest.hexdigest()


def load_release():
    path = HERE.parent / 'phase4' / 'release.py'
    spec = importlib.util.spec_from_file_location('repair_22_33_719_release', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_recovery_reconciliation_response_bound_full_retained_dataset(tmp_path):
    rows = 18000
    with database_peer(tmp_path, rows) as (database, store):
        actual = database.document(SQL)
        assert store.bytes_sent > MAX_PROVIDER_RESPONSE
        assert actual['raw_chain_log_count'] == rows
        assert actual['known_quantity_raw'] == '9007199254740993'
        assert actual['realized_subtotal_usdl_raw'] == '-54321'
        assert actual['indexer_pending_outbox_rows'] == 1
        assert actual['identity_digests']['raw_chain_identities'] == {
            'count': rows, 'sha256': expected_raw_digest(rows)}
        assert set(actual['identity_digests']) == set(RECONCILIATION_STREAMS)
        assert all(actual['identity_digests'][name]['count'] == 1 for name in
                   RECONCILIATION_STREAMS if name != 'raw_chain_identities')
        assert len(json.dumps(actual).encode()) < MAX_PROVIDER_RESPONSE
        assert actual == database.document(SQL)
        # Exercise the existing release artifact consumer with the new document.
        release = load_release()
        journal = release.Journal(tmp_path / 'release-evidence')
        artifact = journal.write('checkpoint-before.json', actual)
        assert json.loads(Path(artifact['path']).read_bytes()) == actual
        assert artifact['sha256'] == hashlib.sha256(Path(artifact['path']).read_bytes()).hexdigest()
        # A changed final identity must not disappear beyond a transport cutoff.
        with store.connect() as writer:
            writer.execute('UPDATE indexer.indexer_raw_logs SET transaction_hash=? WHERE height=? AND chain_id=125',
                           ('0x' + 'f' * 64, rows - 1))
        changed = database.document(SQL)
        assert changed['raw_chain_log_count'] == rows
        assert changed['identity_digests']['raw_chain_identities'] != actual['identity_digests']['raw_chain_identities']


def test_recovery_reconciliation_snapshot_consistency(tmp_path):
    with database_peer(tmp_path) as (database, store):
        def commit_after_summary():
            with store.connect() as writer:
                writer.execute('INSERT INTO indexer.indexer_raw_logs VALUES (125,?,?,0,99,0)',
                               ('new-block', 'new-transaction'))
        store.after_summary = commit_after_summary
        before = database.document(SQL)
        assert before['raw_chain_log_count'] == 8
        assert before['identity_digests']['raw_chain_identities']['count'] == 8
        after = database.document(SQL)
        assert after['raw_chain_log_count'] == 9
        assert after['identity_digests']['raw_chain_identities']['count'] == 9
        assert before['identity_digests']['raw_chain_identities'] != after['identity_digests']['raw_chain_identities']


def test_recovery_reconciliation_empty_streams_remain_exact(tmp_path):
    with database_peer(tmp_path) as (database, store):
        with store.connect() as writer:
            for table in ('indexer.indexer_raw_logs', 'indexer.indexer_ingest_watermarks',
                          'pnl.inventory_l16', 'indexer.delivery_outbox',
                          'indexer.delivery_inbox', 'candles.delivery_inbox',
                          'stats.delivery_inbox', 'pnl.delivery_inbox'):
                writer.execute('DELETE FROM ' + table)
        actual = database.document(SQL)
        assert actual['raw_chain_log_count'] == actual['inventory_count'] == 0
        for name, digest in actual['identity_digests'].items():
            assert digest == {'count': 0, 'sha256': hashlib.sha256(
                ('recovery-reconciliation-v1\0' + name + '\0').encode()).hexdigest()}
        for name in ('known_quantity_raw', 'unknown_quantity_raw',
                     'remaining_cost_usdl_raw', 'realized_subtotal_usdl_raw'):
            assert actual[name] == '0'


@pytest.mark.parametrize('stream', RECONCILIATION_STREAMS)
def test_recovery_reconciliation_every_authoritative_stream_is_committed(tmp_path, stream):
    with database_peer(tmp_path) as (database, store):
        before = database.document(SQL)
        statements = {
            'raw_chain_identities': "UPDATE indexer.indexer_raw_logs SET transaction_hash='changed' WHERE chain_id=125 AND height=4",
            'ingest_watermarks': 'UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=999998',
            'inventory_state': "UPDATE pnl.inventory_l16 SET last_identity='changed'",
            'indexer_outbox_identities': "UPDATE indexer.delivery_outbox SET payload_hash='changed'",
            'indexer_inbox_identities': "UPDATE indexer.delivery_inbox SET envelope_hash='changed'",
            'candle_inbox_identities': "UPDATE candles.delivery_inbox SET envelope_hash='changed'",
            'stats_inbox_identities': "UPDATE stats.delivery_inbox SET envelope_hash='changed'",
            'pnl_inbox_identities': "UPDATE pnl.delivery_inbox SET envelope_hash='changed'",
        }
        with store.connect() as writer:
            writer.execute(statements[stream])
        after = database.document(SQL)
        for name in RECONCILIATION_STREAMS:
            assert (before['identity_digests'][name] == after['identity_digests'][name]) == (name != stream)


def recorded_rows(tmp_path):
    with database_peer(tmp_path) as (database, store):
        request = dict(tool='psql', args=PSQL_ARGS, timeout=30, env=database.environment,
                       sql=database.authentication_sql() + SQL)
        return list(store.response(request))


def test_recovery_reconciliation_rejects_truncation_duplicates_and_foreign_streams(tmp_path):
    rows = recorded_rows(tmp_path)
    with pytest.raises(ProviderFailure, match='missing identity rows'):
        reconciliation_document(bounded_lines(rows[:-1]))
    with pytest.raises(ProviderFailure, match='excess identity rows'):
        reconciliation_document(bounded_lines(rows + rows[-1:]))
    foreign = json.loads(rows[1])
    foreign['stream'] = 'foreign'
    with pytest.raises(ProviderFailure, match='unknown identity stream'):
        reconciliation_document(bounded_lines([rows[0], json.dumps(foreign).encode() + b'\n']))
    with pytest.raises(ProviderFailure, match='stream order changed'):
        reconciliation_document(bounded_lines([*rows[:-1], rows[1]]))
    with pytest.raises(ProviderFailure, match='incomplete row'):
        reconciliation_document(bounded_lines([*rows[:-1], rows[-1].rstrip(b'\n')]))
    with pytest.raises(ProviderFailure, match='summary required'):
        reconciliation_document(bounded_lines(rows[1:]))


def test_recovery_reconciliation_preserves_transport_and_database_bounds():
    with pytest.raises(ProviderFailure, match='oversized row'):
        list(bounded_lines([b'x' * 65536] * (MAX_PROVIDER_RESPONSE // 65536) + [b'x\n']))
    assert list(bounded_lines([b'abc', b'd\n\n', 'Ω'.encode()[:1], 'Ω'.encode()[1:] + b'\n'])) == [b'abcd', b'', 'Ω'.encode()]
    assert "statement_timeout='20s'" in SQL
    assert "lock_timeout='2s'" in SQL
    assert "work_mem='8MB'" in SQL
    assert "current_setting('temp_file_limit')" in SQL
    assert 'NOT BETWEEN 0 AND 268435456' in SQL
    assert 'json_agg' not in SQL and 'LIMIT' not in SQL
    for field in ('knownQuantityRaw', 'unknownQuantityRaw', 'knownRemainingCostUsdlRaw', 'knownRealizedSubtotalUsdlRaw'):
        assert f"state->>'{field}' IS NULL" in SQL
    assert "'^(0|-?[1-9][0-9]{0,77})$'" in SQL


def parser_from_declarations(path):
    tree = ast.parse(path.read_text())
    main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'main')
    declarations = []
    for node in main.body:
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'parser' for t in node.targets):
            declarations.append(node)
        elif (isinstance(node, ast.Expr) and isinstance(node.value, ast.Call) and
              isinstance(node.value.func, ast.Attribute) and
              isinstance(node.value.func.value, ast.Name) and node.value.func.value.id == 'parser'):
            declarations.append(node)
    namespace = {'argparse': argparse}
    exec(compile(ast.Module(body=declarations, type_ignores=[]), str(path), 'exec'), namespace)
    return namespace['parser']


def test_recovery_v3_command_paths_match_real_cli_declarations():
    runbook = (HERE / 'runbook.md').read_text()
    commands = []
    for block in re.findall(r'```sh\n(.*?)\n```', runbook, re.S):
        commands.extend(shlex.split(line) for line in block.replace('\\\n', '').splitlines() if line.strip())
    assert len(commands) == 6
    operations = []
    for executable, relative, *argv in commands:
        assert executable == 'python3'
        path = ROOT / relative
        assert path.is_file() and path.parent == HERE
        assert relative.startswith('deploy/launchpad/production/recovery/phase2/')
        arguments = parser_from_declarations(path).parse_args(argv)
        if path.name == 'operator.py':
            operations.append(arguments.operation)
            assert arguments.configuration == '<private-file>'
            assert arguments.configuration_sha256 == '<trusted-digest>'
            assert arguments.proposal == '<proposal-file>'
            if arguments.operation in ('validate', 'activate'):
                assert arguments.observations == '<signed-v2-file>'
                assert arguments.scenarios == '<signed-v3-file>'
        elif path.name == 'providers.py':
            assert arguments.operation == 'rehearse'
            assert arguments.requirement == 'D14_031'
            assert arguments.source_sha256 == '<providers.py-digest>'
            assert arguments.identity_sha256 == '<admitted-identity-digest>'
        else:
            assert path.name == 'check.py' and arguments.phase == 3
    assert operations == ['propose', 'validate', 'activate', 'status']
    for relative in ('tools/launchpad/production', 'tools/launchpad/production/load',
                     'deploy/launchpad/production/recovery/phase2/broker'):
        assert (ROOT / relative).is_dir()
    assert 'tools/launchpad/production' in runbook
    assert 'deploy/production' not in runbook and 'tools/production' not in runbook
    assert 'operator.py ...' not in runbook
    for safety in ('All thirteen focused checks and all ten native receipts must pass',
                   'Keep the old primary read-only', 'No production readiness is claimed',
                   'Never undo old-primary broker revocation'):
        assert safety in runbook
