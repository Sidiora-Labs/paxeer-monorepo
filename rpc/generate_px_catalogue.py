"""Project committed product route contracts into the node's public catalogue."""
import json
from pathlib import Path

root = Path(__file__).resolve().parent.parent
source = json.loads((root / "tools/paxeer-x/route-catalogue.json").read_text())
service_fields = {
    "id", "purpose", "exposure", "discovery_path", "transport",
    "authentication", "version",
}
private_route_fields = {
    "upstream", "upstream_path", "source", "owner_source",
    "forwarded_headers", "attestor_authorization_headers",
}
services = [
    {key: value for key, value in service.items() if key in service_fields}
    for service in source["services"] if service["exposure"] == "product"
]
public_ids = {service["id"] for service in services}
catalogue = {
    "schema": source["schema"],
    "chain_id": source["chain_id"],
    "services": services,
    "routes": [
        {key: value for key, value in route.items() if key not in private_route_fields}
        for route in source["routes"] if route.get("service") in public_ids
    ],
}
(root / "rpc/px_route_catalogue.json").write_text(json.dumps(catalogue, indent=2) + "\n")
