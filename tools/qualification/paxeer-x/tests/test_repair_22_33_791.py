"""Real qualification clients, TLS peers, enrollment producer and build sidecars.

The node recording is emitted by rpc/sequencer_receipt_vectors_test.go. The
in-process peer persists original transactions and canonical observations; it
never signs a replacement receipt. Source admission operates on actual files.
"""
from contextlib import contextmanager
from copy import deepcopy
from datetime import datetime, timedelta, timezone
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import shutil
import ssl
import sys
import threading
from types import SimpleNamespace

import pytest
import yaml
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))


def load(name):
    spec = importlib.util.spec_from_file_location('repair_791_' + name.replace('-', '_'), QUAL / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


EVENT = load('event-delivery-contract')
PAID = load('paid-web-contract')
BUILD = PAID.BUILD
REVISION = '33e39ac9e249303769aad2d7f4de272148932e12'


def private(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(value if isinstance(value, bytes) else value.encode())
    path.chmod(0o600)
    return path


def recording():
    document = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert document['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    return deepcopy(next(row for row in document['cases'] if row['id'] == 'V1-layerxledger-pay-exact'))


def receipt_pins(row):
    pin = row['pinnedKey']
    return [{'key_id': pin['keyId'][2:], 'public_key': pin['publicKey'][2:],
             'first_height': pin['firstHeight'], 'last_height': pin['lastHeight']}]


@pytest.fixture
def tls_material(tmp_path):
    directory = tmp_path / 'tls'
    directory.mkdir(mode=0o700)
    now = datetime.now(timezone.utc)
    ca_key = ec.generate_private_key(ec.SECP256R1())
    ca_name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'Qualification test root')])
    ca = (x509.CertificateBuilder().subject_name(ca_name).issuer_name(ca_name)
          .public_key(ca_key.public_key()).serial_number(x509.random_serial_number())
          .not_valid_before(now - timedelta(minutes=1)).not_valid_after(now + timedelta(days=1))
          .add_extension(x509.BasicConstraints(ca=True, path_length=0), critical=True)
          .sign(ca_key, hashes.SHA256()))
    ca_path = private(directory / 'ca.pem', ca.public_bytes(serialization.Encoding.PEM))
    result = {'ca': ca_path}
    for role in ('server', 'producer', 'operator', 'roleless'):
        role_dir = directory / role
        role_dir.mkdir(mode=0o700)
        key = ec.generate_private_key(ec.SECP256R1())
        names = [x509.DNSName('localhost')] if role == 'server' else []
        if role in ('producer', 'operator'):
            names.append(x509.UniformResourceIdentifier('urn:layerx:webhooks:role:' + role))
        builder = (x509.CertificateBuilder().subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, role)]))
                   .issuer_name(ca_name).public_key(key.public_key()).serial_number(x509.random_serial_number())
                   .not_valid_before(now - timedelta(minutes=1)).not_valid_after(now + timedelta(days=1))
                   .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
                   .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH if role == 'server'
                                                         else ExtendedKeyUsageOID.CLIENT_AUTH]), critical=False))
        if names:
            builder = builder.add_extension(x509.SubjectAlternativeName(names), critical=False)
        certificate = builder.sign(ca_key, hashes.SHA256())
        private(role_dir / 'cert.pem', certificate.public_bytes(serialization.Encoding.PEM))
        private(role_dir / 'key.pem', key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
                                                       serialization.NoEncryption()))
        result[role] = role_dir
    return result


