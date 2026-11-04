#!/usr/bin/env python3
"""Native paxd generation preparation, committed health and supervision."""
import argparse
import base64
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import signal
import secrets
import ssl
import stat
import subprocess
import sys
import time
from urllib.parse import urlsplit

import data_directory
from reset_state import (
    Store, StoreError, canonical, close_supervisor_lock, decode, supervisor_lock,
)

MAXIMUM = 64 * 1024 * 1024


def read_regular(path, maximum=MAXIMUM):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        info = os.fstat(descriptor)
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
                or info.st_uid != os.geteuid() or info.st_mode & 0o022
                or info.st_size > maximum):
            raise ValueError('unsafe native input: ' + str(path))
        value = os.read(descriptor, maximum + 1)
        if len(value) > maximum:
            raise ValueError('native input exceeds bound')
        return value
    finally:
        os.close(descriptor)


def write_file(path, raw):
    path = Path(path)
    _, parent = data_directory.open_directory(path.parent)
    name = '.' + path.name + '.' + secrets.token_hex(16) + '.pending'
    descriptor = None
    try:
        descriptor = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                             0o600, dir_fd=parent)
        with os.fdopen(descriptor, 'wb', closefd=False) as handle:
            handle.write(raw)
            handle.flush()
            os.fsync(descriptor)
        os.rename(name, path.name, src_dir_fd=parent, dst_dir_fd=parent)
        os.fsync(parent)
    finally:
        if descriptor is not None:
            os.close(descriptor)
        try:
            os.unlink(name, dir_fd=parent)
        except FileNotFoundError:
            pass
        os.close(parent)


def endpoint(value, tls=False, loopback=False):
    parsed = urlsplit(value)
    if (parsed.scheme != ('https' if tls else 'http') or not parsed.hostname
            or parsed.username is not None or parsed.password is not None
            or parsed.query or parsed.fragment or parsed.path not in ('', '/')
            or any(ord(c) <= 32 or ord(c) >= 127 for c in value)
            or (loopback and parsed.hostname != '127.0.0.1')):
        raise ValueError('native RPC requires an explicit root origin')
    if parsed.port is None or not 1 <= parsed.port <= 65535:
        raise ValueError('native RPC requires an explicit port')
    return parsed


def validate_core_inputs(values):
    endpoint(values['LAYERX_CORE_SEQUENCER_ENDPOINT'], tls=True)
    endpoint(values['LAYERX_CORE_CHAIN_RPC_URL'], loopback=True)
    for key in ('LAYERX_CORE_TREASURY_SIGNER_SOCKET',
                'LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER',
                'LAYERX_CORE_FUNDING_SENDER_KEY_FILE'):
        value = values[key]
        if (not value.startswith('/') or os.path.normpath(value) != value
                or any(c.isspace() for c in value)):
            raise ValueError('native core requires a protected absolute input path')


def rpc(url, method, params, ca=None):
    parsed = endpoint(url, tls=ca is not None)
    connection = None
    try:
        if ca is not None:
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
            context.load_verify_locations(cadata=ssl.DER_cert_to_PEM_cert(ca))
            connection = http.client.HTTPSConnection(parsed.hostname, parsed.port,
                                                      timeout=2, context=context)
        else:
            connection = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=2)
        identifier = 'native-health-' + method
        body = canonical({'jsonrpc': '2.0', 'id': identifier,
                          'method': method, 'params': params})
        connection.request('POST', '/', body, {'Content-Type': 'application/json',
                                             'Accept': 'application/json',
                                             'Connection': 'close'})
        response = connection.getresponse()
        raw = response.read(1024 * 1024 + 1)
        if (response.status != 200 or len(raw) > 1024 * 1024
                or response.getheader('Content-Type', '').split(';')[0] != 'application/json'):
            raise ValueError('native RPC transport unavailable')
        document = decode(raw)
        if (not isinstance(document, dict) or document.get('jsonrpc') != '2.0'
                or document.get('id') != identifier or 'error' in document
                or 'result' not in document):
            raise ValueError('native RPC response refused')
        return document['result']
    finally:
        if connection is not None:
            connection.close()


