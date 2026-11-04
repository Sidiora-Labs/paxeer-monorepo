"""Bounded Python qualification of native capture consumption and Go test wiring.

Native Go/PostgreSQL/S3 qualification is registered with the production component
runner; this Python gate neither invokes a toolchain nor claims it executed Go.
"""
from __future__ import annotations

import copy
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
import sqlite3
import threading
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import pytest

import release_harness as harness


POLICY = Path(__file__).resolve().parent
NATIVE_REPAIRS = (
    ('column-grants-not-checked', 'credentials_test.go', 'TestRepairColumnGrantsAndOwnerViews'),
    ('audit-export-not-scheduled', 'export_test.go', 'TestRepairAuditSchedulerStartupCatchUpAndAlerts'),
    ('retained-export-positive-proof', 'export_test.go', 'TestRepairRetainedExportRecoveryAndCommittedCoverage'),
    ('active-body-deadline', 'parsing_test.go', 'TestRepairActiveBodyDeadlinePreservesReadStreams'),
    ('mutation-case-alias', 'parsing_test.go', 'TestRepairNestedMutationCaseAliases'),
)


class ObservationPeer(BaseHTTPRequestHandler):
    """An in-process peer serving actual HTTP outcomes to the capture client."""

    def do_POST(self):
        body = self.rfile.read(int(self.headers['Content-Length']))
        status = int(self.path.rsplit('/', 1)[1])
        wire = harness.canonical({'request_id': self.headers['X-Request-ID'],
                                  'method': self.command, 'path': self.path,
                                  'body_bytes': len(body), 'http_status': status})
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(wire)))
        self.end_headers()
        self.wfile.write(wire)

    def log_message(self, *args):
        pass


@pytest.fixture
def endpoint():
    server = ThreadingHTTPServer(('127.0.0.1', 0), ObservationPeer)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        yield 'http://127.0.0.1:' + str(server.server_port)
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=2)
        assert not worker.is_alive()


class CaptureCase:
    def __init__(self, directory, endpoint, number):
        self.directory = directory
        self.endpoint = endpoint
        self.number = number
        self.records = []
        self.started = time.time_ns()
        sources = {name: harness.sha_file(POLICY / name)
                   for name in ('credentials.go', 'audit.go', 'export.go', 'parsing.go', 'policy.go', 'release_harness.py')}
        self.configuration = {
            'bindings': harness.Bindings(
                candidate=hashlib.sha256(harness.canonical(sources)).hexdigest(),
                interface=hashlib.sha256(b'native-scenario-v1').hexdigest(),
                topology=hashlib.sha256(endpoint.encode()).hexdigest(),
                generator=harness.sha_file(Path(__file__).resolve())),
            'authority': {'run_id': 'repair-22.33.1021', 'replica_ids': ['replica-a', 'replica-b']},
            'join_key': b'in-process-capture-join-key-for-repair-22.33.1021',
        }
        self.row = {'scenario_id': number, 'started_utc_ns': self.started,
                    'ended_utc_ns': self.started + 1, 'raw_artifacts': [], 'facts': {}}

    def emit(self, request_id, producer, kind, payload):
        record = harness.scenario_observation(self.configuration, self.number, request_id,
                                              producer, kind, payload, time.time_ns())
        self.records.append(record)
        return record

    def exchange(self, status, body=b'{}'):
        request_id = 'scenario-' + str(self.number) + '-request-' + str(len(self.records))
        request = Request(self.endpoint + '/observations/' + str(status), data=body,
                          headers={'X-Request-ID': request_id, 'Content-Type': 'application/json'},
                          method='POST')
        try:
            response = urlopen(request, timeout=2)
        except HTTPError as error:
            response = error
        with response:
            observed = json.loads(response.read(), object_pairs_hook=harness.unique)
            assert observed['http_status'] == response.status
            assert observed['request_id'] == request_id
            assert observed['body_bytes'] == len(body)
        self.emit(request_id, 'security.http', 'exchange', observed)
        return request_id

    def publish(self):
        path = self.directory / ('native-' + str(self.number) + '.ndjson')
        path.write_bytes(b''.join(harness.canonical(record) + b'\n' for record in self.records))
        self.row['ended_utc_ns'] = time.time_ns() + 1
        self.row['raw_artifacts'] = [{'kind': 'native_scenario', 'path': str(path), 'sha256': harness.sha_file(path)}]
        return path

    def derive(self):
        self.publish()
        return harness.native_scenario_facts(self.configuration, self.row)

    def document(self):
        self.publish()
        path = self.directory / 'runtime-scenarios.json'
        harness.write_json(path, {'version': 1, 'run_id': self.configuration['authority']['run_id'],
                                  'bindings': self.configuration['bindings'].__dict__, 'scenarios': [self.row]})
        self.configuration.update(runtime_scenarios_path=str(path), runtime_scenarios_sha256=harness.sha_file(path))

    def resign(self, record):
        unsigned = {key: value for key, value in record.items() if key != 'mac'}
        record['mac'] = hmac.new(self.configuration['join_key'], harness.canonical(unsigned), hashlib.sha256).hexdigest()


