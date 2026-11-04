"""Medium kernel findings against the merged production assembly.

The native node-owner transcript transport is shared with the committed backend
repair. No native executable, shell subprocess, Rust loader or live service is
used. Bash syntax is checked by hf-build, before this module's source checks.
"""

import ast
from datetime import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
KERNEL = ROOT / 'docker/kernel'
NODE = ROOT / 'services/platform/hosted/node'
FIXTURES = NODE / 'tests/fixtures/repair_22_31_355'
CAPTURE = json.loads((FIXTURES / 'native_configuration.json').read_text())
GENESIS = hashlib.sha256((FIXTURES / 'genesis.json').read_bytes()).hexdigest()
COMMIT_TIME = datetime.fromisoformat(
    CAPTURE['status']['sync_info']['latest_block_time'].replace('Z', '+00:00')
).timestamp()
PUBLIC_NAMES = tuple(f'api{i}.mainnet-beta.paxeer.network' for i in range(1, 15)) + (
    'api-mainnet-beta.paxeer.network',
)
RECORD = Path(__file__).with_name('repair_22_33_814.kvx')


def load_source(name, path):
    module = importlib.util.module_from_spec(importlib.util.spec_from_file_location(name, path))
    previous_path, previous_bytecode = sys.path[:], sys.dont_write_bytecode
    try:
        sys.dont_write_bytecode = True
        sys.path.insert(0, str(NODE))
        module.__spec__.loader.exec_module(module)
    finally:
        sys.path[:] = previous_path
        sys.dont_write_bytecode = previous_bytecode
    return module


@pytest.fixture(scope='module')
def runtime():
    module = load_source('kernel_runtime_814', KERNEL / 'kernel_runtime.py')
    assert Path(module.native.__file__).resolve() == NODE / 'native_runtime.py'
    return module


@pytest.fixture(scope='module')
def transport():
    # Reuse only the already committed recorded-peer/HTTP transport helpers,
    # never another test function or a replacement production implementation.
    return load_source('kernel_transcript_814', Path(__file__).with_name('test_repair_22_31_427.py'))


@pytest.fixture
def native_home(tmp_path):
    home = tmp_path / 'node'
    home.mkdir(mode=0o700)
    for name in ('config', 'data'):
        (home / name).mkdir(mode=0o700)
    for name in ('genesis.json', 'config.toml', 'app.toml'):
        (home / 'config' / name).write_bytes((FIXTURES / name).read_bytes())
    files = {
        'config/priv_validator_key.json': json.dumps(CAPTURE['validator_key']),
        'config/node_key.json': json.dumps({'priv_key': CAPTURE['validator_key']['priv_key']}),
        'data/priv_validator_state.json': CAPTURE['init_files']['data/priv_validator_state.json'],
        'config/receipt.key': CAPTURE['receipt_seed'] + '\n',
    }
    for name, value in files.items():
        (home / name).write_text(value)
        (home / name).chmod(0o600)
    return home


def tree_snapshot(home):
    snapshot = {}
    for path in sorted(home.rglob('*')):
        mode = path.lstat().st_mode
        value = os.readlink(path) if path.is_symlink() else path.read_bytes() if path.is_file() else None
        snapshot[str(path.relative_to(home))] = (mode, value)
    return snapshot


def docker_instructions():
    lines = iter((KERNEL / 'Dockerfile').read_text().splitlines())
    instructions = []
    for line in lines:
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        while line.endswith('\\'):
            line = line[:-1] + next(lines).lstrip()
        opcode, value = line.split(None, 1)
        delimiter = re.search(r"<<'([A-Z_]+)'", value)
        body = None
        if delimiter:
            content = []
            for item in lines:
                if item == delimiter[1]:
                    break
                content.append(item)
            else:
                raise AssertionError('unterminated Docker heredoc')
            body = '\n'.join(content)
            ast.parse(body)
        instructions.append((opcode, value, body))
    return instructions


