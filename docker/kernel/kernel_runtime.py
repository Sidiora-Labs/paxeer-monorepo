#!/usr/bin/env python3
"""Own one native node and serve live committed-chain readiness across restarts."""

import argparse
from datetime import datetime
import fcntl
import hashlib
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import threading
import time
import tomllib

# The image packages these production node-owner modules beside this runtime.
sys.path.insert(0, str(Path(__file__).resolve().parent / 'node'))
import native_runtime as native
from reset_state import StoreError, canonical, decode


CHAIN_ID = 'hyperpax_125-1'
EVM_CHAIN_ID = 125
PAXD = '/usr/local/bin/paxd'
NODE = {
    'LAYERX_NODE_CHAIN_ID': CHAIN_ID,
    'LAYERX_NODE_EVM_CHAIN_ID': str(EVM_CHAIN_ID),
    'LAYERX_NODE_RPC_URL': 'http://127.0.0.1:8545',
    'LAYERX_NODE_CONSENSUS_RPC_URL': 'http://127.0.0.1:26657',
}
MAXIMUM_COMMIT_AGE = 60


def validate_chain_environment(environment):
    """Domain policy never selects the native EVM signing chain."""
    for name in ('LAYERX_NODE_EVM_CHAIN_ID', 'LAYERX_NODE_PAXEER_CHAIN_ID'):
        if environment.get(name, str(EVM_CHAIN_ID)) != str(EVM_CHAIN_ID):
            raise ValueError(name + ' must be 125 for the merged kernel')


def private_directory(path):
    path = Path(path).absolute()
    if path.resolve() != path:
        raise ValueError('native home contains a symlink or noncanonical path')
    info = path.stat(follow_symlinks=False)
    if (not stat.S_ISDIR(info.st_mode) or info.st_uid != os.geteuid()
            or stat.S_IMODE(info.st_mode) != 0o700):
        raise ValueError('native home requires its node owner and mode 0700')
    return path


def private_file(path, maximum=4096):
    raw = native.read_regular(path, maximum)
    if stat.S_IMODE(Path(path).stat(follow_symlinks=False).st_mode) != 0o600:
        raise ValueError('native private input requires mode 0600')
    return raw


def validate_home(home):
    home = private_directory(home)
    # Migration is an offline owner operation, never an entrypoint side effect.
    # Archived sibling volumes may remain, but legacy state cannot be launched
    # from a native home, even after it has been partly provisioned.
    for name in ('metadata.lxgb', 'settlement.env', 'guarantor-1',
                 'guarantor-2', 'guarantor-submitter'):
        if os.path.lexists(home / name):
            raise ValueError('legacy kernel state requires offline migration; '
                             'preserve the old volume and provision a separate native home')
    for name in ('config', 'data'):
        private_directory(home / name)
    genesis_raw = native.read_regular(home / 'config/genesis.json')
    genesis = decode(genesis_raw)
    if (genesis['chain_id'] != CHAIN_ID or not isinstance(genesis['validators'], list)
            or not genesis['validators']
            or not isinstance(genesis['app_state']['bank'], dict)
            or not isinstance(genesis['app_state']['lxphost'], dict)):
        raise ValueError('provision native validators, bank and merged kernel genesis')
    configs = {name: tomllib.loads(native.read_regular(home / 'config' / name).decode())
               for name in ('config.toml', 'app.toml')}
    config = configs['config.toml']
    if (config['mode'] != 'validator' or config['db-dir'] != 'data'
            or config['genesis-file'] != 'config/genesis.json'
            or config['node-key-file'] != 'config/node_key.json'
            or config['priv-validator']['key-file'] != 'config/priv_validator_key.json'
            or config['priv-validator']['state-file'] != 'data/priv_validator_state.json'
            or config['priv-validator']['laddr'] != ''):
        raise ValueError('native kernel paths must remain in the provisioned node home')
    # paxd owns semantic key/genesis validation. Never replace a missing key or
    # reset signing state during startup or after a crash.
    for name in ('config/priv_validator_key.json', 'config/node_key.json',
                 'data/priv_validator_state.json'):
        if not isinstance(decode(private_file(home / name)), dict):
            raise ValueError('invalid native private material')
    seed = private_file(home / 'config/receipt.key').strip()
    if not re.fullmatch(rb'[0-9a-f]{64}', seed) or seed == b'0' * 64:
        raise ValueError('native receipt seed must be nonzero hexadecimal')
    return hashlib.sha256(genesis_raw).hexdigest()


def node_command(home):
    return [
        PAXD, 'start', '--home', str(home), '--chain-id', CHAIN_ID,
        '--rpc.laddr', 'tcp://127.0.0.1:26657',
        '--evm.http_enabled', 'true', '--evm.http_address', '0.0.0.0',
        '--evm.http_port', '8545', '--evm.ws_enabled', 'true', '--evm.ws_port', '8546',
        '--evm.enable_test_api', 'false', '--evm.enable_unsafe_keyring_rpc', 'false',
        '--sequencer.enable', 'true',
        '--sequencer.wal-dir', str(home / 'data/sequencer-wal'),
        '--sequencer.receipt-key-file', str(home / 'config/receipt.key'),
    ]


def live_process(pid, start):
    try:
        raw = Path('/proc', str(pid), 'stat').read_text()
        fields = raw[raw.rindex(')') + 2:].split()
        return fields[0] not in ('Z', 'X') and fields[19] == start
    except (OSError, IndexError, ValueError):
        return False


