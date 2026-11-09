"""Kernel image wiring and production committed readiness, without native tools.

The peer replays committed node-owner RPC fixtures. It is not a paxd emulator;
actual paxd compilation, BFT startup and backend recovery remain review/owner
qualification obligations recorded in repair_22_31_427.kvx.
"""

import ast
from contextlib import contextmanager
import copy
from datetime import datetime
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import multiprocessing
import os
from pathlib import Path
import re
import shlex
import sys
import threading
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
KERNEL = ROOT / 'docker/kernel'
NODE_OWNER = ROOT / 'services/platform/hosted/node'
FIXTURES = NODE_OWNER / 'tests/fixtures/repair_22_31_355'
RECORD = Path(__file__).with_name('repair_22_31_427.kvx')
CAPTURE = json.loads((FIXTURES / 'native_configuration.json').read_text())
GENESIS = hashlib.sha256((FIXTURES / 'genesis.json').read_bytes()).hexdigest()
COMMIT_TIME = datetime.fromisoformat(
    CAPTURE['status']['sync_info']['latest_block_time'].replace('Z', '+00:00')
).timestamp()


@pytest.fixture(scope='module')
def runtime():
    # These are the exact node-owner modules COPY installs in the image. Load
    # source directly, without bytecode or a subprocess/native toolchain.
    sys.path.insert(0, str(NODE_OWNER))
    module = importlib.util.module_from_spec(
        importlib.util.spec_from_file_location('kernel_runtime_427', KERNEL / 'kernel_runtime.py')
    )
    old = sys.dont_write_bytecode
    try:
        sys.dont_write_bytecode = True
        module.__spec__.loader.exec_module(module)
    finally:
        sys.dont_write_bytecode = old
        sys.path.remove(str(NODE_OWNER))
    return module


def write_peer(path, exchanges=None, unavailable=False):
    value = {'rpc_exchanges': CAPTURE['rpc_exchanges'] if exchanges is None else exchanges,
             'unavailable': unavailable}
    pending = path.with_suffix('.pending')
    pending.write_text(json.dumps(value))
    pending.replace(path)


def replay_peer(path, trace, pipe, port):
    """A loopback transcript store: admit only captured methods and parameters."""
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            length = int(self.headers.get('Content-Length', '0'))
            request = json.loads(self.rfile.read(length))
            assert self.path == '/'
            assert self.headers['Content-Type'] == 'application/json'
            assert request['jsonrpc'] == '2.0'
            with trace.open('a') as output:
                output.write(json.dumps(request) + '\n')
            captured = json.loads(path.read_text())
            records = [item for item in captured['rpc_exchanges']
                       if item['method'] == request['method'] and item['params'] == request['params']]
            if captured['unavailable'] or len(records) != 1:
                self.send_error(503)
                return
            raw = json.dumps({'jsonrpc': '2.0', 'id': request['id'],
                              'result': records[0]['result']}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *_):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', port), Handler)
    pipe.send(server.server_address[1])
    pipe.close()
    server.serve_forever()


@contextmanager
def peer(path, trace, port=0):
    context = multiprocessing.get_context('fork')
    receiver, sender = context.Pipe(duplex=False)
    process = context.Process(target=replay_peer, args=(path, trace, sender, port))
    process.start()
    sender.close()
    try:
        assert receiver.poll(5), 'transcript peer did not start'
        port = receiver.recv()
        yield process, f'http://127.0.0.1:{port}', port
    finally:
        if process.is_alive():
            process.terminate()
        process.join(timeout=5)
        assert not process.is_alive()
        receiver.close()


def node_at(runtime, origin):
    return {**runtime.NODE, 'LAYERX_NODE_RPC_URL': origin,
            'LAYERX_NODE_CONSENSUS_RPC_URL': origin}


