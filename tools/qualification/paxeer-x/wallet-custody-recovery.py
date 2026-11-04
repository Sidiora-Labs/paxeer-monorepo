#!/usr/bin/env python3
"""Qualify the supported native-send contract using a recorded authenticated peer.

The Python gate checks the published provider contract and verifies node-produced
transaction/receipt bytes with the production SDK. It does not certify a deployed
TypeScript gateway, PostgreSQL, or attestor cluster.
"""
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile
from urllib.error import HTTPError
from urllib.parse import urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import (
    ReceiptKey, SequencerReceiptVerificationError, verify_sequencer_receipt,
)

RETIRED_METHODS = ('paxeer_recoverCustody', 'paxeer_signCustody')
MAX_RESPONSE = 131072


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def assert_provider_contract(root=ROOT):
    """Check the actual source contract; do not emulate an EIP-1193 provider."""
    source = (root / 'sdk/wallet/src/provider.ts').read_text()
    errors = (root / 'sdk/wallet/src/errors.ts').read_text()
    unsupported = re.search(r'const UNSUPPORTED_METHODS = new Set\(\[(.*?)\]\);', source, re.S)
    require(unsupported is not None, 'published unsupported-method contract missing')
    names = re.findall(r"'([^']+)'", unsupported.group(1))
    require(all(name in names for name in RETIRED_METHODS), 'retired custody method admitted')
    refusal = 'if (UNSUPPORTED_METHODS.has(method)) throw new UnsupportedMethodError(method);'
    start = source.index('const method = args.method;')
    dispatch = source.index('switch (method)', start)
    require(refusal in source[start:dispatch], 'retired methods must be refused before dispatch')
    require(source.index(refusal, start) < source.index('await this.captureSigningContext()', start),
            'retired methods must be refused before signing or authentication')
    require('unsupportedMethod: 4200' in errors
            and 'super(PROVIDER_ERROR_CODES.unsupportedMethod,' in errors,
            'published EIP-1193 unsupported-method code changed')
    require("'/v1/wallet/send', {\n      tx: wire," in source
            and "case 'eth_sendTransaction':" in source,
            'supported SDK native-send interface changed')
    return {'methods': list(RETIRED_METHODS), 'code': 4200, 'transport': 'local refusal'}


def native_transaction(raw):
    require(isinstance(raw, str) and re.fullmatch(r'0x(?:[0-9a-f]{2})+', raw),
            'canonical signed transaction required')
    data = bytes.fromhex(raw[2:])
    tx, signature = decode_signed(data)
    require(tx.chain_id == 125 and tx.value > 0 and tx.data == b'',
            'chain-125 native value transfer required')
    account = tx.recover_signer(signature).lower()
    require(tx.attach_signature(signature, account) == data, 'signed bytes changed during SDK round trip')
    wire = {'chainId': tx.chain_id, 'from': account, 'to': '0x' + tx.to.hex(),
            'data': '0x', 'value': str(tx.value), 'gas': str(tx.gas_limit),
            'maxFeePerGas': str(tx.max_fee_per_gas),
            'maxPriorityFeePerGas': str(tx.max_priority_fee_per_gas), 'nonce': tx.nonce}
    return data, tx, wire, '0x' + keccak256(data).hex()


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError('qualification redirects refused')