class EventPeer:
    """A durable in-process source/node peer with actual TLS authentication."""
    def __init__(self, path, materials):
        self.path, self.materials = path, materials
        self.record = recording()
        self.key = b'qualification-enrollment-key'
        self.source_token, self.producer_token, self.node_token = 'source-reader-recording', 'source-producer-recording', 'paxd-recording'
        self.data = json.loads(path.read_text()) if path.exists() else {'generation': 0, 'principals': {}, 'events': {}, 'accepted': {}}
        self.requests, self.errors = [], []
        self.damage = None
        self.server = self.thread = None

    def persist(self):
        private(self.path, json.dumps(self.data, sort_keys=True))

    def dispatch(self, handler, raw):
        certificate = x509.load_der_x509_certificate(handler.connection.getpeercert(binary_form=True))
        try:
            roles = certificate.extensions.get_extension_for_class(x509.SubjectAlternativeName).value.get_values_for_type(x509.UniformResourceIdentifier)
        except x509.ExtensionNotFound:
            roles = []
        authorization = handler.headers.get('Authorization')
        self.requests.append((handler.command, handler.path, authorization, tuple(roles), raw))
        if handler.path == '/':
            assert handler.headers['Content-Type'] == 'application/json'
            assert authorization == 'Bearer ' + self.node_token
            request = json.loads(raw)
            assert set(request) == {'jsonrpc', 'id', 'method', 'params'} and request['jsonrpc'] == '2.0'
            method, params = request['method'], request['params']
            reference = self.record['receiptJson']['txHash']
            if method == 'eth_chainId':
                assert params == []
                value = '0x7d'
            elif method == 'eth_sendRawTransaction':
                assert params == [self.record['rawTransaction']]
                self.data['accepted'][reference] = params[0]
                self.persist()
                value = reference
            elif method == 'eth_getRawTransactionByHash':
                assert params == [reference]
                value = self.data['accepted'].get(reference)
                if self.damage == 'raw':
                    value = '0x00'
            elif method == 'pax_sequencerReceipt':
                assert params == [reference] and reference in self.data['accepted']
                value = deepcopy(self.record['receiptJson'])
                if self.damage == 'signature':
                    value['receipt'] = value['receipt'][:-2] + ('00' if value['receipt'][-2:] != '00' else '01')
                elif self.damage == 'json':
                    value['seq'] = hex(int(value['seq'], 16) + 1)
            else:
                raise AssertionError(method)
            return 200, {'jsonrpc': '2.0', 'id': request['id'] + (1 if self.damage == 'id' else 0), 'result': value}
        if handler.path == '/v1/session':
            return 200, {'result': {'principal': 'enrolled-principal'}}
        if handler.path == '/readyz':
            if handler.headers.get('Cookie'):
                assert handler.headers['X-LayerX-CSRF'] == 'recorded-csrf'
                assert '__Host-layerx_access=recorded-access' in handler.headers['Cookie']
                assert handler.headers['Origin'] == self.url
            return (200 if self.data['principals'] else 503), {'ready': bool(self.data['principals']), 'generation': self.data['generation']}
        if handler.path == '/internal/v1/enrollment':
            assert handler.headers['Content-Type'] == 'application/json'
            if authorization != 'Bearer ' + self.producer_token or 'urn:layerx:webhooks:role:producer' not in roles:
                return 403, {'error': {'code': 'producer_role_required'}}
            document = json.loads(raw)
            message = f"layerx-enrollment-v1\njourneys\n{document['generation']}\n".encode()
            principals = {}
            for item in document['principals']:
                credential = Path(item['credential_file']).read_bytes().rstrip(b'\r\n')
                message += item['principal'].encode() + b'\n' + hashlib.sha256(credential).hexdigest().encode() + b'\n'
                principals[item['principal']] = credential.decode()
            if not hmac.compare_digest(document['mac'], hmac.new(self.key, message, hashlib.sha256).hexdigest()):
                return 403, {'error': {'code': 'enrollment_signature_invalid'}}
            if document['generation'] <= self.data['generation']:
                return 409, {'error': {'code': 'enrollment_generation_stale'}}
            self.data.update(generation=document['generation'], principals=principals)
            self.persist()
            return 200, {'generation': document['generation'], 'principals': len(principals)}
        if handler.path == '/internal/v1/observe':
            assert handler.headers['Content-Type'] == 'application/json'
            if authorization != 'Bearer ' + self.producer_token or 'urn:layerx:webhooks:role:producer' not in roles:
                return 403, {'error': {'code': 'producer_role_required'}}
            value = json.loads(raw)
            assert value['id'] == EVENT.event_id(value['kind'], value['resource'], value['sequence'])
            if value['principal'] not in self.data['principals']:
                return 403, {'error': {'code': 'principal_not_enrolled'}}
            canonical = {'id': value['id'], 'principal': value['principal'], 'subject': value['resource'],
                         'subject_sequence': value['sequence'], 'occurred_at': value['occurred_at'], 'facts': value['facts']}
            if canonical['id'] in self.data['events']:
                assert self.data['events'][canonical['id']] == canonical
            self.data['events'][canonical['id']] = canonical
            self.persist()
            return 200, canonical
        if handler.path.startswith('/internal/v1/events/'):
            if authorization != 'Bearer ' + self.source_token:
                return 401, {'error': {'code': 'authentication_required'}}
            identifier = handler.path.rsplit('/', 1)[1]
            return 200, self.data['events'][identifier]
        raise AssertionError(handler.path)

    def start(self):
        peer = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def respond(self):
                raw = self.rfile.read(int(self.headers.get('Content-Length', '0')))
                try:
                    status, value = peer.dispatch(self, raw)
                except Exception as error:
                    peer.errors.append(error)
                    status, value = 500, {'error': type(error).__name__}
                data = json.dumps(value, separators=(',', ':')).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                if self.path == '/v1/session':
                    for name in ('access', 'refresh', 'csrf'):
                        self.send_header('Set-Cookie', '__Host-layerx_' + name + '=recorded-' + name
                                         + '; Secure; Path=/; SameSite=Strict' + ('; HttpOnly' if name != 'csrf' else ''))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                self.respond()

            def do_POST(self):
                self.respond()
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(str(self.materials['server'] / 'cert.pem'), str(self.materials['server'] / 'key.pem'))
        context.load_verify_locations(str(self.materials['ca']))
        context.verify_mode = ssl.CERT_REQUIRED
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        self.port = self.server.server_port
        self.url = 'https://localhost:' + str(self.port)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        assert not self.thread.is_alive() and not self.errors
        self.server = self.thread = None

    def endpoint(self, role='producer', bearer=None):
        result = {'url': self.url, 'ca_pem': str(self.materials['ca']),
                  'client_cert_pem': str(self.materials[role] / 'cert.pem'),
                  'client_key_pem': str(self.materials[role] / 'key.pem')}
        if bearer is not None:
            location = private(self.path.parent / (role + '-bearer'), 'Bearer ' + bearer)
            result['header_files'] = {'Authorization': str(location)}
        return result

    def producer_configuration(self):
        token = private(self.path.parent / 'paxd-token', self.node_token)
        policy = private(self.path.parent / 'chain-policy.json', json.dumps({'chain_id': 'paxeer-125',
                         'validators': [{'public_key': list(bytes.fromhex(self.record['pinnedKey']['publicKey'][2:])), 'voting_power': 1}]}))
        return {'node_url': self.url, 'comet_url': self.url, 'node_token_file': str(token),
                'chain_policy_file': str(policy), 'receipt_keys': receipt_pins(self.record),
                'client_cert_pem': str(self.materials['producer'] / 'cert.pem'),
                'client_key_pem': str(self.materials['producer'] / 'key.pem')}


