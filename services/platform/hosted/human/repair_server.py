"""Loopback JSON-RPC peer for the hosted-human repair contract.

This fixture stores ledger accounts, bindings and receipts locally. Wire values
are encoded with the production EVM codec and the committed precompile ABIs;
no native host, subprocess, environment configuration or remote node is used.
"""
import datetime
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import ssl
import threading
import time

from cryptography import x509
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ed25519, rsa
from cryptography.hazmat.primitives.serialization import Encoding, NoEncryption, PrivateFormat
from cryptography.x509.oid import NameOID

from merged_owner import ADDR, AUTH, LEDGER, PAYMENT, account_id, evm, transaction_fields
from owner_native import digest

ROOT = Path(__file__).resolve().parents[4]


def tls_files(directory):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'localhost')])
    now = datetime.datetime.now(datetime.timezone.utc)
    certificate = (x509.CertificateBuilder().subject_name(subject).issuer_name(subject).public_key(key.public_key())
        .serial_number(x509.random_serial_number()).not_valid_before(now - datetime.timedelta(minutes=1))
        .not_valid_after(now + datetime.timedelta(days=1))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .add_extension(x509.SubjectAlternativeName([x509.DNSName('localhost')]), critical=False)
        .sign(key, hashes.SHA256()))
    certificate_file = directory / 'rpc-ca.pem'
    certificate_file.write_bytes(certificate.public_bytes(Encoding.PEM))
    certificate_file.chmod(0o600)
    key_file = directory / 'rpc-server.pem'
    key_file.write_bytes(key.private_bytes(Encoding.PEM, PrivateFormat.PKCS8, NoEncryption()))
    key_file.chmod(0o600)
    return certificate_file, key_file


