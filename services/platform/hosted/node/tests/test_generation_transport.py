#!/usr/bin/env python3
import base64
import hashlib
import json
import os
from pathlib import Path
import socket
import stat
import sys
import tempfile
import threading
import unittest
from types import SimpleNamespace

import pytest

NODE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(NODE))
import generation_transport as transport
from reset_state import Store, StoreError, canonical, decode

# These fixtures publish native artifacts with the production generation producer.
from test_repair_22_31_355 import (
    ForkedProcess, broker, client, native, paths, recorded_generation, staged_store,
    tls, wait_until,
)

class GenerationTransportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.data = self.root / 'data'
        self.state = self.root / 'state'
        self.run = self.root / 'run'
        for directory in (self.data, self.state, self.run):
            directory.mkdir(mode=0o700)
        Store(str(self.state), str(self.data), str(self.run), 1, {}).initialize()
        self.broker = transport.Broker(str(self.data), str(self.state), str(self.run), os.geteuid(), os.getegid())
        self.capability = bytes(range(32))

    def tearDown(self):
        self.temporary.cleanup()

    def request(self, **updates):
        result = {'version': 2, 'operation': 'core', 'expected_generation': None}
        result.update(updates)
        return result

    def refusal(self, request, expected):
        with self.assertRaises(StoreError) as error:
            self.broker.request(request, (os.geteuid(), os.getegid()))
        self.assertEqual(error.exception.code, expected)

    def test_same_uid_cannot_read_other_slot(self):
        self.refusal(self.request(slot=2, capability=base64.b64encode(self.capability).decode()), 'invalid_request')

    def test_closed_requests_cannot_name_paths(self):
        self.refusal(self.request(path='../../secrets'), 'invalid_request')

    def test_capability_requires_exact_standard_encoding(self):
        self.refusal(self.request(capability=base64.b64encode(self.capability).decode() + '\n'), 'invalid_request')
        self.refusal(self.request(slot=True), 'invalid_request')

    def test_actual_peer_credentials_refuse_socket_request(self):
        self.broker.allowed_uid = (os.geteuid() + 1) % 0xffffffff
        client, server = socket.socketpair(socket.AF_UNIX, socket.SOCK_STREAM)
        worker = threading.Thread(target=self.broker.handle, args=(server,))
        worker.start()
        try:
            client.settimeout(5)
            client.sendall(canonical(self.request()) + b'\n')
            raw, ancillary, flags, _ = client.recvmsg(8192, socket.CMSG_SPACE(16 * 4))
            self.assertEqual(decode(raw), {'error': {'code': 'generation_unavailable'}})
            self.assertEqual(ancillary, [])
            self.assertEqual(flags & socket.MSG_CTRUNC, 0)
        finally:
            client.close()
            worker.join(timeout=5)
        self.assertFalse(worker.is_alive())

    def test_absent_canonical_generation_cannot_return_fds(self):
        with self.assertRaises(FileNotFoundError):
            self.broker.request(self.request(), (os.geteuid(), os.getegid()))

    def test_unsafe_capability_permissions_refuse(self):
        # No capability file is opened: the retired operation is refused first.
        self.refusal(self.request(operation='identity', slot=1,
                                  capability=base64.b64encode(self.capability).decode()), 'invalid_request')
        self.assertFalse((self.state / 'generation-authorizations').exists())

    def test_nofollow_file_and_directory_opens(self):
        real = self.data / 'real'
        real.write_bytes(b'bounded actual filesystem bytes')
        os.chmod(real, 0o600)
        (self.data / 'link').symlink_to(real)
        (self.data / 'linked-directory').symlink_to(self.state, target_is_directory=True)
        directory = transport.open_directory(str(self.data))
        try:
            with self.assertRaises(OSError):
                transport.artifact(directory, 'link')
            with self.assertRaises(OSError):
                transport.artifact(directory, 'linked-directory/state.json')
            with self.assertRaises(StoreError):
                transport.artifact(directory, '../state/state.json')
            descriptor = transport.artifact(directory, 'real', 64)
            try:
                self.assertEqual(transport.contents(descriptor, 64), real.read_bytes())
                with self.assertRaises(OSError):
                    os.write(descriptor, b'forbidden')
            finally:
                os.close(descriptor)
        finally:
            os.close(directory)

    def test_data_mode_and_socket_path_preserve_protected_boundary(self):
        os.chmod(self.data, 0o750)
        self.refusal(self.request(), 'unsafe_state_directory')
        os.chmod(self.data, 0o700)
        with self.assertRaises(StoreError) as error:
            self.broker.serve(str(self.root / 'unleased.sock'))
        self.assertEqual(error.exception.code, 'unsafe_socket_path')
        self.assertEqual(stat.S_IMODE(os.stat(self.data).st_mode), 0o700)