@pytest.mark.parametrize('finding,path,test', NATIVE_REPAIRS, ids=[row[0] for row in NATIVE_REPAIRS])
def test_native_repair_qualification_wiring(finding, path, test):
    # This is an interface check between the production component runner and
    # actual Go declarations, not a claim that Python executes Go or PostgreSQL.
    declared = set(re.findall(r'^func (Test\w+)\(t \*testing\.T\)', (POLICY / path).read_text(), re.MULTILINE))
    assert test in declared, finding
    assert test in harness.component_tests('policy'), finding
    assert test not in harness.component_tests('secrets'), finding


@pytest.mark.parametrize('number,status', [(1, 403), (2, 403), (3, 401), (7, 413), (13, 503)])
def test_raw_http_and_durable_records_override_passing_facts(tmp_path, endpoint, number, status):
    case = CaptureCase(tmp_path, endpoint, number)
    request = case.exchange(status, b'x' * 65537 if number == 7 else b'{}')
    database = sqlite3.connect(tmp_path / 'journal.sqlite')
    database.execute('CREATE TABLE journal(id INTEGER PRIMARY KEY, record TEXT NOT NULL)')
    database.commit()

    def snapshot():
        records = database.execute('SELECT id,record FROM journal ORDER BY id').fetchall()
        return {'jobs': records, 'queue': records, 'audit': records}

    before = snapshot()
    producer = 'security.queue' if number == 3 else 'security.database'
    case.emit(request, producer, 'state_before', {'records': before})
    case.emit(request, producer, 'state_after', {'records': snapshot()})
    facts = case.derive()
    harness.scenario_facts(number, facts, [])
    case.row['facts'] = facts
    database.execute("INSERT INTO journal(record) VALUES ('committed mutation')")
    database.commit()
    database.close()
    database = sqlite3.connect(tmp_path / 'journal.sqlite')
    recovered = snapshot()
    database.close()
    assert recovered != before
    # The raw database outcome changes while the authored passing summary does not.
    raw_after = case.records[-1]
    raw_after['payload'] = {'records': recovered}
    case.resign(raw_after)
    case.document()
    with pytest.raises(ValueError, match='contradict'):
        harness.runtime_scenarios(case.configuration)
    derived = harness.native_scenario_facts(case.configuration, case.row)
    with pytest.raises(ValueError, match='durable state'):
        harness.scenario_facts(number, derived, [])


@pytest.mark.parametrize('number,status', [(1, 403), (2, 403), (3, 401), (7, 413), (13, 503)])
def test_raw_success_cannot_be_hidden_by_denial_facts(tmp_path, endpoint, number, status):
    case = CaptureCase(tmp_path, endpoint, number)
    request = case.exchange(status, b'x' * 65537 if number == 7 else b'{}')
    producer = 'security.queue' if number == 3 else 'security.database'
    for kind in ('state_before', 'state_after'):
        case.emit(request, producer, kind, {'records': {'jobs': [], 'queue': [], 'audit': []}})
    case.row['facts'] = case.derive()
    case.records[0]['payload']['http_status'] = 204
    case.resign(case.records[0])
    case.document()
    with pytest.raises(ValueError, match='contradict'):
        harness.runtime_scenarios(case.configuration)
    with pytest.raises(ValueError, match='failure differs'):
        harness.scenario_facts(number, harness.native_scenario_facts(case.configuration, case.row), [])


