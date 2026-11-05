import importlib.util
from pathlib import Path
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "human-canonical-projections.py",
    "registry-router-bootstrap.py",
    "human-native-send-producer.py",
    "fixtures/xweb_attestation.py",
)


class ServiceCatalogueReaders3Test(unittest.TestCase):
    def test_producers_load_existing_catalogue(self):
        candidate_spec = importlib.util.spec_from_file_location(
            "service_catalogue_candidate", ROOT / "tools/paxeer-x/candidate.py"
        )
        candidate = importlib.util.module_from_spec(candidate_spec)
        candidate_spec.loader.exec_module(candidate)
        self.assertEqual(candidate.SERVICE_CATALOGUE, "spec/layerx-merge/spec.kvx")

        for index, relative in enumerate(PRODUCERS):
            with self.subTest(producer=relative):
                path = ROOT / "tools/qualification/paxeer-x" / relative
                spec = importlib.util.spec_from_file_location(
                    f"service_catalogue_producer_{index}", path
                )
                module = importlib.util.module_from_spec(spec)
                original_path = sys.path[:]
                original_bytecode = sys.dont_write_bytecode
                try:
                    spec.loader.exec_module(module)
                finally:
                    sys.path[:] = original_path
                    sys.dont_write_bytecode = original_bytecode

                for name, value in vars(module).items():
                    if name.isupper():
                        self.assertNotIn("spec/paxeer-x", repr(value), name)
                for number, line in enumerate(path.read_text().splitlines(), 1):
                    self.assertNotIn("spec/paxeer-x", line, f"{path}:{number}")

                self.assertEqual(module.ROOT, ROOT)
                self.assertEqual(module.SERVICE_CATALOGUE, candidate.SERVICE_CATALOGUE)
                catalogue_path = module.ROOT / module.SERVICE_CATALOGUE
                self.assertTrue(catalogue_path.is_file(), str(catalogue_path))
                if relative == "fixtures/xweb_attestation.py":
                    self.assertEqual(module.SPEC, catalogue_path)

                result = subprocess.run(
                    [sys.executable, str(path), "--help"],
                    cwd=ROOT,
                    stdin=subprocess.DEVNULL,
                    capture_output=True,
                    text=True,
                    timeout=30,
                    check=False,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("usage:", result.stdout)


if __name__ == "__main__":
    unittest.main()
