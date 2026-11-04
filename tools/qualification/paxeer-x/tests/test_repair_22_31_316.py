"""Focused wire qualification with artifact-bound release-kit integration cases."""
import copy
from contextlib import contextmanager
import importlib
import importlib.util
import json
import os
import signal
from pathlib import Path
import time

import pytest

ROOT = Path(__file__).resolve().parents[4]
SOURCE = ROOT / 'tools/qualification/paxeer-x/agent_operation_envelope.py'
SPEC = importlib.util.spec_from_file_location('repair_22_31_316_envelope', SOURCE)
ENVELOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ENVELOPE)

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.agent_http import encode_proof_bundle_request
from layerx_sdk.verifier import ReceiptKey, SequencerReceiptVerificationError, verify_sequencer_receipt


native_release_kit = pytest.mark.skipif(
    'PAXEER_X_AGENT_ENVELOPE_ARTIFACT_MANIFEST' not in os.environ,
    reason=('Release-kit integration requires PAXEER_X_AGENT_ENVELOPE_ARTIFACT_MANIFEST '
            'from the candidate-bound multi-toolchain build (daemon, gateway and SDK probes); '
            'these native artifacts are unavailable in the qualification environment.'),
)


@pytest.fixture(scope='module')
def portable_vector():
    path = ROOT / 'sdk/platform/conformance/fixtures/vector-v5-portable.json'
    document = json.loads(path.read_text())
    assert document['provenance']
    return document['bundles'][1]


def canonical_requests():
    transaction = 'ab' * 32
    return {
        'export.offline': {'fact_set': ['receipt:' + transaction], 'requested_rung': 'committed'},
        'read.proof_bundle': {'target': '000103' + transaction, 'requested_verification_level': 'receipted'},
        'program.discover': {'program_id': transaction, 'requested_rung': 'submitted'},
        'program.interface': {'program_id': transaction, 'requested_rung': 'submitted'},
        'wait': {'submission_ref': transaction, 'requested_rung': 'committed'},
        'availability.fetch': {'selector': '1', 'requested_rung': 'receipted',
                               'maximum_bytes': '1048576', 'maximum_chunks': '128'},
    }


@pytest.mark.parametrize('operation', sorted(ENVELOPE.READ_WIRE_OPERATIONS))
def test_current_request_grammar_preserves_fields_and_rejects_legacy_field(operation):
    request = canonical_requests()[operation]
    original = copy.deepcopy(request)
    assert ENVELOPE.read_wire_request(operation, request) == original
    legacy = dict(request)
    required_field = 'requested_verification_level' if operation == 'read.proof_bundle' else 'requested_rung'
    incompatible_field = 'requested_rung' if operation == 'read.proof_bundle' else 'requested_verification_level'
    legacy[incompatible_field] = legacy.pop(required_field)
    with pytest.raises(RuntimeError, match=required_field):
        ENVELOPE.read_wire_request(operation, legacy)
    with pytest.raises(RuntimeError, match='exact wire fields'):
        ENVELOPE.read_wire_request(operation, dict(request, unknown='forged'))
    assert request == original


@pytest.mark.parametrize('operation', sorted(ENVELOPE.READ_WIRE_OPERATIONS))
@pytest.mark.parametrize('legacy', (
    'Unverified', 'SequencerSigned', 'BatchIncluded', 'StateProven',
    'CheckpointFinalised', 'SettlementAnchored', 'unverified', 'sequencer-signed', 'batch-included',
))
def test_legacy_enum_is_never_hidden_by_a_canonical_field(operation, legacy):
    field = 'requested_verification_level' if operation == 'read.proof_bundle' else 'requested_rung'
    request = dict(canonical_requests()[operation], **{field: legacy})
    with pytest.raises(RuntimeError, match=field):
        ENVELOPE.read_wire_request(operation, request)


@pytest.mark.parametrize('operation', ('export.offline', 'read.proof_bundle', 'wait', 'availability.fetch'))
@pytest.mark.parametrize('rung', ENVELOPE.RUNGS)
def test_three_supported_request_rungs(operation, rung):
    field = 'requested_verification_level' if operation == 'read.proof_bundle' else 'requested_rung'
    request = dict(canonical_requests()[operation], **{field: rung})
    assert ENVELOPE.read_wire_request(operation, request)[field] == rung