@pytest.fixture
def peer(tmp_path, tls_material):
    peer = EventPeer(tmp_path / 'state.json', tls_material).start()
    try:
        yield peer
    finally:
        if peer.server is not None:
            peer.stop()


def source_client(peer, role='producer'):
    client = EVENT.Source(peer.materials['ca'], peer.materials[role], peer.source_token, peer.producer_token, [])
    client.process = SimpleNamespace(port=peer.port)
    return client


def rpc_producer(peer):
    configuration = peer.producer_configuration()
    producer = EVENT.PaxdReceiptProducer(configuration, peer.materials['ca'])
    return producer


def test_current_inventory_preserves_private_roles_and_durable_volumes(capsys):
    results = EVENT.Results()
    EVENT.deployment_topology(results)
    assert results.failures == 0 and results.passes > 0
    output = capsys.readouterr().out
    for label in ('all five deployment groups retain independent persistent volumes',
                  'public keeps private TLS and role-only protected mounts',
                  'ingress keeps private TLS and role-only protected mounts',
                  'edge routes webhooks only to the public service'):
        assert 'PASS principal provisioning: ' + label in output


def deployment_copy(tmp_path):
    for name in ('internal/deployment.yaml', 'webhooks/deployment.yaml', 'webhooks/env-init.sh'):
        path = tmp_path / 'services/platform/hosted' / name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / 'services/platform/hosted' / name, path)
    return tmp_path


@pytest.mark.parametrize('damage', ['public-internal', 'shared-volume', 'missing-volume', 'writable-secret',
                                   'world-secret', 'host-network', 'host-port', 'wrong-listener',
                                   'merged-roles', 'public-trigger', 'ingress-identity', 'edge-ingress', 'webhook-public-service',
                                   'external-alias', 'alias-namespace'])
