"""Disposable merged paxd generation and its root JSON-RPC transport.

Release-kit generations retain their native bootstrap contract. The inference
room generation instead creates its own validator, receipt key and funded bank
accounts through the same paxd CLI used by native operators.
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


def seed_inference_programs_genesis(genesis, accounts, bank_mirror):
    """Install the committed fixture's v1 schedules and fee accounts at genesis."""
    host_account = accounts['host']
    public = bytes.fromhex(host_account['peer_key'])
    actor = bytes.fromhex(host_account['principal'])
    main_id = bytes.fromhex(host_account['main_account'])
    main_name = host_account['account_name']
    require(len(public) == len(actor) == len(main_id) == 32
            and main_name == 'agent:' + host_account['did'] + ':main',
            'generated programs genesis authority')

    def uint32(value):
        return value.to_bytes(4, 'big')

    def uint64(value):
        return value.to_bytes(8, 'big')

    def amount(value):
        return value.to_bytes(16, 'big')

    def field(value):
        return uint32(len(value)) + value

    def account_id(name):
        return hashlib.sha256(b'LX:ACCOUNT:v1' + uint32(len(name)) + name).digest()

    require(main_id == account_id(main_name.encode()), 'generated main account identifier')
    salt = b'\x55' * 32
    asset = hashlib.sha256(b'LX:ASSET:v1' + actor + salt).digest()
    treasury_name = 'system:fees'
    treasury_id = account_id(treasury_name.encode())
    native_salt = b'\x66' * 32
    native_asset = hashlib.sha256(b'LX:ASSET:v1' + actor + native_salt).digest()
    custody = genesis['app_state']['layerxcustody']
    mappings = custody['assets'] or []
    require(not any(row['denom'] == 'uhpx' or row['asset_id'] == native_asset.hex()
                    for row in mappings), 'native custody asset already configured')
    custody['assets'] = [*mappings, {'asset_id': native_asset.hex(), 'denom': 'uhpx',
                                   'pointer': '', 'enabled': True, 'paused': False,
                                   'minimum_deposit': '0', 'custody_cap': ''}]
    pay = genesis['app_state']['layerxpay']
    pay['params'].update(paused=False, stream_enabled=True)
    gate_key = b'\x01\x00\x07' + b'module-enable:stream'.ljust(32, b'\x00')
    gates = pay['kernel_records'] or []
    require(not any(base64.b64decode(row['key'], validate=True) == gate_key for row in gates),
            'stream module already configured')
    pay['kernel_records'] = [*gates, {'key': base64.b64encode(gate_key).decode(),
                                     'value': base64.b64encode(bytes(31) + b'\x01').decode()}]

    fee = b'LXFR1' + uint32(1) + uint64(1) * 7 + asset
    fee += b''.join(uint64(value) for value in (1000, 8, 1, 8, 1, 100, 1, 0, 1))
    fee += actor + b'\x00' * 16
    metering = b'LXMR1' + uint32(1)
    metering += b''.join(uint64(value) for value in (1, 1, 1, 1, 1, 8, 8, 64, 8))
    metering += uint64(1) + b'\x01' + actor

    issuance = b'\x00\x03' + asset + bytes((1, ord('T'), 6, 1, 0, 0, 0, 1, ord('T')))
    issuance += amount(2_000_000_000) + actor + b'\x01' + amount(1_250_000_000) + salt
    rows = [(b'\x01\x00\x01asset:' + asset, issuance)]
    native_issuance = b'\x00\x03' + native_asset + bytes((1, ord('N'), 6, 1, 0, 0, 0, 1, ord('N')))
    native_issuance += amount(1_000_000_000) + actor + b'\x01' + amount(1_000_000_000) + native_salt
    rows.append((b'\x01\x00\x01asset:' + native_asset, native_issuance))
    payers = []
    mains = []
    for role in ('host', 'peer0', 'peer1', 'asker', 'evidence0', 'evidence1', 'evidence2'):
        account = accounts[role]
        account_main = account['account_name']
        account_main_id = bytes.fromhex(account['main_account'])
        require(account_main_id == account_id(account_main.encode()),
                'generated main account identifier')
        mains.append((account_main, account_main_id, 1,
                      400_000_000 if role == 'host' else 100_000_000,
                      bytes.fromhex(account['peer_key'])))
        name = 'agent:' + account['did'] + ':asset:' + asset.hex()
        payers.append((name, account_id(name.encode()), 14,
                       500_000_000 if role == 'host' else 125_000_000,
                       bytes.fromhex(account['peer_key'])))
    for _, identifier, _, balance, _ in payers:
        bank_mirror(identifier, asset, balance)

    for name, identifier, kind, balance, authority in (
            [(treasury_name, treasury_id, 10, 0, None)] + mains + payers):
        account_asset = native_asset if kind == 1 else asset
        value = field(identifier) + field(name.encode()) + bytes((kind,)) + amount(balance)
        value += field(account_asset) + b'\x01' + uint64(0) + uint64(0) + b'\x00\x00'
        value += field(authority if authority is not None else b'\x00' * 32)
        value += bytes((int(authority is not None),))
        rows.append((b'\x06' + identifier, value))

    prefix = b'\x01\x00\x09'
    rows.extend((prefix + key, value) for key, value in (
        (b'progfee/active/v1', fee),
        (b'progfee/history/v1/' + uint32(1), fee),
        (b'progmet/active/v1', metering),
        (b'progmet/history/v1/' + uint32(1), metering),
    ))
    host = genesis['app_state']['lxphost']
    existing = host['records'] or []
    decoded = [(base64.b64decode(row['key'], validate=True),
                base64.b64decode(row['value'], validate=True)) for row in existing]
    keys = {key for key, _ in decoded}
    require(all(key not in keys for key, _ in rows), 'programs genesis record already exists')
    decoded.extend(rows)
    decoded.sort(key=lambda row: row[0])
    require(all(left[0] < right[0] for left, right in zip(decoded, decoded[1:])),
            'strictly ascending programs genesis keys')
    host['records'] = [{'key': base64.b64encode(key).decode(),
                        'value': base64.b64encode(value).decode()} for key, value in decoded]


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