class RecoveryQualification:
    """Real HTTP consumer, retaining the original native send across reopen.

    An ambiguous write is only polled. Recovery never constructs a second send.
    The gateway owns rebroadcast of its retained signed bytes.
    """

    def __init__(self, endpoint, token, state, raw, pinned_key):
        parsed = urlparse(endpoint)
        require(parsed.scheme == 'http' and parsed.hostname == '127.0.0.1'
                and parsed.port and parsed.path in ('', '/') and not parsed.query
                and not parsed.fragment and not parsed.username and not parsed.password,
                'recorded loopback HTTP peer required')
        require(isinstance(token, str) and token and '\r' not in token and '\n' not in token,
                'bounded bearer header required')
        self.endpoint = endpoint.rstrip('/')
        self.token = token
        self.state = Path(state)
        self.raw, self.tx, self.wire, self.hash = native_transaction(raw)
        self.key = ReceiptKey(bytes.fromhex(pinned_key['keyId'][2:]),
                              bytes.fromhex(pinned_key['publicKey'][2:]),
                              pinned_key['firstHeight'], pinned_key['lastHeight'])
        self.opener = build_opener(NoRedirect())
        if self.state.exists():
            saved = self.read_state()
            require(saved['raw'] == raw and saved['tx_hash'] == self.hash
                    and saved['tx'] == self.wire, 'reopened native send differs from original')

    def read_state(self):
        fd = os.open(self.state, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd) as stream:
            info = os.fstat(stream.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid()
                    and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
                    and 0 < info.st_size <= MAX_RESPONSE, 'private recovery record required')
            value = json.load(stream)
        require(set(value) == {'raw', 'tx_hash', 'tx', 'phase'}
                and value['phase'] in ('sending', 'unknown', 'settled'), 'invalid recovery record')
        return value

    def save(self, phase):
        value = {'raw': '0x' + self.raw.hex(), 'tx_hash': self.hash, 'tx': self.wire, 'phase': phase}
        fd, name = tempfile.mkstemp(prefix='.native-send-', dir=self.state.parent)
        try:
            with os.fdopen(fd, 'w') as stream:
                json.dump(value, stream, sort_keys=True)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(name, self.state)
            parent = os.open(self.state.parent, os.O_RDONLY | os.O_DIRECTORY)
            try:
                os.fsync(parent)
            finally:
                os.close(parent)
        finally:
            if os.path.exists(name):
                os.unlink(name)

    def request(self, path, body):
        data = json.dumps(body, separators=(',', ':')).encode()
        request = Request(self.endpoint + path, data=data, method='POST', headers={
            'Authorization': 'Bearer ' + self.token, 'Content-Type': 'application/json'})
        try:
            response = self.opener.open(request, timeout=5)
        except HTTPError as error:
            response = error
        with response:
            payload = response.read(MAX_RESPONSE + 1)
            require(len(payload) <= MAX_RESPONSE, 'bounded response required')
            require(response.headers.get_content_type() == 'application/json', 'JSON response required')
            value = json.loads(payload)
            require(isinstance(value, dict), 'response object required')
            return response.status, value

    def submit(self):
        require(not self.state.exists(), 'retained send must be recovered, never resubmitted')
        # Persist before the uncertain HTTP write, including a lost response.
        self.save('sending')
        status, value = self.request('/v1/wallet/send', {'tx': self.wire})
        require(status in (200, 202) and value.get('tx_hash') == self.hash,
                'gateway must identify the exact original signed transaction')
        if status == 202:
            require(value.get('error') == 'outcome_unknown'
                    and value.get('nonce') == self.tx.nonce
                    and value.get('retry') == 'poll_receipt'
                    and value.get('poll') == 'eth_getTransactionReceipt'
                    and value.get('must_not_resubmit') is True, 'ambiguous send recovery contract changed')
        else:
            require(value.get('address', '').lower() == self.wire['from']
                    and value.get('chain_id') == 125, 'gateway account or chain mismatch')
        self.save('unknown')
        return value

    def recover(self):
        saved = self.read_state()
        require(saved['raw'] == '0x' + self.raw.hex() and saved['tx_hash'] == self.hash
                and saved['tx'] == self.wire, 'retained native send changed')
        status, value = self.request('/rpc', {'jsonrpc': '2.0', 'id': 1,
                                             'method': 'pax_sequencerReceipt', 'params': [self.hash]})
        require(status == 200 and value.get('jsonrpc') == '2.0'
                and type(value.get('id')) is int and value['id'] == 1
                and 'error' not in value and 'result' in value, 'receipt RPC failed')
        row = value['result']
        if row is None:
            return None
        require(isinstance(row, dict) and row.get('txHash') == self.hash, 'receipt hash mismatch')
        proof = verify_sequencer_receipt(bytes.fromhex(row['receipt'][2:]), self.raw, (self.key,))
        facts = proof.receipt
        require(facts.module_id == 12 and facts.operation == 2 and facts.result_code == 0
                and facts.from_sequence == self.tx.nonce and facts.amount == self.tx.value
                and facts.from_account == bytes(12) + bytes.fromhex(self.wire['from'][2:])
                and facts.to_account == bytes(12) + self.tx.to, 'native execution binding mismatch')
        expected = {'rung': 'receipted', 'nonce': hex(self.tx.nonce), 'status': '0x1',
                    'height': hex(proof.height), 'seq': hex(facts.global_sequence),
                    'from': self.wire['from'], 'to': self.wire['to'],
                    'feeAmount': hex(facts.fee_charged), 'keyId': '0x' + facts.context_hash.hex(),
                    'signingKey': '0x' + self.key.public_key.hex()}
        require(all(row.get(key) == item for key, item in expected.items()),
                'receipt JSON differs from signed execution')
        self.save('settled')
        return row


def main(arguments=None):
    require(not (sys.argv[1:] if arguments is None else arguments), 'only the declared qualification command is supported')
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from wallet_recovery_recording import qualify_recording
    result = qualify_recording()
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (RuntimeError, OSError, ValueError, KeyError, SequencerReceiptVerificationError) as error:
        print('wallet native recovery qualification refused: ' + str(error), file=sys.stderr)
        sys.exit(1)
