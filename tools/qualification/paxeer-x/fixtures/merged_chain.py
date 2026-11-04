"""The qualification clients' chain-125 RPC and committed receipt boundary."""
import http.client
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import sys
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed, encode_call

_spec = importlib.util.spec_from_file_location('merged_verified_execution', Path(__file__).parents[1] / 'paid-web-contract.py')
_EXECUTION = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_EXECUTION)

PROGRAMS = '0x' + '00' * 18 + '101d'
MAX_BYTES = 4_194_304


def require(value, reason):
    if not value:
        raise ValueError(reason)


def endpoint(raw, path='/'):
    value = urlsplit(raw)
    require(value.scheme in ('http', 'https') and value.hostname
            and (value.scheme == 'https' or value.hostname in ('localhost', '127.0.0.1', '::1'))
            and (value.path in ('', '/') if path == '/' else value.path == path),
            'merged RPC endpoint has the wrong route')
    require(not value.username and not value.password and not value.query and not value.fragment,
            'merged RPC endpoint must not carry credentials or a query')
    return value


def post(raw, document, headers=None, context=None, path='/', timeout=6.5):
    url = endpoint(raw, path)
    connection = (http.client.HTTPSConnection(url.hostname, url.port, timeout=timeout, context=context)
                  if url.scheme == 'https' else http.client.HTTPConnection(url.hostname, url.port, timeout=timeout))
    try:
        data = json.dumps(document, separators=(',', ':')).encode()
        require(len(data) <= 1_048_576, 'RPC request exceeds envelope bound')
        connection.request('POST', path, data, {'Content-Type': 'application/json', **(headers or {})})
        response = connection.getresponse()
        body = response.read(MAX_BYTES + 1)
        require(len(body) <= MAX_BYTES, 'RPC response exceeds byte bound')
        return response.status, json.loads(body)
    finally:
        connection.close()


class RpcError(ValueError):
    def __init__(self, error):
        self.error = error
        super().__init__('merged RPC refused: ' + str(error))


