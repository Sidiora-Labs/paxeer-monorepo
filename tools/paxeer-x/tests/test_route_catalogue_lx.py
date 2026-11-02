#!/usr/bin/env python3
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[3]


class RouteCatalogueLxTest(unittest.TestCase):
    def test_catalogue_lists_no_lx_method(self):
        catalogue = json.loads(
            (ROOT / "tools/paxeer-x/route-catalogue.json").read_text(encoding="utf-8")
        )
        self.assertEqual(catalogue["schema"], "paxeer-x.routes.v1")
        self.assertEqual(
            [record for record in catalogue["rpc_records"] if record.startswith("lx_")],
            [],
        )
        # RPC method names are stored in route paths; rpc_records lists API hosts.
        self.assertEqual(
            [
                route["id"]
                for route in catalogue["routes"]
                if route["transport"] == "json-rpc"
                and (
                    route["path"].startswith("lx_")
                    or route["method"].startswith("lx_")
                )
            ],
            [],
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)
