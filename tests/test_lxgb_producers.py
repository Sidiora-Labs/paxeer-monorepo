"""Task 22.33.892, audit 22.30.184 (partial qualification record).

genesis-tests-retired-attestation: genesis_subaccount_allocation_commitment and
historical_attestation_import_contract cover derivation, account transport,
allocation commitments and the historical refusal contract. The allocation
case runs in test_reserve_reconcile, whose declared target links the production
snapshot codec runtime; test_genesis_reconcile retains historical import checks.
lxgb-test-retired-producer: test_merged_genesis_initialization_handoff exercises
the production native bootstrap and descriptor consumer with the committed paxd
initialization transcript; test_merged_genesis_malformed_input_refusal and
test_merged_genesis_authority_refusal cover native input refusals. Historical
fee, truncation and trailing-byte cases remain explicit compatibility checks.
reserve-tests-retired-backing: kernel_ledger_conservation_recovery checks native
registered supply against bank units before/after each signed transfer and
after disk snapshot reopen; historical_reserve_import_contract retains the
explicit historical v1 report bytes and asserts public custody APIs refuse them.
deleted-web-module-happy-path: current_paid_web_request and
current_paid_web_request_account_creation execute a self-contained guest through
the current registration, payment and exact receipt envelope. Historical ABI-v4
cases retain those assertions; historical_web_read_refusal and
current_registration_web_read_refusal preserve legacy refusal.

Wiring request: tools/bringup/foundation.go and modules/lxphost/genesis.go, domain 1, expose native genesis/bank initialization to the in-process test gate.
Wiring request: modules/layerxledger/keeper and modules/lxphost/host.go, domain 1, prove native bank deposit/spend/withdrawal commits and store reopen in that gate.
Wiring request: modules/xweb/keeper and modules/lxphost, domain 16, consume the guest receipt, escrow fees, attest and commit/read the native answer.
Native bank and xweb findings remain open until those owner paths execute: the
declared C link targets cannot call the Go keepers. The native command transcript
proves the bootstrap contract, not in-process Go bank initialization. Baseline
evidence from attempt 1: make -s test-tests-python exited 2 at revision
83a1af209394089fb635857d0008114389a05734 because pytest was missing, before any
Python or C case executed. No individual baseline failure was observed. The
controlling gate order forbids another unchanged-tree verification here.
Qualification evidence is the private hf-build/hf-done ledger for this task;
it binds the complete candidate revision to the declared commands and logs.
First focused verification: revision 39301a364776aedbc715c6baab1c2af9b23b5a51,
command cg spec done 22.33.892 -f layerx-merge --root /work/paxeer-monorepo,
exit 1, log /out/logs/verify-1.log. Allocation and legacy refusal cases passed;
the reserve fixture omitted registered supply and used refused custody APIs.
The scoped repair supplies native totals and uses explicit historical codecs.
"""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

import pytest

from cryptography.hazmat.primitives.asymmetric import ed25519
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'tests/support'))
from lxgb_metadata import metadata, metadata_withdrawal, metadata_modules


