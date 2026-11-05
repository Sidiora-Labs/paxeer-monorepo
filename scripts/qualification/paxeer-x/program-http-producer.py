#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import urllib.parse
import urllib.request

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import _evm_address, _recover, keccak256, normalize_wallet_signature
from layerx_sdk.evm_transaction import _rlp, _rlp_decode, decode_signed, encode_call
from layerx_sdk.native_program_call import decode_native_program_call, encode_native_program_call
from layerx_sdk.production import PlatformSdkError
from layerx_sdk.program_lifecycle import NativeProgramDeploy
from layerx_sdk.verifier import verify_sequencer_receipt
from layerx_sdk.x402_rpc import PaymentRpcError, _decode_rpc_response, _send_result, decode_receipt_keys

SCHEMA = 'paxeer-x.program-http-producer.v2'
SOURCE_SCHEMA = 'paxeer-x.authenticated-source.v1'
RESULT_SCHEMA = 'paxeer-x.program-http-result.v2'
ROLES = frozenset({'paxd', 'gateway'})
PROGRAMS = bytes.fromhex('00' * 18 + '101d')
CALL_TYPES = ('bytes32', 'uint16', 'string', 'bytes', 'bytes', 'bytes', 'uint32', 'uint64[7]', 'uint128')
DEPLOY_TYPES = ('bytes32', 'uint16', 'uint8', 'bytes32', 'bytes32', 'bytes', 'bytes')


def require(value, message):
    if not value:
        raise ValueError(message)


def unique(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate producer field')
        result[key] = value
    return result


def private(path, directory=False, create=False):
    path = Path(path).absolute()
    if create:
        path.mkdir(mode=0o700)
    info = path.lstat()
    require(path.resolve() == path and info.st_uid == os.geteuid()
            and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600)
            and (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode) and info.st_nlink == 1),
            'canonical caller-owned private producer input required')
    return path


def load(path):
    path = private(path)
    private(path.parent, True)
    require(0 < path.stat().st_size <= 16_777_216, 'bounded producer record required')
    return json.loads(path.read_bytes(), object_pairs_hook=unique)


def read(path):
    path = private(path)
    private(path.parent, True)
    require(0 < path.stat().st_size <= 1_048_576, 'bounded signed producer input required')
    return path.read_bytes()


def write(path, value):
    data = value if isinstance(value, bytes) else (json.dumps(value, indent=2) + '\n').encode()
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())
    return str(path)


def artifact(path):
    path = private(path)
    data = read(path)
    return {'path': str(path), 'sha256': hashlib.sha256(data).hexdigest(), 'bytes': len(data)}


def source_identity(source, revision):
    require(isinstance(source, dict) and set(source) == {'revision', 'tree'}
            and all(isinstance(value, str) and re.fullmatch('[0-9a-f]{40}', value)
                    for value in source.values()) and source['revision'] == revision,
            'producer source must match the independently selected revision')
    return dict(source)


def rpc_bytes(value, size=None):
    require(isinstance(value, str) and len(value) <= 2_097_154
            and re.fullmatch('0x(?:[0-9a-f]{2})*', value)
            and (size is None or len(value) == 2 + size * 2), 'canonical RPC bytes required')
    return bytes.fromhex(value[2:])


def quantity(value):
    require(isinstance(value, str) and re.fullmatch('0x(?:0|[1-9a-f][0-9a-f]{0,15})', value),
            'bounded canonical RPC quantity required')
    return int(value, 16)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, stream, code, message, headers, target):
        raise ValueError('producer redirects are forbidden')


