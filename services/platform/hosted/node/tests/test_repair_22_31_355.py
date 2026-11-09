"""Node-owned lifecycle qualification with committed native peers and real codecs.

The native command peer replays the committed command/file contract; its RPC
listeners serve the committed chain transcript. The producer, broker, descriptor
consumer, signer, reset owner and activation filesystem are production code.
"""
import contextlib
import ctypes
import errno
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import importlib.util
from datetime import datetime
import os
from pathlib import Path
import platform
import signal
import socket
import ssl
import stat
import subprocess
import traceback
import sys
import threading
import time

import pytest
import yaml
from cryptography import x509
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

NODE = Path(__file__).resolve().parents[1]
ROOT = NODE.parents[3]
sys.path.insert(0, str(NODE))
sys.path.insert(0, str(NODE / 'signer'))
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
FIXTURES = Path(__file__).with_name('fixtures') / 'repair_22_31_355'

import data_directory as directory
import generation_client as client
import generation_transport as transport
import native_runtime as native
import reset_state as reset
import projected_secret
import signer as signer_service
from layerx_sdk.evm_transaction import decode_signed, activity_id
from client import SignerClient
from provider import FileProvider, verify


@pytest.fixture
def tls(tmp_path):
    certificate = x509.load_pem_x509_certificate((FIXTURES / 'tls-certificate.pem.txt').read_bytes())
    key = serialization.load_pem_private_key((FIXTURES / 'tls-private-key.pem.txt').read_bytes(), password=None)
    der, key_der = tmp_path / 'tls.der', tmp_path / 'tls-key.der'
    native.write_file(der, certificate.public_bytes(serialization.Encoding.DER))
    native.write_file(key_der, key.private_bytes(serialization.Encoding.DER,
                      serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    return der, key_der


def paths(root):
    result = tuple(root / name for name in ('data', 'state', 'run'))
    for path in result:
        path.mkdir(mode=0o700)
    return result


def recorded_generation(data, tls):
    """Publish retained native configuration through the production producer.

    The fixture carries the native chain export and the native module schemas.
    The node's publication and descriptor consumers run without substitution.
    """
    (data / 'config').mkdir(mode=0o700)
    for name, source in (
        ('genesis.json', FIXTURES / 'genesis.json'),
        ('config.toml', FIXTURES / 'config.toml'),
        ('app.toml', FIXTURES / 'app.toml'),
    ):
        native.write_file(data / 'config' / name, source.read_bytes())
    seed = data.parent / ('receipt-' + data.parent.name + '.key')
    # This is input key material; the real provider derives its actual public identity.
    native.write_file(seed, os.urandom(32))
    provider = FileProvider(seed)
    try:
        public = provider.public_key()
    finally:
        provider.close()
    node = {
        'LAYERX_NODE_NETWORK_ID': '402', 'LAYERX_NODE_CHAIN_ID': 'hyperpax_125-1',
        'LAYERX_NODE_EVM_CHAIN_ID': '125',
        'LAYERX_NODE_SEQUENCER_ID': hashlib.sha256(public).hexdigest(),
        'LAYERX_NODE_SEQUENCER_PUBLIC_KEY': public.hex(),
        'LAYERX_NODE_RPC_URL': 'http://127.0.0.1:8545',
        'LAYERX_NODE_CONSENSUS_RPC_URL': 'http://127.0.0.1:26657',
    }
    core = {
        'LAYERX_CORE_SEQUENCER_ID': node['LAYERX_NODE_SEQUENCER_ID'],
        'LAYERX_CORE_TREASURY_ASSET': 'b5a32b12029f8ddfb905f90f280f664b46390de0fc62770fc197dd87b18cd898',
        'LAYERX_CORE_TREASURY_SIGNER_SOCKET': str(data.parent / 'treasury.sock'),
        'LAYERX_CORE_SEQUENCER_ENDPOINT': 'https://localhost:9443',
        'LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER': '/generation/rpc-ca.der',
        'LAYERX_CORE_CHAIN_RPC_URL': 'http://127.0.0.1:8545',
        'LAYERX_CORE_FUNDING_SENDER_KEY_FILE': str(data.parent / 'funding.key'),
    }
    binding = native.publish_generation(data, node, core, tls[0].read_bytes())
    return binding, core


def wait_until(condition, process=None, timeout=10):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process is not None:
            assert process.poll() is None, 'real service exited before readiness'
        try:
            value = condition()
            if value:
                return value
        except (OSError, ValueError, KeyError, reset.StoreError, http.client.HTTPException):
            pass
        time.sleep(0.02)
    raise AssertionError('real service readiness timed out')


@contextlib.contextmanager
def broker(data, state, run):
    path = run / 'generation.sock'
    command = [sys.executable, str(NODE / 'generation_transport.py'),
               '--data-dir', str(data), '--state-dir', str(state), '--run-dir', str(run),
               '--socket', str(path), '--allowed-uid', str(os.geteuid()),
               '--allowed-gid', str(os.getegid())]
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        wait_until(path.exists, process)
        yield path
    finally:
        native.stop_process(process)
        process.communicate(timeout=10)
        path.unlink(missing_ok=True)


def receive(path, generation=None):
    return client.receive(str(path), 'core', generation, os.geteuid(), os.getegid())


def staged_store(data, state, run, bindings=None):
    if bindings is None and (run / 'bindings.json').exists():
        bindings = reset.decode(native.read_regular(run / 'bindings.json', 65536))
    store = reset.Store(state, data, run, 402, bindings or {'owner': 'native'}, os.getegid())
    store.staged = True
    store.initialize()
    return store


def request(operation='reset', identity='a' * 32):
    value = {'version': 1, 'operation': operation, 'reset_id': identity, 'network': 402}
    value['request_digest'] = reset.request_digest(value)
    return value


def reset_client(store, value):
    first, second = socket.socketpair()
    worker = threading.Thread(target=reset.handle_connection, args=(second, store, 0))
    worker.start()
    try:
        first.settimeout(5)
        first.sendall(reset.canonical(value) + b'\n')
        response = first.makefile('rb').readline(65537)
        assert response.endswith(b'\n') and len(response) <= 65536
        return reset.decode(response)
    finally:
        first.close()
        worker.join(timeout=5)
        assert not worker.is_alive()


def prepared(data, state, run, tls):
    store = staged_store(data, state, run)
    # Admission goes through the real peer-authorized JSON socket consumer.
    result = reset_client(store, request())
    assert result == reset.error_response('reset_timeout')
    stage_root = Path(directory.prepare_stage(data, state, 'a' * 32))
    binding, core = recorded_generation(stage_root / 'data', tls)
    directory.seal_generation(data, stage_root / 'data', 'a' * 32, 2, binding)
    store.transition('a' * 32, 'admitted', 'stopping')
    store.transition('a' * 32, 'stopping', 'bootstrapping')
    store.transition('a' * 32, 'bootstrapping', 'prepared', {'genesis': binding})
    store.transition('a' * 32, 'prepared', 'activating')
    return store, stage_root / 'data', binding, core


def test_core_startup_inputs_real_descriptor_handoff(tmp_path, tls):
    data, state, run = paths(tmp_path)
    binding, core = recorded_generation(data, tls)
    staged_store(data, state, run)
    with broker(data, state, run) as path:
        result, descriptors = receive(path, 1)
        try:
            assert result['version'] == 2 and set(result['artifacts']) == directory.GENESIS_FILES
            adopted = client.child_environment(result, descriptors, 'core')
            view, adopted = client.materialize(result, descriptors, 'core', adopted)
            try:
                for name, value in core.items():
                    if name != 'LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER':
                        assert adopted[name] == value
                assert Path(adopted['LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER']).read_bytes() == tls[0].read_bytes()
                context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
                context.load_verify_locations(cadata=ssl.DER_cert_to_PEM_cert(
                    Path(adopted['LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER']).read_bytes()))
                assert context.cert_store_stats()['x509'] == 1
                assert not any(name in adopted for name in (
                    'LAYERX_NODE_SNAPSHOT', 'LAYERX_AUTHORITY_REPLICA_ID',
                    'LAYERX_NODE_SEQUENCER_PRIVATE_KEY'))
                assert json.loads((data / '.native-prepared.json').read_bytes()) == binding
            finally:
                view.cleanup()
        finally:
            client.close_descriptors(descriptors)
    # Reopen the actual broker and client against their retained producer state.
    with broker(data, state, run) as path:
        result, descriptors = receive(path, 1)
        try:
            assert result['generation'] == 1
            assert client.child_environment(result, descriptors, 'core')['LAYERX_CORE_SEQUENCER_ID'] == core['LAYERX_CORE_SEQUENCER_ID']
        finally:
            client.close_descriptors(descriptors)


def manifests():
    docs = list(yaml.safe_load_all((NODE / 'deployment.yaml').read_text()))
    return docs, next(doc['spec']['template']['spec'] for doc in docs if doc['kind'] == 'StatefulSet')


def test_treasury_projected_secret_startup(tmp_path):
    _, pod = manifests()
    assert [value['name'] for value in pod['initContainers']] == ['native-home']
    assert [value['name'] for value in pod['containers']] == ['paxd']
    assert "'receipt.key': 'config/receipt.key'" in pod['initContainers'][0]['args'][0]
    projection, private = tmp_path / 'projection', tmp_path / 'private'
    projection.mkdir()
    private.mkdir(mode=0o700)
    revision = projection / '..2026_native'
    revision.mkdir()
    seed = os.urandom(32)
    (revision / 'treasury.key').write_bytes(seed)
    (revision / 'treasury.key').chmod(0o440)
    (projection / '..data').symlink_to(revision.name, target_is_directory=True)
    (projection / 'treasury.key').symlink_to('..data/treasury.key')
    with pytest.raises(OSError):
        FileProvider(projection / 'treasury.key')
    projected_secret.main(['--source', str(projection / 'treasury.key'),
                           '--destination', str(private / 'owner/treasury.key')])
    regular = private / 'owner' / 'treasury.key'
    assert not regular.is_symlink() and regular.read_bytes() == seed
    assert stat.S_IMODE(regular.stat().st_mode) == 0o600
    assert stat.S_IMODE(regular.parent.stat().st_mode) == 0o700
    assert regular.stat().st_nlink == 1
    assert all(volume['name'] not in {'treasury-private', 'treasury-material'}
               for volume in pod['volumes'])
    assert {mount['name'] for mount in pod['containers'][0]['volumeMounts']} == {'data', 'tmp'}
    path = tmp_path / 'signer.sock'
    provider = FileProvider(regular)
    signer_backend = signer_service.Signer(provider, [os.geteuid()])
    server = signer_service.Server(str(path), signer_service.Handler)
    server.signer = signer_backend
    server.request_timeout = 5
    path.chmod(0o660)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        public = provider.public_key()
        consumer = SignerClient(str(path), expected_peer_uid=os.geteuid(),
                                expected_peer_gid=os.getegid(), expected_public_key=public)
        assert consumer.public_key() == public
        digest = hashlib.sha256(b'native treasury projection qualification').digest()
        assert verify(public, digest, consumer.sign(digest))
    finally:
        server.shutdown()
        worker.join(timeout=5)
        server.server_close()
        signer_backend.close()
        assert not worker.is_alive()


def test_funding_key_projection_is_private_to_core(tmp_path):
    _, pod = manifests()
    assert [value['name'] for value in pod['initContainers']] == ['native-home']
    assert [value['name'] for value in pod['containers']] == ['paxd']
    assert "'treasury-policy.json': 'treasury-policy.json'" in pod['initContainers'][0]['args'][0]
    projection, private = tmp_path / 'projection', tmp_path / 'private'
    projection.mkdir()
    private.mkdir(mode=0o700)
    version = projection / '..native'
    version.mkdir()
    scalar = bytes.fromhex(json.loads((FIXTURES / 'native_configuration.json').read_text())['funding_scalar'])
    key = ec.derive_private_key(int.from_bytes(scalar, 'big'), ec.SECP256K1())
    assert key.key_size == 256
    (version / 'sender.key').write_text(scalar.hex() + '\n')
    (version / 'sender.key').chmod(0o440)
    (projection / '..data').symlink_to(version.name, target_is_directory=True)
    (projection / 'sender.key').symlink_to('..data/sender.key')
    projected_secret.main(['--source', str(projection / 'sender.key'),
                           '--destination', str(private / 'owner/sender.key')])
    regular = private / 'owner' / 'sender.key'
    assert not regular.is_symlink() and regular.read_text().strip() == scalar.hex()
    assert stat.S_IMODE(regular.stat().st_mode) == 0o600
    assert stat.S_IMODE(regular.parent.stat().st_mode) == 0o700
    assert all(volume['name'] not in {'funding-private', 'funding-material'}
               for volume in pod['volumes'])
    assert not any(entry['name'].startswith('LAYERX_CORE_FUNDING_')
                   for container in pod['containers'] for entry in container.get('env', []))


def test_native_lifecycle_rejects_retired_arguments(tmp_path, capsys):
    for action, retired in (('bootstrap', '--layerxd'), ('supervise', '--role')):
        with pytest.raises(SystemExit) as error:
            native.parser().parse_args([action, '--data-dir', str(tmp_path), retired, 'retired'])
        assert error.value.code == 2
        stderr = capsys.readouterr().err
        assert 'unrecognized arguments' in stderr and retired in stderr


class Registers(ctypes.Structure):
    _fields_ = [(name, ctypes.c_ulonglong) for name in (
        'r15', 'r14', 'r13', 'r12', 'rbp', 'rbx', 'r11', 'r10', 'r9', 'r8', 'rax',
        'rcx', 'rdx', 'rsi', 'rdi', 'orig_rax', 'rip', 'cs', 'eflags', 'rsp', 'ss',
        'fs_base', 'gs_base', 'ds', 'es', 'fs', 'gs')]


def traced_activation(data, stage, binding, failure=None):
    """Linux syscall tracing injects EIO or process death after the real exchange.

    The component, filesystem and rename all run unchanged; only the kernel's
    result of one fsync is faulted, like a failed persistence barrier.
    """
    assert platform.machine() == 'x86_64', 'syscall qualification requires the declared Linux x86_64 image'
    libc = ctypes.CDLL(None, use_errno=True)
    libc.ptrace.argtypes = (ctypes.c_uint, ctypes.c_int, ctypes.c_void_p, ctypes.c_void_p)
    libc.ptrace.restype = ctypes.c_long

    def trace(operation, pid, value=None):
        result = libc.ptrace(operation, pid, None, value)
        if result == -1:
            raise OSError(ctypes.get_errno(), os.strerror(ctypes.get_errno()))
        return result

    child = os.fork()
    if child == 0:
        try:
            trace(0, 0)  # PTRACE_TRACEME
            os.kill(os.getpid(), signal.SIGSTOP)
            directory.activate_generation(data, stage, 'a' * 32, 2, binding)
        except OSError as error:
            os._exit(17 if error.errno == errno.EIO else 18)
        except BaseException:
            os._exit(19)
        os._exit(0)
    synced = []
    changed = False
    entering = True
    pending_failure = False
    injected = False
    status = None
    try:
        _, status = os.waitpid(child, 0)
        assert os.WIFSTOPPED(status)
        trace(0x4200, child, ctypes.c_void_p(1))  # PTRACE_O_TRACESYSGOOD
        while True:
            trace(24, child)  # PTRACE_SYSCALL
            _, status = os.waitpid(child, 0)
            if os.WIFEXITED(status) or os.WIFSIGNALED(status):
                break
            assert os.WIFSTOPPED(status) and os.WSTOPSIG(status) == (signal.SIGTRAP | 0x80)
            registers = Registers()
            trace(12, child, ctypes.cast(ctypes.byref(registers), ctypes.c_void_p))
            if entering:
                if registers.orig_rax == 74 and changed:  # fsync
                    synced.append(os.readlink('/proc/' + str(child) + '/fd/' + str(registers.rdi)))
                    index = len(synced) - 1
                    if failure == ('kill', index):
                        os.kill(child, signal.SIGKILL)
                        _, status = os.waitpid(child, 0)
                        injected = True
                        break
                    if failure == ('eio', index):
                        registers.orig_rax = (1 << 64) - 1
                        trace(13, child, ctypes.cast(ctypes.byref(registers), ctypes.c_void_p))
                        pending_failure = True
                        injected = True
            else:
                if registers.orig_rax == 316 and registers.rax == 0:  # renameat2
                    changed = True
                if pending_failure:
                    registers.rax = (1 << 64) - errno.EIO
                    trace(13, child, ctypes.cast(ctypes.byref(registers), ctypes.c_void_p))
                    pending_failure = False
            entering = not entering
        assert changed, 'activation did not execute the real RENAME_EXCHANGE'
        return os.waitstatus_to_exitcode(status), synced, injected
    finally:
        if status is None or not (os.WIFEXITED(status) or os.WIFSIGNALED(status)):
            os.kill(child, signal.SIGKILL)
            os.waitpid(child, 0)


@pytest.mark.parametrize('failure', [None, ('eio', 0), ('eio', 1), ('kill', 0), ('kill', 1)])
def test_activation_rename_durability_reopen(tmp_path, tls, failure):
    data, state, run = paths(tmp_path)
    store, stage, binding, _ = prepared(data, state, run, tls)
    old = data / 'retained-old-generation'
    old.write_text('old native generation')
    code, synced, injected = traced_activation(data, stage, binding, failure)
    assert (data / directory.GENERATION_MARKER).exists()
    if failure is None:
        assert code == 0 and not injected
        assert synced[:2] == [str(data.parent), str(stage.parent)]
        assert json.loads((data / directory.ACTIVATION_MARKER).read_bytes())['genesis'] == binding
    else:
        assert injected
        assert code == (17 if failure[0] == 'eio' else -signal.SIGKILL)
        assert not (data / directory.ACTIVATION_MARKER).exists()
        assert store.active()['phase'] == 'activating'
    # Reopen in another process and reconcile the selected directory before any success.
    result = subprocess.run([sys.executable, str(NODE / 'data_directory.py'),
                             'activate-generation', str(data), str(stage), 'a' * 32, '2',
                             json.dumps(binding)], capture_output=True, text=True, timeout=10)
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == 'active'
    assert (stage / old.name).read_text() == 'old native generation'
    assert not (data / old.name).exists()
    assert native.generation_binding(data) == binding
    assert json.loads((data / directory.ACTIVATION_MARKER).read_bytes()) == json.loads((data / directory.GENERATION_MARKER).read_bytes())
    reopened = staged_store(data, state, run)
    assert reopened.active()['phase'] == 'activating'  # committed native health is still required
    with broker(data, state, run) as path:
        result, descriptors = receive(path, 2)
        try:
            assert result['reset_id'] == 'a' * 32 and result['generation'] == 2
        finally:
            client.close_descriptors(descriptors)


def test_reset_refuses_unpublished_or_uncommitted_generation(tmp_path, tls):
    data, state, run = paths(tmp_path)
    store, stage, binding, _ = prepared(data, state, run, tls)
    assert reset_client(store, request('status'))['state'] == 'activating'
    directory.activate_generation(data, stage, 'a' * 32, 2, binding)
    assert reset_client(store, request('status'))['state'] == 'activating'
    reopened = staged_store(data, state, run)
    assert reset_client(reopened, request('status'))['state'] == 'activating'
    assert json.loads((state / 'state.json').read_bytes())['generation'] == 1
    assert not (data / 'sequencer.env').exists() and not (data / 'replica.env').exists()
    # Neither an absent durable barrier nor a retired operation can be exported.
    (data / directory.ACTIVATION_MARKER).unlink()
    with broker(data, state, run) as path:
        with pytest.raises(reset.StoreError):
            receive(path, 2)
        with socket.socket(socket.AF_UNIX) as connection:
            connection.connect(str(path))
            connection.sendall(reset.canonical({'version': 2, 'operation': 'identity',
                                                'expected_generation': 2}) + b'\n')
            assert reset.decode(connection.makefile('rb').readline()) == {'error': {'code': 'generation_unavailable'}}


class ForkedProcess:
    """An actual forked Python peer process, with the lifecycle used by Popen."""
    def __init__(self, target, log=None):
        self.returncode = None
        self.pid = os.fork()
        if self.pid == 0:
            try:
                reset.close_supervisor_lock()
                reset.parent_death_signal()
                signal.signal(signal.SIGTERM, signal.SIG_DFL)
                signal.signal(signal.SIGINT, signal.SIG_DFL)
                if log is not None:
                    with log.open('ab', buffering=0) as output:
                        os.dup2(output.fileno(), 1)
                        os.dup2(output.fileno(), 2)
                code = target()
                os._exit(0 if code is None else code)
            except BaseException:
                traceback.print_exc()
                os._exit(1)

    def poll(self):
        if self.returncode is None:
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.returncode = os.waitstatus_to_exitcode(status)
        return self.returncode

    def send_signal(self, number):
        os.kill(self.pid, number)

    def terminate(self):
        self.send_signal(signal.SIGTERM)

    def kill(self):
        self.send_signal(signal.SIGKILL)

    def wait(self, timeout):
        deadline = time.monotonic() + timeout
        while self.poll() is None:
            if time.monotonic() >= deadline:
                raise subprocess.TimeoutExpired('recorded peer', timeout)
            time.sleep(0.01)
        return self.returncode


class RecordedNativePeer:
    """Strict native command/file and loopback RPC peers for the node owner.

    Requests must match the committed transcript. No paxd, core, Docker, shell,
    or crypto executable is discovered or synthesized by this peer.
    """
    def __init__(self, configuration, root):
        self.configuration = configuration
        self.root = root
        self.journal = root / 'peer-requests.jsonl'
        self.funding_state = root / 'peer-funding.json'
        self.children = []
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                try:
                    assert self.path == '/'
                    assert self.headers['Content-Type'] == 'application/json'
                    assert self.headers['Accept'] == 'application/json'
                    assert self.headers['Connection'] == 'close'
                    size = int(self.headers['Content-Length'])
                    assert 0 < size <= 1048576
                    request_value = reset.decode(self.rfile.read(size))
                    assert set(request_value) == {'jsonrpc', 'id', 'method', 'params'}
                    assert request_value['jsonrpc'] == '2.0'
                    method, params = request_value['method'], request_value['params']
                    peer.record({'rpc': request_value, 'tls': isinstance(self.connection, ssl.SSLSocket)})
                    funding = peer.configuration['funding']
                    if method == 'eth_getBalance':
                        assert params == [funding['recipient'], 'latest']
                        funded = peer.funding_state.exists()
                        result = hex(funding['after'] if funded else funding['before'])
                    else:
                        exchange = next(item for item in peer.configuration['rpc_exchanges']
                                        if item['method'] == method and item['params'] == params)
                        if method == 'eth_sendRawTransaction':
                            raw = bytes.fromhex(params[0][2:])
                            transaction, signature = decode_signed(raw)
                            assert transaction.recover_signer(signature) == funding['sender']
                            assert transaction.chain_id == peer.configuration['evm_chain_id']
                            assert transaction.to.hex() == funding['recipient'][2:]
                            assert transaction.value == funding['amount']
                            assert '0x' + activity_id(raw).hex() == funding['transaction_id']
                            native.write_file(peer.funding_state, reset.canonical(funding))
                        if method == 'eth_getTransactionReceipt':
                            assert peer.funding_state.exists()
                        result = exchange['result']
                    body = reset.canonical({'jsonrpc': '2.0', 'id': request_value['id'], 'result': result})
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except BaseException:
                    traceback.print_exc()
                    self.send_error(500, 'request outside committed transcript')

            def log_message(self, *_args):
                pass

        self.chain = HTTPServer(('127.0.0.1', 0), Handler)
        self.consensus = HTTPServer(('127.0.0.1', 0), Handler)
        self.gateway = HTTPServer(('127.0.0.1', 0), Handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(FIXTURES / 'tls-certificate.pem.txt', FIXTURES / 'tls-private-key.pem.txt')
        self.gateway.socket = context.wrap_socket(self.gateway.socket, server_side=True)
        self.servers = (self.chain, self.consensus, self.gateway)

    def record(self, value):
        descriptor = os.open(self.journal, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
        try:
            os.write(descriptor, reset.canonical(value) + b'\n')
        finally:
            os.close(descriptor)

    def expected(self, name, command):
        home = command[command.index('--home') + 1]
        replacements = {'home': home, 'chain_port': self.chain.server_port,
                        'consensus_port': self.consensus.server_port,
                        'receipt_key_file': str(self.root / 'receipt.key')}
        return [item.format(**replacements) for item in self.configuration['commands'][name]]

    def validate_inputs(self, home):
        for name in ('genesis.json', 'config.toml', 'app.toml'):
            assert native.read_regular(home / 'config' / name) == (FIXTURES / name).read_bytes()
        assert reset.decode(native.read_regular(home / 'config/priv_validator_key.json')) == self.configuration['validator_key']

    def run(self, command):
        operation = {'init': 'init', 'validate-genesis': 'validate'}[command[1]]
        assert command == self.expected(operation, command)
        self.record({'command': command, 'exit': 0})
        home = Path(command[command.index('--home') + 1])
        if operation == 'init':
            assert not list(home.iterdir())
            for directory_name in self.configuration['init_directories']:
                (home / directory_name).mkdir(mode=0o700)
            for path, contents in self.configuration['init_files'].items():
                native.write_file(home / path, contents.encode())
        else:
            # The recorded validate result applies only to the exact committed input.
            self.validate_inputs(home)

    def start(self, command):
        assert command == self.expected('start', command)
        self.validate_inputs(Path(command[command.index('--home') + 1]))
        self.record({'command': command})

        def serve():
            for server in self.servers:
                threading.Thread(target=server.serve_forever, daemon=True).start()
            while True:
                signal.pause()

        child = ForkedProcess(serve)
        self.children.append(child)
        return child

    def close(self):
        for child in self.children:
            native.stop_process(child)
        for server in self.servers:
            server.server_close()


@pytest.fixture
def native_configuration(tmp_path, tls):
    value = json.loads((FIXTURES / 'native_configuration.json').read_text())
    assert set(value) == {'network_id', 'chain_id', 'evm_chain_id', 'asset', 'receipt_seed',
                         'receipt_public_key', 'funding_scalar', 'validator_key', 'image',
                         'commands', 'status', 'provenance', 'funding', 'init_directories',
                         'init_files', 'rpc_exchanges'}
    assert value['network_id'] == 402
    inputs = tmp_path / 'native-inputs'
    inputs.mkdir(mode=0o700)
    for name in ('genesis.json', 'config.toml', 'app.toml'):
        native.write_file(inputs / name, (FIXTURES / name).read_bytes())
    native.write_file(inputs / 'priv_validator_key.json', reset.canonical(value['validator_key']))
    native.write_file(inputs / 'rpc-ca.der', tls[0].read_bytes())
    native.write_file(tmp_path / 'receipt.key', value['receipt_seed'].encode())
    native.write_file(tmp_path / 'funding.key', value['funding_scalar'].encode())
    peer = RecordedNativePeer(value, tmp_path)
    config = dict(value, paxd='paxd', native_inputs=str(inputs),
                  receipt_key_file=str(tmp_path / 'receipt.key'),
                  funding_sender_key_file=str(tmp_path / 'funding.key'),
                  chain_rpc_url='http://127.0.0.1:' + str(peer.chain.server_port),
                  consensus_rpc_url='http://127.0.0.1:' + str(peer.consensus.server_port),
                  gateway_endpoint='https://localhost:' + str(peer.gateway.server_port),
                  peer=peer)
    for name in ('chain_rpc_url', 'consensus_rpc_url'):
        native.endpoint(config[name], loopback=True)
    assert native.endpoint(config['gateway_endpoint'], tls=True).hostname == 'localhost'
    try:
        yield config
    finally:
        peer.close()


def native_arguments(config, data, state, run):
    result = ['--data-dir', str(data), '--run-dir', str(run), '--state-dir', str(state),
              '--consumer-uid', str(os.geteuid()), '--consumer-gid', str(os.getegid())]
    for name in ('paxd', 'native_inputs', 'receipt_key_file', 'funding_sender_key_file',
                 'network_id', 'chain_id', 'evm_chain_id', 'asset', 'gateway_endpoint',
                 'chain_rpc_url', 'consensus_rpc_url'):
        result.extend(['--' + name.replace('_', '-'), str(config[name])])
    result.extend(['--treasury-signer-socket', str(run / 'treasury.sock')])
    return result


@contextlib.contextmanager
def native_supervisor(config, data, state, run, log):
    args = native.parser().parse_args(['supervise', *native_arguments(config, data, state, run)])
    process = ForkedProcess(lambda: native.supervise(args, commands=config['peer']), log)
    try:
        def ready():
            metadata = reset.decode(native.read_regular(run / 'native-process.json', 8192))
            expected = {'generation': metadata['generation'], 'reset_id': metadata['reset_id'],
                        'genesis': native.generation_binding(data)}
            if not native.activated_ready(data, run, expected) or not (run / 'generation.sock').exists():
                return False
            result, descriptors = receive(run / 'generation.sock', metadata['generation'])
            client.close_descriptors(descriptors)
            return result['reset_id'] == metadata['reset_id']
        wait_until(ready, process, timeout=10)
        yield process
    finally:
        native.stop_process(process)


def test_bootstrap_retired_prerequisites_empty_native_data(tmp_path, native_configuration):
    config = native_configuration
    data, state, run = paths(tmp_path)
    arguments = native_arguments(config, data, state, run)
    parsed = native.parser().parse_args(['bootstrap', *arguments])
    binding = native.bootstrap(parsed, commands=config['peer'])
    calls = [json.loads(line)['command'] for line in config['peer'].journal.read_text().splitlines()]
    assert [call[1] for call in calls] == ['init', 'validate-genesis']
    assert set(binding) == directory.GENESIS_FILES
    assert native.generation_binding(data) == binding
    genesis = json.loads((data / 'config/genesis.json').read_bytes())
    assert genesis['chain_id'] == config['chain_id']
    assert genesis['app_state']['bank']['balances']
    assert 'lxphost' in genesis['app_state']
    assert not any((data / name).exists() for name in ('sequencer.env', 'replica.env', 'producer-generations', 'genesis'))
    for name in ('layerxd', 'layerx-genesis-build', 'checkpoint-settlement.json', 'guarantor'):
        assert name not in (NODE / 'bootstrap.sh').read_text()


def test_deployment_retired_topology_contract():
    docs, pod = manifests()
    containers = {value['name']: value for value in pod['containers']}
    assert 'paxd' in containers
    assert not {'layerxd', 'layerxd-authority', 'paxeer-relay'} & containers.keys()
    assert pod['shareProcessNamespace'] is False and not pod['automountServiceAccountToken']
    assert all(value['securityContext']['readOnlyRootFilesystem'] for value in pod['containers'])
    assert any(value.get('metadata', {}).get('name') == 'layerx-native-chain' for value in docs)
    assert set(containers) == {'paxd'}
    assert containers['paxd']['command'] == [
        'python3', '/opt/layerx/kernel_runtime.py', '--home', '/data/layerx/node']
    assert not any('LNI' in value['name'] or value['name'].startswith('LAYERX_CORE_')
                   for container in pod['containers'] for value in container.get('env', []))


def test_deployment_retired_topology_native_committed_reads(tmp_path, native_configuration):
    data, state, run = paths(tmp_path)
    with native_supervisor(native_configuration, data, state, run, tmp_path / 'supervisor.log'):
        node = transport.environment((data / 'node.env').read_bytes())
        assert native.committed_health(node)['height'] > 0
        for port in (9401, 9402, 18545):
            with socket.socket() as connection:
                connection.settimeout(0.2)
                assert connection.connect_ex(('127.0.0.1', port)) != 0
    _, pod = manifests()
    container = next(item for item in pod['containers'] if item['name'] == 'paxd')
    assert container['image'] == native_configuration['image']
    assert container['command'][1] == '/opt/layerx/kernel_runtime.py'
    probe = container['readinessProbe']['httpGet']
    assert probe == {'path': '/readyz', 'port': 'health'}
    assert next(port['containerPort'] for port in container['ports']
                if port['name'] == probe['port']) == 9460
    spec = importlib.util.spec_from_file_location('deployed_kernel_runtime', ROOT / 'docker/kernel/kernel_runtime.py')
    kernel = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(kernel)
    stamp = datetime.fromisoformat(native_configuration['status']['sync_info']['latest_block_time'].replace('Z', '+00:00'))
    checkpoint = state / 'committed.json'
    with native_supervisor(native_configuration, data, state, run, tmp_path / 'supervisor.log'):
        node = transport.environment((data / 'node.env').read_bytes())
        pid = reset.decode(native.read_regular(run / 'native-process.json', 8192))['pid']
        readiness = kernel.CommittedReadiness(node, checkpoint, hashlib.sha256((data / 'config/genesis.json').read_bytes()).hexdigest(),
                                     pid, clock=stamp.timestamp)
        for _ in range(2):
            server = kernel.readiness_server(readiness, ('127.0.0.1', 0))
            worker = threading.Thread(target=server.serve_forever)
            worker.start()
            connection = http.client.HTTPConnection('127.0.0.1', server.server_port, timeout=5)
            try:
                connection.request('GET', probe['path'])
                response = connection.getresponse()
                value = json.loads(response.read())
                assert response.status == 200 and value['ready'] is True
                assert value['chain_id'] == 125 and value['height'] > 0
                assert reset.decode(native.read_regular(checkpoint))['height'] == value['height']
            finally:
                connection.close()
                server.shutdown()
                worker.join(timeout=5)
                assert not worker.is_alive()
                server.server_close()
            readiness = kernel.CommittedReadiness(node, checkpoint, readiness.genesis, pid, clock=stamp.timestamp)
        readiness.clock = lambda: stamp.timestamp() + kernel.MAXIMUM_COMMIT_AGE + 1
        assert readiness.probe() == (503, {'ready': False, 'chain_id': 125})


def test_supervisor_retired_launch_reopens_native_process(tmp_path, native_configuration):
    data, state, run = paths(tmp_path)
    with native_supervisor(native_configuration, data, state, run, tmp_path / 'supervisor.log'):
        first = reset.decode(native.read_regular(run / 'native-process.json', 8192))
        child = first['pid']
        assert Path('/proc', str(child), 'exe').resolve() == Path(sys.executable).resolve()
        calls = [json.loads(line).get('command') for line in native_configuration['peer'].journal.read_text().splitlines()]
        assert any(call and call[0:2] == ['paxd', 'start'] for call in calls)
        committed = native.committed_health(transport.environment((data / 'node.env').read_bytes()))
    wait_until(lambda: not Path('/proc', str(child)).exists())
    retained = native.generation_binding(data)
    with native_supervisor(native_configuration, data, state, run, tmp_path / 'supervisor.log'):
        second = reset.decode(native.read_regular(run / 'native-process.json', 8192))
        assert (second['pid'], second['start']) != (first['pid'], first['start'])
        assert native.generation_binding(data) == retained
        assert native.committed_health(transport.environment((data / 'node.env').read_bytes()))['height'] >= committed['height']
        assert second['generation'] == first['generation'] == 1


def test_reset_retired_readiness_interrupted_recovery(tmp_path, native_configuration):
    data, state, run = paths(tmp_path)
    config = native_configuration
    with native_supervisor(config, data, state, run, tmp_path / 'supervisor.log') as supervisor:
        store = staged_store(data, state, run)
        # Stop the owner at a real process boundary, retaining its native generation.
        supervisor.send_signal(signal.SIGSTOP)
        try:
            assert reset_client(store, request()) == reset.error_response('reset_timeout')
            record = store.active()
            stage = Path(directory.prepare_stage(data, state, record['reset_id'])) / 'data'
            store.transition(record['reset_id'], 'admitted', 'stopping')
            store.transition(record['reset_id'], 'stopping', 'bootstrapping')
            parsed = native.parser().parse_args(['bootstrap', *native_arguments(config, stage, state, run)])
            binding = native.bootstrap(parsed, commands=config['peer'])
            directory.seal_generation(data, stage, record['reset_id'], 2, binding)
            store.transition(record['reset_id'], 'bootstrapping', 'prepared', {'genesis': binding})
            store.transition(record['reset_id'], 'prepared', 'activating')
            # Crash before publication. The owner must reconcile this exact retained reset.
            supervisor.kill()
            supervisor.wait(timeout=10)
        finally:
            if supervisor.poll() is None:
                supervisor.send_signal(signal.SIGCONT)
    with native_supervisor(config, data, state, run, tmp_path / 'supervisor.log'):
        reopened = staged_store(data, state, run)
        expected = {'state': 'reset', 'reset_id': 'a' * 32, 'generation': 2}
        assert reset_client(reopened, request('status')) == expected
        assert json.loads((state / 'state.json').read_bytes())['records']['a' * 32]['response'] == expected
        assert not (data / 'sequencer.env').exists() and not (data / 'replica.env').exists()
    with native_supervisor(config, data, state, run, tmp_path / 'supervisor.log'):
        assert reset_client(staged_store(data, state, run), request('status')) == expected


def tls_request(port, ca, method, path, token=None, body=None, key=None):
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.load_verify_locations(cadata=ssl.DER_cert_to_PEM_cert(ca))
    connection = http.client.HTTPSConnection('localhost', port, context=context, timeout=5)
    headers = {'Accept': 'application/json', 'Connection': 'close'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if key:
        headers['Idempotency-Key'] = key
    if body is not None:
        body = reset.canonical(body)
        headers['Content-Type'] = 'application/json'
    try:
        connection.request(method, path, body, headers)
        response = connection.getresponse()
        raw = response.read(1024 * 1024 + 1)
        assert len(raw) <= 1024 * 1024
        return response.status, reset.decode(raw)
    finally:
        connection.close()


def free_ports(count):
    holders = [socket.socket() for _ in range(count)]
    try:
        for holder in holders:
            holder.bind(('127.0.0.1', 0))
        return [holder.getsockname()[1] for holder in holders]
    finally:
        for holder in holders:
            holder.close()


def test_funding_deployment_inputs_readiness_and_transfer(tmp_path, tls, native_configuration):
    config = native_configuration
    data, state, run = paths(tmp_path)
    funding = config['funding']
    raw = bytes.fromhex(funding['raw'][2:])
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == config['evm_chain_id']
    assert transaction.value == funding['amount'] == 1000
    assert transaction.recover_signer(signature) == funding['sender']
    assert '0x' + activity_id(raw).hex() == funding['transaction_id']

    def adopt():
        result, descriptors = receive(run / 'generation.sock', 1)
        try:
            adopted = client.child_environment(result, descriptors, 'core')
            return client.materialize(result, descriptors, 'core', adopted)
        finally:
            client.close_descriptors(descriptors)

    with native_supervisor(config, data, state, run, tmp_path / 'supervisor.log'):
        view, adopted = adopt()
        try:
            assert adopted['LAYERX_CORE_CHAIN_RPC_URL'] == config['chain_rpc_url']
            assert adopted['LAYERX_CORE_SEQUENCER_ENDPOINT'] == config['gateway_endpoint']
            assert adopted['LAYERX_CORE_FUNDING_SENDER_KEY_FILE'] == config['funding_sender_key_file']
            assert 'LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID' not in adopted
            scalar = native.read_regular(adopted['LAYERX_CORE_FUNDING_SENDER_KEY_FILE'], 4096)
            assert scalar.decode() == config['funding_scalar']
            key = ec.derive_private_key(int(scalar, 16), ec.SECP256K1())
            public = key.public_key().public_bytes(serialization.Encoding.X962,
                                                   serialization.PublicFormat.UncompressedPoint)
            from layerx_sdk.account_derivation import _evm_address
            assert _evm_address(public[1:]) == funding['sender']
            ca = native.read_regular(adopted['LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER'])
            assert ca == tls[0].read_bytes()
            # The authenticated hosted origin and the chain origin are consumed
            # through the production JSON-RPC codec, including exact headers.
            keys = native.rpc(adopted['LAYERX_CORE_SEQUENCER_ENDPOINT'], 'px_getReceiptKeys', [], ca)
            assert keys['keys'][0]['publicKey'] == '0x' + config['receipt_public_key']
            assert native.committed_health(transport.environment((data / 'node.env').read_bytes()))['height'] > 0
            chain = adopted['LAYERX_CORE_CHAIN_RPC_URL']
            before = int(native.rpc(chain, 'eth_getBalance', [funding['recipient'], 'latest']), 16)
            identity = native.rpc(chain, 'eth_sendRawTransaction', [funding['raw']])
            assert identity == funding['transaction_id']
            receipt = native.rpc(chain, 'eth_getTransactionReceipt', [identity])
            assert receipt['status'] == '0x1' and int(receipt['blockNumber'], 16) > 0
            assert int(native.rpc(chain, 'eth_getBalance', [funding['recipient'], 'latest']), 16) == before + 1000
        finally:
            view.cleanup()
    # Reopen the real node owner, broker and consumer from the retained state;
    # the chain peer keeps its committed transaction across the process restart.
    with native_supervisor(config, data, state, run, tmp_path / 'supervisor.log'):
        view, adopted = adopt()
        try:
            chain = adopted['LAYERX_CORE_CHAIN_RPC_URL']
            assert native.rpc(chain, 'eth_sendRawTransaction', [funding['raw']]) == identity
            assert native.rpc(chain, 'eth_getTransactionReceipt', [identity]) == receipt
            assert int(native.rpc(chain, 'eth_getBalance', [funding['recipient'], 'latest']), 16) == before + 1000
        finally:
            view.cleanup()
    requests = [json.loads(line) for line in config['peer'].journal.read_text().splitlines()]
    assert any(item.get('tls') is True and item['rpc']['method'] == 'px_getReceiptKeys' for item in requests)
    assert sum(item.get('rpc', {}).get('method') == 'eth_sendRawTransaction' for item in requests) == 2


@pytest.mark.parametrize('fault', ['catching_up', 'canonical', 'chain_id', 'evm_block'])
def test_native_committed_readiness_refuses_divergent_peer(tmp_path, native_configuration, fault):
    config = native_configuration
    data, state, run = paths(tmp_path)
    args = native.parser().parse_args(['supervise', *native_arguments(config, data, state, run)])
    binding = native.bootstrap(args, commands=config['peer'])
    store = staged_store(data, state, run)
    exchanges = config['peer'].configuration['rpc_exchanges']
    if fault == 'catching_up':
        next(item for item in exchanges if item['method'] == 'status')['result']['sync_info']['catching_up'] = True
    elif fault == 'canonical':
        next(item for item in exchanges if item['method'] == 'commit')['result']['canonical'] = False
    elif fault == 'chain_id':
        next(item for item in exchanges if item['method'] == 'eth_chainId')['result'] = '0x1'
    else:
        next(item for item in exchanges if item['method'] == 'eth_getBlockByNumber')['result']['hash'] = '0x' + '00' * 32
    args.ready_seconds = 1
    with pytest.raises(ValueError, match='native paxd committed readiness timed out'):
        native.start_native(args, store, commands=config['peer'])
    assert all(process.poll() is not None for process in config['peer'].children)
    expected = {'generation': 1, 'reset_id': None, 'genesis': binding}
    assert native.activated_ready(data, run, expected) is False
