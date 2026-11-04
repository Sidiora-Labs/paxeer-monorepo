"""Disposable merged paxd generation and its root JSON-RPC transport.

Native genesis, signer inputs and executable provenance are supplied by the
release kit. No standalone daemon, replica, or public router is started here.
"""

import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import signal
import socket
import subprocess
import sys
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[4]
BOOTSTRAP = ROOT / 'services/platform/hosted/node/native_runtime.py'


def require(condition, message):
    if not condition:
        raise RuntimeError('merged runtime refused: ' + message)


def reserve_ports(count):
    sockets = []
    try:
        for _ in range(count):
            sock = socket.socket()
            sockets.append(sock)
            sock.bind(('127.0.0.1', 0))
        return sockets
    except BaseException:
        for sock in sockets:
            sock.close()
        raise


def write_json(path, document):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(document, stream, sort_keys=True)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())


class RootRpc:
    def __init__(self, endpoint):
        self.url = urlsplit(endpoint)
        require(self.url.scheme == 'http' and self.url.hostname == '127.0.0.1'
                and self.url.port is not None and self.url.path in ('', '/')
                and not self.url.username and not self.url.password
                and not self.url.query and not self.url.fragment,
                'owned loopback root RPC origin required')
        self.endpoint = endpoint.rstrip('/') + '/'
        self.identifier = 0

    def request(self, method, params):
        self.identifier += 1
        body = json.dumps({'jsonrpc': '2.0', 'id': self.identifier,
                           'method': method, 'params': params}, separators=(',', ':')).encode()
        connection = http.client.HTTPConnection(self.url.hostname, self.url.port, timeout=10)
        try:
            connection.request('POST', '/', body, {'Content-Type': 'application/json'})
            response = connection.getresponse()
            raw = response.read(1_048_577)
            require(response.status == 200 and response.getheader('Content-Type', '').split(';')[0] == 'application/json'
                    and len(raw) <= 1_048_576, 'RPC HTTP status, type or body bound')
            value = json.loads(raw)
            require(isinstance(value, dict) and value.get('jsonrpc') == '2.0'
                    and type(value.get('id')) is int and value['id'] == self.identifier
                    and ('result' in value) != ('error' in value), 'RPC envelope')
            require('error' not in value, 'RPC error: ' + str(value.get('error')))
            return value['result']
        finally:
            connection.close()


