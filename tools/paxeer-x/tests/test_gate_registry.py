#!/usr/bin/env python3
import configparser
import json
import os
from pathlib import Path
import subprocess
import unittest

TOOLS = Path(__file__).resolve().parents[1]
ROOT = TOOLS.parents[1]


class GateRegistryTest(unittest.TestCase):
    def test_every_gate_is_registered_and_dispatches(self):
        registry = configparser.ConfigParser(interpolation=None)
        registry.read_string((TOOLS / 'gates/registry.kvx').read_text(encoding='utf-8'))
        gates = sorted((TOOLS / 'gates').glob('*.sh'))
        self.assertTrue(gates, 'no gate scripts found')
        self.assertTrue(any(os.access(gate, os.X_OK) for gate in gates),
                        'no executable gate scripts found')
        for gate in gates:
            with self.subTest(gate=gate.name):
                section = 'task.' + gate.stem
                self.assertIn(section, registry)
                self.assertEqual(json.loads(registry[section]['gate']),
                                 gate.relative_to(ROOT).as_posix())

        selector = '999999.999999'
        self.assertNotIn('task.' + selector, registry)
        result = subprocess.run([str(TOOLS / 'verify-task.sh'), selector], cwd=ROOT,
                                stdin=subprocess.DEVNULL, capture_output=True, text=True,
                                timeout=30)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn('unknown task selector ' + selector, result.stderr)


if __name__ == '__main__':
    unittest.main(verbosity=2)
