"""XWeb's native RPC adapter over the maintained Python SDK transaction codec."""
from pathlib import Path
import re
import ssl
import sys
import time

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import merged_qualification as merged
from native_evidence import protected
from layerx_sdk import account_derivation as accounts
from layerx_sdk import evm_transaction as transactions

N = accounts._N
keccak = accounts.keccak256


def unhex(value, length=None):
    merged.require(isinstance(value, str) and re.fullmatch(r'0x(?:[0-9a-fA-F]{2})*', value),
                   '0x-prefixed hexadecimal bytes required')
    raw = bytes.fromhex(value[2:])
    merged.require(length is None or len(raw) == length, 'hexadecimal byte width differs')
    return raw


def checksum(address):
    raw = unhex(address, 20) if isinstance(address, str) else address
    merged.require(type(raw) is bytes and len(raw) == 20, '20-byte address required')
    return accounts.checksum_address(raw)


def read_key(path):
    value = protected(path, maximum=128).decode('ascii').strip()
    merged.require(re.fullmatch(r'(?:0x)?[0-9a-fA-F]{64}', value), '32-byte private key required')
    secret = int(value, 16)
    merged.require(0 < secret < N, 'private key scalar out of range')
    return secret


def compressed_public_key(secret):
    return accounts._secp256k1_public(secret)[0]


def address_of(secret):
    return keccak(accounts._secp256k1_public(secret)[1])[12:]


def parse_signature(signature):
    start = signature.index('(')
    depth = 0
    for end in range(start, len(signature)):
        depth += signature[end] == '('
        depth -= signature[end] == ')'
        if depth == 0:
            break
    else:
        raise ValueError('unbalanced ABI signature')
    tail = signature[end + 1:]
    merged.require(not tail or tail.startswith('(') and tail.endswith(')'), 'invalid ABI output signature')
    inputs = transactions._split(signature[start + 1:end]) if signature[start + 1:end] else []
    outputs = transactions._split(tail[1:-1]) if tail[1:-1] else []
    return signature[:start], inputs, outputs


def selector(signature):
    name, inputs, _ = parse_signature(signature)
    return keccak((name + '(' + ','.join(inputs) + ')').encode())[:4]


def calldata(signature, *values):
    name, kinds, _ = parse_signature(signature)
    converted = []
    for kind, value in zip(kinds, values, strict=True):
        if kind == 'address' and isinstance(value, str):
            value = unhex(value, 20)
        elif kind.startswith('bytes') and isinstance(value, str):
            value = unhex(value)
        converted.append(value)
    return '0x' + transactions.encode_call(name, kinds, converted).hex()


def sign_transaction(secret, transaction):
    key = ec.derive_private_key(secret, ec.SECP256K1())
    r, s = utils.decode_dss_signature(key.sign(transaction.signing_digest(),
        ec.ECDSA(utils.Prehashed(hashes.SHA256()))))
    s = min(s, N - s)
    expected = checksum(address_of(secret))
    for parity in (0, 1):
        signature = r.to_bytes(32, 'big') + s.to_bytes(32, 'big') + bytes([27 + parity])
        if transaction.recover_signer(signature) == expected:
            return transaction.attach_signature(signature, expected)
    raise ValueError('native transaction signature recovery failed')


class Rpc:
    def __init__(self, url, ca=None, timeout=30):
        merged.CHAIN.endpoint(url)
        self.url, self.timeout, self.sequence = url, timeout, 0
        self.context = ssl.create_default_context(cafile=ca) if url.startswith('https://') else None

    def request(self, method, params):
        self.sequence += 1
        status, reply = merged.CHAIN.post(self.url, dict(jsonrpc='2.0', id=self.sequence,
            method=method, params=params), context=self.context, timeout=self.timeout)
        merged.require(status == 200 and isinstance(reply, dict) and reply.get('jsonrpc') == '2.0'
                       and type(reply.get('id')) is int and reply['id'] == self.sequence
                       and 'result' in reply and 'error' not in reply, 'native JSON-RPC response refused')
        return reply['result']

    def chain_id(self):
        return int(self.request('eth_chainId', []), 16)

    def balance(self, address, block='latest'):
        return int(self.request('eth_getBalance', [checksum(address), block]), 16)

    def eth_call(self, to, signature, *values, block='latest'):
        _, _, outputs = parse_signature(signature)
        result = self.request('eth_call', [{'to': checksum(to), 'data': calldata(signature, *values)}, block])
        return tuple(merged.decode_abi(outputs, result))

    def receipt(self, reference, deadline=120):
        end = time.monotonic() + deadline
        while True:
            receipt = self.request('eth_getTransactionReceipt', [reference])
            if receipt is not None:
                merged.require(isinstance(receipt, dict) and receipt.get('transactionHash') == reference
                               and receipt.get('status') == '0x1', 'native transaction receipt identity or execution differs')
                return receipt
            merged.require(time.monotonic() < end, 'native receipt deadline exceeded')
            time.sleep(min(.1, max(0, end - time.monotonic())))

    def send(self, secret, chain_id, to, data='0x', value=0, deadline=120):
        merged.require(chain_id == 125 and self.chain_id() == chain_id, 'native XWeb requires chain 125')
        sender, target = checksum(address_of(secret)), checksum(to)
        nonce = int(self.request('eth_getTransactionCount', [sender, 'pending']), 16)
        price = int(self.request('eth_gasPrice', []), 16)
        call = {'from': sender, 'to': target, 'data': data, 'value': hex(value)}
        gas = int(self.request('eth_estimateGas', [call]), 16)
        transaction = transactions.Type2Transaction(chain_id, nonce, 0, price, gas,
                                                     unhex(target, 20), value, unhex(data))
        raw = sign_transaction(secret, transaction)
        reference = self.request('eth_sendRawTransaction', ['0x' + raw.hex()])
        merged.require(reference == '0x' + keccak(raw).hex(), 'native submission transaction identity differs')
        return self.receipt(reference, deadline)
