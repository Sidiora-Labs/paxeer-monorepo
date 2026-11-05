#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'human_deposit_initial_response.py',
    'gas-station-contract.py',
    'human_program_approvals.py',
    'interop-archive-contract.py',
)


class ServiceCatalogueReaders2Test(unittest.TestCase):
    def test_producers_load_existing_catalogue(self):
        for filename in PRODUCERS:
            with self.subTest(producer=filename):
                path = ROOT / 'tools/qualification/paxeer-x' / filename
                spec = importlib.util.spec_from_file_location(path.stem.replace('-', '_'), path)
                self.assertIsNotNone(spec)
                self.assertIsNotNone(spec.loader)
                module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(module)
                for name, value in vars(module).items():
                    if name.isupper():
                        self.assertNotIn('spec/paxeer-x', str(value), name)
                for number, line in enumerate(path.read_text(encoding='utf-8').splitlines(), 1):
                    self.assertNotIn('spec/paxeer-x', line, f'{filename}:{number}')
                self.assertEqual(module.SERVICE_CATALOGUE, 'spec/layerx-merge/spec.kvx')
                catalogue_path = module.ROOT / module.SERVICE_CATALOGUE
                self.assertEqual(catalogue_path, ROOT / 'spec/layerx-merge/spec.kvx')
                self.assertTrue(catalogue_path.is_file(), str(catalogue_path))
                result = subprocess.run(
                    [sys.executable, str(path), '--help'], cwd=ROOT,
                    capture_output=True, text=True, timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)


if __name__ == '__main__':
    unittest.main()
