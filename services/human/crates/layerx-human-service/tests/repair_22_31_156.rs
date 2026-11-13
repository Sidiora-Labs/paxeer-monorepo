use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agentd::events::{CoreEvent, EventAttributes};
use layerx_human_service::activity::{
    verify_receipt_export, verify_receipt_export_with_consensus, ActivityKind, AgentActivity,
    EvidenceBundle, EvidenceExport, ExportError, Feed, FeedError, FilterDraft, PageRequest,
    PendingStatus, ReceiptExport, ReceiptFact, ReceiptHeaderBinding, UnverifiedReason,
    VerifiedStatus, VerifyError,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_service::store::{PrincipalStore, RowKey, Table};
use layerx_human_test_support as support;
use layerx_intents::vectors::{batch_header, batch_header_signing_digest};
use layerx_proof::export::{InclusionFact, InclusionKind};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::{build_proof, Proof};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

#[path = "support/committed_receipt.rs"]
mod committed_receipt;
use committed_receipt::{fixed, recorded_receipt, RECORDED};

const LOGS: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../agent/crates/layerx-agentd/tests/fixtures/receipted-log-transition.json"
));

fn meaning(id: ActivityEntryId) -> AgentActivity {
    AgentActivity::new(
        id,
        ActivityKind::AgentAction,
        None,
        10,
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .or_fail("activity meaning")
}
fn source_logs() -> Vec<EventDelivery> {
    let fixture: Value = serde_json::from_str(LOGS).or_fail("recorded RPC filter log output");
    fixture["receipted"]
        .as_array()
        .or_fail("logs")
        .iter()
        .map(|log| {
            let source = CoreEvent::from_log(
                log,
                EventAttributes {
                    agent: "did:layerx:agent-a".to_owned(),
                    account: "account-a".to_owned(),
                    activity_type: 1,
                    module: "evm".to_owned(),
                    asset: "LXR".to_owned(),
                    counterparty: "counterparty-a".to_owned(),
                    result_code: 0,
                },
            )
            .or_fail("real agent log codec");
            let position = source.position().or_fail("producer's complete position");
            EventDelivery::new(
                EventIdentity::new(Sha256::digest(&source.canonical_bytes).into()),
                source.canonical_bytes,
                Cursor {
                    block_height: position.block_height,
                    tx_index: position.tx_index,
                    log_index: position.log_index,
                },
                ReceiptReference::None,
            )
            .or_fail("real delivery contract")
        })
        .collect()
}
fn receipt_delivery(fact: &ReceiptFact) -> EventDelivery {
    EventDelivery::new(
        EventIdentity::new(fact.receipt_hash),
        fact.canonical_receipt_bytes.clone(),
        Cursor {
            block_height: fact.block_height,
            tx_index: 1,
            log_index: 0,
        },
        ReceiptReference::Verified {
            receipt_ref: ReceiptRef::new(hex(&fact.receipt_hash)).or_fail("receipt ref"),
            rung: fact.rung,
        },
    )
    .or_fail("producer receipt delivery")
}
fn hex(bytes: &[u8]) -> String {
    layerx_human_service::hex::lower(bytes)
}
fn artifact(fact: ReceiptFact) -> ReceiptExport {
    ReceiptExport {
        receipts: vec![fact],
        inclusions: Vec::new(),
        derived_aggregates: Vec::new(),
    }
}

#[test]
fn finding4_full_cursor_same_block_duplicates_and_reopen() {
    let root = support::directory("repair2231156-full-cursor");
    let retention = support::retention_uniform(10000);
    let (mut store, digest) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        retention,
    );
    let principal = support::principal("alice");
    let deliveries = source_logs();
    assert_eq!(
        deliveries[0].cursor.block_height,
        deliveries[3].cursor.block_height
    );
    assert!(
        deliveries[0].cursor < deliveries[1].cursor && deliveries[1].cursor < deliveries[2].cursor
    );
    let descriptors: Vec<_> = (0..4)
        .map(|i| meaning(ActivityEntryId::new(format!("act_cursor{i}")).or_fail("entry id")))
        .collect();
    {
        let mut scope = store.principal(&principal).or_fail("scope");
        let first = Feed::record_agent_event(&mut scope, &descriptors[0], &deliveries[0], 100)
            .or_fail("first log");
        assert_eq!(
            first,
            Feed::record_agent_event(&mut scope, &descriptors[0], &deliveries[0], 99)
                .or_fail("idempotent old delivery")
        );
        Feed::record_agent_event(&mut scope, &descriptors[1], &deliveries[1], 101)
            .or_fail("second log in same transaction");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).or_fail("resume"),
            Some(deliveries[1].cursor)
        );
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("reopen actual store");
    {
        let mut scope = store.principal(&principal).or_fail("reopened scope");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).or_fail("durable full cursor"),
            Some(deliveries[1].cursor)
        );
        Feed::record_agent_event(&mut scope, &descriptors[1], &deliveries[1], 101)
            .or_fail("replayed after restart");
        for i in 2..4 {
            Feed::record_agent_event(&mut scope, &descriptors[i], &deliveries[i], 100 + i as u64)
                .unwrap_or_else(|error| {
                    panic!(
                        "{}: {error:?}",
                        "remaining same-block transactions and logs"
                    )
                });
        }
        let mut collision = deliveries[3].clone();
        collision.event_bytes.push(1);
        assert!(matches!(
            Feed::record_agent_event(&mut scope, &descriptors[3], &collision, 104),
            Err(FeedError::SourceCollision)
        ));
        let page = Feed::new(5)
            .or_fail("feed")
            .page(
                &scope,
                PageRequest::new(
                    10,
                    Feed::apply_filters(FilterDraft::new()).or_fail("filters"),
                ),
                104,
                deliveries[3].cursor.block_height,
            )
            .or_fail("page");
        assert_eq!(page.entries().len(), 4);
        assert_eq!(
            Feed::projected_agent_cursor(&scope).or_fail("resume"),
            Some(deliveries[3].cursor)
        );
    }
    drop(store);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

