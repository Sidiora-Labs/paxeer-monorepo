use std::collections::BTreeSet;

use layerx_platform_gateway::unified_service_catalogue;
use serde_json::Value;

const PRODUCT_KEYS: &[&str] = &[
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
];

#[test]
fn unified_catalogue_publishes_product_and_location_keys_without_rpc_routes() {
    let locations: Value = serde_json::from_str(include_str!("../route-locations.json"))
        .unwrap_or_else(|error| panic!("location map is valid JSON: {error}"));
    let expected: BTreeSet<_> = PRODUCT_KEYS
        .iter()
        .copied()
        .chain(
            locations
                .as_object()
                .unwrap_or_else(|| panic!("location map is an object"))
                .keys()
                .map(String::as_str),
        )
        .collect();
    let catalogue = unified_service_catalogue();
    let actual: BTreeSet<_> = catalogue
        .as_object()
        .unwrap_or_else(|| panic!("unified catalogue is an object"))
        .keys()
        .map(String::as_str)
        .collect();
    assert_eq!(actual, expected);
    assert!(catalogue["routes"]
        .as_array()
        .unwrap_or_else(|| panic!("unified catalogue routes are an array"))
        .iter()
        .all(|route| route["path"]
            .as_str()
            .is_none_or(|path| path != "/rpc" && !path.starts_with("/rpc/"))));
}
