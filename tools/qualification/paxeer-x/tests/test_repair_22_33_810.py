"""Current clients -> real HTTP/TLS/ABI/receipt codecs -> durable in-process peers.

Signed receipts are original node conformance recordings. The peers own registry,
node and mirror stores; no qualification component, codec or verifier is mocked.
"""
import base64
from contextlib import contextmanager
from copy import deepcopy
import datetime
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import ssl
import sys
from types import SimpleNamespace
import threading

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.serialization import Encoding, NoEncryption, PrivateFormat
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
HUMAN_PATH = ROOT / 'services/platform/hosted/human'
sys.path.insert(0, str(QUAL))
sys.path.insert(0, str(HUMAN_PATH))
import merged_qualification as merged
import native_evidence as evidence
from layerx_sdk.program_lifecycle import NativeProgramDeploy
from layerx_sdk.verifier import SequencerReceiptVerificationError


def load(name, path):
    spec = importlib.util.spec_from_file_location('repair_810_' + name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


REGISTRY = load('registry', QUAL / 'registry-contract.py')
ARCHIVE = load('archive', QUAL / 'interop-archive-contract.py')
BOOTSTRAP = load('bootstrap', QUAL / 'registry-router-bootstrap.py')
HUMAN = load('human', QUAL / 'human-owner-evidence.py')
XWEB = load('xweb', QUAL / 'fixtures/xweb_fixture.py')
RECORDED = load('recorded', QUAL / 'tests/test_repair_22_33_779.py')
OWNER = load('owner', HUMAN_PATH / 'test_repair_22_31_348.py')
REVISION = '76c050242a1edcad6212aa52fac15670ecb5612d'
GENESIS = '0x' + 'aa' * 32
IMAGE = 'paxd@sha256:' + 'ab' * 32


def put(path, value):
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    path.write_bytes(value if isinstance(value, bytes) else evidence.canonical(value))
    path.chmod(0o600)
    return path


def genesis_for(node):
    return {'chain_id': 'hyperpax_125-1', 'app_state': {'sequencer': {'receipt_keys': [
        {'key_id': base64.b64encode(bytes.fromhex(key['key_id'])).decode(),
         'public_key': base64.b64encode(bytes.fromhex(key['public_key'])).decode(),
         'first_height': key['first_height'], 'last_height': key['last_height']} for key in node.receipt_keys]}}}


class NativePeer(RECORDED.Peer):
    """Original signed receipt transcripts with independently persisted block views."""
    def __init__(self, path):
        super().__init__(path)
        self.data['blocks']['0x0'] = {'number': '0x0', 'hash': GENESIS, 'transactions': [],
                                     'parentHash': '0x' + '00' * 32, 'stateRoot': '0x' + 'ab' * 32, 'timestamp': '0x0'}
        for row in self.data['blocks'].values():
            row.setdefault('parentHash', GENESIS)
            row.setdefault('timestamp', '0x6553f100')
        self.persist()

    def reopen(self):
        self.stop()
        self.start()


def install_program(peer, abi=2):
    produced = NativeProgramDeploy.decode(bytes.fromhex(RECORDED.native()['payload_hex']))
    interface = produced.interface
    offset = len(b'LayerX/program-interface/v1\0') + 32
    interface = interface[:offset] + abi.to_bytes(2, 'big') + interface[offset + 2:]
    head_prefix = b'\0\1' + bytes(4)
    head = head_prefix + hashlib.sha256(head_prefix).digest()
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getProgram', [produced.program_id],
              [produced.policy, produced.authority, produced.new_hash, abi, 1, True])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getInterface', [produced.program_id],
              [1, hashlib.sha256(interface).digest(), interface])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getStateHead', [produced.program_id], [head])
    return {'id': f'abi{abi}-restart', 'program_id': produced.program_id.hex(), 'abi': abi, 'version': 1,
            'code_hash': produced.new_hash.hex(), 'interface': interface.hex(),
            'transaction_hash': peer.program_record['receiptJson']['txHash']}


def tls_material(directory):
    directory.mkdir(mode=0o700)
    now = datetime.datetime.now(datetime.timezone.utc)
    ca_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'qualification CA')])
    ca = (x509.CertificateBuilder().subject_name(name).issuer_name(name).public_key(ca_key.public_key())
          .serial_number(x509.random_serial_number()).not_valid_before(now - datetime.timedelta(minutes=1))
          .not_valid_after(now + datetime.timedelta(days=1)).add_extension(x509.BasicConstraints(ca=True, path_length=None), True)
          .sign(ca_key, hashes.SHA256()))
    ca_file = put(directory / 'ca.pem', ca.public_bytes(Encoding.PEM))
    result = {'client_ca': str(ca_file)}
    for role, usage in [('server', ExtendedKeyUsageOID.SERVER_AUTH), ('client', ExtendedKeyUsageOID.CLIENT_AUTH)]:
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        cert = (x509.CertificateBuilder().subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, role)]))
                .issuer_name(name).public_key(key.public_key()).serial_number(x509.random_serial_number())
                .not_valid_before(now - datetime.timedelta(minutes=1)).not_valid_after(now + datetime.timedelta(days=1))
                .add_extension(x509.SubjectAlternativeName([x509.DNSName('localhost')]), False)
                .add_extension(x509.ExtendedKeyUsage([usage]), False).sign(ca_key, hashes.SHA256()))
        result[role + '_certificate'] = str(put(directory / (role + '.pem'), cert.public_bytes(Encoding.PEM)))
        result[role + '_key'] = str(put(directory / (role + '.key'), key.private_bytes(Encoding.PEM, PrivateFormat.PKCS8, NoEncryption())))
    result['request_token'] = str(put(directory / 'request.token', b'12' * 32 + b'\n'))
    result['publication_token'] = str(put(directory / 'publication.token', b'34' * 32 + b'\n'))
    return result