#[test]
fn finding4_filtered_height_jump_and_unseen_regression() {
    let root = support::directory("repair2231156-height-gap");
    let (mut store, _) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        support::retention_uniform(10000),
    );
    let principal = support::principal("alice");
    let (id, fact, _, _) = recorded_receipt();
    let logs = source_logs();
    let mut scope = store.principal(&principal).or_fail("scope");
    Feed::record_agent_event(&mut scope, &meaning(id), &receipt_delivery(&fact), 100)
        .or_fail("earlier real producer block");
    assert!(logs[0].cursor.block_height > fact.block_height + 1);
    let later = meaning(ActivityEntryId::new("act_gaplater").or_fail("id"));
    Feed::record_agent_event(&mut scope, &later, &logs[2], 101).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "filtered and empty heights need no synthetic events"
        )
    });
    assert!(matches!(
        Feed::record_agent_event(&mut scope, &later, &logs[1], 102),
        Err(FeedError::AgentEventGap)
    ));
    assert_eq!(
        Feed::projected_agent_cursor(&scope).or_fail("cursor"),
        Some(logs[2].cursor)
    );
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

#[test]
fn finding4_legacy_height_migration_replays_the_last_block() {
    let root = support::directory("repair2231156-legacy");
    let retention = support::retention_uniform(10000);
    let (mut store, digest) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        retention,
    );
    let principal = support::principal("alice");
    let logs = source_logs();
    let first = meaning(ActivityEntryId::new("act_legacyfirst").or_fail("id"));
    {
        let mut scope = store.principal(&principal).or_fail("scope");
        Feed::record_agent_event(&mut scope, &first, &logs[0], 100).or_fail("first projected log");
        let key = RowKey::new("activity-feed").or_fail("key");
        let mut state: Value =
            serde_json::from_slice(scope.get(Table::Journeys, &key).or_fail("row").bytes())
                .or_fail("state");
        state["version"] = json!(1);
        state["last_agent_cursor"] = json!(logs[0].cursor.block_height);
        scope
            .put(
                Table::Journeys,
                key,
                100,
                serde_json::to_vec(&state).or_fail("legacy encoding"),
            )
            .or_fail("old schema");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("reopen legacy store");
    {
        let mut scope = store.principal(&principal).or_fail("scope");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).or_fail("safe migration resume"),
            Some(Cursor {
                block_height: logs[0].cursor.block_height - 1,
                tx_index: u32::MAX,
                log_index: u32::MAX
            })
        );
        Feed::record_agent_event(&mut scope, &first, &logs[0], 100)
            .or_fail("identity dedup on legacy replay");
        Feed::record_agent_event(
            &mut scope,
            &meaning(ActivityEntryId::new("act_legacysecond").or_fail("id")),
            &logs[1],
            101,
        )
        .or_fail("no same-block event lost in migration");
        let key = RowKey::new("activity-feed").or_fail("key");
        let state: Value =
            serde_json::from_slice(scope.get(Table::Journeys, &key).or_fail("row").bytes())
                .or_fail("new state");
        assert_eq!(state["version"], 2);
        assert_eq!(
            state["last_agent_cursor"]["tx_index"],
            logs[1].cursor.tx_index
        );
        assert_eq!(
            state["last_agent_cursor"]["log_index"],
            logs[1].cursor.log_index
        );
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("reopen migrated state");
    assert_eq!(
        Feed::projected_agent_cursor(&store.principal(&principal).or_fail("scope"))
            .or_fail("full position"),
        Some(logs[1].cursor)
    );
    drop(store);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