@pytest.mark.parametrize('operation', ('program.discover', 'program.interface'))
@pytest.mark.parametrize('rung', ('receipted', 'committed'))
def test_pending_registry_refuses_unsupported_rung(operation, rung):
    with pytest.raises(RuntimeError, match='pending registry selector'):
        ENVELOPE.read_wire_request(operation, dict(canonical_requests()[operation], requested_rung=rung))


@pytest.mark.parametrize('operation', ('wait', 'availability.fetch'))
@pytest.mark.parametrize('deadline', ('01', '-1', '1.0', 1, str(2**64)))
def test_deadline_is_a_canonical_decimal_owned_by_harness(operation, deadline):
    request = dict(canonical_requests()[operation], deadline=deadline)
    with pytest.raises(RuntimeError, match='decimal deadline'):
        ENVELOPE.read_wire_request(operation, request, with_deadline=True)
    with pytest.raises(RuntimeError, match='exact wire fields'):
        ENVELOPE.read_wire_request(operation, dict(canonical_requests()[operation], deadline='1'))


@pytest.mark.parametrize(('field', 'value'), (
    ('maximum_bytes', '0'), ('maximum_bytes', str(2**64)),
    ('maximum_chunks', '0'), ('maximum_chunks', str(2**32)), ('selector', '01'),
))
def test_availability_byte_chunk_and_height_bounds(field, value):
    request = dict(canonical_requests()['availability.fetch'], **{field: value})
    with pytest.raises(RuntimeError, match='bounded availability request'):
        ENVELOPE.read_wire_request('availability.fetch', request)


def test_recorded_signed_transaction_uses_real_sdk_and_independent_receipt_pin(portable_vector):
    bundle = portable_vector
    key = bundle['receiptKey']
    pins = [ReceiptKey(bytes.fromhex(key['keyId'][2:]), bytes.fromhex(key['publicKey'][2:]),
                       int(key['firstHeight'], 16), int(key['lastHeight'], 16))]
    raw = bytes.fromhex(bundle['rawTransaction'][2:])
    signed = bytes.fromhex(bundle['signedReceipt'][2:])
    verified = verify_sequencer_receipt(signed, raw, pins)
    assert verified.receipt.activity_id == keccak256(raw)
    assert verified.canonical_bytes == signed
    assert verified.receipt_digest.hex() == bundle['receiptDigest'][2:]
    with pytest.raises(SequencerReceiptVerificationError) as refusal:
        verify_sequencer_receipt(signed, raw, [ReceiptKey(pins[0].key_id, bytes(32),
                                                        pins[0].first_height, pins[0].last_height)])
    assert refusal.value.check == 'bad_signature'


@pytest.mark.parametrize('rung', ENVELOPE.RUNGS)
@pytest.mark.parametrize('kind', (1, 2, 3))
def test_proof_bundle_current_sdk_request_round_trip(portable_vector, rung, kind):
    # This is a real SDK interface check, not a replacement request decoder.
    raw = bytes.fromhex(portable_vector['rawTransaction'][2:])
    target = '0001' + bytes([kind]).hex() + keccak256(raw).hex()
    if kind == 2:
        target += 'ab' * 32
    request = {'target': target, 'requested_verification_level': rung}
    original = copy.deepcopy(request)
    assert ENVELOPE.read_wire_request('read.proof_bundle', request) == request
    assert encode_proof_bundle_request(request) == request
    assert request == original


@contextmanager
def real_owner(directory):
    # Use exactly the native artifact/configuration inputs of the real producer.
    # When a manifest is supplied, every native qualification prerequisite is mandatory.
    built = ENVELOPE.load_manifest()
    config, credential = ENVELOPE.load_config()
    runtime_module = importlib.import_module('paxeer_x_runtime_fixture')
    ENVELOPE.fixture = runtime_module
    bundle = runtime_module.artifacts(os.environ.get('PAXEER_X_RUNTIME_ARTIFACTS'))
    client = runtime_module.client_artifact(os.environ.get('PAXEER_X_RUNTIME_CLIENT_MANIFEST'))
    runtime = runtime_module.RuntimeFixture(directory, bundle, client)
    qualification = None
    try:
        runtime.generate()
        runtime.start()
        runtime.readiness()
        qualification = ENVELOPE.Qualification(runtime.directory, built, config, credential, runtime)
        (qualification.d / 'responses').mkdir(mode=0o700)
        (qualification.d / 'probes').mkdir(mode=0o700)
        qualification.start_agentd()
        qualification.bindings()
        qualification.start_gateway()
        yield qualification
    finally:
        if qualification is not None:
            qualification.stop()
        runtime.cleanup()


