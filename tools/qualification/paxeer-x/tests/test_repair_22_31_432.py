"""Custody retirement and exact native recovery, without an external toolchain.

Exercise the repaired production qualifier and the real SDK codec/verifier over
HTTP against a recorded peer. The EIP-1193 local rejection is a source-contract
check, not a replacement implementation of PaxeerProvider.
"""
from copy import deepcopy
import json
from pathlib import Path
import sys

import pytest

QUAL = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(QUAL))
from wallet_recovery_recording import (
    RECORDING_TOKEN, RecordedGateway, qualification, qualify_recording, transfer_recording,
)


def client(module, url, state, case, token=RECORDING_TOKEN):
    return module.RecoveryQualification(url, token, state, case['rawTransaction'], case['pinnedKey'])


def test_retired_custody_methods_refused_before_transport():
    assert qualification().assert_provider_contract() == {
        'methods': ['paxeer_recoverCustody', 'paxeer_signCustody'],
        'code': 4200, 'transport': 'local refusal'}


def test_native_send_exact_transaction_and_signed_receipt_across_restart(tmp_path):
    case = transfer_recording()
    result = qualify_recording(tmp_path)
    assert result['exit_code'] == 0
    assert result['tx_hash'] == case['receiptJson']['txHash']
    assert result['recovered_raw'] == case['rawTransaction']
    assert result['economic_sends'] == 1
    consumer = json.loads((tmp_path / 'consumer.json').read_text())
    gateway = json.loads((tmp_path / 'gateway-recording.json').read_text())
    assert consumer['phase'] == 'settled'
    assert consumer['tx'] == gateway['tx']
    assert consumer['tx_hash'] == gateway['tx_hash']
    assert consumer['raw'] == gateway['raw'] == case['rawTransaction']


def test_recorded_command_executes_whole_qualification(capsys):
    module = qualification()
    assert module.main([]) == 0
    result = json.loads(capsys.readouterr().out)
    assert result['qualification'] == 'recorded-native-send-recovery'
    assert result['economic_sends'] == 1


def test_sdk_native_transaction_codec_matches_provider_wire():
    module = qualification()
    case = transfer_recording()
    data, tx, wire, tx_hash = module.native_transaction(case['rawTransaction'])
    assert '0x' + data.hex() == case['rawTransaction']
    assert tx_hash == case['receiptJson']['txHash']
    assert wire == {
        'chainId': 125, 'from': case['receiptJson']['from'], 'to': case['receiptJson']['to'],
        'data': '0x', 'value': '1000000000000000000', 'gas': '21000',
        'maxFeePerGas': '3000000000', 'maxPriorityFeePerGas': '1000000000', 'nonce': 0}
    assert tx.value == 10**18


def test_gateway_authentication_refusal_preserves_uncertain_state(tmp_path):
    module = qualification()
    case = transfer_recording()
    peer = RecordedGateway(tmp_path / 'gateway.json')
    with peer.serving() as url:
        consumer = client(module, url, tmp_path / 'client.json', case, 'foreign-recording-session')
        with pytest.raises(RuntimeError, match='gateway must identify'):
            consumer.submit()
        assert consumer.read_state()['phase'] == 'sending'
        with pytest.raises(RuntimeError, match='never resubmitted'):
            consumer.submit()
    assert peer.accepted is None
    assert not peer.state.exists()
    assert len(peer.requests) == 1


def test_pending_recovery_never_repeats_send(tmp_path):
    module = qualification()
    peer = RecordedGateway(tmp_path / 'gateway.json')
    case = peer.case
    with peer.serving() as url:
        consumer = client(module, url, tmp_path / 'client.json', case)
        consumer.submit()
        for _ in range(2):
            assert consumer.recover() is None
        with pytest.raises(RuntimeError, match='never resubmitted'):
            consumer.submit()
        assert consumer.read_state()['phase'] == 'unknown'
    assert [path for path, _, _ in peer.requests] == ['/v1/wallet/send', '/rpc', '/rpc']


@pytest.mark.parametrize('field,value', [('raw', '0x02'), ('tx_hash', '0x' + '00' * 32),
                                        ('tx', {}), ('phase', 'unverified')])
def test_reopened_consumer_refuses_changed_retained_send(tmp_path, field, value):
    module = qualification()
    peer = RecordedGateway(tmp_path / 'gateway.json')
    state = tmp_path / 'client.json'
    with peer.serving() as url:
        consumer = client(module, url, state, peer.case)
        consumer.submit()
        retained = json.loads(state.read_text())
        retained[field] = value
        state.write_text(json.dumps(retained))
        with pytest.raises(RuntimeError):
            client(module, url, state, peer.case)
    assert len(peer.requests) == 1


@pytest.mark.parametrize('change', ['signature', 'hash', 'amount-json', 'status', 'key', 'rpc-id'])
def test_recovery_refuses_unverified_or_misbound_receipt(tmp_path, change):
    module = qualification()
    case = transfer_recording()
    peer = RecordedGateway(tmp_path / 'gateway.json')
    state = tmp_path / 'client.json'
    with peer.serving() as url:
        client(module, url, state, case).submit()
    reopened = RecordedGateway(peer.state)
    pins = deepcopy(case)
    if change == 'signature':
        raw = bytearray.fromhex(reopened.receipt['receipt'][2:])
        raw[-1] ^= 1
        reopened.receipt['receipt'] = '0x' + raw.hex()
    elif change == 'hash':
        reopened.receipt['txHash'] = '0x' + '00' * 32
    elif change == 'amount-json':
        reopened.receipt['feeAmount'] = '0x0'
    elif change == 'status':
        reopened.receipt['status'] = '0x0'
    elif change == 'key':
        pins['pinnedKey']['publicKey'] = '0x' + '00' * 32
    else:
        reopened.rpc_id = 2
    with reopened.serving() as url:
        consumer = client(module, url, state, pins)
        with pytest.raises((RuntimeError, module.SequencerReceiptVerificationError)):
            consumer.recover()
        assert consumer.read_state()['phase'] == 'unknown'
    assert [path for path, _, _ in reopened.requests] == ['/rpc']


def test_recovery_requires_private_regular_record(tmp_path):
    module = qualification()
    case = transfer_recording()
    state = tmp_path / 'client.json'
    peer = RecordedGateway(tmp_path / 'gateway.json')
    with peer.serving() as url:
        consumer = client(module, url, state, case)
        consumer.submit()
        state.chmod(0o644)
        with pytest.raises(RuntimeError, match='private recovery'):
            consumer.recover()
        state.chmod(0o600)
        link = tmp_path / 'symlink.json'
        link.symlink_to(state)
        with pytest.raises(OSError):
            client(module, url, link, case)
    assert len(peer.requests) == 1


@pytest.mark.parametrize('url', ['https://example.invalid', 'http://127.0.0.1:80/?token=x',
                                 'http://user@127.0.0.1:80', 'http://127.0.0.1:80/foreign'])
def test_recording_never_contacts_external_or_credential_url(tmp_path, url):
    module = qualification()
    with pytest.raises(RuntimeError, match='loopback'):
        client(module, url, tmp_path / 'client.json', transfer_recording())
