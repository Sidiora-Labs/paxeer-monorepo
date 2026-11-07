#!/usr/bin/env python3
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest

from native_key_harness import (
    NativeKeyHarness, generation, native, public_key_of, reset, transport, write_seed,
)
from provider import FileProvider, ProviderError


def environment_lines(path):
    return transport.environment(path.read_bytes())


class SequencerSeedTest(unittest.TestCase):
    recorded_native_peer = False

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='native-seed-')
        self.addCleanup(self.directory.cleanup)
        self.work = Path(self.directory.name)
        self.harness = NativeKeyHarness(self.work, recorded=self.recorded_native_peer)
        self.addCleanup(self.harness.close)
        self.sequencer_seed = self.harness.receipt_seed
        self.sequencer_path = self.harness.receipt_key
        self.data = self.harness.data
        self.run_dir = self.harness.run

    def bootstrap(self, sequencer_key=None):
        return self.harness.bootstrap(receipt_key=sequencer_key)

    def reopen_bootstrap(self):
        target = self.work / 'reopened-data'
        target.mkdir(mode=0o700)
        args = native.parser().parse_args(self.harness.arguments())
        return native.bootstrap(args, data=target, commands=self.harness.peer)

    def assert_seed_only_in_its_file(self):
        self.harness.assert_no_copy(self, self.sequencer_seed, self.sequencer_path)

    def test_bootstrap_records_the_key_file_and_keeps_no_seed_copy(self):
        binding = self.bootstrap()
        exports = environment_lines(self.data / 'node.env')
        self.assertNotIn('LAYERX_NODE_SEQUENCER_PRIVATE_KEY', exports)
        self.assertNotIn('LAYERX_NODE_SEQUENCER_KEY_FILE', exports)
        self.assertEqual(exports['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'],
                         public_key_of(self.sequencer_seed).hex())
        self.assertEqual(exports['LAYERX_NODE_SEQUENCER_ID'],
                         hashlib.sha256(public_key_of(self.sequencer_seed)).hexdigest())
        self.assertEqual((self.data / 'node.env').stat().st_mode & 0o777, 0o600)
        self.assertEqual(json.loads((self.data / '.native-prepared.json').read_bytes()), binding)
        args = native.parser().parse_args(self.harness.arguments('supervise'))
        self.assertEqual(args.receipt_key_file, str(self.sequencer_path))
        for retired in ('sequencer.env', 'replica.env', 'work'):
            self.assertFalse((self.data / retired).exists())
        self.harness.assert_public_handoff(self)
        self.assert_seed_only_in_its_file()

    def test_bootstrap_resolves_a_relative_key_path_against_the_working_directory(self):
        relative = Path(os.path.relpath(self.sequencer_path, Path.cwd()))
        self.bootstrap(sequencer_key=relative)
        self.assertEqual(environment_lines(self.data / 'node.env')['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'],
                         public_key_of(self.sequencer_seed).hex())
        self.harness.assert_public_handoff(self)
        self.assert_seed_only_in_its_file()

    def test_bootstrap_refuses_a_raw_seed_through_an_external_symlink(self):
        self.sequencer_path.write_bytes(self.sequencer_seed)
        link = self.work / 'receipt-link.key'
        link.symlink_to(self.sequencer_path)
        with self.assertRaises(OSError):
            self.bootstrap(sequencer_key=link)
        self.assertEqual(self.sequencer_path.read_bytes(), self.sequencer_seed)
        self.assertFalse((self.data / '.native-prepared.json').exists())
        self.assertFalse(list(self.data.iterdir()))

    def test_bootstrap_refuses_a_hex_seed_through_an_external_symlink(self):
        link = self.work / 'receipt-link.key'
        link.symlink_to(self.sequencer_path)
        with self.assertRaises(OSError):
            self.bootstrap(sequencer_key=link)
        self.assertEqual(self.sequencer_path.read_bytes(), self.sequencer_seed.hex().encode() + b'\n')
        self.assertFalse(list(self.data.iterdir()))

    def test_bootstrap_refuses_a_key_file_inside_the_data_directory(self):
        inside = self.data / 'receipt.key'
        write_seed(inside, self.sequencer_seed.hex().encode())
        with self.assertRaisesRegex(ValueError, 'requires an empty generation'):
            self.bootstrap(sequencer_key=inside)
        self.assertTrue(inside.exists(), 'native bootstrap must refuse before discarding a key')
        self.assertEqual(inside.read_bytes(), self.sequencer_seed.hex().encode())
        self.assertFalse((self.data / '.native-prepared.json').exists())

    def test_bootstrap_refuses_a_key_file_linked_into_the_data_directory(self):
        inside = self.data / 'linked.key'
        write_seed(inside, self.sequencer_seed.hex().encode())
        link = self.work / 'receipt-link.key'
        link.symlink_to(inside)
        with self.assertRaisesRegex(ValueError, 'requires an empty generation'):
            self.bootstrap(sequencer_key=link)
        self.assertEqual(inside.read_bytes(), self.sequencer_seed.hex().encode())
        self.assertFalse((self.data / '.native-prepared.json').exists())

    def test_bootstrap_refuses_an_unreadable_key_file(self):
        with self.assertRaises(FileNotFoundError):
            self.bootstrap(sequencer_key=self.work / 'absent.key')
        self.assertFalse(list(self.data.iterdir()))

    def test_bootstrap_refuses_a_treasury_key_that_is_not_a_regular_file(self):
        with self.assertRaisesRegex(ProviderError, 'regular file'):
            self.harness.signer(key=self.work)
        self.assertFalse(list(self.data.iterdir()))
        self.assertFalse((self.run_dir / 'treasury.sock').exists())

    def test_supervisor_refuses_a_seed_line_in_core_env(self):
        self.bootstrap()
        for parent in (self.data, self.data / '.generation-proof'):
            path = parent / 'core.env'
            with path.open('ab') as handle:
                handle.write(b'LAYERX_NODE_SEQUENCER_PRIVATE_KEY=' +
                             self.sequencer_seed.hex().encode() + b'\n')
        native.write_file(self.data / '.native-prepared.json',
                          reset.canonical(native.generation_binding(self.data)))
        with self.assertRaises(reset.StoreError) as failure:
            with self.harness.consume():
                self.fail('the real consumer accepted a seed in its environment')
        self.assertEqual(failure.exception.code, 'unsafe_environment')

    def test_supervisor_refuses_a_key_file_that_does_not_match_the_bound_public_key(self):
        self.bootstrap()
        self.sequencer_path.write_bytes(os.urandom(32).hex().encode())
        with self.assertRaisesRegex(ValueError, 'receipt signer is not pinned in genesis'):
            self.reopen_bootstrap()
        self.assertFalse((self.work / 'reopened-data/.native-prepared.json').exists())
        self.assertEqual(environment_lines(self.data / 'node.env')['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'],
                         public_key_of(self.sequencer_seed).hex())

    def test_supervisor_refuses_a_missing_key_file(self):
        self.bootstrap()
        self.sequencer_path.unlink()
        with self.assertRaises(FileNotFoundError):
            self.reopen_bootstrap()
        self.assertFalse((self.work / 'reopened-data/.native-prepared.json').exists())
        self.assertTrue((self.data / '.native-prepared.json').is_file())

    def test_supervisor_refuses_a_malformed_key_file(self):
        self.bootstrap()
        self.sequencer_path.write_bytes(b'not a seed\n')
        with self.assertRaisesRegex(ValueError, 'paxd hexadecimal seed file contract'):
            self.reopen_bootstrap()
        self.assertFalse((self.work / 'reopened-data/.native-prepared.json').exists())
        self.assertTrue((self.data / '.native-prepared.json').is_file())

    def test_supervisor_binds_the_hex_form_of_the_same_seed_before_publishing(self):
        self.bootstrap()
        original = native.generation_binding(self.data)
        from test_repair_22_31_355 import native_supervisor

        pids, observed_commands = [], []
        for index in range(2):
            with native_supervisor(self.harness.config, self.data, self.harness.state,
                                   self.run_dir, self.work / ('supervisor-' + str(index) + '.log')):
                identity = json.loads((self.run_dir / 'native-process.json').read_bytes())
                pids.append(identity['pid'])
                if self.harness.peer is None:
                    observed_commands.append(Path('/proc', str(identity['pid']), 'cmdline')
                                             .read_bytes().rstrip(b'\0').decode().split('\0'))
                result, descriptors = generation.receive(str(self.run_dir / 'generation.sock'),
                                                         'core', 1, os.geteuid(), os.getegid())
                try:
                    adopted = generation.child_environment(result, descriptors, 'core')
                    self.assertEqual(adopted['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'],
                                     public_key_of(self.sequencer_seed).hex())
                    self.assertEqual(adopted['LAYERX_GENERATION_NUMBER'], '1')
                    self.assertNotIn('LAYERX_NODE_SEQUENCER_PRIVATE_KEY', adopted)
                finally:
                    generation.close_descriptors(descriptors)
                self.assertEqual(native.generation_binding(self.data), original)
                provider = FileProvider(self.sequencer_path)
                try:
                    self.assertEqual(provider.public_key(), public_key_of(self.sequencer_seed))
                finally:
                    provider.close()
            self.assertEqual(native.generation_binding(self.data), original)
        self.assertNotEqual(*pids)
        commands = observed_commands if self.harness.peer is None else [
            json.loads(line)['command'] for line in self.harness.peer.journal.read_text().splitlines()
            if 'command' in json.loads(line)]
        starts = [command for command in commands if command[1] == 'start']
        self.assertEqual(len(starts), 2)
        for command in starts:
            self.assertEqual(command[0:2], [self.harness.config['paxd'], 'start'])
            self.assertEqual(command[command.index('--sequencer.enable') + 1], 'true')
            self.assertEqual(command[command.index('--sequencer.receipt-key-file') + 1],
                             str(self.sequencer_path))
            self.assertNotIn(self.sequencer_seed.hex(), ' '.join(command))
        self.assert_seed_only_in_its_file()

    def test_bootstrap_refuses_invalid_receipt_encodings_and_unsafe_permissions(self):
        for index, value in enumerate((self.sequencer_seed, self.sequencer_seed.hex().upper().encode(),
                                       b'0' * 63, b'g' * 64, b'0' * 4097)):
            path = self.work / ('invalid-' + str(index) + '.key')
            write_seed(path, value)
            with self.subTest(encoding=index), self.assertRaises(ValueError):
                self.bootstrap(sequencer_key=path)
            self.assertFalse(list(self.data.iterdir()))
        self.sequencer_path.chmod(0o644)
        with self.assertRaises(ProviderError):
            self.bootstrap()
        self.assertFalse((self.data / '.native-prepared.json').exists())


if __name__ == '__main__':
    unittest.main()
