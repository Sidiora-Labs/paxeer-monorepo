#[path = "support/budget_proof_replay.rs"]
mod budget_proof_replay;
#[path = "support/committed_receipt.rs"]
mod committed_receipt;

use budget_proof_replay::Replay;
use ed25519_dalek::SigningKey;
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agentd::budget::budget_proof::BudgetProofError;
use layerx_agentd::budget::{verify_budget_proof, BudgetProofTrust, ProtocolBudgetState};
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawStateEvidence};
use layerx_agentd::read::{CommittedReads, ServedCheckpoint};
use layerx_human_service::activity::{
    verify_receipt_export_with_consensus, ActivityKind, AgentActivity, EvidenceBundle,
    EvidenceExport, Feed, FilterDraft, PendingStatus, ReceiptExport, VerifiedStatus,
};
use layerx_human_service::agents::{SpendProfile, SpendReconciliation};
use layerx_human_service::journeys::VerifiedLegEvidence;
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::{HumanComponentServer, ProductionComponents};
use layerx_human_service::store::PrincipalStore;
use layerx_human_test_support as support;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{ids::Did, rung::Rung, verify::VerificationLevel};
use serde_json::{json, Value};

const ANSWERS: &str =
    include_str!("../../../../../tests/vectors/budget-proof-v2/node-answers.json");
const EXPORT: &[u8] = include_bytes!("../../../../../tests/vectors/budget-proof-v2/export.json");
const OWNER: &str = "did:layerx:0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a";
const NOW: u64 = 1_700_000_000_000;

fn decode_hex(text: &str) -> Vec<u8> {
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}

fn profile() -> SpendProfile {
    let mut budget_id = [0; 32];
    budget_id[31] = 7;
    SpendProfile {
        principal: support::principal("operator"),
        agent_id: [0x31; 32],
        agent_account: [2; 32],
        budget_id,
        asset: [4; 32],
    }
}

fn state_profile() -> SpendProfile {
    SpendProfile {
        agent_account: [3; 32],
        ..profile()
    }
}

fn checkpoint(replay: &Replay) -> ServedCheckpoint {
    let document: Value = serde_json::from_slice(EXPORT).expect("recorded budget export");
    let hash = decode_hex(document["head"]["block_hash"].as_str().expect("block hash"))
        .try_into()
        .expect("block hash width");
    ServedCheckpoint::acquire_at_committed_block(&replay.endpoint, 8, Some(hash), "125")
        .expect("verify recorded node consensus over real HTTP")
}

fn committed_protocol(replay: &Replay, checkpoint: &ServedCheckpoint) -> ProtocolBudgetState {
    let reads =
        CommittedReads::at_checkpoint(&replay.endpoint, checkpoint).expect("committed reads");
    let document: Value = serde_json::from_slice(EXPORT).expect("budget export");
    let call = &document["proofs"]["budget_call"];
    let committed = reads
        .budget_state(
            decode_hex(call["to"].as_str().expect("precompile"))
                .try_into()
                .expect("address width"),
            &decode_hex(call["data"].as_str().expect("calldata")),
        )
        .expect("read and independently repeat getBudget at committed block");
    ProtocolBudgetState {
        evidence: RawStateEvidence::budget_witness(committed, profile().budget_id)
            .expect("canonical budget witness"),
    }
}

#[test]
fn committed_state_is_served_as_committed_and_survives_reopen() {
    let replay = Replay::new(serde_json::from_str(ANSWERS).expect("recorded node answers"));
    let checkpoint = checkpoint(&replay);
    let protocol = committed_protocol(&replay, &checkpoint);
    let verifier =
        EvidenceAuthority::pinned_to_handshake(3, 125, [7; 32]).expect("pinned authority");
    assert_eq!(
        verifier
            .verify_state(&protocol.evidence)
            .expect("verify state")
            .level(),
        VerificationLevel::COMMITTED
    );
    let root = support::directory("repair-22-31-540-state");
    let retention = support::retention_uniform(100_000);
    let (mut store, digest) = support::install_and_open(
        &root,
        &support::tenancy(&[("operator", "tenant")]),
        retention,
    );
    let operator = support::principal("operator");
    let mut scope = store.principal(&operator).expect("principal scope");
    let mut spend = SpendReconciliation::new(state_profile()).expect("spend service");
    let view = spend
        .observe(&mut scope, &protocol, &[], &verifier, NOW)
        .expect("serve committed budget state");
    assert_eq!(view.verification_level(), Rung::Committed);
    assert_eq!(
        (view.spent(), view.limit(), view.remaining()),
        (23, 100, 77)
    );
    let surfaces = spend.both_shells(&operator).expect("served spend surfaces");
    assert_eq!(surfaces.mobile().spend(), &view);
    assert_eq!(surfaces.desktop().spend(), &view);
    drop(scope);
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen principal store");
    let scope = store.principal(&operator).expect("reopened scope");
    let recovered =
        SpendReconciliation::open(&scope, state_profile()).expect("recover spend evidence");
    assert_eq!(
        recovered
            .both_shells(&operator)
            .expect("recovered surfaces"),
        surfaces
    );
    drop(scope);
    drop(store);
    assert!(
        replay.finish() >= 2,
        "committed read independently repeats getBudget"
    );
    std::fs::remove_dir_all(root).expect("remove test state");
}