def restart_owner(owner):
    previous = owner.agentd.pid
    owner.agentd.kill()
    assert owner.agentd.wait(timeout=15) == -signal.SIGKILL
    owner.start_agentd()
    assert owner.agentd.pid != previous


def read(owner, operation, request, case):
    sent, status, body = owner.call(case, operation, request)
    assert sent['request'] == request
    response = owner.read_success(status, body, sent['request_id'], case)
    return owner.read_wire_record(operation, response, request, case)


def sdk_reads(owner, operations, languages, phase, run):
    requests = copy.deepcopy(owner.config['requests'])
    if 'availability.fetch' in operations:
        requests['operation.availability.fetch']['request']['deadline'] = str(int(time.time()) + ENVELOPE.W7_AVAILABILITY_SECONDS)
    cases = tuple('operation.' + operation for operation in operations)
    for language in languages:
        owner.probe(language, cases, phase, owner.d / 'probes' / (language + '-' + run + '.state'),
                    requests, run=run, prefix=True)


@native_release_kit
def test_offline_export_wire_before_after_restart(tmp_path):
    with real_owner(tmp_path) as owner:
        request = owner.config['requests']['operation.export.offline']['request']
        before = read(owner, 'export.offline', request, 'repair.export.pre')
        assert before['facts'] == request['fact_set'] and before['bundles'] and int(before['height']) > 0
        sdk_reads(owner, ('export.offline',), ENVELOPE.LANGUAGES, 'pre-restart', 'repair-export-pre')
        owner.served_read_refused('repair.export.legacy', 'export.offline',
                                  dict(request, requested_verification_level='committed'),
                                  400, 'ProtocolIncompatibility', 'envelope.unknown_field')
        owner.served_read_refused('repair.export.duplicate', 'export.offline',
                                  dict(request, fact_set=[request['fact_set'][0]] * 2),
                                  400, 'ProtocolIncompatibility', 'envelope.malformed')
        oversized = ['receipt:' + i.to_bytes(32, 'big').hex() for i in range(1, 18)]
        owner.served_read_refused('repair.export.count', 'export.offline', dict(request, fact_set=oversized),
                                  400, 'ProtocolIncompatibility', 'envelope.malformed')
        kind, _, identifier = request['fact_set'][0].partition(':')
        assert identifier.upper() != identifier
        owner.served_read_refused('repair.export.uppercase', 'export.offline',
                                  dict(request, fact_set=[kind + ':' + identifier.upper()]),
                                  400, 'ProtocolIncompatibility', 'envelope.malformed')
        restart_owner(owner)
        after = read(owner, 'export.offline', request, 'repair.export.post')
        assert after['facts'] == before['facts']
        assert int(after['height']) >= int(before['height'])
        assert [(b['rawTransaction'], b['signedReceipt']) for b in after['bundles']] == [
            (b['rawTransaction'], b['signedReceipt']) for b in before['bundles']]
        sdk_reads(owner, ('export.offline',), ENVELOPE.LANGUAGES, 'post-restart', 'repair-export-post')


@native_release_kit
def test_proof_bundle_wire_before_after_restart(tmp_path):
    with real_owner(tmp_path) as owner:
        # This signs and submits a genuine activity, tracks the real retained
        # receipt, and retains malformed and five issued wrong-owner cases.
        owner.w7_sign_cases()
        before = {}
        for case, entry in owner.proof_bundle_requests.items():
            if not case.startswith('proof_bundle.refusal.'):
                before[case] = read(owner, entry['operation'], entry['request'], 'repair.' + case + '.pre')
        assert owner.proof_bundle_kinds == {1, 2, 3}
        restart_owner(owner)
        owner.proof_bundle_post_restart()
        for case in before:
            entry = owner.proof_bundle_requests[case]
            after = read(owner, entry['operation'], entry['request'], 'repair.' + case + '.post')
            assert after == before[case]


