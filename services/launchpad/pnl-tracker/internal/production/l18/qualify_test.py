"""Actual-process lifecycle checks; no simulated provider success."""
import importlib.util
import os
import pathlib
import socket
import subprocess
import sys
import tempfile
import time
import unittest

spec = importlib.util.spec_from_file_location('l18_qualify', pathlib.Path(__file__).with_name('qualify.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class Lifecycle(unittest.TestCase):
    def setUp(self):
        module.PORT = 21802

    def child(self, code):
        child = subprocess.Popen([sys.executable, '-c', code], start_new_session=True)
        self.addCleanup(module.stop, child)
        return child

    def test_occupied_port_preserves_peer(self):
        with socket.socket() as peer:
            peer.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            peer.bind(('127.0.0.1', 21802))
            peer.listen()
            with self.assertRaisesRegex(RuntimeError, 'occupied'):
                module.refuse_occupied_port()
            with socket.create_connection(('127.0.0.1', 21802)):
                accepted, _ = peer.accept()
                accepted.close()

    def test_dead_child_never_accepts_peer_readiness(self):
        with socket.socket() as peer:
            peer.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            peer.bind(('127.0.0.1', 21802))
            peer.listen()
            child = self.child('pass')
            child.wait(timeout=5)
            with self.assertRaisesRegex(RuntimeError, 'exited'):
                module.wait_ready(child, attempts=1)

    def test_exhaustion_fails_and_cleanup_is_bounded(self):
        child = self.child('import time; time.sleep(30)')
        with self.assertRaisesRegex(RuntimeError, 'deadline exhausted'):
            module.wait_ready(child, attempts=2)
        started = time.monotonic()
        module.stop(child)
        self.assertIsNotNone(child.poll())
        self.assertLess(time.monotonic() - started, 12)

    def test_live_child_cannot_borrow_peer_listener(self):
        with socket.socket() as peer:
            peer.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            peer.bind(('127.0.0.1', 21802))
            peer.listen()
            child = self.child('import time; time.sleep(30)')
            with self.assertRaisesRegex(RuntimeError, 'not alive and listening'):
                module.assert_owned(child)

    def test_launcher_signal_unwinds_child_group_cleanup(self):
        with tempfile.TemporaryDirectory(prefix='l18-signal-') as private:
            pidfile = pathlib.Path(private) / 'child.pid'
            child_code = ("import os,socket,time,pathlib; "
                          "s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('127.0.0.1',21802));s.listen(); "
                          f"pathlib.Path({str(pidfile)!r}).write_text(str(os.getpid())); "
                          "time.sleep(30)")
            launcher_code = ("import importlib.util,subprocess,sys; "
                             f"s=importlib.util.spec_from_file_location('q',{module.__file__!r}); "
                             "q=importlib.util.module_from_spec(s);s.loader.exec_module(q); "
                             "q.install_signal_handlers(); "
                             f"q.run_bounded([sys.executable,'-c',{child_code!r}],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)")
            launcher = self.child(launcher_code)
            deadline = time.monotonic() + 5
            while not pidfile.exists() and time.monotonic() < deadline:
                time.sleep(.02)
            self.assertTrue(pidfile.exists(), 'owned nested child failed to start')
            child_pid = int(pidfile.read_text())
            launcher.terminate()
            launcher.wait(timeout=12)
            self.assertEqual(launcher.returncode, 143)
            self.assertFalse(pathlib.Path(f'/proc/{child_pid}').exists(), 'nested child survived launcher SIGTERM')
            module.refuse_occupied_port()

    def test_existing_evidence_is_not_reused_or_truncated(self):
        with tempfile.TemporaryDirectory(prefix='l18-evidence-') as private:
            old = pathlib.Path(private) / 'repeat-go.jsonl'
            old.write_text('previous failed result\n')
            result = subprocess.run([sys.executable, str(pathlib.Path(module.__file__)),
                                     '--run', 'repeat', '--output-dir', private],
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10,
                                    env={**os.environ, 'L18_POSTGRES_DSN': 'postgresql://localhost/disposable'})
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'historical output will not be reused', result.stderr)
            self.assertEqual(old.read_text(), 'previous failed result\n')
            self.assertEqual(list(pathlib.Path(private).iterdir()), [old])


class InputRequirements(unittest.TestCase):
    def test_database_input_is_required_before_evidence_is_written(self):
        with tempfile.TemporaryDirectory() as private:
            output = pathlib.Path(private) / 'evidence'
            for environ in ({}, {'L18_POSTGRES_DSN': ''}, {'L18_POSTGRES_DSN': '  '},
                            {'L18_POSTGRES_DSN': 'postgresql://localhost/'},
                            {'L18_POSTGRES_DSN': 'https://localhost/disposable'}):
                with self.subTest(environ=environ):
                    with self.assertRaisesRegex(RuntimeError, 'L18_POSTGRES_DSN'):
                        module.qualification_inputs(output, environ)
                    self.assertFalse(output.exists())

    def test_output_input_is_explicit_and_private(self):
        environ = {'L18_POSTGRES_DSN': 'postgresql://localhost/disposable'}
        with self.assertRaisesRegex(RuntimeError, 'explicit private --output-dir'):
            module.qualification_inputs(None, environ)
        with self.assertRaisesRegex(RuntimeError, 'absolute private'):
            module.qualification_inputs(pathlib.Path('relative-evidence'), environ)
        with tempfile.TemporaryDirectory() as private:
            output = pathlib.Path(private) / 'evidence'
            output.mkdir(mode=0o755)
            with self.assertRaisesRegex(RuntimeError, 'private --output-dir'):
                module.qualification_inputs(output, environ)
            link = pathlib.Path(private) / 'link'
            link.symlink_to(output, target_is_directory=True)
            with self.assertRaisesRegex(RuntimeError, 'without symlinks'):
                module.qualification_inputs(link, environ)
            self.assertEqual(list(output.iterdir()), [])

    def test_operator_database_is_preserved_and_new_directory_is_private(self):
        dsn = 'postgresql://operator@localhost:6543/disposable?sslmode=require'
        with tempfile.TemporaryDirectory() as private:
            output = pathlib.Path(private) / 'evidence'
            evidence, admitted = module.qualification_inputs(output, {'L18_POSTGRES_DSN': dsn})
            self.assertEqual(admitted, dsn)
            self.assertEqual(evidence, output)
            self.assertEqual(output.stat().st_mode & 0o777, 0o700)
            self.assertEqual(list(output.iterdir()), [])


if __name__ == '__main__':
    unittest.main(verbosity=2)
