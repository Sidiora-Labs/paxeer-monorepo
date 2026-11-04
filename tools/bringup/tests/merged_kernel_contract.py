"""Merged kernel qualification against the production owner and recorded RPC.

The peer only replays native_configuration.json; the production runtime reads,
validates and atomically recovers its own committed checkpoint unchanged.
"""
import copy
from datetime import datetime
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import unittest

ROOT = Path(__file__).resolve().parents[3]
NODE = ROOT / 'services/platform/hosted/node'
FIXTURE = NODE / 'tests/fixtures/repair_22_31_355'


def runtime_module():
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(NODE))
    try:
        spec = importlib.util.spec_from_file_location('merged_kernel_owner', ROOT / 'docker/kernel/kernel_runtime.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module
    finally:
        sys.path.remove(str(NODE))


class RoleDirectoryEvidenceRead(unittest.TestCase):
    """Receipt signing custody replaces the independent authority bearer.

    Preserve distinct identity, least privilege, source protection, restart
    preservation and redirect refusals at their actual merged owner.
    """
    @classmethod
    def setUpClass(cls):
        cls.runtime = runtime_module()
        cls.capture = json.loads((FIXTURE / 'native_configuration.json').read_text())
        cls.commit_time = datetime.fromisoformat(
            cls.capture['status']['sync_info']['latest_block_time'].replace('Z', '+00:00')).timestamp()
        cls.genesis = hashlib.sha256((FIXTURE / 'genesis.json').read_bytes()).hexdigest()

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='merged-kernel-contract-')
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.directory.chmod(0o700)
        self.checkpoint = self.directory / 'committed.json'
        self.records = copy.deepcopy(self.capture['rpc_exchanges'])
        self.unavailable = False
        self.requests = []
        owner = self

        class Peer(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                owner.requests.append(request)
                matches = [record for record in owner.records
                           if record['method'] == request['method'] and record['params'] == request['params']]
                if self.path != '/' or owner.unavailable or len(matches) != 1:
                    self.send_error(503)
                    return
                raw = json.dumps(dict(jsonrpc='2.0', id=request['id'], result=matches[0]['result'])).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, *_):
                pass

        self.peer = ThreadingHTTPServer(('127.0.0.1', 0), Peer)
        self.thread = threading.Thread(target=self.peer.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_peer)
        origin = 'http://127.0.0.1:' + str(self.peer.server_port)
        self.node = dict(self.runtime.NODE, LAYERX_NODE_RPC_URL=origin, LAYERX_NODE_CONSENSUS_RPC_URL=origin)

    def stop_peer(self):
        self.peer.shutdown()
        self.peer.server_close()
        self.thread.join(timeout=5)
        self.assertFalse(self.thread.is_alive())

    def reopen(self, genesis=None):
        return self.runtime.CommittedReadiness(
            self.node, self.checkpoint, genesis or self.genesis, os.getpid(),
            clock=lambda: self.commit_time + 1)

    def test_01_signing_material_installed_and_isolated(self):
        first, second = self.directory / 'receipt.key', self.directory / 'peer.key'
        for path in (first, second):
            self.runtime.native.write_file(path, os.urandom(32).hex().encode())
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(path.stat().st_uid, os.geteuid())
        self.assertNotEqual(self.runtime.private_file(first), self.runtime.private_file(second))
        self.assertEqual(self.runtime.private_directory(self.directory), self.directory)
        # Access as an unrelated real UID must fail, even in the same namespace.
        if os.geteuid() != 0:
            raise RuntimeError('cross-uid qualification requires real root')
        pid = os.fork()
        if pid == 0:
            try:
                os.setgroups([])
                os.setgid(4020)
                os.setuid(4021)
                try:
                    first.read_bytes()
                except PermissionError:
                    os._exit(0)
                os._exit(1)
            except BaseException:
                os._exit(2)
        self.assertEqual(os.waitpid(pid, 0)[1], 0, 'another role read the receipt seed')

    def test_02_committed_evidence_uses_only_node_routes(self):
        readiness = self.reopen()
        code, result = readiness.probe()
        self.assertEqual(code, 200)
        self.assertTrue(result['ready'])
        self.assertGreater(result['height'], 0)
        self.assertEqual({r['method'] for r in self.requests},
                         {'status', 'commit', 'eth_chainId', 'eth_getBlockByNumber'})
        self.assertTrue(all(r['params'] in ({}, [], {'height': str(result['height'])},
                                           [hex(result['height']), False]) for r in self.requests))
        server = self.runtime.readiness_server(readiness, ('127.0.0.1', 0))
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            import http.client
            connection = http.client.HTTPConnection(*server.server_address, timeout=5)
            connection.request('GET', '/readyz')
            response = connection.getresponse()
            self.assertEqual(response.status, 200)
            self.assertEqual(response.getheader('Cache-Control'), 'no-store')
            self.assertEqual(json.loads(response.read()), result)
            connection.close()
        finally:
            server.shutdown()
            thread.join(timeout=5)
            server.server_close()

    def test_03_foreign_identity_and_noncanonical_commit_refused(self):
        self.assertEqual(self.reopen().probe()[0], 200)
        with self.assertRaises(ValueError):
            self.reopen('a' * 64)
        for record in self.records:
            if record['method'] == 'commit':
                record['result']['canonical'] = False
        self.assertEqual(self.reopen().probe(), (503, {'ready': False, 'chain_id': 125}))

    def test_04_unprotected_source_is_never_installed(self):
        source = self.directory / 'receipt.key'
        source.write_bytes(b'protected seed')
        source.chmod(0o600)
        original = source.read_bytes()
        for variant in ('mode', 'owner', 'symlink', 'hardlink', 'directory'):
            with self.subTest(variant=variant):
                target = self.directory / ('bad-' + variant)
                if variant == 'symlink':
                    target.symlink_to(source)
                elif variant == 'hardlink':
                    os.link(source, target)
                elif variant == 'directory':
                    target.mkdir(mode=0o700)
                else:
                    target.write_bytes(original)
                    target.chmod(0o640 if variant == 'mode' else 0o600)
                    if variant == 'owner':
                        if os.geteuid() != 0:
                            raise RuntimeError('owner-isolation qualification requires root')
                        os.chown(target, 4021, 4020)
                try:
                    with self.assertRaises((OSError, ValueError)):
                        self.runtime.private_file(target)
                finally:
                    target.rmdir() if target.is_dir() and not target.is_symlink() else target.unlink()
                self.assertEqual(source.read_bytes(), original)
        self.assertFalse(self.checkpoint.exists())

    def test_05_unprotected_retained_evidence_refuses_startup(self):
        self.assertEqual(self.reopen().probe()[0], 200)
        original = self.checkpoint.read_bytes()
        self.checkpoint.chmod(0o640)
        with self.assertRaises(ValueError):
            self.reopen()
        self.checkpoint.chmod(0o600)
        saved = self.directory / 'saved.json'
        self.checkpoint.rename(saved)
        self.checkpoint.symlink_to(saved)
        with self.assertRaises((OSError, ValueError)):
            self.reopen()
        self.assertEqual(saved.read_bytes(), original)

    def test_06_restart_preserves_durable_identity_and_requires_live_rpc(self):
        first = self.reopen()
        expected = first.probe()
        self.assertEqual(expected[0], 200)
        before = self.checkpoint.read_bytes()
        del first
        self.unavailable = True
        second = self.reopen()
        self.assertEqual(second.probe(), (503, {'ready': False, 'chain_id': 125}))
        self.assertEqual(self.checkpoint.read_bytes(), before)
        self.unavailable = False
        self.assertEqual(second.probe(), expected)
        self.assertEqual(self.checkpoint.read_bytes(), before)
        second.start = str(int(second.start) + 1)
        self.assertEqual(second.probe()[0], 503, 'PID reuse established readiness')

    def test_07_restart_refuses_redirected_runtime_directory(self):
        self.assertEqual(self.reopen().probe()[0], 200)
        original = self.checkpoint.read_bytes()
        redirected = self.directory / 'redirected'
        redirected.symlink_to(self.directory, target_is_directory=True)
        with self.assertRaises(ValueError):
            self.runtime.CommittedReadiness(self.node, redirected / 'committed.json', self.genesis, os.getpid())
        self.assertEqual(self.checkpoint.read_bytes(), original)