class RegistryPeer:
    """mTLS registry peer with persistent principal-scoped publication requests."""
    def __init__(self, path, native, case, tls):
        self.path, self.native, self.case, self.tls = path, native, case, tls
        self.source_bytes = b'LXSRCv1\0recorded source archive for the registry peer'
        self.source = {'source_uri': 'https://source.example/native.archive', 'source_digest': hashlib.sha256(self.source_bytes).hexdigest()}
        self.state = {'requests': {}, 'source': {'status': 'unpublished'}, 'builds': 0}
        self.errors, self.requests = [], []
        self.port = 0
        self.persist()
        self.start()

    def persist(self):
        put(self.path, self.state)

    def document(self):
        digest = self.native.program_record['receiptDigest'][2:]
        version = dict(version=1, code_hash=self.case['code_hash'], abi_version=self.case['abi'],
                       deployment_receipt_digest=digest, source=deepcopy(self.state['source']))
        return dict(program_id=self.case['program_id'], latest_version=1, versions=[version],
                    receipt={'deployment_receipt_digest': digest, 'verification': 'receipt-verified',
                             'observed_sequence': 1, 'observed_at': 1},
                    lifecycle='active', state_root='ab' * 32, observed_sequence=1, observed_at=1, valid_through=30001,
                    lifecycle_history=[], exit_routes=[], value_accounts=[])

    def start(self):
        self.state = json.loads(self.path.read_bytes())
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def handle_request(self):
                try:
                    assert self.connection.getpeercert(), 'authenticated TLS client'
                    assert self.headers.get_content_type() == 'application/json'
                    assert self.path in ('/v1/programs/registry/' + owner.case['program_id'],
                                         '/v1/programs/registry/' + owner.case['program_id'] + '/source')
                    body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
                    owner.requests.append((self.command, self.path, self.headers.get('Idempotency-Key'), body))
                    status, result = 200, owner.document()
                    if self.headers.get('Authorization') != 'Bearer ' + Path(owner.tls['publication_token']).read_text().strip():
                        status, result = 401, {'error': {'code': 'unauthorized'}}
                    elif self.headers.get('LayerX-Key') not in ('principal-A', 'principal-B'):
                        status, result = 403, {'error': {'code': 'unauthorized'}}
                    elif self.command == 'POST':
                        request = json.loads(body)
                        key = self.headers.get('LayerX-Key') + ':' + self.headers.get('Idempotency-Key', '')
                        previous = owner.state['requests'].get(key)
                        if previous is not None and previous != request:
                            status, result = 409, {'error': {'code': 'idempotency_conflict'}}
                        elif request != owner.source:
                            status, result = 409, {'error': {'code': 'source_mismatch'}}
                        else:
                            if previous is None:
                                owner.state['requests'][key] = request
                                owner.state['builds'] += 1
                            owner.state['source'] = dict(status='verified', source_digest=request['source_digest'],
                                                        environment_digest=hashlib.sha256(b'committed peer environment').hexdigest(),
                                                        pipeline='reproducible-builder')
                            owner.persist()
                            result = owner.document()
                    raw = evidence.canonical(result)
                    self.send_response(status)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(raw)))
                    self.end_headers()
                    self.wfile.write(raw)
                except Exception as error:
                    owner.errors.append(error)
                    self.send_error(500)

            do_GET = handle_request
            do_POST = handle_request

        self.server = ThreadingHTTPServer(('127.0.0.1', self.port), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(self.tls['server_certificate'], self.tls['server_key'])
        context.load_verify_locations(self.tls['client_ca'])
        context.verify_mode = ssl.CERT_REQUIRED
        self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        self.port = self.server.server_port
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        self.thread.start()
        self.url = 'https://localhost:' + str(self.port)

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(5)
        assert not self.thread.is_alive()

    def reopen(self):
        self.stop()
        self.start()

    def client(self, token=None, principal='principal-A'):
        context = ssl.create_default_context(cafile=self.tls['client_ca'])
        context.load_cert_chain(self.tls['client_certificate'], self.tls['client_key'])
        return REGISTRY.RegistryClient(self.url, token or Path(self.tls['publication_token']).read_text().strip(), principal, context)


@pytest.fixture
def peers(tmp_path):
    tmp_path.chmod(0o700)
    native = NativePeer(tmp_path / 'native.json')
    independent = NativePeer(tmp_path / 'independent.json')
    case = install_program(native)
    install_program(independent)
    tls = tls_material(tmp_path / 'tls')
    registry = RegistryPeer(tmp_path / 'registry.json', native, case, tls)
    try:
        yield native, independent, registry, case, tls
    finally:
        registry.stop()
        native.stop()
        independent.stop()
        assert not registry.errors and not native.failures and not independent.failures


def material_files(tls):
    return {key: value for key, value in tls.items() if key in
            {'request_token', 'publication_token', 'client_ca', 'client_certificate', 'client_key'}}


def public_evidence():
    dns = {host: ['192.0.2.' + str(index)] for index, host in enumerate(BOOTSTRAP.PUBLIC_HOSTS, 1)}
    record = RECORDED.corpus()[0]['receiptJson']
    observations = {host: {'chain_id': '0x7d', 'block_number': record['height'],
                           'block_hash': '0x' + merged.evm.keccak256(record['txHash'].encode()).hex()} for host in dns}
    return {'dns': {'before': dns, 'after': deepcopy(dns)}, 'node_endpoints': BOOTSTRAP.public_endpoints(),
            'gateway_endpoint': 'https://gateway.paxeer.network/', 'node_observations': observations}


def bootstrap_for(directory, peers):
    native, _, registry, case, tls = peers
    directory.mkdir(mode=0o700)
    files = material_files(tls)
    generation = hashlib.sha256(evidence.canonical(BOOTSTRAP.verify_material(files))).hexdigest()
    client = REGISTRY.NativeRegistryDiscovery(native.node(), registry.client())
    return BOOTSTRAP.NativeBootstrap(directory, REVISION, IMAGE, generation, client, native.client(), case, files)


@pytest.mark.parametrize('abi', [1, 2, 3, 4])
def test_f48_native_registry_discovery_and_reopen(peers, abi):
    native, _, registry, _, _ = peers
    case = install_program(native, abi)
    registry.case = case
    discovery = REGISTRY.NativeRegistryDiscovery(native.node(), registry.client())
    before = discovery.read(case)
    registry.reopen()
    native.reopen()
    reopened = REGISTRY.NativeRegistryDiscovery(native.node(), registry.client()).read(case)
    assert before == reopened
    assert before['head'][:6] == b'\0\1' + bytes(4)
    assert {row[2]['method'] for row in native.requests if row[2]} >= {'eth_call', 'pax_sequencerReceipt', 'eth_getBlockByNumber'}
    assert registry.requests[0][1] == '/v1/programs/registry/' + case['program_id']


@pytest.mark.parametrize('field,value', [('code_hash', 'cc' * 32), ('abi', 9), ('version', 2), ('interface', 'ff')])
def test_f48_native_registry_refuses_interface_substitution(peers, field, value):
    native, _, registry, case, _ = peers
    changed = dict(case, **{field: value})
    with pytest.raises(RuntimeError, match='mismatch'):
        REGISTRY.NativeRegistryDiscovery(native.node(), registry.client()).read(changed)
    assert registry.state['builds'] == 0


def test_f48_registry_publication_authentication_idempotency_and_restart(peers):
    native, _, registry, case, tls = peers
    client = registry.client()
    assert client.publish(case['program_id'], registry.source, 'one', authenticated=False)[0] == 401
    assert registry.client(Path(tls['request_token']).read_text().strip()).publish(case['program_id'], registry.source, 'one')[0] == 401
    assert registry.client(principal='foreign').publish(case['program_id'], registry.source, 'one')[0] == 403
    mismatch = dict(registry.source, source_digest='ef' * 32)
    assert client.publish(case['program_id'], mismatch, 'mismatch')[0] == 409
    assert client.publish(case['program_id'], registry.source, 'one')[0] == 200
    assert registry.state['builds'] == 1
    retained = registry.path.read_bytes()
    registry.reopen()
    assert registry.client().publish(case['program_id'], registry.source, 'one')[0] == 200
    assert registry.path.read_bytes() == retained
    assert registry.client().publish(case['program_id'], mismatch, 'one')[1]['error']['code'] == 'idempotency_conflict'
    assert registry.path.read_bytes() == retained
    assert registry.client(principal='principal-B').publish(case['program_id'], registry.source, 'one')[0] == 200
    assert registry.state['builds'] == 2
    untrusted = REGISTRY.RegistryClient(registry.url, client.token, client.principal, ssl.create_default_context())
    with pytest.raises(ssl.SSLError):
        untrusted.read(case['program_id'])
    no_identity = ssl.create_default_context(cafile=tls['client_ca'])
    with pytest.raises((ssl.SSLError, OSError)):
        REGISTRY.RegistryClient(registry.url, client.token, client.principal, no_identity).read(case['program_id'])
    assert REGISTRY.NativeRegistryDiscovery(native.node(), registry.client()).read(case)['registry']['versions'][-1]['source']['status'] == 'verified'


def test_f50_direct_bootstrap_stages_consumers_and_recovery(peers, tmp_path):
    bootstrap = bootstrap_for(tmp_path / 'stages', peers)
    native, _, registry, _, _ = peers
    for stage in BOOTSTRAP.STAGES:
        bootstrap.stage(stage, registry.source, 'bootstrap', public_evidence())
    retained = {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()}
    registry.reopen()
    native.reopen()
    reopened = BOOTSTRAP.NativeBootstrap(bootstrap.directory, REVISION, IMAGE, bootstrap.generation,
        REGISTRY.NativeRegistryDiscovery(native.node(), registry.client()), native.client(), bootstrap.case, bootstrap.material_files)
    for stage in BOOTSTRAP.STAGES:
        reopened.stage(stage, registry.source, 'bootstrap', public_evidence())
    assert {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()} == retained
    assert registry.state['builds'] == 1
    assert any(role == 'gateway' and value and value['method'] == 'px_getCapabilities' for role, _, value in native.requests)
    assert BOOTSTRAP.validate_stage(bootstrap.directory, BOOTSTRAP.STAGES[-1], REVISION, IMAGE, bootstrap.generation)


def test_f50_activation_refusals_retain_state_and_identity(peers, tmp_path):
    bootstrap = bootstrap_for(tmp_path / 'stages', peers)
    native, _, registry, _, tls = peers
    with pytest.raises(BOOTSTRAP.Refused, match='missing=registry-bootstrap'):
        bootstrap.stage('paxd-gateway-activation')
    assert list(bootstrap.directory.iterdir()) == []
    bootstrap.stage('material')
    bootstrap.stage('registry-bootstrap', registry.source, 'activate')
    retained = {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()}
    native.data['chain_id'] = '0x7c'
    with pytest.raises(ValueError, match='chain 125'):
        bootstrap.stage('paxd-gateway-activation')
    native.data['chain_id'] = '0x7d'
    assert {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()} == retained
    original = Path(tls['publication_token']).read_bytes()
    put(tls['publication_token'], Path(tls['request_token']).read_bytes())
    with pytest.raises(BOOTSTRAP.Refused, match='distinct retained'):
        bootstrap.stage('paxd-gateway-activation')
    put(tls['publication_token'], original)
    for revision, image, generation in [('00' * 20, IMAGE, bootstrap.generation), (REVISION, 'paxd@sha256:' + '00' * 32, bootstrap.generation),
                                        (REVISION, IMAGE, '00' * 32)]:
        with pytest.raises(BOOTSTRAP.Refused, match='mismatch'):
            BOOTSTRAP.validate_stage(bootstrap.directory, 'registry-bootstrap', revision, image, generation)
    stage = bootstrap.directory / 'registry-bootstrap.json'
    original_record = stage.read_bytes()
    changed = json.loads(original_record)
    changed['outcome'] = 'failed'
    put(stage, changed)
    with pytest.raises(BOOTSTRAP.Refused, match='missing=registry-bootstrap'):
        bootstrap.stage('paxd-gateway-activation')
    put(stage, original_record)
    bootstrap.stage('paxd-gateway-activation')
    assert registry.state['builds'] == 1


def test_f51_fourteen_hosts_aggregate_root_and_runtime_dns():
    document = public_evidence()
    assert BOOTSTRAP.PUBLIC_HOSTS == tuple(f'api{n}.mainnet-beta.paxeer.network' for n in range(1, 15)) + ('api-mainnet-beta.paxeer.network',)
    assert len(document['node_endpoints']) == 15
    assert BOOTSTRAP.case_public_interface({'doc': document})
    for kind in ('missing', 'retired', 'changed', 'route', 'gateway', 'observation', 'foreign-chain'):
        changed = deepcopy(document)
        if kind == 'missing':
            del changed['dns']['before'][BOOTSTRAP.PUBLIC_HOSTS[0]]
        elif kind == 'retired':
            changed['dns']['after']['api15.mainnet-beta.paxeer.network'] = ['192.0.2.15']
        elif kind == 'changed':
            changed['dns']['after'][BOOTSTRAP.PUBLIC_HOSTS[0]] = ['192.0.2.99']
        elif kind == 'route':
            changed['node_endpoints'][BOOTSTRAP.PUBLIC_HOSTS[0]] += 'rpc'
        elif kind == 'gateway':
            changed['gateway_endpoint'] = BOOTSTRAP.PUBLIC + '/'
        elif kind == 'observation':
            del changed['node_observations'][BOOTSTRAP.PUBLIC_HOSTS[0]]
        else:
            changed['node_observations'][BOOTSTRAP.PUBLIC_HOSTS[0]]['chain_id'] = '0x1'
        with pytest.raises(BOOTSTRAP.Refused):
            BOOTSTRAP.case_public_interface({'doc': changed})


class ArchivePeer:
    """Durable finalized manifest/chunk RPC store using the real EVM ABI codec."""
    def __init__(self, path, archive, commitment):
        self.path = path
        self.port = 0
        self.mode = None
        self.errors, self.requests = [], []
        self.config = {'chain_id': 1, 'genesis_hash': '0x' + '01' * 32, 'contract': '0x' + '02' * 20,
                       'code_hash': merged.evm.keccak256(b'\x60\x00').hex(), 'publisher': '0x' + '03' * 20}
        put(path, {'archive': archive.hex(), 'commitment': commitment})
        self.start()

    def response(self, method, params):
        self.requests.append((method, params))
        if method == 'eth_chainId':
            return '0x2' if self.mode == 'chain' else '0x1'
        if method == 'eth_getBlockByNumber':
            if params[0] == '0x0':
                return {'number': '0x0', 'hash': self.config['genesis_hash']}
            return {'number': '0x20', 'hash': '0x' + ('99' if self.mode == 'reorg' and params[0] != 'finalized' else '04') * 32}
        assert params[-1] == '0x20', 'all archive identity and payload reads use one finalized block'
        if method == 'eth_getCode':
            assert params[0] == self.config['contract']
            return '0x6001' if self.mode == 'code' else '0x6000'
        assert method == 'eth_call' and params[0]['to'] == self.config['contract']
        data = bytes.fromhex(params[0]['data'][2:])
        selectors = {merged.evm.keccak256(signature.encode())[:4]: signature for signature in
                     ('publisher()', 'manifest(bytes32)', 'chunk(bytes32,uint32)')}
        signature = selectors[data[:4]]
        if signature == 'publisher()':
            values, types = [bytes.fromhex(('09' if self.mode == 'publisher' else '03') * 20)], ['address']
        else:
            assert data[4:36].hex() == self.state['commitment']
            archive = bytes.fromhex(self.state['archive'])
            if signature == 'manifest(bytes32)':
                values = [len(archive), 1, hashlib.sha256(archive).digest(), self.mode != 'unfinalized']
                if self.mode == 'length':
                    values[0] += 1
                if self.mode == 'chunks':
                    values[1] = 65_537
                types = ['uint64', 'uint32', 'bytes32', 'bool']
            else:
                assert int.from_bytes(data[36:], 'big') == 0
                if self.mode == 'corrupt':
                    archive = archive[:-1] + bytes([archive[-1] ^ 1])
                values, types = [archive], ['bytes']
        return '0x' + merged.evm.encode_call('archiveResult', types, values)[4:].hex()

    def start(self):
        self.state = json.loads(self.path.read_bytes())
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    assert self.path == '/' and self.headers.get_content_type() == 'application/json'
                    assert self.headers.get('Authorization') == RECORDED.AUTH
                    call = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                    assert set(call) == {'jsonrpc', 'id', 'method', 'params'} and call['jsonrpc'] == '2.0'
                    result = owner.response(call['method'], call['params'])
                    raw = evidence.canonical({'jsonrpc': '2.0', 'id': call['id'], 'result': result})
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(raw)))
                    self.end_headers()
                    self.wfile.write(raw)
                except Exception as error:
                    owner.errors.append(error)
                    self.send_error(500)

        self.server = ThreadingHTTPServer(('127.0.0.1', self.port), Handler)
        self.port = self.server.server_port
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        self.thread.start()
        self.url = 'http://127.0.0.1:' + str(self.port)

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(5)
        assert not self.thread.is_alive()

    def reopen(self):
        self.stop()
        self.start()

    def reader(self, pins):
        node = merged.CHAIN.Node(self.url, pins, {'Authorization': RECORDED.AUTH})
        return ARCHIVE.EthereumArchiveReader(node, self.config)