def committed_health(node):
    consensus_url = node['LAYERX_NODE_CONSENSUS_RPC_URL']
    chain_url = node['LAYERX_NODE_RPC_URL']
    endpoint(consensus_url, loopback=True)
    endpoint(chain_url, loopback=True)
    status_result = rpc(consensus_url, 'status', {})
    info = status_result['sync_info']
    height = info['latest_block_height']
    block_hash = info['latest_block_hash']
    if (status_result['node_info']['network'] != node['LAYERX_NODE_CHAIN_ID']
            or info['catching_up'] is not False or not isinstance(height, str)
            or not re.fullmatch(r'[1-9][0-9]*', height)
            or not isinstance(block_hash, str)
            or not re.fullmatch(r'[0-9A-Fa-f]{64}', block_hash)):
        raise ValueError('native chain is not committed and caught up')
    # A signed commit, rather than an open/pending head, is the readiness boundary.
    commit = rpc(consensus_url, 'commit', {'height': height})
    header = commit['signed_header']['header']
    commit_value = commit['signed_header']['commit']
    if (commit['canonical'] is not True or header['height'] != height
            or header['chain_id'] != node['LAYERX_NODE_CHAIN_ID']
            or commit_value['height'] != height
            or commit_value['block_id']['hash'].lower() != block_hash.lower()
            or not commit_value['signatures']
            or not re.fullmatch(r'[0-9A-Fa-f]{64}', header['app_hash'])):
        raise ValueError('native signed commit does not match status')
    chain_id = rpc(chain_url, 'eth_chainId', [])
    if chain_id != hex(int(node['LAYERX_NODE_EVM_CHAIN_ID'])):
        raise ValueError('native EVM chain identity differs')
    block = rpc(chain_url, 'eth_getBlockByNumber', [hex(int(height)), False])
    if (block['number'] != hex(int(height))
            or not re.fullmatch(r'0x[0-9a-fA-F]{64}', block['hash'])
            or block['hash'][2:].lower() != block_hash.lower()):
        raise ValueError('native EVM read is not the committed block')
    return {'height': int(height), 'block_hash': block_hash.lower()}


def process_start(pid):
    # starttime prevents PID reuse from authorizing a different native process.
    raw = Path('/proc', str(pid), 'stat').read_text()
    return raw[raw.rindex(')') + 2:].split()[19]


def process_identity(data, pid, generation, reset_id, genesis):
    info = os.stat(data, follow_symlinks=False)
    return {'version': 1, 'pid': pid, 'start': process_start(pid),
            'device': info.st_dev, 'inode': info.st_ino,
            'generation': generation, 'reset_id': reset_id, 'genesis': genesis}


def activated_ready(data, run, expected):
    from generation_transport import environment

    try:
        identity = decode(read_regular(Path(run) / 'native-process.json', 8192))
        if (type(identity.get('pid')) is not int or identity['pid'] <= 1
                or identity != process_identity(data, identity['pid'], expected['generation'],
                                                 expected['reset_id'], expected['genesis'])):
            return False
        node = environment(read_regular(Path(data) / 'node.env', 65536))
        committed_health(node)
        return True
    except (OSError, ValueError, KeyError, TypeError, StoreError, http.client.HTTPException):
        return False


def generation_binding(data):
    return {name: hashlib.sha256(read_regular(Path(data) / name)).hexdigest()
            for name in sorted(data_directory.GENESIS_FILES)}


def publish_generation(data, node, core, ca):
    validate_core_inputs(core)
    for name, value in (('node.env', node), ('core.env', core)):
        if any(not re.fullmatch(r'LAYERX_[A-Z0-9_]+', key)
               or any(ord(c) < 32 or ord(c) == 127 for c in str(item))
               for key, item in value.items()):
            raise ValueError('invalid generation environment')
        write_file(Path(data) / name,
                   ''.join(key + '=' + str(value[key]) + '\n' for key in sorted(value)).encode())
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.load_verify_locations(cadata=ssl.DER_cert_to_PEM_cert(ca))
    write_file(Path(data) / 'rpc-ca.der', ca)
    proof = Path(data) / '.generation-proof'
    proof.mkdir(mode=0o700)
    (proof / 'config').mkdir(mode=0o700)
    for name in sorted(data_directory.GENESIS_FILES):
        write_file(proof / name, read_regular(Path(data) / name))
    _, descriptor = data_directory.open_directory(data)
    try:
        data_directory._sync_tree(descriptor)
    finally:
        os.close(descriptor)
    binding = generation_binding(data)
    write_file(Path(data) / '.native-prepared.json', canonical(binding))
    return binding


