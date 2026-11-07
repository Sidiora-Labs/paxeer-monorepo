use layerx_agentd::{
    human::{HumanOperationError, HumanRefusal, HumanWireResponse},
    managed_agent::{self, FinalizationEvidence, ManagedAgentCheck},
    store::{Store, TenantId},
};
use std::{collections::BTreeSet, os::unix::fs::PermissionsExt};

// Minimal lifecycle producers copied from repair_22_31_219; no regression tests are included.
mod lifecycle_fixtures {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_agentd::{
        human::{
            self, HumanAgentJourneyKind, HumanFinalizationEvidence, HumanOperationError, HumanPeer,
            HumanRequest, HumanWireResponse,
        },
        managed_agent::{self, ManagedAgent},
        outbox::Outbox,
        prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget},
        read::{
            self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
            CommitVote, NativeResultSource, ServedCheckpoint,
        },
        receipt,
        store::{Store, TenantId},
    };
    use layerx_client::sequencer::ReceiptKeyEntry;
    use layerx_crypto::evm_transaction::{
        decode_signed, recover_sender, transaction_hash, Eip1559Call,
    };
    use layerx_types::{ids::Did, rung::Rung, verify::VerificationLevel};
    use layerx_wire::{encode::Encoder, hash::receipt_digest};
    use serde_json::{json, Value};
    use sha2::{Digest, Sha256};
    use std::{
        os::unix::{fs::PermissionsExt, net::UnixStream},
        path::PathBuf,
        sync::atomic::{AtomicU64, Ordering},
        thread,
        time::Duration,
    };
    const HEIGHT: u64 = 1000;
    const NOW: u64 = 1_750_000_400_000;
    static NEXT: AtomicU64 = AtomicU64::new(1);
    #[track_caller]
    fn must<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
        match r {
            Ok(value) => value,
            Err(error) => panic!("{error:?}"),
        }
    }
    fn hash(bytes: &[u8]) -> [u8; 32] {
        Sha256::digest(bytes).into()
    }
    fn rpc_hex(bytes: &[u8]) -> String {
        format!("0x{}", hex::encode(bytes))
    }
    fn decode_hex(v: &Value) -> Vec<u8> {
        must(hex::decode(v.as_str().unwrap().strip_prefix("0x").unwrap()))
    }
    struct Directory(PathBuf);
    impl Directory {
        fn new() -> Self {
            let p = must(std::fs::canonicalize(std::env::temp_dir())).join(format!(
                "repair219-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::SeqCst)
            ));
            must(std::fs::create_dir(&p));
            must(std::fs::set_permissions(
                &p,
                std::fs::Permissions::from_mode(0o700),
            ));
            Self(p)
        }
    }
    impl Drop for Directory {
        fn drop(&mut self) {
            must(std::fs::remove_dir_all(&self.0));
        }
    }
    fn validators() -> (Vec<SigningKey>, Vec<CommitValidator>) {
        let keys = (1..=4)
            .map(|n| SigningKey::from_bytes(&[n; 32]))
            .collect::<Vec<_>>();
        let validators = keys
            .iter()
            .map(|key| {
                let public_key = key.verifying_key().to_bytes();
                CommitValidator {
                    address: CommitValidator::address_for_key(&public_key),
                    public_key,
                    voting_power: 1,
                }
            })
            .collect();
        (keys, validators)
    }
    fn committed(
        height: u64,
        seconds: i64,
        parent: [u8; 32],
        result_root: [u8; 32],
        signers: usize,
    ) -> ServedCheckpoint {
        let (keys, validators) = validators();
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "125".into(),
            height,
            time: CommitTimestamp { seconds, nanos: 0 },
            last_block_id: Some(BlockId {
                hash: parent,
                part_set_total: 1,
                part_set_hash: [2; 32],
            }),
            last_commit_hash: Some([3; 32]),
            data_hash: Some([4; 32]),
            validators_hash: read::validator_set_hash(&validators),
            next_validators_hash: read::validator_set_hash(&validators),
            consensus_hash: [5; 32],
            app_hash: [6; 32],
            last_results_hash: Some(result_root),
            evidence_hash: Some([7; 32]),
            proposer_address: validators[0].address,
        };
        let mut commit = BlockCommit {
            height,
            round: 0,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [8; 32],
            },
            votes: Vec::new(),
        };
        for (key, v) in keys.iter().zip(&validators).take(signers) {
            let timestamp = header.time;
            commit.votes.push(CommitVote {
                validator_address: v.address,
                voting_power: v.voting_power,
                timestamp,
                signature: key
                    .sign(&commit.vote_sign_bytes("125", timestamp))
                    .to_bytes(),
            });
        }
        // The negative quorum fixture is intentionally carried to the production verifier.
        ServedCheckpoint {
            height,
            block_hash: header.block_hash(),
            app_hash: header.app_hash,
            commit,
            rung: Rung::Committed,
            header,
            validators,
        }
    }
    fn protobuf_varint(mut value: u64, bytes: &mut Vec<u8>) {
        while value >= 128 {
            bytes.push(value as u8 | 128);
            value >>= 7;
        }
        bytes.push(value as u8);
    }
    fn protobuf_bytes(number: u64, value: &[u8], bytes: &mut Vec<u8>) {
        protobuf_varint(number << 3 | 2, bytes);
        protobuf_varint(value.len() as u64, bytes);
        bytes.extend_from_slice(value);
    }
    fn protobuf_integer(number: u64, value: u64, bytes: &mut Vec<u8>) {
        if value != 0 {
            protobuf_varint(number << 3, bytes);
            protobuf_varint(value, bytes);
        }
    }

    // api/evm/tx.proto and cosmos/base/abci/v1beta1/abci.proto, independently
    // encoded by the peer rather than using NativeResultSource's result encoder.
    fn abci_response(receipt: &Value) -> Vec<u8> {
        let q = |name: &str| {
            u64::from_str_radix(receipt[name].as_str().unwrap().trim_start_matches("0x"), 16)
                .unwrap()
        };
        let mut response = Vec::new();
        protobuf_integer(1, q("gasUsed"), &mut response);
        if receipt["vmError"] != "" {
            protobuf_bytes(
                2,
                receipt["vmError"].as_str().unwrap().as_bytes(),
                &mut response,
            );
        }
        protobuf_bytes(3, &decode_hex(&receipt["returnData"]), &mut response);
        protobuf_bytes(
            4,
            receipt["txHash"].as_str().unwrap().as_bytes(),
            &mut response,
        );
        for log in receipt["logs"].as_array().unwrap() {
            let mut bytes = Vec::new();
            protobuf_bytes(1, log["address"].as_str().unwrap().as_bytes(), &mut bytes);
            for topic in log["topics"].as_array().unwrap() {
                protobuf_bytes(2, topic.as_str().unwrap().as_bytes(), &mut bytes);
            }
            protobuf_bytes(3, &decode_hex(&log["data"]), &mut bytes);
            protobuf_integer(
                4,
                u64::from_str_radix(
                    log["logIndex"].as_str().unwrap().trim_start_matches("0x"),
                    16,
                )
                .unwrap(),
                &mut bytes,
            );
            protobuf_bytes(5, &bytes, &mut response);
        }
        let mut msg = Vec::new();
        protobuf_bytes(1, b"/paxprotocol.paxchain.evm.MsgEVMTransaction", &mut msg);
        protobuf_bytes(2, &response, &mut msg);
        let mut tx = Vec::new();
        protobuf_bytes(1, &msg, &mut tx);
        tx
    }

    fn actor() -> Did {
        must(Did::new(
            format!("did:layerx:{}", hex::encode([9; 32])).as_bytes(),
        ))
    }
    fn owner_word() -> [u8; 32] {
        let call = Eip1559Call {
            chain_id: 125,
            nonce: 1,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100000,
            to: [0; 20],
            data: vec![],
        };
        let (call, sig) = must(decode_signed(&must(call.sign(&[1; 32]))));
        let mut word = [0; 32];
        word[12..].copy_from_slice(&must(recover_sender(&call, &sig)));
        word
    }
    fn session_grant() -> layerx_crypto::session::IssuedSessionKey {
        use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
        must(issue_session_key(&SessionKeyRequest {
            grantor: [8; 32],
            session_public_key: [12; 32],
            not_before: HEIGHT,
            expires_at: Some(HEIGHT + 10000),
            permitted_activity_types: vec![must(layerx_types::payload::ActivityType::new(
                layerx_types::payload::ModuleId::Governance,
                6,
            ))],
            revocation_sequence: Some(1),
            fee_budget: None,
            purpose: SessionPurpose::Activity,
        }))
    }
    fn seed() -> human::HumanAgentLifecycleSeed {
        human::HumanAgentLifecycleSeed {
            agent_id: format!("agt_{}", hex::encode([8; 32])),
            name: "agent".into(),
            purpose: "repair".into(),
            currency: "PAX".into(),
            monthly_limit: 100,
            period_start: 1,
            period_end: NOW / 1000 + 10000,
            created_at: 1,
            updated_at: 1,
            verified_evidence: vec![[1; 32]],
            actor: String::from_utf8(actor().as_bytes().to_vec()).unwrap(),
            primary_authority: hex::encode(owner_word()),
            custody_key: "owner".into(),
            custody_public_key: [9; 32],
            owner_account: format!(
                "agent:{}:main",
                std::str::from_utf8(actor().as_bytes()).unwrap()
            ),
            budget_account: format!(
                "agent:{}:budget:managed",
                std::str::from_utf8(actor().as_bytes()).unwrap()
            ),
            budget_asset: [4; 32],
            purpose_hash: [5; 32],
            recovery_root: [6; 32],
            recovery_threshold: 1,
            capability_id: [7; 32],
            activity_types: vec![7],
            counterparties: vec![[4; 32]],
            assets: vec![[4; 32]],
            amount_ceiling: 100,
            rate_maximum_uses: 10,
            rate_window_sequences: 10,
            purposes: vec!["repair".into()],
            capability_expiry_sequence: HEIGHT + 10000,
            session_scopes: vec!["prepare".into()],
            session_expiry_unix_seconds: NOW / 1000 + 10000,
            protocol_grant_id: session_grant().grant_id,
            budget_period_seconds: 100,
            budget_expiry_seconds: NOW / 1000 + 10000,
            initial_funding: 100,
            network_id: 125,
            creation_receipt_roots: vec![[1; 32]],
        }
    }
    fn word(n: u128) -> [u8; 32] {
        let mut w = [0; 32];
        w[16..].copy_from_slice(&n.to_be_bytes());
        w
    }
    fn event(
        address: [u8; 20],
        name: &str,
        id: [u8; 32],
        data: Vec<u8>,
    ) -> layerx_proof::sequencer_logs::SequencerLog {
        layerx_proof::sequencer_logs::SequencerLog::new(
            address,
            vec![transaction_hash(name.as_bytes()), id],
            data,
            0,
        )
    }
    fn protocol_receipt(
        raw: &[u8],
        logs: &[layerx_proof::sequencer_logs::SequencerLog],
        signature: Option<[u8; 64]>,
    ) -> Vec<u8> {
        let tx = transaction_hash(raw);
        let height = HEIGHT;
        let (call, sig) = must(decode_signed(raw));
        let sender = must(recover_sender(&call, &sig));
        let mut from = [0; 32];
        from[12..].copy_from_slice(&sender);
        let mut to = [0; 32];
        to[12..].copy_from_slice(&call.to);
        let mut summary = Encoder::new(256);
        must(summary.u64(1));
        must(summary.u64(1));
        must(summary.u64(1));
        must(summary.u32(0));
        must(summary.fixed(&[0; 20]));
        must(summary.fixed(&[0; 20]));
        must(summary.fixed(&hash(b"")));
        must(summary.u32(0));
        must(summary.u32(logs.len() as u32));
        must(summary.u8(4));
        must(summary.fixed(b"upax"));
        let mut e = Encoder::new(16384);
        must(e.structure_header_version(0x5201, 2));
        must(e.u16(2));
        must(e.bytes(&tx, 32));
        must(e.u64(height));
        must(e.bytes(&[2; 32], 32));
        let seq = hash(
            &[
                b"PAXEER/v1/sequence\0".as_slice(),
                &[2; 32],
                &height.to_be_bytes(),
                &tx,
            ]
            .concat(),
        );
        must(e.bytes(&seq, 32));
        must(e.bytes(&hash(b""), 32));
        must(e.i32(0));
        must(e.sequence_length(1 + logs.len(), 512));
        for (index, body) in std::iter::once(summary.finish())
            .chain(logs.iter().map(|log| {
                let mut body = log.address().to_vec();
                body.push(log.topics().len() as u8);
                for topic in log.topics() {
                    body.extend_from_slice(topic);
                }
                body.extend_from_slice(&hash(log.data()));
                body
            }))
            .enumerate()
        {
            must(e.u16(12));
            must(e.u16(index as u16));
            must(e.u16(0));
            must(e.tag(if index == 0 { 1 } else { 3 }, 3));
            must(e.u8(0));
            must(e.bytes(&[0; 32], 32));
            must(e.bytes(&body, 256));
        }
        must(e.u128(1));
        let mut block = [0; 32];
        block[24..].copy_from_slice(&height.to_be_bytes());
        must(e.bytes(&block, 32));
        must(e.u16(12));
        must(e.u32(1));
        must(e.u32(0));
        must(e.u8(2));
        must(e.bytes(&[0; 32], 32));
        must(e.u128(0));
        must(e.bytes(&from, 32));
        must(e.u128(1000));
        must(e.u128(999));
        must(e.u64(call.nonce));
        must(e.bytes(&to, 32));
        must(e.u128(0));
        must(e.u128(0));
        must(e.bytes(&[0; 32], 32));
        must(e.bytes(&[0; 32], 32));
        must(e.bytes(&[11; 32], 32));
        must(e.u64(NOW));
        must(e.u8(u8::from(signature.is_some())));
        if let Some(signature) = signature {
            must(e.bytes(&signature, 64));
        }
        e.finish()
    }
    fn retain_receipt(
        store: &mut Store,
        tenant: &TenantId,
        call: Eip1559Call,
        action: [u8; 32],
        logs: Vec<layerx_proof::sequencer_logs::SequencerLog>,
    ) -> HumanFinalizationEvidence {
        let height = HEIGHT;
        let raw = must(call.sign(&[1; 32]));
        let tx = must(Outbox::default().enqueue(store, tenant.clone(), raw.clone()));
        let signer = SigningKey::from_bytes(&[31; 32]);
        let digest = must(receipt_digest(&protocol_receipt(&raw, &logs, None)));
        let canonical = protocol_receipt(&raw, &logs, Some(signer.sign(&digest).to_bytes()));
        // Both signed log effects and their committed ABCI result are emitted by the peer transcript.
        let peer = json!({"gasUsed":"0x1","vmError":"","returnData":"0x","txHash":rpc_hex(&tx),"logs":logs.iter().map(|log|
        json!({"address":rpc_hex(&log.address()),"topics":log.topics().iter().map(|t|rpc_hex(t)).collect::<Vec<_>>(),
            "data":rpc_hex(log.data()),"logIndex":format!("{:#x}",log.log_index())})).collect::<Vec<_>>()});
        let results = json!([{"data":openssl::base64::encode_block(&abci_response(&peer)),"gas_wanted":"1","gas_used":"1"}]);
        let inclusion = must(NativeResultSource::from_results(&results, 0));
        let root =
            layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[&inclusion.result]);
        let checkpoint = committed(height + 1, NOW as i64 / 1000, [1; 32], root, 4);
        let key = ReceiptKeyEntry {
            key_id: [11; 32],
            public_key: signer.verifying_key().to_bytes(),
            first_height: height,
            last_height: height,
        };
        must(receipt::persist_committed_protocol_receipt(
            store,
            tenant.clone(),
            &canonical,
            &[key],
            height,
            0,
            &inclusion,
            &checkpoint,
            "125",
            &checkpoint.validators,
        ));
        HumanFinalizationEvidence {
            action_key: action,
            tx_hash: tx,
            receipt_hash: hash(&canonical),
            observed_height: height,
            rung: Rung::Committed.wire_rank(),
            finalized_at: NOW / 1000 + call.nonce,
        }
    }
    fn socket_exchange(
        request: &HumanRequest,
        handler: impl FnOnce(HumanRequest) -> Result<human::HumanResponse, HumanOperationError>
            + Send
            + 'static,
    ) -> HumanWireResponse {
        use human::frame::HumanFrameTransport;
        let bytes = must(human::encode_lifecycle_request(&request));
        let (a, b) = must(UnixStream::pair());
        let worker = thread::spawn(move || {
            let mut transport = must(human::AcceptedTransport::new(
                a,
                1_048_576,
                Duration::from_secs(3),
            ));
            let peer = HumanPeer {
                uid: 1000,
                principal: "human".into(),
                tenant: "repair219".into(),
                subject: None,
            };
            must(human::serve_one_with(&mut transport, &peer, handler));
        });
        let mut client = must(human::AcceptedTransport::new(
            b,
            1_048_576,
            Duration::from_secs(3),
        ));
        must(client.send(&bytes));
        let result = must(human::decode_response(must(client.receive())));
        must(worker.join());
        result
    }
    fn lifecycle(
        directory: &Directory,
        tenant: &TenantId,
        request: &HumanRequest,
    ) -> HumanWireResponse {
        let path = directory.0.clone();
        let tenant = tenant.clone();
        socket_exchange(request, move |request| {
            let mut store = Store::open(&path).unwrap();
            managed_agent::apply_lifecycle_request(&mut store, &tenant, request)
        })
    }
    fn agent_state(bytes: &[u8]) -> u8 {
        let mut offset = 0;
        for _ in 0..3 {
            let n = u32::from_be_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize;
            offset += 4 + n;
        }
        bytes[offset]
    }
    fn observe_budget(
        store: &mut Store,
        tenant: &TenantId,
        budget: [u8; 32],
        tag: u8,
        remaining: u128,
    ) -> [u8; 32] {
        let state = layerx_agentd::human_runtime::CoreBudgetState {
            revocation_sequence: 1,
            observed_head_sequence: HEIGHT,
            verification: VerificationLevel::COMMITTED.wire_rank(),
            evidence_digest: [tag; 32],
            receipt_digest: [2; 32],
            checkpoint_digest: [3; 32],
            age_sequences: 0,
            maximum_age_sequences: 10,
            remaining,
            asset: [4; 32],
        };
        must(managed_agent::record_budget_snapshot(
            store, tenant, budget, &state,
        ));
        state.evidence_digest
    }
    fn reclaim_evidence(
        store: &mut Store,
        tenant: &TenantId,
        budget: [u8; 32],
    ) -> HumanFinalizationEvidence {
        let amount = 10;
        let call = PrecompileCall {
            target: PrecompileTarget::Pay,
            selector: layerx_crypto::disclosure::call::ledger_selector(
                "budgetDefund(bytes32,uint128)",
            ),
            arguments: vec![AbiArgument::Word(budget), AbiArgument::Word(word(amount))],
        };
        let logs = vec![event(
            call.target.address(),
            "BudgetDefunded(bytes32,uint128,uint128)",
            budget,
            [word(amount).to_vec(), word(100 - amount).to_vec()].concat(),
        )];
        let evidence = retain_receipt(
            store,
            tenant,
            Eip1559Call {
                chain_id: 125,
                nonce: 63,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 100000,
                to: call.target.address(),
                data: must(call.encode()),
            },
            [63; 32],
            logs,
        );
        assert_ne!(evidence.action_key, evidence.tx_hash);
        evidence
    }
    pub(crate) fn archived_reclaim_over_socket() -> HumanWireResponse {
        let directory = Directory::new();
        let tenant = must(TenantId::new("repair219"));
        let mut store = must(Store::open(&directory.0));
        let seed = seed();
        let mut agent = must(ManagedAgent::from_creation(
            &seed,
            &seed.actor,
            [10; 32],
            [11; 32],
            1,
        ));
        agent.state = 4;
        must(managed_agent::publish_creation(&mut store, &tenant, &agent));
        let pre = observe_budget(&mut store, &tenant, agent.active_budget_id, 61, 100);
        let post = observe_budget(&mut store, &tenant, agent.active_budget_id, 62, 90);
        let evidence = reclaim_evidence(&mut store, &tenant, agent.active_budget_id);
        drop(store);
        let store = must(Store::open(&directory.0));
        assert_eq!(
            agent_state(must(managed_agent::get(&store, &tenant, &seed.agent_id)).bytes()),
            4
        );
        drop(store);
        lifecycle(
            &directory,
            &tenant,
            &HumanRequest::AgentJourney {
                agent_id: seed.agent_id,
                kind: HumanAgentJourneyKind::Reclaim {
                    amount: 10,
                    currency: seed.currency,
                },
                pre_observation: pre,
                post_observation: post,
                evidence,
            },
        )
    }
}