#[test]
fn finding5_rung_only_mutation_cannot_upgrade_a_signed_receipt() {
    let (id, mut fact, keys, consensus) = recorded_receipt();
    let principal = support::principal("alice");
    fact.committed = None;
    fact.header_binding = None;
    fact.rung = Rung::Receipted;
    EvidenceBundle::verify_receipt(&fact, &keys).or_fail("a real signature establishes Receipted");
    let valid = EvidenceBundle::receipt(id.clone(), fact.clone(), &principal).or_fail("bundle");
    valid
        .verify(valid.digest().or_fail("digest"), &principal, &keys)
        .or_fail("receipted export");
    fact.rung = Rung::Committed;
    let changed = EvidenceBundle::receipt(id, fact, &principal).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "artifact can relabel both entry and receipt"
        )
    });
    assert_ne!(
        valid.digest().or_fail("old digest"),
        changed.digest().or_fail("attacker recomputes digest")
    );
    assert!(matches!(
        changed.verify_with_consensus(
            changed.digest().or_fail("new digest"),
            &principal,
            &keys,
            &consensus
        ),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
}

fn assert_committed_tampering_refused(
    id: &ActivityEntryId,
    fact: &ReceiptFact,
    principal: &layerx_human_service::store::PrincipalId,
    keys: &[layerx_client::sequencer::ReceiptKeyEntry],
    consensus: &[layerx_human_service::activity::ConsensusAuthority],
) {
    // All mutations keep the receipt signature and recompute the artifact digest.
    for target in [
        "batch",
        "aunt",
        "range",
        "activity",
        "commit",
        "header",
        "execution",
        "total",
    ] {
        let mut tampered = (*fact).clone();
        let proof = tampered.committed.as_mut().or_fail("commit evidence");
        match target {
            "batch" => proof.batch_transaction[40] ^= 1,
            "aunt" => proof.batch_aunts[0][0] ^= 1,
            "range" => proof.batch_transaction[30] ^= 1,
            "activity" => proof.activity_ids[0][0] ^= 1,
            "commit" => proof.commit.votes[0].signature[0] ^= 1,
            "header" => {
                proof.header.data_hash.as_mut().or_fail("root")[0] ^= 1;
            }
            "execution" => {
                proof.execution_header.data_hash.as_mut().or_fail("root")[0] ^= 1;
            }
            "total" => proof.batch_total += 1,
            _ => unreachable!(),
        }
        let bundle =
            EvidenceBundle::receipt((*id).clone(), tampered, principal).or_fail("tampered codec");
        assert!(
            bundle
                .verify_with_consensus(
                    bundle.digest().or_fail("new digest"),
                    principal,
                    keys,
                    consensus
                )
                .is_err(),
            "accepted modified {target}"
        );
    }
    let mut wrong = consensus.to_vec();
    wrong[0].validators[0].voting_power += 1;
    assert!(EvidenceBundle::verify_receipt_with_consensus(fact, keys, &wrong).is_err());
    let mut downgraded = (*fact).clone();
    downgraded.rung = Rung::Receipted;
    assert!(matches!(
        EvidenceBundle::verify_receipt_with_consensus(&downgraded, keys, consensus),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
}

#[test]
fn finding5_real_committed_evidence_export_decode_reopen_and_tamper() {
    let (id, fact, keys, consensus) = recorded_receipt();
    let root = support::directory("repair2231156-committed-export");
    let retention = support::retention_uniform(10000);
    let (mut store, tenancy) = support::install_and_open(
        &root,
        &support::tenancy(&[("alice", "tenant-a")]),
        retention,
    );
    let principal = support::principal("alice");
    let exported = {
        let mut scope = store.principal(&principal).or_fail("scope");
        Feed::record_agent_event(
            &mut scope,
            &meaning(id.clone()),
            &receipt_delivery(&fact),
            100,
        )
        .or_fail("project actual receipt");
        let (bundle, report) = EvidenceExport::new(Feed::new(5).or_fail("feed"), 1024 * 1024)
            .or_fail("exporter")
            .evidence_with_consensus(
                &scope,
                &Feed::apply_filters(FilterDraft::new()).or_fail("filters"),
                std::slice::from_ref(&id),
                vec![artifact(fact.clone())],
                &keys,
                &consensus,
                100,
                fact.block_height,
            )
            .or_fail("verify real producer evidence at export");
        assert_eq!(report.verified_receipts(), 1);
        assert_eq!(report.verified_inclusions(), 1);
        let bytes = bundle.encode().or_fail("v3 evidence codec");
        assert_eq!(&bytes[..9], b"LXHEXPV3\x03");
        scope
            .put(
                Table::Cache,
                RowKey::new("committed-export").or_fail("key"),
                100,
                bytes.clone(),
            )
            .or_fail("persist exact export");
        bytes
    };
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, tenancy).or_fail("restart actual store");
    {
        let scope = store.principal(&principal).or_fail("scope");
        let row = scope
            .get(
                Table::Cache,
                &RowKey::new("committed-export").or_fail("key"),
            )
            .or_fail("recovered export");
        assert_eq!(row.bytes(), exported);
        let decoded = EvidenceBundle::decode(row.bytes()).or_fail("decode recovered evidence");
        assert_eq!(decoded.protocol_evidence()[0].receipts[0], fact);
        assert_eq!(decoded.entries()[0].rung(), Rung::Committed);
        let report = decoded
            .verify_with_consensus(
                decoded.digest().or_fail("digest"),
                &principal,
                &keys,
                &consensus,
            )
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "portable committed verification after restart"
                )
            });
        assert_eq!(report.verified_receipts(), 1);
        assert_eq!(report.verified_inclusions(), 1);
        assert!(matches!(
            decoded.verify(decoded.digest().or_fail("digest"), &principal, &keys),
            Err(VerifyError::Unavailable(
                ExportError::AuthorityUnavailable { .. }
            ))
        ));
    }
    assert_committed_tampering_refused(&id, &fact, &principal, &keys, &consensus);
    drop(store);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