class NativeCommands:
    def run(self, command):
        run_native(command)

    def start(self, command):
        return spawn_child(command)


def bootstrap(args, data=None, commands=None):
    commands = commands if commands is not None else NativeCommands()
    data = Path(data or args.data_dir).absolute()
    _, directory = data_directory.open_directory(data, create=True)
    try:
        data_directory._private_directory(directory)
        if os.listdir(directory):
            raise ValueError('native bootstrap requires an empty generation')
    finally:
        os.close(directory)
    source = Path(args.native_inputs)
    receipt = read_regular(args.receipt_key_file, 4096).strip()
    if not re.fullmatch(rb'[0-9a-f]{64}', receipt):
        raise ValueError('native receipt key must use the paxd hexadecimal seed file contract')
    validator = source / 'priv_validator_key.json'
    if stat.S_IMODE(validator.stat(follow_symlinks=False).st_mode) != 0o600:
        raise ValueError('native validator key requires private mode 0600')
    genesis = decode(read_regular(source / 'genesis.json'))
    if (genesis['chain_id'] != args.chain_id or not genesis['validators']
            or not isinstance(genesis['app_state']['bank'], dict)
            or not isinstance(genesis['app_state']['lxphost'], dict)):
        raise ValueError('authoritative native genesis must include validators, bank and merged kernel')
    # paxd owns configuration defaults and genesis validation; no private legacy codec.
    commands.run([args.paxd, 'init', args.moniker, '--chain-id', args.chain_id,
                '--mode', 'validator', '--home', str(data)])
    for target, origin in (('config/genesis.json', 'genesis.json'),
                           ('config/config.toml', 'config.toml'),
                           ('config/app.toml', 'app.toml'),
                           ('config/priv_validator_key.json', 'priv_validator_key.json')):
        write_file(data / target, read_regular(source / origin))
    commands.run([args.paxd, 'validate-genesis', '--home', str(data)])
    # The signer seed stays in the private native-key mount. Publish only its public identity.
    sys.path.insert(0, str(Path(__file__).parent / 'signer'))
    from provider import FileProvider

    provider = FileProvider(args.receipt_key_file)
    try:
        public_key = provider.public_key()
    finally:
        provider.close()
    key_id = hashlib.sha256(public_key).hexdigest()
    pins = genesis['app_state']['sequencer']['receipt_keys']
    if not any(base64.b64decode(pin['public_key'], validate=True) == public_key
               and base64.b64decode(pin['key_id'], validate=True).hex() == key_id
               and int(pin['first_height']) == 1 and int(pin['last_height']) >= 1
               for pin in pins):
        raise ValueError('native receipt signer is not pinned in genesis')
    core = {
        'LAYERX_CORE_SEQUENCER_ID': key_id,
        'LAYERX_CORE_TREASURY_ASSET': args.asset,
        'LAYERX_CORE_TREASURY_SIGNER_SOCKET': args.treasury_signer_socket,
        'LAYERX_CORE_SEQUENCER_ENDPOINT': args.gateway_endpoint,
        'LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER': '/generation/rpc-ca.der',
        'LAYERX_CORE_CHAIN_RPC_URL': args.chain_rpc_url,
        'LAYERX_CORE_FUNDING_SENDER_KEY_FILE': args.funding_sender_key_file,
    }
    node = {
        'LAYERX_NODE_NETWORK_ID': str(args.network_id),
        'LAYERX_NODE_CHAIN_ID': args.chain_id,
        'LAYERX_NODE_EVM_CHAIN_ID': str(args.evm_chain_id),
        'LAYERX_NODE_SEQUENCER_ID': key_id,
        'LAYERX_NODE_SEQUENCER_PUBLIC_KEY': public_key.hex(),
        'LAYERX_NODE_RPC_URL': args.chain_rpc_url,
        'LAYERX_NODE_CONSENSUS_RPC_URL': args.consensus_rpc_url,
    }
    return publish_generation(data, node, core, read_regular(source / 'rpc-ca.der', 1048576))