def load_fees():
    spec = importlib.util.spec_from_file_location(
        'historical_genesis_fees',
        ROOT / 'services/platform/hosted/node/genesis_fees.py',
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


FEES = load_fees()
HISTORICAL_FIXTURE = ROOT / 'tests/fixtures/public-testnet-genesis'


def native_harness(tmp_path):
    sys.path.insert(0, str(ROOT / 'services/platform/hosted/node/tests'))
    from native_key_harness import NativeKeyHarness

    return NativeKeyHarness(tmp_path, recorded=True)


def test_merged_genesis_initialization_handoff(tmp_path):
    harness = native_harness(tmp_path)
    try:
        source = tmp_path / 'native-inputs/genesis.json'
        raw = source.read_bytes()
        genesis = json.loads(raw)
        assert genesis['chain_id'] == 'hyperpax_125-1'
        assert genesis['validators']
        assert {'bank', 'sequencer', 'lxphost'} <= genesis['app_state'].keys()
        assert not {'layerxcustody', 'layerxanchor', 'guarantor'} & genesis['app_state'].keys()
        binding = harness.bootstrap()
        assert (harness.data / 'config/genesis.json').read_bytes() == raw
        assert binding['config/genesis.json'] == hashlib.sha256(raw).hexdigest()
        requests = [json.loads(line) for line in harness.peer.journal.read_text().splitlines()]
        assert [row['command'][1] for row in requests] == ['init', 'validate-genesis']
        assert all(row['exit'] == 0 for row in requests)
        public = ed25519.Ed25519PrivateKey.from_private_bytes(harness.receipt_seed).public_key()
        public_bytes = public.public_bytes(Encoding.Raw, PublicFormat.Raw)
        # Reopen the broker/consumer twice against the persisted generation.
        for _ in range(2):
            with harness.consume() as (result, adopted, view):
                assert result['generation'] == 1
                assert result['public']['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'] == public_bytes.hex()
                assert adopted['LAYERX_CORE_SEQUENCER_ID'] == hashlib.sha256(public_bytes).hexdigest()
                assert adopted['LAYERX_GENERATION_NUMBER'] == '1'
                assert 'LAYERX_NODE_SEQUENCER_PRIVATE_KEY' not in adopted
                assert view.is_dir()
        assert (harness.data / 'config/genesis.json').read_bytes() == raw
    finally:
        harness.close()


@pytest.mark.parametrize('mutation', ('truncated', 'trailing', 'duplicate'))
def test_merged_genesis_malformed_input_refusal(tmp_path, mutation):
    harness = native_harness(tmp_path)
    try:
        from reset_state import StoreError

        source = tmp_path / 'native-inputs/genesis.json'
        raw = source.read_bytes()
        malformed = {
            'truncated': raw[:-2],
            'trailing': raw + b'\0',
            'duplicate': b'{"chain_id":"hyperpax_125-1","chain_id":"other"}',
        }[mutation]
        source.write_bytes(malformed)
        with pytest.raises(StoreError, match='invalid_json'):
            harness.bootstrap()
        assert not harness.peer.journal.exists()
        assert not (harness.data / '.native-prepared.json').exists()
    finally:
        harness.close()


@pytest.mark.parametrize('mutation', ('chain', 'validators', 'bank', 'kernel'))
def test_merged_genesis_authority_refusal(tmp_path, mutation):
    harness = native_harness(tmp_path)
    try:
        source = tmp_path / 'native-inputs/genesis.json'
        genesis = json.loads(source.read_bytes())
        if mutation == 'chain':
            genesis['chain_id'] = 'retired-chain'
        elif mutation == 'validators':
            genesis['validators'] = []
        elif mutation == 'bank':
            genesis['app_state']['bank'] = None
        else:
            genesis['app_state']['lxphost'] = None
        source.write_text(json.dumps(genesis))
        with pytest.raises(ValueError, match='authoritative native genesis'):
            harness.bootstrap()
        assert not harness.peer.journal.exists()
        assert not (harness.data / '.native-prepared.json').exists()
    finally:
        harness.close()


def historical_fixture():
    """Read an actual archived producer output; never provision guarantors."""
    request = (HISTORICAL_FIXTURE / 'genesis-request.lxgb').read_bytes()
    suffix_length = len(metadata_modules(bytes(range(32)), bytes(32), bytes(32), 0,
                                         (4, 4, 4, 4, 4, 4, 0)))
    suffix = request[-suffix_length:]
    assert request[:5] == b'LXGB\x02'
    assert suffix[:6] == b'\0\x01\0\xba\0\x03'
    cursor = 21 + int.from_bytes(request[19:21], 'big') * 66
    cursor += 2 + int.from_bytes(request[cursor:cursor + 2], 'big') * 81
    assert request[cursor:cursor + 32] == suffix[6:38]
    return request[:-suffix_length], suffix[6:38]


def historical_request(suffix):
    """Vary only archived LXGB metadata for explicit offline import tests."""
    cursor = 2
    for _ in range(int.from_bytes(suffix[:2], 'big')):
        cursor += 2 + int.from_bytes(suffix[cursor:cursor + 2], 'big')
    schedule = suffix[cursor + 2:]
    price = int.from_bytes(schedule[247:255], 'big') if len(schedule) >= 255 else 0
    FEES.withdrawal_metadata(suffix, price)
    prefix, asset = historical_fixture()
    if suffix[6:38] != asset:
        raise ValueError('historical fixture asset must remain bound')
    return prefix + suffix


def test_historical_fee_metadata_roundtrip():
    _, asset = historical_fixture()
    issuer, salt = bytes(range(32)), bytes(range(32, 64))
    legacy = metadata(asset, issuer, salt)
    withdrawal = metadata_withdrawal(asset, issuer, salt, 17)
    prices = (4, 4, 4, 4, 4, 4, 0)
    modules = metadata_modules(asset, issuer, salt, 17, prices)
    assert withdrawal[-255:] == (ROOT / 'tests/fixtures/fee-params-v3.bin').read_bytes()
    assert modules[-368:] == (ROOT / 'tests/fixtures/fee-params-v4.bin').read_bytes()
    assert withdrawal[:-257] == legacy[:-249]
    assert FEES.withdrawal_metadata(legacy, 17) == withdrawal
    assert FEES.withdrawal_metadata(withdrawal, 17) == withdrawal
    for source in (legacy, withdrawal, modules):
        assert FEES.module_metadata(source, 17, prices) == modules
    for index in range(7):
        changed = list(prices)
        changed[index] += 1
        with pytest.raises(ValueError, match='module prices disagree'):
            FEES.module_metadata(modules, 17, changed)
    with pytest.raises(ValueError, match='withdrawal price disagrees'):
        FEES.withdrawal_metadata(withdrawal, 0)


def test_historical_metadata_truncation_and_trailing_bytes():
    _, asset = historical_fixture()
    issuer, salt = bytes(range(32)), bytes(range(32, 64))
    schedules = (
        metadata(asset, issuer, salt),
        metadata_withdrawal(asset, issuer, salt, 17),
        metadata_modules(asset, issuer, salt, 17, (4, 4, 4, 4, 4, 4, 0)),
    )
    for suffix in schedules:
        assert historical_request(suffix).endswith(suffix)
        for malformed in (suffix[:-1], suffix + b'\0', b'', suffix[:219], bytes(16384)):
            with pytest.raises(ValueError):
                historical_request(malformed)
    malformed = bytearray(schedules[-1])
    malformed[-113] = 6
    with pytest.raises(ValueError):
        FEES.withdrawal_metadata(bytes(malformed), 17)


def test_historical_metadata_fee_bounds(tmp_path):
    _, asset = historical_fixture()
    suffix = metadata(asset, bytes(range(32)), bytes(range(32, 64)))
    prices = (4, 4, 4, 4, 4, 4, 0)
    for invalid in ((), prices[:-1], prices + (0,), (-1, *prices[1:]),
                    (2**128, *prices[1:]), (True, *prices[1:])):
        with pytest.raises(ValueError):
            metadata_modules(asset, bytes(range(32)), bytes(range(32, 64)), 17, invalid)
        with pytest.raises(ValueError):
            FEES.module_metadata(suffix, 17, invalid)
    for price in (-1, 2**64, True):
        with pytest.raises(ValueError):
            metadata_withdrawal(asset, bytes(range(32)), bytes(range(32, 64)), price)
        with pytest.raises(ValueError):
            FEES.withdrawal_metadata(suffix, price)
    path = tmp_path / 'prices.json'
    for text in ('{}', '{"escrow":4,"escrow":4}',
                 '{"escrow":4,"budget":4,"stream":4,"service":4,"perps":4,"governance":4,"bridge":0,"asset":0}'):
        path.write_text(text)
        with pytest.raises(ValueError):
            FEES.module_prices(path)
    path.write_text(json.dumps(dict(zip(FEES.MODULE_NAMES, prices))))
    assert FEES.module_prices(path) == prices


def test_historical_artifacts_bind_producer_roots():
    expected = json.loads((HISTORICAL_FIXTURE / 'expected.json').read_text())
    for name, digest in expected['sha256'].items():
        assert hashlib.sha256((HISTORICAL_FIXTURE / name).read_bytes()).hexdigest() == digest
    registration = (HISTORICAL_FIXTURE / 'paxeer-registration-request.lxrr').read_bytes()
    assert len(registration) == 73
    assert registration[:9] == b'LXRR\x01' + expected['network_id'].to_bytes(4, 'big')
    assert registration[9:41].hex() == expected['canonical_state_root']
    assert registration[41:73].hex() == expected['receipt_state_root']


def main():
    parser = argparse.ArgumentParser(
        description='Offline historical LXGB import compatibility; does not initialize paxd.',
    )
    parser.add_argument('--builder', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    work = args.output.resolve()
    work.mkdir(mode=0o700, parents=True, exist_ok=False)
    seed = os.urandom(32)
    with (work / 'signer').open('xb') as out:
        os.chmod(work / 'signer', 0o600)
        out.write(seed)
    issuer = ed25519.Ed25519PrivateKey.from_private_bytes(seed).public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    _, asset = historical_fixture()
    salt = os.urandom(32)
    suffix = metadata(asset, issuer, salt)
    request = historical_request(suffix)
    assert request[:5] == b'LXGB\x02' and request.endswith(suffix)
    for label, payload, accepted in [('valid', request, True), ('truncated', request[:-1], False),
                                     ('trailing', request + b'\0', False),
                                     ('missing-metadata', request[:-len(suffix)], False)]:
        path = work / f'{label}.lxgb'
        path.write_bytes(payload)
        result = subprocess.run([str(args.builder.resolve()), str(path), str(work / 'signer'), str(work / label)],
                                capture_output=True)
        assert (result.returncode == 0) == accepted, f'{label}: exit {result.returncode}'
        assert (work / label / 'genesis.manifest').exists() == accepted
        print(f'{label}: native builder exit={result.returncode}; signed artifacts={accepted}')
    withdrawal = metadata_withdrawal(asset, issuer, salt, 17)
    native_fee = (ROOT / 'tests/fixtures/fee-params-v3.bin').read_bytes()
    assert len(native_fee) == 255 and withdrawal[-255:] == native_fee
    assert len(withdrawal) == len(suffix) + 8 and withdrawal[:-257] == suffix[:-249]
    withdrawal_request = historical_request(withdrawal)
    for label, payload, accepted in [('withdrawal', withdrawal_request, True),
                                     ('withdrawal-truncated', withdrawal_request[:-1], False),
                                     ('withdrawal-trailing', withdrawal_request + b'\0', False)]:
        path = work / f'{label}.lxgb'
        path.write_bytes(payload)
        result = subprocess.run([str(args.builder.resolve()), str(path), str(work / 'signer'), str(work / label)],
                                capture_output=True)
        assert (result.returncode == 0) == accepted, f'{label}: exit {result.returncode}'
        assert (work / label / 'genesis.manifest').exists() == accepted
        print(f'{label}: native builder exit={result.returncode}; signed artifacts={accepted}')
    assert (work / 'withdrawal/genesis.manifest').read_bytes() != (work / 'valid/genesis.manifest').read_bytes()
    assert (work / 'withdrawal/00000000000000000000.lxs').read_bytes() != (work / 'valid/00000000000000000000.lxs').read_bytes()
    prices = (4, 4, 4, 4, 4, 4, 0)
    modules = metadata_modules(asset, issuer, salt, 17, prices)
    native_modules = (ROOT / 'tests/fixtures/fee-params-v4.bin').read_bytes()
    assert len(native_modules) == 368 and modules[-368:] == native_modules
    assert FEES.module_metadata(suffix, 17, prices) == modules
    assert FEES.module_metadata(withdrawal, 17, prices) == modules
    assert FEES.module_metadata(modules, 17, prices) == modules
    modules_request = historical_request(modules)
    bad_count = bytearray(modules_request)
    bad_count[-113] = 6
    for label, payload, accepted in [('modules', modules_request, True),
                                     ('modules-truncated', modules_request[:-1], False),
                                     ('modules-trailing', modules_request + b'\0', False),
                                     ('modules-count', bytes(bad_count), False)]:
        path = work / f'{label}.lxgb'
        path.write_bytes(payload)
        result = subprocess.run([str(args.builder.resolve()), str(path), str(work / 'signer'), str(work / label)],
                                capture_output=True)
        assert (result.returncode == 0) == accepted, f'{label}: exit {result.returncode}'
        assert (work / label / 'genesis.manifest').exists() == accepted
        print(f'{label}: native builder exit={result.returncode}; signed artifacts={accepted}')
    assert (work / 'modules/genesis.manifest').read_bytes() != (work / 'withdrawal/genesis.manifest').read_bytes()
    assert (work / 'modules/00000000000000000000.lxs').read_bytes() != (work / 'withdrawal/00000000000000000000.lxs').read_bytes()
    for invalid in [(), prices[:-1], prices + (0,), (-1, *prices[1:]), (2**128, *prices[1:]), (True, *prices[1:])]:
        for emitter in [lambda: metadata_modules(asset, issuer, salt, 17, invalid),
                        lambda: FEES.module_metadata(suffix, 17, invalid)]:
            try:
                emitter()
            except ValueError:
                pass
            else:
                raise AssertionError('module fee count or integer bound was not enforced')
    for index in range(7):
        changed = list(prices)
        changed[index] += 1
        try:
            FEES.module_metadata(modules, 17, changed)
        except ValueError:
            pass
        else:
            raise AssertionError('supplied module schedule was silently replaced')
    for text in ['{}', '{"escrow":4,"escrow":4}',
                 '{"escrow":4,"budget":4,"stream":4,"service":4,"perps":4,"governance":4,"bridge":0,"asset":0}']:
        path = work / 'invalid-prices.json'
        path.write_text(text)
        try:
            FEES.module_prices(path)
        except ValueError:
            pass
        else:
            raise AssertionError('noncanonical module fee configuration accepted')
    for price in (-1, 2**64, True):
        try:
            metadata_withdrawal(asset, issuer, salt, price)
        except ValueError:
            pass
        else:
            raise AssertionError('withdrawal price u64 bound was not enforced')
    for suffix_input in [b'', suffix[:219], bytes(16384)]:
        try:
            historical_request(suffix_input)
        except ValueError:
            pass
        else:
            raise AssertionError('producer metadata bounds were not enforced')
    print('Historical LXGB v2 import preserves metadata; offline signing and malformed-request refusals passed')


if __name__ == '__main__':
    main()
