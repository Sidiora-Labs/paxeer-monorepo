use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::{ReceiptPolicy, node_commit::NodeCommitAuthority};
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
            .expect("hex field")
            .strip_prefix("0x")
            .expect("hex prefix"),
    )
    .expect("hex bytes")
    .try_into()
    .expect("32 bytes")
}

#[test]
fn committed_wait_uses_unfiltered_batch_activity_ids() {
    let path = std::env::var("ISSUE_1764_CONFIG")
        .expect("run through TestCommittedWaitUsesBatchActivityIDs with a live production node");
    let config: Value = serde_json::from_slice(&std::fs::read(path).expect("independent pins"))
        .expect("pin configuration");
    let height = config["height"].as_u64().expect("height");
    let validators = config["validators"]
        .as_array()
        .expect("genesis validator set")
        .iter()
        .map(|validator| {
            (
                fixed(&validator["key"]),
                validator["power"].as_u64().expect("power"),
            )
        })
        .collect::<Vec<_>>();
    let authority = NodeCommitAuthority::new(
        config["chainId"].as_str().expect("chain").into(),
        validators,
        height,
        height,
    )
    .expect("independently pinned genesis authority");
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
    let sequence = config["targetSequence"].as_u64().expect("sequence");
    assert_eq!(
        sequence,
        config["fromSequence"].as_u64().expect("batch start") + 1
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
            config["endpoints"][mode].as_str().expect("live endpoint"),
            None,
        )
        .expect("production SDK HTTP transport")
        .with_clock(Arc::new(SystemClock(Instant::now())))
        .with_node_commit_authority(authority.clone())
        .expect("pinned authority");
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
                .expect("production protocol receipt");
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
