#!/usr/bin/env python3
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
GOV = ROOT / 'tools/paxeer-x/gov'


class GovTemplatesTest(unittest.TestCase):
    def test_gov_suite_passes_without_retired_templates(self):
        retired_templates = (
            '04-exchange-markets.json',
            '05-custody.json',
            '06-anchor-sequencer-authorization.json',
        )
        for name in retired_templates:
            self.assertFalse((GOV / 'templates' / name).exists(), name)

        with tempfile.TemporaryDirectory() as work:
            out = Path(work) / 'out'
            env = dict(os.environ, GOV_TEST_OUT=str(out))
            result = subprocess.run(
                ['bash', 'tools/paxeer-x/gov/test.sh'], cwd=ROOT,
                env=env, stdin=subprocess.DEVNULL, capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertIn('governance window kit: ok', result.stdout)
            for name in retired_templates:
                self.assertFalse((out / name).exists(), name)

            proposals = sorted(out.rglob('*.json'))
            self.assertTrue(proposals, 'no rendered proposals found')
            for proposal in proposals:
                with self.subTest(proposal=proposal.relative_to(out).as_posix()):
                    rendered = json.dumps(json.loads(proposal.read_text(encoding='utf-8')))
                    for message in ('MsgSetMarket', 'layerxcustody', 'layerxanchor'):
                        self.assertNotIn(message, rendered)


if __name__ == '__main__':
    unittest.main(verbosity=2)
