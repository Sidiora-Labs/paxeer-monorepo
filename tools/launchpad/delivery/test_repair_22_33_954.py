import asyncio
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import sqlite3
import threading
import time

import pytest

from measure_phase4 import (
    Capture,
    canonical,
    create_identity_database,
    read_clock_calibration,
    read_receipts,
    read_source_commits,
    reconcile_sources,
    record_workload_receipt,
    sha,
)
from phase2_http import HTTP


NS = 1000000000
MS = 1000000
FIXTURE = Path(__file__).with_name('fixtures') / 'repair_22_33_954.json'


class RecordedIngestStore:
    def __init__(self, rows):
        self.db = sqlite3.connect(':memory:')
        self.db.execute('CREATE TABLE commits(event_id TEXT PRIMARY KEY,wire BLOB NOT NULL)')
        self.db.executemany('INSERT INTO commits VALUES(?,?)', [(row['event_id'], canonical(row)) for row in rows])

    async def committed(self, *, after=None):
        return [json.loads(row[0]) for row in self.db.execute('SELECT wire FROM commits WHERE event_id>? ORDER BY event_id LIMIT 2', (after or '',))]

    def close(self):
        self.db.close()


@contextmanager
def receipt_server(exchanges):
    requests = []
    pending = set(exchanges)

    class ReceiptHandler(BaseHTTPRequestHandler):
        def do_POST(self):
            assert self.path == '/'
            assert self.headers['Content-Type'] == 'application/json'
            assert self.headers['Accept'] == 'application/json'
            assert self.headers['Origin'] == 'https://delivery.example'
            request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            assert set(request) == {'jsonrpc', 'id', 'method', 'params'}
            assert request['jsonrpc'] == '2.0'
            assert request['id'] == 1
            assert request['method'] == 'eth_getTransactionReceipt'
            assert len(request['params']) == 1
            transaction_hash = request['params'][0]
            requests.append(transaction_hash)
            result = exchanges[transaction_hash]['result']
            if transaction_hash in pending:
                pending.remove(transaction_hash)
                result = None
            raw = canonical(dict(jsonrpc='2.0', id=request['id'], result=result))
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *_):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), ReceiptHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield HTTP(f'http://127.0.0.1:{server.server_port}', 'https://delivery.example', owned_local=True), requests
    finally:
        server.shutdown()
        thread.join()
        server.server_close()


def pin(path):
    return dict(path=str(path.resolve()), sha256=sha(path.read_bytes()))


async def produce_journals(directory, *, latency_ns=50 * MS, mutate=None):
    fixture = json.loads(FIXTURE.read_bytes())
    # Replay the committed peer transcript in the current measured clock window.
    delta = time.time_ns() + 50 * MS - fixture['origin_utc_ns']
    clock = fixture['clock']
    for key in ('measured_utc_ns', 'valid_from_utc_ns', 'valid_to_utc_ns'):
        clock[key] += delta
    clock_path = directory / 'clock.json'
    clock_path.write_bytes(canonical(clock))
    calibration = pin(clock_path)
    source_rows, ingest_rows = [], []
    exchanges = {}
    for event in fixture['events']:
        source = event['source_commit']
        source['source_commit_utc_ns'] += delta
        event['scheduled_utc_ns'] += delta
        event['ingest']['durable_commit_utc_ns'] = str(source['source_commit_utc_ns'] - clock['source_offset_ns'] + latency_ns + clock['postgres_offset_ns'])
        source_rows.append(source)
        ingest_rows.append(event['ingest'])
        exchanges[source['transaction_hash']] = event['rpc_response']
    if mutate:
        mutate(fixture)
    source_path = directory / 'source-commits.jsonl'
    source_path.write_bytes(b''.join(canonical(row) + b'\n' for row in source_rows))
    receipt_rows = []
    with receipt_server(exchanges) as (http, requests):
        for event in sorted(fixture['events'], key=lambda event: event['scheduled_utc_ns']):
            # Receipt polling occurs well after the independently recorded source commit.
            poll_at = event['source_commit']['source_commit_utc_ns'] - clock['source_offset_ns'] + 100 * MS
            await asyncio.sleep(max(0, (poll_at - time.time_ns()) / NS))
            receipt_path = directory / (event['profile'] + '-receipts.jsonl')
            with receipt_path.open('a') as output:
                receipt_rows.append(await record_workload_receipt(http, event['source_commit']['transaction_hash'], event['scheduled_utc_ns'], event['source_commit'], calibration, output))
        assert len(requests) == 2 * len(fixture['events'])
        assert set(requests) == set(exchanges)
    return fixture, calibration, source_path, receipt_rows, ingest_rows


