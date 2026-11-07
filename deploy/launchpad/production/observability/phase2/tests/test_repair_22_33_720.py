"""Connected bundle proof with loopback origin, rule and alert protocol peers.

The peers evaluate the admitted synthetic expression and owner routes in process;
no native Prometheus/Alertmanager installation or external service is required.
"""
from contextlib import contextmanager
from copy import deepcopy
from datetime import datetime, timezone
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
import secrets
import struct
import sys
import threading

import pytest

PHASE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PHASE))
from contracts import Invalid, canonical, inventory, load, sha, validate_binding
from probes import exposition, read_exact, run_once, write_observations
from render import atomic_write, immutable_bundle, render, verify_bundle
from runtime import fetch, prometheus_samples
from synthetic import metrics_server


class JSONPeer(BaseHTTPRequestHandler):
    def reply(self, status, document):
        raw = canonical(document)
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def document(self):
        assert self.headers['Content-Type'] == 'application/json'
        return json.loads(self.rfile.read(int(self.headers['Content-Length'])))

    def log_message(self, *_args):
        pass


@contextmanager
def serve(handler):
    with ThreadingHTTPServer(('127.0.0.1', 0), handler) as server:
        with running(server):
            yield 'http://127.0.0.1:' + str(server.server_port)


@contextmanager
def running(server):
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        thread.join(timeout=3)
        assert not thread.is_alive()


