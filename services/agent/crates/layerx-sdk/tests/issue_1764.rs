use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::{node_commit::NodeCommitAuthority, ReceiptPolicy};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::rung::Rung;
use serde_json::Value;
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

struct SystemClock(Instant);
impl Clock for SystemClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        Ok(ClockReading {
            generation: [0x17; 16],
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Regression)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .0
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

fn fixed(value: &Value) -> [u8; 32] {
    hex::decode(
        value
            .as_str()
            .unwrap_or_else(|| panic!("hex field"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex prefix")),
    )
    .unwrap_or_else(|error| panic!("hex bytes: {error:?}"))
    .try_into()
    .unwrap_or_else(|error| panic!("32 bytes: {error:?}"))
}

#[test]
fn committed_wait_uses_unfiltered_batch_activity_ids() {
    let path = std::env::var("ISSUE_1764_CONFIG")
        .unwrap_or_else(|error| panic!("run through TestCommittedWaitUsesBatchActivityIDs with a live production node: {error:?}"));
    let config: Value = serde_json::from_slice(
        &std::fs::read(path).unwrap_or_else(|error| panic!("independent pins: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("pin configuration: {error:?}"));
    let height = config["height"]
        .as_u64()
        .unwrap_or_else(|| panic!("height"));
    let validators = pinned_validators(&config);
    let authority = NodeCommitAuthority::new(
        config["chainId"]
            .as_str()
            .unwrap_or_else(|| panic!("chain"))
            .into(),
        validators,
        height,
        height,
    )
    .unwrap_or_else(|error| panic!("independently pinned genesis authority: {error:?}"));
    let policy = ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(
            fixed(&config["keyId"]),
            fixed(&config["receiptKey"]),
            1,
            u64::MAX,
        ),
        trusted_checkpoint_context_digest: None,
    };
    let activity = fixed(&config["activity"]);
    let sequence = config["targetSequence"]
        .as_u64()
        .unwrap_or_else(|| panic!("sequence"));
    assert_eq!(
        sequence,
        config["fromSequence"]
            .as_u64()
            .unwrap_or_else(|| panic!("batch start"))
            + 1
    );
    for mode in [
        "genuine",
        "short_eth",
        "missing",
        "truncated",
        "reordered",
        "altered",
        "malformed",
    ] {
        let rpc = RpcClient::connect_node(
            config["endpoints"][mode]
                .as_str()
                .unwrap_or_else(|| panic!("live endpoint")),
            None,
        )
        .unwrap_or_else(|error| panic!("production SDK HTTP transport: {error:?}"))
        .with_clock(Arc::new(SystemClock(Instant::now())))
        .with_node_commit_authority(authority.clone())
        .unwrap_or_else(|error| panic!("pinned authority: {error:?}"));
        let result = rpc.wait_for(
            activity,
            Commitment::Committed,
            &policy,
            Duration::from_secs(10),
        );
        if matches!(mode, "genuine" | "short_eth") {
            let verified = result.unwrap_or_else(|error| panic!("{mode}: {error:?}"));
            assert_eq!(verified.rung(), Rung::Committed);
            assert_eq!(verified.commitment(), Commitment::Committed);
            assert_eq!(
                verified.committed_block(),
                Some((height, fixed(&config["blockHash"])))
            );
            let receipt = verified
                .receipt()
                .protocol()
                .unwrap_or_else(|| panic!("production protocol receipt"));
            assert_eq!(receipt.activity_id(), activity);
            assert_eq!(receipt.global_sequence(), sequence);
            println!("{mode}: verified Committed receipt at sequence {sequence}");
        } else {
            assert!(
                matches!(result, Err(RpcError::Verification)),
                "{mode} was not refused by verification: {:?}",
                result.as_ref().err()
            );
            println!("{mode}: refused by verification");
        }
    }
}

fn pinned_validators(config: &Value) -> Vec<([u8; 32], u64)> {
    config["validators"]
        .as_array()
        .unwrap_or_else(|| panic!("genesis validator set"))
        .iter()
        .map(|validator| {
            (
                fixed(&validator["key"]),
                validator["power"]
                    .as_u64()
                    .unwrap_or_else(|| panic!("power")),
            )
        })
        .collect::<Vec<_>>()
}
