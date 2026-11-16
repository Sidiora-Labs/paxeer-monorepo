import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
EXPECTED_KEYS = {
    "schema",
    "canonical_origin",
    "chain_id",
    "rpc_records",
    "preserve_api1_original_host",
    "wallet_authentication",
    "wallet_placement",
    "services",
    "routes",
    "denied_surfaces",
    "node_signing_methods",
    "rpc_paths",
}


def test_catalogue_top_keys_are_product_keys():
    catalogue = json.loads(
        (ROOT / "tools/paxeer-x/route-catalogue.json").read_text(encoding="utf-8")
    )

    assert set(catalogue) == EXPECTED_KEYS
