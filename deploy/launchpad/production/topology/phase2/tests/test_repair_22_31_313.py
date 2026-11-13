from __future__ import annotations

import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
import sys
import threading

import pytest

TOPOLOGY = Path(__file__).resolve().parents[2]
SOURCE_ROOT = TOPOLOGY.parents[3]
ADAPTER = SOURCE_ROOT / 'tools/launchpad/production/integration/adapters.py'
sys.path.insert(0, str(TOPOLOGY))

from phase2 import v1
from phase2.attestations import regular_bytes
from phase2.operator import Journal
from phase2.providers import DOCUMENT_ENV, connected_plan, validate_connected_plan


def actual_adapter():
    spec = spec_from_file_location('repair_22_31_313_runtime', ADAPTER)
    module = module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def topology_v2():
    config = v1.load(TOPOLOGY / 'fixtures/synthetic-config.json')
    config['schemaVersion'] = 2
    config['workloads'] = {}
    config['runtimeDocuments'] = {}
    for role in v1.RUNTIME_ROLES:
        values = v1.SERVICES[role] if role in v1.SERVICES else (3, 6, 2, 4, 1)
        config['workloads'][role] = dict(zip(
            ('replicas', 'maxReplicas', 'cpu', 'memoryGiB', 'poolConnections'), values))
        config['images'][role] = (
            'example.invalid/kindle/' + role + '@sha256:' + v1.digest(role))
        config['secretRefs']['runtime'][role] = 'fixture-' + role + '-runtime'
        config['runtimeDocuments'][role] = {
            'budgetSHA256': v1.digest(['budget', role]),
            'expectedSchemaSHA256': v1.digest(['readiness', role]),
            'poolId': 'pool-' + role,
        }
    return v1.validate(config)


@pytest.fixture
def connected_inputs(tmp_path):
    config = topology_v2()
    fixture = v1.load(Path(__file__).with_name('fixtures') / 'repair_22_31_313_documents.json')
    grants = v1.load(SOURCE_ROOT / 'migrations/grants.json')
    manifests = {manifest['runtime_role']: manifest for manifest in grants['roles']}
    documents = {}
    for role, bodies in fixture['roles'].items():
        readiness = bodies['readiness']
        readiness['privileges'] = manifests[readiness['runtime_role']]['grants']
        documents[role] = {}
        for kind, body in dict(bodies, budget=fixture['budget']).items():
            path = tmp_path / (role + '-' + kind + '.json')
            raw = v1.canonical(body)
            path.write_bytes(raw)
            documents[role][kind] = {
                'path': str(path), 'sha256': hashlib.sha256(raw).hexdigest()}
        config['runtimeDocuments'][role]['budgetSHA256'] = documents[role]['budget']['sha256']
        config['runtimeDocuments'][role]['expectedSchemaSHA256'] = documents[role]['readiness']['sha256']
        config['runtimeDocuments'][role]['poolId'] = 'interactive'
    chain = tmp_path / 'chain.json'
    raw = v1.canonical(fixture['chain'])
    chain.write_bytes(raw)
    return (config, SOURCE_ROOT, hashlib.sha256(regular_bytes(ADAPTER)).hexdigest(),
            documents, chain, hashlib.sha256(raw).hexdigest())


def test_topology_adapter_path_preserves_digest_verification(tmp_path):
    config = topology_v2()
    observed = hashlib.sha256(regular_bytes(ADAPTER)).hexdigest()
    wrong_digest = ('0' if observed[0] != '0' else '1') + observed[1:]
    with pytest.raises(v1.Invalid, match='integrated runtime adapter digest mismatch'):
        connected_plan(config, SOURCE_ROOT, wrong_digest, {},
                       tmp_path / 'unread-chain.json', '0' * 64)


def test_topology_adapter_path_real_runtime_contract_all_twelve_roles():
    adapters = actual_adapter()
    missing = [name for name in ('ROLES', 'DOCUMENT_ENV', 'document_binding')
               if not hasattr(adapters, name)]
    assert not missing, (
        'Canonical runtime adapter lacks connected topology exports: ' + ', '.join(missing))
    assert len(adapters.ROLES) == 12
    assert set(adapters.ROLES) == set(v1.RUNTIME_ROLES)
    assert {role: binding[2] for role, binding in adapters.ROLES.items()} == v1.RUNTIME_ROLES
    assert adapters.DOCUMENT_ENV == DOCUMENT_ENV
    assert callable(adapters.document_binding)


def test_topology_adapter_path_actual_client_transports_compiler_output():
    adapters = actual_adapter()
    config = topology_v2()
    produced = v1.compile_plan(config)
    payload = v1.canonical(produced)
    requests = []

    class CompilerOutput(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers['Content-Type']))
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), CompilerOutput)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        client = adapters.LocalHTTP('http://127.0.0.1:' + str(port), {port})
        status, received = client.request('GET', '/topology')
        assert status == 200
        assert received == payload
        consumed = v1.decode(received)
        assert v1.validate_rendered(consumed, config) is True
        assert consumed == produced
        assert requests == [('GET', '/topology', 'application/json')]
        deployments = {item['metadata']['name']: item for item in consumed['resources']['items']
                       if item['kind'] == 'Deployment'}
        assert set(deployments) == set(v1.RUNTIME_ROLES)
        assert consumed['capacity'] == v1.capacity(config)
        assert consumed['capacity']['classification'] == 'arithmetic_plan_not_measured_capacity'
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
    assert not thread.is_alive()


