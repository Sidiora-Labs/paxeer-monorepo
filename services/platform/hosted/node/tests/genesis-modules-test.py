import base64
import copy
import json
import re
from pathlib import Path
import sys

import pytest

TESTS = Path(__file__).resolve().parent
sys.path.insert(0, str(TESTS))
from native_key_harness import NativeKeyHarness, native
from test_repair_22_31_355 import (
    FIXTURES, RecordedNativePeer, staged_store, tls,
)
import generation_transport as transport

MODULE_FIXTURES = TESTS / 'fixtures' / 'repair_22_33_1456'


class ModuleGenesisPeer(RecordedNativePeer):
    """Recorded native CLI/RPC peer bound to one committed module configuration."""
    def __init__(self, configuration, root, genesis):
        self.genesis = genesis
        configuration = copy.deepcopy({key: value for key, value in configuration.items() if key != 'peer'})
        configuration['rpc_exchanges'].append({
            'method': 'genesis', 'params': {}, 'result': {'genesis': genesis},
        })
        super().__init__(configuration, root)

    def validate_inputs(self, home):
        assert json.loads(native.read_regular(home / 'config/genesis.json')) == self.genesis
        for name in ('config.toml', 'app.toml'):
            assert native.read_regular(home / 'config' / name) == (FIXTURES / name).read_bytes()
        assert json.loads(native.read_regular(home / 'config/priv_validator_key.json')) == self.configuration['validator_key']


def assert_native_modules(genesis, modules):
    pay = genesis['app_state']['layerxpay']
    assert pay['params']['paused'] is False
    for module in ('escrow', 'budget', 'stream', 'service'):
        assert pay['params'][module + '_enabled'] is (module in modules)
    rows = pay['kernel_records']
    keys = [base64.b64decode(row['key'], validate=True) for row in rows]
    assert keys == sorted(keys) and len(keys) == len(set(keys)) == len(modules)
    assert keys == [b'\x01\x00\x07' + ('module-enable:' + module).encode().ljust(32, b'\0')
                    for module in sorted(modules)]
    assert all(base64.b64decode(row['value'], validate=True) == bytes(31) + b'\x01' for row in rows)
    assert genesis['app_state']['layerxexchange']['params']['paused'] is False
    assert genesis['app_state']['lxphost']['last_sequence'] == 0
    assert genesis['app_state']['bank']['balances']


def check_reconstruction(root, selection):
    root.mkdir(mode=0o700)
    harness = NativeKeyHarness(root, recorded=True)
    original_peer = harness.peer
    overlay = json.loads((MODULE_FIXTURES / (selection + '.json')).read_text())
    genesis = json.loads((FIXTURES / 'genesis.json').read_text())
    genesis['app_state'].update(overlay['app_state'])
    native.write_file(Path(harness.config['native_inputs']) / 'genesis.json',
                      json.dumps(genesis, sort_keys=True, separators=(',', ':')).encode())
    peer = ModuleGenesisPeer(harness.config, root, genesis)
    harness.peer = peer
    harness.config.update(chain_rpc_url='http://127.0.0.1:' + str(peer.chain.server_port),
                          consensus_rpc_url='http://127.0.0.1:' + str(peer.consensus.server_port),
                          gateway_endpoint='https://localhost:' + str(peer.gateway.server_port))
    process = None
    try:
        first = harness.bootstrap()
        args = native.parser().parse_args(harness.arguments('supervise'))
        store = staged_store(harness.data, harness.state, harness.run)
        process = native.start_native(args, store, commands=peer)
        node = transport.environment((harness.data / 'node.env').read_bytes())
        assert native.committed_health(node)['height'] > 0
        observed = native.rpc(node['LAYERX_NODE_CONSENSUS_RPC_URL'], 'genesis', {})['genesis']
        assert observed == genesis
        assert_native_modules(observed, overlay['modules'])
        with harness.consume() as (result, adopted, view):
            assert_native_modules(json.loads((view / 'genesis.json').read_bytes()), overlay['modules'])
            assert result['generation'] == 1
            assert adopted['LAYERX_CORE_CHAIN_RPC_URL'] == harness.config['chain_rpc_url']
        rebuilt = root / 'rebuilt'
        rebuilt.mkdir(mode=0o700)
        second = native.bootstrap(args, data=rebuilt, commands=peer)
        assert first == second == native.generation_binding(rebuilt)
        for name in first:
            assert (harness.data / name).read_bytes() == (rebuilt / name).read_bytes()
        native.stop_process(process)
        process = native.start_native(args, store, commands=peer)
        assert native.rpc(node['LAYERX_NODE_CONSENSUS_RPC_URL'], 'genesis', {})['genesis'] == observed
        assert native.generation_binding(harness.data) == first
        return overlay['modules']
    finally:
        native.stop_process(process)
        peer.close()
        harness.peer = original_peer
        harness.close()


def test_enabled_rows_are_canonical_and_rebuild_identically(tmp_path, tls):
    assert check_reconstruction(tmp_path / 'stream', 'stream') == ['stream']
    assert check_reconstruction(tmp_path / 'all', 'all') == ['budget', 'escrow', 'perps', 'service', 'spot', 'stream']


def test_unknown_and_repeated_modules_refused_before_generation(tmp_path, capsys):
    # Module selection now belongs to authoritative native genesis, never daemon CLI flags.
    for modules in (('asset',), ('escrow', 'escrow'), ('Escrow',), ('',)):
        data = tmp_path / 'must-not-generate'
        arguments = ['bootstrap', '--data-dir', str(data)]
        for module in modules:
            arguments.extend(['--enable-module', module])
        with pytest.raises(SystemExit) as error:
            native.parser().parse_args(arguments)
        assert error.value.code == 2
        assert 'unrecognized arguments: --enable-module' in capsys.readouterr().err
        assert not data.exists()


def test_public_defaults_enable_every_runtime_and_match_the_registry(tmp_path, tls):
    modules = check_reconstruction(tmp_path / 'default', 'all')
    assert (TESTS.parent / 'genesis-modules.conf').read_text().splitlines() == modules
    registry = (TESTS.parents[4] / 'include/layerx/lxp_module.h').read_text()
    for module, identifier in (('escrow', 2), ('budget', 3), ('stream', 4),
                               ('service', 5), ('perps', 6), ('spot', 10)):
        assert module in modules
        assert re.search(r'\bLXP_MODULE_' + module.upper() + r'\s*=\s*' + str(identifier) + r'\b', registry)


if __name__ == '__main__':
    raise SystemExit(pytest.main([__file__]))