@contextmanager
def archive_environment(peers, directory):
    native, independent, _, _, _ = peers
    directory.mkdir(mode=0o700)
    contract = ARCHIVE.CommittedArchiveContract([native.node(), independent.node()], GENESIS, directory)
    height = int(native.program_record['receiptJson']['height'], 16)
    commitment, produced = contract.acquire(height)
    first = ArchivePeer(directory / 'archive-peer-1.json', produced, commitment)
    second = ArchivePeer(directory / 'archive-peer-2.json', produced, commitment)
    try:
        yield contract, height, commitment, produced, first, second
    finally:
        first.stop()
        second.stop()
        assert not first.errors and not second.errors


def test_f49_committed_v4_producer_transport_consumers_and_reopen(peers, tmp_path):
    with archive_environment(peers, tmp_path / 'archives') as (contract, height, commitment, produced, first, second):
        readers = [first.reader(peers[0].node().receipt_keys), second.reader(peers[0].node().receipt_keys)]
        assert contract.readback(height, readers) == (commitment, produced)
        assert produced[:22] == b'LXMIRROR\0\4' + (125).to_bytes(4, 'big') + height.to_bytes(8, 'big')
        signed = bytes.fromhex(peers[0].program_record['signedReceipt'][2:])
        assert produced.endswith(len(signed).to_bytes(4, 'big') + signed)
        retained = {path.name: path.read_bytes() for path in contract.directory.iterdir()}
        first.reopen()
        second.reopen()
        peers[0].reopen()
        peers[1].reopen()
        reopened = ARCHIVE.CommittedArchiveContract([peers[0].node(), peers[1].node()], GENESIS, contract.directory)
        assert reopened.readback(height, [first.reader(peers[0].node().receipt_keys), second.reader(peers[0].node().receipt_keys)]) == (commitment, produced)
        assert {path.name: path.read_bytes() for path in contract.directory.iterdir()} == retained
        # External archive retrieval and commitment verification need no live native listener.
        pins = peers[0].node().receipt_keys
        peers[0].stop()
        peers[1].stop()
        assert first.reader(pins).retrieve(commitment) == produced
        peers[0].start()
        peers[1].start()
        assert all(params[-1] == '0x20' for method, params in first.requests if method in ('eth_call', 'eth_getCode'))
        assert {method for method, _ in first.requests} >= {'eth_chainId', 'eth_getBlockByNumber', 'eth_getCode', 'eth_call'}


