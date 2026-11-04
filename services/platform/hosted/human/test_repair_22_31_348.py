"""Signed loopback RPC -> ABI receipts -> retained producer -> installed policy."""
import copy
from contextlib import contextmanager
import datetime
import hashlib
import ipaddress
import json
from pathlib import Path
import sys
import time
from types import SimpleNamespace

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ed25519, rsa
from cryptography.hazmat.primitives.serialization import Encoding, NoEncryption, PrivateFormat, PublicFormat
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import guardians
import material
import merged_owner
import onboarding_material
import owner_native
import provision
from merged_owner import ADDR, AUTH, LEDGER, PAYMENT, OwnerClient, account_id, evm
from repair_server import LoopbackRpcServer, ROOT


def put(path, data):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    path.write_bytes(data)
    path.chmod(0o600)
    return path


def document(path, value):
    return put(path, provision.encode_json(value))


def bind(server, seed, secret):
    signer = ed25519.Ed25519PrivateKey.from_private_bytes(seed)
    public = signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
    sender = evm.address_of(secret)
    message = b'LX:PAXEER-BIND:v1' + (125).to_bytes(32, 'big') + sender + bytes(8)
    server.rpc.send(secret, 125, ADDR, evm.calldata('bindLayerX(bytes32,bytes)', '0x' + public, signer.sign(message)))
    did = ('did:layerx:' + public).encode()
    did_id = owner_native.digest(b'did-id', len(did).to_bytes(2, 'big') + did)
    payload = b'\x71\1\0\2' + did_id + bytes.fromhex(public)
    server.rpc.send(secret, 125, AUTH, evm.calldata(merged_owner.LIFECYCLE, payload, hashlib.sha256(payload).digest()))
    return public, did_id


@pytest.fixture(scope='module')
def environment(tmp_path_factory):
    with merged_environment(tmp_path_factory.mktemp('repair-human')) as value:
        yield value


