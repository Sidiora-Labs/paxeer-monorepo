use super::*;

#[path = "../support/mod.rs"]
mod support;

use layerx_mirror::committed::{CommittedBlockSource, ReceiptKeyConfig};
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorLag, MirrorObservation, MirrorTargetIdentity,
};
use layerx_mirror::{Archive, ArchiveData, MirrorEvidenceLevel, MirrorVerifyError, NodeHead};
use layerx_proof::receipt::ReceiptCheck;

const README: &str = include_str!("../../README.md");
const EXAMPLE: &str = include_str!("../../examples/verifier.json");

fn documented_requests() -> Vec<Value> {
    README
        .split("```json\n")
        .skip(1)
        .map(|part| {
            serde_json::from_str(part.split("```").next().or_panic("expected Some value"))
                .or_panic("expected Ok value")
        })
        .collect()
}

fn authorizations(keys: &[ReceiptKeyConfig]) -> Vec<SequencerAuthorization> {
    keys.iter()
        .map(|key| {
            SequencerAuthorization::from_config(
                &key.sequencer_id_hex,
                &key.public_key_hex,
                &key.first_height.to_string(),
                &key.last_height.to_string(),
            )
            .or_panic("expected Ok value")
        })
        .collect()
}

#[test]
fn finding_39_linked_example_uses_the_production_closed_schema() {
    assert!(README.contains("[verifier configuration example](examples/verifier.json)"));
    let config: Config = serde_json::from_str(EXAMPLE).or_panic("expected Ok value");
    assert_eq!(config.layerx_network_id, 125);
    assert_eq!(config.receipt_keys.len(), 1);
    assert_eq!(config.sources.len(), 2);
    let key = &config.receipt_keys[0];
    let producer = support::fixture();
    assert_eq!(
        fixed_hex::<32>(&key.sequencer_id_hex)
            .or_panic("expected Ok value")
            .as_slice(),
        support::bytes(&producer["receiptKey"]["keyId"])
    );
    assert_eq!(
        fixed_hex::<32>(&key.public_key_hex)
            .or_panic("expected Ok value")
            .as_slice(),
        support::bytes(&producer["receiptKey"]["publicKey"])
    );
    assert_eq!((key.first_height, key.last_height), (1000, 1999));
    for source in config.sources {
        let rpc = match source {
            SourceConfig::Ethereum {
                id,
                rpc,
                chain_id,
                genesis_hash_hex,
                archive_contract_hex,
                archive_code_hash_hex,
                publisher_hex,
            } => {
                assert_eq!(id, "ethereum-primary");
                assert_eq!(chain_id, 11_155_111);
                fixed_hex::<32>(&genesis_hash_hex).or_panic("expected Ok value");
                fixed_hex::<20>(&archive_contract_hex).or_panic("expected Ok value");
                fixed_hex::<32>(&archive_code_hash_hex).or_panic("expected Ok value");
                fixed_hex::<20>(&publisher_hex).or_panic("expected Ok value");
                rpc
            }
            SourceConfig::Solana {
                id,
                rpc,
                genesis_hash_hex,
                archive_program_hex,
                upgradeable_loader_hex,
                program_data_account_hex,
                program_code_hash_hex,
                publisher_hex,
            } => {
                assert_eq!(id, "solana-primary");
                for pin in [
                    genesis_hash_hex,
                    archive_program_hex,
                    upgradeable_loader_hex,
                    program_data_account_hex,
                    program_code_hash_hex,
                    publisher_hex,
                ] {
                    fixed_hex::<32>(&pin).or_panic("expected Ok value");
                }
                rpc
            }
        };
        assert_eq!(rpc.endpoints.len(), 3);
        assert_eq!(rpc.quorum, 2);
        assert_eq!(rpc.connect_timeout_ms, 3000);
        assert_eq!(rpc.request_timeout_ms, 15000);
        assert_eq!(rpc.maximum_response_bytes, 16_777_216);
        let mut identities = std::collections::BTreeSet::new();
        for endpoint in rpc.endpoints {
            assert!(endpoint.url.starts_with("https://"));
            assert!(endpoint.ca_certificate_der.is_absolute());
            assert!(endpoint.bearer_token_file.is_absolute());
            assert!(identities.insert(endpoint.independent_backend));
        }
    }
    // A copied publisher schema or obsolete verifier field must still fail.
    for path in [vec![], vec!["sources", "0"], vec!["receipt_keys", "0"]] {
        let mut invalid: Value = serde_json::from_str(EXAMPLE).or_panic("expected Ok value");
        let target = match path.as_slice() {
            [] => &mut invalid,
            ["sources", "0"] => &mut invalid["sources"][0],
            _ => &mut invalid["receipt_keys"][0],
        };
        target["obsolete_field"] = json!(true);
        assert!(serde_json::from_value::<Config>(invalid).is_err());
    }
}