def stop_process(child):
    if child is not None and child.poll() is None:
        child.terminate()
        try:
            child.wait(timeout=30)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait(timeout=10)


def child_command(command):
    # Linux parent-death signaling covers abrupt owner death as well as SIGTERM.
    return [sys.executable, str(Path(__file__).with_name('reset_state.py')),
            'exec-daemon', '--', *command]


def run_native(command):
    raw = os.environ.get('LAYERX_RESET_OWNER_FD')
    descriptors = () if raw is None else (int(raw),)
    subprocess.run(child_command(command), pass_fds=descriptors, check=True, timeout=60)


def spawn_child(command):
    raw = os.environ.get('LAYERX_RESET_OWNER_FD')
    descriptors = () if raw is None else (int(raw),)
    return subprocess.Popen(child_command(command), pass_fds=descriptors)


def start_native(args, store, record=None, commands=None):
    commands = commands if commands is not None else NativeCommands()
    from generation_transport import environment

    node = environment(read_regular(Path(args.data_dir) / 'node.env', 65536))
    chain = endpoint(node['LAYERX_NODE_RPC_URL'], loopback=True)
    consensus = endpoint(node['LAYERX_NODE_CONSENSUS_RPC_URL'], loopback=True)
    generation = record['generation'] if record else store.generation()
    reset_id = record['reset_id'] if record else None
    genesis = record['genesis'] if record else generation_binding(args.data_dir)
    process = commands.start([
        args.paxd, 'start', '--home', args.data_dir,
        '--rpc.laddr', 'tcp://' + args.rpc_bind_address + ':' + str(consensus.port),
        '--evm.http_enabled', 'true', '--evm.http_address', args.rpc_bind_address,
        '--evm.http_port', str(chain.port), '--sequencer.enable', 'true',
        '--sequencer.wal-dir', str(Path(args.data_dir) / 'sequencer-wal'),
        '--sequencer.receipt-key-file', args.receipt_key_file,
    ])
    try:
        write_file(Path(args.run_dir) / 'native-process.json',
                   canonical(process_identity(args.data_dir, process.pid, generation, reset_id, genesis)))
        deadline = time.monotonic() + args.ready_seconds
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise ValueError('native paxd exited before committed readiness')
            try:
                committed_health(node)
                return process
            except (OSError, ValueError, KeyError, TypeError, StoreError, http.client.HTTPException):
                time.sleep(0.2)
        raise ValueError('native paxd committed readiness timed out')
    except BaseException:
        stop_process(process)
        raise


def clear_unsealed(path):
    """Resume only an unsealed, owner-controlled bootstrap stage."""
    import shutil

    _, descriptor = data_directory.open_directory(path)
    try:
        data_directory._private_directory(descriptor)
        if data_directory._marker(descriptor) is not None:
            raise ValueError('cannot rebuild a sealed generation')
        if not shutil.rmtree.avoids_symlink_attacks:
            raise ValueError('safe generation removal unavailable')
        for name in os.listdir(descriptor):
            info = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
            if stat.S_ISDIR(info.st_mode):
                shutil.rmtree(name, dir_fd=descriptor)
            else:
                os.unlink(name, dir_fd=descriptor)
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def reset_generation(args, store, record, child, commands=None):
    reset_id = record['reset_id']
    phase = record['phase']
    stage = Path(args.state_dir) / 'generations' / reset_id / 'data'
    if phase == 'admitted':
        record = store.transition(reset_id, phase, 'stopping')
        phase = record['phase']
    if phase == 'stopping':
        data_directory.prepare_stage(args.data_dir, args.state_dir, reset_id)
        record = store.transition(reset_id, phase, 'bootstrapping')
        phase = record['phase']
    if phase == 'bootstrapping':
        if not (stage / data_directory.GENERATION_MARKER).exists():
            clear_unsealed(stage)
            binding = bootstrap(args, stage, commands)
            data_directory.seal_generation(args.data_dir, stage, reset_id,
                                           record['generation'], binding)
        binding = generation_binding(stage)
        record = store.transition(reset_id, phase, 'prepared', {'genesis': binding})
        phase = record['phase']
    if phase == 'prepared':
        record = store.transition(reset_id, phase, 'activating')
        phase = record['phase']
    if phase != 'activating':
        raise ValueError('native reset phase requires reconciliation')
    stop_process(child)
    data_directory.activate_generation(args.data_dir, stage, reset_id,
                                        record['generation'], record['genesis'])
    child = start_native(args, store, record, commands)
    # Store owns durable completion and its original response, after native committed health.
    if store.active() is not None:
        stop_process(child)
        raise ValueError('native generation has not qualified activation')
    return child


