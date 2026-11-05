import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
GATEWAY_OPENRPC = "services/platform/hosted/gateway/openrpc.json"


def test_catalogued_rpc_methods_are_served():
    catalogue = json.loads(
        (ROOT / "tools/paxeer-x/route-catalogue.json").read_text(encoding="utf-8")
    )
    openrpc = json.loads((ROOT / GATEWAY_OPENRPC).read_text(encoding="utf-8"))
    served_methods = {method["name"] for method in openrpc["methods"]}
    gateway_routes = [
        route
        for route in catalogue["routes"]
        if route["transport"] == "json-rpc" and route["source"] == GATEWAY_OPENRPC
    ]

    assert served_methods, "Gateway OpenRPC must declare served methods"
    assert gateway_routes, "Catalogue must include gateway JSON-RPC routes"
    unserved_routes = [
        f"{route['id']} (method {route['path']})"
        for route in gateway_routes
        if route["path"] not in served_methods
    ]
    assert not unserved_routes, (
        f"Catalogue methods missing from {GATEWAY_OPENRPC}: {unserved_routes}"
    )
