use layerx_agent_api::identity::ContractError;
use layerx_agent_api::read::{FeeProjection, FeeProjectionRequest, Freshness, RelativeTo};
use layerx_agent_api::Amount;
use layerx_client::rpc::payments::{CommittedSnapshot, FeeEstimate};
use layerx_client::rpc::Head;

const HEIGHT: u64 = 9_007_199_254_740_993;

fn request() -> FeeProjectionRequest {
    FeeProjectionRequest {
        protocol_activity_type: 1,
        canonical_bytes: 256,
        execution_units: 2,
        storage_units: 3,
    }
}

fn head() -> Head {
    Head {
        chain_sequence: HEIGHT,
        sealed_batch: HEIGHT - 1,
        finalised_checkpoint: [4; 32],
    }
}

fn snapshot(observed_sequence: u64) -> CommittedSnapshot<FeeEstimate> {
    CommittedSnapshot {
        observed_sequence,
        state_root: [5; 32],
        value: FeeEstimate {
            parameter_version: 7,
            fee: u128::MAX,
            canonical_schedule: vec![1, 2, 3],
        },
    }
}

#[test]
fn fee_read_refuses_stale_and_future_snapshots_against_the_rpc_chain_head() {
    for observed in [HEIGHT - 1, HEIGHT + 1] {
        assert_eq!(
            FeeProjection::from_committed_snapshot(
                request(),
                head(),
                snapshot(observed),
                21_000,
                "upax".to_owned()
            ),
            Err(ContractError::Mismatch("fee_snapshot_height"))
        );
    }
}

#[test]
fn fee_read_accepts_current_snapshot_and_preserves_the_projection() {
    let read = FeeProjection::from_committed_snapshot(
        request(),
        head(),
        snapshot(HEIGHT),
        21_000,
        "upax".to_owned(),
    )
    .unwrap();
    assert_eq!(read.request, request());
    assert_eq!(read.parameter_version, 7);
    assert_eq!(read.fee, Amount(u128::MAX));
    assert_eq!(read.gas_units, 21_000);
    assert_eq!(read.denom, "upax");
    assert_eq!(read.canonical_schedule.as_bytes(), &[1, 2, 3]);
    assert_eq!(read.snapshot_height, HEIGHT);
    assert_eq!(read.snapshot_state_root, [5; 32]);
    let freshness = Freshness {
        chain_head: HEIGHT,
        value_height: HEIGHT,
        relative_to: RelativeTo::Block {
            height: HEIGHT,
            hash: [6; 32],
        },
    };
    let projection = read
        .clone()
        .into_projection("committed schedule estimate", freshness.clone())
        .unwrap();
    assert_eq!(projection.projected, read);
    assert_eq!(projection.observed_freshness, freshness);
    let stale = Freshness {
        chain_head: HEIGHT + 1,
        ..freshness
    };
    assert_eq!(
        read.into_projection("committed schedule estimate", stale),
        Err(ContractError::Mismatch("fee_snapshot_height"))
    );
}

#[test]
fn fee_read_keeps_the_existing_snapshot_validation() {
    let mut unversioned = snapshot(HEIGHT);
    unversioned.value.parameter_version = 0;
    assert_eq!(
        FeeProjection::from_committed_snapshot(
            request(),
            head(),
            unversioned,
            21_000,
            "upax".to_owned()
        ),
        Err(ContractError::Zero("parameter_version"))
    );
    let mut rootless = snapshot(HEIGHT);
    rootless.state_root = [0; 32];
    assert_eq!(
        FeeProjection::from_committed_snapshot(
            request(),
            head(),
            rootless,
            21_000,
            "upax".to_owned()
        ),
        Err(ContractError::Zero("snapshot_state_root"))
    );
    let mut empty = snapshot(HEIGHT);
    empty.value.canonical_schedule.clear();
    assert!(FeeProjection::from_committed_snapshot(
        request(),
        head(),
        empty,
        21_000,
        "upax".to_owned()
    )
    .is_err());
}