def supervise(args, commands=None):
    for path in (args.data_dir, args.run_dir, args.state_dir):
        _, descriptor = data_directory.open_directory(path, create=True)
        try:
            if path == args.run_dir:
                metadata = os.fstat(descriptor)
                if metadata.st_uid != os.geteuid() or stat.S_IMODE(metadata.st_mode) not in (0o700, 0o750):
                    raise ValueError('unsafe native run directory')
                os.fchmod(descriptor, 0o750)
            else:
                data_directory._private_directory(descriptor)
        finally:
            os.close(descriptor)
    owner = supervisor_lock(args.state_dir)
    os.environ['LAYERX_RESET_OWNER_FD'] = str(owner)
    os.environ['LAYERX_NODE_RESET_STAGED_GENERATIONS'] = '1'
    children = []
    stopping = False

    def stop(_number, _frame):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        inputs = {name: hashlib.sha256(read_regular(Path(args.native_inputs) / name)).hexdigest()
                  for name in ('genesis.json', 'config.toml', 'app.toml',
                               'priv_validator_key.json', 'rpc-ca.der')}
        # Freeze all bootstrap options; changed endpoints or signer paths cannot replay a reset.
        bindings = {'inputs': inputs, 'options': vars(args)}
        store = Store(args.state_dir, args.data_dir, args.run_dir, args.network_id,
                      bindings, args.consumer_gid)
        store.initialize()
        active = store.active()
        child = None
        if active is None:
            if not (Path(args.data_dir) / '.native-prepared.json').exists():
                if store.completed() is not None:
                    raise ValueError('completed native generation is missing')
                clear_unsealed(args.data_dir)
                bootstrap(args, commands=commands)
            from generation_transport import environment
            node = environment(read_regular(Path(args.data_dir) / 'node.env', 65536))
            if node['LAYERX_NODE_NETWORK_ID'] != str(args.network_id):
                raise ValueError('retained native network differs')
            if decode(read_regular(Path(args.data_dir) / '.native-prepared.json', 8192)) != generation_binding(args.data_dir):
                raise ValueError('native generation differs from bootstrap publication')
            completed = store.completed()
            if completed is not None:
                stage = Path(args.state_dir) / 'generations' / completed['reset_id'] / 'data'
                data_directory.activate_generation(args.data_dir, stage, completed['reset_id'],
                                                    completed['generation'], completed['genesis'])
            child = start_native(args, store, completed, commands)
        else:
            child = reset_generation(args, store, active, child, commands)
        children.append(child)
        here = Path(__file__).parent
        for filename, command in (
            ('generation.sock', [sys.executable, str(here / 'generation_transport.py'),
                                 '--data-dir', args.data_dir, '--state-dir', args.state_dir,
                                 '--run-dir', args.run_dir, '--socket', str(Path(args.run_dir) / 'generation.sock'),
                                 '--allowed-uid', str(args.consumer_uid), '--allowed-gid', str(args.consumer_gid)]),
            ('supervisor.sock', [sys.executable, str(here / 'reset_state.py'), 'serve',
                                 '--data-dir', args.data_dir, '--state-dir', args.state_dir,
                                 '--run-dir', args.run_dir, '--network', str(args.network_id),
                                 '--allowed-gid', str(args.consumer_gid),
                                 '--bindings', str(Path(args.run_dir) / 'bindings.json'),
                                 '--socket', str(Path(args.run_dir) / 'supervisor.sock')]),
        ):
            path = Path(args.run_dir) / filename
            if path.exists() or path.is_symlink():
                if not stat.S_ISSOCK(path.lstat().st_mode) or path.lstat().st_uid != os.geteuid():
                    raise ValueError('unsafe retained native socket')
                path.unlink()
            write_file(Path(args.run_dir) / 'bindings.json', canonical(bindings))
            children.append(spawn_child(command))
        while not stopping:
            if any(process.poll() is not None for process in children):
                raise ValueError('native supervised process exited')
            active = store.active()
            if active is not None:
                child = reset_generation(args, store, active, child, commands)
                children[0] = child
            time.sleep(0.2)
        return 0
    finally:
        for child in reversed(children):
            stop_process(child)
        close_supervisor_lock()