@contextmanager
def consumer(runtime, readiness):
    server = runtime.readiness_server(readiness, ('127.0.0.1', 0))
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server.server_address
    finally:
        server.shutdown()
        thread.join(timeout=5)
        assert not thread.is_alive()
        server.server_close()


def get_ready(address, path='/readyz'):
    connection = http.client.HTTPConnection(*address, timeout=5)
    try:
        connection.request('GET', path)
        response = connection.getresponse()
        assert response.getheader('Content-Type') == 'application/json'
        assert response.getheader('Cache-Control') == 'no-store'
        raw = response.read()
        assert len(raw) == int(response.getheader('Content-Length'))
        return response.status, json.loads(raw)
    finally:
        connection.close()


def test_kernel_backend_cold_start_and_reopened_committed_readiness(runtime, tmp_path):
    transcript, trace = tmp_path / 'rpc.json', tmp_path / 'requests.jsonl'
    checkpoint = tmp_path / 'committed.json'
    write_peer(transcript, unavailable=True)
    with peer(transcript, trace) as (process, origin, port):
        start = runtime.native.process_start(process.pid)
        assert runtime.live_process(process.pid, start)
        assert not runtime.live_process(process.pid, str(int(start) + 1))
        readiness = runtime.CommittedReadiness(
            node_at(runtime, origin), checkpoint, GENESIS, process.pid, clock=lambda: COMMIT_TIME + 1
        )
        with consumer(runtime, readiness) as address:
            assert get_ready(address) == (503, {'ready': False, 'chain_id': 125})
            assert not checkpoint.exists()
            write_peer(transcript)
            expected = {'ready': True, 'chain_id': 125, 'height': 23869863,
                        'block_hash': CAPTURE['status']['sync_info']['latest_block_hash'].lower()}
            assert get_ready(address) == (200, expected)
            retained = checkpoint.read_bytes()
            stored = json.loads(retained)
            assert stored == {'schema': 'layerx.kernel.committed.v1', 'genesis': GENESIS,
                              'height': expected['height'], 'block_hash': expected['block_hash']}
            assert checkpoint.stat().st_mode & 0o777 == 0o600
            assert get_ready(address, '/readyz?cached=1') == (404, {'error': 'not found'})
            process.terminate()
            process.join(timeout=5)
            # A valid retained record cannot keep a dead node ready.
            assert get_ready(address) == (503, {'ready': False, 'chain_id': 125})
            assert checkpoint.read_bytes() == retained
    # Stop and reopen both the peer and production consumer at the same
    # transport origin, retaining only their durable files.
    del readiness
    with peer(transcript, trace, port) as (process, origin, _):
        reopened = runtime.CommittedReadiness(
            node_at(runtime, origin), checkpoint, GENESIS, process.pid, clock=lambda: COMMIT_TIME + 2
        )
        assert reopened.retained == stored
        with consumer(runtime, reopened) as address:
            assert get_ready(address) == (200, expected)
            write_peer(transcript, unavailable=True)
            assert get_ready(address) == (503, {'ready': False, 'chain_id': 125})
            assert checkpoint.read_bytes() == retained
            write_peer(transcript)
            assert get_ready(address) == (200, expected)
    requests = [json.loads(line) for line in trace.read_text().splitlines()]
    methods = [item['method'] for item in requests]
    assert methods[1:6] == ['status', 'commit', 'eth_chainId', 'eth_getBlockByNumber', 'status']
    assert all(item['id'] == 'native-health-' + item['method'] for item in requests)
    assert not list(tmp_path.glob('*.pending'))
    assert not list(tmp_path.glob('.*.pending'))