#[test]
fn finding_39_documented_requests_cross_native_source_codec_and_receipt_consumer() {
    let config: Config = serde_json::from_str(EXAMPLE).or_panic("expected Ok value");
    let document = support::fixture();
    let mut peer = support::TestRpc::new(vec![document.clone(); 3]);
    // The committed source's TLS peers replay the producer's committed transcript;
    // the receipt trust comes independently from the documented operator config.
    peer.config.receipt_keys = config.receipt_keys.clone();
    let source = CommittedBlockSource::open(&peer.config).or_panic("expected Ok value");
    let block = source.acquire(1000).or_panic("expected Ok value");
    let coordinate = CommittedCoordinate {
        height: block.header.height,
        block_hash: block.header.block_hash,
    };
    let head = NodeHead {
        latest_sealed_batch: coordinate.height,
        latest_committed: Some(coordinate),
    };
    let produced = Archive::from_committed(&block, config.layerx_network_id, head)
        .or_panic("expected Ok value");
    let transport = produced.bytes().to_vec();
    let decoded = ArchiveData::decode(&transport).or_panic("expected Ok value");
    assert_eq!(decoded.network_id, 125);
    assert_eq!(decoded.batch_number, 1000);
    assert_eq!(decoded.committed.receipts, block.receipts);
    let requests = documented_requests();
    assert_eq!(requests.len(), 2);
    let keys = authorizations(&config.receipt_keys);
    let mut selectors = Vec::new();
    for mut request in requests {
        // Follow the documented instruction to replace the commitment placeholder
        // with the actual producer commitment, retaining the rest of the request.
        request["policy"]["candidate"]["commitment_hex"] =
            json!(encode_hex(produced.commitment().as_bytes()));
        let request: Request = serde_json::from_value(request).or_panic("expected Ok value");
        assert_eq!(canonical_u64(&request.batch_number), Ok(1000));
        let MirrorReadPolicy::Exact(locator) =
            policy(&request.policy).or_panic("expected Ok value")
        else {
            panic!("documented exact policy")
        };
        assert_eq!(locator.source_index, 0);
        assert_eq!(locator.commitment, produced.commitment());
        let observed = observation(&config, &transport, &locator);
        assert_eq!(observed.freshness.latest_batch, None);
        assert_eq!(observed.freshness.committed_lag, MirrorLag::Unknown);
        let consumer =
            MirrorVerifier::admit_committed(&transport, observed).or_panic("expected Ok value");
        assert_eq!(consumer.committed(), Ok(coordinate));
        let output = match request.evidence {
            EvidenceRequest::Receipt { index } => {
                selectors.push("receipt");
                assert_eq!(index, 0);
                let verified = consumer
                    .committed_receipt(index, &keys)
                    .or_panic("expected Ok value");
                assert_eq!(verified.level(), MirrorEvidenceLevel::Committed);
                let unsigned = layerx_wire::receipt::encode_unsigned(verified.value())
                    .or_panic("expected Ok value");
                let digest =
                    layerx_wire::hash::receipt_digest(&unsigned).or_panic("expected Ok value");
                assert_eq!(
                    digest.as_slice(),
                    support::bytes(&document["receipts"][0]["receiptDigest"])
                );
                report(
                    verified.observation().or_panic("expected Some value"),
                    coordinate,
                    Some(digest),
                )
            }
            EvidenceRequest::Committed {} => {
                selectors.push("committed");
                report(
                    consumer.observation().or_panic("expected Some value"),
                    coordinate,
                    None,
                )
            }
        };
        assert_documented_output(&output, coordinate, selectors.last() == Some(&"committed"));
        assert_eq!(
            consumer.committed_receipt(block.receipts.len(), &keys),
            Err(MirrorVerifyError::ReceiptMissing)
        );
        assert_eq!(
            consumer.committed_receipt(0, &[]),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
        assert_receipt_key_windows(&consumer, &config);
    }
    assert_eq!(selectors, ["receipt", "committed"]);

    assert_damaged_receipt_refused(&config, &block, head, coordinate, &keys, &peer);
}

fn assert_documented_output(
    output: &Value,
    coordinate: CommittedCoordinate,
    committed_selector: bool,
) {
    assert_eq!(output["level"], "Committed");
    assert_eq!(output["committedHeight"], "1000");
    assert_eq!(
        output["committedBlockHash"],
        encode_hex(&coordinate.block_hash)
    );
    assert_eq!(output["latestBatch"], Value::Null);
    assert_eq!(output["batchLag"], "Unknown");
    assert_eq!(output["evidenceDigest"].is_null(), committed_selector);
}

fn assert_receipt_key_windows(consumer: &MirrorVerifier, config: &Config) {
    for (first, last) in [(1000, 1000), (1001, 1999), (1, 999)] {
        let mut window = config.receipt_keys.clone();
        window[0].first_height = first;
        window[0].last_height = last;
        let result = consumer.committed_receipt(0, &authorizations(&window));
        if first == 1000 {
            assert!(result.is_ok());
        } else {
            assert_eq!(
                result,
                Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
            );
        }
    }
    let mut wrong_key = config.receipt_keys.clone();
    wrong_key[0].public_key_hex = "11".repeat(32);
    assert_eq!(
        consumer.committed_receipt(0, &authorizations(&wrong_key)),
        Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
    );
}

fn assert_damaged_receipt_refused(
    config: &Config,
    block: &layerx_mirror::committed::CommittedBlock,
    head: NodeHead,
    coordinate: CommittedCoordinate,
    keys: &[SequencerAuthorization],
    peer: &support::TestRpc,
) {
    // Coordinate admission is deliberately weaker than signature authentication.
    let mut damaged = block.clone();
    *damaged.receipts[0]
        .last_mut()
        .or_panic("expected Some value") ^= 1;
    let damaged = Archive::from_committed(&damaged, 125, head).or_panic("expected Ok value");
    let locator = MirrorLocator {
        source_index: 0,
        commitment: damaged.commitment(),
    };
    let consumer = MirrorVerifier::admit_committed(
        damaged.bytes(),
        observation(config, damaged.bytes(), &locator),
    )
    .or_panic("expected Ok value");
    assert_eq!(consumer.committed(), Ok(coordinate));
    assert_eq!(
        consumer.committed_receipt(0, keys),
        Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
    );
    let requests = peer.requests.lock().or_panic("expected Ok value");
    for method in [
        "eth_chainId",
        "eth_getBlockByNumber",
        "pax_sequencerReceipt",
    ] {
        assert!(requests
            .iter()
            .any(|(_, request)| request["method"] == method));
    }
}

fn observation(config: &Config, bytes: &[u8], locator: &MirrorLocator) -> MirrorObservation {
    let SourceConfig::Ethereum {
        id,
        chain_id,
        genesis_hash_hex,
        archive_contract_hex,
        archive_code_hash_hex,
        publisher_hex,
        ..
    } = &config.sources[locator.source_index]
    else {
        panic!("source 0 is Ethereum")
    };
    // Destination coordinates are fixture context at the public codec boundary;
    // this proof does not claim a live Ethereum deployment or consensus inclusion.
    MirrorObservation::from_archive(
        MirrorSourceId::new(id.clone()).or_panic("expected Ok value"),
        MirrorTargetIdentity::Ethereum {
            chain_id: *chain_id,
            genesis_hash: fixed_hex(genesis_hash_hex).or_panic("expected Ok value"),
            contract: fixed_hex(archive_contract_hex).or_panic("expected Ok value"),
            code_hash: fixed_hex(archive_code_hash_hex).or_panic("expected Ok value"),
            publisher: fixed_hex(publisher_hex).or_panic("expected Ok value"),
        },
        bytes,
        config.layerx_network_id,
        1000,
        locator.commitment,
        MirrorCanonicalPosition::Ethereum {
            block_number: 50,
            block_hash: [5; 32],
            reference_head_number: 62,
            reference_head_hash: [6; 32],
        },
    )
    .or_panic("expected Ok value")
}

#[test]
fn finding_39_selectors_and_durable_upgrade_limits_remain_explicit() {
    for request in documented_requests() {
        let parsed: Request = serde_json::from_value(request.clone()).or_panic("expected Ok value");
        assert_eq!(canonical_u64(&parsed.batch_number), Ok(1000));
        assert!(matches!(
            policy(&parsed.policy).or_panic("expected Ok value"),
            MirrorReadPolicy::Exact(_)
        ));
        for field in ["obsolete_field", "state"] {
            let mut invalid = request.clone();
            invalid[field] = json!(true);
            assert!(serde_json::from_value::<Request>(invalid).is_err());
        }
        let mut unsupported = request.clone();
        unsupported["evidence"] = json!({"kind":"state"});
        assert!(serde_json::from_value::<Request>(unsupported).is_err());
        let mut unsupported = request.clone();
        unsupported["evidence"]["tx_hash"] = json!("11".repeat(32));
        assert!(serde_json::from_value::<Request>(unsupported).is_err());
    }
    for invalid in ["", "0", "01000", "0x3e8", "-1", "18446744073709551616"] {
        assert_eq!(canonical_u64(invalid), Err("malformed"));
    }
    for statement in [
        "chain ID 125",
        "pure v4 archive",
        "inclusive at both ends",
        "does not authenticate a receipt signature",
        "consensus certificate",
        "There is no automatic v2-to-v3 migration",
        "LXMJNL03",
        "LXMHED03",
        "LXMSPL03",
        "publisher operator owns",
        "Unresolved transactions block cutover",
        "distinct empty directory with `first_height`",
        "Domain 11 (interop)",
    ] {
        assert!(
            README.contains(statement),
            "missing documented boundary: {statement}"
        );
    }
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