class RuntimeFixture:
    def __init__(self, directory, config, revision):
        self.directory = Path(directory)
        self.config = config
        row = config['paxd']
        binary = Path(row['path'])
        require(binary.is_absolute() and binary.is_file() and not binary.is_symlink()
                and os.access(binary, os.X_OK), 'paxd executable')
        require(row['source_revision'] == revision
                and hashlib.sha256(binary.read_bytes()).hexdigest() == row['sha256'], 'paxd candidate binding')
        self.binary = str(binary)
        self.rpc = RootRpc(config['chain_rpc_url'])
        self.rpc_url = self.rpc.endpoint
        consensus = RootRpc(config['consensus_rpc_url'])
        self.consensus_port = consensus.url.port
        self.env = {key: value for key, value in os.environ.items()
                    if not key.startswith(('LAYERX_', 'PAXEER_X_'))}
        self.process = None
        self.manifest = {'rpc_url': self.rpc_url, 'topology': 'merged-paxd'}

    def generate(self):
        require(not self.directory.exists(), 'fresh disposable generation required')
        self.directory.mkdir(mode=0o700)
        self.home = self.directory / 'chain'
        required = {'native_inputs', 'chain_id', 'network_id', 'evm_chain_id', 'asset',
                    'gateway_endpoint', 'receipt_key_file', 'treasury_signer_socket', 'funding_sender_key_file'}
        values = self.config['bootstrap']
        require(isinstance(values, dict) and set(values) == required, 'authoritative native bootstrap inputs')
        argv = [sys.executable, str(BOOTSTRAP), 'bootstrap', '--paxd', self.binary,
                '--data-dir', str(self.home), '--chain-rpc-url', self.rpc_url,
                '--consensus-rpc-url', self.config['consensus_rpc_url']]
        for key, value in values.items():
            argv.extend(('--' + key.replace('_', '-'), str(value)))
        with (self.directory / 'bootstrap.log').open('xb') as log:
            result = subprocess.run(argv, env=self.env, stdout=log, stderr=log, timeout=120)
        require(result.returncode == 0, 'native bootstrap failed; inspect bootstrap.log')
        write_json(self.directory / 'runtime.json', self.manifest)

    def start(self):
        require(self.process is None or self.process.poll() is not None, 'paxd already running')
        self.home = self.directory / 'chain'
        argv = [self.binary, 'start', '--home', str(self.home),
                '--rpc.laddr', 'tcp://127.0.0.1:' + str(self.consensus_port),
                '--evm.http_enabled', 'true', '--evm.http_address', '127.0.0.1',
                '--evm.http_port', str(self.rpc.url.port), '--sequencer.enable', 'true',
                '--sequencer.wal-dir', str(self.home / 'sequencer-wal'),
                '--sequencer.receipt-key-file', self.config['bootstrap']['receipt_key_file']]
        with (self.directory / 'paxd.log').open('ab') as log:
            self.process = subprocess.Popen(argv, env=self.env, stdout=log, stderr=log, start_new_session=True)

    def wait(self, predicate, seconds=120):
        deadline = time.monotonic() + seconds
        last = None
        while time.monotonic() < deadline:
            require(self.process is not None and self.process.poll() is None, 'paxd exited during readiness')
            try:
                if predicate():
                    return
            except (OSError, ValueError, RuntimeError, http.client.HTTPException) as error:
                last = error
            time.sleep(.1)
        raise RuntimeError('merged runtime readiness timed out') from last

    def readiness(self):
        chain_id = int(self.config['bootstrap']['evm_chain_id'])
        self.wait(lambda: self.rpc.request('eth_chainId', []) == hex(chain_id))
        self.wait(lambda: int(self.rpc.request('eth_blockNumber', []), 16) > 0)

    def submit(self, transaction):
        require(isinstance(transaction, str) and re.fullmatch('0x([0-9a-f]{2})+', transaction),
                'canonical signed merged transaction')
        reference = self.rpc.request('eth_sendRawTransaction', [transaction])
        require(isinstance(reference, str) and re.fullmatch('0x[0-9a-f]{64}', reference), 'transaction reference')
        receipt = None

        def executed():
            nonlocal receipt
            receipt = self.rpc.request('eth_getTransactionReceipt', [reference])
            return receipt is not None

        self.wait(executed)
        require(receipt['transactionHash'] == reference and receipt['status'] == '0x1', 'successful merged receipt')
        return receipt

    def restart(self):
        before = self.rpc.request('eth_getBlockByNumber', ['latest', False])
        self.cleanup()
        self.start()
        self.readiness()
        retained = self.rpc.request('eth_getBlockByNumber', [before['number'], False])
        require(retained == before, 'merged restart changed a retained committed block')

    def cleanup(self):
        if self.process is not None:
            try:
                os.killpg(self.process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                self.process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                os.killpg(self.process.pid, signal.SIGKILL)
                self.process.wait()


class RpcProofFaultRelay:
    """One armed corruption of a genuine merged-node/hosted JSON-RPC reply."""

    MODES = {'wrong_header_signature': 'px_getCommit', 'wrong_native_network': 'eth_chainId',
             'wrong_sequencer_key': 'px_getReceiptKeys', 'maintenance_link': 'eth_getProof',
             'moving_head': 'eth_blockNumber'}

    def __init__(self, node_url, hosted_url, context, mode):
        from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
        import threading

        require(mode in self.MODES, 'unsupported RPC proof fault')
        self.mode = mode
        self.node = urlsplit(node_url)
        self.hosted = urlsplit(hosted_url)
        self.context = context
        self._lock = threading.Lock()
        self._armed = False
        self._callback = None
        self.mutated_frame_count = self.trigger_count = 0
        self.failure_info = []
        relay = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    require(self.path == '/', 'RPC fault relay only serves root')
                    length = int(self.headers['Content-Length'])
                    require(0 < length <= 1_048_576, 'RPC relay body bound')
                    raw = self.rfile.read(length)
                    request = json.loads(raw)
                    value = relay.exchange(request, raw)
                    encoded = json.dumps(value, separators=(',', ':')).encode()
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(encoded)))
                    self.end_headers()
                    self.wfile.write(encoded)
                except Exception as error:
                    relay.failure_info.append(str(error))
                    self.send_error(502)

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.endpoint = 'http://127.0.0.1:' + str(self.server.server_port) + '/'
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def exchange(self, request, raw):
        method = request['method']
        origin = self.hosted if method.startswith('px_') else self.node
        connection = (http.client.HTTPSConnection(origin.hostname, origin.port, context=self.context, timeout=10)
                      if origin.scheme == 'https' else http.client.HTTPConnection(origin.hostname, origin.port, timeout=10))
        try:
            connection.request('POST', '/', raw, {'Content-Type': 'application/json'})
            response = connection.getresponse()
            data = response.read(1_048_577)
            require(response.status == 200 and len(data) <= 1_048_576, 'RPC fault upstream status/bound')
            value = json.loads(data)
        finally:
            connection.close()
        with self._lock:
            selected = self._armed and method == self.MODES[self.mode] and 'result' in value
            if selected:
                self._armed = False
                self.trigger_count += 1
                if self.mode == 'moving_head':
                    require(self._callback is not None, 'head mutation requires a genuine producer')
                    self._callback()
                else:
                    self.corrupt(value)
                    self.mutated_frame_count += 1
        return value

    def corrupt(self, value):
        result = value['result']
        if self.mode == 'wrong_native_network':
            require(isinstance(result, str) and result.startswith('0x'), 'chain ID reply')
            value['result'] = hex(int(result, 16) ^ 1)
        elif self.mode == 'wrong_sequencer_key':
            key = result['keys'][0]
            encoded = key['publicKey']
            prefix = '0x' if encoded.startswith('0x') else ''
            raw = bytearray.fromhex(encoded.removeprefix('0x'))
            require(len(raw) == 32, 'receipt public key')
            raw[0] ^= 1
            key['publicKey'] = prefix + raw.hex()
        elif self.mode == 'wrong_header_signature':
            import base64
            signature = result['signedHeader']['commit']['signatures'][0]
            raw = bytearray(base64.b64decode(signature['signature'], validate=True))
            require(len(raw) == 64, 'Comet signature size')
            raw[0] ^= 1
            signature['signature'] = base64.b64encode(raw).decode()
        else:
            # The node's proof operations are retained; only the authenticated
            # root is changed. No proof bytes or positive answer are invented.
            root = result['storageHash']
            require(isinstance(root, str) and re.fullmatch('0x[0-9a-fA-F]{64}', root), 'account proof storage root')
            result['storageHash'] = '0x' + bytes([bytes.fromhex(root[2:])[0] ^ 1]).hex() + root[4:]

    def start(self):
        self.thread.start()
        return self

    def arm(self, *, on_selected=None):
        with self._lock:
            require(not self._armed, 'RPC fault already armed')
            self._armed = True
            self._callback = on_selected

    def disarm(self):
        with self._lock:
            self._armed = False

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        require(not self.thread.is_alive(), 'RPC fault relay did not stop')