FAULTS = [
    ('status', ('node_info', 'network'), 'another-chain'),
    ('status', ('sync_info', 'catching_up'), True),
    ('status', ('sync_info', 'latest_block_height'), '0'),
    ('status', ('sync_info', 'latest_block_hash'), ''),
    ('status', ('sync_info', 'last_committed_block_height'), '0'),
    ('status', ('sync_info', 'latest_block_time'), '2000-01-01T00:00:00Z'),
    ('commit', ('canonical',), False),
    ('commit', ('signed_header', 'header', 'chain_id'), 'another-chain'),
    ('commit', ('signed_header', 'header', 'height'), '1'),
    ('commit', ('signed_header', 'header', 'app_hash'), ''),
    ('commit', ('signed_header', 'commit', 'signatures'), []),
    ('commit', ('signed_header', 'commit', 'block_id', 'hash'), '00' * 32),
    ('eth_chainId', (), '0x1'),
    ('eth_getBlockByNumber', ('number',), '0x0'),
    ('eth_getBlockByNumber', ('hash',), '0x' + '00' * 32),
]


@pytest.mark.parametrize('method,keys,value', FAULTS, ids=[f'{m}-{"-".join(k)}' for m, k, _ in FAULTS])
def test_kernel_backend_refuses_uncommitted_or_divergent_peers(runtime, tmp_path, method, keys, value):
    transcript, trace = tmp_path / 'rpc.json', tmp_path / 'requests.jsonl'
    exchanges = copy.deepcopy(CAPTURE['rpc_exchanges'])
    item = next(item for item in exchanges if item['method'] == method)
    if keys:
        target = item['result']
        for key in keys[:-1]:
            target = target[key]
        target[keys[-1]] = value
    else:
        item['result'] = value
    write_peer(transcript, exchanges)
    with peer(transcript, trace) as (process, origin, _):
        readiness = runtime.CommittedReadiness(
            node_at(runtime, origin), tmp_path / 'committed.json', GENESIS, process.pid,
            clock=lambda: COMMIT_TIME + 1,
        )
        with consumer(runtime, readiness) as address:
            assert get_ready(address) == (503, {'ready': False, 'chain_id': 125})
        assert not readiness.checkpoint.exists()


@pytest.mark.parametrize('fault', ['rollback', 'same-height-divergence', 'stale', 'future'])
def test_kernel_backend_preserves_recovery_floor_on_refusal(runtime, tmp_path, fault):
    transcript, trace, checkpoint = tmp_path / 'rpc.json', tmp_path / 'requests.jsonl', tmp_path / 'floor.json'
    write_peer(transcript)
    floor = {'schema': 'layerx.kernel.committed.v1', 'genesis': GENESIS,
             'height': 23869863 + (1 if fault == 'rollback' else 0),
             'block_hash': ('00' * 32 if fault == 'same-height-divergence' else
                            CAPTURE['status']['sync_info']['latest_block_hash'].lower())}
    checkpoint.write_text(json.dumps(floor))
    checkpoint.chmod(0o600)
    before = checkpoint.read_bytes()
    now = COMMIT_TIME + (61 if fault == 'stale' else -1 if fault == 'future' else 1)
    with peer(transcript, trace) as (process, origin, _):
        readiness = runtime.CommittedReadiness(
            node_at(runtime, origin), checkpoint, GENESIS, process.pid, clock=lambda: now
        )
        assert readiness.probe() == (503, {'ready': False, 'chain_id': 125})
        assert checkpoint.read_bytes() == before


@pytest.mark.parametrize('fault', ['genesis', 'malformed', 'symlink', 'mode'])
def test_kernel_backend_refuses_untrusted_recovery_record(runtime, tmp_path, fault):
    checkpoint = tmp_path / 'floor.json'
    value = {'schema': 'layerx.kernel.committed.v1', 'genesis': '00' * 32,
             'height': 23869863, 'block_hash': '11' * 32}
    checkpoint.write_text(json.dumps(value) if fault != 'malformed' else '{')
    checkpoint.chmod(0o644 if fault == 'mode' else 0o600)
    if fault == 'symlink':
        original = tmp_path / 'original'
        checkpoint.rename(original)
        checkpoint.symlink_to(original)
    with pytest.raises((OSError, ValueError, runtime.StoreError)):
        runtime.CommittedReadiness(runtime.NODE, checkpoint, GENESIS, os.getpid())


