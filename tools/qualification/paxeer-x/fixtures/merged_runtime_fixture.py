"""Disposable merged paxd generation and its root JSON-RPC transport.

Native genesis, signer inputs and executable provenance are supplied by the
release kit. No standalone daemon, replica, or public router is started here.
"""

import hashlib
import base64
import http.client
import json
import os
from pathlib import Path
import re
import signal
import socket
import ssl
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
        self.path = self.url.path or '/'
        require(self.url.scheme == 'http' and self.url.hostname == '127.0.0.1'
                and self.url.port is not None
                and (self.path == '/' or re.fullmatch(r'/chain/[1-9][0-9]*', self.path))
                and not self.url.username and not self.url.password
                and not self.url.query and not self.url.fragment,
                'owned loopback chain RPC origin required')
        self.endpoint = endpoint.rstrip('/') + ('/' if self.path == '/' else '')
        self.identifier = 0

    def for_chain(self, registry_number):
        require(type(registry_number) is int and registry_number > 0, 'chain registry number')
        return RootRpc(f'http://127.0.0.1:{self.url.port}/chain/{registry_number}')

    def exchange(self, method, params):
        self.identifier += 1
        body = json.dumps({'jsonrpc': '2.0', 'id': self.identifier,
                           'method': method, 'params': params}, separators=(',', ':')).encode()
        connection = http.client.HTTPConnection(self.url.hostname, self.url.port, timeout=10)
        try:
            connection.request('POST', self.path, body, {'Content-Type': 'application/json'})
            response = connection.getresponse()
            raw = response.read(1_048_577)
            require(response.status == 200 and response.getheader('Content-Type', '').split(';')[0] == 'application/json'
                    and len(raw) <= 1_048_576, 'RPC HTTP status, type or body bound')
            value = json.loads(raw)
            require(isinstance(value, dict) and value.get('jsonrpc') == '2.0'
                    and type(value.get('id')) is int and value['id'] == self.identifier
                    and ('result' in value) != ('error' in value), 'RPC envelope')
            return value
        finally:
            connection.close()

    def request(self, method, params):
        value = self.exchange(method, params)
        require('error' not in value, 'RPC error: ' + str(value.get('error')))
        return value['result']


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

    def start(self, sequencer=True):
        require(self.process is None or self.process.poll() is not None, 'paxd already running')
        self.home = self.directory / 'chain'
        argv = [self.binary, 'start', '--home', str(self.home),
                '--rpc.laddr', 'tcp://127.0.0.1:' + str(self.consensus_port),
                '--sequencer.enable=' + ('true' if sequencer else 'false'),
                '--sequencer.wal-dir', str(self.home / 'sequencer-wal'),
                '--sequencer.receipt-key-file', self.config['bootstrap']['receipt_key_file']]
        if 'empty_block_interval' in self.config:
            argv.extend(('--consensus.create-empty-blocks-interval', self.config['empty_block_interval']))
        if 'sequencer_queue' in self.config:
            per_sender, total, lease = self.config['sequencer_queue']
            argv.extend(('--sequencer.nonce-queue-max-per-sender', str(per_sender),
                         '--sequencer.nonce-queue-max-total', str(total),
                         '--sequencer.lease-duration', lease))
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
        raise RuntimeError('merged runtime readiness timed out: ' + str(last)) from last

    def readiness(self):
        chain_id = int(self.config['bootstrap']['evm_chain_id'])
        self.wait(lambda: self.rpc.request('eth_chainId', []) == hex(chain_id))
        self.wait(lambda: int(self.rpc.request('eth_blockNumber', []), 16) > 0)

    def committed_readiness(self):
        def committed():
            connection = http.client.HTTPConnection('127.0.0.1', self.consensus_port, timeout=10)
            try:
                connection.request('GET', '/status')
                response = connection.getresponse()
                require(response.status == 200, 'consensus status response')
                status = json.loads(response.read(1_048_577))
                return int(status['sync_info']['latest_block_height']) > 0
            finally:
                connection.close()

        self.wait(committed)

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


