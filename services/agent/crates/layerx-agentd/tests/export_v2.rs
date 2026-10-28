#[path = "export.rs"]
mod fixtures;

use fixtures::{canonical, must, Fixture};
use layerx_agent_api::export::{check_export_response, FactRef, OfflineExport};
use layerx_agent_api::read::ReadRequest;
use layerx_agentd::export::{
    build, decode_validators, deserialize_export, produce_retained, retain_bundle,
    serialize_export, verify, ExportProduceError, ExportTrustSource, MAX_EXPORT_BYTES,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_types::rung::Rung;
use std::sync::Mutex;

#[test]
fn offline_export_verifies_with_only_receipt_keys_and_validator_set() {
    for success in [true, false] {
        let fixture = Fixture::new(8, success);
        let encoded = must(serialize_export(
            &must(build(fixture.artifact.clone(), &fixture.trust)).artifact,
        ));
        let offline = must(deserialize_export(&encoded));
        let receipt_keys = offline
            .bundles
            .iter()
            .map(|bundle| bundle.receipt_key)
            .collect();
        let validators = must(decode_validators(
            offline.bundles[0]
                .validator_set
                .as_ref()
                .unwrap()
                .as_bytes(),
        ));
        let trust = must(ExportTrustSource::new(receipt_keys, validators));
        let verified = must(verify(&offline, &trust));
        assert_eq!(verified.verified_receipts, 1);
        assert_eq!(verified.verified_inclusions, 1);
        assert_eq!(verified.verified_checkpoints, 1);
        assert_eq!(verified.height, 8);
        assert_eq!(verified.block_hash, fixture.header.block_hash());
        assert_eq!(verified.achieved_rung, Rung::Committed);
        assert_eq!(offline, fixture.artifact);

        let mut altered = offline.clone();
        let mut bytes = altered.bundles[0].signed_receipt.as_bytes().to_vec();
        let end = bytes.len() - 1;
        bytes[end] ^= 1;
        altered.bundles[0].signed_receipt = canonical(bytes);
        assert!(verify(&altered, &trust).is_err());
        let mut altered = offline.clone();
        altered.bundles[0].raw_transaction = canonical(vec![2, 1]);
        assert!(verify(&altered, &trust).is_err());
        let mut keys = trust.receipt_keys().to_vec();
        keys[0].first_height = 9;
        let wrong_interval = must(ExportTrustSource::new(keys, trust.validator_set().to_vec()));
        assert!(verify(&offline, &wrong_interval).is_err());
        let mut changed = offline.clone();
        changed.bundles[0].commit = None;
        assert!(verify(&changed, &trust).is_err());
    }

    let fixture = Fixture::new(8, true);
    let directory = std::env::temp_dir().join(format!("layerx-export-v2-{}", std::process::id()));
    let tenant = must(TenantId::new("export-owner"));
    let other = must(TenantId::new("other-owner"));
    let mut store = must(Store::open(&directory));
    must(retain_bundle(
        &mut store,
        tenant.clone(),
        fixture.artifact.clone(),
        &fixture.trust,
    ));
    must(retain_bundle(
        &mut store,
        tenant.clone(),
        fixture.state_artifact(),
        &fixture.trust,
    ));
    let mutex = Mutex::new(store);
    let produced = must(produce_retained(
        &mutex,
        &tenant,
        None,
        &fixture.trust,
        &fixture.artifact.facts,
    ));
    assert_eq!(produced.response.achieved_rung, Rung::Committed);
    assert_eq!(produced.response.freshness.value_height, 8);
    assert_eq!(
        produced.json_value_bytes,
        must(serialize_export(&produced.response.value)).len()
    );
    assert!(produced.binary_bytes > 0 && produced.binary_bytes < produced.json_value_bytes);
    must(check_export_response(
        &ReadRequest {
            selector: fixture.artifact.facts.clone(),
            requested_rung: Rung::Committed,
        },
        &produced.response,
    ));
    assert!(matches!(
        produce_retained(
            &mutex,
            &other,
            None,
            &fixture.trust,
            &fixture.artifact.facts
        ),
        Err(ExportProduceError::NotOwned { fact: 0 })
    ));
    let state = fixture.state_artifact();
    assert!(matches!(
        produce_retained(&mutex, &tenant, None, &fixture.trust, &state.facts),
        Err(ExportProduceError::AccountNotBound { fact: 0 })
    ));
    assert_eq!(
        must(produce_retained(
            &mutex,
            &tenant,
            Some(fixture.account_id),
            &fixture.trust,
            &state.facts
        ))
        .response
        .value,
        state
    );
    let later = Fixture::new(9, true);
    must(retain_bundle(
        &mut mutex.lock().unwrap(),
        tenant.clone(),
        later.artifact.clone(),
        &later.trust,
    ));
    let facts = [
        fixture.artifact.facts[0].clone(),
        later.artifact.facts[0].clone(),
    ];
    assert!(matches!(
        produce_retained(&mutex, &tenant, None, &fixture.trust, &facts),
        Err(ExportProduceError::MixedSnapshot)
    ));
    drop(mutex);
    must(std::fs::remove_dir_all(directory));
}

#[test]
fn offline_export_names_no_settlement_domain() {
    let fixture = Fixture::new(8, true);
    let built = must(build(fixture.artifact.clone(), &fixture.trust));
    let bytes = must(serialize_export(&built.artifact));
    let value: serde_json::Value = must(serde_json::from_slice(&bytes));
    let object = value.as_object().unwrap();
    assert_eq!(object.len(), 3);
    assert!(
        object.contains_key("facts")
            && object.contains_key("bundles")
            && object.contains_key("height")
    );
    fn check_names(value: &serde_json::Value) {
        match value {
            serde_json::Value::Object(fields) => {
                for (name, value) in fields {
                    let lower = name.to_ascii_lowercase();
                    assert!(
                        !lower.contains("settlement") && !lower.contains("certificate"),
                        "unexpected field {name}"
                    );
                    check_names(value);
                }
            }
            serde_json::Value::Array(values) => {
                for value in values {
                    check_names(value);
                }
            }
            _ => {}
        }
    }
    check_names(&value);
    assert_eq!(value["bundles"][0]["verificationLevel"], "committed");
    assert!(value["bundles"][0]["receiptKey"].is_object());
    assert!(value["bundles"][0]["validatorSet"].is_string());
    assert_eq!(must(deserialize_export(&bytes)), built.artifact);
    let mut extra = value.clone();
    extra["settlementDomain"] = serde_json::json!({});
    assert!(deserialize_export(&must(serde_json::to_vec(&extra))).is_err());
    let mut extra = value.clone();
    extra["bundles"][0]["certificate"] = serde_json::json!("0x01");
    assert!(deserialize_export(&must(serde_json::to_vec(&extra))).is_err());
    let empty = OfflineExport {
        facts: Vec::new(),
        bundles: Vec::new(),
        height: 8,
    };
    assert!(build(empty, &fixture.trust).is_err());
    let mut orphan = built.artifact.clone();
    orphan.facts = vec![must(FactRef::new(format!("receipt:{}", "99".repeat(32))))];
    assert!(build(orphan, &fixture.trust).is_err());
    let mut large = built.artifact;
    large.bundles[0].raw_transaction = canonical(vec![1; MAX_EXPORT_BYTES]);
    assert!(build(large, &fixture.trust).is_err());
    assert!(layerx_agentd::export::require_frame_within_bound(MAX_EXPORT_BYTES).is_ok());
    assert!(matches!(
        layerx_agentd::export::require_frame_within_bound(MAX_EXPORT_BYTES + 1),
        Err(ExportProduceError::Oversize)
    ));
}