def test_same_principal_budget_uses_each_native_redis_and_http_outcome(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 4)
    for index in range(61):
        admitted = index < 10
        request = case.exchange(200 if admitted else 429)
        case.emit(request, 'security.redis', 'admission', {
            'redis_ms': 1000, 'admitted': admitted, 'replica_id': 'replica-a' if index % 2 else 'replica-b',
            'principal_reference': hashlib.sha256(b'principal').hexdigest(), 'policy_class': 'account-write', 'cost': 1})
    facts = case.derive()
    harness.scenario_facts(4, facts, [])
    assert sum(event['admitted'] for event in facts['redis_admissions']) == 10
    # A signed raw native allow contradicting its actual HTTP rejection fails.
    case.records[-1]['payload']['admitted'] = True
    case.resign(case.records[-1])
    case.publish()
    with pytest.raises(ValueError, match='contradicts HTTP'):
        harness.native_scenario_facts(case.configuration, case.row)


def test_restart_identity_and_recovered_counter_enumeration_are_required(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 5)
    request = case.exchange(503)
    case.emit(request, 'security.redis', 'before_restart', {'instance_id': 'process-before', 'storage_id': 'durable-a'})
    case.emit(request, 'security.redis', 'after_restart', {'instance_id': 'process-after', 'storage_id': 'durable-a', 'ping': 'PONG', 'counter_ttls_ms': [120000, 999]})
    case.emit(request, 'security.policy', 'cache', {'entries': []})
    harness.scenario_facts(5, case.derive(), [])
    case.records[2]['payload']['counter_ttls_ms'].append(-1)
    case.resign(case.records[2])
    with pytest.raises(ValueError, match='counter observations'):
        harness.scenario_facts(5, case.derive(), [])


@pytest.mark.parametrize('number', [6, 15])
def test_loss_scenarios_join_native_arrivals_by_request_and_run(tmp_path, endpoint, number):
    case = CaptureCase(tmp_path, endpoint, number)
    timings = []
    classes = [('account-write', False, 503)]
    if number == 15:
        classes += [('cached-public-read', True, 200), ('replay-attack', False, 429)]
    for route, allowed, status in classes:
        request = case.exchange(status)
        event = case.emit(request, 'security.policy', 'admission', {'allowed': allowed, 'backend_unavailable': True, 'route_class': route})
        timings.append({'run_id': case.configuration['authority']['run_id'], 'replica_id': 'replica-a',
                        'candidate_sha256': case.configuration['bindings'].candidate, 'policy_version': 'v3',
                        'request_reference': event['request_reference'], 'stage': 'limiter',
                        'observed_utc_ns': event['observed_utc_ns'], 'duration_ns': 100,
                        'allowed': allowed, 'backend_unavailable': True})
    path = tmp_path / 'middleware.ndjson'
    path.write_bytes(b''.join(harness.canonical(row) + b'\n' for row in timings))
    case.configuration.update(timings_path=str(path), timings_sha256=harness.sha_file(path), policy_version='v3')
    assert case.derive() == {'native_arrivals_bound': True}
    timings[0]['allowed'] = True
    path.write_bytes(b''.join(harness.canonical(row) + b'\n' for row in timings))
    case.configuration['timings_sha256'] = harness.sha_file(path)
    with pytest.raises(ValueError, match='detached'):
        case.derive()