#[test]
fn managed_agent_refusal_reasons_are_unique_and_prefixed() {
    assert!(!ManagedAgentCheck::ALL.is_empty());
    let mut reasons = BTreeSet::new();
    let mut variants = Vec::new();
    for &check in ManagedAgentCheck::ALL {
        let reason = check.reason();
        assert!(reason.starts_with("managed_agent."), "{check:?}: {reason}");
        assert!(
            reasons.insert(reason),
            "duplicate reason: {check:?}: {reason}"
        );
        assert!(!variants.contains(&check), "duplicate variant: {check:?}");
        variants.push(check);
        assert_eq!(HumanRefusal::ManagedAgent(check).reason(), reason);
    }
    assert_eq!(reasons.len(), ManagedAgentCheck::ALL.len());
}

#[test]
fn lifecycle_request_refusal_names_its_check_over_the_socket() {
    let response = lifecycle_fixtures::archived_reclaim_over_socket();
    let HumanWireResponse::Refused(refusal) = response else {
        panic!("expected an archived-agent refusal: {response:?}");
    };
    assert_eq!(refusal.reason, ManagedAgentCheck::ArchivedState.reason());
    assert_ne!(refusal.reason, "operation.refused");
    assert_eq!(refusal.nonce, None);
}

#[test]
fn authority_revocation_refusal_names_the_window_check() {
    let root = std::env::temp_dir().join(format!("managed-check-613-{}", std::process::id()));
    std::fs::create_dir(&root).expect("create private store directory");
    std::fs::set_permissions(&root, std::fs::Permissions::from_mode(0o700))
        .expect("private directory permissions");
    let store = Store::open(&root).expect("real durable store");
    let tenant = TenantId::new("managed-check-613").expect("tenant");
    let evidence = FinalizationEvidence {
        action_key: [1; 32],
        activity_id: [2; 32],
        receipt_digest: [3; 32],
        observed_sequence: 100,
        verification: layerx_types::rung::Rung::Committed.wire_rank(),
        finalized_at: 100,
    };
    for kind in [1, 2] {
        for ready_at in [evidence.finalized_at, evidence.finalized_at - 1] {
            let result = managed_agent::validate_authority_revocation(
                &store,
                &tenant,
                "archived-agent",
                (kind, 10, ready_at),
                [4; 32],
                [0; 32],
                evidence,
            );
            assert_eq!(
                result.err(),
                Some(HumanOperationError::Typed(HumanRefusal::ManagedAgent(
                    ManagedAgentCheck::RevocationWindow,
                ))),
                "kind {kind}, ready_at {ready_at}",
            );
        }
    }
    drop(store);
    std::fs::remove_dir_all(root).expect("remove private store");
}
