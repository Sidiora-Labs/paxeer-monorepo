"""Legitimate protocol coverage source remains bound, outputs/secrets do not."""
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from dispatcher.source_policy import SOURCE_POLICY_VERSION, ignored
from dispatcher.evidence import source_digest, _paths, EvidenceError
from dispatcher.patches import build_patch, stage_patches, PatchError
from dispatcher.workspace import create_archive, create_workspace, WorkspaceError


class SourcePolicyTests(unittest.TestCase):
    def test_exact_protocol_source_exception_and_nested_outputs(self):
        for prefix in ('', 'sdk/launchpad/'):
            coverage = prefix + 'protocol/coverage'
            for path in (coverage, coverage + '/checkpoint.go', coverage + '/fixtures/canonical.json'):
                self.assertFalse(ignored(path), path)
            for path in ('coverage/index.html', 'client/coverage/index.html', coverage + '/coverage/index.html', coverage + '/dist/generated.go', coverage + '/node_modules/pkg/a.js', coverage + '/.env', coverage + '/secrets/key.json', coverage + '/coverage.out', 'other/' + coverage + '/check.go'):
                self.assertTrue(ignored(path), path)
        self.assertEqual(SOURCE_POLICY_VERSION, 'qualification-source-v1')

    def test_real_source_hash_archive_patch_and_declared_input(self):
        for prefix in ('', 'sdk/launchpad/'):
            with self.subTest(prefix=prefix), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                base = root / 'base'
                base.mkdir()
                (base / 'anchor.txt').write_text('baseline')
                candidate = root / 'candidate'
                candidate.mkdir()
                (candidate / 'anchor.txt').write_text('baseline')
                coverage = prefix + 'protocol/coverage'
                source = candidate / coverage
                source.mkdir(parents=True)
                (source / 'checkpoint.go').write_text('package coverage\n')
                (source / 'fixtures').mkdir()
                (source / 'fixtures/receipt.json').write_text('{"height":"125"}\n')
                before = source_digest(candidate)
                self.assertNotEqual(source_digest(base), before)
                inputs = [coverage + '/checkpoint.go', coverage + '/fixtures/receipt.json']
                declared = _paths(inputs, candidate)
                self.assertEqual(declared, {str(candidate / path): hashlib.sha256((candidate / path).read_bytes()).hexdigest() for path in inputs})
                (source / 'checkpoint.go').write_text('package coverage\n// changed\n')
                self.assertNotEqual(source_digest(candidate), before)
                (source / 'node_modules').symlink_to(root / 'missing-dependencies', target_is_directory=True)
                (source / '.env').symlink_to(root / 'missing-secret')
                (source / 'dist').mkdir()
                (source / 'dist/evil').symlink_to('/etc/passwd')
                (candidate / 'coverage').mkdir()
                (candidate / 'coverage/evil').symlink_to('/etc/passwd')
                bound = source_digest(candidate)
                excluded = [coverage + '/dist/generated.go', coverage + '/secrets/key.json', coverage + '/coverage/index.html', coverage + '/coverage.out']
                for relative in excluded:
                    path = candidate / relative
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text('excluded material')
                    with self.assertRaises(EvidenceError):
                        _paths([relative], candidate)
                self.assertEqual(source_digest(candidate), bound)
                archive = root / 'candidate.tar'
                archive_digest = create_archive(candidate, archive)
                self.assertEqual(archive_digest, hashlib.sha256(archive.read_bytes()).hexdigest())
                extracted = root / 'extract'
                self.assertEqual(create_workspace(archive, extracted), archive_digest)
                self.assertEqual(source_digest(extracted), bound)
                self.assertTrue((extracted / inputs[0]).exists())
                self.assertTrue((extracted / inputs[1]).exists())
                self.assertFalse((extracted / coverage / 'dist').exists())
                self.assertFalse((extracted / coverage / '.env').exists())
                for relative in excluded:
                    self.assertFalse((extracted / relative).exists())
                owned = [coverage + '/**']
                patch = build_patch(base, candidate, owned, root / 'patch')
                self.assertEqual({entry['path'] for entry in patch['entries']}, set(inputs))
                manifest = json.loads(Path(patch['manifest_path']).read_bytes())
                self.assertEqual(manifest['owned'], owned)
                staged = stage_patches(base, [root / 'patch'], root / 'stage', verify=lambda tree: source_digest(tree) == bound, ownership={patch['patch_digest']: owned})
                self.assertTrue(staged['qualified'])
                self.assertEqual(source_digest(root / 'stage'), bound)
                self.assertFalse((root / 'stage/coverage').exists())
                for relative in excluded:
                    self.assertFalse((root / 'stage' / relative).exists())

    def test_shared_secrets_go_only_source_boundary(self):
        for prefix in ('', 'sdk/launchpad/'):
            with self.subTest(prefix=prefix), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                base = root / 'base'
                base.mkdir()
                candidate = root / 'candidate'
                candidate.mkdir()
                secrets = prefix + 'shared/secrets'
                source = candidate / secrets
                source.mkdir(parents=True)
                (source / 'manager.go').write_text('package secrets\n')
                (source / 'manager_test.go').write_text('package secrets\n// tests\n')
                excluded = ('credentials.json', 'private.pem', '.env', 'key.txt', 'generated/secret.go')
                for name in excluded:
                    path = source / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.symlink_to(root / 'unread-secret-target')
                before = source_digest(candidate)
                inputs = [secrets + '/manager.go', secrets + '/manager_test.go']
                self.assertEqual(_paths(inputs, candidate), {str(candidate / path): hashlib.sha256((candidate / path).read_bytes()).hexdigest() for path in inputs})
                for name in excluded:
                    self.assertTrue(ignored(secrets + '/' + name))
                    with self.assertRaises(EvidenceError):
                        _paths([secrets + '/' + name], candidate)
                (source / 'manager.go').write_text('package secrets\n// actual change\n')
                bound = source_digest(candidate)
                self.assertNotEqual(before, bound)
                archive = root / 'source.tar'
                digest = create_archive(candidate, archive)
                extracted = root / 'extract'
                self.assertEqual(create_workspace(archive, extracted), digest)
                self.assertEqual({path.name for path in (extracted / secrets).iterdir()}, {'manager.go', 'manager_test.go'})
                self.assertEqual(source_digest(extracted), bound)
                owned = [secrets + '/**']
                patch = build_patch(base, candidate, owned, root / 'patch')
                self.assertEqual({entry['path'] for entry in patch['entries']}, set(inputs))
                result = stage_patches(base, [root / 'patch'], root / 'stage', verify=lambda tree: source_digest(tree) == bound, ownership={patch['patch_digest']: owned})
                self.assertTrue(result['qualified'])
                self.assertEqual(source_digest(root / 'stage'), bound)
                self.assertEqual({path.name for path in (root / 'stage' / secrets).iterdir()}, {'manager.go', 'manager_test.go'})
                (source / 'evil.go').symlink_to('/etc/passwd')
                with self.assertRaises(EvidenceError):
                    source_digest(candidate)
                with self.assertRaises(EvidenceError):
                    _paths([secrets + '/evil.go'], candidate)
                with self.assertRaises(WorkspaceError):
                    create_archive(candidate, root / 'unsafe.tar')
                with self.assertRaises(PatchError):
                    build_patch(base, candidate, owned, root / 'unsafe-patch')
                (source / 'evil.go').unlink()
                os.link(source / 'manager.go', source / 'alias.go')
                with self.assertRaises(EvidenceError):
                    source_digest(candidate)
                with self.assertRaises(EvidenceError):
                    _paths([secrets + '/alias.go'], candidate)
                with self.assertRaises(WorkspaceError):
                    create_archive(candidate, root / 'hardlink.tar')
                with self.assertRaises(PatchError):
                    build_patch(base, candidate, owned, root / 'hardlink-patch')
