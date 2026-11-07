#[path = "availability.rs"]
mod support;

use layerx_agentd::read::{
    availability, AvailabilityRead, AvailabilityReadError, AvailabilityRequest,
};
use serde_json::Value;
use support::{
    assert_available, available_exchanges, block_exchange, head_exchange, must, Node, BLOCK_HEIGHT,
    HEAD, PRUNED_HEIGHT,
};

#[test]
fn availability_reports_block_roots_and_transaction_count() {
    let height = BLOCK_HEIGHT;
    let node = Node::connect(available_exchanges(height));
    assert!(
        assert_available(&node, height) > 0,
        "select a block containing transactions"
    );
    node.finish();
}

#[test]
fn availability_for_a_missing_block_is_pruned_or_ahead() {
    let missing = PRUNED_HEIGHT;
    let node = Node::connect(vec![
        head_exchange(),
        block_exchange(missing, false, Value::Null),
        head_exchange(),
        block_exchange(missing, true, Value::Null),
        head_exchange(),
    ]);
    let head = node.head();
    assert_eq!(head, HEAD);
    assert!(
        missing <= head,
        "the pruned height must be at or below head"
    );
    assert!(
        node.block(missing).is_null(),
        "the test node must have pruned this block"
    );
    assert_eq!(
        must(availability(
            &node.endpoint,
            &AvailabilityRequest { height: missing }
        )),
        AvailabilityRead::Pruned { height: missing }
    );

    let requested = u64::MAX;
    assert!(head < requested);
    let error = availability(&node.endpoint, &AvailabilityRequest { height: requested })
        .err()
        .unwrap_or_else(|| panic!("a height above head must be refused"));
    let AvailabilityReadError::Ahead {
        requested: refused,
        head: observed,
    } = error
    else {
        panic!("expected Ahead, got {error:?}");
    };
    assert_eq!(refused, requested);
    assert!(observed >= head);
    assert_eq!(observed, head);
    assert!(observed < requested);
    node.finish();
}