def test_topology_rejects_each_product_security_regression(tmp_path, damage):
    root = deployment_copy(tmp_path)
    path = root / 'services/platform/hosted' / ('internal' if damage in {
        'public-internal', 'shared-volume', 'missing-volume', 'writable-secret', 'world-secret', 'host-network', 'host-port', 'wrong-listener', 'external-alias', 'alias-namespace'
    } else 'webhooks') / 'deployment.yaml'
    documents = list(yaml.safe_load_all(path.read_text()))
    internal = next(row for row in documents if row['kind'] == 'Deployment')
    if damage == 'public-internal':
        next(row for row in documents if row['kind'] == 'Service')['spec']['type'] = 'LoadBalancer'
    elif damage in ('external-alias', 'alias-namespace'):
        alias = next(row for row in documents if row['kind'] == 'Service' and row['metadata']['name'] == 'identity')
        if damage == 'external-alias':
            alias['spec']['externalName'] = 'untrusted.example'
        else:
            alias['metadata']['namespace'] = 'public'
    elif damage in ('shared-volume', 'missing-volume'):
        selected = next(row for row in documents if row['kind'] == 'Deployment' and row['metadata']['name'] == 'journeys')
        selected['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName'] = 'kms-state' if damage == 'shared-volume' else 'absent'
    elif damage == 'writable-secret':
        internal['spec']['template']['spec']['containers'][0]['volumeMounts'][1]['readOnly'] = False
    elif damage == 'world-secret':
        internal['spec']['template']['spec']['volumes'][1]['secret']['defaultMode'] = 0o644
    elif damage == 'host-network':
        internal['spec']['template']['spec']['hostNetwork'] = True
    elif damage == 'host-port':
        internal['spec']['template']['spec']['containers'][0]['ports'][0]['hostPort'] = 9443
    elif damage == 'wrong-listener':
        internal['spec']['template']['spec']['containers'][0]['env'][0]['value'] = '0.0.0.0:80'
    elif damage in ('merged-roles', 'public-trigger', 'ingress-identity'):
        role = 'ingress' if damage == 'ingress-identity' else 'public'
        selected = next(row for row in documents if row['kind'] == 'Deployment'
                        and row['spec']['template']['metadata'].get('labels', {}).get('role') == role)
        pod = selected['spec']['template']['spec']
        if damage == 'merged-roles':
            selected['spec']['template']['metadata']['labels']['role'] = 'ingress'
        else:
            suffix = 'identity' if damage == 'ingress-identity' else 'source-trigger'
            pod['volumes'][0]['projected']['sources'][0]['secret']['items'].append({'key': suffix, 'path': 'tokens/' + suffix})
    elif damage == 'edge-ingress':
        selected = next(row for row in documents if row['kind'] == 'Ingress'
                        and any(p['path'] == '/v1/webhooks' for rule in row['spec']['rules'] for p in rule['http']['paths']))
        next(p for rule in selected['spec']['rules'] for p in rule['http']['paths'] if p['path'] == '/v1/webhooks')['backend']['service']['name'] = 'layerx-webhooks'
    elif damage == 'webhook-public-service':
        next(row for row in documents if row['kind'] == 'Service' and row['spec'].get('selector', {}).get('app') == 'layerx-webhooks')['spec']['type'] = 'NodePort'
    path.write_text(yaml.safe_dump_all(documents, sort_keys=False))
    results = EVENT.Results()
    with pytest.raises(EVENT.Missing):
        EVENT.deployment_topology(results, root)
    assert results.failures == 1


