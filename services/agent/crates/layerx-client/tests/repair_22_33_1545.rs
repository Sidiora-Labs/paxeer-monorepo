//! Audit 22.30.96: real RPC consumers over loopback HTTP and committed producer records.
//! Closure status and external wiring requests live in rpc/repair_22_33_1545.kvx.
#[path = "support/gateway_22_33_1546.rs"]
mod gateway;

use gateway::{must, Server};
use layerx_client::rpc::{
    commit::{acquire_committed_header, decode_validators},
    payments::{self, CallRequest, CommittedSnapshot, FeeEstimate},
    withdrawal::{registry, WithdrawalConfigurationError},
    CommittedBlockContext, Head, ObservedBlockContext,
};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_programs_registry::chain_proof::ChainProofVerifier;
use layerx_types::payload::{ActivityType, ModuleId};
use serde_json::{json, Value};

fn node_record() -> Value {
    must(serde_json::from_str(include_str!(
        "fixtures/repair_22_33_1546_node.json"
    )))
}

fn bytes(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn hex(value: &[u8]) -> String {
    format!(
        "0x{}",
        value.iter().map(|v| format!("{v:02x}")).collect::<String>()
    )
}

#[test]
fn f26_registered_history_preserves_gateway_observations_without_batch_proof_authority() {
    gateway::history();
}

#[test]
fn f28_committed_context_requires_verified_node_evidence_and_keeps_sequence_separate() {
    let record = node_record()["storage"].clone();
    let verifier = ChainProofVerifier {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .expect("chain id")
            .to_owned(),
        validators: must(decode_validators(&record["validators"])),
    };
    let height = record["height"].as_u64().expect("height");
    let expected_hash: [u8; 32] = must(bytes(record["hash"].as_str().expect("hash")).try_into());
    let expected_root: [u8; 32] = must(bytes(record["appHash"].as_str().expect("root")).try_into());
    let served = record.clone();
    let server = Server::start(move |r| {
        assert_eq!(r["params"], json!([height.to_string()]));
        match r["method"].as_str().expect("method") {
            "px_getCommit" => json!({"result":served["material"]}),
            "px_getValidators" => json!({"result":served["validators"]}),
            method => panic!("unexpected {method}"),
        }
    });
    let verified = must(acquire_committed_header(
        &server.sequencer(),
        height,
        &verifier,
    ));
    let context = must(CommittedBlockContext::from_verified(&verified));
    let compatibility: Head = ObservedBlockContext::new(context, 7);
    assert_eq!(compatibility.committed(), context);
    assert_eq!(compatibility.observed_sequence(), 7);
    assert_eq!(compatibility.chain_sequence, 7);
    assert_eq!(context.committed_height(), height);
    assert_eq!(context.block_hash(), expected_hash);
    assert_eq!(context.app_hash(), expected_root);
    let snapshot = CommittedSnapshot {
        observed_sequence: 7,
        state_root: context.app_hash(),
        value: FeeEstimate {
            parameter_version: 0,
            fee: 21_000,
            canonical_schedule: vec![],
        },
    };
    assert_eq!(snapshot.observed_sequence, 7);
    assert_ne!(snapshot.observed_sequence, context.committed_height());
    assert_eq!(snapshot.state_root, expected_root);
    assert_eq!(server.count(), 2);
    for damage in ["app_hash", "signature"] {
        let mut damaged = record.clone();
        if damage == "app_hash" {
            damaged["material"]["signedHeader"]["header"]["app_hash"] = json!("ff".repeat(32));
        } else {
            damaged["material"]["signedHeader"]["commit"]["signatures"][0]["signature"] =
                json!("A".repeat(88));
        }
        let server = Server::start(move |r| match r["method"].as_str().expect("method") {
            "px_getCommit" => json!({"result":damaged["material"]}),
            "px_getValidators" => json!({"result":damaged["validators"]}),
            method => panic!("unexpected {method}"),
        });
        assert!(
            acquire_committed_header(&server.sequencer(), height, &verifier).is_err(),
            "accepted {damage}"
        );
    }
}

#[test]
fn f27_payments_keep_flat_gateway_metadata_and_node_gas_estimates() {
    gateway::assets();
    let call = CallRequest {
        from: Some([0x12; 20]),
        to: Some([0x34; 20]),
        value: 7,
        data: vec![0xde, 0xad, 0xbe, 0xef, 0, 0xff],
    };
    let expected = call.clone();
    let server = Server::start(move |r| {
        assert_eq!(r["method"], "eth_estimateGas");
        assert_eq!(
            r["params"],
            json!([{
            "from":hex(&expected.from.expect("sender")),
            "to":hex(&expected.to.expect("target")),
            "value":"0x7", "data":hex(&expected.data)
        }, "pending"])
        );
        json!({"result":"0x12345"})
    });
    assert_eq!(
        must(payments::estimate_fee(&server.sequencer(), &call)),
        FeeEstimate {
            parameter_version: 0,
            fee: 0x12345,
            canonical_schedule: vec![],
        }
    );
    assert_eq!(server.count(), 1);
}

fn custody_vectors() -> Value {
    must(serde_json::from_str(include_str!(
        "../src/rpc/withdrawal_get_asset.json"
    )))
}

fn custody_server(native: Value, table: Value, gas: Value) -> Server {
    Server::start(move |r| match r["method"].as_str().expect("method") {
        "eth_estimateGas" => gas.clone(),
        "px_listAssets" => {
            assert_eq!(r["params"], json!([]));
            json!({"result":table})
        }
        "eth_call" => {
            let selector = &transaction_hash(b"getAsset(bytes32)")[..4];
            let data = [selector, &[0x11; 32]].concat();
            assert_eq!(
                r["params"],
                json!([{
                "to":"0x0000000000000000000000000000000000001013",
                "data":hex(&data)
            },"0x10"])
            );
            native.clone()
        }
        method => panic!("unexpected {method}"),
    })
}

#[test]
fn f29_native_configuration_codec_drives_eligibility_and_every_refusal() {
    // These ABI vectors cover the client codec; they do not claim a live merged producer.
    let abi: Value = must(serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxcustody/abi.json"
    )));
    let method = abi
        .as_array()
        .expect("ABI")
        .iter()
        .find(|m| m["name"] == "getAsset")
        .expect("getAsset");
    let fields = method["outputs"][0]["components"]
        .as_array()
        .expect("tuple fields");
    assert_eq!(
        fields
            .iter()
            .map(|f| f["type"].as_str().expect("type"))
            .collect::<Vec<_>>(),
        vec![
            "bytes32", "string", "address", "bool", "bool", "uint256", "uint256", "uint256",
            "uint256", "uint256"
        ]
    );
    let vectors = custody_vectors();
    let table = gateway::records()["assets"].clone();
    let call = CallRequest::default();
    let server = custody_server(
        json!({"result":vectors["eligible"]}),
        table.clone(),
        json!({"result":"0x5208"}),
    );
    let modules = must(registry(
        &server.sequencer(),
        &call,
        &server.px(),
        [0x11; 32],
    ));
    for (module, ordinal) in [(ModuleId::Asset, 9), (ModuleId::Bridge, 1)] {
        assert!(modules.declares(must(ActivityType::new(module, ordinal))));
    }
    assert!(!modules.declares(must(ActivityType::new(ModuleId::Asset, 1))));
    assert_eq!(server.count(), 3);
    for case in ["disabled", "paused", "zero_pointer", "wrong_asset"] {
        let server = custody_server(
            json!({"result":vectors[case]}),
            table.clone(),
            json!({"result":"0x5208"}),
        );
        assert_eq!(
            registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
            Err(WithdrawalConfigurationError::AssetUnavailable),
            "{case}"
        );
        assert_eq!(server.count(), 3);
    }
    let eligible = bytes(vectors["eligible"].as_str().expect("vector"));
    let mut malformed = vec![eligible[..eligible.len() - 1].to_vec()];
    for (offset, value) in [
        (31, 64),
        (95, 0),
        (96, 1),
        (159, 2),
        (191, 2),
        (383, 0),
        (415, 1),
    ] {
        let mut bad = eligible.clone();
        bad[offset] = value;
        malformed.push(bad);
    }
    let mut trailing = eligible.clone();
    trailing.extend_from_slice(&[0; 32]);
    malformed.push(trailing);
    for bad in malformed {
        let server = custody_server(
            json!({"result":hex(&bad)}),
            table.clone(),
            json!({"result":"0x5208"}),
        );
        assert_eq!(
            registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
            Err(WithdrawalConfigurationError::Unavailable)
        );
    }
    let server = custody_server(
        json!({"error":{"code":3,"message":"execution reverted: LXP_MODULE_DISABLED: module disabled; layerxCustody retired"}}),
        table.clone(),
        json!({"result":"0x5208"}),
    );
    assert_eq!(
        registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
        Err(WithdrawalConfigurationError::Unsupported)
    );
    let server = custody_server(
        json!({"error":{"code":-32000,"message":"historical state unavailable"}}),
        table.clone(),
        json!({"result":"0x5208"}),
    );
    assert_eq!(
        registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
        Err(WithdrawalConfigurationError::Unavailable)
    );
    for (field, value, error) in [
        (
            "asset_id",
            json!(format!("0x{}", "22".repeat(32))),
            WithdrawalConfigurationError::AssetUnavailable,
        ),
        (
            "kind",
            json!("erc20"),
            WithdrawalConfigurationError::Unsupported,
        ),
    ] {
        let mut other = table.clone();
        other["assets"][0][field] = value;
        let server = custody_server(
            json!({"result":vectors["eligible"]}),
            other,
            json!({"result":"0x5208"}),
        );
        assert_eq!(
            registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
            Err(error)
        );
        assert_eq!(server.count(), 2);
    }
    let mut zero_height = table.clone();
    zero_height["height"] = json!(0);
    let server = custody_server(
        json!({"result":vectors["eligible"]}),
        zero_height,
        json!({"result":"0x5208"}),
    );
    assert_eq!(
        registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
        Err(WithdrawalConfigurationError::AssetUnavailable)
    );
    let server = custody_server(
        json!({"result":vectors["eligible"]}),
        table,
        json!({"error":{"code":3,"message":"execution reverted"}}),
    );
    assert_eq!(
        registry(&server.sequencer(), &call, &server.px(), [0x11; 32]),
        Err(WithdrawalConfigurationError::Unavailable)
    );
    assert_eq!(server.count(), 1);
}