@pytest.fixture
def home(tmp_path):
    root = tmp_path / 'node'
    root.mkdir(mode=0o700)
    for name in ('config', 'data'):
        (root / name).mkdir(mode=0o700)
    for name in ('genesis.json', 'config.toml', 'app.toml'):
        (root / 'config' / name).write_bytes((FIXTURES / name).read_bytes())
    # Use the committed node-owner private-key and initial signing-state
    # contracts; never ask paxd or another external binary to generate a peer.
    (root / 'config/priv_validator_key.json').write_text(json.dumps(CAPTURE['validator_key']))
    (root / 'config/node_key.json').write_text(json.dumps({'priv_key': CAPTURE['validator_key']['priv_key']}))
    (root / 'data/priv_validator_state.json').write_text(CAPTURE['init_files']['data/priv_validator_state.json'])
    (root / 'config/receipt.key').write_text(CAPTURE['receipt_seed'] + '\n')
    for name in ('config/priv_validator_key.json', 'config/node_key.json',
                 'data/priv_validator_state.json', 'config/receipt.key'):
        (root / name).chmod(0o600)
    return root


def test_kernel_backend_native_home_and_launch_contract(runtime, home):
    assert runtime.validate_home(home) == GENESIS
    command = runtime.node_command(home)
    assert command[:2] == ['/usr/local/bin/paxd', 'start']
    arguments = dict(zip(command[2::2], command[3::2], strict=True))
    assert arguments == {
        '--home': str(home), '--chain-id': 'hyperpax_125-1',
        '--rpc.laddr': 'tcp://127.0.0.1:26657',
        '--evm.http_enabled': 'true', '--evm.http_address': '0.0.0.0',
        '--evm.http_port': '8545', '--evm.ws_enabled': 'true', '--evm.ws_port': '8546',
        '--evm.enable_test_api': 'false', '--evm.enable_unsafe_keyring_rpc': 'false',
        '--sequencer.enable': 'true', '--sequencer.wal-dir': str(home / 'data/sequencer-wal'),
        '--sequencer.receipt-key-file': str(home / 'config/receipt.key'),
    }
    flag_sources = (ROOT / 'rpc/config/config.go').read_text() + (ROOT / 'node/sequencer/config.go').read_text()
    for flag in arguments:
        if flag.startswith(('--evm.', '--sequencer.')):
            assert '"' + flag[2:] + '"' in flag_sources
    source = (KERNEL / 'kernel_runtime.py').read_text()
    assert source.index('fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)') < source.index('genesis = validate_home(home)')
    assert source.index("native.run_native([PAXD, 'validate-genesis'") < source.index('child = native.spawn_child(node_command(home))')
    assert 'native.stop_process(child)' in source
    assert 'server.shutdown()' in source and 'os.close(lock)' in source
    assert 'signal.SIGTERM, signal.SIGINT' in source
    # The owning spawn path installs PR_SET_PDEATHSIG before exec; no copied
    # retired supervisor or alternative executable can own the node.
    reset = (NODE_OWNER / 'reset_state.py').read_text()
    assert 'libc.prctl(1, signal.SIGTERM' in reset
    native = (NODE_OWNER / 'native_runtime.py').read_text()
    assert "'exec-daemon', '--', *command" in native


@pytest.mark.parametrize('fault', ['missing-key', 'public-key', 'empty-key', 'zero-receipt',
                                  'wrong-chain', 'missing-host', 'old-daemon', 'symlink', 'external-db', 'external-sign-state'])