def test_kernel_image_retired_assembly_current_dependency_closure():
    instructions = docker_instructions()
    stages = [shlex.split(value) for op, value, _ in instructions if op == 'FROM']
    assert [stage[-1] for stage in stages] == [
        'build', 'paxd-build', 'native-manifest', 'native-artifacts', 'native-runtime', 'full-runtime',
    ]
    assert stages[-1][0] == 'native-runtime'
    assert all('@sha256:' in stage[0] for stage in stages if ':' in stage[0])
    runs = '\n'.join(value for op, value, _ in instructions if op == 'RUN')
    assert 'make LXP_REVISION="$LXP_REVISION" paxd-native-libs' in runs
    assert 'go build -mod=readonly' in runs and '-o build/paxd ./daemon/paxd' in runs
    assert 'CGO_ENABLED=1' in (KERNEL / 'Dockerfile').read_text()
    assert runs.count('sha256sum --check --strict') == 2
    copies = [shlex.split(value) for op, value, _ in instructions if op == 'COPY']
    assert ['--from=native-artifacts', '--chmod=0555', '/bin/paxd', '/usr/local/bin/paxd'] in copies
    assert ['docker/kernel/kernel_runtime.py', '/opt/layerx/kernel_runtime.py'] in copies
    assert ['--chmod=0555', 'docker/kernel/init.sh', '/usr/local/bin/kernel-init'] in copies
    for filename in ('native_runtime.py', 'data_directory.py', 'reset_state.py'):
        assert any(f'services/platform/hosted/node/{filename}' in parts
                   and parts[-1] == '/opt/layerx/node/' for parts in copies)
    assert [json.loads(value) for op, value, _ in instructions if op == 'ENTRYPOINT'] == [
        ['/usr/local/bin/kernel-init'], ['/usr/local/bin/kernel-init'],
    ]
    active = '\n'.join(value for _, value, _ in instructions)
    for retired in ('bootstrap.sh', 'supervisor.sh', 'generation_client.py',
                    'generation_transport.py', 'layerxd', 'layerx-receipt-authority',
                    'layerx-human-kms-gateway', 'layerx-core-boundary'):
        assert retired not in active
    ignore = (KERNEL / 'Dockerfile.dockerignore').read_text().splitlines()
    for source in ('docker/kernel/init.sh', 'docker/kernel/kernel_runtime.py',
                   'services/platform/hosted/node/native_runtime.py',
                   'services/platform/hosted/node/data_directory.py',
                   'services/platform/hosted/node/reset_state.py'):
        assert '!' + source in ignore and (ROOT / source).is_file()
    for pattern in ('**/*.key', '**/*.seed', '**/.env', '**/secrets/**'):
        assert pattern in ignore
    assert all(not line or line.startswith('#') for line in (KERNEL / 'files.tsv').read_text().splitlines())
    # Real consumer of the linked kernel, rather than a binary filename alone.
    app = (ROOT / 'node/app.go').read_text()
    assert 'app.LayerXHost, err = NewHost(' in app
    for keeper in ('layerxledgerkeeper', 'layerxauthkeeper', 'layerxpaykeeper', 'layerxagentkeeper'):
        assert keeper + '.NewKeeper' in app
    assert 'import "C"' in (ROOT / 'modules/lxphost/host.go').read_text()


@pytest.mark.parametrize('legacy', [
    'metadata.lxgb', 'settlement.env', 'guarantor-1', 'guarantor-2', 'guarantor-submitter',
])
def test_kernel_retired_state_offline_migration_preserves_volume(runtime, native_home, legacy):
    marker = native_home / legacy
    if legacy.startswith('guarantor'):
        marker.mkdir(mode=0o700)
        (marker / 'retained').write_bytes(b'durable legacy data\x00')
    else:
        marker.write_bytes(b'durable legacy data\x00')
    before = tree_snapshot(native_home)
    with pytest.raises(ValueError, match='offline migration.*preserve the old volume'):
        runtime.validate_home(native_home)
    assert tree_snapshot(native_home) == before
    # Reopening after a refused attempt must neither adopt nor erase old data.
    reopened = load_source('kernel_runtime_814_reopened', KERNEL / 'kernel_runtime.py')
    with pytest.raises(ValueError, match='offline migration'):
        reopened.validate_home(native_home)
    assert tree_snapshot(native_home) == before