class ProducerRpc:
    def __init__(self, endpoint, headers):
        require(isinstance(endpoint, str) and 0 < len(endpoint) <= 4096,
                'bounded producer endpoint required')
        url = urllib.parse.urlsplit(endpoint)
        require(url.hostname and not url.username and not url.password and not url.query and not url.fragment
                and url.path in ('', '/') and (url.scheme == 'https'
                    or url.scheme == 'http' and url.hostname in ('localhost', '127.0.0.1', '::1')),
                'supported root JSON-RPC producer endpoint required')
        self.endpoint = endpoint.rstrip('/') + '/'
        self.headers = dict(headers)
        self.opener = urllib.request.build_opener(NoRedirect())
        self.request_id = 0

    def call(self, method, params):
        self.request_id += 1
        request = urllib.request.Request(self.endpoint, method='POST',
            headers={**self.headers, 'Content-Type': 'application/json', 'Accept': 'application/json'},
            data=json.dumps({'jsonrpc': '2.0', 'id': self.request_id, 'method': method, 'params': params},
                            allow_nan=False, separators=(',', ':')).encode())
        with self.opener.open(request, timeout=15) as response:
            require(response.status == 200 and response.headers.get('Content-Type') == 'application/json',
                    'producer must answer HTTP 200 application/json')
            raw = response.read(8_388_609)
        require(len(raw) <= 8_388_608, 'bounded producer response required')
        return _decode_rpc_response(json.loads(raw, object_pairs_hook=unique), self.request_id)

    def receipt_keys(self):
        return decode_receipt_keys(self.call('px_getReceiptKeys', []))

    def send(self, raw):
        canonical = rpc_bytes(raw)
        transaction(canonical)
        return _send_result(self.call('eth_sendRawTransaction', [raw]), canonical)


