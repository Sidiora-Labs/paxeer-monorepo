use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agentd::events::{CoreEvent, EventAttributes};
use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_human_service::activity::{
    verify_receipt_export, verify_receipt_export_with_consensus, ActivityKind, AgentActivity,
    CommittedReceiptEvidence, ConsensusAuthority, EvidenceBundle, EvidenceExport, ExportError,
    Feed, FeedError, FilterDraft, PageRequest, PendingStatus, ReceiptExport, ReceiptFact,
    ReceiptHeaderBinding, UnverifiedReason, VerifiedStatus, VerifyError,
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

const COMMITTED: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../interop/specs/conformance/committed/vector-v4-committed.json"
));
const RECORDED: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../daemon/committed-fixture/vectors/vector-v4-committed.json"
));
const LOGS: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../agent/crates/layerx-agentd/tests/fixtures/receipted-log-transition.json"
));

fn bytes(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .expect("hex string")
        .strip_prefix("0x")
        .expect("0x prefix");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().expect("fixed-width field")
}
fn time(value: &Value) -> CommitTimestamp {
    CommitTimestamp {
        seconds: value["seconds"].as_i64().expect("seconds"),
        nanos: value["nanos"]
            .as_u64()
            .expect("nanos")
            .try_into()
            .expect("u32 nanos"),
    }
}
fn block_id(value: &Value) -> BlockId {
    BlockId {
        hash: fixed(&value["hash"]),
        part_set_total: value["parts"]["total"]
            .as_u64()
            .expect("parts")
            .try_into()
            .expect("u32 parts"),
        part_set_hash: fixed(&value["parts"]["hash"]),
    }
}
fn header(v: &Value) -> CheckpointHeader {
    let optional = |key: &str| {
        if bytes(&v[key]).is_empty() {
            None
        } else {
            Some(fixed(&v[key]))
        }
    };
    CheckpointHeader {
        version_block: v["version"]["block"].as_u64().expect("version"),
        version_app: v["version"]["app"].as_u64().expect("app version"),
        chain_id: v["chain_id"].as_str().expect("chain id").to_owned(),
        height: v["height"].as_u64().expect("height"),
        time: time(&v["time"]),
        last_block_id: Some(block_id(&v["last_block_id"])),
        last_commit_hash: optional("last_commit_hash"),
        data_hash: optional("data_hash"),
        validators_hash: fixed(&v["validators_hash"]),
        next_validators_hash: fixed(&v["next_validators_hash"]),
        consensus_hash: fixed(&v["consensus_hash"]),
        app_hash: fixed(&v["app_hash"]),
        last_results_hash: optional("last_results_hash"),
        evidence_hash: optional("evidence_hash"),
        proposer_address: fixed(&v["proposer_address"]),
    }
}

