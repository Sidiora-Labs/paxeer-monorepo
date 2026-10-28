#[path = "handshake_gate.rs"]
mod handshake_gate;

use handshake_gate::{config, empty_node_config, must};
use layerx_agentd::boot::{readiness_gate, Gate, GateError, Status, WriteGateError};
use layerx_agentd::compat::{
    matrix, verify, CompatibilityError, CONTRACT_VERSION, DAEMON_VERSION, SDK_VERSION,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcError};

#[test]
fn daemon_is_not_ready_without_receipt_keys() {
    let mut gate = must(Gate::new(&config()));
    assert_eq!(gate.status(), &Status::NotReady);
    let (node_config, server) = empty_node_config(2, 4);
    let node = must(NodeRpc::connect(&node_config));
    assert_eq!(node.chain_id(), node_config.chain_id);
    let mut mismatched_config = node_config.clone();
    mismatched_config.chain_id ^= 1;
    assert_eq!(
        NodeRpc::connect(&mismatched_config).err(),
        Some(NodeRpcError::ChainIdMismatch {
            expected: mismatched_config.chain_id,
            observed: node_config.chain_id,
        })
    );
    assert_eq!(gate.status(), &Status::NotReady);
    assert!(
        must(node.receipt_keys()).is_empty(),
        "this criterion requires a real node with no receipt keys"
    );
    assert_eq!(readiness_gate(&node), Err(GateError::NoReceiptKeys));
    let operation_ran = std::cell::Cell::new(false);
    for _ in 0..2 {
        assert_eq!(gate.readiness_gate(&node), Err(GateError::NoReceiptKeys));
        assert_eq!(gate.status(), &Status::NotReady);
        assert_eq!(
            gate.guard_write(|| operation_ran.set(true)),
            Err(WriteGateError::NotReady)
        );
        assert!(matches!(
            gate.evidence_authority(),
            Err(WriteGateError::NotReady)
        ));
        assert!(!operation_ran.get());
        gate.disconnected();
        assert_eq!(gate.status(), &Status::NotReady);
    }
    must(server.join());
}

#[test]
fn compatibility_matrix_names_contract_major_two() {
    assert_eq!(CONTRACT_VERSION, 2);
    assert!(!matrix().is_empty());
    for row in matrix() {
        assert_eq!(row.contract_version, 2);
        for client in [
            format!("paxd/v{}.0.0/linux-amd64/go1.25", row.node_interface_major),
            format!("paxd/{}.12.3", row.node_interface_major),
        ] {
            assert_eq!(verify(DAEMON_VERSION, &client, 2, SDK_VERSION), Ok(*row));
            assert_eq!(
                verify(DAEMON_VERSION, &client, 1, SDK_VERSION),
                Err(CompatibilityError::UnsupportedContract)
            );
        }
        assert_eq!(
            verify(
                DAEMON_VERSION,
                &format!("paxd/v{}.0.0", row.node_interface_major + 1),
                2,
                SDK_VERSION
            ),
            Err(CompatibilityError::UnsupportedNodeInterface)
        );
    }
    for client in [
        "",
        "paxd",
        "paxd/v",
        "paxd/v1",
        "paxd/v1.0",
        "paxd/v1.0.bad",
        "paxd/v1.0.-rc1",
        "Geth/linux-amd64/go1.25",
        "/v1.0.0",
    ] {
        assert_eq!(
            verify(DAEMON_VERSION, client, 2, SDK_VERSION),
            Err(CompatibilityError::UnsupportedNodeInterface)
        );
    }
    let client = format!("paxd/v{}.0.0", matrix()[0].node_interface_major);
    assert_eq!(
        verify("unsupported", &client, 2, SDK_VERSION),
        Err(CompatibilityError::UnsupportedDaemon)
    );
    assert_eq!(
        verify(DAEMON_VERSION, &client, 2, "unsupported"),
        Err(CompatibilityError::UnsupportedSdk)
    );
}