def test_enrollment_authorization_redelivery_and_restart_use_real_tls_clients(peer, tmp_path):
    source = source_client(peer)
    assert source.ready() == (503, {'ready': False, 'generation': 0})
    enrollment = EVENT.Enrollment(tmp_path, 'journeys', peer.key)
    credential = enrollment.credential(b'issued-recording-credential')
    document = enrollment.document(1, [('enrolled-principal', credential)])
    enrollment.write(document)
    api = EVENT.ProvisioningHttp(peer.endpoint(bearer=peer.producer_token))
    assert api.request('POST', '/internal/v1/enrollment', json.loads(enrollment.path.read_text())) == (200, {'generation': 1, 'principals': 1})
    assert source.ready() == (200, {'ready': True, 'generation': 1})
    raw = EVENT.observation('journeys', 'enrolled-principal', 'owned-journey', 1)
    status, _, event = source.produce(raw)
    assert status == 200 and event['id'] == EVENT.event_id('journey', 'owned-journey', 1)
    assert source.event(event['id']) == (200, None, event)
    assert source.call('GET', '/internal/v1/events/' + event['id'], 'wrong-reader')[:2] == (401, 'authentication_required')
    assert source_client(peer, 'operator').produce(raw)[:2] == (403, 'producer_role_required')
    assert source_client(peer, 'roleless').produce(raw)[:2] == (403, 'producer_role_required')
    assert source.produce(raw) == (200, None, event)
    assert len(peer.data['events']) == 1
    peer.stop()
    reopened = EventPeer(peer.path, peer.materials).start()
    try:
        source = source_client(reopened)
        assert source.ready() == (200, {'ready': True, 'generation': 1})
        assert source.event(event['id']) == (200, None, event)
        assert source.produce(raw) == (200, None, event)
        assert reopened.data['principals'] == {'enrolled-principal': 'issued-recording-credential'}
        assert len(reopened.data['events']) == 1
        api = EVENT.ProvisioningHttp(reopened.endpoint(bearer=reopened.producer_token))
        assert api.request('POST', '/internal/v1/enrollment', document)[0] == 409
        rotated = enrollment.document(2, [('enrolled-principal', credential)])
        bad = dict(rotated, mac='00' * 32)
        assert api.request('POST', '/internal/v1/enrollment', bad)[0] == 403
        assert reopened.data['generation'] == 1
        assert api.request('POST', '/internal/v1/enrollment', rotated)[0] == 200
        assert source.ready() == (200, {'ready': True, 'generation': 2})
    finally:
        reopened.stop()


def test_provisioning_cookie_security_and_header_shapes(peer):
    client = EVENT.ProvisioningHttp(peer.endpoint())
    assert client.request('POST', '/v1/session', {'principal': 'enrolled-principal'})[0] == 200
    assert set(client.cookies) == {'__Host-layerx_access', '__Host-layerx_refresh', '__Host-layerx_csrf'}
    assert client.request('GET', '/readyz')[0] == 503
    assert peer.requests[0][4] == b'{"principal":"enrolled-principal"}'


@pytest.mark.parametrize('retired', ['layerxd', 'layerx-guarantor', 'layerx-human-kms', 'layerx-human-security-provider',
                                    'layerx-human-movement-provider', 'layerx-human-identity-provider', 'layerx-human-onboarding',
                                    'layerx-agent-boundary'])
def test_provisioning_rejects_every_retired_executable(retired):
    artifacts = dict.fromkeys(['paxd', 'node', 'layerx-runtime-clock'], {})
    EVENT.provisioning_executables(artifacts)
    with pytest.raises(EVENT.Missing, match='retired executable'):
        EVENT.provisioning_executables(dict(artifacts, **{retired: {}}))


def test_receipt_configuration_requires_paxd_and_current_governed_keys(peer):
    configuration = peer.producer_configuration()
    environment = EVENT.receipt_environment(configuration)
    assert set(environment) == {'LAYERX_WEBHOOKS_NODE_URL', 'LAYERX_WEBHOOKS_COMET_URL',
                                'LAYERX_WEBHOOKS_NODE_TOKEN_FILE', 'LAYERX_WEBHOOKS_CHAIN_POLICY_FILE'}
    assert not any('SEQUENCER' in key or 'AUTHORITY' in key for key in environment)
    with pytest.raises(EVENT.Missing, match='merged paxd'):
        EVENT.provisioning_executables({'node': {}, 'layerx-runtime-clock': {}})


@pytest.mark.parametrize('damage', ['retired', 'http', 'path', 'userinfo', 'missing-pins', 'key-window', 'key-id', 'duplicate-key', 'unprotected-token'])
def test_receipt_configuration_refuses_retired_or_unbound_inputs(peer, damage):
    configuration = peer.producer_configuration()
    if damage == 'retired':
        configuration['sequencer_public_key'] = '11' * 32
    elif damage == 'http':
        configuration['node_url'] = configuration['node_url'].replace('https:', 'http:')
    elif damage == 'path':
        configuration['node_url'] += '/evm'
    elif damage == 'userinfo':
        configuration['node_url'] = configuration['node_url'].replace('https://', 'https://user@')
    elif damage == 'missing-pins':
        configuration['receipt_keys'] = []
    elif damage == 'key-window':
        configuration['receipt_keys'][0]['last_height'] = 0
    elif damage == 'key-id':
        configuration['receipt_keys'][0]['key_id'] = 'not-a-key'
    elif damage == 'duplicate-key':
        configuration['receipt_keys'] *= 2
    elif damage == 'unprotected-token':
        Path(configuration['node_token_file']).chmod(0o644)
    with pytest.raises(EVENT.Missing):
        EVENT.receipt_environment(configuration)


