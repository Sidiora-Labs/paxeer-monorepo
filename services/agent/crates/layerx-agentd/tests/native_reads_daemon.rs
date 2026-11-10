#[path = "native_routes_rpc.rs"]
mod native_routes_rpc;

use layerx_agentd::read::{AvailabilityReadError, NativeReadError};
use native_routes_rpc::{assert_receipt, endpoint, live_route, must};

#[test]
fn native_rpc_availability_refusals() {
    let (mut route, config) = live_route();
    assert_receipt(&mut route, &config);
    let client = endpoint(&config);
    let head = must(client.block_number());
    let ahead = u64::MAX;
    assert!(
        ahead > head,
        "real node head must leave an out-of-range height"
    );
    assert!(matches!(
        route.read(&format!("/v1/reads/availability/{ahead}")),
        Err(NativeReadError::Availability(AvailabilityReadError::Ahead { requested, head: observed }))
            if requested == ahead && observed >= head && observed < ahead
    ));
    let available = must(route.read(&format!("/v1/reads/availability/{head}")));
    let block = lint_present(
        must(client.block_by_number(head)),
        "real node retains its head block",
    );
    assert_eq!(available["height"], head);
    assert_eq!(available["complete"], true);
    assert_eq!(
        available["transactions_root"],
        layerx_programs::hex::encode(&block.transactions_root)
    );
    assert_eq!(
        available["receipts_root"],
        layerx_programs::hex::encode(&block.receipts_root)
    );
    assert_eq!(available["transaction_count"], block.transactions.len());
    assert!(available["rung"].is_null());
    let hash = native_routes_rpc::receipt_hash();
    let account = format!("0x{}", layerx_programs::hex::encode(&hash));
    let page = must(route.read(&format!("/v1/reads/history/{account}?limit=2")));
    assert!(lint_present(page["items"].as_array(), "bounded history items").len() <= 2);
    assert!(
        lint_present(page["freshness"]["value_height"].as_u64(), "indexed height")
            <= lint_present(page["freshness"]["chain_head"].as_u64(), "chain height")
    );
    assert!(page["rung"].is_null());
    assert!(matches!(
        route.read("/v1/reads/history/account?limit=2&cursor=bad"),
        Err(NativeReadError::CursorMismatch)
    ));
    for path in [
        "/v1/reads/availability/01",
        "/v1/reads/checkpoint/-1",
        "/v1/reads/receipt/not-a-digest",
        "/v1/reads/proof/not-a-digest",
        "/v1/reads/history/account?limit=0",
    ] {
        assert!(
            matches!(route.read(path), Err(NativeReadError::InvalidRequest)),
            "malformed native read selector must be refused: {path}"
        );
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}
