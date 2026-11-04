"""Binding contract repair: production Python RPC, file and Unix client paths.

Rust publisher/consumer agreement is checked against their committed sources.
The Python-only gate does not execute Rust enrolment or MCP; native lifecycle
and host configuration wiring are recorded in repair-22.31.332.kvx.
"""

import ast
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import re
import socket
import threading

import pytest


ROOT = Path(__file__).resolve().parents[4]
QUAL = ROOT / 'tools/qualification/paxeer-x'
PUBLISHER = ROOT / 'services/agent/crates/layerx-agentd/src/enrolment.rs'
CONSUMER = ROOT / 'services/agent/crates/layerx-mcp/src/binding.rs'
SHAPE = ROOT / 'tools/platform/cli/src/install/binding_shape.rs'
VECTORS = ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'


def production():
    spec = importlib.util.spec_from_file_location('binding_listener_repair',
                                                QUAL / 'mcp_binding_listener.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def recorded_key():
    corpus = json.loads(VECTORS.read_text())
    assert corpus['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    case = next(case for case in corpus['cases'] if case['id'] == 'V1-pax-transfer')
    return deepcopy(case['pinnedKey'])


def private_json(path, value):
    path.write_text(json.dumps(value) + '\n')
    path.chmod(0o600)
    return path


@contextmanager
def receipt_server(payload):
    """Serve the node's recorded key bytes using its JSON-RPC envelope and headers."""
    requests, errors = [], []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            try:
                assert self.path == '/'
                assert self.headers['Content-Type'] == 'application/json'
                assert self.headers['Accept'] == 'application/json'
                request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                assert request == {'jsonrpc': '2.0', 'id': 1,
                                   'method': 'px_getReceiptKeys', 'params': []}
                requests.append(request)
                data = json.dumps(payload).encode()
                self.send_response(200)
            except Exception as error:
                errors.append(error)
                data = b'{}'
                self.send_response(500)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)

    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    thread = threading.Thread(target=lambda: server.serve_forever(poll_interval=0.01), daemon=True)
    thread.start()
    try:
        yield f'http://127.0.0.1:{server.server_port}/', requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert not errors


def rpc_payload(keys=None):
    return {'jsonrpc': '2.0', 'id': 1, 'result': {'keys': [recorded_key()] if keys is None else keys}}


def configuration(endpoint):
    key = recorded_key()
    corpus = json.loads(VECTORS.read_text())
    case = next(case for case in corpus['cases'] if case['id'] == 'V1-pax-transfer')
    return {
        'LAYERX_AGENT_NODE_ENDPOINT': endpoint,
        'LAYERX_AGENT_MCP_WEB_ENDPOINT': 'http://127.0.0.1:8080/',
        'LAYERX_AGENT_MCP_WEB_NETWORK': 'eip155:' + str(corpus['chainId']),
        'LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX': '0',
        'LAYERX_AGENT_MCP_WEB_PAYER_ADDRESS': case['receiptJson']['from'],
        'LAYERX_AGENT_MCP_WEB_RECEIVER_ADDRESS': case['receiptJson']['to'],
        'LAYERX_AGENT_MCP_WEB_FIRST_BATCH_NUMBER': str(key['firstHeight']),
        'LAYERX_AGENT_MCP_WEB_LAST_BATCH_NUMBER': str(key['lastHeight']),
        'LAYERX_AGENT_MCP_WEB_TIMEOUT_MS': '5000',
        'LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS': '3',
        'LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD': str((1 << 128) - 1),
    }


def rust_array(source, name):
    match = re.search(r'const ' + name + r':.*?=\s*\[(.*?)\];', source, re.S)
    assert match, name
    return set(re.findall(r'"([a-z_]+)"', match.group(1)))


def rust_impl(source, name):
    return source.split('impl ' + name + ' {', 1)[1].split('\n}\n', 1)[0]


def nested_function(name):
    source = (QUAL / 'mcp_binding_listener.py').read_text()
    node = next(node for node in ast.walk(ast.parse(source))
                if isinstance(node, ast.FunctionDef) and node.name == name)
    return ast.get_source_segment(source, node)


def test_binding_core_coordinate_producer_consumer_contract():
    module = production()
    publisher, consumer, shape = PUBLISHER.read_text(), CONSUMER.read_text(), SHAPE.read_text()
    assert module.TOP_KEYS == rust_array(consumer, 'BINDING_KEYS') | {'listener'}
    assert 'pub core_height: u64,' in shape
    assert 'pub core_sequence:' not in shape
    assert 'core_height: enrolment.core_height,' in rust_impl(publisher, 'BindingPublisher')
    assert publisher.count('core_height: request.core_sequence,') == 2
    assert 'root.contains_key("core_sequence")' in consumer
    assert 'mcp.binding_retired_key' in consumer
    assert 'core_height: unsigned(root, "core_height")?' in consumer
    assert 'serde_json::to_value(document)' in publisher
    assert 'serde_json::from_value::<BindingDocument>' in publisher
    assert 'check_core_coordinate(document,' in nested_function('producer_schema')
    restart = nested_function('restart_reload_no_duplicate_session')
    assert 'self.stop(mcp)' in restart and 'self.stop(daemon)' in restart
    assert "self.boot(main, 'agentd-restart')" in restart
    assert 'current == published' in restart
    assert 'check_core_coordinate(json.loads(private_file(binding))' in restart
    assert "served_session('mcp-restart', stale)" in restart and 'exchange()' in restart


@pytest.mark.parametrize('height', [0, 1000, (1 << 53) + 1, (1 << 64) - 1])
def test_binding_core_coordinate_json_file_reopens(tmp_path, height):
    module = production()
    # This is the harness's coordinate check, not a substitute native binding document.
    path = private_json(tmp_path / 'coordinate.json', {'core_height': height})
    before = (path.stat().st_ino, path.read_bytes())
    assert module.check_core_coordinate(json.loads(module.private_file(path)), height) == height
    del module
    reopened = production()
    assert reopened.check_core_coordinate(json.loads(reopened.private_file(path)), height) == height
    assert (path.stat().st_ino, path.read_bytes()) == before


@pytest.mark.parametrize('value', ['1000', '0x3e8', 1000.0, True, None, -1, 1 << 64])
def test_binding_core_coordinate_refuses_wrong_numeric_encoding(value):
    module = production()
    with pytest.raises(module.GateFailure, match='core_height invalid unsigned integer'):
        module.check_core_coordinate({'core_height': value}, 1000)


def test_binding_core_coordinate_refuses_retired_missing_or_changed_height():
    module = production()
    for document, reason in [({'core_sequence': 1000}, 'retired'), ({}, 'required'),
                             ({'core_height': 1001}, 'differs')]:
        with pytest.raises(module.GateFailure, match=reason):
            module.check_core_coordinate(document, 1000)


def test_binding_web_authority_producer_consumer_contract():
    module = production()
    publisher, consumer = PUBLISHER.read_text(), CONSUMER.read_text()
    declaration = rust_impl(publisher, 'WebDeclaration')
    emitted = declaration.split('Ok(json!({', 1)[1]
    assert set(re.findall(r'^\s*"([a-z_]+)":', emitted, re.M)) == module.WEB_KEYS
    assert module.WEB_KEYS == rust_array(consumer, 'WEB_KEYS')
    assert '"sequencer_public_key":' not in declaration
    for field in ('keyId', 'publicKey', 'firstHeight', 'lastHeight'):
        assert '"' + field + '"' in declaration
        assert '"' + field + '"' in consumer
    assert '"firstHeight": key.first_height' in declaration
    assert '"lastHeight": key.last_height' in declaration
    assert 'self.approval_threshold.to_string()' in declaration
    assert 'network.strip_prefix("eip155:")' in declaration
    assert "!chain_id.starts_with('0')" in declaration
    assert 'chain_id.len() <= 64' in declaration
    assert 'self.authority.as_ref().ok_or(EnrolmentError::InvalidWeb(' in declaration
    assert 'key.public_key == self.receipt_public_key' in declaration
    assert re.search(r'\.map\(WebDeclaration::document\)\s*\.transpose\(\)\?', publisher)
    assert 'Vec<ReceiptKeyEntry>' in publisher
    authority = ' '.join(rust_impl(publisher, 'WebAuthority').split())
    for guard in ('receipt_key_index.is_empty()', 'receipt_key_index.len() > MAX_WEB_RECEIPT_KEYS',
                  'key.key_id == [0; 32]', 'key.public_key == [0; 32]',
                  'key.first_height > key.last_height', '!seen.insert(key.key_id)',
                  'sequencer_id == [0; 32]', 'first_batch_number > last_batch_number',
                  'key.key_id == sequencer_id', 'key.first_height <= first_batch_number',
                  'key.last_height >= last_batch_number'):
        assert guard in authority
    assert 'const MAX_WEB_RECEIPT_KEYS: usize = 64;' in publisher
    retained = rust_impl(publisher, 'BindingPublisher')
    assert 'existing != document' in retained and 'self.check_secrets(enrolment)?' in retained
    assert 'Some(&self.document(&enrolment)?)' in retained
    assert 'Err(EnrolmentError::AlreadyPublished(binding))' in retained


def test_binding_web_authority_preserves_landed_constructor_and_authority_guards():
    publisher = PUBLISHER.read_text()
    declaration = rust_impl(publisher, 'WebDeclaration')
    constructor = declaration.split('pub fn new(', 1)[1].split(') -> Result<Self,', 1)[0]
    assert re.findall(r'^\s*([a-z_]+):', constructor, re.M) == [
        'endpoint', 'network', 'payer_address', 'receiver_address', 'receipt_key_index',
        'selected_key', 'timeout_ms', 'pending_attempts', 'approval_threshold',
    ]
    assert 'network != "eip155:125"' in declaration
    assert 'selected_key >= receipt_key_index.len()' in declaration
    assert 'let key = &receipt_key_index[selected_key];' in declaration
    assert 'let authority = WebAuthority::new(' in declaration
    assert 'authority: Some(authority)' in declaration
    authority = rust_impl(publisher, 'WebAuthority')
    assert 'payer_address == [0; 20] || receiver_address == [0; 20]' in authority
    narrowing = declaration.split('pub fn with_authority', 1)[1].split('fn document', 1)[0]
    for guard in ('authority.payer_address != current.payer_address',
                  'authority.receiver_address != current.receiver_address',
                  'authority.receipt_key_index != current.receipt_key_index',
                  'authority.sequencer_id != current.sequencer_id',
                  'authority.first_batch_number < current.first_batch_number',
                  'authority.last_batch_number > current.last_batch_number'):
        assert guard in narrowing
    assert all(marker not in publisher for marker in ('<<<<<<<', '=======', '>>>>>>>'))


def test_binding_web_authority_real_rpc_reference_and_reopen(tmp_path):
    module = production()
    with receipt_server(rpc_payload()) as (endpoint, requests):
        expected = module.web_reference(configuration(endpoint))
        key = recorded_key()
        assert expected['receipt_key_index'] == [dict(key, keyId=key['keyId'][2:],
                                                    publicKey=key['publicKey'][2:])]
        assert expected['sequencer_id'] == key['keyId'][2:]
        assert expected['first_batch_number'] == key['firstHeight']
        assert expected['last_batch_number'] == key['lastHeight']
        assert expected['approval_threshold'] == str((1 << 128) - 1)
        assert set(expected) == module.WEB_KEYS
        path = private_json(tmp_path / 'web-reference.json', {'web': expected})
        before = (path.stat().st_ino, path.read_bytes())
        assert module.check_web_reference(json.loads(module.private_file(path)), expected) == expected
        del module
        reopened = production()
        after = reopened.web_reference(configuration(endpoint))
        assert reopened.check_web_reference(json.loads(reopened.private_file(path)), after) == expected
        assert after == expected
        assert (path.stat().st_ino, path.read_bytes()) == before
        assert len(requests) == 2


@pytest.mark.parametrize('field', ['payer_address', 'receiver_address', 'sequencer_id',
                                 'first_batch_number', 'last_batch_number', 'receipt_key_index'])
def test_binding_web_authority_changed_reference_refused(tmp_path, field):
    module = production()
    with receipt_server(rpc_payload()) as (endpoint, _):
        expected = module.web_reference(configuration(endpoint))
    changed = deepcopy(expected)
    if field == 'receipt_key_index':
        changed[field][0]['publicKey'] = '11' * 32
    elif field in ('first_batch_number', 'last_batch_number'):
        changed[field] += 1
    else:
        changed[field] = ('0x' if field.endswith('address') else '') + '22' * (
            20 if field.endswith('address') else 32)
    path = private_json(tmp_path / 'changed-reference.json', {'web': changed})
    with pytest.raises(module.GateFailure, match='differs from the configured references'):
        module.check_web_reference(json.loads(module.private_file(path)), expected)


@pytest.mark.parametrize('change', ['empty', 'oversized', 'duplicate', 'zero-id', 'zero-key',
                                   'hex-height', 'bool-height', 'overflow-height', 'reversed',
                                   'retired-key', 'missing-field', 'foreign-envelope'])
def test_binding_web_authority_rpc_shape_refused(change):
    module = production()
    payload = rpc_payload()
    keys = payload['result']['keys']
    if change == 'empty':
        keys.clear()
    elif change == 'oversized':
        keys *= 65
    elif change == 'duplicate':
        keys.append(deepcopy(keys[0]))
    elif change in ('zero-id', 'zero-key'):
        keys[0]['keyId' if change == 'zero-id' else 'publicKey'] = '0x' + '00' * 32
    elif change == 'hex-height':
        keys[0]['firstHeight'] = hex(keys[0]['firstHeight'])
    elif change == 'bool-height':
        keys[0]['firstHeight'] = True
    elif change == 'overflow-height':
        keys[0]['lastHeight'] = 1 << 64
    elif change == 'reversed':
        keys[0]['firstHeight'] = keys[0]['lastHeight'] + 1
    elif change == 'retired-key':
        keys[0]['sequencer_public_key'] = keys[0]['publicKey']
    elif change == 'missing-field':
        del keys[0]['keyId']
    elif change == 'foreign-envelope':
        payload['id'] = True
    with receipt_server(payload) as (endpoint, requests):
        with pytest.raises(module.GateFailure):
            module.web_reference(configuration(endpoint))
        assert len(requests) == 1


@pytest.mark.parametrize(('field', 'value'), [
    ('RECEIPT_KEY_INDEX', '1'), ('FIRST_BATCH_NUMBER', '999'),
    ('LAST_BATCH_NUMBER', '2000'), ('FIRST_BATCH_NUMBER', '2000'),
    ('PAYER_ADDRESS', 'did:layerx:retired'), ('RECEIVER_ADDRESS', '0x12'),
    ('NETWORK', 'layerx:9'), ('NETWORK', 'eip155:0125'),
    ('NETWORK', 'eip155:' + '1' * 65), ('TIMEOUT_MS', '0'), ('TIMEOUT_MS', '60001'),
    ('PENDING_ATTEMPTS', '0'), ('PENDING_ATTEMPTS', '11'),
    ('APPROVAL_THRESHOLD', str(1 << 128)),
    ('PAYER_ADDRESS', '0x' + '00' * 20), ('RECEIVER_ADDRESS', '0x' + '00' * 20),
    ('NETWORK', 'eip155:1'),
])
def test_binding_web_authority_configuration_refused(field, value):
    module = production()
    with receipt_server(rpc_payload()) as (endpoint, _):
        config = configuration(endpoint)
        config['LAYERX_AGENT_MCP_WEB_' + field] = value
        with pytest.raises(module.GateFailure):
            module.web_reference(config)


def test_binding_web_authority_environment_transport(tmp_path):
    module = production()
    values = configuration('http://127.0.0.1:9/')
    values.update({name: '1' for name in module.AGENTD_ENV})
    values.update({
        'LAYERX_AGENT_HUMAN_STORE': str(tmp_path / 'store'),
        'LAYERX_AGENT_HUMAN_SOCKET': str(tmp_path / 'human.sock'),
        'LAYERX_AGENT_MCP_MODE': 'full',
        'LAYERX_AGENT_MCP_SESSION_ID': '44' * 32,
        'LAYERX_AGENT_MCP_CAPABILITY_ID': '55' * 32,
        'LAYERX_AGENT_MCP_SCOPES': 'read:receipt',
        'LAYERX_AGENT_PROGRAM_LISTEN': '127.0.0.1:9',
    })
    path = tmp_path / 'daemon.env'
    path.write_text('\n'.join(name + '=' + value for name, value in values.items()) + '\n')
    path.chmod(0o600)
    daemon = module.daemon_environment(path)
    state = {name: tmp_path / name for name in ('store', 'human_socket', 'root', 'audit', 'socket')}
    web = module.binding_environment(daemon, state, 1001, 1002, 'write:web:search', web=True)
    plain = module.binding_environment(daemon, state, 1001, 1002, 'write:web:search')
    assert all(web[name] == values[name] for name in module.WEB_ENV)
    assert all(name not in plain for name in module.WEB_ENV)
    assert web['LAYERX_AGENT_MCP_SCOPES'] == 'write:web:search'
    assert plain['LAYERX_AGENT_MCP_SCOPES'] == 'read:receipt'
    for name in ('PAYER_ADDRESS', 'RECEIVER_ADDRESS', 'FIRST_BATCH_NUMBER', 'LAST_BATCH_NUMBER'):
        missing = dict(values)
        del missing['LAYERX_AGENT_MCP_WEB_' + name]
        path.write_text('\n'.join(key + '=' + value for key, value in missing.items()) + '\n')
        with pytest.raises(module.GateFailure, match='lacks LAYERX_AGENT_MCP_WEB_' + name):
            module.daemon_environment(path)


@contextmanager
def protocol_peer(path, replies, received):
    """An in-process wire peer for the production client, not a Rust server replacement."""
    errors = []
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as listener:
        listener.bind(str(path))
        listener.listen(1)
        listener.settimeout(5)

        def serve():
            try:
                connection, _ = listener.accept()
                with connection:
                    connection.settimeout(5)
                    with connection.makefile('rb') as stream:
                        received.extend(json.loads(line) for line in stream if line.strip())
                    encoded = b''.join((json.dumps(reply) + '\n').encode() for reply in replies)
                    # Split inside JSON tokens to exercise stream framing, not packet boundaries.
                    for offset in range(0, len(encoded), 7):
                        connection.sendall(encoded[offset:offset + 7])
            except Exception as error:
                errors.append(error)

        thread = threading.Thread(target=serve, daemon=True)
        thread.start()
        try:
            yield
        finally:
            thread.join(timeout=5)
            assert not thread.is_alive()
            assert not errors
    path.unlink()


def protocol_responses():
    return [
        {'jsonrpc': '2.0', 'id': 1, 'result': {
            'protocolVersion': '2025-06-18', 'capabilities': {'tools': {}},
            'serverInfo': {'name': 'layerx', 'title': 'LayerX', 'version': '1'},
            '_meta': {'layerx/binding': 'agent-daemon', 'layerx/deployment_mode': 'full'},
        }},
        {'jsonrpc': '2.0', 'id': 2, 'result': {'tools': [
            {'name': 'web.search'}, {'name': 'web.fetch'}, {'name': 'web.content'},
        ]}},
    ]


def test_binding_web_authority_unix_client_initializes_and_reconnects(tmp_path):
    path = tmp_path / 'mcp.sock'
    for _ in range(2):
        module = production()
        requests = module.initialization_requests()
        received = []
        with protocol_peer(path, protocol_responses(), received):
            reply = module.peer_exchange(path, requests, timeout=5)
            assert module.initialized_web_tools(reply, 'write:web:search,write:web:fetch') == [
                'web.fetch', 'web.search']
        assert received == requests
        assert [item['method'] for item in received] == [
            'initialize', 'notifications/initialized', 'tools/list']
        assert reply == {'transport': 'eof', 'responses': protocol_responses()}
        assert module.CLIENT.count('def peer_exchange(') == 1
        del module


@pytest.mark.parametrize('failure', ['missing-initialize', 'missing-paid-tool', 'read-only',
                                   'daemon-error', 'duplicate-response'])
def test_binding_web_authority_initialization_refusals(failure):
    module = production()
    replies = protocol_responses()
    if failure == 'missing-initialize':
        replies.pop(0)
    elif failure == 'missing-paid-tool':
        replies[1]['result']['tools'] = [{'name': 'receipt.get'}]
    elif failure == 'read-only':
        replies[0]['result']['_meta']['layerx/deployment_mode'] = 'read-only'
    elif failure == 'daemon-error':
        del replies[0]['result']
        replies[0]['error'] = {'code': -32000, 'message': 'authority refused'}
    elif failure == 'duplicate-response':
        replies.append(deepcopy(replies[0]))
    with pytest.raises(module.GateFailure):
        module.initialized_web_tools({'transport': 'eof', 'responses': replies}, 'write:web:search')


def test_binding_web_authority_native_harness_retains_actual_lifecycle_checks():
    module = production()
    for name in ('paid_tool_initialize', 'paid_tool_restart',
                 'restart_refuses_changed_web_authority', 'paid_tool_requires_payer'):
        assert name in module.CASES
        assert "self.case('" + name + "', " + name + ')' in (QUAL / 'mcp_binding_listener.py').read_text()
    start = nested_function('paid_tool_initialize')
    assert "served_session('mcp-web', state=web, path=web_binding)" in start
    assert 'web_exchange()' in start
    restart = nested_function('paid_tool_restart')
    for operation in ('self.stop(web_server)', 'self.stop(web_daemon)',
                      "self.boot(web, 'agentd-web-restart', web=True)",
                      'current == web_published', 'json.loads(private_file(web_binding))',
                      'check_web_reference(reopened, web_expected)', "served_session('mcp-web-restart'",
                      'web_exchange()'):
        assert operation in restart
    refusal = nested_function('restart_refuses_changed_web_authority')
    for field in ('PAYER_ADDRESS', 'RECEIVER_ADDRESS', 'FIRST_BATCH_NUMBER', 'RECEIPT_KEY_INDEX'):
        assert field in refusal
    assert '== web_published' in refusal
    assert 'self.refused(process, log,' in refusal
    assert 'self.peer(self.admitted,' in nested_function('web_exchange')
    assert 'initialized_web_tools(' in nested_function('web_exchange')
    assert 'self.serve(path, label)' in nested_function('served_session')