def test_kernel_backend_refuses_invalid_native_home(runtime, home, fault):
    if fault == 'missing-key':
        (home / 'config/node_key.json').unlink()
    elif fault == 'public-key':
        (home / 'config/receipt.key').chmod(0o644)
    elif fault == 'empty-key':
        (home / 'config/priv_validator_key.json').write_text('')
    elif fault == 'zero-receipt':
        (home / 'config/receipt.key').write_text('0' * 64)
    elif fault in ('wrong-chain', 'missing-host'):
        path = home / 'config/genesis.json'
        genesis = json.loads(path.read_text())
        if fault == 'wrong-chain':
            genesis['chain_id'] = 'other'
        else:
            del genesis['app_state']['lxphost']
        path.write_text(json.dumps(genesis))
    elif fault in ('external-db', 'external-sign-state'):
        path = home / 'config/config.toml'
        source = path.read_text()
        source = source.replace('db-dir = "data"', 'db-dir = "/tmp/other"') if fault == 'external-db' else source.replace('state-file = "data/priv_validator_state.json"', 'state-file = "/tmp/other"')
        path.write_text(source)
    elif fault == 'old-daemon':
        (home / 'config/genesis.json').unlink()
        (home / 'metadata.lxgb').write_bytes(b'old daemon state')
    else:
        path = home / 'config/receipt.key'
        path.rename(home / 'receipt-original')
        path.symlink_to(home / 'receipt-original')
    before = sorted(str(path.relative_to(home)) for path in home.rglob('*'))
    with pytest.raises((OSError, ValueError, KeyError, runtime.StoreError)):
        runtime.validate_home(home)
    assert sorted(str(path.relative_to(home)) for path in home.rglob('*')) == before


def docker_instructions():
    """Read Docker continuations and heredocs without treating Python as RUN."""
    lines = iter((KERNEL / 'Dockerfile').read_text().splitlines())
    instructions = []
    for line in lines:
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        while line.endswith('\\'):
            line = line[:-1] + next(lines).lstrip()
        heredoc = re.search(r"<<'([A-Z_]+)'", line)
        if heredoc:
            body = []
            for item in lines:
                if item == heredoc[1]:
                    break
                body.append(item)
            else:
                raise AssertionError('unterminated Docker heredoc')
            instructions.append((line.split(None, 1)[0], line.split(None, 1)[1], '\n'.join(body)))
        else:
            opcode, value = line.split(None, 1)
            instructions.append((opcode, value, None))
    return instructions