def test_kernel_retired_state_archived_sibling_is_not_consumed(runtime, native_home):
    legacy = native_home.parent / 'archived-legacy'
    legacy.mkdir(mode=0o700)
    (legacy / 'settlement.env').write_bytes(b'preserve offline\n')
    before = tree_snapshot(native_home.parent)
    assert runtime.validate_home(native_home) == GENESIS
    assert runtime.validate_home(native_home) == GENESIS
    assert tree_snapshot(native_home.parent) == before
    command = runtime.node_command(native_home)
    assert command[:2] == ['/usr/local/bin/paxd', 'start']
    flags = dict(zip(command[2::2], command[3::2], strict=True))
    assert flags['--home'] == str(native_home)
    assert flags['--chain-id'] == CAPTURE['chain_id']
    assert flags['--sequencer.wal-dir'] == str(native_home / 'data/sequencer-wal')
    assert flags['--sequencer.receipt-key-file'] == str(native_home / 'config/receipt.key')
    assert not any(str(legacy) in item for item in command)


def test_kernel_retired_state_committed_identity_survives_readiness_restart(runtime, transport, tmp_path):
    transcript, trace = tmp_path / 'rpc.json', tmp_path / 'requests.jsonl'
    checkpoint = tmp_path / 'committed.json'
    transport.write_peer(transcript)
    with transport.peer(transcript, trace) as (process, origin, port):
        node = transport.node_at(runtime, origin)
        readiness = runtime.CommittedReadiness(
            node, checkpoint, GENESIS, process.pid, clock=lambda: COMMIT_TIME + 1,
        )
        with transport.consumer(runtime, readiness) as address:
            code, value = transport.get_ready(address)
            assert code == 200
            assert value == {
                'ready': True, 'chain_id': CAPTURE['evm_chain_id'],
                'height': int(CAPTURE['status']['sync_info']['latest_block_height']),
                'block_hash': CAPTURE['status']['sync_info']['latest_block_hash'].lower(),
            }
            durable = checkpoint.read_bytes()
            process.terminate()
            process.join(timeout=5)
            assert transport.get_ready(address) == (503, {'ready': False, 'chain_id': 125})
            assert checkpoint.read_bytes() == durable
    del readiness
    # Reopen the actual production checkpoint consumer and loopback transcript
    # peer; retained state cannot substitute for fresh canonical chain reads.
    with transport.peer(transcript, trace, port) as (process, origin, _):
        reopened = runtime.CommittedReadiness(
            transport.node_at(runtime, origin), checkpoint, GENESIS, process.pid,
            clock=lambda: COMMIT_TIME + 2,
        )
        assert reopened.retained == json.loads(durable)
        with transport.consumer(runtime, reopened) as address:
            assert transport.get_ready(address) == (200, value)
            transport.write_peer(transcript, unavailable=True)
            assert transport.get_ready(address) == (503, {'ready': False, 'chain_id': 125})
            assert checkpoint.read_bytes() == durable
    methods = [json.loads(line)['method'] for line in trace.read_text().splitlines()]
    assert methods[:5] == ['status', 'commit', 'eth_chainId', 'eth_getBlockByNumber', 'status']
    assert methods.count('commit') == 2 and methods.count('eth_chainId') == 2


@pytest.mark.parametrize('name', PUBLIC_NAMES + (
    'api0.mainnet-beta.paxeer.network', 'api15.mainnet-beta.paxeer.network',
    'api16.mainnet-beta.paxeer.network', 'api01.mainnet-beta.paxeer.network',
    'api1.mainnet-beta.paxeer.network.evil.invalid',
))
def test_kernel_rpc_hosts_public_names_cannot_replace_local_committed_node(runtime, name):
    # Public selection was retired by the prerequisite, including its old
    # allowlist. Every public name is refused as a local readiness authority.
    for url in (f'http://{name}:8545', f'https://{name}:443/'):
        with pytest.raises(ValueError):
            runtime.native.endpoint(url, loopback=True)
    node = {**runtime.NODE, 'LAYERX_NODE_RPC_URL': f'http://{name}:8545'}
    with pytest.raises(ValueError):
        runtime.CommittedReadiness(node, Path('/unopened/checkpoint'), GENESIS, os.getpid())


@pytest.mark.parametrize('url', [
    'http://127.0.0.1:8545/evm', 'https://127.0.0.1:8545/',
    'http://user@127.0.0.1:8545/', 'http://127.0.0.1:8545/?cached=1',
    'http://127.0.0.1:8545/#head', 'http://127.0.0.1/',
])
def test_kernel_rpc_hosts_refuses_nonroot_or_ambiguous_local_origin(runtime, url):
    with pytest.raises(ValueError):
        runtime.native.endpoint(url, loopback=True)