@pytest.mark.parametrize('mode', ['chain', 'code', 'publisher', 'unfinalized', 'length', 'chunks', 'corrupt', 'reorg'])
def test_f49_mirror_corruption_identity_and_finality_refused(peers, tmp_path, mode):
    with archive_environment(peers, tmp_path / 'archives') as (contract, height, _, _, first, second):
        first.mode = mode
        before = {path.name: path.read_bytes() for path in contract.directory.iterdir()}
        with pytest.raises(ARCHIVE.Failure):
            contract.readback(height, [first.reader(peers[0].node().receipt_keys), second.reader(peers[0].node().receipt_keys)])
        assert {path.name: path.read_bytes() for path in contract.directory.iterdir()} == before
        first.mode = None
        assert contract.readback(height, [first.reader(peers[0].node().receipt_keys)])


@pytest.mark.parametrize('mode', ['divergent', 'signature', 'uncommitted', 'wrong-block', 'head-behind', 'wrong-genesis'])
def test_f49_independent_native_acquisition_refusals(peers, tmp_path, mode):
    native, independent, _, _, _ = peers
    directory = tmp_path / 'archive-refusal'
    directory.mkdir(mode=0o700)
    height = int(native.program_record['receiptJson']['height'], 16)
    reference = independent.program_record['receiptJson']['txHash']
    original = deepcopy(independent.data)
    old_damage = independent.response_damage

    def damage(role, method, answer):
        if role != 'node':
            return
        if method == 'pax_sequencerReceipt':
            if mode == 'signature':
                data = answer['result']['receipt']
                answer['result']['receipt'] = data[:-2] + ('01' if data[-2:] != '01' else '02')
            elif mode == 'uncommitted':
                answer['result']['rung'] = 'receipted'
            elif mode == 'wrong-block':
                answer['result']['blockHash'] = GENESIS
        if method == 'eth_getBlockByNumber':
            if mode == 'head-behind':
                answer['result']['number'] = '0x1'
            elif mode == 'wrong-genesis':
                answer['result']['hash'] = '0x' + 'ee' * 32
    if mode == 'divergent':
        independent.data['blocks'][hex(height)]['stateRoot'] = '0x' + 'cc' * 32
    else:
        independent.response_damage = damage
    contract = ARCHIVE.CommittedArchiveContract([native.node(), independent.node()], GENESIS, directory)
    try:
        with pytest.raises((ARCHIVE.Failure, ValueError, SequencerReceiptVerificationError)):
            contract.acquire(height)
        assert list(directory.iterdir()) == []
    finally:
        independent.data = original
        independent.response_damage = old_damage
    assert contract.acquire(height)
    assert reference in independent.records