def test_egress_connections_contradict_passing_ssrf_facts(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 8)
    request = case.exchange(403)
    event = case.emit(request, 'security.egress', 'redirect', {
        'location': 'http://169.254.169.254/latest/meta-data', 'connected_addresses': [], 'error': 'policy: denied'})
    facts = case.derive()
    harness.scenario_facts(8, facts, [])
    case.row['facts'] = facts
    event['payload']['connected_addresses'].append('169.254.169.254')
    case.resign(event)
    case.document()
    with pytest.raises(ValueError, match='contradict'):
        harness.runtime_scenarios(case.configuration)
    with pytest.raises(ValueError, match='prohibited destination'):
        harness.scenario_facts(8, harness.native_scenario_facts(case.configuration, case.row), [])


def test_startup_fallback_cannot_hide_in_unrelated_raw_artifact(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 9)
    request = case.exchange(503)
    event = case.emit(request, 'security.secrets', 'startup', {
        'configured_provider': 'vault', 'loaded_provider': '', 'exit_code': 1, 'ready': False})
    harness.scenario_facts(9, case.derive(), [])
    event['payload']['loaded_provider'] = 'env'
    case.resign(event)
    with pytest.raises(ValueError, match='alternate provider'):
        case.derive()


def test_rotation_assertions_use_native_verification_results(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 10)
    request = case.exchange(403)
    now = time.time_ns()
    event = case.emit(request, 'security.rotation', 'rotation', {
        'activated_utc_ns': now - 1000000, 'retired_utc_ns': now - 100,
        'cache_refreshed_utc_ns': now - 999999, 'revoked_utc_ns': now - 10,
        'revocation_observed_utc_ns': now - 1,
        'retirement_checks': [{'audience': 'private', 'accepted': False, 'observed_utc_ns': now}],
        'unsigned_checks': [{'audience': 'private', 'accepted': False, 'observed_utc_ns': now}],
        'outgoing_version': 'v3', 'newest_active_version': 'v3', 'audiences': ['private']})
    harness.scenario_facts(10, case.derive(), [])
    event['payload']['retirement_checks'][0]['accepted'] = True
    case.resign(event)
    with pytest.raises(ValueError, match='rotation'):
        harness.scenario_facts(10, case.derive(), [])


def test_role_outcomes_require_catalog_and_actual_durable_records(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 11)
    request = case.exchange(403)
    records = {'stats': [{'id': 1}], 'identity': []}
    for kind in ('state_before', 'state_after'):
        case.emit(request, 'security.database', kind, {'records': records})
    event = case.emit(request, 'security.database', 'role', {'sqlstate': '42501', 'rolsuper': False, 'owns_schema': False})
    harness.scenario_facts(11, case.derive(), [])
    event['payload']['sqlstate'] = '00000'
    case.resign(event)
    with pytest.raises(ValueError, match='runtime data credential'):
        harness.scenario_facts(11, case.derive(), [])


def test_telemetry_requests_are_joined_and_scanned_for_actual_secret_bytes(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 12)
    request = case.exchange(403)
    import os
    names = ['REPAIR_22_33_1021_CANARY_' + str(index) for index in range(5)]
    previous = {name: os.environ.get(name) for name in names}
    try:
        for index, name in enumerate(names):
            os.environ[name] = 'secret-canary-' + str(index) + '-unique-long-value'
        case.emit(request, 'security.telemetry', 'canaries', {'canary_env': names})
        facts = case.derive()
        paths = []
        for kind in ('indexed_logs', 'indexed_traces'):
            path = tmp_path / kind
            path.write_bytes(harness.canonical({'request_reference': facts['request_references'][0]}) + b'\n')
            paths.append((kind, path))
        harness.scenario_facts(12, facts, paths)
        paths[0][1].write_bytes(paths[0][1].read_bytes() + os.environ[names[0]].encode())
        with pytest.raises(ValueError, match='secret found'):
            harness.scenario_facts(12, facts, paths)
    finally:
        for name, value in previous.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value


def test_registry_manifest_is_compared_with_native_unclassified_routes(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 14)
    request = case.exchange(503)
    event = case.emit(request, 'security.registry', 'validation', {
        'exit_code': 1, 'unclassified_routes': ['POST /unclassified'], 'manifest_routes': ['GET /public']})
    harness.scenario_facts(14, case.derive(), [])
    event['payload']['manifest_routes'].append('POST /unclassified')
    case.resign(event)
    with pytest.raises(ValueError, match='unclassified endpoint'):
        harness.scenario_facts(14, case.derive(), [])


