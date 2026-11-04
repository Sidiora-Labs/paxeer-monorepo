"""Current node/gateway transport and canonical ABI for qualification clients."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk import evm_transaction as evm


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).parent / filename)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


CHAIN = module('qualification_merged_chain', 'fixtures/merged_chain.py')
OWNED = module('qualification_owned_paxd', 'programs_accounts_winddown.py')
OWNED.CHAIN = CHAIN


def require(value, message):
    if not value:
        raise ValueError(message)


def abi_method(surface, name):
    methods = json.loads((ROOT / 'precompiles' / surface / 'abi.json').read_text())
    rows = [row for row in methods if row.get('type') == 'function' and row['name'] == name]
    require(len(rows) == 1, 'unknown current ABI method: ' + name)
    return rows[0]


def decode_abi(types, value):
    """Decode bounded ABI, then require exact re-encoding by the production codec."""
    require(isinstance(value, str) and re.fullmatch(r'0x(?:[0-9a-fA-F]{2})*', value)
            and len(value) <= 8_388_610, 'bounded ABI hex required')
    raw = bytes.fromhex(value[2:])
    kinds = [evm._parse(kind) for kind in types]

    def word(offset):
        require(0 <= offset <= len(raw) - 32, 'truncated ABI word')
        return raw[offset:offset + 32]

    def size(kind):
        if evm._dynamic(kind) or isinstance(kind, str):
            return 32
        return sum(size(child) for child in kind[1]) if kind[0] == 'tuple' else size(kind[1]) * kind[2]

    def sequence(items, base):
        require(len(items) <= 65536, 'ABI collection bound')
        cursor, result = base, []
        for kind in items:
            target = base + int.from_bytes(word(cursor), 'big') if evm._dynamic(kind) else cursor
            require(target % 32 == 0, 'unaligned ABI offset')
            result.append(decode(kind, target))
            cursor += size(kind)
        return result

    def decode(kind, offset):
        if isinstance(kind, tuple):
            if kind[0] == 'tuple':
                return sequence(kind[1], offset)
            length = kind[2]
            if length is None:
                length = int.from_bytes(word(offset), 'big')
                offset += 32
            require(length <= 65536, 'ABI array bound')
            return sequence([kind[1]] * length, offset)
        data = word(offset)
        number = int.from_bytes(data, 'big')
        if kind in ('bytes', 'string'):
            require(number <= len(raw) - offset - 32, 'truncated ABI bytes')
            data = raw[offset + 32:offset + 32 + number]
            return data.decode('utf-8') if kind == 'string' else data
        if kind == 'bool':
            require(number in (0, 1), 'noncanonical ABI boolean')
            return bool(number)
        if kind == 'address':
            require(data[:12] == bytes(12), 'noncanonical ABI address')
            return data[12:]
        if kind.startswith('uint'):
            require(number < 1 << int(kind[4:]), 'ABI integer overflow')
            return number
        length = int(kind[5:])
        require(data[length:] == bytes(32 - length), 'noncanonical fixed ABI bytes')
        return data[:length]

    result = sequence(kinds, 0)
    require(evm.encode_call('qualificationResult', types, result)[4:] == raw,
            'noncanonical ABI offsets, padding or trailing bytes')
    return result


class Client:
    def __init__(self, node, gateway_url, headers=None, context=None):
        CHAIN.endpoint(gateway_url)
        self.node, self.gateway_url, self.headers, self.context = node, gateway_url, headers, context
        self.sequence = 0

    def hosted(self, method, params):
        require(method.startswith('px_'), 'hosted capability method required')
        self.sequence += 1
        status, result = CHAIN.post(self.gateway_url,
            {'jsonrpc': '2.0', 'id': self.sequence, 'method': method, 'params': params}, self.headers, self.context)
        require(status == 200 and isinstance(result, dict) and result.get('jsonrpc') == '2.0'
                and type(result.get('id')) is int and result['id'] == self.sequence
                and (('result' in result) != ('error' in result)), 'gateway RPC identity mismatch')
        if 'error' in result:
            raise CHAIN.RpcError(result['error'])
        return result['result']

    def capabilities(self):
        require(self.node.call('eth_chainId', []) == '0x7d', 'merged chain 125 required')
        result = self.hosted('px_getCapabilities', [])
        surfaces = {'exchange': '1015', 'bridge': '1016', 'launchpad': '1017',
                    'ledger': '101a', 'pay': '101b', 'auth': '101c', 'programs': '101d'}
        require(isinstance(result, dict) and set(result) == set(surfaces) | {'addresses', 'probed_at', 'rpc_height'}
                and all(type(result[name]) is bool for name in surfaces), 'current capability document required')
        require(result['addresses'] == {name: '0x' + '00' * 18 + suffix for name, suffix in surfaces.items()},
                'capability precompile address substitution')
        require(type(result['probed_at']) is int and result['probed_at'] > 0
                and isinstance(result['rpc_height'], str) and re.fullmatch(r'0|[1-9][0-9]*', result['rpc_height']),
                'capability observation required')
        height = hex(int(result['rpc_height']))
        block = self.node.call('eth_getBlockByNumber', [height, False])
        require(isinstance(block, dict) and block.get('number') == height
                and re.fullmatch(r'0x[0-9a-fA-F]{64}', block.get('hash', ''))
                and int(block['hash'], 16) != 0, 'capabilities lack a committed node block')
        return result

    def view(self, surface, address, name, arguments, block='finalized'):
        method = abi_method(surface, name)
        types = [CHAIN.Node.abi_type(row) for row in method['inputs']]
        data = evm.encode_call(name, types, arguments)
        value = self.node.call('eth_call', [{'to': address, 'data': '0x' + data.hex()}, block])
        return decode_abi([CHAIN.Node.abi_type(row) for row in method['outputs']], value)


def source_artifacts(manifest, revision):
    require(manifest['source_revision'] == revision, 'artifact manifest source revision differs')
    for name, row in manifest['artifacts'].items():
        path = Path(row['path'])
        require(path.is_absolute() and path.resolve(strict=True) == path and path.is_file()
                and not path.is_symlink() and os.access(path, os.X_OK)
                and row['source_revision'] == revision, 'source-bound executable required: ' + name)
        require(hashlib.sha256(path.read_bytes()).hexdigest() == row['sha256'], 'artifact digest changed: ' + name)
    require('paxd' in manifest['artifacts'] and Path(manifest['artifacts']['paxd']['path']).name == 'paxd',
            'merged paxd artifact required')
    with Path(manifest['artifacts']['paxd']['path']).open('rb') as stream:
        require(stream.read(4) == b'\x7fELF', 'native merged paxd executable required')