@contextmanager
def merged_environment(private):
    private = Path(private)
    private.chmod(0o700)
    server = LoopbackRpcServer(private)
    try:
        # Native budget administration requires a genesis main account; funding
        # below still opens and credits the owner's separate live asset account.
        genesis_public = ed25519.Ed25519PrivateKey.from_private_bytes(bytes([13]) * 32).public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
        server.genesis_main(genesis_public)
        funder, issuer = bind(server, bytes([11]) * 32, 11)
        salt = bytes([41]) * 32
        asset = hashlib.sha256(b'LX:ASSET:v1' + issuer + salt).hexdigest()
        server.rpc.send(11, 125, LEDGER, evm.calldata(
            'register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)',
            '0x' + asset, salt, 'HUM', 'Human native', 0, 1000000, 1, b''))
        server.rpc.send(11, 125, LEDGER, evm.calldata('openAccount(bytes32)', '0x' + asset))
        source = account_id(funder, asset)
        server.rpc.send(11, 125, LEDGER, evm.calldata('mint(bytes32,bytes32,uint128)', '0x' + asset, '0x' + source, 10000))
        receiver, _ = bind(server, bytes([12]) * 32, 12)
        server.rpc.send(12, 125, LEDGER, evm.calldata('openAccount(bytes32)', '0x' + asset))
        destination = account_id(receiver, asset)

        work = private / 'work'
        inputs = work / 'human-evidence-input'
        inputs.mkdir(mode=0o700, parents=True)
        secret_dir = private / 'secrets'
        secret_dir.mkdir(mode=0o700)
        for index, role in enumerate(owner_native.GUARDIAN_ROLES, 1):
            guardians.enroll(work, secret_dir, role, bytes([index]).hex() * 32, 1)
        guardians.assemble(work)
        recovery = provision.protected_json(inputs / 'recovery-policy.json')
        seed = bytes([13]) * 32
        public = ed25519.Ed25519PrivateKey.from_private_bytes(seed).public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
        principal = 'did:layerx:' + public
        document(work / 'human-owner-result.json', dict(principal=principal, did=principal,
            recovery_root=recovery['root'], recovery_threshold=recovery['threshold'], recovery_delay_seconds=recovery['delay_seconds']))
        put(inputs / 'owner.seed', seed)
        put(inputs / 'owner.evm', ('%064x' % 13).encode())
        put(inputs / 'funder.evm', ('%064x' % 11).encode())
        config = dict(rpc_url=server.url, rpc_ca_file=str(server.ca), network_id=125, chain_id=125,
            confirmations=1, owner_seed_file=str(inputs / 'owner.seed'), evm_key_file=str(inputs / 'owner.evm'),
            asset_id=asset, funding_key_file=str(inputs / 'funder.evm'), funding_source=source, funding_amount=100)
        document(inputs / 'owner-native.json', config)
        owner_native.produce(work)
        registration = provision.owner_registration(work)
        document(work / 'identity/source-binding.json', dict(tenant='beta', principal=principal))
        document(inputs / 'treasury.json', dict(account=source))
        document(inputs / 'sequencer.json', dict(account=destination))
        document(inputs / 'account-head-result.json', dict(consumed=0))
        registry = private / 'module-registry.json'
        document(registry, dict(schema_version=2, network_id=125, assets=[dict(asset=asset)],
            modules=[dict(module=1, ordinals=[1, 4, 5, 10]), dict(module=3, ordinals=[1, 6]), dict(module=7, ordinals=[1, 3])]))

        # Adjacent naming input is an unchanged, signed deployment recording.
        # This task checks its preservation/pairing, not naming admission semantics.
        recorded = json.loads((ROOT / 'services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json').read_text())
        receipt = bytes.fromhex(recorded['receipt_hex'])
        recorded_fields = owner_native.receipt_fields(receipt, bytes.fromhex(recorded['sequencer_public_key_hex']), 'recorded naming receipt')
        receipt_digest = recorded_fields['receipt_digest']
        journal = work / 'registry-journal'
        put(journal / (receipt_digest + '.admission'), bytes.fromhex(recorded['header_hex']))
        put(journal / (receipt_digest + '.deployment'), receipt)
        document(work / 'naming-deployment-result.json', dict(state='deployed', receipt_digest=receipt_digest,
            activity_id=recorded_fields['activity_id']))
        provision.assemble(work, registry, asset, journal)
        deployment = document(private / 'deployment.json', dict(network_id=125, chain_id=125))
        bundle = private / 'bundle'
        bundle.mkdir(mode=0o700)
        material.assemble_policy(work / 'human-evidence', deployment, registry, bundle / 'policy.json', 125, 125)
        installed = private / 'installed'
        installed.mkdir(mode=0o700)
        for name in ('components', 'kms', 'config', 'agent-config', 'movement-config', 'authority-config', 'authority', 'identity', 'agent'):
            (installed / name).mkdir(mode=0o700)
        put(private / 'receipt-authority-replica-id', bytes([29]).hex().encode() * 32)
        material.install(installed, 125, 125, bundle / 'policy.json')
        policy = provision.protected_json(bundle / 'policy.json')
        context = provision.protected_json(inputs / 'native-context.json')
        client = OwnerClient(context, server.ca.read_bytes(), 13, inputs / 'owner-native-run')
        yield SimpleNamespace(private=private, work=work, inputs=inputs, server=server, asset=asset, source=source,
            destination=destination, public=public, registration=registration, registry=registry, deployment=deployment,
            bundle=bundle, installed=installed, policy=policy, context=context, client=client, recovery=recovery)
    finally:
        server.close()


