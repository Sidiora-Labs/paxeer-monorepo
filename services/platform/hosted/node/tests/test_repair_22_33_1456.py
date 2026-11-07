"""Audit 22.30.83: native tests, real RPC, descriptor adoption and durable recovery."""
import hashlib
import http.client
import importlib.util
import json
import os
from pathlib import Path
import threading
import unittest

import pytest

from test_repair_22_31_355 import (
    native, native_configuration, native_supervisor, paths, staged_store, tls,
)
from native_key_harness import NativeKeyHarness, public_key_of
from provider import verify
from layerx_sdk.evm_transaction import activity_id, decode_signed

TESTS = Path(__file__).resolve().parent


def load_suite(name, filename):
    spec = importlib.util.spec_from_file_location(name, TESTS / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


GENESIS = load_suite('native_genesis_modules', 'genesis-modules-test.py')
LOST = load_suite('native_lost_response', 'lni-drop-ack.py')
TOPOLOGY = load_suite('native_node_topology', 'node-topology-test.py')
DIRECTORY = load_suite('native_data_directory_tests', 'test_data_directory.py')
TRANSPORT = load_suite('native_generation_transport_tests', 'test_generation_transport.py')
RETIREMENT = load_suite('native_no_guarantor_tests', 'test_node_without_guarantor.py')
RESET = load_suite('native_reset_state_tests', 'test_reset_state.py')


def run_case(case_type, method):
    case = case_type(method)
    result = unittest.TestResult()
    case.run(result)
    assert result.testsRun == 1
    assert not result.skipped, result.skipped
    assert not result.expectedFailures, result.expectedFailures
    assert not result.unexpectedSuccesses, result.unexpectedSuccesses
    assert not result.errors, '\n'.join(value for _, value in result.errors)
    assert not result.failures, '\n'.join(value for _, value in result.failures)
    assert result.wasSuccessful()


@pytest.mark.parametrize('method', [
    'test_enabled_rows_are_canonical_and_rebuild_identically',
    'test_public_defaults_enable_every_runtime_and_match_the_registry',
    'test_unknown_and_repeated_modules_refused_before_generation',
])
def test_genesis_test_retired_tools(tmp_path, tls, capsys, method):
    if method == 'test_unknown_and_repeated_modules_refused_before_generation':
        GENESIS.test_unknown_and_repeated_modules_refused_before_generation(tmp_path, capsys)
    else:
        getattr(GENESIS, method)(tmp_path, tls)


def test_lost_ack_retired_lni(tmp_path, native_configuration):
    config = native_configuration
    data, state, run = paths(tmp_path)
    funding = config['funding']
    raw = bytes.fromhex(funding['raw'][2:])
    transaction, signature = decode_signed(raw)
    assert transaction.recover_signer(signature) == funding['sender']
    assert transaction.chain_id == config['evm_chain_id']
    identity = '0x' + activity_id(raw).hex()
    assert identity == funding['transaction_id']
    assert transaction.value == funding['amount']
    evidence = tmp_path / 'lost-response.json'
    with native_supervisor(config, data, state, run, tmp_path / 'owner.log'):
        with LOST.LostResponseProxy(('127.0.0.1', 0), config['chain_rpc_url'], evidence) as proxy:
            worker = threading.Thread(target=proxy.serve_forever, kwargs={'poll_interval': 0.01})
            worker.start()
            url = 'http://127.0.0.1:' + str(proxy.server_port)
            try:
                before = int(native.rpc(url, 'eth_getBalance', [funding['recipient'], 'latest']), 16)
                with pytest.raises((http.client.RemoteDisconnected, ConnectionResetError)):
                    native.rpc(url, 'eth_sendRawTransaction', [funding['raw']])
                assert proxy.dropped and proxy.transaction_hash == identity
                # Recover using the identity derived before the response was lost.
                receipt = native.rpc(url, 'eth_getTransactionReceipt', [identity])
                assert receipt['transactionHash'] == identity and receipt['status'] == '0x1'
                assert int(receipt['blockNumber'], 16) > 0
                assert int(native.rpc(url, 'eth_getBalance', [funding['recipient'], 'latest']), 16) == before + funding['amount']
                # A second submit is refused before the upstream can see it.
                with pytest.raises(ValueError, match='native RPC transport unavailable'):
                    native.rpc(url, 'eth_sendRawTransaction', [funding['raw']])
                assert sum(item['method'] == 'eth_sendRawTransaction' for item in proxy.requests) == 1
                proxy.record()
            finally:
                proxy.shutdown()
                worker.join(timeout=5)
                assert not worker.is_alive()
    # Stop/reopen the production owner, broker and RPC consumer over retained state.
    with native_supervisor(config, data, state, run, tmp_path / 'owner.log'):
        assert native.rpc(config['chain_rpc_url'], 'eth_getTransactionReceipt', [identity]) == receipt
        assert int(native.rpc(config['chain_rpc_url'], 'eth_getBalance', [funding['recipient'], 'latest']), 16) == before + funding['amount']
    requests = [json.loads(line) for line in config['peer'].journal.read_text().splitlines()]
    submitted = [item['rpc'] for item in requests if item.get('rpc', {}).get('method') == 'eth_sendRawTransaction']
    assert len(submitted) == 1 and submitted[0]['params'] == [funding['raw']]
    recorded = json.loads(evidence.read_bytes())
    assert recorded['submits'] == 1 and recorded['transaction_hash'] == identity
    assert recorded['dropped'] is True


def test_native_lost_response_proxy_enforces_bounds(tmp_path, native_configuration):
    config = native_configuration
    data, state, run = paths(tmp_path)
    with native_supervisor(config, data, state, run, tmp_path / 'owner.log'):
        with LOST.LostResponseProxy(('127.0.0.1', 0), config['chain_rpc_url']) as proxy:
            worker = threading.Thread(target=proxy.serve_forever, kwargs={'poll_interval': 0.01})
            worker.start()
            url = 'http://127.0.0.1:' + str(proxy.server_port)
            try:
                for path, size in (('/rpc', 1), ('/', LOST.MAX_BODY + 1)):
                    connection = http.client.HTTPConnection('127.0.0.1', proxy.server_port, timeout=5)
                    try:
                        connection.request('POST', path, b'x', {'Content-Type': 'application/json',
                                                               'Content-Length': str(size)})
                        assert connection.getresponse().status == 400
                    finally:
                        connection.close()
                assert proxy.requests == []
                for _ in range(LOST.MAX_REQUESTS):
                    assert native.rpc(url, 'eth_chainId', []) == hex(config['evm_chain_id'])
                with pytest.raises(ValueError, match='native RPC transport unavailable'):
                    native.rpc(url, 'eth_chainId', [])
                assert len(proxy.requests) == LOST.MAX_REQUESTS
            finally:
                proxy.shutdown()
                worker.join(timeout=5)
                assert not worker.is_alive()


@pytest.mark.parametrize('method', unittest.defaultTestLoader.getTestCaseNames(TOPOLOGY.NodeTopologyTest))
def test_topology_test_retired_roles(method):
    run_case(TOPOLOGY.NodeTopologyTest, method)


@pytest.mark.parametrize('method', unittest.defaultTestLoader.getTestCaseNames(DIRECTORY.DataDirectoryTests))
def test_existing_data_directory_refusals(method):
    run_case(DIRECTORY.DataDirectoryTests, method)


@pytest.mark.parametrize('failure', [None, ('kill', 0), ('kill', 1)])
def test_activation_success_test_gap(tmp_path, tls, failure):
    DIRECTORY.test_native_activation_exchange_and_post_rename_reopen(tmp_path, tls, failure)


@pytest.mark.parametrize('method', unittest.defaultTestLoader.getTestCaseNames(TRANSPORT.GenerationTransportTests))
def test_generation_transport_refusals(method):
    run_case(TRANSPORT.GenerationTransportTests, method)


def test_generation_test_retired_operation(tmp_path, tls):
    TRANSPORT.test_native_core_generation_adoption_and_broker_reopen(tmp_path, tls)


def test_retirement_test_requires_lni(tmp_path):
    RETIREMENT.test_node_deployment_runs_no_guarantor()
    harness = NativeKeyHarness(tmp_path, recorded=True)
    process = None
    try:
        signer = harness.signer()
        binding = harness.bootstrap(treasury_socket=signer)
        args = native.parser().parse_args(harness.arguments('supervise', treasury_socket=signer))
        store = staged_store(harness.data, harness.state, harness.run)
        process = native.start_native(args, store, commands=harness.peer)
        with harness.consume() as (result, adopted, view):
            assert result['generation'] == 1
            assert adopted['LAYERX_CORE_TREASURY_SIGNER_SOCKET'] == str(signer)
            assert adopted['LAYERX_CORE_CHAIN_RPC_URL'] == harness.config['chain_rpc_url']
            assert not any('LNI' in key or 'GUARANTOR' in key for key in adopted)
            assert native.rpc(adopted['LAYERX_CORE_CHAIN_RPC_URL'], 'eth_chainId', []) == '0x7d'
            consumer = harness.signer_client(Path(adopted['LAYERX_CORE_TREASURY_SIGNER_SOCKET']))
            digest = hashlib.sha256(b'native core without guarantor').digest()
            assert verify(public_key_of(harness.treasury_seed), digest, consumer.sign(digest))
            assert native.generation_binding(harness.data) == binding
    finally:
        native.stop_process(process)
        harness.close()


@pytest.mark.parametrize('method', unittest.defaultTestLoader.getTestCaseNames(RESET.ResetStateTest))
def test_existing_reset_authorization_transition_and_corruption_refusals(method):
    run_case(RESET.ResetStateTest, method)


@pytest.mark.parametrize('interrupted', [False, True])
def test_reset_staged_success_test_gap(tmp_path, native_configuration, interrupted):
    RESET.test_staged_native_reset_success_and_interrupted_reopen(tmp_path, native_configuration, interrupted)


def test_reset_staged_corrupt_generation_is_not_completed(tmp_path, native_configuration):
    RESET.test_staged_native_reset_refuses_corruption_without_success(tmp_path, native_configuration)