#[test]
fn state_proven_state_cannot_promote_served_spend() {
    let replay = Replay::new(serde_json::from_str(ANSWERS).expect("recorded node answers"));
    let checkpoint = checkpoint(&replay);
    let committed = committed_protocol(&replay, &checkpoint);
    let protocol = ProtocolBudgetState {
        evidence: support::raw_state_leaf(committed.evidence.canonical_state().to_vec(), 8),
    };
    let verifier = support::evidence_verifier(&SigningKey::from_bytes(&[0x84; 32]));
    assert_eq!(
        verifier
            .verify_state(&protocol.evidence)
            .expect("verify state leaf")
            .level(),
        VerificationLevel::STATE_PROVEN
    );
    let root = support::directory("repair-22-31-540-state-proven");
    let (mut store, _) = support::install_and_open(
        &root,
        &support::tenancy(&[("operator", "tenant")]),
        support::retention_uniform(100_000),
    );
    let operator = support::principal("operator");
    let mut scope = store.principal(&operator).expect("scope");
    let mut spend = SpendReconciliation::new(state_profile()).expect("spend service");
    let view = spend
        .observe(&mut scope, &protocol, &[], &verifier, NOW)
        .expect("serve state-proven spend");
    assert_eq!(view.verification_level(), Rung::Receipted);
    assert_eq!(
        spend
            .both_shells(&operator)
            .expect("surfaces")
            .mobile()
            .spend(),
        &view
    );
    drop(scope);
    drop(store);
    assert!(replay.finish() >= 2, "real canonical state producer");
    std::fs::remove_dir_all(root).expect("remove state-proven store");
}

#[test]
fn committed_budget_proof_is_accepted_and_state_proven_proof_is_refused_at_ingress() {
    let replay = Replay::new(serde_json::from_str(ANSWERS).expect("recorded node answers"));
    let checkpoint = checkpoint(&replay);
    let owner = Did::new(OWNER.as_bytes()).expect("owner DID");
    let trust = BudgetProofTrust {
        tenant: "tenant",
        principal: "operator",
        owner: &owner,
        protocol_version: 3,
        network_id: 125,
        checkpoint: &checkpoint,
        endpoint: &replay.endpoint,
        current_height: 8,
        now_ms: NOW,
        maximum_age_seconds: 60,
    };
    let mut export: Value = serde_json::from_slice(EXPORT).expect("recorded export");
    export["budget_state"]["verification"] = json!(VerificationLevel::COMMITTED.wire_rank());
    let bytes = serde_json::to_vec(&export).expect("current committed label");
    let proof = verify_budget_proof(&bytes, profile().budget_id, &trust)
        .expect("real committed proof verifier");
    assert_eq!(proof.verification(), VerificationLevel::COMMITTED);
    let root = support::directory("repair-22-31-540-proof");
    let (mut store, _) = support::install_and_open(
        &root,
        &support::tenancy(&[("operator", "tenant")]),
        support::retention_uniform(100_000),
    );
    let operator = support::principal("operator");
    let mut scope = store.principal(&operator).expect("scope");
    let verifier = EvidenceAuthority::pinned_to_handshake(3, 125, [7; 32]).expect("authority");
    let mut spend = SpendReconciliation::new(profile()).expect("spend service");
    let view = spend
        .observe_budget_proof(&mut scope, &proof, &[], &verifier, NOW)
        .expect("spend guard accepts COMMITTED proof");
    assert_eq!(view.verification_level(), Rung::Committed);
    assert_eq!((view.spent(), view.remaining()), (23, 77));
    export["budget_state"]["verification"] = json!(VerificationLevel::STATE_PROVEN.wire_rank());
    assert!(matches!(
        verify_budget_proof(
            &serde_json::to_vec(&export).expect("state-proven export"),
            profile().budget_id,
            &trust
        ),
        Err(BudgetProofError::Refused)
    ));
    assert_eq!(
        spend
            .both_shells(&operator)
            .expect("retained spend")
            .desktop()
            .spend(),
        &view
    );
    drop(scope);
    drop(store);
    assert!(
        replay.finish() >= 2,
        "proof verifies independent budget calls"
    );
    std::fs::remove_dir_all(root).expect("remove proof state");
}

