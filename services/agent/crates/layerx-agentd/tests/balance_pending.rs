#[path = "balance.rs"]
mod balance_tests;

use balance_tests::{must, Node};
use layerx_agentd::read::balance;
use serde_json::json;

#[test]
fn pending_balance_reports_value_height_and_chain_head() {
    let node = Node::funded();
    let value = must(balance(&node.config, [0x11; 20], None));
    assert_eq!(value.amount.value(), 900);
    assert_eq!(value.account, [0x11; 20]);
    assert_eq!(value.canonical_bytes, 900_u128.to_be_bytes());
    assert_eq!(value.achieved_rung, None);
    assert_eq!(value.freshness.chain_head, 40);
    assert_eq!(value.freshness.value_height, 41);
    assert_eq!(value.freshness.relative_to.tag(), "pending");
    assert_eq!(
        node.rpc(
            "eth_getBalance",
            json!([format!("0x{}", "11".repeat(20)), "pending"])
        ),
        format!("{:#x}", value.amount.value())
    );
    let at_height = must(balance(&node.config, [0x11; 20], Some(40)));
    assert_eq!(at_height.amount.value(), 900);
    assert_eq!(at_height.freshness.value_height, 40);
    assert_eq!(at_height.freshness.chain_head, 40);
    assert_eq!(at_height.freshness.relative_to.tag(), "0x28");
    assert_eq!(at_height.achieved_rung, None);

    let accounts = node.rpc("eth_accounts", json!([]));
    node.rpc(
        "eth_sendTransaction",
        json!([{
            "from": accounts[0],
            "to": format!("0x{}", "11".repeat(20)),
            "value": "0x1",
            "gas": "0x5208"
        }]),
    );
    let pending = must(balance(&node.config, [0x11; 20], None));
    assert_eq!(pending.amount.value(), 901);
    assert_eq!(pending.freshness.chain_head, 40);
    assert_eq!(pending.freshness.value_height, 41);
    assert_eq!(pending.freshness.relative_to.tag(), "pending");
    assert_eq!(pending.achieved_rung, None);
    assert_eq!(
        must(balance(&node.config, [0x11; 20], Some(40)))
            .amount
            .value(),
        900
    );
}

#[test]
fn balance_at_a_height_ahead_of_head_is_refused() {
    let node = Node::funded();
    let error = balance(&node.config, [0x11; 20], Some(45))
        .expect_err("a height beyond the captured chain head must not return an amount");
    assert_eq!(error.reason.as_str(), "read.height_ahead");
    assert_eq!(node.rpc("eth_blockNumber", json!([])), "0x28");
    let error = balance(&node.config, [0x11; 20], Some(41))
        .expect_err("the pending height is also ahead when requested explicitly");
    assert_eq!(error.reason.as_str(), "read.height_ahead");

    let pruned = Node::with_options(&["--prune-history", "1"]);
    pruned.rpc("anvil_mine", json!(["0x28"]));
    let error = balance(&pruned.config, [0x11; 20], Some(1))
        .expect_err("pruned historical state must not return a balance");
    assert_eq!(error.reason.as_str(), "read.height_pruned");
}
