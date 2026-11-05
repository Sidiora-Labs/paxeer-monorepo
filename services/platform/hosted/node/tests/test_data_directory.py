import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import unittest

import pytest
from test_repair_22_31_355 import (
    broker, client, directory as generation_directory, native, paths, prepared,
    receive, staged_store, tls, traced_activation, reset as native_reset,
)

SCRIPT = Path(__file__).resolve().parents[1] / 'data_directory.py'


class DataDirectoryTests(unittest.TestCase):
    def run_directory(self, operation, path):
        return subprocess.run([sys.executable, str(SCRIPT), operation, str(path)],
                              capture_output=True, text=True, check=False)

    def test_force_refuses_directory_and_parent_symlinks(self):
        with tempfile.TemporaryDirectory(prefix='node-data-directory-') as directory:
            root = Path(directory)
            target = root / 'target'
            target.mkdir()
            sentinel = target / 'retained'
            sentinel.write_text('retained')
            link = root / 'link'
            link.symlink_to(target, target_is_directory=True)
            for path in (link, link / 'child'):
                for operation in ('prepare', 'clear'):
                    result = self.run_directory(operation, path)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(sentinel.read_text(), 'retained')

    def test_force_removes_owned_contents_without_following_child_symlink(self):
        with tempfile.TemporaryDirectory(prefix='node-data-directory-') as directory:
            root = Path(directory)
            target = root / 'target'
            target.mkdir()
            sentinel = target / 'retained'
            sentinel.write_text('retained')
            data = root / 'data'
            self.assertEqual(self.run_directory('prepare', data).returncode, 0)
            (data / 'link').symlink_to(target, target_is_directory=True)
            (data / 'nested').mkdir()
            (data / 'nested' / 'file').write_text('discard')
            self.assertEqual(self.run_directory('clear', data).returncode, 0)
            self.assertEqual(list(data.iterdir()), [])
            self.assertEqual(sentinel.read_text(), 'retained')
            self.assertEqual(data.stat().st_mode & 0o777, 0o700)

    def test_stage_retains_live_data_and_is_idempotent(self):
        with tempfile.TemporaryDirectory(prefix='node-generation-stage-') as directory:
            root = Path(directory)
            data = root / 'data'
            state = root / 'state'
            data.mkdir(mode=0o700)
            state.mkdir(mode=0o700)
            sentinel = data / 'retained-economic-generation'
            sentinel.write_text('retained')
            identity = 'a' * 32
            command = [sys.executable, str(SCRIPT), 'prepare-stage',
                       str(data), str(state), identity]
            first = subprocess.run(command, capture_output=True, text=True, check=False)
            self.assertEqual(first.returncode, 0, first.stderr)
            stage = state / 'generations' / identity
            self.assertEqual(first.stdout.strip(), str(stage))
            for path in (stage.parent, stage, stage / 'data', stage / 'run'):
                self.assertEqual(path.stat().st_mode & 0o777, 0o700)
            (stage / 'run').chmod(0o750)
            second = subprocess.run(command, capture_output=True, text=True, check=False)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(first.stdout, second.stdout)
            self.assertEqual(sentinel.read_text(), 'retained')
            self.assertEqual(list((stage / 'data').iterdir()), [])

    def test_stage_refuses_aliases_overlap_and_invalid_identity(self):
        with tempfile.TemporaryDirectory(prefix='node-generation-refusal-') as directory:
            root = Path(directory)
            data = root / 'data'
            state = root / 'state'
            data.mkdir(mode=0o700)
            state.mkdir(mode=0o700)
            sentinel = data / 'retained-economic-generation'
            sentinel.write_text('retained')
            alias = root / 'alias'
            alias.symlink_to(state, target_is_directory=True)
            for state_path, identity in ((alias, 'b' * 32), (data, 'b' * 32),
                                         (state, '../escape'), (state, 'B' * 32)):
                result = subprocess.run([sys.executable, str(SCRIPT), 'prepare-stage',
                                         str(data), str(state_path), identity],
                                        capture_output=True, text=True, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(sentinel.read_text(), 'retained')
            generations = state / 'generations'
            generations.symlink_to(data, target_is_directory=True)
            result = subprocess.run([sys.executable, str(SCRIPT), 'prepare-stage',
                                     str(data), str(state), 'b' * 32],
                                    capture_output=True, text=True, check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(sentinel.read_text(), 'retained')

    def test_activation_without_canonical_producer_refuses_before_effect(self):
        with tempfile.TemporaryDirectory(prefix='node-generation-activation-refusal-') as directory:
            root = Path(directory)
            data = root / 'data'
            staged = root / 'staged'
            data.mkdir(mode=0o700)
            staged.mkdir(mode=0o700)
            sentinel = data / 'retained-economic-generation'
            sentinel.write_text('retained')
            for operation in ('seal-generation', 'activate-generation'):
                result = subprocess.run([sys.executable, str(SCRIPT), operation,
                                         str(data), str(staged), 'c' * 32, '1', '{}'],
                                        capture_output=True, text=True, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(sentinel.read_text(), 'retained')
                self.assertEqual(list(staged.iterdir()), [])

    def test_rollback_removes_only_unsealed_staging(self):
        with tempfile.TemporaryDirectory(prefix='node-generation-rollback-') as directory:
            root = Path(directory)
            data = root / 'data'
            state = root / 'state'
            data.mkdir(mode=0o700)
            state.mkdir(mode=0o700)
            sentinel = data / 'retained-economic-generation'
            sentinel.write_text('retained')
            identity = 'd' * 32
            prepare = subprocess.run([sys.executable, str(SCRIPT), 'prepare-stage',
                                      str(data), str(state), identity],
                                     capture_output=True, text=True, check=False)
            self.assertEqual(prepare.returncode, 0, prepare.stderr)
            stage = state / 'generations' / identity
            (stage / 'data' / 'incomplete-producer-output').write_text('incomplete')
            rollback = subprocess.run([sys.executable, str(SCRIPT), 'rollback-stage',
                                       str(data), str(state), identity],
                                      capture_output=True, text=True, check=False)
            self.assertEqual(rollback.returncode, 0, rollback.stderr)
            self.assertEqual(rollback.stdout.strip(), 'rolled_back')
            self.assertFalse(stage.exists())
            self.assertEqual(sentinel.read_text(), 'retained')




@pytest.mark.parametrize('failure', [None, ('kill', 0), ('kill', 1)])
def test_native_activation_exchange_and_post_rename_reopen(tmp_path, tls, failure):
    data, state, run = paths(tmp_path)
    store, stage, binding, _ = prepared(data, state, run, tls)
    sentinel = data / 'retained-economic-generation'
    sentinel.write_bytes(b'previous selected native generation')
    original_inode = data.stat().st_ino
    staged_inode = stage.stat().st_ino
    code, synced, injected = traced_activation(data, stage, binding, failure)
    assert code == (0 if failure is None else -signal.SIGKILL)
    assert injected is (failure is not None)
    assert data.stat().st_ino == staged_inode
    assert stage.stat().st_ino == original_inode
    assert (stage / sentinel.name).read_bytes() == b'previous selected native generation'
    assert not sentinel.exists()
    assert synced[:1] == [str(data.parent)]
    assert native.generation_binding(data) == binding
    if failure is not None:
        assert not (data / generation_directory.ACTIVATION_MARKER).exists()
        with broker(data, state, run) as socket_path:
            with pytest.raises(native_reset.StoreError):
                receive(socket_path, 2)
    # A new interpreter reopens production code with no inherited directory handles.
    command = [sys.executable, str(SCRIPT), 'activate-generation', str(data), str(stage),
               'a' * 32, '2', json.dumps(binding)]
    reopened = subprocess.run(command, capture_output=True, text=True, timeout=10)
    assert reopened.returncode == 0, reopened.stderr
    assert json.loads(reopened.stdout) == 'active'
    marker = json.loads((data / generation_directory.GENERATION_MARKER).read_bytes())
    assert json.loads((data / generation_directory.ACTIVATION_MARKER).read_bytes()) == marker
    assert marker['genesis'] == binding and marker['generation'] == 2
    assert data.stat().st_ino == staged_inode
    assert (stage / sentinel.name).read_bytes() == b'previous selected native generation'
    retained = {name: (data / name).read_bytes() for name in binding}
    with broker(data, state, run) as socket_path:
        result, descriptors = receive(socket_path, 2)
        try:
            adopted = client.child_environment(result, descriptors, 'core')
            view, adopted = client.materialize(result, descriptors, 'core', adopted)
            try:
                assert result['reset_id'] == 'a' * 32
                assert adopted['LAYERX_GENERATION_NUMBER'] == '2'
                for name, raw in retained.items():
                    assert (Path(view.name) / name.rsplit('/', 1)[-1]).read_bytes() == raw
            finally:
                view.cleanup()
        finally:
            client.close_descriptors(descriptors)
    assert staged_store(data, state, run).active()['phase'] == 'activating'
    assert native.generation_binding(data) == binding


if __name__ == '__main__':
    unittest.main()