def test_kernel_rpc_hosts_native_acceptance_and_public_owner_policy(runtime):
    assert len(PUBLIC_NAMES) == len(set(PUBLIC_NAMES)) == 15
    for key, port in (('LAYERX_NODE_RPC_URL', 8545), ('LAYERX_NODE_CONSENSUS_RPC_URL', 26657)):
        parsed = runtime.native.endpoint(runtime.NODE[key], loopback=True)
        assert parsed.hostname == '127.0.0.1' and parsed.port == port and parsed.path == ''
    init = (KERNEL / 'init.sh').read_text()
    assert 'LAYERX_KERNEL_PAXEER_RPC_NAMES' not in init
    assert 'LAYERX_KERNEL_PAXEER_CA_DER' not in init
    documentation = (KERNEL / 'kernel.env.example').read_text()
    for fact in ('api1.mainnet-beta.paxeer.network', 'api14.mainnet-beta.paxeer.network',
                 'api-mainnet-beta.paxeer.network', 'api15/api16 are retired',
                 'HTTPS at /', 'TLS name/CA', 'distinct peers'):
        assert fact in documentation
    # The retained shared public transport still enables authenticated TLS;
    # the kernel does not invoke it or select public peers for readiness.
    native = (NODE / 'native_runtime.py').read_text()
    tree = ast.parse(native)
    rpc = next(item for item in tree.body if isinstance(item, ast.FunctionDef) and item.name == 'rpc')
    rpc_source = ast.get_source_segment(native, rpc)
    for fact in ('ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)', 'context.load_verify_locations(',
                 'http.client.HTTPSConnection(', 'context=context'):
        assert fact in rpc_source


@pytest.mark.parametrize('profile', ['native', 'full'])
def test_kernel_native_network_name_default_inputs(runtime, native_home, profile):
    environment = dict(line.split('=', 1) for line in (KERNEL / 'kernel.env.example').read_text().splitlines()
                       if line and not line.startswith('#'))
    environment['LAYERX_KERNEL_PROFILE'] = profile
    assert 'LAYERX_NODE_NETWORK_NAME' not in environment
    runtime.validate_chain_environment(environment)
    assert runtime.validate_home(native_home) == GENESIS
    assert runtime.node_command(native_home)[0] == '/usr/local/bin/paxd'
    init = (KERNEL / 'init.sh').read_text()
    assert 'full|native) ;;' in init
    for obsolete in ('network_name', 'LAYERX_NODE_NETWORK_NAME', 'receipt_authority_prepare',
                     'agent_boundary_prepare', 'generation.sock', 'core.env'):
        assert obsolete not in init
    command = shlex.split(init[init.index('exec setpriv'):].replace('\\\n', ' '))
    assert command == [
        'exec', 'setpriv', '--reuid=4020', '--regid=4020', '--clear-groups', '--no-new-privs',
        'python3', '/opt/layerx/kernel_runtime.py', '--home', '$node_home',
    ]
    assert 'set -euo pipefail' in init and 'umask 077' in init
    assert init.index('full|native) ;;') < init.index('exec setpriv')
    assert '/data/*) ;;' in init
    assert 'node home must be on /data' in init
    assert init.index('node home must be on /data') < init.index('exec setpriv')


def test_kernel_kms_chain_selection_domain_is_not_evm_identity(runtime):
    assert CAPTURE['network_id'] != CAPTURE['evm_chain_id'] == 125
    environment = {
        'LAYERX_NODE_NETWORK_ID': str(CAPTURE['network_id']),
        'LAYERX_NODE_EVM_CHAIN_ID': str(CAPTURE['evm_chain_id']),
        'LAYERX_NODE_PAXEER_CHAIN_ID': str(CAPTURE['evm_chain_id']),
    }
    before = dict(environment)
    runtime.validate_chain_environment(environment)
    assert environment == before  # Preserve domain policy material verbatim.
    assert runtime.NODE['LAYERX_NODE_EVM_CHAIN_ID'] == environment['LAYERX_NODE_EVM_CHAIN_ID']
    init = (KERNEL / 'init.sh').read_text()
    assert '${LAYERX_NODE_NETWORK_ID' not in init
    for key in ('LAYERX_NODE_EVM_CHAIN_ID', 'LAYERX_NODE_PAXEER_CHAIN_ID'):
        assert '"${' + key + '-125}" = 125' in init
    source = (KERNEL / 'kernel_runtime.py').read_text()
    main = next(node for node in ast.parse(source).body if isinstance(node, ast.FunctionDef) and node.name == 'main')
    main_source = ast.get_source_segment(source, main)
    assert main_source.index('validate_chain_environment(os.environ)') < main_source.index('supervise(args.home)')
    for retired in ('human_kms_chain_snapshot', 'human_kms_prepare', 'LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE'):
        assert retired not in init and retired not in source
    # Source contract only: no Rust compiler/loader execution is claimed.
    loader = (ROOT / 'services/human/crates/layerx-human-service/src/bin/layerx-human-kms-gateway.rs').read_text()
    assert '#[serde(deny_unknown_fields)]\nstruct ChainSnapshot {\n    chain_id: u32,\n}' in loader
    assert 'serde_json::from_slice(&material("REGISTRY_FILE", false)?)' in loader
    assert 'if snapshot.chain_id != 125 {' in loader
    assert 'network_id: snapshot.chain_id,' in loader