def test_kernel_backend_image_build_transport_and_typed_consumers():
    instructions = docker_instructions()
    stages = [shlex.split(value) for opcode, value, _ in instructions if opcode == 'FROM']
    names = [parts[-1] for parts in stages]
    assert names == ['build', 'paxd-build', 'native-manifest', 'native-artifacts', 'native-runtime', 'full-runtime']
    assert all('@sha256:' in parts[0] for parts in stages if ':' in parts[0])
    assert stages[2][0] == 'paxd-build' and stages[-1][0] == 'native-runtime'
    runs = '\n'.join(value for opcode, value, _ in instructions if opcode == 'RUN')
    assert 'make LXP_REVISION="$LXP_REVISION" paxd-native-libs' in runs
    assert 'go build -mod=readonly' in runs and '-o build/paxd ./daemon/paxd' in runs
    assert 'CGO_ENABLED=1' in (KERNEL / 'Dockerfile').read_text()
    assert "go list -mod=readonly -tags 'netgo ledger' -deps -json ./daemon/paxd" in runs
    assert 'test "${#expected}" -eq 64' in runs
    assert runs.count('sha256sum --check --strict') == 2
    entries = [json.loads(value) for opcode, value, _ in instructions if opcode == 'ENTRYPOINT']
    assert entries == [['/usr/local/bin/kernel-init'], ['/usr/local/bin/kernel-init']]
    copies = [shlex.split(value) for opcode, value, _ in instructions if opcode == 'COPY']
    assert ['--from=native-artifacts', '--chmod=0555', '/bin/paxd', '/usr/local/bin/paxd'] in copies
    for path in ('native_runtime.py', 'data_directory.py', 'reset_state.py'):
        assert any(f'services/platform/hosted/node/{path}' in parts and parts[-1] == '/opt/layerx/node/' for parts in copies)
    assert ['docker/kernel/kernel_runtime.py', '/opt/layerx/kernel_runtime.py'] in copies
    assert ['--chmod=0555', 'docker/kernel/init.sh', '/usr/local/bin/kernel-init'] in copies
    combined = '\n'.join(value for _, value, _ in instructions)
    for retired in ('layerx-core-boundary', 'layerx-receipt-authority', 'layerx-agent-boundary',
                    'supervisor.sh', 'generation_client.py', 'layerxd', 'bootstrap.sh'):
        assert retired not in combined
    ignore = (KERNEL / 'Dockerfile.dockerignore').read_text().splitlines()
    for path in ('docker/kernel/init.sh', 'docker/kernel/kernel_runtime.py',
                 'services/platform/hosted/node/native_runtime.py',
                 'services/platform/hosted/node/data_directory.py',
                 'services/platform/hosted/node/reset_state.py'):
        assert '!' + path in ignore and (ROOT / path).is_file()
    for needed in ('daemon', 'node', 'rpc', 'precompiles', 'wasm', 'wasm-runtime', 'modules', 'core'):
        assert needed not in ignore
    assert '**/*.key' in ignore and '**/*.seed' in ignore
    # Trace the real merged node, rather than merely asserting a paxd filename.
    app = (ROOT / 'node/app.go').read_text()
    assert 'app.LayerXHost, err = NewHost(' in app
    for keeper in ('layerxledgerkeeper.NewKeeper', 'layerxauthkeeper.NewKeeper',
                   'layerxagentkeeper.NewKeeper', 'layerxpaykeeper.NewKeeper'):
        assert keeper in app
    assert 'host:   app.LayerXHost' in app
    assert 'CGO_ENABLED=1' in (KERNEL / 'Dockerfile').read_text()
    host = (ROOT / 'modules/lxphost/host.go').read_text()
    assert 'import "C"' in host


def test_kernel_backend_artifact_admission_still_checks_candidate_sources():
    instructions = docker_instructions()
    bodies = {re.search(r"<<'([A-Z_]+)'", value)[1]: body
              for _, value, body in instructions if body is not None}
    assert set(bodies) == {'PY_GO_INPUTS', 'PY_MANIFEST', 'PY_VERIFY'}
    for source in bodies.values():
        assert isinstance(ast.parse(source), ast.Module)
    producer, admission = bodies['PY_MANIFEST'], bodies['PY_VERIFY']
    assert '"paxd": ("build/paxd", c_deps,' in producer
    assert 'src + "/build/paxd-source-inputs.txt"' in producer
    assert '"source_inputs": dict(sorted(closure.items()))' in producer
    assert 'hashlib.file_digest(stream, "sha256")' in producer
    assert 'src + "/programs/target/release/liblayerx_programs_sandbox.d"' in producer
    assert 'glob.glob(src + "/build/obj/**/*.d", recursive=True)' in producer
    assert 'required = {"paxd"}' in admission
    for check in ('set(artifacts) != required', 'manifest.get("candidate_revision") != revision',
                  'actual != expected', 'verified_sources[relative] != digest',
                  'source.is_symlink()', 'not source.resolve().is_relative_to(source_root)',
                  'admit_runtime_linkage(executable)', 'duplicate JSON field',
                  'ELF program-header table is outside the file',
                  'ELF interpreter differs from the pinned runtime loader',
                  'runtime dependency mismatch for '):
        assert check in admission
    for path in ('wasm-runtime/internal/api/libwasmvm.x86_64.so',
                 'wasm/x/wasm/artifacts/v152/api/libwasmvm152.x86_64.so',
                 'wasm/x/wasm/artifacts/v155/api/libwasmvm155.x86_64.so'):
        assert path in producer
    for field in ('GoFiles', 'CgoFiles', 'CFiles', 'CXXFiles', 'HFiles', 'SFiles', 'SysoFiles', 'EmbedFiles'):
        assert '"' + field + '"' in bodies['PY_GO_INPUTS']