@pytest.mark.parametrize('change', ['candidate', 'run', 'request', 'time', 'signature', 'duplicate', 'missing-durable'])
def test_foreign_or_unbound_native_records_fail_closed(tmp_path, endpoint, change):
    case = CaptureCase(tmp_path, endpoint, 1)
    request = case.exchange(403)
    for kind in ('state_before', 'state_after'):
        case.emit(request, 'security.database', kind, {'records': {}})
    case.derive()
    record = case.records[-1]
    if change == 'candidate':
        record['bindings'] = {**record['bindings'], 'candidate': hashlib.sha256(b'foreign').hexdigest()}
    elif change == 'run':
        record['run_id'] = 'foreign-run'
    elif change == 'request':
        record['request_reference'] = hashlib.sha256(b'foreign-request').hexdigest()
    elif change == 'time':
        record['observed_utc_ns'] = case.started - 1
    elif change == 'signature':
        record['payload']['records'] = {'changed': True}
    elif change == 'duplicate':
        case.records.append(copy.deepcopy(record))
    elif change == 'missing-durable':
        case.records.pop()
    if change != 'signature':
        case.resign(record)
    case.publish()
    with pytest.raises(ValueError):
        harness.native_scenario_facts(case.configuration, case.row)


def test_hash_valid_unrelated_artifact_does_not_qualify_authored_facts(tmp_path, endpoint):
    case = CaptureCase(tmp_path, endpoint, 1)
    path = tmp_path / 'unrelated.log'
    path.write_text('not a native request or durable database outcome\n')
    case.row['raw_artifacts'] = [{'kind': 'provider_log', 'path': str(path), 'sha256': harness.sha_file(path)}]
    case.row['ended_utc_ns'] = time.time_ns() + 1
    case.row['facts'] = {'before_sha256': hashlib.sha256(b'').hexdigest(),
                         'after_sha256': hashlib.sha256(b'').hexdigest(), 'http_status': 403}
    with pytest.raises(ValueError, match='matching sent HTTP'):
        harness.native_scenario_facts(case.configuration, case.row)