fn recorded_receipt() -> (
    ActivityEntryId,
    ReceiptFact,
    Vec<ReceiptKeyEntry>,
    Vec<ConsensusAuthority>,
) {
    let v: Value = serde_json::from_str(COMMITTED).expect("Go node conformance recording");
    let c = &v["cases"][0];
    let raw: Value = serde_json::from_str(RECORDED).expect("original Go recorder output");
    // Pin the recording's provenance and retain the actual tx-zero bytes.
    assert_eq!(c["signedReceipt"], raw["positive"]["receipt"]);
    assert_eq!(
        bytes(&c["batchProof"]["data"]),
        STANDARD
            .decode(
                raw["settlement"]["sequence_batch_proof"]["data"]
                    .as_str()
                    .expect("batch base64")
            )
            .expect("recorded batch")
    );
    let key = &c["pinnedKey"];
    let keys = vec![ReceiptKeyEntry {
        key_id: fixed(&key["keyId"]),
        public_key: fixed(&key["publicKey"]),
        first_height: key["firstHeight"].as_u64().expect("first"),
        last_height: key["lastHeight"].as_u64().expect("last"),
    }];
    let validators: Vec<_> = c["validators"]
        .as_array()
        .expect("validators")
        .iter()
        .map(|v| {
            let public_key = fixed(&v["pub_key"]);
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: v["voting_power"].as_u64().expect("power"),
            }
        })
        .collect();
    let h = header(&c["header"]);
    let commit = &c["commit"];
    let votes = commit["signatures"]
        .as_array()
        .expect("votes")
        .iter()
        .map(|vote| {
            assert_eq!(vote["block_id_flag"], 2);
            let address = fixed(&vote["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: validators
                    .iter()
                    .find(|v| v.address == address)
                    .expect("pinned validator")
                    .voting_power,
                timestamp: time(&vote["timestamp"]),
                signature: fixed(&vote["signature"]),
            }
        })
        .collect();
    let id = ActivityEntryId::new("act_repair2231156").expect("entry");
    let mut fact = EvidenceBundle::receipt_fact(&id, bytes(&c["signedReceipt"]), Rung::Committed)
        .expect("real receipt codec");
    fact.committed = Some(CommittedReceiptEvidence {
        header: h.clone(),
        execution_header: header(&c["executionHeader"]),
        commit: BlockCommit {
            height: commit["height"].as_u64().expect("commit height"),
            round: commit["round"]
                .as_u64()
                .expect("round")
                .try_into()
                .expect("round u32"),
            block_id: block_id(&commit["block_id"]),
            votes,
        },
        batch_transaction: bytes(&c["batchProof"]["data"]),
        batch_total: c["batchProof"]["total"].as_u64().expect("total"),
        batch_aunts: c["batchProof"]["aunts"]
            .as_array()
            .expect("aunts")
            .iter()
            .map(fixed)
            .collect(),
        activity_ids: vec![fact.tx_hash],
    });
    let binding = &v["positive"];
    fact.header_binding = Some(ReceiptHeaderBinding {
        canonical_header_bytes: bytes(&binding["header"]),
        header_signature: fixed(&binding["header_signature"]),
        receipt_proof: Proof::new(
            binding["proof"]["LeafIndex"]
                .as_u64()
                .expect("leaf index")
                .try_into()
                .expect("index u32"),
            binding["proof"]["LeafCount"]
                .as_u64()
                .expect("count")
                .try_into()
                .expect("count u32"),
            binding["proof"]["Siblings"]
                .as_array()
                .expect("siblings")
                .iter()
                .map(|s| serde_json::from_value(s.clone()).expect("sibling digest"))
                .collect(),
        )
        .expect("real receipt Merkle proof"),
    });
    (
        id,
        fact,
        keys,
        vec![ConsensusAuthority {
            chain_id: h.chain_id,
            first_height: h.height,
            last_height: h.height,
            validators,
        }],
    )
}