#[test]
fn finding6_real_producer_receipt_header_inclusion_roundtrip() {
    let (id, mut receipt, keys, _) = recorded_receipt();
    receipt.rung = Rung::Receipted;
    receipt.committed = None;
    let export = artifact(receipt.clone());
    let report = verify_receipt_export(&export, &keys).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "node-produced receipt and exact signed inclusion header"
        )
    });
    assert_eq!(report.verified_receipts, 1);
    assert_eq!(report.verified_inclusions, 1);
    let principal = support::principal("alice");
    let bundle = EvidenceBundle::receipt(id, receipt.clone(), &principal).or_fail("bundle");
    let encoded = bundle.encode().or_fail("transport encode");
    let decoded = EvidenceBundle::decode(&encoded).or_fail("transport decode");
    assert_eq!(decoded.encode().or_fail("canonical reencode"), encoded);
    assert_eq!(
        decoded
            .verify(decoded.digest().or_fail("digest"), &principal, &keys)
            .or_fail("pinned inclusion survives transport")
            .verified_inclusions(),
        1
    );
    let mut expired = keys.clone();
    expired[0].last_height = receipt.block_height - 1;
    assert!(verify_receipt_export(&export, &expired).is_err());
    assert!(verify_receipt_export(&export, &[]).is_err());
    let mut substituted = receipt.clone();
    substituted
        .header_binding
        .as_mut()
        .or_fail("binding")
        .receipt_proof = Proof::new(
        1,
        2,
        receipt
            .header_binding
            .as_ref()
            .or_fail("binding")
            .receipt_proof
            .siblings()
            .to_vec(),
    )
    .or_fail("other receipt slot");
    assert!(verify_receipt_export(&artifact(substituted), &keys).is_err());
    let raw: Value = serde_json::from_str(RECORDED).or_fail("recorded producer refusions");
    let mut foreign = receipt;
    foreign
        .header_binding
        .as_mut()
        .or_fail("binding")
        .header_signature = fixed(&raw["refusals"][1]["header_signature"]);
    assert!(verify_receipt_export(&artifact(foreign), &keys).is_err());
}