class OwnerNetworkPeer(OWNER.LoopbackRpcServer):
    """Native owner transaction peer with the node's governed key query contract."""
    def request(self, method, params):
        if method == 'px_getReceiptKeys':
            assert params == []
            return {'keys': [deepcopy(RECORDED.corpus()[0]['pinnedKey'])]}
        if method == 'eth_getBlockByNumber' and params == ['0x0', False]:
            return {'number': '0x0', 'hash': GENESIS, 'transactions': []}
        return super().request(method, params)


@pytest.fixture(scope='module')
def owner_environment(tmp_path_factory):
    # Assemble original owner/guardian material through the real merged
    # producer. This peer supplies the public precompile ABI and stores state;
    # the qualifier never provisions a standalone receipt authority.
    directory = tmp_path_factory.mktemp('real-owner-810')
    directory.chmod(0o700)
    server = OwnerNetworkPeer(directory)
    try:
        public = OWNER.ed25519.Ed25519PrivateKey.from_private_bytes(bytes([13]) * 32).public_key().public_bytes(
            OWNER.Encoding.Raw, OWNER.PublicFormat.Raw).hex()
        server.genesis_main(public)
        funder, issuer = OWNER.bind(server, bytes([11]) * 32, 11)
        salt = bytes([41]) * 32
        asset = hashlib.sha256(b'LX:ASSET:v1' + issuer + salt).hexdigest()
        evm = OWNER.evm
        server.rpc.send(11, 125, OWNER.LEDGER, evm.calldata('register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)',
            '0x' + asset, salt, 'HUM', 'Human native', 0, 1000000, 1, b''))
        server.rpc.send(11, 125, OWNER.LEDGER, evm.calldata('openAccount(bytes32)', '0x' + asset))
        source = OWNER.account_id(funder, asset)
        server.rpc.send(11, 125, OWNER.LEDGER, evm.calldata('mint(bytes32,bytes32,uint128)', '0x' + asset, '0x' + source, 10000))
        receiver, _ = OWNER.bind(server, bytes([12]) * 32, 12)
        server.rpc.send(12, 125, OWNER.LEDGER, evm.calldata('openAccount(bytes32)', '0x' + asset))
        destination = OWNER.account_id(receiver, asset)
        work = directory / 'work'
        inputs = work / 'human-evidence-input'
        inputs.mkdir(mode=0o700, parents=True)
        secrets = directory / 'secrets'
        secrets.mkdir(mode=0o700)
        for index, role in enumerate(OWNER.owner_native.GUARDIAN_ROLES, 1):
            OWNER.guardians.enroll(work, secrets, role, bytes([index]).hex() * 32, 1)
        OWNER.guardians.assemble(work)
        recovery = OWNER.provision.protected_json(inputs / 'recovery-policy.json')
        principal = 'did:layerx:' + public
        OWNER.document(work / 'human-owner-result.json', dict(principal=principal, did=principal,
            recovery_root=recovery['root'], recovery_threshold=recovery['threshold'], recovery_delay_seconds=recovery['delay_seconds']))
        put(inputs / 'owner.seed', bytes([13]) * 32)
        put(inputs / 'owner.evm', ('%064x' % 13).encode())
        put(inputs / 'funder.evm', ('%064x' % 11).encode())
        config = dict(rpc_url=server.url, rpc_ca_file=str(server.ca), network_id=125, chain_id=125,
            confirmations=1, owner_seed_file=str(inputs / 'owner.seed'), evm_key_file=str(inputs / 'owner.evm'),
            asset_id=asset, funding_key_file=str(inputs / 'funder.evm'), funding_source=source, funding_amount=100)
        OWNER.document(inputs / 'owner-native.json', config)
        OWNER.owner_native.produce(work)
        registration = OWNER.provision.owner_registration(work)
        OWNER.document(work / 'identity/source-binding.json', dict(tenant='beta', principal=principal))
        OWNER.document(inputs / 'treasury.json', dict(account=source))
        OWNER.document(inputs / 'sequencer.json', dict(account=destination))
        OWNER.document(inputs / 'account-head-result.json', dict(consumed=0))
        registry = OWNER.document(directory / 'module-registry.json', dict(schema_version=2, network_id=125,
            assets=[dict(asset=asset)], modules=[dict(module=1, ordinals=[1, 4, 5, 10]),
                                              dict(module=3, ordinals=[1, 6]), dict(module=7, ordinals=[1, 3])]))
        recorded = json.loads((ROOT / 'services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json').read_text())
        receipt = bytes.fromhex(recorded['receipt_hex'])
        fields = OWNER.owner_native.receipt_fields(receipt, bytes.fromhex(recorded['sequencer_public_key_hex']), 'recorded naming receipt')
        journal = work / 'registry-journal'
        put(journal / (fields['receipt_digest'] + '.admission'), bytes.fromhex(recorded['header_hex']))
        put(journal / (fields['receipt_digest'] + '.deployment'), receipt)
        OWNER.document(work / 'naming-deployment-result.json', dict(state='deployed', receipt_digest=fields['receipt_digest'],
                                                                  activity_id=fields['activity_id']))
        OWNER.provision.assemble(work, registry, asset, journal)
        deployment = OWNER.document(directory / 'deployment.json', dict(network_id=125, chain_id=125))
        bundle = directory / 'bundle'
        bundle.mkdir(mode=0o700)
        OWNER.material.assemble_policy(work / 'human-evidence', deployment, registry, bundle / 'policy.json', 125, 125)
        policy = OWNER.provision.protected_json(bundle / 'policy.json')
        context = OWNER.provision.protected_json(inputs / 'native-context.json')
        client = OWNER.OwnerClient(context, server.ca.read_bytes(), 13, inputs / 'owner-native-run')
        yield SimpleNamespace(server=server, public=public, bundle=bundle, policy=policy, context=context,
                              inputs=inputs, client=client, destination=destination)
    finally:
        server.close()