class ChainWriter:
    """Submit signed bytes to an owned root and retain each attempted write."""

    def __init__(self, directory, rpc, wait):
        self.directory, self.rpc, self.wait = Path(directory), rpc, wait

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
        if receipt['status'] != '0x1':
            transaction = self.rpc.request('eth_getTransactionByHash', [reference])
            call = {key: transaction[key] for key in ('from', 'to', 'gas', 'value')}
            call['data'] = transaction['input']
            diagnostic = None
            try:
                diagnostic = {'result': self.rpc.request('eth_call', [call, receipt['blockNumber']])}
            except (OSError, ValueError, RuntimeError, http.client.HTTPException) as error:
                diagnostic = {'error': str(error)}
            consensus = getattr(self, 'config', {}).get('consensus_rpc_url')
            if consensus:
                try:
                    diagnostic['block_results'] = RootRpc(consensus).request(
                        'block_results', {'height': str(int(receipt['blockNumber'], 16))})
                except (OSError, ValueError, RuntimeError, http.client.HTTPException) as error:
                    diagnostic['block_results_error'] = str(error)
            try:
                diagnostic['trace'] = self.rpc.request('debug_traceTransaction', [reference, {'tracer': 'callTracer'}])
            except (OSError, ValueError, RuntimeError, http.client.HTTPException) as error:
                diagnostic['trace_error'] = str(error)
            write_json(self.directory / ('refused-' + reference[2:] + '.json'),
                       {'receipt': receipt, 'transaction': transaction, 'replay': diagnostic})
            trace = diagnostic.get('trace')
            summary = {key: trace[key] for key in ('error', 'revertReason', 'output') if key in trace} \
                if isinstance(trace, dict) else diagnostic
            node_log = self.directory / 'paxd.log'
            require(node_log.is_file(), 'native node log required at ' + str(node_log))
            tail = node_log.read_text(errors='replace').splitlines()[-150:]
            print('native node log: ' + str(node_log) + '\n' + '\n'.join(tail),
                  file=sys.stderr, flush=True)
            require(False, 'successful merged receipt at height ' + receipt['blockNumber']
                    + ' result ' + receipt['status'] + '; native replay: ' + json.dumps(summary)
                    + '; native node log: ' + str(node_log))
        require(receipt['transactionHash'] == reference and receipt['status'] == '0x1', 'successful merged receipt')
        return receipt

    def recorded_submit(self, name, transaction):
        require(re.fullmatch(r'[a-z0-9_-]+', name), 'chain write evidence name')
        path = self.directory / ('write-' + name + '.json')
        require(not path.exists(), 'chain write evidence already exists')
        record = {'name': name, 'signed_transaction': transaction}
        try:
            receipt = self.submit(transaction)
            record['receipt'] = receipt
            return receipt
        except BaseException as error:
            record['error'] = str(error)
            raise
        finally:
            write_json(path, record)


