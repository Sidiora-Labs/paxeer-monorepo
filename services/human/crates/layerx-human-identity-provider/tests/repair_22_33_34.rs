#[path = "support/account_head.rs"]
mod account_head;

use std::fs;

use layerx_human_identity_provider::provision_head::validate_account_head;
use serde_json::{json, Value};

use account_head::{must, snapshot_at, write_snapshot, Damage, TranscriptPeer};

const NOW: u64 = 1_700_000_000_000;

fn snapshot(height: u64) -> Value {
    snapshot_at(height, NOW)
}

fn observe(input: &Value, now: u64) -> std::io::Result<Value> {
    let mut output = Vec::new();
    validate_account_head(input.to_string().as_bytes(), &mut output, now)?;
    serde_json::from_slice(&output).map_err(std::io::Error::other)
}

fn assert_refused(input: &Value, now: u64) {
    let mut output = Vec::new();
    assert!(validate_account_head(input.to_string().as_bytes(), &mut output, now).is_err());
    assert!(output.is_empty(), "refusal published a consumption result");
}

#[test]
fn identity_first_batch_provisions_after_restart_at_later_height() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    let mut first = snapshot(17);
    write_snapshot(&store, &first);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    let result = must(observe(&peer.input(), NOW));
    assert_eq!(result["committed_height"], 17);
    assert_eq!(result["consumed"], 0);
    assert_eq!(result["account"], first["owner_account"]);
    assert_eq!(result["owner_did"], first["owner_did"]);
    assert_eq!(result["balance"], "100");
    let calls = must(peer.requests.lock()).clone();
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        5
    );
    drop(peer);
    // Stop the peer and discard its in-memory state, then reopen its retained
    // account at a later committed height. The consumer is reopened per call.
    first["height"] = json!(42);
    first["block"] = snapshot(42)["block"].clone();
    first["account"]["paxeer"]["height"] = json!(42);
    write_snapshot(&store, &first);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    let recovered = must(observe(&peer.input(), NOW));
    assert_eq!(recovered["committed_height"], 42);
    assert_ne!(recovered["block_hash"], result["block_hash"]);
    assert_ne!(recovered["state_root"], result["state_root"]);
    for field in [
        "schema",
        "chain_id",
        "owner_did",
        "account",
        "asset_id",
        "balance",
        "consumed",
    ] {
        assert_eq!(recovered[field], result[field], "recovered {field}");
    }
}

#[test]
fn identity_first_batch_refuses_recovered_spending_history() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    let mut record = snapshot(17);
    record["calls"][4]["result"] = json!(format!("0x{:064x}", 7));
    write_snapshot(&store, &record);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW);
    drop(peer);
    record["height"] = json!(42);
    record["block"] = snapshot(42)["block"].clone();
    write_snapshot(&store, &record);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW);
    let persisted: Value = must(serde_json::from_slice(&must(fs::read(&store))));
    assert_eq!(persisted["calls"][4]["result"], format!("0x{:064x}", 7));
}

#[test]
fn identity_first_batch_preserves_identity_and_canonical_state_refusals() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    for (path, value) in [
        ("/account/account/bound", json!(false)),
        (
            "/account/account/layerx_did",
            json!(format!("did:layerx:{}", "ab".repeat(32))),
        ),
        ("/account/account/layerx_account", json!("ab".repeat(32))),
        (
            "/account/account/evm_address",
            json!(format!("0x{}", "00".repeat(20))),
        ),
        (
            "/account/paxeer/address",
            json!(format!("0x{}", "ab".repeat(20))),
        ),
        ("/calls/0/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/1/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/2/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/3/result", json!(format!("0x{}", "ff".repeat(32)))),
        ("/calls/4/result", json!(format!("0x{}00", "00".repeat(32)))),
        ("/calls/4/result", json!("0x")),
        ("/block/hash", json!(format!("0x{}", "00".repeat(32)))),
        ("/block/stateRoot", json!(format!("0x{}", "00".repeat(32)))),
    ] {
        let mut record = snapshot(17);
        *record.pointer_mut(path).unwrap_or_else(|| panic!("{path}")) = value;
        write_snapshot(&store, &record);
        let peer = TranscriptPeer::reopen(&store, Damage::None);
        assert_refused(&peer.input(), NOW);
    }
    write_snapshot(&store, &snapshot(17));
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    for (field, value) in [
        (
            "owner_did",
            json!(format!("did:layerx:{}", "00".repeat(32))),
        ),
        (
            "owner_did",
            json!(format!("did:layerx:{}", "AB".repeat(32))),
        ),
        ("owner_account", json!("ab".repeat(32))),
        ("asset_id", json!("ab".repeat(32))),
        ("chain_id", json!(126)),
        ("maximum_age_seconds", json!(0)),
        ("maximum_age_seconds", json!(301)),
        ("head", json!({"current":true,"consumed":0})),
    ] {
        let mut input = peer.input();
        input[field] = value;
        assert_refused(&input, NOW);
    }
}