def owner_node(owner, pins):
    return merged.CHAIN.Node(owner.server.url, pins, context=ssl.create_default_context(cafile=str(owner.server.ca)))


def test_f52_owner_authorization_native_trust_private_journals_and_restart(owner_environment, peers, tmp_path):
    owner = owner_environment
    node = owner_node(owner, peers[0].node().receipt_keys)
    genesis = genesis_for(node)
    root = tmp_path / 'native-owner'
    before = {path.name: path.read_bytes() for path in (owner.bundle / 'journal').iterdir()}
    installed = HUMAN.consume_bundle(root, owner.bundle, 125, 125, node, genesis, GENESIS)
    assert installed['owner'] == owner.public
    assert installed['trust']['receipt_keys'] == node.receipt_keys
    assert installed['verified']['records'] == 2
    assert not (root / 'receipt-authority-replica-id').exists()
    assert {path.name: path.read_bytes() for path in (root / 'journal').iterdir()} == before
    retained = HUMAN.tree(root)
    owner.server.restart()
    peers[0].reopen()
    reopened = load('human_reopened', QUAL / 'human-owner-evidence.py')
    assert reopened.consume_bundle(root, owner.bundle, 125, 125, owner_node(owner, peers[0].node().receipt_keys), genesis, GENESIS) == installed
    assert HUMAN.tree(root) == retained
    assert all(path.stat().st_mode & 0o777 == 0o600 for path in root.rglob('*') if path.is_file())
    assert 'receipt-authority' not in (root / 'native-binding.json').read_text()
    assert any(row['method'] == 'eth_getTransactionReceipt' for row in owner.server.requests)
    assert any(row['method'] == 'eth_getBlockByNumber' for row in owner.server.requests)
    assert any(row['method'] == 'px_getReceiptKeys' for row in owner.server.requests)