def test_assembly_custody_records(environment):
    e = environment
    produced = provision.producer_records(e.work, provision.protected_json(e.registry), provision.journal_records(e.work / 'registry-journal'))
    assert set(produced) == set(material.PRODUCER_FILES)
    assert not {'custody.profile', 'owner-custody.json', 'custody-credit.bin'} & set(produced)
    assert material.verify_bundle(e.bundle, 125, 125)['records'] == 2
    provision.assemble(e.work, e.registry, e.asset, e.work / 'registry-journal')
    assert provision.protected_json(e.bundle / 'policy.json') == e.policy
    assert e.policy['principal_policy']['principals'][0]['identities'] == [e.registration['identity']]
    assert provision.protected_json(e.installed / 'authority/principal-policy.json') == e.policy['principal_policy']
    tampered = copy.deepcopy(json.loads(produced['merged-owner-evidence.json']))
    tampered['funding_amount'] += 1
    with pytest.raises(ValueError, match='exact funding'):
        merged_owner.validate_evidence(tampered, e.context, e.server.ca.read_bytes(), e.registration, 125, 125)


def test_purpose_selector_shape(environment):
    e = environment
    catalog = provision.purpose_catalog(HERE / 'beta-purpose-catalog.json', e.registry,
        e.inputs / 'treasury.json', e.inputs / 'sequencer.json', e.asset)
    installed = provision.protected_json(e.installed / 'components/purpose-catalog.json')
    assert catalog == installed == e.policy['purpose_catalog']
    assert all('activity_types' not in p for p in installed['presets'])
    before = e.server.balance(e.destination)[0]
    reference = hashlib.sha256(b'purpose-payment').hexdigest()
    paid = e.client.pay(e.policy, 'beta-protocol', e.destination, 7, reference, time.time_ns() // 1000000 + 300000)
    assert e.server.balance(e.destination)[0] == before + 7
    assert paid['receipt']['to'] == LEDGER
    calls = len(e.server.raw)
    for recipient, amount in [(e.destination, 1001), ('ff' * 32, 1), (e.destination, 0)]:
        with pytest.raises(ValueError):
            e.client.pay(e.policy, 'beta-protocol', recipient, amount, reference, time.time_ns() // 1000000 + 300000)
    for selector, asset in [('0xa9059cbb', e.asset), ('0x78ea11db', 'ff' * 32)]:
        with pytest.raises(ValueError, match='authorization'):
            material.authorize_call(installed, 'beta-protocol', selector, e.destination, asset, 1)
    restricted = copy.deepcopy(e.policy)
    restricted['purpose_catalog']['presets'][0]['call_selectors'] = ['0x8608d32b']
    with pytest.raises(ValueError, match='authorization'):
        e.client.pay(restricted, 'beta-protocol', e.destination, 1, reference, time.time_ns() // 1000000 + 300000)
    assert len(e.server.raw) == calls


def test_disabled_custody_bindings(environment):
    e = environment
    for key, address in [('PAXEER_LEDGER_CONTRACT', LEDGER), ('PAXEER_PAYMENT_CONTRACT', PAYMENT)]:
        assert e.policy['components'][key] == e.policy['movement'][key] == address
        assert (e.installed / 'config' / ('LAYERX_HUMAN_' + key)).read_text() == address
    assert '0x0000000000000000000000000000000000001013' not in json.dumps(e.policy)
    owner = e.registration['owner_account']
    before = e.server.balance(owner)[0]
    recipient_before = e.server.balance(e.destination)[0]
    budget = hashlib.sha256(b'live-human-budget').hexdigest()
    start = time.time_ns() // 1000000 - 1000
    created = e.client.create_budget(e.policy, 'beta-protocol', budget, 20, start)
    assert created['receipt']['to'] == PAYMENT
    assert e.server.balance(owner)[0] == before - 20
    spent = e.client.spend_budget(e.policy, 'beta-protocol', budget, e.destination, 3, hashlib.sha256(b'budget-spend').hexdigest())
    assert spent['receipt']['to'] == PAYMENT
    assert e.server.balance(e.destination)[0] == recipient_before + 3
    assert e.client.create_budget(e.policy, 'beta-protocol', budget, 20, start) == created


def test_owner_custody_bootstrap_restart(environment):
    e = environment
    before = dict(e.server.raw)
    balances = {account: e.server.balance(account) for account in (e.registration['owner_account'], e.source, e.destination)}
    retained = {p.name: p.read_bytes() for p in (e.inputs / 'owner-native-run').iterdir()}
    e.server.restart()
    owner_native.produce(e.work)
    assert dict(e.server.raw) == before
    assert {account: e.server.balance(account) for account in balances} == balances
    assert {p.name: p.read_bytes() for p in (e.inputs / 'owner-native-run').iterdir()} == retained
    assert provision.owner_registration(e.work) == e.registration
    reopened_client = OwnerClient(e.context, e.server.ca.read_bytes(), 13, e.inputs / 'owner-native-run')
    assert reopened_client.rpc.eth_call(LEDGER, 'balanceOf(bytes32,bytes32)(uint128)',
        '0x' + e.registration['owner_account'], '0x' + e.asset) == [balances[e.registration['owner_account']][0]]
    evidence = provision.protected_json(e.inputs / 'merged-owner-evidence.json')
    state = merged_owner.identity_state(evidence['transactions']['recovery']['receipt'], e.public)
    assert list(state[77:109]) == e.recovery['root']
    assert int.from_bytes(state[109:111], 'big') == e.recovery['threshold']
    assert int.from_bytes(state[183:191], 'big') == e.recovery['delay_seconds']
    assert evidence['funding_amount'] == 100 and evidence['funding_source'] == e.source
    with pytest.raises(ValueError, match='network and chain'):
        merged_owner.validate_evidence(evidence, e.context, e.server.ca.read_bytes(), e.registration, 126, 125)
    with pytest.raises(ValueError, match='CA pin'):
        merged_owner.validate_evidence(evidence, e.context, b'wrong CA', e.registration, 125, 125)
    altered = copy.deepcopy(e.registration)
    altered['identity']['recovery']['required_delay_seconds'] += 1
    with pytest.raises(ValueError):
        merged_owner.validate_evidence(evidence, e.context, e.server.ca.read_bytes(), altered, 125, 125)
    transaction = evidence['transactions']['funding']['receipt']['transactionHash']
    block = e.server.blocks[e.server.receipts[transaction]['blockNumber']]
    original = block['hash']
    try:
        block['hash'] = '0x' + 'ff' * 32
        with pytest.raises(ValueError, match='canonical block inclusion'):
            merged_owner.validate_evidence(evidence, e.context, e.server.ca.read_bytes(), e.registration, 125, 125)
    finally:
        block['hash'] = original


def kms_tls(root):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'Human KMS root')])
    now = datetime.datetime.now(datetime.timezone.utc)
    base = x509.CertificateBuilder().not_valid_before(now - datetime.timedelta(minutes=1)).not_valid_after(now + datetime.timedelta(days=1))
    ca = base.subject_name(name).issuer_name(name).public_key(key.public_key()).serial_number(x509.random_serial_number()).add_extension(
        x509.BasicConstraints(ca=True, path_length=None), critical=True).sign(key, hashes.SHA256())
    for role, (common_name, usage) in material.HUMAN_KMS_IDENTITIES.items():
        private = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, common_name)])
        server = usage == 'TLS Web Server Authentication'
        certificate = base.subject_name(subject).issuer_name(name).public_key(private.public_key()).serial_number(
            x509.random_serial_number()).add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True).add_extension(
                x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH if server else ExtendedKeyUsageOID.CLIENT_AUTH]), critical=False)
        if server:
            certificate = certificate.add_extension(x509.SubjectAlternativeName([
                x509.DNSName(material.HUMAN_KMS_SERVER_NAME), x509.IPAddress(ipaddress.ip_address('127.0.0.1'))]), critical=False)
        certificate = certificate.sign(key, hashes.SHA256())
        put(root / role / 'cert.der', certificate.public_bytes(Encoding.DER))
        put(root / role / 'ca.der', ca.public_bytes(Encoding.DER))
        put(root / role / 'key.der', private.private_bytes(Encoding.DER, PrivateFormat.PKCS8, NoEncryption()))