@native_release_kit
def test_program_read_selectors(tmp_path):
    with real_owner(tmp_path) as owner:
        operations = ('program.discover', 'program.interface')
        for phase in ('pre-restart', 'post-restart'):
            for operation in operations:
                request = owner.config['requests']['operation.' + operation]['request']
                result = read(owner, operation, request, 'repair.' + operation + '.' + phase)
                assert result['program_id'] == request['program_id']
                for suffix, invalid, reason in (
                    ('unknown', dict(request, extra='forged'), 'envelope.unknown_field'),
                    ('rung', dict(request, requested_rung='committed'), 'envelope.malformed'),
                    ('legacy', dict(request, requested_verification_level='submitted'), 'envelope.unknown_field'),
                ):
                    owner.served_read_refused('repair.' + operation + '.' + phase + '.' + suffix,
                                              operation, invalid, 400, 'ProtocolIncompatibility', reason)
            sdk_reads(owner, operations, ENVELOPE.LANGUAGES, phase, 'repair-program-' + phase)
            if phase == 'pre-restart':
                restart_owner(owner)


@native_release_kit
def test_wait_request_rung(tmp_path):
    with real_owner(tmp_path) as owner:
        request = owner.config['requests']['wait_bounded']['request']
        sent, status, body = owner.call('repair.wait.track', 'track', {'submission_ref': request['submission_ref']})
        progress = owner.read_success(status, body, sent['request_id'], 'repair.wait.track')['value']
        assert progress['submission']['submission_ref'] == request['submission_ref']
        owner.w20_wait()
        restart_owner(owner)
        owner.w20_wait('post-restart')
        request = dict(request, deadline=str(int(time.time()) + ENVELOPE.W20_WAIT_SECONDS))
        owner.served_read_refused('repair.wait.legacy', 'wait',
                                  dict(request, requested_verification_level='committed'),
                                  400, 'ProtocolIncompatibility', 'envelope.unknown_field')
        owner.served_read_refused('repair.wait.legacy-enum', 'wait',
                                  dict(request, requested_rung='StateProven'),
                                  400, 'ProtocolIncompatibility', 'envelope.malformed')


@native_release_kit
def test_availability_rung(tmp_path):
    with real_owner(tmp_path) as owner:
        base = owner.config['requests']['operation.availability.fetch']['request']
        for rung in ENVELOPE.RUNGS:
            request = dict(base, requested_rung=rung, deadline=str(int(time.time()) + ENVELOPE.W7_AVAILABILITY_SECONDS))
            started = time.monotonic()
            result = read(owner, 'availability.fetch', request, 'repair.availability.' + rung)
            assert result['completion']['state'] == 'complete'
            assert time.monotonic() - started <= ENVELOPE.W7_AVAILABILITY_SECONDS
            owner.config['requests']['operation.availability.fetch']['request']['requested_rung'] = rung
            sdk_reads(owner, ('availability.fetch',), ENVELOPE.NATIVE_LANGUAGES,
                      'pre-restart', 'repair-availability-' + rung)
        request = dict(base, deadline=str(int(time.time()) + ENVELOPE.W7_AVAILABILITY_SECONDS))
        alias = dict(request)
        alias.pop('requested_rung')
        alias['requested_verification_level'] = 'batch-included'
        for suffix, invalid in (
            ('alias-legacy-enum', alias),
            ('bytes', dict(request, maximum_bytes='0')),
            ('chunks', dict(request, maximum_chunks='0')),
            ('legacy-enum', dict(request, requested_rung='batch-included')),
        ):
            owner.served_read_refused('repair.availability.' + suffix, 'availability.fetch', invalid,
                                      400, 'ProtocolIncompatibility', 'envelope.malformed')
        owner.served_read_refused('repair.availability.expired', 'availability.fetch', dict(request, deadline='0'),
                                  403, 'PolicyRefusal', 'owner.refused')