def test_complete_runtime_document_qualifies_only_derived_current_outcomes(tmp_path, endpoint):
    import os
    cases = []
    native_timings = []
    canaries = ['REPAIR_RUNTIME_CANARY_' + str(index) for index in range(5)]
    previous = {name: os.environ.get(name) for name in canaries}
    for index, name in enumerate(canaries):
        os.environ[name] = 'runtime-secret-' + str(index) + '-never-in-indexed-records'
    try:
        for number in range(1, 16):
            case = CaptureCase(tmp_path, endpoint, number)
            if number == 4:
                for index in range(61):
                    request = case.exchange(200 if index < 10 else 429)
                    case.emit(request, 'security.redis', 'admission', {
                        'redis_ms': 1000, 'admitted': index < 10, 'replica_id': 'replica-a' if index % 2 else 'replica-b',
                        'principal_reference': hashlib.sha256(b'principal').hexdigest(), 'policy_class': 'account-write', 'cost': 1})
            elif number in (6, 15):
                classes = [('account-write', False, 503)]
                if number == 15:
                    classes += [('cached-public-read', True, 200), ('replay-attack', False, 429)]
                for route, allowed, status in classes:
                    request = case.exchange(status)
                    event = case.emit(request, 'security.policy', 'admission', {'allowed': allowed, 'backend_unavailable': True, 'route_class': route})
                    native_timings.append({'run_id': case.configuration['authority']['run_id'], 'replica_id': 'replica-a',
                                           'candidate_sha256': case.configuration['bindings'].candidate, 'policy_version': 'v3',
                                           'request_reference': event['request_reference'], 'stage': 'limiter',
                                           'observed_utc_ns': event['observed_utc_ns'], 'duration_ns': 100,
                                           'allowed': allowed, 'backend_unavailable': True})
            else:
                status = {1: 403, 2: 403, 3: 401, 5: 503, 7: 413, 8: 403, 9: 503, 10: 403, 11: 403, 12: 403, 13: 503, 14: 503}[number]
                request = case.exchange(status, b'x' * 65537 if number == 7 else b'{}')
                if number in (1, 2, 3, 7, 11, 13):
                    producer = 'security.queue' if number == 3 else 'security.database'
                    for kind in ('state_before', 'state_after'):
                        case.emit(request, producer, kind, {'records': {'jobs': [], 'queue': [], 'audit': [], 'stats': [], 'identity': []}})
                    if number == 11:
                        case.emit(request, 'security.database', 'role', {'sqlstate': '42501', 'rolsuper': False, 'owns_schema': False})
                elif number == 5:
                    case.emit(request, 'security.redis', 'before_restart', {'instance_id': 'process-before', 'storage_id': 'disk-a'})
                    case.emit(request, 'security.redis', 'after_restart', {'instance_id': 'process-after', 'storage_id': 'disk-a', 'ping': 'PONG', 'counter_ttls_ms': [1000]})
                    case.emit(request, 'security.policy', 'cache', {'entries': []})
                elif number == 8:
                    case.emit(request, 'security.egress', 'redirect', {'location': 'http://169.254.169.254/latest', 'connected_addresses': [], 'error': 'policy: denied'})
                elif number == 9:
                    case.emit(request, 'security.secrets', 'startup', {'configured_provider': 'vault', 'loaded_provider': '', 'exit_code': 1, 'ready': False})
                elif number == 10:
                    now = time.time_ns()
                    case.emit(request, 'security.rotation', 'rotation', {
                        'activated_utc_ns': now - 1000, 'retired_utc_ns': now - 100,
                        'cache_refreshed_utc_ns': now - 999, 'revoked_utc_ns': now - 10,
                        'revocation_observed_utc_ns': now - 1,
                        'retirement_checks': [{'audience': 'private', 'accepted': False, 'observed_utc_ns': now}],
                        'unsigned_checks': [{'audience': 'private', 'accepted': False, 'observed_utc_ns': now}],
                        'outgoing_version': 'v3', 'newest_active_version': 'v3', 'audiences': ['private']})
                elif number == 12:
                    case.emit(request, 'security.telemetry', 'canaries', {'canary_env': canaries})
                elif number == 14:
                    case.emit(request, 'security.registry', 'validation', {'exit_code': 1, 'unclassified_routes': ['POST /missing'], 'manifest_routes': ['GET /public']})
            case.publish()
            cases.append(case)
        timing_path = tmp_path / 'runtime-middleware.ndjson'
        timing_path.write_bytes(b''.join(harness.canonical(event) + b'\n' for event in native_timings))
        for case in cases:
            case.configuration.update(timings_path=str(timing_path), timings_sha256=harness.sha_file(timing_path), policy_version='v3')
            case.row['facts'] = harness.native_scenario_facts(case.configuration, case.row)
            if case.number == 12:
                for kind in ('indexed_logs', 'indexed_traces'):
                    path = tmp_path / ('runtime-' + kind)
                    path.write_bytes(harness.canonical({'request_reference': case.row['facts']['request_references'][0]}) + b'\n')
                    case.row['raw_artifacts'].append({'kind': kind, 'path': str(path), 'sha256': harness.sha_file(path)})
        document = tmp_path / 'all-runtime-scenarios.json'
        configuration = cases[0].configuration
        harness.write_json(document, {'version': 1, 'run_id': configuration['authority']['run_id'],
                                      'bindings': configuration['bindings'].__dict__, 'scenarios': [case.row for case in cases]})
        configuration.update(runtime_scenarios_path=str(document), runtime_scenarios_sha256=harness.sha_file(document))
        result = harness.runtime_scenarios(configuration)
        assert [row['scenario_id'] for row in result] == list(range(1, 16))
        assert all(row['observed_assertions_passed'] is True for row in result)
        assert all(row['release_qualified'] is False for row in result)
    finally:
        for name, value in previous.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value