def prepare_launch_runtime(directory, binary, revision, funded_accounts, funding_secret, quote_token):
    """Prepare a private single-validator genesis with current module defaults."""
    directory = Path(directory)
    directory.mkdir(mode=0o700)
    template = directory / 'template'
    template.mkdir(mode=0o700)
    inputs = directory / 'native-inputs'
    inputs.mkdir(mode=0o700)
    chain_id = 'hyperpax_125-1'

    def command(label, *argv):
        with (directory / (label + '.log')).open('xb') as log:
            result = subprocess.run([str(binary), *argv, '--home', str(template)],
                                    stdout=log, stderr=subprocess.STDOUT, timeout=120)
        require(result.returncode == 0, label + ' command')
        return (directory / (label + '.log')).read_text()

    command('init', 'init', 'chain-launch', '--chain-id', chain_id, '--mode', 'validator')
    command('validator-key', 'keys', 'add', 'validator', '--keyring-backend', 'test', '--output', 'json')
    command('validator-fund', 'add-genesis-account', 'validator',
            '100000000000000000000uhpx', '--keyring-backend', 'test')
    command('validator-gentx', 'gentx', 'validator', '7000000000000000uhpx',
            '--chain-id', chain_id, '--keyring-backend', 'test')
    command('validator-collect', 'collect-gentxs')
    receipt = directory / 'receipt.key'
    generated = json.loads(command('receipt-key', 'sequencer', 'gen-receipt-key', '--output', str(receipt)))
    command('receipt-pin', 'add-genesis-receipt-key', '--public-key', generated['public_key'],
            '--first-height', '1', '--last-height', str(2**64 - 1))
    validator = json.loads((template / 'config/priv_validator_key.json').read_text())
    leader = base64.b64decode(validator['pub_key']['value'], validate=True).hex()
    command('leader-pin', 'add-genesis-sequencer-leader', '--pubkey', leader)
    for address in funded_accounts:
        account = subprocess.run([str(binary), 'debug', 'addr', address[2:], '--home', str(template)],
                                 capture_output=True, text=True, timeout=30)
        require(account.returncode == 0, 'funded account conversion')
        found = re.findall(r'^Bech32 Acc: (pax1[0-9a-z]+)$', account.stdout, re.M)
        require(len(found) == 1, 'funded account address')
        command('fund-' + address[2:10].lower(), 'add-genesis-account', found[0], '1000000000uhpx')
    genesis = json.loads((template / 'config/genesis.json').read_text())
    genesis['validators'] = [{'address': validator['address'], 'pub_key': validator['pub_key'],
                              'power': '7000000000', 'name': 'chain-launch'}]
    genesis['app_state']['sequencer']['activation_height'] = 2
    genesis['app_state']['launchpad']['params']['quote_token'] = quote_token
    (template / 'config/genesis.json').write_text(json.dumps(genesis, separators=(',', ':')) + '\n')
    command('validate', 'validate-genesis')
    ports = reserve_ports(3)
    rpc_port, consensus_port, grpc_port = (sock.getsockname()[1] for sock in ports)
    for sock in ports:
        sock.close()
    app_config = (template / 'config/app.toml').read_text()
    for name, value in (('http_enabled', 'true'), ('http_address', '"127.0.0.1"'),
                        ('http_port', str(rpc_port))):
        app_config, count = re.subn(r'(?m)^' + name + r' = .*$', name + ' = ' + value, app_config)
        require(count == 1, 'EVM HTTP configuration ' + name)
    lines = app_config.splitlines(keepends=True)
    section = ''
    grpc_fields = set()
    for index, line in enumerate(lines):
        if line.startswith('['):
            section = line.strip()
        if section == '[grpc]':
            for name, value in (('enable', 'true'), ('address', f'"127.0.0.1:{grpc_port}"')):
                if line.startswith(name + ' = '):
                    lines[index] = name + ' = ' + value + '\n'
                    grpc_fields.add(name)
    require(grpc_fields == {'enable', 'address'}, 'local gRPC configuration')
    app_config = ''.join(lines)
    files = {'genesis.json': json.dumps(genesis, separators=(',', ':')).encode() + b'\n',
             'config.toml': (template / 'config/config.toml').read_bytes(),
             'app.toml': app_config.encode(),
             'priv_validator_key.json': (template / 'config/priv_validator_key.json').read_bytes(),
             'rpc-ca.der': ssl.PEM_cert_to_DER_cert((ROOT / 'services/platform/hosted/node/tests/fixtures/repair_22_31_355/tls-certificate.pem.txt').read_text())}
    for name, content in files.items():
        target = inputs / name
        target.write_bytes(content)
        target.chmod(0o600)
    funding_key = directory / 'funding.key'
    funding_key.write_text(f'{funding_secret:064x}\n')
    funding_key.chmod(0o600)
    config = {'paxd': {'path': str(binary), 'source_revision': revision,
                       'sha256': hashlib.sha256(Path(binary).read_bytes()).hexdigest()},
              'chain_rpc_url': f'http://127.0.0.1:{rpc_port}/',
              'consensus_rpc_url': f'http://127.0.0.1:{consensus_port}/',
              'empty_block_interval': '1s',
              'sequencer_queue': (16, 64, '3s'),
              'bootstrap': {'native_inputs': str(inputs), 'chain_id': chain_id,
                            'network_id': 402, 'evm_chain_id': 125,
                            'asset': hashlib.sha256(b'chain-launch-journey').hexdigest(),
                            'gateway_endpoint': 'https://localhost:9443',
                            'receipt_key_file': str(receipt),
                            'treasury_signer_socket': str(directory / 'treasury.sock'),
                            'funding_sender_key_file': str(funding_key)}}
    return RuntimeFixture(directory / 'runtime', config, revision)


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
