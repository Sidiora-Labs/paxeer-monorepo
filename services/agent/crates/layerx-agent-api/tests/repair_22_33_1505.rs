//! Build repair: first candidate f4835403bb4cd08e8083ff466dcfa14012483ce1
//! failed linking undeclared verifier crates (exit 101, /out/logs/build-1.log);
//! the independent test verifier retains all signature/inclusion assertions.
//! Task 22.33.1505 repair record; exact qualified revision/command/exit/log are
//! supplied by the private hf-build/hf-done ledger, never guessed in this file.
//! proof-corpus-gap: proof_corpus_gap_encoded_landing_evidence_verifies_after_reopen;
//! proof-corpus-gap: corpus::proof_agent_api_encoded_node_evidence_refuses_tampering.
//! proof-rung-test: proof_rung_test_committed_evidence_satisfies_both_selectors;
//! proof-rung-test: heights::proof_bundle_mirror_requires_receipted_or_committed_rungs.
//! Baseline: the repair target did not exist and no earlier gate ledger existed.
//! No unchanged verification ran: the explicit gate order requires complete
//! implementation before hf-build and hf-done; no baseline failures are claimed.
//! Wiring request, agent domain 9: sdk/platform/conformance/fixtures/vector-v5-portable.json and daemon/committed-fixture/vectors must publish this recorded block's encoded Comet/ABCI evidence and a current type-2 landing transaction; the recorded block uses a legacy EVM envelope and Comet result inclusion, not an Ethereum receipt trie.
//! Wiring request, agent domain 9: services/agent/crates/layerx-agentd/src/read/proof_bundle.rs and services/agent/sdk/typescript/src/committed-proof.ts must share the native comet-abci-results-v1 codec and run the same encoded corpus through daemon acquisition and the language SDK gates.

#[path = "proof_agent_api.rs"]
mod corpus;
#[path = "read_heights.rs"]
mod heights;

use layerx_agent_api::export::{
    check_export_response, validate_export_request, FactRef, OfflineExport,
};
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agent_api::read::{ReadRequest, VerifiedRead};
use layerx_agent_api::verify::Rung;
use std::fs;
use std::io::{Read, Write};

#[test]
fn proof_corpus_gap_encoded_landing_evidence_verifies_after_reopen() {
    let produced = corpus::committed::produced();
    corpus::committed::verify(&produced).expect("independent committed evidence verification");
    let recording = corpus::committed::document();
    let id = recording.hex_array_at("bundle.receiptJson.txHash").unwrap();
    let observed = corpus::committed::round_trip(ProofBundleTarget::Receipt(id), Rung::Committed);
    corpus::committed::verify(&observed).unwrap();
    let facts = vec![FactRef::new(format!(
        "receipt:{}",
        id.iter().map(|b| format!("{b:02x}")).collect::<String>()
    ))
    .unwrap()];
    let request = validate_export_request(ReadRequest {
        selector: facts.clone(),
        requested_rung: Rung::Committed,
    })
    .unwrap();
    let export = OfflineExport {
        facts,
        bundles: vec![observed.value],
        height: observed.freshness.value_height,
    };
    let path = std::env::temp_dir().join(format!("repair-22-33-1505-{}", std::process::id()));
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .unwrap();
    file.write_all(export.encode_json().unwrap().as_bytes())
        .unwrap();
    file.sync_all().unwrap();
    drop(file);
    drop(export);
    drop(produced);
    let mut reopened = fs::File::open(&path).unwrap();
    let mut retained = String::new();
    reopened.read_to_string(&mut retained).unwrap();
    drop(reopened);
    fs::remove_file(&path).unwrap();
    let recovered = OfflineExport::decode_json(&retained).unwrap();
    let response = VerifiedRead::new(
        recovered.clone(),
        Rung::Committed,
        observed.freshness.clone(),
    );
    check_export_response(&request, &response).unwrap();
    assert_eq!(recovered.bundles.len(), 1);
    let recovered = VerifiedRead::new(
        recovered.bundles[0].clone(),
        Rung::Committed,
        observed.freshness,
    );
    assert_eq!(recovered, corpus::committed::produced());
    corpus::committed::verify(&recovered).expect("reopened evidence authenticates independently");
}

#[test]
fn proof_rung_test_committed_evidence_satisfies_both_selectors() {
    let recording = corpus::committed::document();
    let id = recording.hex_array_at("bundle.receiptJson.txHash").unwrap();
    for target in [
        ProofBundleTarget::Activity(id),
        ProofBundleTarget::Receipt(id),
    ] {
        for requested_rung in [Rung::Receipted, Rung::Committed] {
            let response = corpus::committed::round_trip(target, requested_rung);
            assert_eq!(response.achieved_rung, Rung::Committed);
            corpus::committed::verify(&response).unwrap();
            let request = ReadRequest {
                selector: target,
                requested_rung,
            };
            layerx_agent_api::proof::ProofBundle::check_response(&request, &response).unwrap();
            let mut lower = response.clone();
            lower.achieved_rung = Rung::Receipted;
            lower.value.verification_level = Rung::Receipted;
            lower.value.block_header = None;
            lower.value.receipt_inclusion = None;
            lower.value.commit = None;
            lower.value.validator_set = None;
            lower.validate().unwrap();
            let committed = ReadRequest {
                requested_rung: Rung::Committed,
                ..request
            };
            assert!(
                layerx_agent_api::proof::ProofBundle::check_response(&committed, &lower).is_err()
            );
            assert!(corpus::committed::verify(&lower).is_err());
            let mut inconsistent = response;
            inconsistent.achieved_rung = Rung::Receipted;
            assert!(inconsistent.encode_json().is_err());
            assert!(layerx_agent_api::proof::ProofBundle::check_response(
                &committed,
                &inconsistent
            )
            .is_err());
        }
    }
}