class RuntimeFixture(ChainWriter):
    @classmethod
    def for_inference_room(cls, directory, revision, paxd, accounts):
        """Create a chain-125 generation without prepared genesis or signed writes."""
        directory = Path(directory)
        require(isinstance(accounts, dict) and accounts, 'generated room accounts')
        addresses = set()
        for name, account in accounts.items():
            require(re.fullmatch(r'[a-z0-9_-]+', name)
                    and isinstance(account, dict)
                    and re.fullmatch(r'0x[0-9a-f]{40}', account['address'])
                    and account['address'] not in addresses, 'distinct generated room account')
            addresses.add(account['address'])
        reservations = reserve_ports(4)
        rpc, consensus, p2p, grpc = [sock.getsockname()[1] for sock in reservations]
        config = {
            'paxd': paxd,
            'chain_rpc_url': 'http://127.0.0.1:' + str(rpc),
            'consensus_rpc_url': 'http://127.0.0.1:' + str(consensus),
            'bootstrap': {'evm_chain_id': 125,
                          'receipt_key_file': str(directory / 'receipt-key.hex')},
            'generated_accounts': accounts,
            'generated_ports': {'p2p': p2p, 'grpc': grpc},
            'sequencer_queue': (16, 64, '3s'),
        }
        try:
            fixture = cls(directory, config, revision)
        except BaseException:
            for sock in reservations:
                sock.close()
            raise
        fixture.port_reservations = reservations
        return fixture

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
        if 'generated_accounts' in self.config:
            self.generate_inference_room()
            return
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

    def generate_inference_room(self):
        """Use product genesis commands; never seed private program state."""
        chain_id = 'hyperpax_125-1'
        with (self.directory / 'bootstrap.log').open('xb') as log:
            def command(*arguments):
                argv = [self.binary, *map(str, arguments), '--home', str(self.home)]
                result = subprocess.run(argv, cwd=ROOT, env=self.env, stdin=subprocess.DEVNULL,
                                        stdout=subprocess.PIPE, stderr=log, timeout=120)
                log.write(result.stdout)
                log.flush()
                require(result.returncode == 0,
                        'native genesis command ' + str(arguments[0]) + ' exited '
                        + str(result.returncode) + '; inspect ' + str(self.directory / 'bootstrap.log'))
                return result.stdout.decode('utf-8')

            command('init', 'inference-room', '--chain-id', chain_id, '--mode', 'validator')
            command('keys', 'add', 'room-validator', '--keyring-backend', 'test',
                    '--no-backup', '--output', 'json')
            validator = json.loads(command('keys', 'show', 'room-validator',
                                           '--keyring-backend', 'test', '--output', 'json'))
            genesis_path = self.home / 'config/genesis.json'
            genesis = json.loads(genesis_path.read_text())
            genesis['app_state']['staking']['params']['max_voting_power_ratio'] = '1.000000000000000000'
            genesis['app_state']['oracle']['params']['min_valid_per_window'] = '0.000000000000000000'
            genesis['consensus_params']['timeout']['commit'] = '1000000000'
            genesis_path.write_text(json.dumps(genesis) + '\n')
            command('add-genesis-account', validator['address'], '1000000000000000000000000uhpx',
                    '--keyring-backend', 'test')
            funded = {}
            for name, account in self.config['generated_accounts'].items():
                converted = command('debug', 'addr', account['address'][2:])
                addresses = [line.removeprefix('Bech32 Acc: ').strip()
                             for line in converted.splitlines() if line.startswith('Bech32 Acc: ')]
                require(len(addresses) == 1, 'native account address conversion')
                command('add-genesis-account', addresses[0], '1000000000000000000000000uhpx',
                        '--keyring-backend', 'memory')
                funded[name] = {'address': account['address'], 'bank_address': addresses[0],
                                'denom': 'uhpx', 'amount': '1000000000000000000000000'}
            command('gentx', 'room-validator', '1000000000uhpx', '--chain-id', chain_id,
                    '--keyring-backend', 'test', '--ip', '127.0.0.1',
                    '--p2p-port', self.config['generated_ports']['p2p'])
            command('collect-gentxs')
            public = json.loads(command('sequencer', 'gen-receipt-key', '--output',
                                        self.config['bootstrap']['receipt_key_file']))
            command('add-genesis-receipt-key', '--public-key', public['public_key'],
                    '--first-height', '1', '--last-height', '9223372036854775807')
            validator_key = json.loads((self.home / 'config/priv_validator_key.json').read_text())
            consensus_public = base64.b64decode(validator_key['pub_key']['value'], validate=True)
            command('add-genesis-sequencer-leader', '--pubkey', consensus_public.hex())
            genesis = json.loads(genesis_path.read_text())
            genesis['validators'] = [{'address': validator_key['address'],
                                      'pub_key': validator_key['pub_key'],
                                      'power': '1000', 'name': 'room-validator'}]
            genesis['app_state']['sequencer']['activation_height'] = 2
            associations = genesis['app_state']['evm']['address_associations'] or []
            associations.extend({'eth_address': item['address'], 'pax_address': item['bank_address']}
                                for item in funded.values())
            genesis['app_state']['evm']['address_associations'] = associations
            def bank_mirror(identifier, asset, balance):
                material = subprocess.run(
                    [self.config['ledger_encoder'], 'bank-mirror', identifier.hex(), asset.hex()],
                    cwd=ROOT, env=self.env, capture_output=True, text=True, timeout=30)
                require(material.returncode == 0, 'native ledger bank mirror identifiers')
                mirror = json.loads(material.stdout)
                converted = command('debug', 'addr', mirror['address'])
                addresses = [line.removeprefix('Bech32 Acc: ').strip()
                             for line in converted.splitlines() if line.startswith('Bech32 Acc: ')]
                require(len(addresses) == 1, 'native ledger bank mirror address')
                command('add-genesis-account', addresses[0], str(balance) + mirror['denom'],
                        '--keyring-backend', 'memory')
                return addresses[0]

            mirrors = []
            seed_inference_programs_genesis(
                genesis, self.config['generated_accounts'],
                lambda identifier, asset, balance: mirrors.append((identifier, asset, balance)))
            genesis_path.write_text(json.dumps(genesis) + '\n')
            for identifier, asset, balance in mirrors:
                bank_mirror(identifier, asset, balance)
            command('validate-genesis', genesis_path)
            write_json(self.home / 'data/upgrade-info.json',
                       {'name': 'v7.0', 'height': 1,
                        'info': json.dumps(genesis['app_state']['sequencer'])})
        self.configure_inference_listeners()
        for path in self.home.rglob('*'):
            if path.is_file():
                path.chmod(0o600)
            elif path.is_dir():
                path.chmod(0o700)
        self.manifest.update(chain_id=chain_id, network_id=125,
                             consensus_rpc_url=self.config['consensus_rpc_url'],
                             validators=[{'pub_key': list(consensus_public)}],
                             genesis_sha256=hashlib.sha256(genesis_path.read_bytes()).hexdigest(),
                             sequencer_public_key=public['public_key'], funded_accounts=funded)
        write_json(self.directory / 'genesis-funding.json', funded)
        write_json(self.directory / 'runtime.json', self.manifest)

    def configure_inference_listeners(self):
        def update(path, changes):
            section = ''
            remaining = dict(changes)
            lines = []
            for line in path.read_text().splitlines():
                stripped = line.strip()
                if stripped.startswith('[') and stripped.endswith(']'):
                    section = stripped[1:-1]
                key = stripped.split('=', 1)[0].strip()
                identity = (section, key)
                if '=' in stripped and identity in remaining:
                    line = key + ' = ' + remaining.pop(identity)
                lines.append(line)
            require(not remaining, 'native listener configuration fields absent: ' + str(remaining))
            path.write_text('\n'.join(lines) + '\n')
        update(self.home / 'config/config.toml', {
            ('rpc', 'laddr'): json.dumps('tcp://127.0.0.1:' + str(self.consensus_port)),
            ('p2p', 'laddr'): json.dumps('tcp://127.0.0.1:' + str(self.config['generated_ports']['p2p'])),
            ('consensus.sequencer', 'enable'): 'true',
            ('consensus.sequencer', 'timeout-commit'): '"1s"',
        })
        if 'inference_program' in self.config:
            app_path = self.home / 'config/app.toml'
            app_text = app_path.read_text().replace('[evm]', '[evm]\npx_inference_program = ' + json.dumps(self.config['inference_program'])\
                + '\npx_inference_reader = ' + json.dumps(self.config['inference_reader']), 1)
            app_path.write_text(app_text)
        update(self.home / 'config/app.toml', {
            ('evm', 'http_enabled'): 'true',
            ('evm', 'http_address'): '"127.0.0.1"',
            ('evm', 'http_port'): str(self.rpc.url.port),
            ('grpc', 'enable'): 'true',
            ('grpc', 'address'): json.dumps('127.0.0.1:' + str(self.config['generated_ports']['grpc'])),
            ('grpc-web', 'enable'): 'false',
            ('api', 'enable'): 'false',
        })

    def start(self, sequencer=True):
        require(self.process is None or self.process.poll() is not None, 'paxd already running')
        for sock in getattr(self, 'port_reservations', []):
            sock.close()
        self.port_reservations = []
        self.home = self.directory / 'chain'
        if 'generated_accounts' in self.config and not (self.directory / 'genesis-commit.json').exists():
            bootstrap = [self.binary, 'start', '--home', str(self.home),
                         '--sequencer.enable=false', '--halt-height', '1']
            with (self.directory / 'genesis-commit.log').open('xb') as log:
                result = subprocess.run(bootstrap, env=self.env, stdout=log, stderr=log, timeout=120)
            require(result.returncode == 0,
                    'native genesis commit failed; inspect ' + str(self.directory / 'genesis-commit.log'))
            validator_state = json.loads((self.home / 'data/priv_validator_state.json').read_text())
            require(int(validator_state['height']) == 1, 'native genesis must halt at height one before activation')
            write_json(self.directory / 'genesis-commit.json',
                       {'command': bootstrap, 'exit': result.returncode, 'validator_state': validator_state})
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
            require(self.process is not None, 'paxd was not started')
            if self.process.poll() is not None:
                path = self.directory / 'paxd.log'
                with path.open('rb') as log:
                    lines = log.read(16384).decode('utf-8', errors='replace').splitlines()
                message = next((line for line in lines if line.startswith(('panic:', 'Error:'))
                                or 'level=ERROR' in line or 'level=error' in line),
                               lines[0] if lines else '(no startup output)')
                evidence = self.directory / 'paxd-failure.json'
                if not evidence.exists():
                    write_json(evidence, {'exit': self.process.returncode,
                                          'first_message': message, 'log': str(path)})
                raise RuntimeError('paxd exited ' + str(self.process.returncode) + ': '
                                   + message + '; inspect ' + str(path))
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
        minimum_height = 1 if 'generated_accounts' in self.config else 0
        self.wait(lambda: int(self.rpc.request('eth_blockNumber', []), 16) > minimum_height)
        if 'generated_accounts' in self.config:
            consensus = RootRpc(self.config['consensus_rpc_url'])
            validators = consensus.request('validators', {})['validators']
            require(len(validators) == 1, 'isolated chain must have its one generated validator')
            validator = validators[0]
            public_key = list(base64.b64decode(validator['pub_key']['value'], validate=True))
            require(public_key == self.manifest['validators'][0]['pub_key']
                    and validator['pub_key']['type'] == 'tendermint/PubKeyEd25519',
                    'consensus validator differs from the generated genesis key')
            power = int(validator['voting_power'])
            require(power > 0, 'isolated validator must have positive voting power')
            self.manifest['validators'][0]['voting_power'] = power
            write_json(self.directory / 'validator-authority.json',
                       {'chain_id': self.manifest['chain_id'],
                        'validators': self.manifest['validators']})

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
        return super().submit(transaction)

    def restart(self):
        before = self.rpc.request('eth_getBlockByNumber', ['latest', False])
        self.cleanup()
        self.start()
        self.readiness()
        retained = self.rpc.request('eth_getBlockByNumber', [before['number'], False])
        require(retained == before, 'merged restart changed a retained committed block')

    def collect_chain_writes(self, first_height=1):
        """Retain node-produced transactions and receipts, including daemon writes."""
        last = int(self.rpc.request('eth_blockNumber', []), 16)
        require(type(first_height) is int and 0 <= first_height <= last,
                'chain evidence height range')
        from layerx_sdk.account_derivation import keccak256
        submitted = {}
        for path in self.directory.glob('write-*.json'):
            record = json.loads(path.read_text())
            raw = record['signed_transaction']
            require(isinstance(raw, str) and re.fullmatch('0x([0-9a-f]{2})+', raw),
                    'recorded signed chain transaction')
            reference = '0x' + keccak256(bytes.fromhex(raw[2:])).hex()
            require(reference not in submitted or submitted[reference] == raw,
                    'recorded transaction hash binds identical bytes')
            submitted[reference] = raw
        rows = []
        for height in range(first_height, last + 1):
            block = self.rpc.request('eth_getBlockByNumber', [hex(height), False])
            require(isinstance(block, dict) and block['number'] == hex(height),
                    'chain evidence block identity')
            for reference in block['transactions']:
                raw = submitted[reference] if reference in submitted else self.rpc.request(
                    'eth_getRawTransactionByHash', [reference])
                receipt = self.rpc.request('eth_getTransactionReceipt', [reference])
                require(isinstance(raw, str) and re.fullmatch('0x([0-9a-f]{2})+', raw)
                        and '0x' + keccak256(bytes.fromhex(raw[2:])).hex() == reference
                        and isinstance(receipt, dict) and receipt['transactionHash'] == reference
                        and receipt['blockHash'] == block['hash']
                        and receipt['blockNumber'] == block['number'],
                        'chain evidence transaction inclusion')
                rows.append({'transaction_hash': reference, 'raw_transaction': raw,
                             'receipt': receipt})
        write_json(self.directory / 'chain-writes.json',
                   {'first_height': first_height, 'last_height': last, 'transactions': rows})
        return rows

    def cleanup(self):
        for sock in getattr(self, 'port_reservations', []):
            sock.close()
        self.port_reservations = []
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