@pytest.mark.parametrize('mode', ['genesis', 'key', 'range', 'chain', 'owner', 'journal', 'policy', 'permissions', 'symlink'])
def test_f52_owner_native_trust_and_material_refusals(owner_environment, peers, tmp_path, mode):
    owner = owner_environment
    node = owner_node(owner, peers[0].node().receipt_keys)
    genesis = genesis_for(node)
    root = tmp_path / 'retained-owner'
    HUMAN.consume_bundle(root, owner.bundle, 125, 125, node, genesis, GENESIS)
    retained = HUMAN.tree(root)
    bundle = HUMAN.private_copy(owner.bundle, tmp_path / 'bundle-copy')
    genesis_hash = GENESIS
    original_chain = peers[0].data['chain_id']
    if mode == 'genesis':
        genesis_hash = '0x' + 'ff' * 32
    elif mode == 'key':
        genesis['app_state']['sequencer']['receipt_keys'][0]['public_key'] = base64.b64encode(bytes([9]) * 32).decode()
    elif mode == 'range':
        genesis['app_state']['sequencer']['receipt_keys'][0]['last_height'] = 0
    elif mode == 'chain':
        node = peers[0].node()
        peers[0].data['chain_id'] = '0x7c'
    else:
        path = bundle / ('inputs/producer-records/merged-owner-evidence.json' if mode == 'owner' else
                         'policy.json' if mode == 'policy' else 'inputs/authority.json' if mode in ('permissions', 'symlink') else
                         'journal/' + next((bundle / 'journal').iterdir()).name)
        if mode == 'permissions':
            path.chmod(0o644)
        elif mode == 'symlink':
            raw = path.read_bytes()
            path.unlink()
            destination = put(tmp_path / 'linked-input', raw)
            path.symlink_to(destination)
        else:
            raw = bytearray(path.read_bytes())
            raw[-1] ^= 1
            HUMAN.rewrite(path, bytes(raw))
    try:
        with pytest.raises((ValueError, HUMAN.Case)):
            HUMAN.consume_bundle(root, bundle, 125, 125, node, genesis, genesis_hash)
        assert HUMAN.tree(root) == retained
    finally:
        peers[0].data['chain_id'] = original_chain


def test_f52_owner_purpose_authorization_and_unchanged_pending_intent(owner_environment):
    owner = owner_environment
    client = owner.client
    preset = owner.policy['purpose_catalog']['presets'][0]
    before = deepcopy(owner.server.accounts)
    with pytest.raises(ValueError, match='authorization'):
        client.pay(owner.policy, preset['id'], 'ee' * 32, 1, 'dd' * 32, 2**63)
    assert owner.server.accounts == before
    reference = 'da' * 32
    result = client.pay(owner.policy, preset['id'], owner.destination, 1, reference, 2**63)
    pending = owner.inputs / 'owner-native-run' / ('payment-' + reference + '.transaction.json')
    original = pending.read_bytes()
    owner.server.restart()
    from merged_owner import OwnerClient
    reopened = OwnerClient(owner.context, owner.server.ca.read_bytes(), 13, owner.inputs / 'owner-native-run')
    assert reopened.pay(owner.policy, preset['id'], owner.destination, 1, reference, 2**63) == result
    assert pending.read_bytes() == original
    after = deepcopy(owner.server.accounts)
    with pytest.raises(ValueError, match='intent binding'):
        reopened.pay(owner.policy, preset['id'], owner.destination, 2, reference, 2**63)
    assert owner.server.accounts == after


@pytest.mark.parametrize('damage', ['programs-unavailable', 'gateway-id', 'wrong-address', 'uncommitted-receipt'])
def test_f50_negative_gateway_activation_never_publishes_success(peers, tmp_path, damage):
    native, _, registry, _, _ = peers
    bootstrap = bootstrap_for(tmp_path / 'stages', peers)
    bootstrap.stage('material')
    bootstrap.stage('registry-bootstrap', registry.source, 'gate')
    before = {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()}

    def damage_reply(role, method, answer):
        if role == 'gateway' and method == 'px_getCapabilities':
            if damage == 'programs-unavailable':
                answer['result']['programs'] = False
            elif damage == 'gateway-id':
                answer['id'] += 1
            elif damage == 'wrong-address':
                answer['result']['addresses']['programs'] = '0x' + '00' * 18 + '1014'
        if role == 'node' and method == 'pax_sequencerReceipt' and damage == 'uncommitted-receipt':
            answer['result']['rung'] = 'receipted'
    native.response_damage = damage_reply
    try:
        with pytest.raises((ValueError, BOOTSTRAP.Refused)):
            bootstrap.stage('paxd-gateway-activation')
        assert {path.name: path.read_bytes() for path in bootstrap.directory.iterdir()} == before
    finally:
        native.response_damage = None
    bootstrap.stage('paxd-gateway-activation')


def test_f48_qualify_restart_uses_current_native_and_registry_consumers(peers):
    native, _, registry, case, _ = peers
    discovery = REGISTRY.NativeRegistryDiscovery(native.node(), registry.client())
    starts = []

    def reopen():
        starts.append('reopen')
        native.reopen()
        registry.reopen()

    observed = discovery.qualify(case, reopen)
    assert starts == ['reopen']
    assert observed['receipt']['transaction_hash'] == case['transaction_hash']
    assert observed['registry']['program_id'] == case['program_id']


