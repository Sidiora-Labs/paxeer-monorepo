"""Private immutable qualification evidence bound to the merged native network."""
import base64
import json
import os
from pathlib import Path
import re
import stat

import merged_qualification as merged
from layerx_sdk.x402_rpc import decode_receipt_keys


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def protected(path, maximum=67_108_864):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, 'rb') as stream:
        info = os.fstat(stream.fileno())
        merged.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                       and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600
                       and 0 < info.st_size <= maximum, 'private bounded regular evidence required')
        data = stream.read(maximum + 1)
        merged.require(0 < len(data) <= maximum and len(data) == info.st_size,
                       'private evidence byte bound or size changed')
        return data


def retain(path, data):
    """Publish once and fsync; reopening can only accept identical private bytes."""
    path = Path(path)
    info = path.parent.lstat()
    merged.require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                   and stat.S_IMODE(info.st_mode) == 0o700, 'private evidence directory required')
    merged.require(type(data) is bytes and 0 < len(data) <= 67_108_864, 'bounded evidence bytes required')
    try:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    except FileExistsError:
        merged.require(protected(path) == data, 'reconciliation required: retained native evidence changed')
    else:
        with os.fdopen(descriptor, 'wb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)


def genesis_receipt_keys(genesis):
    merged.require(isinstance(genesis, dict) and genesis.get('chain_id') == 'hyperpax_125-1',
                   'native genesis must bind merged chain 125')
    entries = genesis['app_state']['sequencer']['receipt_keys']
    merged.require(isinstance(entries, list) and entries, 'governed genesis receipt keys required')
    keys, seen = [], set()
    for entry in entries:
        merged.require(isinstance(entry, dict) and set(entry) == {
            'key_id', 'public_key', 'first_height', 'last_height'}, 'governed receipt key fields differ')
        key = {}
        for name in ('key_id', 'public_key'):
            value = entry[name]
            merged.require(type(value) is str, 'governed receipt key base64 required')
            raw = base64.b64decode(value, validate=True)
            merged.require(len(raw) == 32 and raw != bytes(32)
                           and base64.b64encode(raw).decode() == value, 'governed receipt key bytes differ')
            key[name] = raw.hex()
        for name in ('first_height', 'last_height'):
            value = entry[name]
            merged.require(type(value) is int or type(value) is str
                           and re.fullmatch(r'0|[1-9][0-9]*', value), 'governed receipt height encoding differs')
            key[name] = int(value)
        merged.require(0 < key['first_height'] <= key['last_height'] < 2**64,
                       'governed receipt key height range differs')
        merged.require(key['key_id'] not in seen, 'duplicate governed receipt key')
        seen.add(key['key_id'])
        keys.append(key)
    return sorted(keys, key=lambda row: row['key_id'])


def native_binding(node, genesis, genesis_hash):
    merged.require(node.call('eth_chainId', []) == '0x7d', 'native owner requires merged chain 125')
    merged.require(isinstance(genesis_hash, str) and re.fullmatch(r'0x[0-9a-f]{64}', genesis_hash)
                   and int(genesis_hash, 16) != 0, 'native genesis hash required')
    block = node.call('eth_getBlockByNumber', ['0x0', False])
    merged.require(isinstance(block, dict) and block.get('number') == '0x0'
                   and block.get('hash') == genesis_hash, 'native genesis binding differs')
    governed = genesis_receipt_keys(genesis)
    pinned = sorted(node.receipt_keys, key=lambda row: row['key_id'])
    merged.require(pinned == governed, 'pinned receipt keys differ from governed genesis')
    node.sequence += 1
    status, reply = merged.CHAIN.post(node.url, {
        'jsonrpc': '2.0', 'id': node.sequence, 'method': 'px_getReceiptKeys', 'params': []},
        node.headers, node.context)
    merged.require(status == 200 and isinstance(reply, dict) and reply.get('jsonrpc') == '2.0'
                   and type(reply.get('id')) is int and reply['id'] == node.sequence
                   and 'result' in reply and 'error' not in reply, 'native receipt key RPC identity differs')
    keys = decode_receipt_keys(reply['result'])
    observed = sorted([dict(key_id=key.key_id.hex(), public_key=key.public_key.hex(),
                            first_height=key.first_height, last_height=key.last_height) for key in keys],
                      key=lambda row: row['key_id'])
    merged.require(observed == governed, 'native receipt keys differ from governed genesis')
    return dict(chain_id=125, genesis_hash=genesis_hash, receipt_keys=pinned)