def transaction(raw):
    require(isinstance(raw, bytes) and 0 < len(raw) <= 1_048_576, 'bounded signed transaction required')
    if raw[0] == 2:
        tx, signature = decode_signed(raw)
        signer = bytes.fromhex(tx.recover_signer(signature)[2:])
        return tx.chain_id, tx.nonce, tx.to, tx.value, tx.data, signer
    # The supported paxd recorder also emits EIP-155 legacy transactions.
    fields = _rlp_decode(raw)
    require(len(fields) == 9 and all(isinstance(field, bytes) for field in fields),
            'canonical EIP-155 transaction required')
    for index, width in ((0, 8), (1, 32), (2, 8), (4, 32), (6, 8), (7, 32), (8, 32)):
        require(len(fields[index]) <= width and fields[index][:1] != b'\0', 'noncanonical EIP-155 integer')
    nonce, v, r, s = (int.from_bytes(fields[index], 'big') for index in (0, 6, 7, 8))
    require(v >= 35 and len(fields[3]) == 20 and int.from_bytes(fields[2], 'big') > 0,
            'replay-protected EIP-155 transaction required')
    chain, parity = (v - 35) // 2, (v - 35) % 2
    signature = r.to_bytes(32, 'big') + s.to_bytes(32, 'big') + bytes([27 + parity])
    require(normalize_wallet_signature(signature) == signature, 'canonical low-S EIP-155 signature required')
    digest = keccak256(_rlp(fields[:6] + (chain.to_bytes((chain.bit_length() + 7) // 8, 'big'), b'', b'')))
    signer = bytes.fromhex(_evm_address(_recover(digest, r, s, parity))[2:])
    return chain, nonce, fields[3], int.from_bytes(fields[4], 'big'), fields[5], signer


def verified_receipt(row, raw, pins, network_id=125):
    require(type(network_id) is int and 0 < network_id < 1 << 32, 'independently selected bounded paxd network required')
    require(isinstance(row, dict), 'actual source receipt required')
    chain, nonce, target, value, _, signer = transaction(raw)
    require(chain == network_id, 'transaction differs from the independently selected paxd network')
    verified = verify_sequencer_receipt(rpc_bytes(row.get('receipt')), raw, pins)
    facts = verified.receipt
    # Paxd binds operation to etx.Type(), including EIP-155 legacy type zero.
    operation = 2 if raw[0] == 2 else 0
    require(verified.height > 0 and facts.module_id == 12 and facts.operation == operation
            and facts.result_code in (0, 1) and facts.from_sequence == nonce
            and facts.from_account == bytes(12) + signer and facts.to_account == bytes(12) + target
            and facts.amount == value, 'signed transaction execution binding mismatch')
    expected = {'txHash': '0x' + keccak256(raw).hex(), 'nonce': hex(nonce),
                'seq': hex(facts.global_sequence), 'height': hex(verified.height),
                'timestamp': hex(facts.timestamp), 'status': hex(1 - facts.result_code),
                'keyId': '0x' + facts.context_hash.hex(), 'from': '0x' + signer.hex(),
                'to': '0x' + target.hex(), 'feeAmount': hex(facts.fee_charged)}
    require(all(row.get(key) == value for key, value in expected.items()), 'source JSON differs from signed receipt')
    require(any(key.key_id == facts.context_hash and key.first_height <= verified.height <= key.last_height
                and row.get('signingKey') == '0x' + key.public_key.hex() for key in pins),
            'source signing key differs from independent receipt pins')
    result = rpc_bytes(row.get('returnData'))
    require(hashlib.sha256(result).digest() == facts.activity_root and facts.effects,
            'source return data differs from signed receipt')
    summary = facts.effects[0]
    body, logs = summary.body, row.get('logs')
    require(summary.module_id == facts.module_id and summary.ordinal == 0 and summary.event_type == 0
            and summary.kind == 1 and not summary.monetary and len(body) >= 110
            and 1 <= body[108] <= 64 and len(body) == 109 + body[108]
            and isinstance(logs, list) and len(logs) == len(facts.effects) - 1
            and len(logs) == int.from_bytes(body[104:108], 'big'), 'signed source summary mismatch')
    expected = {'gasUsed': hex(int.from_bytes(body[:8], 'big')),
                'cumulativeGasUsed': hex(int.from_bytes(body[8:16], 'big')),
                'effectiveGasPrice': hex(int.from_bytes(body[16:24], 'big')),
                'index': hex(int.from_bytes(body[24:28], 'big')),
                'contractAddress': '0x' + body[28:48].hex(), 'sponsor': '0x' + body[48:68].hex(),
                'feeDenom': body[109:].decode('ascii')}
    require(all(row.get(key) == value for key, value in expected.items())
            and isinstance(row.get('vmError'), str)
            and hashlib.sha256(row['vmError'].encode()).digest() == body[68:100]
            and len(result) == int.from_bytes(body[100:104], 'big')
            and isinstance(row.get('paymentDenom'), str)
            and hashlib.sha256(row['paymentDenom'].encode()).digest() == facts.asset,
            'source execution JSON differs from signed summary')
    for index, (log, effect) in enumerate(zip(logs, facts.effects[1:], strict=True)):
        require(isinstance(log, dict) and isinstance(log.get('topics'), list) and len(log['topics']) <= 4,
                'bounded source log required')
        encoded = (rpc_bytes(log.get('address'), 20) + bytes([len(log['topics'])])
                   + b''.join(rpc_bytes(topic, 32) for topic in log['topics'])
                   + hashlib.sha256(rpc_bytes(log.get('data'))).digest())
        require(effect.module_id == facts.module_id and effect.ordinal == index + 1
                and effect.event_type == 0 and effect.kind == 3 and not effect.monetary
                and effect.body == encoded and log.get('logIndex') == hex(index),
                'source log differs from signed effect')
    require(row.get('rung') in ('receipted', 'committed'), 'authenticated source rung required')
    if row['rung'] == 'committed':
        require(quantity(row.get('blockNumber')) >= verified.height and any(rpc_bytes(row.get('blockHash'), 32)),
                'source block commit does not cover the receipt')
    else:
        require('blockNumber' not in row and 'blockHash' not in row, 'uncommitted source carries block evidence')
    return verified


def consume_source(record, pins, source, network_id=125):
    require(isinstance(record, dict) and set(record) == {'source', 'network_id', 'raw_transaction', 'receipts'}
            and record['source'] == source and type(record['network_id']) is int
            and record['network_id'] == network_id and set(record['receipts']) == ROLES,
            'exact source-bound paxd/gateway receipt pair required')
    raw = rpc_bytes(record['raw_transaction'])
    receipts = record['receipts']
    verified = {role: verified_receipt(receipts[role], raw, pins, network_id) for role in ROLES}
    require(receipts['paxd'] == receipts['gateway']
            and verified['paxd'].canonical_bytes == verified['gateway'].canonical_bytes,
            'paxd and gateway source evidence differs')
    return verified['paxd']


def program_envelope(row, network_id=125):
    require(isinstance(row, dict) and set(row) == {'name', 'operation', 'payload_file', 'transaction_file',
            'signer', 'fee_limit', 'expected_result_code', 'expected_terminal_result'}, 'exact signed program input required')
    require(isinstance(row['name'], str) and re.fullmatch('[a-z0-9][a-z0-9-]{0,63}', row['name']),
            'bounded program evidence name required')
    require(type(row['fee_limit']) is int and 0 <= row['fee_limit'] < 1 << 128
            and type(row['expected_result_code']) is int and row['expected_result_code'] in (0, 1),
            'explicit bounded fee and result required')
    payload, raw = read(row['payload_file']), read(row['transaction_file'])
    chain, _, target, value, calldata, signer = transaction(raw)
    require(chain == network_id and target == PROGRAMS and value == 0
            and row['signer'] == '0x' + signer.hex(), 'signed program transaction owner/network mismatch')
    code_hash = None
    if row['operation'] == 'deploy':
        deploy = NativeProgramDeploy.decode(payload)
        require(deploy.encode() == payload and row['expected_terminal_result'] is None,
                'canonical deployment envelope required')
        expected = encode_call('deploy', DEPLOY_TYPES, [deploy.program_id, deploy.guest_abi, deploy.policy,
            deploy.authority, deploy.new_hash, deploy.interface or b'', deploy.wasm])
        program, abi, code_hash = deploy.program_id, deploy.guest_abi, deploy.new_hash.hex()
    else:
        require(row['operation'] == 'call' and type(row['expected_terminal_result']) is int
                and -(1 << 31) <= row['expected_terminal_result'] < 1 << 31, 'explicit program terminal result required')
        call = decode_native_program_call(payload)
        require(encode_native_program_call(call) == payload, 'canonical program call envelope required')
        expected = encode_call('call', CALL_TYPES, [call.program_id, call.guest_abi, call.entrypoint,
            call.calldata, call.capabilities, call.access_declaration, call.response_capacity,
            call.resources, row['fee_limit']])
        program, abi = call.program_id, call.guest_abi
    require(calldata == expected, 'signed program calldata differs from deployment/call envelope')
    return raw, {'name': row['name'], 'operation': row['operation'], 'program_id': program.hex(),
                 'guest_abi': abi, 'code_hash': code_hash, 'payload': artifact(row['payload_file'])}


class ProgramHttpProducer:
    def __init__(self, endpoints, receipt_keys, source, revision, network_id=125):
        self.source = source_identity(source, revision)
        require(type(network_id) is int and 0 < network_id < 1 << 32,
                'explicit bounded paxd network required')
        self.network_id = network_id
        self.pins = decode_receipt_keys(receipt_keys)
        require(isinstance(endpoints, dict) and set(endpoints) == ROLES, 'only paxd and hosted gateway producers are supported')
        self.peers = {}
        for role, config in endpoints.items():
            require(isinstance(config, dict) and set(config) == {'endpoint', 'authorization_file'},
                    'explicit supported producer endpoint required')
            headers = {}
            if config['authorization_file'] is not None:
                authorization = read(config['authorization_file']).decode('ascii')
                require(re.fullmatch(r'(?:LayerX-Key|Bearer) [!-~]{1,512}', authorization),
                        'bounded protected HTTP authorization required')
                headers['Authorization'] = authorization
            self.peers[role] = ProducerRpc(config['endpoint'], headers)
        require(self.peers['paxd'].endpoint != self.peers['gateway'].endpoint,
                'distinct paxd and gateway producer endpoints required')

    def check_network(self):
        for peer in self.peers.values():
            require(peer.call('eth_chainId', []) == hex(self.network_id), 'one independently selected paxd network required')
            require(peer.receipt_keys() == self.pins, 'endpoint receipt keys differ from independent pins')

    def observe(self, raw):
        require(transaction(raw)[0] == self.network_id, 'transaction differs from the independently selected paxd network')
        self.check_network()
        tx_hash = '0x' + keccak256(raw).hex()
        receipts = {role: peer.call('pax_sequencerReceipt', [tx_hash]) for role, peer in self.peers.items()}
        record = {'source': self.source, 'network_id': self.network_id,
                  'raw_transaction': '0x' + raw.hex(), 'receipts': receipts}
        consume_source(record, self.pins, self.source, self.network_id)
        return record

    def submit_program(self, row):
        raw, envelope = program_envelope(row, self.network_id)
        self.check_network()
        tx_hash = self.peers['gateway'].send('0x' + raw.hex())
        record = self.observe(raw)
        verified = consume_source(record, self.pins, self.source, self.network_id)
        require(verified.receipt.result_code == row['expected_result_code'], 'committed program result differs from expected result')
        response = rpc_bytes(record['receipts']['paxd']['returnData'])
        if row['operation'] == 'call' and verified.receipt.result_code == 0:
            require(len(response) >= 128 and len(response) % 32 == 0, 'program terminal ABI response required')
            result = int.from_bytes(response[:32], 'big', signed=True)
            kind, offset, length = (int.from_bytes(response[start:start + 32], 'big') for start in (32, 64, 96))
            require(result == row['expected_terminal_result'] and kind in (1, 2) and offset == 96
                    and len(response) == 128 + (length + 31) // 32 * 32 and not any(response[128 + length:]),
                    'signed program terminal result differs from expected result')
        if row['operation'] == 'deploy' and verified.receipt.result_code == 0:
            require(not response, 'deployment unexpectedly returned an execution envelope')
        require(tx_hash == record['receipts']['paxd']['txHash'], 'HTTP submission and source receipt differ')
        return {'envelope': envelope, 'evidence': record}


def produce(manifest_path, output, revision):
    manifest = load(manifest_path)
    require(isinstance(manifest, dict) and set(manifest) == {'schema', 'source', 'network_id', 'endpoints', 'receipt_keys', 'programs', 'source_transactions'}
            and manifest['schema'] == SCHEMA, 'supported paxd/gateway producer manifest required')
    client = ProgramHttpProducer(manifest['endpoints'], manifest['receipt_keys'], manifest['source'], revision, manifest['network_id'])
    require(isinstance(manifest['programs'], list) and isinstance(manifest['source_transactions'], list)
            and 0 < len(manifest['programs']) + len(manifest['source_transactions']) <= 256,
            'bounded actual program or custody source inputs required')
    if manifest['programs']:
        require({'deploy', 'call'} <= {row['operation'] for row in manifest['programs']},
                'program qualification requires deployment and signed call assertions')
        names = [row['name'] for row in manifest['programs']]
        require(len(names) == len(set(names)), 'duplicate program evidence name')
        envelopes = [program_envelope(row, client.network_id)[1] for row in manifest['programs']]
        deployed = {(bound['program_id'], bound['guest_abi']) for row, bound in zip(manifest['programs'], envelopes, strict=True)
                    if row['operation'] == 'deploy' and row['expected_result_code'] == 0}
        require(all((bound['program_id'], bound['guest_abi']) in deployed for bound in envelopes
                    if bound['operation'] == 'call'), 'every signed call requires its matching successful deployment')
    directory = private(output, True, True)
    result = {'schema': RESULT_SCHEMA, 'source': client.source, 'network_id': client.network_id, 'programs': [],
              'authenticated_source': None, 'passed': False, 'unqualified_criteria': ['block-commit']}
    try:
        for row in manifest['programs']:
            result['programs'].append(client.submit_program(row))
        sources = [client.observe(read(path)) for path in manifest['source_transactions']]
        require(len({record['raw_transaction'] for record in sources}) == len(sources),
                'duplicate authenticated source transaction')
        exported = {'schema': SOURCE_SCHEMA, 'source': client.source, 'network_id': client.network_id, 'records': sources}
        result['authenticated_source'] = write(directory / 'authenticated-source.json', exported)
        all_records = sources + [row['evidence'] for row in result['programs']]
        require(all(record['receipts']['paxd']['rung'] == 'committed' for record in all_records),
                'unqualified until every authenticated receipt reaches block commit')
        result['unqualified_criteria'] = []
        result['passed'] = True
    finally:
        write(directory / 'result.json', result)
    return result


def main(argv=None):
    os.umask(0o077)
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--revision', required=True)
    args = parser.parse_args(argv)
    try:
        result = produce(args.manifest, args.output, args.revision)
        print(json.dumps(result))
        return 0
    except (OSError, ValueError, KeyError, TypeError, PlatformSdkError, PaymentRpcError) as error:
        print('program HTTP producer refused: ' + str(error), file=sys.stderr)
        return 78


if __name__ == '__main__':
    sys.exit(main())