def test_kms_network_registry(environment):
    e = environment
    tls = e.private / 'tls'
    kms_tls(tls)
    destination = e.private / 'kms-material'
    material.kms_prerequisite(e.registry, tls, destination, 125, e.asset, e.private / 'kms-state')
    configuration = e.private / 'onboarding'
    configuration.mkdir(mode=0o700)
    # Reuse the tenancy digest emitted by the real runtime material producer.
    put(configuration / 'LAYERX_HUMAN_TENANCY_DIGEST', (e.installed / 'config/LAYERX_HUMAN_TENANCY_DIGEST').read_bytes())
    args = SimpleNamespace(configuration=str(configuration), state=str(e.private / 'onboarding-state'), runtime=str(e.private / 'run'),
        registry=str(e.registry), native_asset=e.asset, network_id=125, chain_id=125, tenant='beta', initial_funding=1,
        kms_address=material.HUMAN_KMS_ENDPOINT, kms_server_name=material.HUMAN_KMS_SERVER_NAME, tls=str(destination), executable='layerx-human')
    onboarding_material.prepare(args)
    assert provision.protected_json(destination / 'registry.json') == provision.protected_json(configuration / 'registry.json') == e.policy['registry']
    assert provision.protected_json(e.installed / 'kms/registry.json') == e.policy['registry']
    registry_bytes = e.registry.read_bytes()
    wrong = document(e.private / 'wrong-network.json', dict(provision.protected_json(e.registry), network_id=126))
    with pytest.raises(ValueError, match='network-bound'):
        material.kms_prerequisite(wrong, tls, e.private / 'wrong-kms', 125, e.asset, e.private / 'wrong-state')
    with pytest.raises(ValueError, match='network-bound'):
        onboarding_material.prepare(SimpleNamespace(**dict(vars(args), registry=str(wrong))))
    bundle_inputs = {name: (e.bundle / 'inputs' / name).read_bytes() for name in material.EVIDENCE_INPUTS.values()}
    bundle_inputs.update({'producer-records/' + name: (e.bundle / 'inputs/producer-records' / name).read_bytes() for name in material.PRODUCER_FILES})
    bundle_inputs.update({'deployment.json': e.deployment.read_bytes(), 'module-registry.json': wrong.read_bytes()})
    with pytest.raises(ValueError, match='network'):
        material.policy_from_inputs(bundle_inputs, provision.journal_records(e.work / 'registry-journal'), 125, 125)
    assert e.registry.read_bytes() == registry_bytes
    # The in-process TLS validator must still enforce the original CA rows,
    # trust root, EKU separation, server names and possession of its key.
    files = {name: (destination / name).read_bytes() for name in (
        'ca.der', 'kms-server.der', 'kms-server-key.der', 'kms-client.der', 'kms-executor.der')}
    for field, value in (
        ('kms-client.der', files['kms-server.der']),
        ('kms-executor.der', files['kms-client.der']),
        ('kms-server.der', files['kms-client.der']),
        ('kms-server-key.der', (tls / 'human-kms-client/key.der').read_bytes()),
        ('ca.der', files['kms-client.der']),
    ):
        with pytest.raises(ValueError, match='KMS'):
            material.kms_identities(tls, e.private, dict(files, **{field: value}))
    for changes in ({'schema_version': 1}, {'unexpected': True}, {'network_id': True}):
        candidate = dict(provision.protected_json(e.registry), **changes)
        with pytest.raises(ValueError, match='network-bound'):
            material.canonical_module_registry(candidate, 125, e.asset)