def reconcile_transcript(directory, fixture, calibration, source_path, ingest_rows):
    db = create_identity_database(directory / 'identities.sqlite3')
    capture = Capture(directory, dict(candidate_sha256='a' * 64, capacity_ledger_sha256='b' * 64))
    observer = RecordedIngestStore(ingest_rows)
    try:
        authority = read_source_commits(source_path, calibration, db)
        capture.record('source_commit_authority', authority)
        assert authority['source_commits'] == 4
        assert authority['source_commit_journal_sha256'] == sha(source_path.read_bytes())
        profiles = {}
        for profile in ('sustained', 'burst'):
            summary = read_receipts(directory / (profile + '-receipts.jsonl'), db, profile, 1, 2, 10 * NS)
            assert summary['receipts'] == 2
            profiles[profile] = dict(receipt=summary)
        ledger = dict(sustained=dict(rate=1, seconds=2), burst=dict(rate=1, seconds=2), budgets=dict(ingest_p99_ns=500 * MS))
        result = asyncio.run(reconcile_sources(observer, ledger, db, capture, profiles))
        latencies = db.execute('SELECT latency_ns FROM sources ORDER BY source_id').fetchall()
        commits = db.execute('SELECT committed_ns FROM sources ORDER BY source_id').fetchall()
        assert db.execute('SELECT count(*) FROM source_receipts').fetchone()[0] == 4
        return result, latencies, commits
    finally:
        observer.close()
        capture.close()
        db.close()


@pytest.mark.parametrize('latency_ns,before_poll', [(50 * MS, True), (400 * MS, False)])
def test_delivery_observation_clock_ingestion_before_and_after_receipt_polling(tmp_path, latency_ns, before_poll):
    fixture, calibration, source_path, receipts, ingest_rows = asyncio.run(produce_journals(tmp_path, latency_ns=latency_ns))
    for receipt in receipts:
        ingest = next(row for row in ingest_rows if row['transaction_hash'] == receipt['transaction_hash'])
        durable_utc = int(ingest['durable_commit_utc_ns']) - fixture['clock']['postgres_offset_ns']
        assert (durable_utc < receipt['observed_utc_ns']) is before_poll
    result, latencies, commits = reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)
    assert latencies == [(latency_ns,)] * 4
    assert commits == [(int(row['durable_commit_utc_ns']) - fixture['clock']['postgres_offset_ns'],) for row in ingest_rows]
    assert result['ingest_p99_ns'] == latency_ns + 2 * MS
    assert result['committed_sources'] == {'sustained': 2, 'burst': 2}
    assert result['source_timestamp_authority'] == 'chain_transaction_commit'
    assert result['latency_statistic'] == 'upper_bound_including_measured_clock_uncertainty'
    observations = [json.loads(line) for line in (tmp_path / 'observations.jsonl').read_bytes().splitlines()]
    commits = [row['value'] for row in observations if row['kind'] == 'canonical_commit']
    assert len(commits) == 4
    assert all(row['clock_uncertainty_ns'] == 2 * MS and row['latency_ns'] == latency_ns for row in commits)


@pytest.mark.parametrize('latency_ns,expected', [(498 * MS, 500 * MS), (-MS, MS)])
def test_delivery_observation_clock_exact_budget_and_signed_latency(tmp_path, latency_ns, expected):
    fixture, calibration, source_path, _, ingest_rows = asyncio.run(produce_journals(tmp_path, latency_ns=latency_ns))
    result, latencies, _ = reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)
    assert latencies == [(latency_ns,)] * 4
    assert result['ingest_p99_ns'] == expected


@pytest.mark.parametrize('latency_ns,message', [(498 * MS + 1, 'exceeds frozen target'), (600 * MS, 'exceeds frozen target'), (-2 * MS - 1, 'precedes source commit')])
def test_delivery_observation_clock_rejects_budget_and_causality_failures(tmp_path, latency_ns, message):
    fixture, calibration, source_path, _, ingest_rows = asyncio.run(produce_journals(tmp_path, latency_ns=latency_ns))
    with pytest.raises(ValueError, match=message):
        reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)