#[test]
fn real_committed_receipt_exports_committed_verification_and_statement_label() {
    let (id, fact, keys, consensus) = committed_receipt::recorded_receipt();
    let artifact = ReceiptExport {
        receipts: vec![fact.clone()],
        inclusions: Vec::new(),
        derived_aggregates: Vec::new(),
    };
    let report = verify_receipt_export_with_consensus(&artifact, &keys, &consensus)
        .expect("verify recorded signed receipt and consensus");
    assert_eq!(report.achieved_levels, [VerificationLevel::COMMITTED]);
    assert_eq!(report.verified_receipts, 1);
    assert_eq!(
        HumanComponentServer::verification_level_name(Rung::Committed),
        "committed"
    );
    let decoded_receipt =
        layerx_wire::receipt::decode(&fact.canonical_receipt_bytes).expect("recorded receipt");
    let protocol = decoded_receipt.protocol().expect("protocol receipt");
    let evidence = VerifiedLegEvidence {
        action_key: [1; 32],
        activity_id: fact.tx_hash,
        canonical_receipt: fact.canonical_receipt_bytes.clone(),
        receipt_digest: fact.receipt_hash,
        authorised_batch: AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            keys[0].public_key,
        ),
        tx_hash: fact.tx_hash,
        block_height: fact.block_height,
        rung: fact.rung,
        signed_receipt: fact.canonical_receipt_bytes.clone(),
        receipt_key: keys[0],
        inclusion_height: fact.block_height,
    };
    let evidence_id = format!(
        "evd_{}",
        fact.receipt_hash
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    let material = ProductionComponents::managed_receipt_material(&evidence_id, &evidence)
        .expect("real owner receipt material producer");
    assert_eq!(material["verification"], "committed");
    let schema = ApiSchema::v1().expect("executable schema");
    let operation = schema
        .operation("evidence.get")
        .expect("evidence operation");
    schema
        .encode_response(operation, &material)
        .expect("current label crosses response validation");
    for retired in [
        "checkpoint-finalised",
        "settlement-anchored",
        "paxeer-finalised",
    ] {
        let mut invalid = material.clone();
        invalid["verification"] = json!(retired);
        assert!(
            schema.encode_response(operation, &invalid).is_err(),
            "retired output {retired}"
        );
    }
    let root = support::directory("repair-22-31-540-export");
    let (mut store, _) = support::install_and_open(
        &root,
        &support::tenancy(&[("operator", "tenant")]),
        support::retention_uniform(100_000),
    );
    let operator = support::principal("operator");
    let mut scope = store.principal(&operator).expect("scope");
    let activity = AgentActivity::new(
        id.clone(),
        ActivityKind::AgentAction,
        None,
        10,
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .expect("activity meaning");
    let reference: String = fact
        .receipt_hash
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    let delivery = EventDelivery::new(
        EventIdentity::new(fact.receipt_hash),
        fact.canonical_receipt_bytes.clone(),
        Cursor {
            block_height: fact.block_height,
            tx_index: 1,
            log_index: 0,
        },
        ReceiptReference::Verified {
            receipt_ref: ReceiptRef::new(reference.clone()).expect("receipt ref"),
            rung: fact.rung,
        },
    )
    .expect("recorded receipt delivery");
    Feed::record_agent_event(&mut scope, &activity, &delivery, 100).expect("record actual receipt");
    let filters = Feed::apply_filters(FilterDraft::new()).expect("filters");
    let (bundle, _) = EvidenceExport::new(Feed::new(5).expect("feed"), 1024 * 1024)
        .expect("exporter")
        .evidence_with_consensus(
            &scope,
            &filters,
            &[id],
            vec![artifact],
            &keys,
            &consensus,
            100,
            fact.block_height,
        )
        .expect("real activity export");
    let decoded = EvidenceBundle::decode(&bundle.encode().expect("encode")).expect("decode");
    let report =
        verify_receipt_export_with_consensus(&decoded.protocol_evidence()[0], &keys, &consensus)
            .expect("verify transported export");
    assert_eq!(report.achieved_levels, [VerificationLevel::COMMITTED]);
    let statement = EvidenceExport::new(Feed::new(5).expect("feed"), 1024 * 1024)
        .expect("statement exporter")
        .statement(&scope, &filters, 100, fact.block_height)
        .expect("statement");
    let csv = std::str::from_utf8(statement.content()).expect("CSV");
    assert_eq!(
        csv,
        format!(
            "entry_id,kind,status,occurred_at,projected_at,verification,receipt_references\r\n\
             act_repair2231156,agent-action,\"Done, finalised\",10,100,committed,{reference}\r\n"
        )
    );
    assert_eq!(statement.entries(), 1);
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).expect("remove export state");
}