class CommittedReadiness:
    """A retained checkpoint is a recovery floor, never cached health."""

    def __init__(self, node, checkpoint, genesis, pid, clock=time.time):
        if node['LAYERX_NODE_CHAIN_ID'] != CHAIN_ID or node['LAYERX_NODE_EVM_CHAIN_ID'] != '125':
            raise ValueError('kernel readiness requires the merged chain identity')
        for name in ('LAYERX_NODE_RPC_URL', 'LAYERX_NODE_CONSENSUS_RPC_URL'):
            native.endpoint(node[name], loopback=True)
        self.node = dict(node)
        self.checkpoint = Path(checkpoint)
        private_directory(self.checkpoint.parent)
        if not re.fullmatch(r'[0-9a-f]{64}', genesis):
            raise ValueError('invalid genesis binding')
        self.genesis = genesis
        self.pid, self.start = pid, native.process_start(pid)
        self.clock = clock
        self.lock = threading.Lock()
        self.retained = None
        if os.path.lexists(self.checkpoint):
            value = decode(private_file(self.checkpoint, 8192))
            if (set(value) != {'schema', 'genesis', 'height', 'block_hash'}
                    or value['schema'] != 'layerx.kernel.committed.v1'
                    or value['genesis'] != genesis or type(value['height']) is not int
                    or value['height'] <= 0
                    or not isinstance(value['block_hash'], str)
                    or not re.fullmatch(r'[0-9a-f]{64}', value['block_hash'])):
                raise ValueError('retained committed-chain checkpoint differs')
            self.retained = value

    def probe(self):
        with self.lock:
            try:
                if not live_process(self.pid, self.start):
                    raise ValueError('native node process is unavailable')
                # Reuse the node owner's actual consensus -> canonical commit ->
                # EVM reader path. No generation socket, environment projection
                # or independent replica is involved.
                committed = native.committed_health(self.node)
                status = native.rpc(self.node['LAYERX_NODE_CONSENSUS_RPC_URL'], 'status', {})
                info = status['sync_info']
                height = committed['height']
                if (status['node_info']['network'] != CHAIN_ID
                        or info['catching_up'] is not False
                        or info['latest_block_height'] != str(height)
                        or int(info['last_committed_block_height']) < height
                        or info['latest_block_hash'].lower() != committed['block_hash']):
                    raise ValueError('executed state is not a stable consensus commit')
                stamp = datetime.fromisoformat(info['latest_block_time'].replace('Z', '+00:00'))
                if stamp.tzinfo is None or not 0 <= self.clock() - stamp.timestamp() <= MAXIMUM_COMMIT_AGE:
                    raise ValueError('native committed chain is stale')
                if not live_process(self.pid, self.start):
                    raise ValueError('native node stopped during readiness')
                current = {'schema': 'layerx.kernel.committed.v1', 'genesis': self.genesis, **committed}
                if self.retained is not None:
                    previous = self.retained
                    if height < previous['height']:
                        raise ValueError('native committed state rolled back')
                    if height == previous['height'] and current != previous:
                        raise ValueError('native committed state diverged')
                if current != self.retained:
                    native.write_file(self.checkpoint, canonical(current))
                    self.retained = current
                return 200, {'ready': True, 'chain_id': EVM_CHAIN_ID, **committed}
            except (OSError, ValueError, KeyError, TypeError, OverflowError,
                    StoreError, AttributeError, http.client.HTTPException):
                return 503, {'ready': False, 'chain_id': EVM_CHAIN_ID}


def readiness_server(readiness, address=('0.0.0.0', 9460)):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path != '/readyz':
                code, value = 404, {'error': 'not found'}
            else:
                code, value = readiness.probe()
            raw = canonical(value)
            self.send_response(code)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Cache-Control', 'no-store')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *_):
            pass

    return ThreadingHTTPServer(address, Handler)


def supervise(home):
    home = private_directory(home)
    lock = os.open(home / '.kernel-owner.lock', os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    child = server = thread = None
    stopped = threading.Event()
    old_signals = {}
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        genesis = validate_home(home)
        native.run_native([PAXD, 'validate-genesis', '--home', str(home)])
        for number in (signal.SIGTERM, signal.SIGINT):
            old_signals[number] = signal.signal(number, lambda *_: stopped.set())
        child = native.spawn_child(node_command(home))
        readiness = CommittedReadiness(NODE, home / '.committed-readiness.json', genesis, child.pid)
        server = readiness_server(readiness)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        while child.poll() is None and not stopped.wait(0.2):
            pass
        # The container owner restarts the entire component. A clean paxd exit
        # without a shutdown request is still loss of the serving kernel.
        return 0 if stopped.is_set() else (child.returncode or 1)
    finally:
        native.stop_process(child)
        if server is not None:
            if thread is not None and thread.is_alive():
                server.shutdown()
                thread.join(timeout=5)
            server.server_close()
        for number, handler in old_signals.items():
            signal.signal(number, handler)
        os.close(lock)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--home', required=True, type=Path)
    args = parser.parse_args()
    try:
        validate_chain_environment(os.environ)
        return supervise(args.home)
    except (OSError, ValueError, KeyError, TypeError, StoreError, subprocess.SubprocessError) as error:
        print('kernel-init: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
