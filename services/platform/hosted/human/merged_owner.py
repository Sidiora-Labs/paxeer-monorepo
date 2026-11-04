"""Merged owner bootstrap through the public EVM precompiles.

Receipts are checked against the authenticated RPC again when retained evidence is
consumed. A pending transaction is saved before broadcast and is the only
transaction a restart may submit for that operation.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import ssl
import time
from urllib.parse import urlsplit

from provision import fields, h32, protected_bytes, protected_json, require, uint, write_json

_spec = importlib.util.spec_from_file_location('human_paxeer_evm', Path(__file__).parents[1] / 'paxeer/evm.py')
evm = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(evm)

LEDGER = '0x000000000000000000000000000000000000101a'
PAYMENT = '0x000000000000000000000000000000000000101b'
AUTH = '0x000000000000000000000000000000000000101c'
ADDR = '0x0000000000000000000000000000000000001004'
SCHEMA = 'layerx.human.merged-owner.v1'
CONTEXT_FIELDS = 'network_id chain_id rpc_url rpc_ca_sha256 confirmations'
PAY_EXACT = 'payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)'
LIFECYCLE = 'recordLifecycle(bytes,bytes32)'


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def account_name(public, asset):
    return 'agent:did:layerx:' + public + ':asset:' + asset


def account_id(public, asset):
    name = account_name(public, asset).encode('ascii')
    return hashlib.sha256(b'LX:ACCOUNT:v1' + len(name).to_bytes(4, 'big') + name).hexdigest()


def rlp_decode(raw):
    def item(offset):
        require(offset < len(raw), 'transaction', 'RLP bound')
        tag = raw[offset]
        if tag < 0x80:
            return raw[offset:offset + 1], offset + 1
        base = 0x80 if tag < 0xc0 else 0xc0
        length = tag - base
        offset += 1
        if length > 55:
            width = length - 55
            require(offset + width <= len(raw) and raw[offset] != 0, 'transaction', 'RLP length')
            length = int.from_bytes(raw[offset:offset + width], 'big')
            require(length >= 56, 'transaction', 'minimal RLP length')
            offset += width
        end = offset + length
        require(end <= len(raw), 'transaction', 'RLP payload bound')
        if base == 0x80:
            return raw[offset:end], end
        result = []
        while offset < end:
            value, offset = item(offset)
            result.append(value)
        require(offset == end, 'transaction', 'RLP list bound')
        return result, end
    value, end = item(0)
    require(end == len(raw) and evm.rlp(value) == raw, 'transaction', 'canonical RLP')
    return value


def transaction_fields(raw_hex, chain):
    raw = evm.unhex(raw_hex)
    values = rlp_decode(raw)
    require(type(values) is list and len(values) == 9 and all(type(v) is bytes for v in values),
            'transaction', 'signed EIP-155 transaction')
    numbers = [0, 1, 2, 4, 6, 7, 8]
    for index in numbers:
        require(not values[index] or values[index][0] != 0, 'transaction', 'minimal transaction integer')
    v, r, s = (int.from_bytes(values[i], 'big') for i in (6, 7, 8))
    require(v in (chain * 2 + 35, chain * 2 + 36) and 0 < r < evm.N and 0 < s <= evm.N // 2,
            'transaction', 'chain-bound canonical signature')
    point = evm.recover(evm.keccak(evm.rlp(values[:6] + [chain, 0, 0])), r, s, (v - 35) % 2)
    require(point is not None and len(values[3]) == 20, 'transaction', 'transaction signer and target')
    return dict(hash='0x' + evm.keccak(raw).hex(), sender='0x' + evm.address_of_point(point).hex(),
                to='0x' + values[3].hex(), data='0x' + values[5].hex(),
                value=int.from_bytes(values[4], 'big'), nonce=int.from_bytes(values[0], 'big'))


def rpc_for(context, ca_bytes):
    fields(context, CONTEXT_FIELDS, 'merged owner', 'RPC context')
    uint(context['network_id'], 32, 'merged owner', 'network', 1)
    uint(context['chain_id'], 64, 'merged owner', 'chain', 1)
    uint(context['confirmations'], 32, 'merged owner', 'confirmations', 1)
    require(context['rpc_ca_sha256'] == hashlib.sha256(ca_bytes).hexdigest(), 'merged owner', 'RPC CA pin')
    url = urlsplit(context['rpc_url'])
    require(url.scheme == 'https' and url.hostname and url.username is None and url.password is None
            and not url.query and not url.fragment, 'merged owner', 'authenticated HTTPS JSON-RPC')
    rpc = evm.Rpc(context['rpc_url'], timeout=10)
    rpc.context = ssl.create_default_context(cadata=ca_bytes.decode('ascii'))
    require(rpc.chain_id() == context['chain_id'], 'merged owner', 'RPC chain binding')
    return rpc


def checked_receipt(rpc, transaction, confirmations):
    h32(transaction.removeprefix('0x'), 'merged owner', 'transaction hash')
    receipt = rpc.receipt(transaction, deadline=30)
    require(type(receipt) is dict and receipt.get('transactionHash') == transaction
            and receipt.get('status') == '0x1' and type(receipt.get('logs')) is list,
            transaction, 'successful transaction receipt')
    number = int(receipt['blockNumber'], 16)
    require(number > 0 and int(rpc.request('eth_blockNumber', []), 16) >= number + confirmations - 1,
            transaction, 'confirmed transaction')
    block = rpc.request('eth_getBlockByNumber', [receipt['blockNumber'], False])
    require(block['hash'] == receipt['blockHash'] and block['number'] == receipt['blockNumber']
            and transaction in block['transactions'], transaction, 'canonical block inclusion')
    for index, log in enumerate(receipt['logs']):
        require(log.get('transactionHash') == transaction and log.get('blockHash') == receipt['blockHash']
                and log.get('blockNumber') == receipt['blockNumber'] and log.get('removed') is False
                and int(log['logIndex'], 16) >= 0, transaction, 'canonical receipt log')
    return receipt


def event(receipt, address, signature, topics, types):
    expected = ['0x' + evm.keccak(signature.encode('ascii')).hex(), *topics]
    matches = [log for log in receipt['logs'] if log['address'].lower() == address
               and log['topics'] == expected]
    require(len(matches) == 1, 'merged owner', 'exact ' + signature + ' event')
    data = evm.unhex(matches[0]['data'])
    result = evm.decode(types, data)
    require(evm.encode(types, result) == data, 'merged owner', 'canonical event ABI')
    return result


def reference(receipt):
    return dict(activity_id=receipt['transactionHash'][2:], receipt_digest=hashlib.sha256(canonical(receipt)).hexdigest())


def identity_state(receipt, public):
    from owner_native import digest
    did = ('did:layerx:' + public).encode('ascii')
    did_id = digest(b'did-id', len(did).to_bytes(2, 'big') + did)
    state, = event(receipt, AUTH, 'IdentityStateUpdated(bytes32,bytes)', ['0x' + did_id.hex()], ['bytes'])
    state = evm.unhex(state)
    require(len(state) == 223 and state[:5] == b'LXGI1' and state[5:37] == did_id
            and state[37:69] == bytes.fromhex(public), 'merged owner', 'committed owner identity snapshot')
    return state


def registration_from_receipts(evidence):
    public, asset = evidence['public_key'], evidence['asset_id']
    h32(public, 'merged owner', 'public key')
    h32(asset, 'merged owner', 'asset')
    state = identity_state(evidence['transactions']['recovery']['receipt'], public)
    read = lambda offset: int.from_bytes(state[offset:offset + 8], 'big')
    require(read(175) > 0 and read(183) > 0 and read(191) >= read(183),
            'merged owner', 'committed recovery delay bounds')
    # recordLifecycle serves registration and recovery. Rotation is unavailable
    # at this interface, so a bootstrap cannot advertise a rotation policy.
    require(state[111:167] == bytes(56) and read(167) == 0 and read(199) == 0 and read(207) == 0,
            'merged owner', 'no unproven rotation policy')
    recovered = evidence['transactions']['recovery']['receipt']
    return dict(schema=SCHEMA, owner_account=account_id(public, asset), authority=public,
        identity=dict(did='did:layerx:' + public, authorities=[dict(kind='primary_key', id=public)],
            revocation_sequence=read(69), frozen=False, evidence=reference(recovered), capabilities=[],
            recovery=dict(policy_revision=read(175), required_delay_seconds=read(183),
                maximum_delay_seconds=read(191), effective_sequence=read(215), evidence=reference(recovered))))


def validate_registration(value, path):
    from provision import evidence as check_evidence, key_policy
    fields(value, 'schema owner_account authority identity', path, 'merged registration')
    require(value['schema'] == SCHEMA, path, 'merged registration schema')
    h32(value['owner_account'], path, 'ledger owner account')
    h32(value['authority'], path, 'owner authority')
    identity = value['identity']
    fields(identity, 'did authorities revocation_sequence frozen evidence capabilities recovery', path, 'merged identity')
    require(identity['did'] == 'did:layerx:' + value['authority']
            and identity['authorities'] == [dict(kind='primary_key', id=value['authority'])]
            and identity['frozen'] is False and identity['capabilities'] == [], path, 'merged owner identity binding')
    uint(identity['revocation_sequence'], 64, path, 'revocation sequence', 1)
    check_evidence(identity['evidence'], path, 'identity evidence', None)
    key_policy(identity['recovery'], path, 'recovery policy', None)


def validate_evidence(evidence, context, ca_bytes, registration, network, chain):
    fields(evidence, 'schema public_key asset_id funding_source funding_amount transactions', 'merged owner', 'evidence')
    require(evidence['schema'] == SCHEMA and context['network_id'] == network and context['chain_id'] == chain,
            'merged owner', 'evidence network and chain')
    validate_registration(registration, 'merged owner')
    require(registration == registration_from_receipts(evidence), 'merged owner', 'registration from current receipts')
    public, asset = evidence['public_key'], evidence['asset_id']
    account = account_id(public, asset)
    uint(evidence['funding_amount'], 128, 'merged owner', 'exact funding amount', 1)
    h32(evidence['funding_source'], 'merged owner', 'funding source')
    transactions = evidence['transactions']
    require(set(transactions) == {'binding', 'identity', 'account', 'recovery', 'funding'}, 'merged owner', 'bootstrap evidence set')
    rpc = rpc_for(context, ca_bytes)
    sender = None
    for label, record in transactions.items():
        fields(record, 'raw receipt', 'merged owner', 'transaction record')
        signed = transaction_fields(record['raw'], chain)
        receipt = record['receipt']
        require(signed['hash'] == receipt['transactionHash'] and signed['to'] == receipt['to'].lower()
                and signed['sender'] == receipt['from'].lower() and signed['value'] == 0,
                'merged owner', 'signed transaction and receipt binding')
        require(checked_receipt(rpc, signed['hash'], context['confirmations']) == receipt,
                'merged owner', 'retained receipt differs from authenticated chain')
        if label != 'funding':
            sender = signed['sender'] if sender is None else sender
            require(signed['sender'] == sender, 'merged owner', 'single owner EVM signer')
        if label == 'binding':
            require(signed['to'] == ADDR and signed['data'].startswith('0x' + evm.selector('bindLayerX(bytes32,bytes)').hex()),
                    'merged owner', 'owner binding call')
            data = evm.unhex(signed['data'])[4:]
            key, signature = evm.decode(['bytes32', 'bytes'], data)
            require(evm.encode(['bytes32', 'bytes'], [key, signature]) == data and key == '0x' + public,
                    'merged owner', 'canonical owner binding')
            nonce, = event(receipt, ADDR, 'LayerXBound(address,bytes32,uint64)',
                ['0x' + bytes(12).hex() + signed['sender'][2:], '0x' + public], ['uint64'])
            from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
            Ed25519PublicKey.from_public_bytes(bytes.fromhex(public)).verify(evm.unhex(signature),
                b'LX:PAXEER-BIND:v1' + chain.to_bytes(32, 'big') + evm.unhex(signed['sender'], 20) + nonce.to_bytes(8, 'big'))
        elif label in ('identity', 'recovery'):
            require(signed['to'] == AUTH, 'merged owner', 'live identity precompile')
            state = identity_state(receipt, public)
            from owner_native import digest
            did = ('did:layerx:' + public).encode('ascii')
            did_id = digest(b'did-id', len(did).to_bytes(2, 'big') + did)
            if label == 'identity':
                payload = b'\x71\1\0\2' + did_id + bytes.fromhex(public)
                require(int.from_bytes(state[69:77], 'big') > 0, 'merged owner', 'identity registration')
            else:
                payload = b'\x71\3\0\5' + did_id + state[77:111] + state[183:199]
            action = hashlib.sha256(b'LX:HUMAN:MERGED:v1\0' + label.encode() + bytes.fromhex(public)).digest()
            require(signed['data'] == evm.calldata(LIFECYCLE, payload, action), 'merged owner', 'identity lifecycle payload binding')
        elif label == 'account':
            require(signed['to'] == LEDGER and signed['data'] == evm.calldata('openAccount(bytes32)', '0x' + asset),
                    'merged owner', 'live owner account opening')
            event(receipt, LEDGER, 'AccountOpened(bytes32)', ['0x' + account], [])
        else:
            require(signed['to'] == LEDGER and signed['data'].startswith('0x' + evm.selector(PAY_EXACT).hex()),
                    'merged owner', 'ledger funding transaction')
            args = evm.decode(['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'], evm.unhex(signed['data'])[4:])
            require(signed['data'] == evm.calldata(PAY_EXACT, *args)
                    and args[:4] == ['0x' + evidence['funding_source'], '0x' + account, '0x' + asset, evidence['funding_amount']],
                    'merged owner', 'exact funding source, owner, asset and amount')
            amount, kind = event(receipt, LEDGER, 'Transfer(bytes32,bytes32,bytes32,uint128,uint8)', args[:3], ['uint128', 'uint8'])
            require(amount == evidence['funding_amount'] and kind == 5, 'merged owner', 'funding transfer effect')
            actual_asset, actual_amount = event(receipt, LEDGER, 'PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)',
                                              [args[5], args[0], args[1]], ['bytes32', 'uint128'])
            require(actual_asset == '0x' + asset and actual_amount == amount, 'merged owner', 'funding payment effect')
    bound, did = rpc.eth_call(ADDR, 'getLayerXDid(address)(bytes32,string)', sender)
    require(bound == '0x' + public and did == registration['identity']['did'], 'merged owner', 'current owner binding')
    ledger_account, = rpc.eth_call(LEDGER, 'accountId(bytes32,bytes32)(bytes32)', '0x' + public, '0x' + asset)
    require(ledger_account == '0x' + account, 'merged owner', 'current asset account binding')
    return evidence


class OwnerClient:
    def __init__(self, context, ca_bytes, secret, run_dir):
        self.context, self.ca_bytes = context, ca_bytes
        self.rpc = rpc_for(context, ca_bytes)
        self.secret, self.run_dir = secret, Path(run_dir)
        self.sender = '0x' + evm.address_of(secret).hex()

    def send(self, label, target, data):
        from owner_native import protected_write
        directory = self.run_dir
        directory.mkdir(mode=0o700, exist_ok=True)
        from material import protected_file, sync_directory
        protected_file(directory, 0o700)
        pending = directory / (label + '.transaction.json')
        if pending.exists() or pending.is_symlink():
            stored = protected_json(pending)
            fields(stored, 'raw', pending, 'durable pending transaction')
            raw = stored['raw']
        else:
            call = dict(to=target, data=data, value='0x0', **{'from': self.sender})
            gas = int(self.rpc.request('eth_estimateGas', [call]), 16)
            gas += gas // 5
            nonce = int(self.rpc.request('eth_getTransactionCount', [self.sender, 'pending']), 16)
            gas_price = int(self.rpc.request('eth_gasPrice', []), 16)
            raw = '0x' + evm.sign_transaction(self.secret, self.context['chain_id'], nonce, gas_price, gas,
                                              evm.unhex(target, 20), 0, evm.unhex(data)).hex()
            write_json(pending, dict(raw=raw))
            sync_directory(directory)
        signed = transaction_fields(raw, self.context['chain_id'])
        require(signed['sender'] == self.sender and signed['to'] == target and signed['data'] == data,
                pending, 'retained transaction intent binding')
        receipt = self.rpc.request('eth_getTransactionReceipt', [signed['hash']])
        if receipt is None:
            require(self.rpc.request('eth_sendRawTransaction', [raw]) == signed['hash'], pending, 'broadcast hash binding')
        receipt = checked_receipt(self.rpc, signed['hash'], self.context['confirmations'])
        result = dict(raw=raw, receipt=receipt)
        retained = directory / (label + '.json')
        if retained.exists():
            require(protected_json(retained) == result, retained, 'unchanged confirmed transaction')
        else:
            protected_write(retained, canonical(result))
            sync_directory(directory)
        return result

    def pay(self, policy, preset_id, recipient, amount, payment_reference, expires_at):
        from material import authorize_call
        public = policy['components']['AGENT_AUTHORITY']
        asset = policy['principal_policy']['principals'][0]['asset_id']
        source = account_id(public, asset)
        data = evm.calldata(PAY_EXACT, '0x' + source, '0x' + recipient, '0x' + asset,
                            amount, expires_at, '0x' + payment_reference)
        authorize_call(policy['purpose_catalog'], preset_id, data, recipient, asset, amount)
        return self.send('payment-' + payment_reference, LEDGER, data)


    def create_budget(self, policy, preset_id, budget_id, amount, start_ms):
        from material import validate_catalog
        presets = [p for p in validate_catalog(policy['purpose_catalog'])['presets'] if p['id'] == preset_id]
        require(len(presets) == 1, 'merged owner', 'declared budget purpose')
        preset = presets[0]
        require(type(amount) is int and 0 < amount <= preset['amount_ceiling'], 'merged owner', 'bounded budget funding')
        h32(budget_id, 'merged owner', 'budget id')
        public = policy['components']['AGENT_AUTHORITY']
        asset = policy['principal_policy']['principals'][0]['asset_id']
        source = account_id(public, asset)
        name = ('agent:did:layerx:' + public + ':budget:' + budget_id).encode('ascii')
        budget_account = hashlib.sha256(b'LX:ACCOUNT:v1' + len(name).to_bytes(4, 'big') + name).hexdigest()
        pending = self.run_dir / ('budget-create-' + budget_id + '.transaction.json')
        if pending.exists():
            signed = transaction_fields(protected_json(pending)['raw'], self.context['chain_id'])
            sequence = evm.decode(['bytes32'] * 4 + ['uint128'] * 3 + ['uint64'] * 4
                                  + ['uint8', 'bytes32', 'uint64'], evm.unhex(signed['data'])[4:])[-1]
        else:
            sequence, = self.rpc.eth_call(LEDGER, 'nextSequence(bytes32,bytes32)(uint64)', '0x' + source, '0x' + asset)
        data = evm.calldata('budgetCreateNative(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bytes32,uint64)',
            '0x' + budget_id, '0x' + budget_account, '0x' + asset, hashlib.sha256(preset_id.encode()).digest(),
            preset['amount_ceiling'], 0, amount, preset['budget_period_seconds'] * 1000,
            start_ms, start_ms + preset['budget_expiry_seconds'] * 1000,
            policy['principal_policy']['principals'][0]['identities'][0]['revocation_sequence'],
            1, '0x' + source, sequence)
        return self.send('budget-create-' + budget_id, PAYMENT, data)

    def spend_budget(self, policy, preset_id, budget_id, recipient, amount, operation_id):
        from material import authorize_call
        h32(operation_id, 'merged owner', 'budget payment operation')
        asset = policy['principal_policy']['principals'][0]['asset_id']
        data = evm.calldata('budgetSpend(bytes32,bytes32,uint128)', '0x' + budget_id, '0x' + recipient, amount)
        authorize_call(policy['purpose_catalog'], preset_id, data, recipient, asset, amount)
        return self.send('budget-spend-' + operation_id, PAYMENT, data)


def produce(work_dir):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
    from owner_native import guardian_set, guardian_commitment, digest
    root = Path(work_dir) / 'human-evidence-input'
    path = root / 'owner-native.json'
    config = protected_json(path)
    fields(config, 'rpc_url rpc_ca_file network_id chain_id confirmations owner_seed_file evm_key_file asset_id funding_key_file funding_source funding_amount',
           path, 'merged owner configuration')
    for name in ('asset_id', 'funding_source'):
        h32(config[name], path, name)
    uint(config['funding_amount'], 128, path, 'funding_amount', 1)
    ca_bytes = protected_bytes(config['rpc_ca_file'], 65536)
    context = {key: config[key] for key in ('network_id', 'chain_id', 'rpc_url', 'confirmations')}
    context['rpc_ca_sha256'] = hashlib.sha256(ca_bytes).hexdigest()
    seed = protected_bytes(config['owner_seed_file'], 32)
    require(len(seed) == 32, path, 'owner Ed25519 seed')
    signer = Ed25519PrivateKey.from_private_bytes(seed)
    public = signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
    owner = protected_json(Path(work_dir) / 'human-owner-result.json')
    require(owner['did'] == 'did:layerx:' + public, path, 'original owner DID must match the merged binding key')
    policy = protected_json(root / 'recovery-policy.json')
    from provision import recovery_policy
    recovery_policy(policy, path)
    require(policy == dict(root=owner['recovery_root'], threshold=owner['recovery_threshold'],
                           delay_seconds=owner['recovery_delay_seconds']), path, 'original recovery binding')
    guardians = guardian_set(protected_json(root / 'recovery-guardians.json'), path, policy['threshold'])
    require(list(guardian_commitment(policy['threshold'], guardians)) == policy['root'], path, 'signed guardian set')
    protected_bytes(config['evm_key_file'], 128)
    client = OwnerClient(context, ca_bytes, evm.read_key(config['evm_key_file']), root / 'owner-native-run')
    transactions = {}
    binding_file = client.run_dir / 'binding.transaction.json'
    if binding_file.exists():
        raw = protected_json(binding_file)['raw']
        binding_data = transaction_fields(raw, config['chain_id'])['data']
    else:
        nonce, = client.rpc.eth_call(ADDR, 'layerXBindNonce(address)(uint64)', client.sender)
        message = b'LX:PAXEER-BIND:v1' + config['chain_id'].to_bytes(32, 'big') + evm.unhex(client.sender, 20) + nonce.to_bytes(8, 'big')
        binding_data = evm.calldata('bindLayerX(bytes32,bytes)', '0x' + public, signer.sign(message))
    transactions['binding'] = client.send('binding', ADDR, binding_data)
    did = owner['did'].encode('ascii')
    did_id = digest(b'did-id', len(did).to_bytes(2, 'big') + did)
    payloads = dict(identity=b'\x71\1\0\2' + did_id + bytes.fromhex(public),
                    recovery=b'\x71\3\0\5' + did_id + bytes(policy['root'])
                    + policy['threshold'].to_bytes(2, 'big') + policy['delay_seconds'].to_bytes(8, 'big') * 2)
    for label, payload in payloads.items():
        action = hashlib.sha256(b'LX:HUMAN:MERGED:v1\0' + label.encode() + bytes.fromhex(public)).digest()
        transactions[label] = client.send(label, AUTH, evm.calldata(LIFECYCLE, payload, action))
    transactions['account'] = client.send('account', LEDGER, evm.calldata('openAccount(bytes32)', '0x' + config['asset_id']))
    protected_bytes(config['funding_key_file'], 128)
    funder = OwnerClient(context, ca_bytes, evm.read_key(config['funding_key_file']), client.run_dir)
    funding_file = client.run_dir / 'funding.transaction.json'
    if funding_file.exists():
        retained = transaction_fields(protected_json(funding_file)['raw'], config['chain_id'])
        expiry = evm.decode(['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'],
                            evm.unhex(retained['data'])[4:])[4]
    else:
        expiry = time.time_ns() // 1000000 + 300000
    funding_reference = hashlib.sha256(b'LX:HUMAN:FUNDING:v1\0' + canonical(dict(
        network_id=config['network_id'], chain_id=config['chain_id'], public_key=public,
        asset_id=config['asset_id'], source=config['funding_source'], amount=config['funding_amount']))).digest()
    transactions['funding'] = funder.send('funding', LEDGER, evm.calldata(PAY_EXACT,
        '0x' + config['funding_source'], '0x' + account_id(public, config['asset_id']),
        '0x' + config['asset_id'], config['funding_amount'], expiry, funding_reference))
    evidence = dict(schema=SCHEMA, public_key=public, asset_id=config['asset_id'], funding_source=config['funding_source'],
                    funding_amount=config['funding_amount'], transactions=transactions)
    registration = registration_from_receipts(evidence)
    validate_evidence(evidence, context, ca_bytes, registration, config['network_id'], config['chain_id'])
    state = identity_state(transactions['recovery']['receipt'], public)
    require(list(state[77:109]) == policy['root'] and int.from_bytes(state[109:111], 'big') == policy['threshold']
            and int.from_bytes(state[183:191], 'big') == policy['delay_seconds']
            and int.from_bytes(state[191:199], 'big') == policy['delay_seconds'], path, 'exact committed recovery policy')
    balance, = client.rpc.eth_call(LEDGER, 'balanceOf(bytes32,bytes32)(uint128)',
                                  '0x' + registration['owner_account'], '0x' + config['asset_id'])
    output = root / 'owner-registration.json'
    if not output.exists():
        require(balance >= config['funding_amount'], path, 'funded owner ledger balance')
    from onboarding_material import write
    for name, data in [('merged-owner-evidence.json', canonical(evidence)), ('native-context.json', canonical(context)),
                       ('rpc-ca.pem', ca_bytes), ('owner-registration.json', canonical(registration))]:
        write(root / name, data)
    return registration