#[test]
fn finding6_artifact_only_keys_and_unrelated_roots_are_refused() {
    let (_, mut receipt, keys, _) = recorded_receipt();
    receipt.committed = None;
    receipt.rung = Rung::Receipted;
    // The attack uses real Merkle and header codecs and a cryptographically
    // valid attacker signature, while retaining the genuine selected receipt.
    for kind in [InclusionKind::Activity, InclusionKind::State] {
        let leaf = b"unrelated exported tree".to_vec();
        let (proof, root) = build_proof(&[leaf.as_slice()], 0).or_fail("real tree producer");
        let signer = SigningKey::from_bytes(&[91; 32]);
        let public = signer.verifying_key().to_bytes();
        let h = batch_header(root, root, public).or_fail("real canonical header codec");
        let signature = signer
            .sign(&batch_header_signing_digest(&h).or_fail("header digest"))
            .to_bytes();
        let inclusion = InclusionFact {
            statement: "attacker-selected root".to_owned(),
            kind,
            canonical_leaf_bytes: leaf,
            proof,
            named_root: root,
            canonical_header_bytes: h,
            header_signature: signature,
            sequencer_authorization: SequencerAuthorization::new(public, public, 1, 100),
        };
        let mut attack = artifact(receipt.clone());
        attack.inclusions.push(inclusion);
        assert!(matches!(
            verify_receipt_export(&attack, &keys),
            Err(ExportError::UnboundProtocolEvidence)
        ));
        // Supplying the substituted header as the selected receipt binding
        // cannot authenticate it, even if the artifact supplies its key.
        let bad = &attack.inclusions[0];
        attack.receipts[0].header_binding = Some(ReceiptHeaderBinding {
            canonical_header_bytes: bad.canonical_header_bytes.clone(),
            header_signature: bad.header_signature,
            receipt_proof: attack.receipts[0]
                .header_binding
                .as_ref()
                .or_fail("real original binding")
                .receipt_proof
                .clone(),
        });
        assert!(verify_receipt_export(&attack, &keys).is_err());
    }
    let binding = receipt.header_binding.as_ref().or_fail("real binding");
    let header = layerx_wire::receipt::decode_batch_header(&binding.canonical_header_bytes)
        .or_fail("header codec");
    let (proof, _) = build_proof(&[receipt.tx_hash.as_slice()], 0).or_fail("proof codec");
    let foreign = SigningKey::from_bytes(&[93; 32]).verifying_key().to_bytes();
    let mut substituted_authority = artifact(receipt.clone());
    substituted_authority.inclusions.push(InclusionFact {
        statement: "artifact key attached to exact selected header".to_owned(),
        kind: InclusionKind::Activity,
        canonical_leaf_bytes: receipt.tx_hash.to_vec(),
        proof,
        named_root: header.activity_merkle_root(),
        canonical_header_bytes: binding.canonical_header_bytes.clone(),
        header_signature: binding.header_signature,
        sequencer_authorization: SequencerAuthorization::new(
            header.sequencer_id(),
            foreign,
            keys[0].first_height,
            keys[0].last_height,
        ),
    });
    assert!(matches!(
        verify_receipt_export(&substituted_authority, &keys),
        Err(ExportError::Unverified(UnverifiedReason::AuthorityMismatch))
    ));
    let (id, committed, keys, consensus) = recorded_receipt();
    let report =
        verify_receipt_export_with_consensus(&artifact(committed.clone()), &keys, &consensus)
            .unwrap_or_else(|error| {
                panic!("{}: {error:?}", "real committed receipt/header linkage")
            });
    assert_eq!(
        report.achieved_levels,
        [layerx_types::verify::VerificationLevel::COMMITTED]
    );
    let mut wrong = keys;
    wrong[0].public_key = SigningKey::from_bytes(&[92; 32]).verifying_key().to_bytes();
    let bundle =
        EvidenceBundle::receipt(id, committed, &support::principal("alice")).or_fail("bundle");
    assert!(bundle
        .verify_with_consensus(
            bundle.digest().or_fail("digest"),
            &support::principal("alice"),
            &wrong,
            &consensus
        )
        .is_err());
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
