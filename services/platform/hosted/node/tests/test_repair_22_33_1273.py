"""Audit 22.30.63 findings 006/007: native owner, handoff and signer behavior."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

import pytest
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from native_key_harness import NativeKeyHarness, native, public_key_of, signer_service
from client import SignerClient, SignerError
from provider import FileProvider, ProviderError, verify, verify_binding

TESTS = Path(__file__).resolve().parent


def load_suite(name, filename):
    spec = importlib.util.spec_from_file_location(name, TESTS / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


SEED_SUITE = load_suite('native_seed_suite', 'sequencer-seed-test.py')
SIGNER_SUITE = load_suite('native_signer_suite', 'signer-test.py')
SEED_TESTS = unittest.defaultTestLoader.getTestCaseNames(SEED_SUITE.SequencerSeedTest)
TREASURY_TESTS = unittest.defaultTestLoader.getTestCaseNames(SIGNER_SUITE.BootstrapTreasuryCase)


def run_native_case(case_type, method):
    case = case_type(method)
    # Explicit committed native peer; the owner and every signer/descriptor component are real.
    case.recorded_native_peer = True
    result = unittest.TestResult()
    case.run(result)
    assert result.testsRun == 1
    assert not result.skipped, result.skipped
    assert not result.expectedFailures, result.expectedFailures
    assert not result.unexpectedSuccesses, result.unexpectedSuccesses
    assert not result.errors, '\n'.join(value for _, value in result.errors)
    assert not result.failures, '\n'.join(value for _, value in result.failures)
    assert result.wasSuccessful()


@pytest.mark.parametrize('method', SEED_TESTS)
def test_finding_006_native_seed_harness(method):
    assert len(SEED_TESTS) == 14
    run_native_case(SEED_SUITE.SequencerSeedTest, method)


@pytest.mark.parametrize('method', TREASURY_TESTS)
def test_finding_007_native_treasury_bootstrap(method):
    assert len(TREASURY_TESTS) == 3
    run_native_case(SIGNER_SUITE.BootstrapTreasuryCase, method)


def test_finding_007_recipient_binding_private_file_and_restart():
    with tempfile.TemporaryDirectory(prefix='native-binding-') as temporary:
        harness = NativeKeyHarness(temporary, recorded=True)
        try:
            policy_path = Path(temporary) / 'recipient-policy.json'
            native.write_file(policy_path, json.dumps({
                'version': 1, 'network_id': harness.config['network_id'],
                'asset_id': harness.config['asset'], 'recipient': 'ab' * 20,
            }).encode())
            public = public_key_of(harness.treasury_seed)
            digest = hashlib.sha256(b'native recipient binding recovery').digest()
            signatures = []
            for restart in range(2):
                policy = signer_service.BindingPolicy(policy_path)
                with harness.serving_signer(policy=policy) as socket_path:
                    consumer = harness.signer_client(socket_path)
                    did = consumer.did()
                    name = ('agent:' + did + ':main').encode('ascii')
                    account = hashlib.sha256(b'LX:ACCOUNT:v1' + len(name).to_bytes(4, 'big') + name).digest()
                    fields = (harness.config['network_id'], account,
                              bytes.fromhex(harness.config['asset']), bytes.fromhex('ab' * 20), digest)
                    signature = consumer.bind(*fields)
                    payload = fields[0].to_bytes(4, 'big') + b''.join(fields[1:])
                    assert verify_binding(public, payload, signature)
                    assert consumer.bind(*fields) == signature
                    signatures.append((signature, consumer.sign(digest)))
                    assert verify(public, digest, signatures[-1][1])
                    verifier = Ed25519PublicKey.from_public_bytes(public)
                    with pytest.raises(InvalidSignature):
                        verifier.verify(signature, b'LX:SETTLE:RECIPIENT:v2\0' + payload)
                    for offset in (0, 4, 36, 68, 88):
                        changed = bytearray(payload)
                        changed[offset] ^= 1
                        assert not verify_binding(public, bytes(changed), signature)
                    for index in range(5):
                        changed = list(fields)
                        changed[index] = fields[0] + 1 if index == 0 else os.urandom(len(fields[index]))
                        if index == 4:
                            other = consumer.bind(*changed)
                            assert other != signature
                            assert verify_binding(public, changed[0].to_bytes(4, 'big') +
                                                  b''.join(changed[1:]), other)
                        else:
                            with pytest.raises(SignerError):
                                consumer.bind(*changed)
                    with pytest.raises(SignerError):
                        SignerClient(str(socket_path)).bind(*fields)
                    for uid, gid in ((os.geteuid() + 1, os.getegid()),
                                     (os.geteuid(), os.getegid() + 1)):
                        refused = SignerClient(str(socket_path), expected_peer_uid=uid,
                                               expected_peer_gid=gid, expected_public_key=public)
                        with pytest.raises(SignerError):
                            refused.bind(*fields)
                    with pytest.raises(SignerError):
                        harness.signer_client(socket_path, public_key=bytes(32)).bind(*fields)
                    assert socket_path.stat().st_mode & 0o777 == 0o600
                    assert harness.treasury_key.stat().st_mode & 0o777 == 0o600
                    assert harness.treasury_key.read_bytes() == harness.treasury_seed
                    assert consumer.bind(*fields) == signature
                assert not socket_path.exists(), restart
                with pytest.raises(SignerError):
                    consumer.public_key()
            assert signatures[0] == signatures[1]
            harness.assert_no_copy(unittest.TestCase(), harness.treasury_seed, harness.treasury_key)
        finally:
            harness.close()


@pytest.mark.parametrize('mode', [0o644, 0o620])
def test_finding_007_private_seed_permissions_are_required(mode):
    with tempfile.TemporaryDirectory(prefix='native-private-') as temporary:
        harness = NativeKeyHarness(temporary, recorded=True)
        try:
            harness.treasury_key.chmod(mode)
            with pytest.raises(ProviderError):
                harness.signer()
            assert not (harness.run / 'treasury.sock').exists()
            assert harness.treasury_key.read_bytes() == harness.treasury_seed
            assert not list(harness.data.iterdir())
        finally:
            harness.close()


def test_finding_007_private_seed_links_and_malformed_material_are_refused():
    with tempfile.TemporaryDirectory(prefix='native-material-') as temporary:
        harness = NativeKeyHarness(temporary, recorded=True)
        try:
            symlink = Path(temporary) / 'symlink.key'
            symlink.symlink_to(harness.treasury_key)
            with pytest.raises(OSError):
                harness.signer(key=symlink)
            hardlink = Path(temporary) / 'hardlink.key'
            os.link(harness.treasury_key, hardlink)
            for path in (hardlink, harness.treasury_key):
                with pytest.raises(ProviderError):
                    harness.signer(key=path)
            hardlink.unlink()
            harness.treasury_key.write_bytes(b'not a treasury seed\n')
            with pytest.raises(ProviderError):
                harness.signer()
            assert not (harness.run / 'treasury.sock').exists()
            assert not list(harness.data.iterdir())
        finally:
            harness.close()