def test_topology_adapter_path_connected_render_capacity_and_recovery(connected_inputs, tmp_path):
    config, _, _, documents, _, chain_sha = connected_inputs
    adapters = actual_adapter()
    for role_documents in documents.values():
        for reference in role_documents.values():
            body, sha = adapters.document_binding(reference['path'])
            assert body == v1.load(reference['path'])
            assert sha == reference['sha256']
    produced = connected_plan(*connected_inputs)
    assert validate_connected_plan(produced, config) is True
    assert produced['runtimeOwners'] == v1.RUNTIME_ROLES
    assert len(produced['runtimeOwners']) == 12
    assert produced['chainManifestDigest'] == chain_sha
    assert produced['chainId'] == 125
    assert produced['capacity'] == v1.capacity(config)
    assert produced['capacity']['classification'] == 'arithmetic_plan_not_measured_capacity'
    expected_bindings = {
        role: {kind: reference['sha256'] for kind, reference in role_documents.items()}
        for role, role_documents in documents.items()}
    assert produced['documentBindings'] == expected_bindings
    deployments = {item['metadata']['name']: item for item in produced['resources']['items']
                   if item['kind'] == 'Deployment'}
    assert set(deployments) == set(v1.RUNTIME_ROLES)
    for role, deployment in deployments.items():
        pod = deployment['spec']['template']
        assert pod['metadata']['annotations']['kindle.io/runtime-role'] == v1.RUNTIME_ROLES[role]
        assert pod['metadata']['annotations']['kindle.io/runtime-documents-sha256'] == v1.digest(expected_bindings[role])
        env = {item['name']: item['value'] for item in pod['spec']['containers'][0]['env']
               if 'value' in item}
        for kind, sha in expected_bindings[role].items():
            path_env, sha_env = DOCUMENT_ENV[kind]
            assert env[path_env] == '/run/kindle/' + kind + '.json'
            assert env[sha_env] == sha

    payload = v1.canonical(produced)
    requests = []

    class ConnectedOutput(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers['Content-Type']))
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), ConnectedOutput)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        client = adapters.LocalHTTP('http://127.0.0.1:' + str(port), {port})
        status, received = client.request('GET', '/topology/connected')
        assert status == 200
        assert received == payload
        consumed = v1.decode(received)
        assert consumed == produced
        assert validate_connected_plan(consumed, config) is True
        assert requests == [('GET', '/topology/connected', 'application/json')]
        journal_path = tmp_path / 'topology.db'
        journal = Journal(journal_path)
        try:
            journal.propose(config, consumed, 'fixture-operator')
        finally:
            journal.close()
        reopened = Journal(journal_path)
        try:
            recovered = v1.decode(reopened.proposal(config['revision'])['plan'].encode())
            assert recovered == consumed
            assert validate_connected_plan(recovered, config) is True
            assert recovered['documentBindings'] == expected_bindings
            assert recovered['capacity'] == v1.capacity(config)
        finally:
            reopened.close()
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
    assert not thread.is_alive()


@pytest.mark.parametrize(('role', 'kind'), [
    ('api', 'budget'), ('sockets', 'readiness'), ('indexer', 'receipt'),
    ('charts', 'delivery'), ('gateway', 'identity'), ('api', 'accounting_read')])
def test_topology_adapter_path_changed_document_rejected(connected_inputs, role, kind):
    reference = connected_inputs[3][role][kind]
    path = Path(reference['path'])
    path.write_bytes(path.read_bytes() + b'\n')
    with pytest.raises(v1.Invalid, match='runtime document changed: ' + role + '/' + kind):
        connected_plan(*connected_inputs)


def test_topology_adapter_path_changed_chain_rejected(connected_inputs):
    chain = connected_inputs[4]
    chain.write_bytes(chain.read_bytes() + b'\n')
    with pytest.raises(v1.Invalid, match='chain manifest digest mismatch'):
        connected_plan(*connected_inputs)


@pytest.mark.parametrize('raw', [b'{', b'{"role":1,"role":2}', b'{"value":NaN}',
                                b'[]', b'null', b'"text"', b' ' * (2 * 1024 * 1024 + 1)])
def test_topology_adapter_path_invalid_document_rejected(tmp_path, raw):
    path = tmp_path / 'invalid.json'
    path.write_bytes(raw)
    with pytest.raises(ValueError):
        actual_adapter().document_binding(path)


def test_topology_adapter_path_nonregular_document_rejected(tmp_path):
    adapters = actual_adapter()
    path = tmp_path / 'document.json'
    path.write_bytes(b'{"role":"apid:rest"}')
    link = tmp_path / 'linked.json'
    link.symlink_to(path)
    for invalid in (link, tmp_path, tmp_path / 'missing.json'):
        with pytest.raises(ValueError, match='bounded regular artifact required'):
            adapters.document_binding(invalid)