class Node:
    def __init__(self, url, receipt_keys, headers=None, context=None):
        endpoint(url)
        require(isinstance(receipt_keys, list) and receipt_keys, 'pinned node receipt keys required')
        self.url, self.receipt_keys, self.headers, self.context = url, receipt_keys, headers, context
        self.sequence = 0

    def call(self, method, params):
        require(method.startswith(('eth_', 'pax_', 'net_', 'web3_')), 'hosted RPC cannot use the node endpoint')
        self.sequence += 1
        request = {'jsonrpc': '2.0', 'id': self.sequence, 'method': method, 'params': params}
        status, result = post(self.url, request, self.headers, self.context)
        require(status == 200 and isinstance(result, dict) and result.get('jsonrpc') == '2.0'
                and type(result.get('id')) is int and result['id'] == self.sequence
                and (('result' in result) != ('error' in result)), 'node JSON-RPC response identity mismatch')
        if 'error' in result:
            raise RpcError(result['error'])
        return result['result']

    def committed(self, transaction_hash, owner=None, canonical=None, refused=False):
        require(self.call('eth_chainId', []) == '0x7d', 'qualification requires merged chain 125')
        require(isinstance(transaction_hash, str) and re.fullmatch('0x[0-9a-f]{64}', transaction_hash),
                'canonical transaction hash required')
        row = self.call('pax_sequencerReceipt', [transaction_hash])
        require(isinstance(row, dict) and row.get('rung') == 'committed', 'receipt is not committed')
        raw = self.call('eth_getRawTransactionByHash', [transaction_hash])
        require(isinstance(raw, str) and re.fullmatch('0x(?:[0-9a-f]{2})+', raw), 'raw committed transaction missing')
        transaction = bytes.fromhex(raw[2:])
        require('0x' + keccak256(transaction).hex() == transaction_hash, 'committed transaction hash mismatch')
        verified, _ = _EXECUTION.verified_execution(row, transaction, {'receipt_keys': self.receipt_keys})
        tx, signature = decode_signed(transaction)
        if owner is not None:
            require(tx.recover_signer(signature).lower() == owner.lower(), 'committed receipt belongs to another tenant payer')
        require(verified.receipt.result_code == int(refused), 'committed execution outcome mismatch')
        if canonical is not None:
            require(verified.canonical_bytes.hex() == canonical, 'tenant receipt differs from committed node receipt')
        standard = self.call('eth_getTransactionReceipt', [transaction_hash])
        require(isinstance(standard, dict) and standard.get('transactionHash') == transaction_hash
                and standard.get('blockHash') == row['blockHash'] and standard.get('blockNumber') == row['blockNumber']
                and standard.get('status') == row['status'] and standard.get('from', '').lower() == row['from'].lower()
                and standard.get('to', '').lower() == row['to'].lower(), 'committed standard receipt mismatch')
        require(standard.get('transactionIndex') == row.get('index'), 'committed transaction index mismatch')
        logs = standard.get('logs')
        require(isinstance(logs, list) and len(logs) == len(row['logs']) and all(
            all(actual.get(key) == expected.get(key) for key in ('address', 'topics', 'data', 'logIndex'))
            and actual.get('removed', False) is False
            for actual, expected in zip(logs, row['logs'], strict=True)), 'committed logs differ from signed execution')
        require(int(row['blockHash'], 16) != 0, 'committed block hash is zero')
        block = self.call('eth_getBlockByNumber', [row['blockNumber'], False])
        require(isinstance(block, dict) and block.get('hash') == row['blockHash']
                and block.get('number') == row['blockNumber'] and transaction_hash in block.get('transactions', []),
                'receipt block is not the committed node block')
        return {'transaction_hash': transaction_hash, 'block_hash': row['blockHash'],
                'block_number': row['blockNumber'], 'receipt_digest': verified.receipt_digest.hex(),
                'canonical_bytes': verified.canonical_bytes.hex(), 'payer': row['from'],
                'receipt_sha256': hashlib.sha256(verified.canonical_bytes).hexdigest(), 'raw_transaction': raw}

    def wait_committed(self, transaction_hash, timeout=180):
        deadline = time.monotonic() + timeout
        while True:
            row = self.call('pax_sequencerReceipt', [transaction_hash])
            if isinstance(row, dict) and row.get('rung') == 'committed':
                return self.committed(transaction_hash)
            require(row is None or isinstance(row, dict) and row.get('txHash') == transaction_hash
                    and row.get('rung') in ('submitted', 'receipted'), 'invalid pending Programs receipt')
            require(time.monotonic() < deadline, 'Programs committed receipt deadline')
            time.sleep(min(0.1, max(0, deadline - time.monotonic())))

    @staticmethod
    def method(name):
        abi = json.loads((ROOT / 'precompiles/layerxprograms/abi.json').read_text())
        methods = [row for row in abi if row.get('type') == 'function' and row.get('name') == name]
        require(len(methods) == 1, 'unknown merged Programs method')
        return methods[0]

    @staticmethod
    def abi_type(entry):
        kind = entry['type']
        if kind.startswith('tuple'):
            return '(' + ','.join(Node.abi_type(item) for item in entry['components']) + ')' + kind[5:]
        return kind

    def json_arguments(self, name, arguments):
        def convert(entry, value):
            kind = entry['type']
            if kind.endswith(']'):
                require(isinstance(value, list), 'ABI array required')
                child = dict(entry, type=kind[:kind.rindex('[')])
                return [convert(child, item) for item in value]
            if kind == 'tuple':
                require(isinstance(value, list) and len(value) == len(entry['components']), 'ABI tuple required')
                return [convert(child, item) for child, item in zip(entry['components'], value, strict=True)]
            if kind.startswith('bytes'):
                require(isinstance(value, str) and re.fullmatch('(?:0x)?(?:[0-9a-f]{2})*', value), 'canonical ABI bytes required')
                return bytes.fromhex(value.removeprefix('0x'))
            return value
        inputs = self.method(name)['inputs']
        require(isinstance(arguments, list) and len(arguments) == len(inputs), 'exact Programs argument count required')
        return [convert(entry, value) for entry, value in zip(inputs, arguments, strict=True)]

    def program_call(self, name, arguments):
        types = [self.abi_type(entry) for entry in self.method(name)['inputs']]
        return encode_call(name, types, arguments)

    def program_read(self, name, arguments, block='latest'):
        require(self.call('eth_chainId', []) == '0x7d', 'Programs requires merged chain 125')
        data = self.program_call(name, arguments)
        return self.call('eth_call', [{'to': PROGRAMS, 'data': '0x' + data.hex()}, block])

    def program_send(self, raw, name, arguments, refused=False):
        transaction = bytes.fromhex(raw.removeprefix('0x'))
        tx, _ = decode_signed(transaction)
        require(tx.chain_id == 125 and tx.to.hex() == PROGRAMS[2:] and tx.value == 0
                and tx.data == self.program_call(name, arguments), 'signed Programs transaction does not bind the declared action')
        require(self.call('eth_chainId', []) == '0x7d', 'Programs requires merged chain 125')
        reference = self.call('eth_sendRawTransaction', ['0x' + transaction.hex()])
        require(reference == '0x' + keccak256(transaction).hex(), 'Programs submission identity mismatch')
        if refused:
            return self.committed(reference, refused=True)
        return self.wait_committed(reference)