def test_native_core_generation_adoption_and_broker_reopen(tmp_path, tls):
    data, state, run = paths(tmp_path)
    binding, core = recorded_generation(data, tls)
    staged_store(data, state, run)
    output = tmp_path / 'adopted.json'
    command = [sys.executable, '-c', """import hashlib, json, os, pathlib, signal, sys
core = pathlib.Path(os.environ['LAYERX_CORE_ENV_FILE'])
ca = pathlib.Path(os.environ['LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER'])
result = {'pid': os.getpid(), 'core': hashlib.sha256(core.read_bytes()).hexdigest(),
          'ca': hashlib.sha256(ca.read_bytes()).hexdigest(),
          'generation': os.environ['LAYERX_GENERATION_NUMBER'],
          'rpc': os.environ['LAYERX_CORE_CHAIN_RPC_URL'],
          'signer': os.environ['LAYERX_CORE_TREASURY_SIGNER_SOCKET']}
path = pathlib.Path(sys.argv[1])
temporary = path.with_suffix('.pending')
temporary.write_text(json.dumps(result))
temporary.rename(path)
signal.pause()
""", str(output)]
    children = []
    for _ in range(2):
        output.unlink(missing_ok=True)
        with broker(data, state, run) as socket_path:
            args = SimpleNamespace(socket=str(socket_path), operation='core',
                                   broker_uid=os.geteuid(), broker_gid=os.getegid(), watch_seconds=0.1)
            process = ForkedProcess(lambda: client.supervise(args, command))
            try:
                wait_until(output.exists, process)
                adopted = json.loads(output.read_bytes())
                children.append(adopted['pid'])
                assert adopted['generation'] == '1'
                assert adopted['core'] == binding['core.env']
                assert adopted['ca'] == hashlib.sha256(tls[0].read_bytes()).hexdigest()
                assert adopted['rpc'] == core['LAYERX_CORE_CHAIN_RPC_URL']
                assert adopted['signer'] == core['LAYERX_CORE_TREASURY_SIGNER_SOCKET']
                assert native.generation_binding(data) == binding
                with socket.socket(socket.AF_UNIX) as connection:
                    connection.settimeout(5)
                    connection.connect(str(socket_path))
                    connection.sendall(canonical({'version': 2, 'operation': 'identity',
                                                   'expected_generation': 1}) + b'\n')
                    raw, ancillary, flags, _ = connection.recvmsg(8192, socket.CMSG_SPACE(64))
                    assert decode(raw) == {'error': {'code': 'generation_unavailable'}}
                    assert ancillary == [] and not flags & socket.MSG_CTRUNC
                with pytest.raises(StoreError) as error:
                    client.receive(str(socket_path), 'identity', 1, os.geteuid(), os.getegid())
                assert error.value.code == 'invalid_operation'
            finally:
                native.stop_process(process)
        wait_until(lambda: not Path('/proc', str(children[-1])).exists())
        reopened = staged_store(data, state, run)
        assert reopened.generation() == 1
    assert children[0] != children[1]


if __name__ == '__main__':
    unittest.main()
