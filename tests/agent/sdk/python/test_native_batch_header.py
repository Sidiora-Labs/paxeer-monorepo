from __future__ import annotations

import sys
import unittest
from copy import deepcopy
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.px import decode_px_network_head  # noqa: E402


class NativeBatchHeaderTest(unittest.TestCase):
    def setUp(self):
        self.head = {
            "network_id": "paxeer-x",
            "paxeer": {"chain_id": "125", "latest_block": "13"},
            "consensus": {"latest_height": "12"},
            "layerx": None,
        }

    def test_network_head_reads_native_block_and_committed_height(self):
        original = deepcopy(self.head)
        header = decode_px_network_head(self.head)
        self.assertEqual(header.network_id, "paxeer-x")
        self.assertEqual(header.paxeer.chain_id, 125)
        self.assertEqual(header.paxeer.latest_block, 13)
        self.assertEqual(header.latest_committed_height, 12)
        self.assertIsNone(header.layerx)
        self.assertEqual(self.head, original)
        for height in ("0", "12", "13"):
            self.head["consensus"]["latest_height"] = height
            self.assertEqual(decode_px_network_head(self.head).latest_committed_height, int(height))

    def test_network_head_refuses_unavailable_or_retired_finality(self):
        for consensus in (None, {}, [], "12"):
            changed = deepcopy(self.head)
            changed["consensus"] = consensus
            with self.subTest(consensus=consensus), self.assertRaises(ValueError):
                decode_px_network_head(changed)
        for anchor in (None, {}, {"latest_height": "12"}):
            changed = deepcopy(self.head)
            changed["anchor"] = anchor
            with self.subTest(anchor=anchor), self.assertRaisesRegex(ValueError, "^invalid-px-finality$"):
                decode_px_network_head(changed)
        self.head["consensus"]["latest_height"] = "14"
        with self.assertRaisesRegex(ValueError, "^invalid-px-finality$"):
            decode_px_network_head(self.head)

    def test_network_head_requires_unsigned_height_bounds(self):
        for section, field in (("paxeer", "chain_id"), ("paxeer", "latest_block"),
                               ("consensus", "latest_height")):
            for invalid in (True, False, -1, 1.5, None, "", "-1", "01", "1.0", "0xg", str(1 << 64)):
                changed = deepcopy(self.head)
                changed[section][field] = invalid
                with self.subTest(section=section, field=field, invalid=invalid), self.assertRaises(ValueError):
                    decode_px_network_head(changed)
        for latest, committed in ((13, 12), ("0xd", "0xc")):
            changed = deepcopy(self.head)
            changed["paxeer"]["latest_block"] = latest
            changed["consensus"]["latest_height"] = committed
            header = decode_px_network_head(changed)
            self.assertEqual((header.paxeer.latest_block, header.latest_committed_height), (13, 12))
        maximum = str((1 << 64) - 1)
        self.head["paxeer"]["latest_block"] = maximum
        self.head["consensus"]["latest_height"] = maximum
        header = decode_px_network_head(self.head)
        self.assertEqual(header.paxeer.latest_block, (1 << 64) - 1)
        self.assertEqual(header.latest_committed_height, (1 << 64) - 1)

    def test_network_head_requires_network_and_block_fields(self):
        for field in ("network_id", "paxeer", "consensus"):
            changed = deepcopy(self.head)
            del changed[field]
            with self.subTest(field=field), self.assertRaises(ValueError):
                decode_px_network_head(changed)
        for field in ("chain_id", "latest_block"):
            changed = deepcopy(self.head)
            del changed["paxeer"][field]
            with self.subTest(field=field), self.assertRaises(ValueError):
                decode_px_network_head(changed)
        for invalid in (None, [], "", 125):
            with self.subTest(value=invalid), self.assertRaises(ValueError):
                decode_px_network_head(invalid)


if __name__ == "__main__":
    unittest.main()