@pytest.mark.parametrize('key', ['LAYERX_NODE_EVM_CHAIN_ID', 'LAYERX_NODE_PAXEER_CHAIN_ID'])
@pytest.mark.parametrize('value', ['', '402', '713714', '0x7d', '0125', '125\n'])
def test_kernel_kms_chain_selection_refuses_wrong_evm_input(runtime, key, value):
    environment = {'LAYERX_NODE_NETWORK_ID': '402', key: value}
    with pytest.raises(ValueError, match=key + ' must be 125'):
        runtime.validate_chain_environment(environment)


def test_kernel_task_record_finding_proofs_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text())
    task = record['task']['22']['33']['814']
    assert task['audit'] == '22.30.179'
    functions = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    expected = {
        'kernel-image-retired-assembly', 'kernel-retired-state', 'kernel-rpc-hosts',
        'kernel-native-network-name', 'kernel-kms-chain-selection',
    }
    assert set(record['finding']) == expected
    for finding in record['finding'].values():
        assert set(finding['tests']) <= functions and finding['tests']
        assert finding['closure_boundary']
    assert 'No pre-change verification' in task['baseline']
    assert 'Rust KMS loader' in task['review_only']
    assert 'hf-build' in task['evidence'] and 'hf-done' in task['evidence']
    for owner_path in ('services/wallet/deploy/human.toml', 'interfaces.kvx',
                       'services/platform/hosted/gateway', 'services/human'):
        assert any(owner_path in item for item in task['wiring_requests'])
    assert any('fifteen' in item and 'TLS' in item and 'distinct' in item for item in task['wiring_requests'])
    assert any('402' in item and '125' in item for item in task['wiring_requests'])
    assert record['gotcha']


def test_kernel_image_unit_and_persistent_volume_contract():
    sections, current = {}, None
    for line in (KERNEL / 'layerx-kernel.service').read_text().splitlines():
        if line.startswith('['):
            current = line[1:-1]
            sections[current] = {}
        elif '=' in line:
            key, value = line.split('=', 1)
            sections[current].setdefault(key, []).append(value)
    service = sections['Service']
    assert service['EnvironmentFile'] == ['/etc/layerx/kernel.env']
    assert service['Restart'] == ['always']
    command = shlex.split(service['ExecStart'][0])
    assert command[:3] == ['/usr/bin/docker', 'run', '--rm']
    assert command[command.index('-v') + 1] == '/var/lib/layerx/kernel:/data'
    assert command[command.index('--env-file') + 1] == '/etc/layerx/kernel.env'
    assert command[command.index('--stop-timeout') + 1] == '60'
    assert command[-1] == '${LAYERX_KERNEL_IMAGE}'
    assert '--privileged' not in command
    ports = [command[i + 1] for i, token in enumerate(command) if token == '-p']
    assert ports == ['127.0.0.1:8545:8545', '127.0.0.1:8546:8546', '26656:26656', '127.0.0.1:9460:9460']
    assert service['ExecStop'] == ['/usr/bin/docker stop -t 60 layerx-kernel']
    assert service['TimeoutStopSec'] == ['70']
    assert '/usr/bin/test -s /var/lib/layerx/kernel/layerx/node/config/genesis.json' in service['ExecStartPre']
    policy = (KERNEL / 'kernel.env.example').read_text()
    for fact in ('retain the legacy volume offline', 'provision a separate', 'switch the mount',
                 'LAYERX_KERNEL_NODE_HOME=/data/layerx/node'):
        assert fact in policy