#[test]
fn identity_first_batch_preserves_authenticated_current_head_refusals() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    write_snapshot(&store, &snapshot(17));
    for damage in [
        Damage::RpcError,
        Damage::WrongResponseId,
        Damage::AdvancingHead,
        Damage::ChangedHash,
        Damage::ChangedRoot,
    ] {
        let peer = TranscriptPeer::reopen(&store, damage);
        assert_refused(&peer.input(), NOW);
    }
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW + 60_001);
    assert_refused(&peer.input(), NOW - 1);
    for url in [
        "http://chain.example.test/",
        "https://chain.example.test/",
        "file:///tmp/chain",
    ] {
        let mut input = peer.input();
        input["rpc_url"] = json!(url);
        assert_refused(&input, NOW);
    }
    for input in [
        json!({}),
        json!([]),
        json!({"head":null}),
        json!({"network_id":125,"sequencer_id":"61".repeat(32),"public_key":"61".repeat(32),"head":{"current":true}}),
    ] {
        assert_refused(&input, NOW);
    }
    let mut output = Vec::new();
    assert!(validate_account_head(&vec![b' '; 1_048_577][..], &mut output, NOW).is_err());
    assert!(output.is_empty());
}

#[test]
fn identity_first_batch_hosted_job_observes_after_merged_registration() {
    let script = include_str!("../../../../platform/hosted/human/provision.sh");
    assert!(!script.contains("/v1/protocol/account-state/head"));
    assert!(script.contains("pod['containers'] = [c for c in pod['containers'] if c['name'] != 'validate-account-head']"));
    let native = script
        .split("human_native_provision() (")
        .nth(1)
        .unwrap_or_else(|| panic!("native provisioning"))
        .split_once("\n)\n")
        .unwrap_or_else(|| panic!("native provisioning body"))
        .0;
    let produce = native
        .find("--produce-owner-registration")
        .unwrap_or_else(|| panic!("produce"));
    let validate = native
        .find("--validate-owner-registration")
        .unwrap_or_else(|| panic!("validate"));
    assert!(produce < validate);
    let evidence = script
        .split("human_evidence_provision() (")
        .nth(1)
        .unwrap_or_else(|| panic!("evidence provisioning"))
        .split_once("\n)\n")
        .unwrap_or_else(|| panic!("evidence provisioning body"))
        .0;
    let register = evidence
        .find("human_native_provision\n")
        .unwrap_or_else(|| panic!("register"));
    let fund = evidence
        .find("--funding-evidence")
        .unwrap_or_else(|| panic!("fund"));
    let validate = evidence
        .find("--validate-owner-registration")
        .unwrap_or_else(|| panic!("validate funded registration"));
    let observe = evidence
        .find("human_account_head_observe\n")
        .unwrap_or_else(|| panic!("observe"));
    let naming = evidence
        .find("naming_program_deploy\n")
        .unwrap_or_else(|| panic!("naming"));
    assert!(register < fund && fund < validate && validate < observe && observe < naming);
    assert!(script.contains("pod['containers'] = [validator]"));
    assert!(script.contains("rpc_ca_der_hex=[ssl.PEM_cert_to_DER_cert(ca).hex()]"));
    assert!(script.contains("dict(consumed=result['consumed'])"));
    assert!(script.contains("result['account'] == request['owner_account']"));
}