def test_paxd_signed_receipt_transport_and_exact_retry_survive_reopen(peer):
    producer = rpc_producer(peer)
    reference = peer.record['receiptJson']['txHash']
    assert producer.node.call('eth_sendRawTransaction', [peer.record['rawTransaction']]) == reference
    verified = producer.receipt(reference)
    assert verified.canonical_bytes.hex() == peer.record['receiptJson']['receipt'][2:]
    assert verified.height == int(peer.record['receiptJson']['height'], 16)
    peer.stop()
    reopened = EventPeer(peer.path, peer.materials).start()
    try:
        recovered = rpc_producer(reopened)
        assert recovered.receipt(reference).canonical_bytes == verified.canonical_bytes
        assert recovered.node.call('eth_sendRawTransaction', [peer.record['rawTransaction']]) == reference
        assert reopened.data['accepted'] == {reference: peer.record['rawTransaction']}
    finally:
        reopened.stop()


@pytest.mark.parametrize('damage', ['signature', 'json', 'raw', 'id', 'pin'])
def test_paxd_event_receipt_refuses_substitution(peer, damage):
    producer = rpc_producer(peer)
    reference = peer.record['receiptJson']['txHash']
    producer.node.call('eth_sendRawTransaction', [peer.record['rawTransaction']])
    if damage == 'pin':
        producer.node.receipt_keys[0]['public_key'] = '11' * 32
    else:
        peer.damage = damage
    from layerx_sdk.verifier import SequencerReceiptVerificationError
    with pytest.raises((EVENT.Missing, ValueError, RuntimeError, SequencerReceiptVerificationError)):
        producer.receipt(reference)
    assert peer.data['accepted'] == {reference: peer.record['rawTransaction']}


@pytest.fixture
def built_source(tmp_path):
    checkout = tmp_path / 'checkout'
    checkout.mkdir()
    # Each fixture source is copied verbatim from a real producer/build/consumer.
    names = ['go.mod', 'go.sum', 'Makefile', 'chain.mk', 'rpc/sequencer_receipt.go',
             'tools/qualification/paxeer-x/paid-web-contract.py', 'services/platform/hosted/webhooks/src/trusted.rs']
    for name in names:
        path = checkout / name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, path)
    candidate = {'revision': REVISION, 'tree': 'a1' * 20, 'mainline_revision': REVISION,
                 'integrated': True, 'dirty': False, 'release_credit': False}
    source = BUILD.capture_files(checkout, candidate, names)
    output = tmp_path / 'private'
    output.mkdir(mode=0o700)
    artifact = output / 'recorded-producer-source'
    shutil.copy2(ROOT / names[-1], artifact)
    artifact.chmod(0o700)
    log = private(output / 'build.log', 'recorded source artifact copied byte-for-byte\n')
    sidecar = BUILD.publish(output / 'build.json', source, deepcopy(source), ['recorded-source-copy', names[-1]], 0, log, [artifact])
    row = {'path': str(artifact), 'sha256': BUILD.file_digest(artifact), 'source_revision': REVISION,
           'source_digest': BUILD.source_digest(source), 'build_exit': 0, 'build_sidecar': sidecar}
    return checkout, source, row, output


def test_complete_source_build_producer_transport_and_paid_consumer_reopen(built_source):
    checkout, source, row, _ = built_source
    assert BUILD.verify_files(checkout, source) == source
    assert PAID.artifact(row, REVISION, BUILD.source_digest(source), checkout) == Path(row['path'])
    reopened = load('paid-web-contract')
    restored = json.loads(json.dumps(row))
    assert reopened.artifact(restored, REVISION, BUILD.source_digest(source), checkout).read_bytes() == Path(row['path']).read_bytes()
    assert set(source['files']) >= {'go.mod', 'go.sum', 'Makefile', 'chain.mk', 'rpc/sequencer_receipt.go',
                                   'tools/qualification/paxeer-x/paid-web-contract.py'}


@pytest.mark.parametrize('name', ['go.mod', 'go.sum', 'Makefile', 'chain.mk', 'rpc/sequencer_receipt.go',
                                 'tools/qualification/paxeer-x/paid-web-contract.py', 'services/platform/hosted/webhooks/src/trusted.rs'])
