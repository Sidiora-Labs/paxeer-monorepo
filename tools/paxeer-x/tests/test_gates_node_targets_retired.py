#!/usr/bin/env python3
import configparser
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]
TOOLS = ROOT / "tools/paxeer-x"


class GatesNodeTargetsRetiredTest(unittest.TestCase):
    def test_node_target_gates_are_retired(self):
        gates = TOOLS / "gates"
        registry = configparser.ConfigParser(interpolation=None)
        registry.read_string((gates / "registry.kvx").read_text(encoding="utf-8"))
        operations = (ROOT / "spec/layerx-merge/contracts/operations.md").read_text(
            encoding="utf-8"
        )

        for selector in ("103.6.1", "108.1.1"):
            with self.subTest(selector=selector):
                script = gates / (selector + ".sh")
                self.assertFalse(script.exists() or script.is_symlink(), str(script))
                self.assertNotIn("task." + selector, registry)
                result = subprocess.run(
                    ["bash", str(TOOLS / "verify-task.sh"), selector],
                    cwd=ROOT,
                    stdin=subprocess.DEVNULL,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                self.assertIn("unknown task selector " + selector, result.stderr)
                self.assertNotIn(f"tools/paxeer-x/gates/{selector}.sh", operations)
                interface = "interface.16.cli-pxx-paxeer-x-gates-" + selector.replace(
                    ".", "-"
                )
                self.assertNotIn(f"<code>{interface}</code>", operations)

        self.assertNotIn("interface.16.env-pxx-paxeer-x-gates-108-1-1-", operations)
        for path in sorted(gates.rglob("*")):
            if path.is_file():
                with self.subTest(path=path.relative_to(ROOT).as_posix()):
                    self.assertNotIn(b"platform-test-node", path.read_bytes())


if __name__ == "__main__":
    unittest.main(verbosity=2)
