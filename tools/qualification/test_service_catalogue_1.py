import importlib.util
from pathlib import Path
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer_x_release.py',
    'tools/qualification/paxeer-x/human_native_custody.py',
    'tools/qualification/paxeer-x/event-delivery-contract.py',
    'tools/qualification/paxeer-x/human_attestor_intents.py',
    'tools/qualification/paxeer-x/native_send_purpose.py',
)


class ServiceCatalogueReaders1Test(unittest.TestCase):
    def test_producers_load_existing_catalogue(self):
        original_path = sys.path[:]
        self.addCleanup(setattr, sys, 'path', original_path)
        self.addCleanup(setattr, sys, 'dont_write_bytecode', sys.dont_write_bytecode)
        for index, relative_path in enumerate(PRODUCERS):
            with self.subTest(producer=relative_path):
                path = ROOT / relative_path
                spec = importlib.util.spec_from_file_location(
                    'service_catalogue_producer_' + str(index), path)
                self.assertIsNotNone(spec)
                self.assertIsNotNone(spec.loader)
                module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(module)

                for name, value in vars(module).items():
                    if name.isupper() and isinstance(value, (str, Path)):
                        self.assertNotIn('spec/paxeer-x', str(value), name)
                for number, line in enumerate(path.read_text().splitlines(), 1):
                    self.assertNotIn('spec/paxeer-x', line, f'{relative_path}:{number}')

                self.assertEqual(module.ROOT, ROOT)
                self.assertEqual(module.SERVICE_CATALOGUE, 'spec/layerx-merge/spec.kvx')
                catalogue_path = module.ROOT / module.SERVICE_CATALOGUE
                self.assertTrue(catalogue_path.is_file(), str(catalogue_path))

                result = subprocess.run(
                    [sys.executable, str(path), '--help'], cwd=ROOT,
                    stdin=subprocess.DEVNULL, capture_output=True, text=True,
                    timeout=30, check=False)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)


if __name__ == '__main__':
    unittest.main()