class XWebRpcPeer:
    """Loopback native RPC peer consuming SDK-signed requests and retaining receipts."""
    def __init__(self, path):
        self.path, self.mode, self.errors = path, None, []
        self.requests = []
        self.expected_payload = b'https://origin.example/page'
        self.state = json.loads(path.read_bytes()) if path.exists() else {'raw': [], 'receipts': {}}
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    assert self.path == '/'
                    assert self.headers.get_content_type() == 'application/json'
                    request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                    assert set(request) == {'jsonrpc', 'id', 'method', 'params'}
                    assert request['jsonrpc'] == '2.0' and type(request['id']) is int
                    owner.requests.append(request)
                    answer = owner.request(request['method'], request['params'])
                    body = evidence.canonical(dict(jsonrpc='2.0', id=request['id'] + int(owner.mode == 'id'), result=answer))
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except Exception as error:
                    owner.errors.append(error)
                    self.send_error(500)

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.url = 'http://127.0.0.1:' + str(self.server.server_port)

    def request(self, method, params):
        if method == 'eth_chainId':
            assert params == []
            return '0x7c' if self.mode == 'chain' else '0x7d'
        if method == 'eth_getTransactionCount':
            assert params == [XWEB.evm.checksum(XWEB.evm.address_of(13)), 'pending']
            return hex(len(self.state['raw']))
        if method == 'eth_gasPrice':
            assert params == []
            return '0x7'
        if method == 'eth_estimateGas':
            assert params == [{'from': XWEB.evm.checksum(XWEB.evm.address_of(13)),
                               'to': XWEB.evm.checksum(XWEB.XWEB_PRECOMPILE),
                               'data': '0x' + merged.evm.encode_call('request', ['uint8', 'bytes', 'uint64'],
                                   [1, self.expected_payload, XWEB.CALLBACK_GAS]).hex(), 'value': '0xb'}]
            return '0x186a0'
        if method == 'eth_call':
            assert params == [{'to': XWEB.XWEB_PRECOMPILE,
                               'data': '0x' + merged.evm.encode_call('fee', [], []).hex()}, 'latest']
            return '0x' + (11).to_bytes(32, 'big').hex()
        if method == 'eth_sendRawTransaction':
            assert len(params) == 1
            raw = bytes.fromhex(params[0][2:])
            transaction, signature = merged.evm.decode_signed(raw)
            assert transaction.chain_id == 125 and transaction.nonce == len(self.state['raw'])
            assert transaction.to.hex() == XWEB.XWEB_PRECOMPILE[2:]
            assert transaction.value == 11 and transaction.gas_limit == 100000
            assert transaction.max_fee_per_gas == 7 and transaction.max_priority_fee_per_gas == 0
            assert transaction.recover_signer(signature) == XWEB.evm.checksum(XWEB.evm.address_of(13))
            assert transaction.data == merged.evm.encode_call('request', ['uint8', 'bytes', 'uint64'],
                [1, self.expected_payload, XWEB.CALLBACK_GAS])
            reference = '0x' + merged.evm.keccak256(raw).hex()
            self.state['raw'].append(params[0])
            request_id = len(self.state['raw'])
            self.state['receipts'][reference] = dict(transactionHash=reference, status='0x1', logs=[{
                'address': XWEB.XWEB_PRECOMPILE,
                'data': '0x' + merged.evm.encode_call('eventData', ['uint8', 'bytes', 'uint64', 'uint256', 'uint64'],
                    [1, self.expected_payload, XWEB.CALLBACK_GAS, 11, 121])[4:].hex(),
                'topics': ['0x' + merged.evm.keccak256(
                    b'XWebRequested(uint64,address,uint8,bytes,uint64,uint256,uint64)').hex(),
                    '0x' + request_id.to_bytes(32, 'big').hex(),
                    '0x' + XWEB.evm.address_of(13).rjust(32, b'\x00').hex()]}])
            put(self.path, self.state)
            return '0x' + 'ee' * 32 if self.mode == 'submission' else reference
        if method == 'eth_getTransactionReceipt':
            assert len(params) == 1 and params[0] in self.state['receipts']
            result = deepcopy(self.state['receipts'][params[0]])
            if self.mode == 'receipt':
                result['transactionHash'] = '0x' + 'ee' * 32
            if self.mode == 'reverted':
                result['status'] = '0x0'
            if self.mode == 'logs':
                result['logs'] = []
            return result
        raise AssertionError('unexpected native method: ' + method)

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        assert not self.errors


def test_xweb_maintained_sdk_producer_root_rpc_consumer_and_reopen(tmp_path):
    key = put(tmp_path / 'requester.key', ('%064x' % 13 + '\n').encode())
    store = tmp_path / 'xweb.json'
    peer = XWebRpcPeer(store)
    try:
        assert XWEB.evm.Rpc(peer.url).eth_call(XWEB.XWEB_PRECOMPILE, 'fee()(uint256)') == (11,)
        assert XWEB.produce(peer.url, key, peer.expected_payload.decode(), 11) == 1
        retained = store.read_bytes()
        assert len(peer.state['raw']) == 1
    finally:
        peer.close()
    reopened = XWebRpcPeer(store)
    try:
        assert store.read_bytes() == retained and len(reopened.state['raw']) == 1
        fixture = XWEB.XWebFixture.__new__(XWEB.XWebFixture)
        fixture.rpc, fixture.requester_key, fixture.fee = XWEB.evm.Rpc(reopened.url), key, 11
        assert fixture.request_payload(reopened.expected_payload) == 2
        assert fixture.refund_calldata(2) == '0x' + merged.evm.encode_call('refund', ['uint64'], [2]).hex()
        assert fixture.evm_key() == 13
        assert XWEB.evm.compressed_public_key(13).hex() == '03f28773c2d975288bc7d1d205c3748651b075fbc6610e58cddeeddf8f19405aa8'
        assert len(reopened.state['raw']) == 2 and key.stat().st_mode & 0o777 == 0o600
        assert {r['method'] for r in reopened.requests} >= {'eth_chainId', 'eth_getTransactionCount',
            'eth_gasPrice', 'eth_estimateGas', 'eth_sendRawTransaction', 'eth_getTransactionReceipt'}
    finally:
        reopened.close()


@pytest.mark.parametrize('mode', ['chain', 'id', 'submission', 'receipt', 'reverted', 'logs'])
def test_xweb_native_transport_and_execution_refusals(tmp_path, mode):
    key = put(tmp_path / 'requester.key', ('%064x' % 13).encode())
    peer = XWebRpcPeer(tmp_path / 'xweb.json')
    peer.mode = mode
    try:
        with pytest.raises((ValueError, RuntimeError)):
            XWEB.produce(peer.url, key, peer.expected_payload.decode(), 11)
        assert len(peer.state['raw']) == (0 if mode in ('chain', 'id') else 1)
    finally:
        peer.close()


@pytest.mark.parametrize('mode', ['public', 'symlink', 'hardlink', 'zero', 'range', 'encoding'])
def test_xweb_native_private_key_refusals(tmp_path, mode):
    key = put(tmp_path / 'requester.key', ('%064x' % 13).encode())
    if mode == 'public':
        key.chmod(0o644)
    elif mode == 'symlink':
        target = tmp_path / 'original.key'
        key.rename(target)
        key.symlink_to(target)
    elif mode == 'hardlink':
        os.link(key, tmp_path / 'linked.key')
    else:
        put(key, ('0' * 64 if mode == 'zero' else '%064x' % XWEB.evm.N if mode == 'range' else 'not-a-key').encode())
    with pytest.raises((ValueError, OSError)):
        XWEB.evm.read_key(key)