fn meaning(id: ActivityEntryId) -> AgentActivity {
    AgentActivity::new(
        id,
        ActivityKind::AgentAction,
        None,
        10,
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .expect("activity meaning")
}
fn source_logs() -> Vec<EventDelivery> {
    let fixture: Value = serde_json::from_str(LOGS).expect("recorded RPC filter log output");
    fixture["receipted"]
        .as_array()
        .expect("logs")
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
            .expect("real agent log codec");
            let position = source.position().expect("producer's complete position");
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
            .expect("real delivery contract")
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
            receipt_ref: ReceiptRef::new(hex(&fact.receipt_hash)).expect("receipt ref"),
            rung: fact.rung,
        },
    )
    .expect("producer receipt delivery")
}
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
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
        .map(|i| meaning(ActivityEntryId::new(format!("act_cursor{i}")).expect("entry id")))
        .collect();
    {
        let mut scope = store.principal(&principal).expect("scope");
        let first = Feed::record_agent_event(&mut scope, &descriptors[0], &deliveries[0], 100)
            .expect("first log");
        assert_eq!(
            first,
            Feed::record_agent_event(&mut scope, &descriptors[0], &deliveries[0], 99)
                .expect("idempotent old delivery")
        );
        Feed::record_agent_event(&mut scope, &descriptors[1], &deliveries[1], 101)
            .expect("second log in same transaction");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).expect("resume"),
            Some(deliveries[1].cursor)
        );
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen actual store");
    {
        let mut scope = store.principal(&principal).expect("reopened scope");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).expect("durable full cursor"),
            Some(deliveries[1].cursor)
        );
        Feed::record_agent_event(&mut scope, &descriptors[1], &deliveries[1], 101)
            .expect("replayed after restart");
        for i in 2..4 {
            Feed::record_agent_event(&mut scope, &descriptors[i], &deliveries[i], 100 + i as u64)
                .expect("remaining same-block transactions and logs");
        }
        let mut collision = deliveries[3].clone();
        collision.event_bytes.push(1);
        assert!(matches!(
            Feed::record_agent_event(&mut scope, &descriptors[3], &collision, 104),
            Err(FeedError::SourceCollision)
        ));
        let page = Feed::new(5)
            .expect("feed")
            .page(
                &scope,
                PageRequest::new(
                    10,
                    Feed::apply_filters(FilterDraft::new()).expect("filters"),
                ),
                104,
                deliveries[3].cursor.block_height,
            )
            .expect("page");
        assert_eq!(page.entries().len(), 4);
        assert_eq!(
            Feed::projected_agent_cursor(&scope).expect("resume"),
            Some(deliveries[3].cursor)
        );
    }
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
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
    let mut scope = store.principal(&principal).expect("scope");
    Feed::record_agent_event(&mut scope, &meaning(id), &receipt_delivery(&fact), 100)
        .expect("earlier real producer block");
    assert!(logs[0].cursor.block_height > fact.block_height + 1);
    let later = meaning(ActivityEntryId::new("act_gaplater").expect("id"));
    Feed::record_agent_event(&mut scope, &later, &logs[2], 101)
        .expect("filtered and empty heights need no synthetic events");
    assert!(matches!(
        Feed::record_agent_event(&mut scope, &later, &logs[1], 102),
        Err(FeedError::AgentEventGap)
    ));
    assert_eq!(
        Feed::projected_agent_cursor(&scope).expect("cursor"),
        Some(logs[2].cursor)
    );
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
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
    let first = meaning(ActivityEntryId::new("act_legacyfirst").expect("id"));
    {
        let mut scope = store.principal(&principal).expect("scope");
        Feed::record_agent_event(&mut scope, &first, &logs[0], 100).expect("first projected log");
        let key = RowKey::new("activity-feed").expect("key");
        let mut state: Value =
            serde_json::from_slice(scope.get(Table::Journeys, &key).expect("row").bytes())
                .expect("state");
        state["version"] = json!(1);
        state["last_agent_cursor"] = json!(logs[0].cursor.block_height);
        scope
            .put(
                Table::Journeys,
                key,
                100,
                serde_json::to_vec(&state).expect("legacy encoding"),
            )
            .expect("old schema");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen legacy store");
    {
        let mut scope = store.principal(&principal).expect("scope");
        assert_eq!(
            Feed::projected_agent_cursor(&scope).expect("safe migration resume"),
            Some(Cursor {
                block_height: logs[0].cursor.block_height - 1,
                tx_index: u32::MAX,
                log_index: u32::MAX
            })
        );
        Feed::record_agent_event(&mut scope, &first, &logs[0], 100)
            .expect("identity dedup on legacy replay");
        Feed::record_agent_event(
            &mut scope,
            &meaning(ActivityEntryId::new("act_legacysecond").expect("id")),
            &logs[1],
            101,
        )
        .expect("no same-block event lost in migration");
        let key = RowKey::new("activity-feed").expect("key");
        let state: Value =
            serde_json::from_slice(scope.get(Table::Journeys, &key).expect("row").bytes())
                .expect("new state");
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
    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen migrated state");
    assert_eq!(
        Feed::projected_agent_cursor(&store.principal(&principal).expect("scope"))
            .expect("full position"),
        Some(logs[1].cursor)
    );
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn finding5_rung_only_mutation_cannot_upgrade_a_signed_receipt() {
    let (id, mut fact, keys, consensus) = recorded_receipt();
    let principal = support::principal("alice");
    fact.committed = None;
    fact.header_binding = None;
    fact.rung = Rung::Receipted;
    EvidenceBundle::verify_receipt(&fact, &keys).expect("a real signature establishes Receipted");
    let valid = EvidenceBundle::receipt(id.clone(), fact.clone(), &principal).expect("bundle");
    valid
        .verify(valid.digest().expect("digest"), &principal, &keys)
        .expect("receipted export");
    fact.rung = Rung::Committed;
    let changed = EvidenceBundle::receipt(id, fact, &principal)
        .expect("artifact can relabel both entry and receipt");
    assert_ne!(
        valid.digest().expect("old digest"),
        changed.digest().expect("attacker recomputes digest")
    );
    assert!(matches!(
        changed.verify_with_consensus(
            changed.digest().expect("new digest"),
            &principal,
            &keys,
            &consensus
        ),
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
        let mut scope = store.principal(&principal).expect("scope");
        Feed::record_agent_event(
            &mut scope,
            &meaning(id.clone()),
            &receipt_delivery(&fact),
            100,
        )
        .expect("project actual receipt");
        let (bundle, report) = EvidenceExport::new(Feed::new(5).expect("feed"), 1024 * 1024)
            .expect("exporter")
            .evidence_with_consensus(
                &scope,
                &Feed::apply_filters(FilterDraft::new()).expect("filters"),
                &[id.clone()],
                vec![artifact(fact.clone())],
                &keys,
                &consensus,
                100,
                fact.block_height,
            )
            .expect("verify real producer evidence at export");
        assert_eq!(report.verified_receipts(), 1);
        assert_eq!(report.verified_inclusions(), 1);
        let bytes = bundle.encode().expect("v3 evidence codec");
        assert_eq!(&bytes[..9], b"LXHEXPV3\x03");
        scope
            .put(
                Table::Cache,
                RowKey::new("committed-export").expect("key"),
                100,
                bytes.clone(),
            )
            .expect("persist exact export");
        bytes
    };
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, tenancy).expect("restart actual store");
    {
        let scope = store.principal(&principal).expect("scope");
        let row = scope
            .get(Table::Cache, &RowKey::new("committed-export").expect("key"))
            .expect("recovered export");
        assert_eq!(row.bytes(), exported);
        let decoded = EvidenceBundle::decode(row.bytes()).expect("decode recovered evidence");
        assert_eq!(decoded.protocol_evidence()[0].receipts[0], fact);
        assert_eq!(decoded.entries()[0].rung(), Rung::Committed);
        let report = decoded
            .verify_with_consensus(
                decoded.digest().expect("digest"),
                &principal,
                &keys,
                &consensus,
            )
            .expect("portable committed verification after restart");
        assert_eq!(report.verified_receipts(), 1);
        assert_eq!(report.verified_inclusions(), 1);
        assert!(matches!(
            decoded.verify(decoded.digest().expect("digest"), &principal, &keys),
            Err(VerifyError::Unavailable(
                ExportError::AuthorityUnavailable { .. }
            ))
        ));
    }
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
        let mut tampered = fact.clone();
        let proof = tampered.committed.as_mut().expect("commit evidence");
        match target {
            "batch" => proof.batch_transaction[40] ^= 1,
            "aunt" => proof.batch_aunts[0][0] ^= 1,
            "range" => proof.batch_transaction[30] ^= 1,
            "activity" => proof.activity_ids[0][0] ^= 1,
            "commit" => proof.commit.votes[0].signature[0] ^= 1,
            "header" => proof.header.data_hash.as_mut().expect("root")[0] ^= 1,
            "execution" => proof.execution_header.data_hash.as_mut().expect("root")[0] ^= 1,
            "total" => proof.batch_total += 1,
            _ => unreachable!(),
        }
        let bundle =
            EvidenceBundle::receipt(id.clone(), tampered, &principal).expect("tampered codec");
        assert!(
            bundle
                .verify_with_consensus(
                    bundle.digest().expect("new digest"),
                    &principal,
                    &keys,
                    &consensus
                )
                .is_err(),
            "accepted modified {target}"
        );
    }
    let mut wrong = consensus.clone();
    wrong[0].validators[0].voting_power += 1;
    assert!(EvidenceBundle::verify_receipt_with_consensus(&fact, &keys, &wrong).is_err());
    let mut downgraded = fact.clone();
    downgraded.rung = Rung::Receipted;
    assert!(matches!(
        EvidenceBundle::verify_receipt_with_consensus(&downgraded, &keys, &consensus),
        Err(VerifyError::Unverified(UnverifiedReason::Tampered))
    ));
    drop(store);
    std::fs::remove_dir_all(root).expect("cleanup");
}

#[test]
fn finding6_real_producer_receipt_header_inclusion_roundtrip() {
    let (id, mut receipt, keys, _) = recorded_receipt();
    receipt.rung = Rung::Receipted;
    receipt.committed = None;
    let export = artifact(receipt.clone());
    let report = verify_receipt_export(&export, &keys)
        .expect("node-produced receipt and exact signed inclusion header");
    assert_eq!(report.verified_receipts, 1);
    assert_eq!(report.verified_inclusions, 1);
    let principal = support::principal("alice");
    let bundle = EvidenceBundle::receipt(id, receipt.clone(), &principal).expect("bundle");
    let encoded = bundle.encode().expect("transport encode");
    let decoded = EvidenceBundle::decode(&encoded).expect("transport decode");
    assert_eq!(decoded.encode().expect("canonical reencode"), encoded);
    assert_eq!(
        decoded
            .verify(decoded.digest().expect("digest"), &principal, &keys)
            .expect("pinned inclusion survives transport")
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
        .expect("binding")
        .receipt_proof = Proof::new(
        1,
        2,
        receipt
            .header_binding
            .as_ref()
            .expect("binding")
            .receipt_proof
            .siblings()
            .to_vec(),
    )
    .expect("other receipt slot");
    assert!(verify_receipt_export(&artifact(substituted), &keys).is_err());
    let raw: Value = serde_json::from_str(RECORDED).expect("recorded producer refusions");
    let mut foreign = receipt;
    foreign
        .header_binding
        .as_mut()
        .expect("binding")
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
        let (proof, root) = build_proof(&[leaf.as_slice()], 0).expect("real tree producer");
        let signer = SigningKey::from_bytes(&[91; 32]);
        let public = signer.verifying_key().to_bytes();
        let h = batch_header(root, root, public).expect("real canonical header codec");
        let signature = signer
            .sign(&batch_header_signing_digest(&h).expect("header digest"))
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
                .expect("real original binding")
                .receipt_proof
                .clone(),
        });
        assert!(verify_receipt_export(&attack, &keys).is_err());
    }
    let binding = receipt.header_binding.as_ref().expect("real binding");
    let header = layerx_wire::receipt::decode_batch_header(&binding.canonical_header_bytes)
        .expect("header codec");
    let (proof, _) = build_proof(&[receipt.tx_hash.as_slice()], 0).expect("proof codec");
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
            .expect("real committed receipt/header linkage");
    assert_eq!(
        report.achieved_levels,
        [layerx_types::verify::VerificationLevel::CHECKPOINT_FINALISED]
    );
    let mut wrong = keys;
    wrong[0].public_key = SigningKey::from_bytes(&[92; 32]).verifying_key().to_bytes();
    let bundle =
        EvidenceBundle::receipt(id, committed, &support::principal("alice")).expect("bundle");
    assert!(bundle
        .verify_with_consensus(
            bundle.digest().expect("digest"),
            &support::principal("alice"),
            &wrong,
            &consensus
        )
        .is_err());
}
