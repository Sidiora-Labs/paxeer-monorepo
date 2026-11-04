"""Audit 22.30.83: native generation, retired capabilities and signer files."""
import base64
import hashlib
import json
import os
from pathlib import Path
import signal
import socket
import sys
from types import SimpleNamespace

import pytest

NODE = Path(__file__).resolve().parent
sys.path.insert(0, str(NODE / 'tests'))
sys.path.insert(0, str(NODE))

from native_key_harness import NativeKeyHarness, public_key_of
from test_repair_22_31_355 import (
    ForkedProcess, broker, client, native, paths, recorded_generation,
    staged_store, tls, wait_until,
)
import generation_transport as transport
from reset_state import StoreError, canonical, decode
from provider import FileProvider, verify


def request(**updates):
    value = {'version': transport.NATIVE_CORE_VERSION, 'operation': 'core',
             'expected_generation': 1}
    value.update(updates)
    return value


def refused(path, value):
    with socket.socket(socket.AF_UNIX) as connection:
        connection.settimeout(5)
        connection.connect(str(path))
        connection.sendall(canonical(value) + b'\n')
        raw, ancillary, flags, _ = connection.recvmsg(8192, socket.CMSG_SPACE(64))
        assert decode(raw) == {'error': {'code': 'generation_unavailable'}}
        assert ancillary == []
        assert not flags & (socket.MSG_CTRUNC | socket.MSG_TRUNC)


def test_generation_legacy_artifacts_descriptor_adoption_and_child_restart(tmp_path, tls):
    data, state, run = paths(tmp_path)
    binding, core = recorded_generation(data, tls)
    staged_store(data, state, run)
    output = tmp_path / 'core-adopted.json'
    command = [sys.executable, str(NODE / 'tests/fixtures/repair_22_33_1453/core_child.py'),
               str(output)]
    pids = []
    for _ in range(2):
        output.unlink(missing_ok=True)
        with broker(data, state, run) as socket_path:
            args = SimpleNamespace(socket=str(socket_path), operation='core',
                                   broker_uid=os.geteuid(), broker_gid=os.getegid(),
                                   watch_seconds=0.1)
            process = ForkedProcess(lambda: client.supervise(args, command))
            try:
                wait_until(output.exists, process)
                adopted = json.loads(output.read_bytes())
                pids.append(adopted['pid'])
                assert adopted['generation'] == '1'
                assert adopted['artifacts'] == {
                    name.rsplit('/', 1)[-1]: digest for name, digest in binding.items()}
                assert adopted['trust'] == hashlib.sha256(tls[0].read_bytes()).hexdigest()
                assert adopted['rpc'] == core['LAYERX_CORE_CHAIN_RPC_URL']
                assert adopted['funding_key'] == core['LAYERX_CORE_FUNDING_SENDER_KEY_FILE']
                assert adopted['public_key'] == transport.environment(
                    (data / 'node.env').read_bytes())['LAYERX_NODE_SEQUENCER_PUBLIC_KEY']
                assert not set(adopted['keys']) & client.RETIRED_FIELDS
                assert not any(key.startswith(('LAYERX_AUTHORITY_', 'LAYERX_GUARANTOR_'))
                               for key in adopted['keys'])
            finally:
                native.stop_process(process)
        wait_until(lambda: not Path('/proc', str(pids[-1])).exists())
        assert staged_store(data, state, run).generation() == 1
        assert native.generation_binding(data) == binding
    assert pids[0] != pids[1]


@pytest.mark.parametrize('updates', [
    {'operation': 'identity'},
    {'operation': 'identity', 'slot': 1,
     'capability': base64.b64encode(bytes(range(32))).decode()},
    {'slot': 1}, {'slot': True}, {'capability': 'not-a-capability'},
    {'path': '../../private/key.pem'}, {'version': 1}, {'version': True},
])
def test_generation_guarantor_operation_refused_with_core_handoff(tmp_path, tls, updates):
    data, state, run = paths(tmp_path)
    binding, _ = recorded_generation(data, tls)
    staged_store(data, state, run)
    with broker(data, state, run) as socket_path:
        refused(socket_path, request(**updates))
        result, descriptors = client.receive(str(socket_path), 'core', 1,
                                             os.geteuid(), os.getegid())
        try:
            assert result['version'] == 2
            assert result['artifacts'] == list(transport.GENESIS)
            assert {name: hashlib.sha256(os.pread(fd, transport.MAX_ARTIFACT, 0)).hexdigest()
                    for name, fd in zip(result['artifacts'], descriptors)} == binding
            assert client.child_environment(result, descriptors, 'core')['LAYERX_GENERATION_NUMBER'] == '1'
            with pytest.raises(StoreError) as error:
                client.receive(str(socket_path), 'identity', 1, os.geteuid(), os.getegid())
            assert error.value.code == 'invalid_operation'
        finally:
            client.close_descriptors(descriptors)
    assert not (state / 'generation-authorizations').exists()
    assert not (data / 'producer-generations').exists()