class Origin(JSONPeer):
    protocol_version = 'HTTP/1.1'
    quote_available = False
    requests = []

    def do_GET(self):
        type(self).requests.append((self.path, dict(self.headers), None))
        if self.path == '/socket':
            self.socket_replay()
        elif self.path == '/quote' and not self.quote_available:
            self.reply(503, {'error': {'code': 'unavailable'}})
        else:
            self.reply(200, {'ready': True})

    def do_POST(self):
        body = self.document()
        type(self).requests.append((self.path, dict(self.headers), body))
        assert self.path == '/v2/auth/challenges'
        assert set(body) == {'wallet', 'origin'}
        assert body['origin'] == self.headers['Origin']
        self.reply(201, {'challengeId': 'qualification-challenge', 'message': 'Sign in'})

    def socket_replay(self):
        self.close_connection = True
        assert self.headers['Authorization'].startswith('Bearer ')
        assert self.headers['Sec-WebSocket-Protocol'] == 'sidiora.v2'
        accept = base64.b64encode(hashlib.sha1(
            (self.headers['Sec-WebSocket-Key'] + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()
        ).digest()).decode()
        self.send_response(101)
        self.send_header('Upgrade', 'websocket')
        self.send_header('Connection', 'Upgrade')
        self.send_header('Sec-WebSocket-Accept', accept)
        self.send_header('Sec-WebSocket-Protocol', 'sidiora.v2')
        self.end_headers()
        first, second = read_exact(self.rfile, 2)
        assert first == 0x81 and second & 0x80
        length = second & 0x7f
        if length == 126:
            length = struct.unpack('!H', read_exact(self.rfile, 2))[0]
        mask = read_exact(self.rfile, 4)
        raw = read_exact(self.rfile, length)
        frame = json.loads(bytes(value ^ mask[i % 4] for i, value in enumerate(raw)))
        assert frame == {'op': 'subscribe', 'afterCursor': 'admitted-checkpoint'}
        for document in ({'op': 'subscribed'}, {'op': 'checkpoint', 'cursor': 'next-checkpoint'}):
            payload = canonical(document)
            self.wfile.write(bytes((0x81, len(payload))) + payload)
            self.wfile.flush()


def authorized(destination):
    return {'Authorization': 'Bearer ' + Path(destination['token_file']).read_text().strip()}


def duration(value):
    match = re.fullmatch(r'(\d+)(s|m)', value)
    assert match is not None
    return int(match[1]) * (60 if match[2] == 'm' else 1)


class RulePeer:
    """In-process vector evaluator for the emitted synthetic rule's operators."""
    def __init__(self, bundle, config):
        self.prometheus = load(bundle/'prometheus.json')
        rules = load(bundle/self.prometheus['rule_files'][0])
        self.rule = next(rule for group in rules['groups'] for rule in group['rules']
                         if rule.get('alert') == 'synthetic_failed')
        self.config = config
        self.pending = {}
        self.scraped = None

    def evaluate(self, now):
        job, = [job for job in self.prometheus['scrape_configs']
                if job['job_name'] == 'synthetic-journeys']
        target, = job['static_configs'][0]['targets']
        status, raw, _ = fetch(job['scheme']+'://'+target+job['metrics_path'],
                               headers=authorized({'token_file': job['authorization']['credentials_file']}))
        assert status == 200
        self.scraped = raw
        samples = prometheus_samples(raw, self.config)
        # Parse the whole expression, including the missing-journey join and age
        # condition. The fixture never manufactures a preselected firing alert.
        match = re.fullmatch(
            r'(\w+) == (\d+) or \((\w+) unless on\(([^)]+)\) (\w+)\) or '
            r'\(time\(\) - (\w+) > (\d+)\)', self.rule['expr'])
        assert match is not None, self.rule['expr']
        success, failed_value, expected, join, observed_success, timestamp, age = match.groups()
        vector = lambda name: {labels: value for (metric, labels), value in samples.items() if metric == name}
        failed = {labels: value for labels, value in vector(success).items() if value == float(failed_value)}
        join_keys = join.split(',')
        identity = lambda labels: tuple(dict(labels).get(key) for key in join_keys)
        present = {identity(labels) for labels in vector(observed_success)}
        failed.update({labels: value for labels, value in vector(expected).items() if identity(labels) not in present})
        failed.update({labels: now-value for labels, value in vector(timestamp).items() if now-value > float(age)})
        self.pending = {labels: self.pending.get(labels, now) for labels in failed}
        alerts = []
        for labels, started in self.pending.items():
            if now-started < duration(self.rule['for']):
                continue
            labels = {**dict(labels), **self.rule['labels'],
                      'alertname': self.rule['alert'], **self.prometheus['global']['external_labels']}
            annotations = {key: value.replace('{{ $externalLabels.deployment_digest }}', labels['deployment_digest'])
                           for key, value in self.rule['annotations'].items()}
            alerts.append({'labels': labels, 'annotations': annotations,
                           'startsAt': datetime.fromtimestamp(started, timezone.utc).isoformat(),
                           'endsAt': datetime.fromtimestamp(now+120, timezone.utc).isoformat(),
                           'generatorURL': 'http://localhost/graph'})
        return alerts

    def notify(self, alerts):
        target, = self.prometheus['alerting']['alertmanagers']
        address, = target['static_configs'][0]['targets']
        url = target['scheme']+'://'+address+target['path_prefix'].rstrip('/')+'/api/v2/alerts'
        return fetch(url, method='POST', body=canonical(alerts),
                     headers={'Content-Type': 'application/json',
                              **authorized({'token_file': target['authorization']['credentials_file']})})[0]


@contextmanager
def deployment(tmp_path):
    config = load(PHASE.parent/'config-v2.json')
    credentials = {}
    for name in ('scrape', 'manager', 'responder', 'session'):
        path = tmp_path/(name+'.token')
        atomic_write(path, secrets.token_urlsafe(32).encode())
        credentials[name] = str(path)
    received, delivered = [], []
    state = {}

    class Responder(JSONPeer):
        def do_POST(self):
            if self.headers.get('Authorization') != 'Bearer '+Path(credentials['responder']).read_text():
                self.reply(401, {'error': 'unauthorized'})
                return
            delivered.append((self.path, self.document()))
            self.reply(200, {})

    class Manager(JSONPeer):
        def do_POST(self):
            if self.headers.get('Authorization') != 'Bearer '+Path(credentials['manager']).read_text():
                self.reply(401, {'error': 'unauthorized'})
                return
            assert self.path == '/assigned/api/v2/alerts'
            alerts = self.document()
            received.extend(alerts)
            configuration = load(state['bundle']/'alertmanager.json')
            for alert in alerts:
                route = configuration['route']
                name = route['receiver']
                for child in route['routes']:
                    matcher, = child['matchers']
                    match = re.fullmatch(r'(\w+)="([^"]+)"', matcher)
                    assert match is not None
                    if alert['labels'].get(match[1]) == match[2]:
                        name = child['receiver']
                        break
                receiver, = [item for item in configuration['receivers'] if item['name'] == name]
                hook, = receiver['webhook_configs']
                payload = {'version': '4', 'groupKey': '{}:{alertname="synthetic_failed"}',
                           'truncatedAlerts': 0, 'status': 'firing', 'receiver': name,
                           'groupLabels': {key: alert['labels'][key] for key in route['group_by']},
                           'commonLabels': alert['labels'], 'commonAnnotations': alert['annotations'],
                           'externalURL': 'http://localhost/assigned',
                           'alerts': [{'status': 'firing', **alert, 'fingerprint': sha(canonical(alert['labels']))[:16]}]}
                status, _, _ = fetch(hook['url'], method='POST', body=canonical(payload),
                                     headers={'Content-Type': 'application/json',
                                              **authorized({'token_file': hook['http_config']['authorization']['credentials_file']})})
                assert status == 200
            self.reply(200, {})

    class JourneyOrigin(Origin):
        requests = []

    with serve(JourneyOrigin) as origin, serve(Responder) as responder, serve(Manager) as manager:
        schema = PHASE/'tests/fixtures/synthetic-response.schema.json'
        binding = {'version': '2.0', 'candidate_sha256': '1'*64, 'input_interface_sha256': '2'*64,
                   'deployment_digest': '3'*64, 'configuration_revision': config['revision'],
                   'environment': config['environment'], 'chain_id': 125,
                   'contract_addresses': {'router': '0x'+'1'*40},
                   'services': [{'service': name, 'zone': 'zone-b', 'base_url': origin, 'owner': 'platform'}
                                for name in inventory.SERVICES],
                   'prometheus_url': origin,
                   'responders': {'platform-oncall': 'assigned-responder', 'incident-commander': 'assigned-commander'},
                   'synthetic': {'url': 'http://127.0.0.1:0/metrics',
                                 'textfile_directory': str(tmp_path/'textfile'), 'zone': 'zone-b',
                                 'token_file': credentials['scrape'], 'ca_file': None, 'cert_file': None, 'key_file': None},
                   'alertmanager': {'url': manager+'/assigned', 'token_file': credentials['manager'], 'ca_file': None,
                                    'receivers': {'platform-oncall': {'url': responder+'/oncall',
                                                                    'token_file': credentials['responder'], 'ca_file': None}}},
                   'probes': []}
        for journey, path in zip(inventory.ROUTES[:5], ('/browse', '/quote', '/v2/auth/challenges', '/metadata', '/socket')):
            wallet = journey == 'wallet_challenge'
            socket = journey == 'websocket_replay'
            binding['probes'].append({'journey': journey, 'url': (origin.replace('http:', 'ws:') if socket else origin)+path,
                                      'method': 'POST' if wallet else 'GET',
                                      'body': {'wallet': '0x'+'2'*40, 'origin': origin} if wallet else None,
                                      'headers': {'Origin': origin} if wallet or socket else {},
                                      'schema_path': str(schema.relative_to(PHASE)), 'schema_sha256': sha(schema.read_bytes()),
                                      'frame': {'op': 'subscribe', 'afterCursor': 'admitted-checkpoint'} if socket else None,
                                      'token_file': credentials['session'] if socket else None})
        with metrics_server(binding, config) as exporter:
            binding['synthetic']['url'] = 'http://127.0.0.1:'+str(exporter.server_port)+'/metrics'
            bundle, manifest = immutable_bundle(binding, config, tmp_path/'bundles')
            state['bundle'] = bundle
            with running(exporter):
                yield binding, config, bundle, manifest, received, delivered, JourneyOrigin


def test_observability_synthetic_transport_failed_journey_reaches_rule_and_responder(tmp_path):
    with deployment(tmp_path) as (binding, config, bundle, manifest, received, delivered, origin):
        assert verify_bundle(binding, config, bundle) == manifest
        observations = run_once(binding, config, PHASE)
        outcomes = {row['journey']: row['success'] for row in observations}
        assert outcomes == {'browse': 1, 'quote': 0, 'wallet_challenge': 1, 'metadata': 1, 'websocket_replay': 1}
        assert fetch(binding['services'][0]['base_url']+'/health')[0] == 200
        assert any(path == '/quote' for path, _, _ in origin.requests)
        challenge, = [row for row in origin.requests if row[0] == '/v2/auth/challenges']
        assert challenge[2] == binding['probes'][2]['body']
        write_observations(binding, config, observations, binding['synthetic']['textfile_directory'])
        peer = RulePeer(bundle, config)
        started = max(row['observed_at'] for row in observations)
        assert peer.evaluate(started) == []
        assert peer.evaluate(started+29) == []
        alerts = peer.evaluate(started+30)
        assert len(alerts) == 1
        assert peer.scraped == exposition(binding, observations)
        labels = alerts[0]['labels']
        assert labels == {'journey': 'quote', 'environment': 'qualification', 'zone': 'zone-b',
                          'alertname': 'synthetic_failed', 'severity': 'critical',
                          'owner': 'platform-oncall', 'commander': 'incident-commander',
                          'deployment_digest': binding['deployment_digest']}
        assert peer.notify(alerts) == 200
        assert received == alerts
        path, notification = delivered[0]
        assert path == '/oncall'
        assert notification['receiver'] == 'platform-oncall.assigned-responder'
        assert notification['alerts'][0]['labels'] == labels
        assert notification['alerts'][0]['status'] == 'firing'
        assert notification['commonAnnotations']['deployment'] == binding['deployment_digest']
        assert len(delivered) == 1
        # Recovery really runs the client against the re-enabled route.
        origin.quote_available = True
        healthy = run_once(binding, config, PHASE)
        assert all(row['success'] == 1 for row in healthy)
        write_observations(binding, config, healthy, binding['synthetic']['textfile_directory'])
        assert peer.evaluate(max(row['observed_at'] for row in healthy)) == []


def test_bundle_binds_mounts_authentication_destinations_and_identity(tmp_path):
    with deployment(tmp_path) as (binding, config, bundle, manifest, *_):
        transport = load(bundle/'transport.json')
        mounts = transport['mounts']
        directory = binding['synthetic']['textfile_directory']
        assert {'component': 'probes', 'path': directory, 'read_only': False} in mounts
        assert {'component': 'synthetic-exporter', 'path': directory, 'read_only': True} in mounts
        for component, path in (('prometheus', binding['synthetic']['token_file']),
                                ('synthetic-exporter', binding['synthetic']['token_file']),
                                ('prometheus', binding['alertmanager']['token_file']),
                                ('alertmanager', binding['alertmanager']['receivers']['platform-oncall']['token_file'])):
            assert {'component': component, 'path': path, 'read_only': True} in mounts
        assert load(bundle/'synthetic-exporter.json') == binding['synthetic']
        assert transport['responders'] == {'platform-oncall': 'assigned-responder'}
        assert transport['probes']['output'] == directory
        assert {'alertmanager.json', 'synthetic-exporter.json', 'transport.json'} <= set(manifest['files'])
        for token in tmp_path.glob('*.token'):
            assert all(token.read_bytes() not in path.read_bytes() for path in bundle.iterdir())
        other = deepcopy(binding)
        other['alertmanager']['receivers']['platform-oncall']['url'] += '/foreign'
        with pytest.raises(Invalid, match='identity differs'):
            verify_bundle(other, config, bundle)
        for name in ('transport.json', 'synthetic-exporter.json', 'alertmanager.json', 'prometheus.json'):
            path = bundle/name
            raw = path.read_bytes()
            path.write_bytes(raw+b' ')
            with pytest.raises(Invalid, match='changed'):
                verify_bundle(binding, config, bundle)
            path.write_bytes(raw)


def test_synthetic_ingestion_authentication_and_bound_output_fail_closed(tmp_path):
    with deployment(tmp_path) as (binding, config, bundle, *_):
        endpoint = binding['synthetic']['url']
        observations = run_once(binding, config, PHASE)
        write_observations(binding, config, observations, binding['synthetic']['textfile_directory'])
        assert fetch(endpoint)[0] == 401
        assert fetch(endpoint, headers={'Authorization': 'Bearer wrong'})[0] == 401
        assert fetch(endpoint, headers=authorized(binding['synthetic']))[0] == 200
        assert fetch(endpoint+'/other', headers=authorized(binding['synthetic']))[0] == 404
        with pytest.raises(Invalid, match='output differs'):
            write_observations(binding, config, observations, tmp_path/'unbound')
        assert not (tmp_path/'unbound').exists()
        target = Path(binding['synthetic']['textfile_directory'])/'synthetic.prom'
        target.chmod(0o644)
        assert fetch(endpoint, headers=authorized(binding['synthetic']))[0] == 503
        target.chmod(0o600)
        raw = target.read_bytes()
        atomic_write(target, raw.replace(b'zone="zone-b"', b'zone="zone-a"'))
        assert fetch(endpoint, headers=authorized(binding['synthetic']))[0] == 503
        atomic_write(target, raw)
        token = Path(binding['synthetic']['token_file'])
        token.chmod(0o644)
        assert fetch(endpoint, headers=authorized(binding['synthetic']))[0] == 503
        token.chmod(0o600)
        target.unlink()
        target.symlink_to(token)
        assert fetch(endpoint, headers=authorized(binding['synthetic']))[0] == 503


def test_incomplete_or_insecure_transport_cannot_be_admitted(tmp_path):
    with deployment(tmp_path) as (binding, config, *_):
        variants = []
        for key in ('synthetic', 'alertmanager'):
            changed = deepcopy(binding)
            del changed[key]
            variants.append(changed)
        for field, value in (('token_file', 'relative'), ('textfile_directory', '/'), ('zone', 'foreign')):
            changed = deepcopy(binding)
            changed['synthetic'][field] = value
            variants.append(changed)
        changed = deepcopy(binding)
        changed['synthetic']['token_file'] = changed['synthetic']['textfile_directory']+'/credential'
        variants.append(changed)
        changed = deepcopy(binding)
        changed['alertmanager']['receivers'] = {}
        variants.append(changed)
        changed = deepcopy(binding)
        changed['alertmanager']['url'] = 'http://remote.example/assigned'
        variants.append(changed)
        for changed in variants:
            with pytest.raises(Invalid):
                render(changed, config, tmp_path/'rejected')
        assert not (tmp_path/'rejected').exists()
        production = deepcopy(binding)
        production_config = deepcopy(config)
        production['environment'] = production_config['environment'] = 'production'
        with pytest.raises(Invalid, match='TLS required'):
            validate_binding(production, production_config)


def test_https_bundle_carries_trust_and_exporter_server_mounts(tmp_path):
    with deployment(tmp_path) as (binding, config, *_):
        production, inventory_config = deepcopy(binding), deepcopy(config)
        production['environment'] = inventory_config['environment'] = 'production'
        production['prometheus_url'] = 'https://prometheus.internal'
        for service in production['services']:
            service['base_url'] = 'https://'+service['service']+'.internal'
        for probe in production['probes']:
            probe['url'] = probe['url'].replace('http:', 'https:').replace('ws:', 'wss:')
        production['synthetic'].update(url='https://synthetic.internal:9443/metrics',
                                       ca_file='/run/secrets/synthetic-ca.pem',
                                       cert_file='/run/secrets/synthetic-cert.pem',
                                       key_file='/run/secrets/synthetic-key.pem')
        production['alertmanager'].update(url='https://alerts.internal/assigned',
                                          ca_file='/run/secrets/manager-ca.pem')
        destination = production['alertmanager']['receivers']['platform-oncall']
        destination.update(url='https://responder.internal/oncall', ca_file='/run/secrets/responder-ca.pem')
        target = tmp_path/'https-bundle'
        manifest = render(production, inventory_config, target)
        assert verify_bundle(production, inventory_config, target) == manifest
        prometheus = load(target/'prometheus.json')
        scrape, = [job for job in prometheus['scrape_configs'] if job['job_name'] == 'synthetic-journeys']
        manager, = prometheus['alerting']['alertmanagers']
        assert scrape['scheme'] == manager['scheme'] == 'https'
        assert scrape['tls_config'] == {'ca_file': production['synthetic']['ca_file']}
        assert manager['tls_config'] == {'ca_file': production['alertmanager']['ca_file']}
        assert manager['path_prefix'] == '/assigned'
        mounts = load(target/'transport.json')['mounts']
        for component, paths in (('prometheus', ('synthetic-ca.pem', 'manager-ca.pem')),
                                 ('synthetic-exporter', ('synthetic-cert.pem', 'synthetic-key.pem')),
                                 ('alertmanager', ('responder-ca.pem',))):
            for path in paths:
                assert {'component': component, 'path': '/run/secrets/'+path, 'read_only': True} in mounts
        for field in ('cert_file', 'key_file', 'ca_file'):
            changed = deepcopy(production)
            changed['synthetic'][field] = None
            with pytest.raises(Invalid):
                validate_binding(changed, inventory_config)