def test_kernel_backend_entrypoint_unit_and_environment_agree():
    source = (KERNEL / 'init.sh').read_text()
    assert 'set -euo pipefail' in source and 'umask 077' in source
    assert 'full|native)' in source
    assert '[ "${LAYERX_NODE_EVM_CHAIN_ID-125}" = 125 ] &&' in source
    assert '[ "${LAYERX_NODE_PAXEER_CHAIN_ID-125}" = 125 ] || {' in source
    assert 'node_home=${LAYERX_KERNEL_NODE_HOME:-/data/layerx/node}' in source
    command = shlex.split(source[source.index('exec setpriv'):].replace('\\\n', ' '))
    assert command == ['exec', 'setpriv', '--reuid=4020', '--regid=4020', '--clear-groups',
                       '--no-new-privs', 'python3', '/opt/layerx/kernel_runtime.py', '--home', '$node_home']
    for retired in ('generation.sock', 'core.env', '9401', '9402', 'supervisor.sh', 'layerxd',
                    'LAYERX_KERNEL_PAXEER_RPC_NAMES', 'kernel-genesis.sh'):
        assert retired not in source
    sections, section = {}, None
    for line in (KERNEL / 'layerx-kernel.service').read_text().splitlines():
        if line.startswith('['):
            section = line[1:-1]
            sections[section] = {}
        elif '=' in line:
            key, value = line.split('=', 1)
            sections[section].setdefault(key, []).append(value)
    service = sections['Service']
    command = shlex.split(service['ExecStart'][0])
    assert command[:3] == ['/usr/bin/docker', 'run', '--rm']
    assert '--privileged' not in command
    assert '-v' in command and '/var/lib/layerx/kernel:/data' in command
    ports = [command[i + 1] for i, arg in enumerate(command) if arg == '-p']
    assert ports == ['127.0.0.1:8545:8545', '127.0.0.1:8546:8546', '26656:26656', '127.0.0.1:9460:9460']
    assert service['Restart'] == ['always'] and service['TimeoutStopSec'] == ['70']
    assert service['ExecStop'] == ['/usr/bin/docker stop -t 60 layerx-kernel']
    assert '/usr/bin/test -s /var/lib/layerx/kernel/layerx/node/config/genesis.json' in service['ExecStartPre']
    environment = dict(line.split('=', 1) for line in (KERNEL / 'kernel.env.example').read_text().splitlines()
                       if line and not line.startswith('#'))
    assert environment['LAYERX_KERNEL_NODE_HOME'] == '/data/layerx/node'
    assert environment['LAYERX_NODE_NETWORK_ID'] == '125'
    assert environment['LAYERX_NODE_EVM_CHAIN_ID'] == '125'
    assert environment['LAYERX_KERNEL_PROFILE'] == 'native'
    assert not any('TOKEN' in key or 'RPC_NAMES' in key for key in environment)
    assert all(not line or line.startswith('#') for line in (KERNEL / 'files.tsv').read_text().splitlines())


def test_kernel_backend_record_names_proofs_limits_and_owner_requests():
    record = tomllib.loads(RECORD.read_text())
    task = record['task']['22']['31']['427']
    assert task['audit'] == '22.30.179'
    assert task['finding'] == 'kernel-backend'
    functions = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    assert set(task['proofs']) <= functions and len(task['proofs']) >= 7
    assert 'No pre-change verification' in task['baseline']
    assert 'actual paxd' in task['review_only']
    assert 'hf-done' in task['evidence'] and 'candidate revision' in task['evidence']
    assert any('services/platform/hosted/core' in item for item in task['wiring_requests'])
    assert any('Autobahn' in item for item in task['wiring_requests'])
    assert any('actual paxd' in item for item in task['wiring_requests'])
