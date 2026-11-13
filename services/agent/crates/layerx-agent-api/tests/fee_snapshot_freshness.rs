use layerx_agent_api::identity::ContractError;
use layerx_agent_api::read::{FeeProjection, FeeProjectionRequest, Freshness, RelativeTo};
use layerx_agent_api::Amount;
use layerx_client::rpc::commit::{decode_validators, verify_commit_material};
use layerx_client::rpc::payments::{CommittedSnapshot, FeeEstimate};
use layerx_client::rpc::{ChainProofVerifier, CommittedBlockContext, Head};

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
    // Lossless excerpts of layerx-client/tests/fixtures/repair_22_33_1546_node.json
    // (storage). Keep the observed sequence's >2^53 boundary independent of height.
    let validators = decode_validators(
        &r#"{
  "height": "1200",
  "total": "1",
  "validators": [
    {
      "address": "c0742a797bd5374378f26a371f35e61b32c0d0cc",
      "pubKey": {
        "type": "tendermint/PubKeyEd25519",
        "value": "W2SJycf9Dc9QVF58FkiG70BJHsBsfxsSMEF5foEXU14="
      },
      "votingPower": "10"
    }
  ]
}"#
        .parse()
        .unwrap_or_else(|error| panic!("required value: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("required value: {error:?}"));
    let verifier = ChainProofVerifier {
        chain_id: "hyperpax_125-1".to_owned(),
        validators,
    };
    let authenticated = verify_commit_material(
        &r#"{
  "commit": {
    "block_id": {
      "hash": "FFDA0D5270A18312F1E9F243572C9A070C0A2A93C67C866CBE56D8E7ED62B468",
      "parts": {
        "hash": "3333333333333333333333333333333333333333333333333333333333333333",
        "total": 1
      }
    },
    "height": "1200",
    "round": 0,
    "signatures": [
      {
        "block_id_flag": 2,
        "signature": "C6+bBz9kdP2VV/+6/kfZhVzlLV6YNUV8xNcgvLeuWvDIGWocsQdxkVSglYFR+kKLyaeZxft8W2WVGeyUv0McDA==",
        "timestamp": "1970-01-01T00:16:40Z",
        "validator_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC"
      }
    ]
  },
  "header": {
    "app_hash": "6FBB9C33250A0A9CE954D5F421D8272972B728C3BE29FEB0CB8B51F0C3722186",
    "chain_id": "hyperpax_125-1",
    "consensus_hash": "3333333333333333333333333333333333333333333333333333333333333333",
    "data_hash": "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",
    "evidence_hash": "3333333333333333333333333333333333333333333333333333333333333333",
    "height": "1200",
    "last_block_id": {
      "hash": "3333333333333333333333333333333333333333333333333333333333333333",
      "parts": {
        "hash": "3333333333333333333333333333333333333333333333333333333333333333",
        "total": 1
      }
    },
    "last_commit_hash": "3333333333333333333333333333333333333333333333333333333333333333",
    "last_results_hash": "3333333333333333333333333333333333333333333333333333333333333333",
    "next_validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
    "proposer_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC",
    "time": "1970-01-01T00:16:40Z",
    "validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
    "version": {
      "app": "1",
      "block": "11"
    }
  }
}"#
        .parse().unwrap_or_else(|error| panic!("required value: {error:?}")),
        &verifier,
    ).unwrap_or_else(|error| panic!("required value: {error:?}"));
    let committed = CommittedBlockContext::from_verified(&authenticated)
        .unwrap_or_else(|error| panic!("required value: {error:?}"));
    assert_eq!(committed.committed_height(), 1200);
    assert_eq!(committed.block_hash(), authenticated.header_hash());
    assert_eq!(committed.app_hash(), authenticated.app_hash());
    let head = Head::new(committed, HEIGHT);
    assert_eq!(head.observed_sequence(), HEIGHT);
    assert_ne!(
        head.observed_sequence(),
        head.committed().committed_height()
    );
    head
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
fn fee_read_accepts_current_snapshot_and_preserves_the_projection() -> Result<(), String> {
    let read = FeeProjection::from_committed_snapshot(
        request(),
        head(),
        snapshot(HEIGHT),
        21_000,
        "upax".to_owned(),
    )
    .map_err(|error| format!("required value: {error:?}"))?;
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
        .map_err(|error| format!("required value: {error:?}"))?;
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

    Ok(())
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
