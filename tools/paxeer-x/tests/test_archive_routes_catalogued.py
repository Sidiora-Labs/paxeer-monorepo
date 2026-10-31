import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]


def test_catalogue_lists_archive_routes_only():
    catalogue = json.loads(
        (ROOT / "tools/paxeer-x/route-catalogue.json").read_text(encoding="utf-8")
    )
    routes = catalogue["routes"]
    archive_prefix = "/api/v2/paxeer-x/archive/"
    expected_paths = {
        f"{archive_prefix}anchors",
        f"{archive_prefix}receipts",
        f"{archive_prefix}receipts/{{id}}",
    }
    archive_routes = [
        route
        for route in routes
        if route.get("upstream_path", "").startswith(archive_prefix)
    ]
    assert len(archive_routes) == 3
    assert {route["upstream_path"] for route in archive_routes} == expected_paths
    for route in archive_routes:
        upstream_path = route["upstream_path"]
        assert route["method"] == "GET"
        assert route["id"] == f"explorer-backend:GET:{upstream_path}"
        assert route["path"] == f"/explorer/backend{upstream_path}"
        assert route["service"] == "explorer"
        assert route["upstream"] == "explorer-backend"
        assert route["proxy"] is True

    retired_route = re.compile(r"/api/v2/paxeer-x/(?:anchors|receipts)(?:/|$)")
    for route in routes:
        for field in ("id", "path", "upstream_path"):
            assert not retired_route.search(route.get(field, "")), (
                f"Retired route remains in {route['id']} field {field}"
            )