@pytest.mark.parametrize('field,value', [('blockHash', '0x' + 'f' * 64), ('transactionHash', '0x' + 'e' * 64), ('blockNumber', '0xffff'), ('status', '0x0')])
def test_delivery_observation_clock_producer_rejects_unbound_receipts(tmp_path, field, value):
    def mutate(fixture):
        fixture['events'][0]['rpc_response']['result'][field] = value
    with pytest.raises(ValueError, match='independent source commit identity'):
        asyncio.run(produce_journals(tmp_path, mutate=mutate))
    assert not (tmp_path / 'sustained-receipts.jsonl').read_bytes()


@pytest.mark.parametrize('field,value,message', [('timestamp_authority', 'receipt_observation', 'clock authority'), ('clock_id', 'different_clock', 'clock authority')])
def test_delivery_observation_clock_rejects_observation_as_source_authority(tmp_path, field, value, message):
    def mutate(fixture):
        fixture['events'][0]['source_commit'][field] = value
    with pytest.raises(ValueError, match=message):
        asyncio.run(produce_journals(tmp_path, mutate=mutate))


@pytest.mark.parametrize('field,value', [('source_commit_sha256', 'e' * 64), ('clock_calibration_sha256', 'f' * 64), ('height', '99999'), ('block_hash', '0x' + 'f' * 64)])
def test_delivery_observation_clock_consumer_rejects_changed_binding(tmp_path, field, value):
    fixture, calibration, source_path, _, ingest_rows = asyncio.run(produce_journals(tmp_path))
    path = tmp_path / 'sustained-receipts.jsonl'
    receipts = [json.loads(line) for line in path.read_bytes().splitlines()]
    receipts[0][field] = value
    path.write_bytes(b''.join(canonical(row) + b'\n' for row in receipts))
    with pytest.raises(ValueError, match='matching independent source commit'):
        reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)


def test_delivery_observation_clock_calibration_is_pinned_bounded_and_current(tmp_path):
    fixture = json.loads(FIXTURE.read_bytes())
    path = tmp_path / 'clock.json'
    path.write_bytes(canonical(fixture['clock']))
    calibration = pin(path)
    path.write_bytes(canonical(dict(fixture['clock'], source_offset_ns=0)))
    with pytest.raises(ValueError, match='source binding changed'):
        read_clock_calibration(calibration)
    for change in ({'source_uncertainty_ns': 5000001}, {'postgres_uncertainty_ns': -1}, {'source_offset_ns': True}, {'valid_to_utc_ns': fixture['clock']['valid_from_utc_ns']}):
        path.write_bytes(canonical(dict(fixture['clock'], **change)))
        with pytest.raises(ValueError):
            read_clock_calibration(pin(path))
    path.write_bytes(canonical(fixture['clock']))
    calibration = pin(path)
    source_path = tmp_path / 'source-commits.jsonl'
    source = deepcopy(fixture['events'][0]['source_commit'])
    source['source_commit_utc_ns'] = fixture['clock']['valid_to_utc_ns'] + NS
    source_path.write_bytes(canonical(source) + b'\n')
    with sqlite3.connect(':memory:') as db:
        with pytest.raises(ValueError, match='outside measured clock validity'):
            read_source_commits(source_path, calibration, db)


def test_delivery_observation_clock_rejects_expired_database_timestamp(tmp_path):
    fixture, calibration, source_path, _, ingest_rows = asyncio.run(produce_journals(tmp_path))
    ingest_rows[0]['durable_commit_utc_ns'] = str(fixture['clock']['valid_to_utc_ns'] + NS)
    with pytest.raises(ValueError, match='outside measured clock validity'):
        reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)


@pytest.mark.parametrize('field,value,message', [('clock_id', 'unmeasured_clock', 'calibrated database clock identity'), ('timestamp_authority', 'receipt_observation', 'durable timestamp inconsistent')])
def test_delivery_observation_clock_rejects_unbound_database_clock(tmp_path, field, value, message):
    fixture, calibration, source_path, _, ingest_rows = asyncio.run(produce_journals(tmp_path))
    ingest_rows[0][field] = value
    with pytest.raises(ValueError, match=message):
        reconcile_transcript(tmp_path, fixture, calibration, source_path, ingest_rows)