def test_paid_consumer_refuses_actual_source_change_even_with_claimed_clean_manifest(built_source, name):
    checkout, source, row, output = built_source
    path = checkout / name
    path.write_bytes(path.read_bytes() + b'\nchanged checkout\n')
    assert source['candidate']['dirty'] is False
    with pytest.raises(PAID.Refusal, match='candidate source changed'):
        PAID.artifact(row, REVISION, BUILD.source_digest(source), checkout)
    assert not (output / 'assertions.jsonl').exists()


@pytest.mark.parametrize('damage', ['source', 'dirty', 'changed-build', 'exit', 'command', 'log', 'artifact', 'sidecar', 'missing-sidecar', 'row-revision', 'row-source', 'row-exit', 'mode', 'symlink'])
def test_paid_consumer_refuses_each_build_binding_failure(built_source, damage):
    checkout, source, row, output = built_source
    sidecar_path = Path(row['build_sidecar']['path'])
    value = json.loads(sidecar_path.read_text())
    if damage == 'source':
        value['source_start']['candidate']['revision'] = '00' * 20
    elif damage == 'dirty':
        value['source_start']['candidate']['dirty'] = True
        value['source_end'] = deepcopy(value['source_start'])
        row['source_digest'] = BUILD.source_digest(value['source_start'])
    elif damage == 'changed-build':
        value['source_end']['candidate']['tree'] = '00' * 20
    elif damage == 'exit':
        value['exit'] = 1
    elif damage == 'command':
        value['argv'] = []
    elif damage == 'log':
        Path(value['log']['path']).write_text('changed build evidence')
    elif damage == 'artifact':
        Path(row['path']).write_text('changed artifact')
    elif damage == 'sidecar':
        sidecar_path.write_text('{}')
    elif damage == 'missing-sidecar':
        del row['build_sidecar']
    elif damage == 'row-revision':
        row['source_revision'] = '00' * 20
    elif damage == 'row-source':
        row['source_digest'] = '00' * 32
    elif damage == 'row-exit':
        row['build_exit'] = 1
    elif damage == 'mode':
        sidecar_path.chmod(0o644)
    elif damage == 'symlink':
        (checkout / 'go.mod').unlink()
        (checkout / 'go.mod').symlink_to(checkout / 'go.sum')
    if damage in {'source', 'dirty', 'changed-build', 'exit', 'command'}:
        sidecar_path.write_bytes(BUILD.canonical(value))
        row['build_sidecar']['sha256'] = BUILD.file_digest(sidecar_path)
    with pytest.raises((PAID.Refusal, ValueError, KeyError, OSError)):
        PAID.artifact(row, REVISION, row['source_digest'] if damage == 'dirty' else BUILD.source_digest(source), checkout)


@pytest.mark.parametrize('damage', ['dirty', 'changed', 'failed'])
def test_build_producer_never_publishes_dirty_changed_or_failed_evidence(built_source, damage):
    _, source, row, output = built_source
    before, after = deepcopy(source), deepcopy(source)
    exit_code = 0
    if damage == 'dirty':
        before['candidate']['dirty'] = after['candidate']['dirty'] = True
    elif damage == 'changed':
        after['candidate']['tree'] = '00' * 20
    else:
        exit_code = 1
    target = output / 'refused-build.json'
    with pytest.raises(ValueError):
        BUILD.publish(target, before, after, ['recorded-source-copy'], exit_code, output / 'build.log', [row['path']])
    assert not target.exists()


def test_all_nested_artifact_roles_use_the_same_build_binding(built_source):
    checkout, source, row, _ = built_source
    manifest = {'artifacts': {'websearch': row}, 'gateway': {'artifact': deepcopy(row)},
                'relays': [{'artifact': deepcopy(row)}], 'reference_program': deepcopy(row)}
    rows = list(PAID.bound_build_rows(manifest))
    assert len(rows) == 4
    for artifact in rows:
        assert PAID.artifact(artifact, REVISION, BUILD.source_digest(source), checkout) == Path(row['path'])
    manifest['gateway']['artifact']['source_digest'] = '00' * 32
    with pytest.raises(PAID.Refusal):
        for artifact in PAID.bound_build_rows(manifest):
            PAID.artifact(artifact, REVISION, BUILD.source_digest(source), checkout)
