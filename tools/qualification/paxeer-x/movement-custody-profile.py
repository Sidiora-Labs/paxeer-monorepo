#!/usr/bin/env python3
"""Qualify merged bank movement authority and immutable committed evidence pins."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged

CASES = ('valid-bank-authority', 'same-authority-restart', 'wrong-chain', 'wrong-genesis',
         'wrong-module', 'wrong-asset', 'non-owner', 'changed-authority-retained-pin',
         'changed-receipt-key', 'changed-committed-proof', 'restored-state-restart')


def require(value, reason):
    if not value:
        raise RuntimeError(reason)


def private_directory(path):
    path = Path(path)
    info = path.lstat()
    require(path.is_absolute() and path.resolve(strict=True) == path and stat.S_ISDIR(info.st_mode)
            and info.st_uid == os.geteuid() and stat.S_IMODE(info.st_mode) == 0o700,
            'owner-only canonical fixture directory required')
    return path


def private_bytes(path, maximum=1048576):
    path = Path(path)
    private_directory(path.parent)
    require(path.is_absolute() and path.resolve(strict=True) == path, 'canonical private input required')
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, 'rb') as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                and info.st_uid == os.geteuid() and stat.S_IMODE(info.st_mode) == 0o600
                and 0 < info.st_size <= maximum, 'owner-only regular bounded fixture input required')
        value = stream.read(maximum + 1)
        require(0 < len(value) <= maximum, 'private input exceeds bound')
        return value


def unique(items):
    output = {}
    for key, value in items:
        require(key not in output, 'duplicate fixture field refused')
        output[key] = value
    return output


def document(path):
    return json.loads(private_bytes(path), object_pairs_hook=unique)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode() + b'\n'


def retain(path, value):
    """Create once; a restart can only read exactly the same authority/evidence."""
    path = Path(path)
    private_directory(path.parent)
    data = canonical(value)
    try:
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    except FileExistsError:
        require(private_bytes(path) == data, 'retained bank authority or committed proof changed')
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


class BankMovement:
    def __init__(self, node, profile_path, state_directory):
        self.node = node
        self.profile = document(profile_path)
        self.state = private_directory(state_directory)
        profile = self.profile
        require(set(profile) == {'version', 'chain_id', 'genesis_hash', 'module', 'asset', 'owner', 'receipt_keys'}
                and profile['version'] == 1 and type(profile['chain_id']) is int and profile['chain_id'] == 125
                and profile['module'] == 'bank' and profile['asset'] == 'uhpx',
                'merged bank authority on chain 125 and native asset required')
        require(re.fullmatch(r'0x[0-9a-f]{40}', profile['owner']) and int(profile['owner'], 16)
                and re.fullmatch(r'0x[0-9a-f]{64}', profile['genesis_hash']) and int(profile['genesis_hash'], 16),
                'nonzero canonical owner and genesis required')
        require(profile['receipt_keys'] == node.receipt_keys, 'node receipt keys differ from owner authority')
        self.check_chain()
        retain(self.state / 'bank-authority.pin', profile)

    def check_chain(self):
        require(self.node.call('eth_chainId', []) == '0x7d', 'wrong merged chain identity')
        genesis = self.node.call('eth_getBlockByNumber', ['0x0', False])
        require(isinstance(genesis, dict) and genesis.get('number') == '0x0'
                and genesis.get('hash') == self.profile['genesis_hash'], 'wrong merged genesis authority')

    def prove(self, transaction_hash, recipient, amount):
        require(re.fullmatch(r'0x[0-9a-f]{40}', recipient) and int(recipient, 16)
                and type(amount) is int and 0 < amount < 1 << 256, 'canonical bank destination and amount required')
        self.check_chain()
        proof = self.node.committed(transaction_hash, owner=self.profile['owner'])
        transaction, _ = merged.evm.decode_signed(bytes.fromhex(proof['raw_transaction'][2:]))
        require(transaction.chain_id == 125 and transaction.to.hex() == recipient[2:]
                and transaction.value == amount and transaction.data == b'',
                'committed native bank transfer binding mismatch')
        row = self.node.call('pax_sequencerReceipt', [transaction_hash])
        verified, _ = merged.CHAIN._EXECUTION.verified_execution(
            row, bytes.fromhex(proof['raw_transaction'][2:]), {'receipt_keys': self.node.receipt_keys})
        require(row['paymentDenom'] == self.profile['asset']
                and verified.receipt_digest.hex() == proof['receipt_digest'], 'native bank asset or proof mismatch')
        retained = {'authority_sha256': hashlib.sha256(canonical(self.profile)).hexdigest(),
                    'recipient': recipient, 'amount': str(amount), 'proof': proof}
        retain(self.state / (transaction_hash[2:] + '.proof'), retained)
        return retained


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', default=os.environ.get('LAYERX_MOVEMENT_BANK_FIXTURE'))
    args = parser.parse_args()
    require(args.fixture, 'explicit protected merged bank fixture required')
    fixture = document(args.fixture)
    require(set(fixture) == {'profile', 'state_directory', 'node_url', 'transaction_hash', 'recipient', 'amount'},
            'closed merged bank fixture required')
    profile = document(fixture['profile'])
    node = merged.CHAIN.Node(fixture['node_url'], profile['receipt_keys'])
    result = BankMovement(node, fixture['profile'], fixture['state_directory']).prove(
        fixture['transaction_hash'], fixture['recipient'], fixture['amount'])
    print(json.dumps({'status': 'passed', 'exit_code': 0, 'evidence': result}))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, RuntimeError) as error:
        print(json.dumps({'status': 'refused', 'exit_code': 2, 'reason': str(error)}))
        sys.exit(2)