def parser():
    result = argparse.ArgumentParser()
    result.add_argument('action', choices=('bootstrap', 'supervise', 'health'))
    result.add_argument('--data-dir', required=True)
    result.add_argument('--run-dir')
    result.add_argument('--state-dir')
    result.add_argument('--paxd', default='/usr/local/bin/paxd')
    result.add_argument('--native-inputs')
    result.add_argument('--moniker', default='hosted-native')
    result.add_argument('--chain-id')
    result.add_argument('--network-id', type=int)
    result.add_argument('--evm-chain-id', type=int)
    result.add_argument('--asset')
    result.add_argument('--gateway-endpoint')
    result.add_argument('--chain-rpc-url', default='http://127.0.0.1:8545')
    result.add_argument('--consensus-rpc-url', default='http://127.0.0.1:26657')
    result.add_argument('--rpc-bind-address', choices=('127.0.0.1', '0.0.0.0'), default='127.0.0.1')
    result.add_argument('--receipt-key-file')
    result.add_argument('--treasury-signer-socket')
    result.add_argument('--funding-sender-key-file')
    result.add_argument('--consumer-uid', type=int, default=4021)
    result.add_argument('--consumer-gid', type=int, default=4020)
    result.add_argument('--ready-seconds', type=int, default=120)
    return result


def main():
    command = parser()
    args = command.parse_args()
    try:
        args.data_dir = os.path.abspath(args.data_dir)
        if args.action == 'health':
            from generation_transport import environment

            node = environment(read_regular(Path(args.data_dir) / 'node.env', 65536))
            if not args.run_dir:
                raise ValueError('health requires run directory')
            identity = decode(read_regular(Path(args.run_dir) / 'native-process.json', 8192))
            expected = {'generation': identity['generation'], 'reset_id': identity['reset_id'],
                        'genesis': generation_binding(args.data_dir)}
            if not activated_ready(args.data_dir, args.run_dir, expected):
                raise ValueError('native process is not ready')
            print(json.dumps(committed_health(node), sort_keys=True))
            return 0
        for name in ('native_inputs', 'chain_id', 'network_id', 'evm_chain_id', 'asset',
                     'gateway_endpoint', 'receipt_key_file', 'treasury_signer_socket',
                     'funding_sender_key_file'):
            if not getattr(args, name):
                raise ValueError('missing native input --' + name.replace('_', '-'))
        if (not 1 <= args.network_id <= 0xffffffff or not 1 <= args.evm_chain_id <= 0xffffffffffffffff
                or not re.fullmatch(r'[A-Za-z0-9_-]{1,128}', args.chain_id)
                or not re.fullmatch(r'[0-9a-f]{64}', args.asset) or args.asset == '0' * 64
                or not 1 <= args.ready_seconds <= 300):
            raise ValueError('invalid native identity or readiness bound')
        if args.action == 'bootstrap':
            print(json.dumps(bootstrap(args), sort_keys=True))
            return 0
        if not args.state_dir or not args.run_dir:
            raise ValueError('supervision requires persistent state and runtime directories')
        args.state_dir = os.path.abspath(args.state_dir)
        args.run_dir = os.path.abspath(args.run_dir)
        return supervise(args)
    except (OSError, ValueError, KeyError, TypeError, StoreError, subprocess.SubprocessError,
            http.client.HTTPException) as error:
        print('native node refused: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
