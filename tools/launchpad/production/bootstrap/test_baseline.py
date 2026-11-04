#!/usr/bin/env python3
import hashlib
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('baseline', Path(__file__).with_name('baseline.py'))
baseline = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(baseline)

class BaselineControlFlow(unittest.TestCase):
    def test_missing_evidence_directory_fails_before_work(self):
        env = dict(os.environ)
        env.pop('QUALIFICATION_EVIDENCE_DIR', None)
        result = subprocess.run([sys.executable, baseline.__file__], env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn('QUALIFICATION_EVIDENCE_DIR is required', result.stderr)

    def test_real_pin_file_aliases_and_reject_multiple_versions(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / '.tool-versions'
            path.write_text('golang 1.26.4\nnodejs 24.16.0\npnpm 9.15.0\n')
            self.assertEqual(baseline.read_pins(path), {'go': '1.26.4', 'node': '24.16.0', 'pnpm': '9.15.0'})
            path.write_text('pnpm 9.15.0 11.5.2\n')
            with self.assertRaises(ValueError):
                baseline.read_pins(path)

    def test_source_manifest_excludes_secrets_generated_and_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / 'apps/launchpad/private-run'
            evidence.mkdir(parents=True)
            (evidence / 'baseline.json').write_text('evidence')
            (root / 'source.go').write_text('unrelated monorepo source')
            for home in ['apps/launchpad/web', 'services/launchpad/api']:
                base = root / home
                base.mkdir(parents=True)
                for name in ['source.go', '.env', '.env.local']:
                    (base / name).write_text('content')
                for name in ['node_modules', 'dist', 'evidence']:
                    (base / name).mkdir()
                    (base / name / 'generated').write_text('content')
                (base / 'link').symlink_to(base / 'source.go')
                (base / 'linked-directory').symlink_to(evidence, target_is_directory=True)
            selected = list(baseline.source_files(root, evidence))
            self.assertEqual([p.relative_to(root).as_posix() for p in selected], [
                'apps/launchpad/web/source.go', 'services/launchpad/api/source.go',
            ])
            for path in selected:
                self.assertEqual(baseline.sha(path), hashlib.sha256(b'content').hexdigest())

if __name__ == '__main__':
    unittest.main()