class LoopbackRpcServer:
    def __init__(self, directory, port=0):
        self.directory = Path(directory)
        self.store = self.directory / 'rpc-state.json'
        self.bindings, self.nonces, self.identities = {}, {}, {}
        self.receipts, self.raw, self.blocks = {}, {}, {}
        self.accounts, self.assets, self.budgets = {}, {}, {}
        self.main_accounts = []
        self.global_sequence = 0
        self.requests = []
        self.lock = threading.Lock()
        self.state_fields = ('bindings', 'nonces', 'identities', 'receipts', 'raw', 'blocks',
                             'accounts', 'assets', 'budgets', 'main_accounts', 'global_sequence')
        if self.store.exists():
            state = json.loads(self.store.read_bytes())
            assert set(state) == set(self.state_fields), 'persisted RPC fixture schema'
            for name in self.state_fields:
                setattr(self, name, state[name])
        self.abis, self.events = {}, {}
        for address, module in [(ADDR, 'addr'), (AUTH, 'layerxauth'), (LEDGER, 'layerxledger'), (PAYMENT, 'layerxpay')]:
            for row in json.loads((ROOT / 'precompiles' / module / 'abi.json').read_text()):
                if row['type'] == 'function' and all('tuple' not in v['type'] for v in row['inputs']):
                    signature = row['name'] + '(' + ','.join(v['type'] for v in row['inputs']) + ')'
                    self.abis[(address, evm.selector(signature))] = row
                elif row['type'] == 'event':
                    self.events[(address, row['name'])] = row
        self.ca = self.directory / 'rpc-ca.pem'
        key = self.directory / 'rpc-server.pem'
        if not self.ca.exists():
            self.ca, key = tls_files(directory)
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                assert self.client_address[0] == '127.0.0.1', 'loopback peer only'
                assert self.headers.get('Content-Type') == 'application/json', 'JSON content type'
                length = int(self.headers['Content-Length'])
                assert 0 < length <= 1048576, 'bounded RPC request'
                call = json.loads(self.rfile.read(length))
                assert set(call) == {'jsonrpc', 'id', 'method', 'params'} and call['jsonrpc'] == '2.0', 'JSON-RPC envelope'
                response = dict(jsonrpc='2.0', id=call['id'])
                with owner.lock:
                    owner.requests.append(call)
                    try:
                        response['result'] = owner.request(call['method'], call['params'])
                    except (AssertionError, ValueError, KeyError) as error:
                        response['error'] = dict(code=-32000, message=type(error).__name__ + ': ' + str(error))
                body = json.dumps(response).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        self.server = ThreadingHTTPServer(('127.0.0.1', port), Handler)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.load_cert_chain(str(self.ca), str(key))
        self.server.socket = tls.wrap_socket(self.server.socket, server_side=True)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.url = 'https://localhost:' + str(self.server.server_port)
        self.rpc = evm.Rpc(self.url, str(self.ca))

    def persist(self):
        pending = self.store.with_suffix('.pending')
        pending.write_text(json.dumps({name: getattr(self, name) for name in self.state_fields}))
        pending.chmod(0o600)
        pending.replace(self.store)

    def genesis_main(self, public):
        assert self.global_sequence == 0 and not self.raw, 'genesis must precede RPC execution'
        self.main_accounts.append(public)
        self.persist()

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        assert not self.thread.is_alive(), 'RPC server stopped'

    def restart(self):
        port = self.server.server_port
        self.persist()
        self.close()
        # Discard all stores and reopen them along with the listening socket.
        self.__init__(self.directory, port)

    def balance(self, account):
        row = self.accounts[account]
        return row['balance'], row['sequence']

    def decode_call(self, target, data):
        raw = evm.unhex(data)
        row = self.abis[(target, raw[:4])]
        types = [v['type'] for v in row['inputs']]
        args = evm.decode(types, raw[4:])
        assert evm.encode(types, args) == raw[4:], 'noncanonical calldata'
        return row, args

    def log(self, address, name, values):
        row = self.events[(address, name)]
        assert len(values) == len(row['inputs']), 'event field count'
        signature = name + '(' + ','.join(v['type'] for v in row['inputs']) + ')'
        topics = ['0x' + evm.keccak(signature.encode()).hex()]
        types, data = [], []
        for field, value in zip(row['inputs'], values):
            if field['indexed']:
                topics.append('0x' + evm.encode([field['type']], [value]).hex())
            else:
                types.append(field['type'])
                data.append(value)
        return dict(address=address, topics=topics, data='0x' + evm.encode(types, data).hex())

    def lifecycle(self, public, payload):
        did = ('did:layerx:' + public).encode()
        did_id = digest(b'did-id', len(did).to_bytes(2, 'big') + did)
        assert payload[4:36] == did_id, 'lifecycle DID binding'
        if payload[:4] == b'\x71\1\0\2':
            assert len(payload) == 68 and payload[36:] == bytes.fromhex(public), 'registration key'
            assert public not in self.identities, 'identity already registered'
            state = bytearray(b'LXGI1' + did_id + bytes.fromhex(public) + bytes(154))
            state[69:77] = (self.global_sequence + 1).to_bytes(8, 'big')
        else:
            assert payload[:4] == b'\x71\3\0\5' and len(payload) == 86, 'recovery lifecycle framing'
            state = bytearray.fromhex(self.identities[public])
            assert any(payload[36:68]) and int.from_bytes(payload[68:70], 'big') > 0, 'recovery guardians'
            required = int.from_bytes(payload[70:78], 'big')
            maximum = int.from_bytes(payload[78:86], 'big')
            assert 0 < required <= maximum, 'recovery delay bounds'
            state[77:111] = payload[36:70]
            state[175:183] = (1).to_bytes(8, 'big')
            state[183:199] = payload[70:86]
        state[215:223] = (self.global_sequence + 1).to_bytes(8, 'big')
        assert len(state) == 223, 'LXGI1 state framing'
        self.identities[public] = state.hex()
        return self.log(AUTH, 'IdentityStateUpdated', ['0x' + did_id.hex(), bytes(state)])

    def transfer(self, source, destination, asset, amount):
        debit, credit = self.accounts[source[2:]], self.accounts[destination[2:]]
        assert debit['asset'] == credit['asset'] == asset[2:], 'ledger asset binding'
        assert 0 < amount <= debit['balance'], 'funded ledger debit'
        debit['balance'] -= amount
        debit['sequence'] += 1
        credit['balance'] += amount

    def transaction(self, signed):
        tx = transaction_fields(signed, 125)
        if tx['hash'] in self.receipts:
            assert self.raw[tx['hash']] == signed, 'identical replay'
            return tx['hash']
        assert tx['value'] == 0 and tx['nonce'] == self.nonces.get(tx['sender'], 0), 'transaction nonce/value'
        row, args = self.decode_call(tx['to'], tx['data'])
        method = row['name']
        logs = []
        if method == 'bindLayerX':
            key, signature = args
            assert tx['sender'] not in self.bindings and key[2:] not in self.bindings.values(), 'unique binding'
            nonce = 0
            ed25519.Ed25519PublicKey.from_public_bytes(evm.unhex(key)).verify(evm.unhex(signature),
                b'LX:PAXEER-BIND:v1' + (125).to_bytes(32, 'big') + evm.unhex(tx['sender']) + nonce.to_bytes(8, 'big'))
            self.bindings[tx['sender']] = key[2:]
            logs.append(self.log(ADDR, 'LayerXBound', [tx['sender'], key, nonce]))
        else:
            public = self.bindings[tx['sender']]
            if method == 'recordLifecycle':
                payload, action = map(evm.unhex, args)
                assert len(action) == 32 and any(action), 'lifecycle action'
                logs.append(self.lifecycle(public, payload))
            elif method == 'register':
                asset, salt, symbol, name, decimals, cap, issuer, custody = args
                did = ('did:layerx:' + public).encode()
                did_id = digest(b'did-id', len(did).to_bytes(2, 'big') + did)
                assert asset[2:] == hashlib.sha256(b'LX:ASSET:v1' + did_id + evm.unhex(salt)).hexdigest(), 'asset derivation'
                assert asset[2:] not in self.assets and cap > 0 and issuer == 1 and custody == '0x', 'native asset registration'
                self.assets[asset[2:]] = dict(issuer=public, cap=cap, supply=0)
                logs.append(self.log(LEDGER, 'AssetRegistered', [asset]))
            elif method == 'openAccount':
                asset, = args
                assert asset[2:] in self.assets, 'registered asset'
                account = account_id(public, asset[2:])
                assert account not in self.accounts, 'new asset account'
                self.accounts[account] = dict(owner=public, asset=asset[2:], balance=0, sequence=0)
                logs.append(self.log(LEDGER, 'AccountOpened', ['0x' + account]))
            elif method == 'mint':
                asset, account, amount = args
                token, target = self.assets[asset[2:]], self.accounts[account[2:]]
                assert token['issuer'] == public and target['asset'] == asset[2:], 'mint authority/asset'
                assert amount > 0 and token['supply'] + amount <= token['cap'], 'mint supply cap'
                token['supply'] += amount
                target['balance'] += amount
                logs.append(self.log(LEDGER, 'Transfer', ['0x' + bytes(32).hex(), account, asset, amount, 10]))
            elif method == 'payExact':
                source, destination, asset, amount, expiry, reference = args
                assert source[2:] == account_id(public, asset[2:]), 'payment source authority'
                assert expiry > time.time_ns() // 1000000, 'payment expiry'
                self.transfer(source, destination, asset, amount)
                logs += [self.log(LEDGER, 'Transfer', [source, destination, asset, amount, 5]),
                         self.log(LEDGER, 'PaymentExact', [reference, source, destination, asset, amount])]
            elif method == 'budgetCreateNative':
                budget, account, asset, purpose, cap, period_cap, amount, period, start, expiry, revision, rollover, source, sequence = args
                assert public in self.main_accounts, 'genesis main account prerequisite'
                assert source[2:] == account_id(public, asset[2:]), 'budget source authority'
                assert sequence == self.balance(source[2:])[1], 'budget funding sequence'
                assert 0 < amount <= cap and start < expiry and period > 0 and rollover == 1, 'budget bounds'
                assert budget not in self.budgets and account[2:] not in self.accounts, 'new budget'
                self.accounts[account[2:]] = dict(owner=public, asset=asset[2:], balance=0, sequence=0)
                self.transfer(source, account, asset, amount)
                self.budgets[budget] = dict(owner=public, account=account, asset=asset, remaining=amount, expiry=expiry)
                logs.append(self.log(PAYMENT, 'BudgetCreated', [budget, account, amount]))
            elif method == 'budgetSpend':
                budget, destination, amount = args
                state = self.budgets[budget]
                assert state['owner'] == public and time.time_ns() // 1000000 < state['expiry'], 'live owner budget'
                assert 0 < amount <= state['remaining'], 'funded budget spend'
                self.transfer(state['account'], destination, state['asset'], amount)
                state['remaining'] -= amount
                logs.append(self.log(PAYMENT, 'BudgetSpent', args))
            else:
                raise ValueError('unsupported loopback peer method: ' + method)
        self.global_sequence += 1
        self.nonces[tx['sender']] = tx['nonce'] + 1
        block_number = len(self.blocks) + 1
        block_hash = '0x' + evm.keccak(evm.unhex(tx['hash']) + block_number.to_bytes(8, 'big')).hex()
        block = dict(number=hex(block_number), hash=block_hash, transactions=[tx['hash']])
        self.blocks[block['number']] = block
        for index, log in enumerate(logs):
            log.update(transactionHash=tx['hash'], transactionIndex='0x0', blockHash=block_hash,
                       blockNumber=hex(block_number), logIndex=hex(index), removed=False)
        self.receipts[tx['hash']] = dict(transactionHash=tx['hash'], blockHash=block_hash,
            blockNumber=hex(block_number), status='0x1', to=tx['to'], **{'from': tx['sender']}, logs=logs)
        self.raw[tx['hash']] = signed
        self.persist()
        return tx['hash']

    def request(self, method, params):
        if method == 'eth_chainId':
            return '0x7d'
        if method == 'eth_blockNumber':
            return hex(len(self.blocks))
        if method == 'eth_getBlockByNumber':
            return self.blocks[params[0]]
        if method == 'eth_getTransactionCount':
            return hex(self.nonces.get(params[0].lower(), 0))
        if method == 'eth_gasPrice':
            return '0x1'
        if method == 'eth_estimateGas':
            self.decode_call(params[0]['to'].lower(), params[0]['data'])
            return '0x7a120'
        if method == 'eth_sendRawTransaction':
            return self.transaction(params[0])
        if method == 'eth_getTransactionReceipt':
            return self.receipts.get(params[0])
        if method == 'eth_call':
            row, args = self.decode_call(params[0]['to'].lower(), params[0]['data'])
            name = row['name']
            if name == 'layerXBindNonce':
                result = [1 if args[0].lower() in self.bindings else 0]
            elif name == 'getLayerXDid':
                key = self.bindings[args[0].lower()]
                result = ['0x' + key, 'did:layerx:' + key]
            elif name == 'accountId':
                result = ['0x' + account_id(args[0][2:], args[1][2:])]
            elif name == 'balanceOf':
                assert self.accounts[args[0][2:]]['asset'] == args[1][2:], 'balance asset binding'
                result = [self.balance(args[0][2:])[0]]
            elif name == 'nextSequence':
                result = [self.balance(args[0][2:])[1]]
            else:
                raise ValueError('unsupported read: ' + name)
            return '0x' + evm.encode([v['type'] for v in row['outputs']], result).hex()
        raise ValueError('unsupported RPC: ' + method)