@pytest.mark.parametrize('key', sorted(client.RETIRED_FIELDS) + [
    'LAYERX_AUTHORITY_REPLICA_ID', 'LAYERX_GUARANTOR_SLOT',
])
def test_native_child_refuses_inherited_retired_exports(tmp_path, tls, monkeypatch, key):
    data, state, run = paths(tmp_path)
    recorded_generation(data, tls)
    staged_store(data, state, run)
    with broker(data, state, run) as socket_path:
        result, descriptors = client.receive(str(socket_path), 'core', 1,
                                             os.geteuid(), os.getegid())
        try:
            monkeypatch.setenv(key, 'retired-setting')
            with pytest.raises(StoreError) as error:
                client.child_environment(result, descriptors, 'core')
            assert error.value.code == 'retired_environment'
            monkeypatch.delenv(key)
            assert client.child_environment(result, descriptors, 'core')['LAYERX_GENERATION_NUMBER'] == '1'
        finally:
            client.close_descriptors(descriptors)


@pytest.mark.parametrize('target,key', [
    ('node', 'LAYERX_NODE_SNAPSHOT'), ('node', 'LAYERX_AUTHORITY_REPLICA_ID'),
    ('core', 'LAYERX_NODE_SEQUENCER_PRIVATE_KEY'), ('core', 'LAYERX_CORE_LNI_SOCKET'),
])
def test_native_producer_refuses_retired_schema_before_publication(tmp_path, tls, target, key):
    data, state, run = paths(tmp_path)
    binding, core = recorded_generation(data, tls)
    node = transport.environment((data / 'node.env').read_bytes())
    (node if target == 'node' else core)[key] = 'retired-setting'
    with pytest.raises(ValueError, match='closed core schema'):
        native.publish_generation(data, node, core, tls[0].read_bytes())
    assert native.generation_binding(data) == binding
    assert decode((data / '.native-prepared.json').read_bytes()) == binding


def test_broker_refuses_retired_node_artifact_even_with_matching_hashes(tmp_path, tls):
    data, state, run = paths(tmp_path)
    recorded_generation(data, tls)
    staged_store(data, state, run)
    raw = (data / 'node.env').read_bytes() + b'LAYERX_NODE_SNAPSHOT=/retired/snapshot\n'
    native.write_file(data / 'node.env', raw)
    native.write_file(data / '.generation-proof/node.env', raw)
    native.write_file(data / '.native-prepared.json',
                      canonical(native.generation_binding(data)))
    with broker(data, state, run) as socket_path:
        refused(socket_path, request())


def test_fixture_value_native_signer_file_contract_and_reopen(tmp_path):
    assert not (NODE / 'sequencer-env.sh').exists()
    harness = NativeKeyHarness(tmp_path, recorded=True)
    signatures = []
    pids = []
    try:
        binding = harness.bootstrap()
        store = staged_store(harness.data, harness.state, harness.run)
        args = native.parser().parse_args(harness.arguments('supervise'))
        digest = hashlib.sha256(b'native receipt after restart').digest()
        for _ in range(2):
            process = native.start_native(args, store, commands=harness.peer)
            try:
                pids.append(process.pid)
                with harness.consume() as (result, adopted, view):
                    assert result['version'] == 2
                    assert not set(adopted) & client.RETIRED_FIELDS
                    assert public_key_of(harness.receipt_seed).hex() == adopted['LAYERX_NODE_SEQUENCER_PUBLIC_KEY']
                    assert harness.receipt_seed.hex() not in json.dumps(adopted)
                    assert native.generation_binding(harness.data) == binding
                    for artifact in view.iterdir():
                        raw = artifact.read_bytes()
                        assert harness.receipt_seed not in raw
                        assert harness.receipt_seed.hex().encode() not in raw
                provider = FileProvider(harness.receipt_key)
                try:
                    signature = provider.sign(digest)
                    assert verify(public_key_of(harness.receipt_seed), digest, signature)
                    signatures.append(signature)
                finally:
                    provider.close()
            finally:
                native.stop_process(process)
            assert process.poll() == -signal.SIGTERM
            store = staged_store(harness.data, harness.state, harness.run)
        assert pids[0] != pids[1]
        assert signatures[0] == signatures[1]
        exchanges = [json.loads(line) for line in harness.peer.journal.read_text().splitlines()]
        starts = [row['command'] for row in exchanges
                  if 'command' in row and row['command'][1] == 'start']
        assert len(starts) == 2
        for command in starts:
            assert command[command.index('--sequencer.receipt-key-file') + 1] == str(harness.receipt_key)
            assert harness.receipt_seed.hex() not in ' '.join(command)
        for artifact in harness.data.rglob('*'):
            if artifact.is_file():
                assert harness.receipt_seed not in artifact.read_bytes()
                assert harness.receipt_seed.hex().encode() not in artifact.read_bytes()
        assert harness.receipt_key.stat().st_mode & 0o777 == 0o600
    finally:
        harness.close()
