include!("native_send_preview_evm.rs");

use layerx_crypto::evm_transaction::Eip1559Call;

mod material {
    use super::*;
    use ed25519_dalek::SigningKey;
    use layerx_agentd::read::{
        self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
        ServedCheckpoint,
    };
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    use layerx_types::rung::Rung;
    use layerx_wire::encode::Encoder;
    pub const HEIGHT: u64 = 1000;
    pub const NOW: u64 = 1_750_000_400_000;
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn must<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
        match r {
            Ok(value) => value,
            Err(error) => panic!("{error:?}"),
        }
    }
    pub fn hash(bytes: &[u8]) -> [u8; 32] {
        Sha256::digest(bytes).into()
    }
    pub fn rpc_hex(bytes: &[u8]) -> String {
        format!("0x{}", hex::encode(bytes))
    }
    pub fn quantity(v: u64) -> String {
        format!("{v:#x}")
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn decode_hex(v: &Value) -> Vec<u8> {
        must(hex::decode(super::lint_present(
            super::lint_present(v.as_str(), "required test operation").strip_prefix("0x"),
            "required test operation",
        )))
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn validators() -> (Vec<SigningKey>, Vec<CommitValidator>) {
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
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn committed(
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
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn signed_header(c: &ServedCheckpoint) -> Value {
        let h = &c.header;
        let time = if h.time.seconds
            == super::lint_success(i64::try_from(NOW), "integer fits i64") / 1000 - 1
        {
            "2025-06-15T15:13:19Z"
        } else {
            "2025-06-15T15:13:20Z"
        };
        let id = |b: BlockId| json!({"hash":hex::encode(b.hash),"parts":{"total":b.part_set_total,"hash":hex::encode(b.part_set_hash)}});
        json!({"header":{"version":{"block":"11","app":"1"},"chain_id":"125","height":h.height.to_string(),"time":time,
        "last_block_id":id(super::lint_present(h.last_block_id, "required test operation")),"last_commit_hash":hex::encode(super::lint_present(h.last_commit_hash, "required test operation")),"data_hash":hex::encode(super::lint_present(h.data_hash, "required test operation")),"validators_hash":hex::encode(h.validators_hash),"next_validators_hash":hex::encode(h.next_validators_hash),"consensus_hash":hex::encode(h.consensus_hash),"app_hash":hex::encode(h.app_hash),"last_results_hash":hex::encode(super::lint_present(h.last_results_hash, "required test operation")),"evidence_hash":hex::encode(super::lint_present(h.evidence_hash, "required test operation")),"proposer_address":hex::encode(h.proposer_address)},
        "commit":{"height":c.height.to_string(),"round":c.commit.round,"block_id":id(c.commit.block_id),"signatures":c.commit.votes.iter().map(|v|json!({"block_id_flag":2,"validator_address":hex::encode(v.validator_address),"timestamp":time,"signature":openssl::base64::encode_block(&v.signature)})).collect::<Vec<_>>()}})
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn protobuf_varint(mut value: u64, bytes: &mut Vec<u8>) {
        while value >= 128 {
            bytes.push(super::lint_success(u8::try_from(value & 0x7f), "integer fits u8") | 128);
            value >>= 7;
        }
        bytes.push(super::lint_success(u8::try_from(value), "integer fits u8"));
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn protobuf_bytes(number: u64, value: &[u8], bytes: &mut Vec<u8>) {
        protobuf_varint(number << 3 | 2, bytes);
        protobuf_varint(value.len() as u64, bytes);
        bytes.extend_from_slice(value);
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn protobuf_integer(number: u64, value: u64, bytes: &mut Vec<u8>) {
        if value != 0 {
            protobuf_varint(number << 3, bytes);
            protobuf_varint(value, bytes);
        }
    }
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn abci_response(receipt: &Value) -> Vec<u8> {
        let q = |name: &str| {
            super::lint_success(
                u64::from_str_radix(
                    super::lint_present(receipt[name].as_str(), "required test operation")
                        .trim_start_matches("0x"),
                    16,
                ),
                "required test operation",
            )
        };
        let mut response = Vec::new();
        protobuf_integer(1, q("gasUsed"), &mut response);
        if receipt["vmError"] != "" {
            protobuf_bytes(
                2,
                super::lint_present(receipt["vmError"].as_str(), "required test operation")
                    .as_bytes(),
                &mut response,
            );
        }
        protobuf_bytes(3, &decode_hex(&receipt["returnData"]), &mut response);
        protobuf_bytes(
            4,
            super::lint_present(receipt["txHash"].as_str(), "required test operation").as_bytes(),
            &mut response,
        );
        for log in super::lint_present(receipt["logs"].as_array(), "required test operation") {
            let mut bytes = Vec::new();
            protobuf_bytes(
                1,
                super::lint_present(log["address"].as_str(), "required test operation").as_bytes(),
                &mut bytes,
            );
            for topic in super::lint_present(log["topics"].as_array(), "required test operation") {
                protobuf_bytes(
                    2,
                    super::lint_present(topic.as_str(), "required test operation").as_bytes(),
                    &mut bytes,
                );
            }
            protobuf_bytes(3, &decode_hex(&log["data"]), &mut bytes);
            protobuf_integer(
                4,
                super::lint_success(
                    u64::from_str_radix(
                        super::lint_present(log["logIndex"].as_str(), "required test operation")
                            .trim_start_matches("0x"),
                        16,
                    ),
                    "required test operation",
                ),
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
    /// # Panics
    /// Panics if required fixture data is absent, invalid, or an operation fails.
    pub fn protocol_receipt(
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
        must(summary.u32(super::lint_success(
            u32::try_from(logs.len()),
            "integer fits u32",
        )));
        must(summary.u8(4));
        must(summary.fixed(b"upax"));
        let mut e = Encoder::new(16_384);
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
                body.push(super::lint_success(
                    u8::try_from(log.topics().len()),
                    "integer fits u8",
                ));
                for topic in log.topics() {
                    body.extend_from_slice(topic);
                }
                body.extend_from_slice(&hash(log.data()));
                body
            }))
            .enumerate()
        {
            must(e.u16(12));
            must(e.u16(super::lint_success(
                u16::try_from(index),
                "integer fits u16",
            )));
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
        must(e.bytes(&hash(b"upax"), 32));
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
}

use layerx_agentd::agent_rpc::{handle_rpc, AgentRpcResponse};
use layerx_agentd::config::StateExportSource;
use layerx_agentd::export::{deserialize_export, serialize_export, verify, ExportTrustSource};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::read::NativeResultSource;
use layerx_agentd::store::{ObjectKind, TenantKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::rung::Rung;
use material::{HEIGHT, NOW};

fn issue_storage(address: [u8; 20]) -> (Value, [u8; 32], [u8; 32]) {
    let slot = [7; 32];
    let value = [9; 32];
    let key = [&[3][..], &address, &slot].concat();
    let existence = |key: &[u8], value: &[u8], prefix: &[u8]| {
        let value_hash = Sha256::digest(value);
        let mut preimage = prefix.to_vec();
        material::protobuf_varint(key.len() as u64, &mut preimage);
        preimage.extend_from_slice(key);
        material::protobuf_varint(value_hash.len() as u64, &mut preimage);
        preimage.extend_from_slice(&value_hash);
        let root: [u8; 32] = Sha256::digest(&preimage).into();
        let mut leaf = Vec::new();
        material::protobuf_integer(1, 1, &mut leaf);
        material::protobuf_integer(3, 1, &mut leaf);
        material::protobuf_integer(4, 1, &mut leaf);
        material::protobuf_bytes(5, prefix, &mut leaf);
        let mut proof = Vec::new();
        material::protobuf_bytes(1, key, &mut proof);
        material::protobuf_bytes(2, value, &mut proof);
        material::protobuf_bytes(3, &leaf, &mut proof);
        let mut commitment = Vec::new();
        material::protobuf_bytes(1, &proof, &mut commitment);
        (root, commitment)
    };
    let (store_root, store_proof) = existence(&key, &value, &[0, 2, 0xe0, 0x12]);
    let (app_hash, app_proof) = existence(b"evm", &store_root, &[0]);
    let response = json!({"address":format!("0x{}",hex::encode(address)),
    "hexValues":[hex::encode(value)], "storageProof":[{"ops":[
        {"type":"ics23:iavl", "key":openssl::base64::encode_block(&key), "data":openssl::base64::encode_block(&store_proof)},
        {"type":"ics23:simple", "key":openssl::base64::encode_block(b"evm"), "data":openssl::base64::encode_block(&app_proof)}
    ]}]});
    (response, app_hash, slot)
}

struct IssueNode {
    url: String,
    _peer: Peer,
    storage: Arc<Mutex<Value>>,
    calls: Arc<Mutex<Vec<String>>>,
    trust: ExportTrustSource,
    raw: Vec<u8>,
    address: [u8; 20],
    slot: [u8; 32],
}

impl IssueNode {
    fn start() -> Self {
        let identity = must(derive_from_seed(&[3; 64], 0));
        let public_key = identity.layerx_public_key();
        let raw = must(
            Eip1559Call {
                chain_id: 125,
                nonce: 3,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 21_000,
                to: [0x22; 20],
                data: vec![],
            }
            .sign(lint_present(identity.evm_secret(), "owner EVM signing key")),
        );
        let (call, signature) = must(decode_signed(&raw));
        let address = must(recover_sender(&call, &signature));
        assert_eq!(address, identity.evm_address());
        let IssueReceipt {
            canonical,
            key,
            mut receipt,
            results,
            results_root,
        } = issue_receipt(&raw, address, call.to);
        let (storage, app_hash, slot) = issue_storage(address);
        let (current, next) =
            issue_checkpoints(app_hash, results_root, &mut receipt, &canonical, address);
        let trust = must(ExportTrustSource::new(
            vec![key],
            current.validators.clone(),
        ));
        let authorizations = [layerx_proof::inclusion::SequencerAuthorization::new(
            key.key_id,
            key.public_key,
            key.first_height,
            key.last_height,
        )];
        must(layerx_proof::receipt::verify_sequencer_signature(
            &canonical,
            &authorizations[..],
        ));
        let block = json!({"number":format!("{HEIGHT:#x}"), "hash":material::rpc_hex(&current.block_hash),
            "parentHash":material::rpc_hex(&[1;32]), "timestamp":format!("{:#x}",NOW / 1000),
            "transactionsRoot":material::rpc_hex(&[4;32]), "receiptsRoot":material::rpc_hex(&results_root),
            "stateRoot":material::rpc_hex(&app_hash), "transactions":[]});
        let next_block = json!({"number":format!("{:#x}", HEIGHT+1), "hash":material::rpc_hex(&next.block_hash),
            "parentHash":material::rpc_hex(&current.block_hash), "timestamp":format!("{:#x}",NOW / 1000),
            "transactionsRoot":material::rpc_hex(&[4;32]), "receiptsRoot":material::rpc_hex(&results_root),
            "stateRoot":material::rpc_hex(&app_hash), "transactions":[]});
        let storage = Arc::new(Mutex::new(storage));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        must(listener.set_nonblocking(true));
        let replies = IssueRpcReplies {
            block,
            next_block,
            next,
            key,
            receipt,
            results,
            storage: storage.clone(),
            calls: calls.clone(),
            address,
            public_key,
            slot,
        };
        let peer = Peer::spawn(move |stop| serve_issue_rpc(&listener, &stop, &replies));
        assert_issue_receipt_export(&url, &raw, &canonical, &trust, key, app_hash);
        Self {
            url,
            _peer: peer,
            storage,
            calls,
            trust,
            raw,
            address,
            slot,
        }
    }
}

struct IssueOwner {
    owner: SharedAgentOwner<RemoteHumanAuthority>,
    store: Arc<Mutex<Store>>,
    credential: session::SessionCredential,
    _peer: HumanPeer,
    limit_id: LimitId,
    _workers: Vec<Peer>,
}

impl IssueOwner {
    fn open(root: &Path, node: &IssueNode, configured: bool, initialize: bool) -> Self {
        let (peer, did, tenant, public_key) = issue_peer();
        let activity = must(ActivityType::new(ModuleId::Asset, 5));
        let (mut authority, authority_worker) = authority_peer(
            peer.clone(),
            CoreIdentity {
                canonical_bytes: public_key.to_vec(),
                head_sequence: HEAD,
                revocation_sequence: 1,
                verification_level: layerx_types::verify::VerificationLevel::COMMITTED,
                frozen: false,
                authorities: vec![ProtocolAuthority::PrimaryKey(public_key)],
            },
            activity,
        );
        let (store, credential) = open_issue_session(
            root,
            node,
            initialize,
            &mut authority,
            &peer,
            (did, tenant, public_key),
        );
        let store = Arc::new(Mutex::new(store));
        let clock_root = root.join(format!("clock-{}", hex::encode(random::<4>())));
        must(fs::create_dir(&clock_root));
        must(fs::set_permissions(
            &clock_root,
            fs::Permissions::from_mode(0o700),
        ));
        let (clock, clock_worker) = clock_peer(&clock_root);
        let peers = BTreeMap::from([(
            7,
            ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
        )]);
        let config = NodeRpcConfig {
            url: node.url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        };
        let mut operations = must(ProductionHumanOperations::new(
            authority,
            must(SequencerEndpoint::new(&node.url, &[])),
            Box::new(must(NodeRpc::connect(&config))),
            store.clone(),
            &peers,
            1_048_576,
            30_000,
            clock,
        ));
        operations.install_export_trust(node.trust.clone(), Duration::from_secs(3));
        if configured {
            operations.install_state_export_source(must(StateExportSource::load(
                &root.join("slots.json"),
            )));
        }
        must(operations.authorize_subject(&peer));
        let keys = issue_key_directory(root);
        let limit = LimitConfig {
            id: LimitId([1; 16]),
            name: "state export".to_owned(),
            scope: LimitScope::Tenant([2; 32]),
            ceiling: u128::MAX,
            consumed: 0,
        };
        let mut owner = must(UnifiedAgentOwner::new(
            operations,
            store.clone(),
            &peers,
            vec![limit.clone()],
            must(SessionKeyRegistry::open(
                keys,
                vec![3; 32],
                125,
                must(fs::metadata(root)).uid(),
            )),
        ));
        let mut stored = must(store.lock());
        let installed = must(layerx_agentd::budget::install_enrolment_limits(
            &mut stored,
            &owner.budgets,
            credential.tenant(),
            &[limit],
        ));
        drop(stored);
        assert_eq!(installed.len(), 1);
        let limit_id = installed[0].limit_id;
        assert_eq!(must(owner.budgets.consumed(limit_id)), 0);
        ready_issue_owner(&mut owner);
        Self {
            owner: SharedAgentOwner::new(owner),
            store,
            credential,
            _peer: peer,
            limit_id,
            _workers: vec![authority_worker, clock_worker],
        }
    }

    fn request(&self, fact: &str, other_tenant: bool) -> AgentRpcResponse {
        let envelope = json!({"version":1,"request_id":"1752","operation":"export.offline",
            "credential":{"tenant":if other_tenant {"foreign-tenant"} else {self.credential.tenant().as_str()},
                "session_id":hex::encode(self.credential.session_id().0),"token_id":hex::encode(self.credential.token_id()),
                "generation":self.credential.generation().to_string()},
            "request":{"fact_set":[fact],"requested_rung":"committed"}});
        handle_rpc(&self.owner, &must(serde_json::to_vec(&envelope)))
    }

    fn no_state_bundle(&self) {
        assert!(!must(self.store.lock())
            .list_object_ids(self.credential.tenant(), ObjectKind::Configuration)
            .iter()
            .any(|key| key.starts_with(b"export-proof-v2:state:")));
    }
}

fn daemon_log() -> String {
    let path = lint_present(
        std::env::var_os("LAYERX_ISSUE_1752_DAEMON_LOG"),
        "focused case runs with a private daemon log",
    );
    must(fs::read_to_string(path))
}

#[test]
fn state_export_owner_rpc_retains_and_recovers_witnesses() {
    // Run the in-process daemon routing with its stderr in a private file, so
    // assertion failures include the production owner's refusal stages.
    if std::env::var_os("LAYERX_ISSUE_1752_DAEMON_LOG").is_none() {
        let logs = directory("issue-1752-daemon-log");
        let log = logs.join("daemon.log");
        let output = must(
            Command::new(must(std::env::current_exe()))
                .args([
                    "--exact",
                    "state_export_owner_rpc_retains_and_recovers_witnesses",
                    "--nocapture",
                ])
                .env("LAYERX_ISSUE_1752_DAEMON_LOG", &log)
                .stderr(must(fs::File::create(&log)))
                .output(),
        );
        assert!(
            output.status.success(),
            "focused daemon case: {}\ndaemon log: {}",
            String::from_utf8_lossy(&output.stdout),
            must(fs::read_to_string(&log))
        );
        must(fs::remove_dir_all(logs));
        return;
    }
    let _guard = must(FIXTURE_LOCK.lock());
    let node = IssueNode::start();
    let root = directory("issue-1752");
    let previous_rpc = std::env::var_os("LAYERX_AGENT_NODE_RPC_URL");
    let previous_chain = std::env::var_os("LAYERX_AGENT_NODE_CHAIN_ID");
    let previous_comet = std::env::var_os("LAYERX_AGENT_NODE_COMET_RPC_URL");
    std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &node.url);
    std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "125");
    std::env::set_var("LAYERX_AGENT_NODE_COMET_RPC_URL", &node.url);
    let mut account = [0; 32];
    account[12..].copy_from_slice(&node.address);
    let tx = transaction_hash(&node.raw);
    let fact = format!("state:{}:{}", hex::encode(tx), hex::encode(account));
    let owner = assert_issue_admission(&root, &node, account, tx, &fact);
    let refused_acquisitions = refuse_issue_storage(&node, &owner, &fact);
    let response = owner.request(&fact, false);
    assert_eq!(
        response.status,
        200,
        "production export: {}; node methods: {:?}; daemon log: {}",
        String::from_utf8_lossy(&response.body),
        must(node.calls.lock()),
        daemon_log()
    );
    let exported = decode_issue_export(&response, &fact);
    let bytes = assert_issue_export_inclusion(&exported, &node.trust);
    let acquisitions = must(node.calls.lock())
        .iter()
        .filter(|method| method.as_str() == "eth_getProof")
        .count();
    assert_eq!(acquisitions - refused_acquisitions, 1);
    drop(owner);
    let recovered = IssueOwner::open(&root, &node, true, false);
    let response = recovered.request(&fact, false);
    assert_eq!(
        response.status,
        200,
        "recovered export: {}; daemon log: {}",
        String::from_utf8_lossy(&response.body),
        daemon_log()
    );
    let response: Value = must(serde_json::from_slice(&response.body));
    let success = must(lint_present(
        layerx_sdk::agent_envelope::decode_response(200, &response),
        "required test operation",
    ));
    let exported = must(layerx_sdk::rpc_export::decode_offline_export(
        &success.value["value"],
        layerx_sdk::Operation::ExportOffline,
    ));
    assert_eq!(must(serialize_export(&exported)), bytes);
    assert_eq!(
        must(node.calls.lock())
            .iter()
            .filter(|method| method.as_str() == "eth_getProof")
            .count(),
        acquisitions
    );
    drop(recovered);
    for (name, value) in [
        ("LAYERX_AGENT_NODE_RPC_URL", previous_rpc),
        ("LAYERX_AGENT_NODE_CHAIN_ID", previous_chain),
        ("LAYERX_AGENT_NODE_COMET_RPC_URL", previous_comet),
    ] {
        match value {
            Some(value) => std::env::set_var(name, value),
            None => std::env::remove_var(name),
        }
    }
    must(fs::remove_dir_all(root));
}

struct IssueReceipt {
    canonical: Vec<u8>,
    key: layerx_client::sequencer::ReceiptKeyEntry,
    receipt: Value,
    results: Value,
    results_root: [u8; 32],
}

fn issue_receipt(raw: &[u8], address: [u8; 20], recipient: [u8; 20]) -> IssueReceipt {
    let tx = transaction_hash(raw);
    let signer = ed25519_dalek::SigningKey::from_bytes(&[31; 32]);
    let unsigned = material::protocol_receipt(raw, &[], None);
    let canonical = material::protocol_receipt(
        raw,
        &[],
        Some(
            signer
                .sign(&must(layerx_wire::hash::receipt_digest(&unsigned)))
                .to_bytes(),
        ),
    );
    let key = layerx_client::sequencer::ReceiptKeyEntry {
        key_id: [11; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: 1,
        last_height: 2000,
    };
    let receipt = json!({"rung":"receipted", "txHash":material::rpc_hex(&tx),
            "nonce":"0x3", "seq":format!("{HEIGHT:#x}"), "height":format!("{HEIGHT:#x}"),
            "index":"0x0", "timestamp":format!("{NOW:#x}"), "status":"0x1", "returnData":"0x",
            "logs":[], "gasUsed":"0x1", "cumulativeGasUsed":"0x1", "effectiveGasPrice":"0x1",
            "contractAddress":material::rpc_hex(&[0;20]), "sponsor":material::rpc_hex(&[0;20]),
            "feeDenom":"upax", "paymentDenom":"upax", "feeAmount":"0x1",
            "from":material::rpc_hex(&address), "to":material::rpc_hex(&recipient),
            "keyId":material::rpc_hex(&key.key_id), "signingKey":material::rpc_hex(&key.public_key),
            "receipt":material::rpc_hex(&canonical), "vmError":""});
    let results = json!([{"data":openssl::base64::encode_block(&material::abci_response(&receipt)), "gas_wanted":"1", "gas_used":"1"}]);
    let inclusion = must(NativeResultSource::from_results(&results, 0));
    let results_root =
        layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[&inclusion.result]);
    IssueReceipt {
        canonical,
        key,
        receipt,
        results,
        results_root,
    }
}

fn issue_checkpoints(
    app_hash: [u8; 32],
    results_root: [u8; 32],
    receipt: &mut Value,
    canonical: &[u8],
    address: [u8; 20],
) -> (
    layerx_agentd::read::ServedCheckpoint,
    layerx_agentd::read::ServedCheckpoint,
) {
    let mut current = material::committed(
        HEIGHT,
        lint_success(i64::try_from(NOW), "integer fits i64") / 1000,
        [1; 32],
        results_root,
        4,
    );
    let resign = |checkpoint: &mut layerx_agentd::read::ServedCheckpoint| {
        checkpoint.header.app_hash = app_hash;
        checkpoint.app_hash = app_hash;
        checkpoint.block_hash = checkpoint.header.block_hash();
        checkpoint.commit.block_id.hash = checkpoint.block_hash;
        for (index, vote) in checkpoint.commit.votes.clone().iter().enumerate() {
            checkpoint.commit.votes[index].signature = ed25519_dalek::SigningKey::from_bytes(
                &[lint_success(u8::try_from(index), "integer fits u8") + 1; 32],
            )
            .sign(&checkpoint.commit.vote_sign_bytes("125", vote.timestamp))
            .to_bytes();
        }
    };
    resign(&mut current);
    receipt["rung"] = json!("committed");
    receipt["blockNumber"] = json!(material::quantity(HEIGHT));
    receipt["blockHash"] = json!(material::rpc_hex(&current.block_hash));
    let mut account = [0; 32];
    account[12..].copy_from_slice(&address);
    assert_eq!(
        lint_present(
            must(layerx_wire::receipt::decode(canonical)).protocol(),
            "owned transaction receipt"
        )
        .from(),
        account,
        "receipt binding fact must name the credential's bound EVM account"
    );
    let mut next = material::committed(
        HEIGHT + 1,
        lint_success(i64::try_from(NOW), "integer fits i64") / 1000,
        current.block_hash,
        results_root,
        4,
    );
    resign(&mut next);
    (current, next)
}

struct IssueRpcReplies {
    block: Value,
    next_block: Value,
    next: layerx_agentd::read::ServedCheckpoint,
    key: layerx_client::sequencer::ReceiptKeyEntry,
    receipt: Value,
    results: Value,
    storage: Arc<Mutex<Value>>,
    calls: Arc<Mutex<Vec<String>>>,
    address: [u8; 20],
    public_key: [u8; 32],
    slot: [u8; 32],
}

fn serve_issue_rpc(listener: &TcpListener, stop: &AtomicBool, replies: &IssueRpcReplies) {
    while !stop.load(Ordering::Acquire) {
        let Some(mut stream) = incoming(listener) else {
            continue;
        };
        let request = read_issue_request(&mut stream);
        let method = lint_present(request["method"].as_str(), "required test operation");
        must(replies.calls.lock()).push(method.to_owned());
        let answer = match method {
            "eth_chainId" => json!("0x7d"),
            "eth_blockNumber" => json!(format!("{:#x}", HEIGHT + 1)),
            "eth_getBlockByNumber" => {
                if request["params"][0] == format!("{HEIGHT:#x}") {
                    replies.block.clone()
                } else {
                    assert_eq!(request["params"][0], format!("{:#x}", HEIGHT + 1));
                    replies.next_block.clone()
                }
            }
            "eth_call" => {
                let binding_call = [
                    layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX.as_slice(),
                    replies.public_key.as_slice(),
                ]
                .concat();
                assert_eq!(
                    request["params"][0]["data"],
                    material::rpc_hex(&binding_call),
                    "resolve only the authenticated credential's account"
                );
                let mut account = [0; 32];
                account[12..].copy_from_slice(&replies.address);
                json!(material::rpc_hex(&account))
            }
            "px_getReceiptKeys" => {
                json!({"keys":[{"keyId":material::rpc_hex(&replies.key.key_id),"publicKey":material::rpc_hex(&replies.key.public_key),"firstHeight":replies.key.first_height,"lastHeight":replies.key.last_height}]})
            }
            "pax_sequencerReceipt" => replies.receipt.clone(),
            "eth_getTransactionReceipt" => Value::Null,
            "commit" => {
                assert_eq!(request["params"]["height"], (HEIGHT + 1).to_string());
                json!({"signed_header":material::signed_header(&replies.next)})
            }
            "block_results" => {
                assert_eq!(request["params"]["height"], HEIGHT.to_string());
                json!({"height":HEIGHT.to_string(),"txs_results":replies.results})
            }
            "eth_getProof" => {
                assert_eq!(
                    request["params"],
                    json!([
                        material::rpc_hex(&replies.address),
                        [material::rpc_hex(&replies.slot)],
                        format!("{HEIGHT:#x}")
                    ])
                );
                must(replies.storage.lock()).clone()
            }
            other => panic!("unexpected node method {other}"),
        };
        http_response(
            &mut stream,
            &json!({"jsonrpc":"2.0","id":request["id"],"result":answer}),
        );
    }
}

fn read_issue_request(stream: &mut TcpStream) -> Value {
    let mut headers = String::new();
    let mut reader = BufReader::new(stream);
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        headers.push_str(&line);
    }
    let length = lint_present(
        headers
            .lines()
            .filter_map(|line| line.split_once(':'))
            .find(|(key, _)| key.eq_ignore_ascii_case("content-length"))
            .map(|(_, value)| must(value.trim().parse::<usize>())),
        "required test operation",
    );
    assert!(length <= 1_048_576);
    let mut bytes = vec![0; length];
    must(reader.read_exact(&mut bytes));
    must(serde_json::from_slice(&bytes))
}

fn assert_issue_receipt_export(
    url: &str,
    raw: &[u8],
    canonical: &[u8],
    trust: &ExportTrustSource,
    key: layerx_client::sequencer::ReceiptKeyEntry,
    app_hash: [u8; 32],
) {
    let tx = transaction_hash(raw);
    let endpoint = must(SequencerEndpoint::new(url, &[]));
    let bundle = must(
        layerx_agentd::read::NativeProofRequest {
            target: layerx_agent_api::proof::ProofBundleTarget::Receipt(tx),
            requested_rung: Rung::Committed,
            raw_transaction: raw,
            served_receipt: canonical,
            chain_id: "125",
            validators: trust.validator_set(),
            account_slots: &[],
        }
        .acquire(&endpoint, &must(NativeResultSource::connect(url, None))),
    );
    assert_eq!(bundle.value.verification_level, Rung::Committed);
    assert_eq!(bundle.value.receipt_key, key);
    assert!(trust.receipt_keys().contains(&bundle.value.receipt_key));
    // NativeProofRequest emits JSON comet-abci-results-v1 inclusion at H,
    // with the signed header and quorum commit at H+1. Its protobuf ABCI
    // result (not an Ethereum RLP receipt) is covered by last_results_hash;
    // H+1's app_hash authenticates storage queried at H.
    let native_inclusion: Value = must(serde_json::from_slice(
        lint_present(
            bundle.value.receipt_inclusion.as_ref(),
            "required test operation",
        )
        .as_bytes(),
    ));
    assert_eq!(native_inclusion["algorithm"], "comet-abci-results-v1");
    assert_eq!(native_inclusion["height"], HEIGHT);
    let proof_header = must(layerx_agentd::export::decode_header(
        lint_present(
            bundle.value.block_header.as_ref(),
            "required test operation",
        )
        .as_bytes(),
    ));
    assert_eq!(proof_header.height, HEIGHT + 1);
    assert_eq!(proof_header.app_hash, app_hash);
    let receipt_export = layerx_agentd::export::OfflineExport {
        facts: vec![must(layerx_agent_api::export::FactRef::new(format!(
            "receipt:{}",
            hex::encode(tx)
        )))],
        height: HEIGHT,
        bundles: vec![bundle.value],
    };
    verify(&receipt_export, trust)
        .unwrap_or_else(|error| panic!("native reader committed receipt refusal stage: {error:?}"));
}

fn issue_peer() -> (HumanPeer, Did, TenantId, [u8; 32]) {
    let public_key = must(derive_from_seed(&[3; 64], 0)).layerx_public_key();
    let did = must(Did::new(
        format!("did:layerx:{}", hex::encode(public_key)).as_bytes(),
    ));
    let principal = hex::encode(public_key);
    let tenant = must(TenantId::new(must(
        layerx_identity_binding::subject_namespace("bootstrap-service", &principal),
    )));
    let peer = HumanPeer {
        uid: 7,
        principal,
        tenant: tenant.as_str().to_owned(),
        subject: Some(HumanSubject {
            transport_tenant: "bootstrap-service".to_owned(),
            transport_principal: "bootstrap-owner".to_owned(),
            owner: must(String::from_utf8(did.as_bytes().to_vec())),
            account: format!("agent:{}:main", must(std::str::from_utf8(did.as_bytes()))),
            asset: transaction_hash(b"upax"),
            registration: None,
        }),
    };
    (peer, did, tenant, public_key)
}

fn open_issue_session(
    root: &Path,
    node: &IssueNode,
    initialize: bool,
    authority: &mut RemoteHumanAuthority,
    peer: &HumanPeer,
    (did, tenant, public_key): (Did, TenantId, [u8; 32]),
) -> (Store, session::SessionCredential) {
    let mut store = must(Store::open(root.join("store")));
    let identity = must(identity::register(
        &mut store,
        tenant.clone(),
        did.clone(),
        &mut AuthorityIdentity { authority, peer },
    ));
    let credential = if initialize {
        must(session::open(
            &mut store,
            &mut SessionRegistry::default(),
            &identity,
            OpenRequest {
                session_id: SessionId([8; 32]),
                token_id: [9; 32],
                tenant: tenant.clone(),
                agent: did,
                authority: ProtocolAuthority::PrimaryKey(public_key),
                permitted_activity_types: BTreeSet::from([5]),
                scopes: BTreeSet::from(["export".to_owned()]),
                expiry_sequence: 2000,
                expiry_seconds: Some(4_000_000_000),
                opening_client: "sdk".to_owned(),
                policy_version: "v1".to_owned(),
            },
            HEAD,
        ))
        .credential()
    } else {
        session::SessionCredential::new(tenant.clone(), SessionId([8; 32]), [9; 32], 1)
    };
    if initialize {
        let tx = must(Outbox::default().enqueue(&mut store, tenant.clone(), node.raw.clone()));
        let key = must(TenantKey::new(
            tenant.clone(),
            ObjectKind::Idempotency,
            [b"human-activity-submission-v1:".as_slice(), &tx].concat(),
        ));
        must(store.put_local(
            key,
            [&[2][..], &tx, &tx, peer.principal.as_bytes()].concat(),
        ));
    }
    (store, credential)
}

fn assert_issue_slot_selection(
    root: &Path,
    node: &IssueNode,
    account: [u8; 32],
    owner: &IssueOwner,
) {
    let selection = json!({"version":1,"selections":[{"tenant":owner.credential.tenant().as_str(),
        "account":hex::encode(account),"slots":[hex::encode(node.slot)]}]});
    let path = root.join("slots.json");
    for slots in [
        json!([]),
        json!([hex::encode(node.slot), hex::encode(node.slot)]),
        json!(vec![hex::encode(node.slot); 1025]),
    ] {
        let mut invalid = selection.clone();
        invalid["selections"][0]["slots"] = slots;
        must(fs::write(&path, must(serde_json::to_vec(&invalid))));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o600),
        ));
        assert!(StateExportSource::load(&path).is_err());
    }
    must(fs::write(&path, must(serde_json::to_vec(&selection))));
    must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o644),
    ));
    assert!(StateExportSource::load(&path).is_err());
    must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o600),
    ));
    assert!(StateExportSource::load(&path).is_ok());
}

fn assert_issue_admission(
    root: &Path,
    node: &IssueNode,
    account: [u8; 32],
    tx: [u8; 32],
    fact: &str,
) -> IssueOwner {
    let mut owner = IssueOwner::open(root, node, false, true);
    assert_issue_slot_selection(root, node, account, &owner);
    // slot_selection: "no selection for authenticated tenant and account";
    // the authenticated RPC returns owner.refused and retains no state bundle.
    assert_ne!(
        owner.request(fact, false).status,
        200,
        "absent slot selection"
    );
    owner.no_state_bundle();
    drop(owner);
    owner = IssueOwner::open(root, node, true, false);
    // Credential resolution refuses the foreign tenant before owner execution.
    assert_ne!(owner.request(fact, true).status, 200, "foreign tenant");
    let mut other_account = account;
    other_account[31] ^= 1;
    // account_binding: "requested account differs from authenticated account".
    assert_ne!(
        owner
            .request(
                &format!("state:{}:{}", hex::encode(tx), hex::encode(other_account)),
                false
            )
            .status,
        200,
        "foreign account"
    );
    // Ownership lookup: Refused for a transaction absent from the tenant's outbox.
    assert_ne!(
        owner
            .request(
                &format!("state:{}:{}", hex::encode([0x55; 32]), hex::encode(account)),
                false
            )
            .status,
        200,
        "unowned transaction"
    );
    owner.no_state_bundle();
    owner
}

fn refuse_issue_storage(node: &IssueNode, owner: &IssueOwner, fact: &str) -> usize {
    let limit_id = owner.limit_id;
    let idempotency_before = {
        let store = must(owner.store.lock());
        store
            .list_object_ids(owner.credential.tenant(), ObjectKind::Idempotency)
            .into_iter()
            .map(|id| {
                let key = must(TenantKey::new(
                    owner.credential.tenant().clone(),
                    ObjectKind::Idempotency,
                    id.clone(),
                ));
                (
                    id,
                    lint_present(store.get(&key), "required test operation")
                        .bytes()
                        .to_vec(),
                )
            })
            .collect::<Vec<_>>()
    };
    assert_eq!(must(must(owner.owner.lock()).budgets.consumed(limit_id)), 0);
    let original = must(node.storage.lock()).clone();
    let witness_calls = || {
        must(node.calls.lock())
            .iter()
            .filter(|method| method.as_str() == "eth_getProof")
            .count()
    };
    assert_eq!(witness_calls(), 0);
    for mutation in 0..4 {
        let altered = altered_issue_storage(&original, mutation);
        *must(node.storage.lock()) = altered;
        // Each request must reach storage_witness, not merely fail earlier at
        // account_binding or retain_committed_receipt (Binding { fact: 0 }).
        // Altered operation/account/slot replies are malformed RPC evidence;
        // the altered app-root proof fails ICS23 verification. All map to
        // owner.refused, without caching a response or consuming a limit.
        let refused = owner.request(fact, false);
        assert_ne!(
            refused.status,
            200,
            "invalid storage evidence {mutation}: {}",
            String::from_utf8_lossy(&refused.body)
        );
        assert_eq!(
            witness_calls(),
            mutation + 1,
            "witness refusal stage {mutation}: {}; node methods: {:?}; daemon log: {}",
            String::from_utf8_lossy(&refused.body),
            must(node.calls.lock()),
            daemon_log()
        );
        owner.no_state_bundle();
        let store = must(owner.store.lock());
        assert_eq!(
            store.list_object_ids(owner.credential.tenant(), ObjectKind::Idempotency),
            idempotency_before
                .iter()
                .map(|(id, _)| id.clone())
                .collect::<Vec<_>>(),
            "refused export must not retain an idempotency response"
        );
        for (id, bytes) in &idempotency_before {
            let key = must(TenantKey::new(
                owner.credential.tenant().clone(),
                ObjectKind::Idempotency,
                id.clone(),
            ));
            assert_eq!(
                lint_present(store.get(&key), "required test operation").bytes(),
                bytes
            );
        }
        drop(store);
        assert_eq!(must(must(owner.owner.lock()).budgets.consumed(limit_id)), 0);
    }
    *must(node.storage.lock()) = original;
    let refused_acquisitions = witness_calls();
    assert_eq!(refused_acquisitions, 4);
    refused_acquisitions
}

fn altered_issue_storage(original: &Value, mutation: usize) -> Value {
    let mut altered = original.clone();
    match mutation {
        0 => altered["storageProof"][0]["ops"][0]["type"] = json!("ics23:altered"),
        1 => altered["address"] = json!(material::rpc_hex(&[0x55; 20])),
        2 => {
            let mut key = must(openssl::base64::decode_block(lint_present(
                altered["storageProof"][0]["ops"][0]["key"].as_str(),
                "required test operation",
            )));
            key[52] ^= 1;
            altered["storageProof"][0]["ops"][0]["key"] =
                json!(openssl::base64::encode_block(&key));
        }
        _ => {
            altered["storageProof"][0]["ops"][1]["data"] =
                json!(openssl::base64::encode_block(&[0; 32]));
        }
    }
    altered
}

fn decode_issue_export(
    response: &AgentRpcResponse,
    fact: &str,
) -> layerx_agentd::export::OfflineExport {
    let response: Value = must(serde_json::from_slice(&response.body));
    let success = must(lint_present(
        layerx_sdk::agent_envelope::decode_response(200, &response),
        "required test operation",
    ));
    let exported = must(layerx_sdk::rpc_export::decode_offline_export(
        &success.value["value"],
        layerx_sdk::Operation::ExportOffline,
    ));
    assert_eq!(exported.facts[0].as_str(), fact);
    assert!(exported.bundles[0].account_proof.is_some());
    // The diagnostic run refused checkpoint_height: (1001, 1000). The
    // native reader commits facts at H in the signed header at H+1. The RPC
    // boundary now uses that layout and the producer reports the verified
    // checkpoint coordinate in freshness, while preserving value_height H.
    let freshness = must(layerx_agent_api::read::Freshness::decode_json(
        &success.value["freshness"].to_string(),
    ));
    let header = must(layerx_agentd::export::decode_header(
        lint_present(
            exported.bundles[0].block_header.as_ref(),
            "required test operation",
        )
        .as_bytes(),
    ));
    assert_eq!(freshness.value_height, HEIGHT);
    assert_eq!(freshness.chain_head, HEIGHT + 1);
    assert_eq!(
        freshness.relative_to,
        layerx_agent_api::read::RelativeTo::Block {
            height: HEIGHT + 1,
            hash: header.block_hash(),
        }
    );
    let request = layerx_agent_api::read::ReadRequest {
        selector: exported.facts.clone(),
        requested_rung: Rung::Committed,
    };
    let verified_response =
        layerx_agent_api::read::VerifiedRead::new(exported.clone(), Rung::Committed, freshness);
    for mutation in 0..3 {
        let mut altered = verified_response.clone();
        match mutation {
            0 => {
                altered.freshness.relative_to = layerx_agent_api::read::RelativeTo::Block {
                    height: HEIGHT,
                    hash: header.block_hash(),
                }
            }
            1 => {
                altered.freshness.relative_to = layerx_agent_api::read::RelativeTo::Block {
                    height: HEIGHT + 1,
                    hash: [0; 32],
                }
            }
            _ => {
                let mut header = header.clone();
                header.height = HEIGHT;
                altered.value.bundles[0].block_header =
                    Some(must(layerx_agent_api::prepare::CanonicalBytes::new(
                        layerx_agentd::export::encode_header(&header),
                    )));
            }
        }
        let refused = lint_present(
            layerx_agentd::agent_rpc_adapters::offline_export_operation(
                layerx_agent_api::error::RequestId(1752),
                &request,
                &altered,
            )
            .err(),
            "RPC must reject altered checkpoint coordinates",
        );
        assert_eq!(
            refused.class,
            layerx_agent_api::error::ErrorClass::VerificationFailure
        );
        assert_eq!(refused.reason, "export.invalid_proof");
    }
    exported
}

fn assert_issue_export_inclusion(
    exported: &layerx_agentd::export::OfflineExport,
    trust: &ExportTrustSource,
) -> Vec<u8> {
    let bytes = must(serialize_export(exported));
    let independent = must(ExportTrustSource::new(
        trust.receipt_keys().to_vec(),
        trust.validator_set().to_vec(),
    ));
    assert_eq!(
        must(verify(&must(deserialize_export(&bytes)), &independent)).height,
        HEIGHT
    );
    // Independent verification refuses malformed native inclusion without
    // retrying the retained RLP layout, and reports the exact binding stage.
    for (field, value, stage) in [
        ("height", json!(HEIGHT + 1), "inclusion_height_or_algorithm"),
        (
            "algorithm",
            json!("ethereum-receipt"),
            "inclusion_height_or_algorithm",
        ),
        ("total", json!(0), "inclusion_bounds"),
        ("index", json!(1), "inclusion_bounds"),
        ("result", json!("00"), "results_merkle_value"),
        (
            "leaf_hash",
            json!(hex::encode([0; 32])),
            "results_merkle_value",
        ),
    ] {
        let mut altered = exported.clone();
        let mut inclusion: Value = must(serde_json::from_slice(
            lint_present(
                altered.bundles[0].receipt_inclusion.as_ref(),
                "required test operation",
            )
            .as_bytes(),
        ));
        inclusion[field] = value;
        altered.bundles[0].receipt_inclusion = Some(must(
            layerx_agent_api::prepare::CanonicalBytes::new(must(serde_json::to_vec(&inclusion))),
        ));
        assert!(
            matches!(verify(&altered, &independent),
                Err(layerx_agentd::export::ExportProduceError::BindingAt { fact: 0, stage: refused }) if refused == stage),
            "native inclusion {field} must refuse at {stage}: {:?}",
            verify(&altered, &independent)
        );
    }
    let mut wrong_height = exported.clone();
    wrong_height.height += 1;
    assert!(matches!(
        verify(&wrong_height, &independent),
        Err(layerx_agentd::export::ExportProduceError::BindingAt {
            fact: 0,
            stage: "receipt_height"
        })
    ));
    bytes
}

fn ready_issue_owner(owner: &mut UnifiedAgentOwner<RemoteHumanAuthority>) {
    layerx_agentd::degraded::enter(
        &mut owner.degraded,
        layerx_agentd::degraded::Observation::Ready {
            reference: layerx_agentd::degraded::Reference {
                head_sequence: HEAD,
                checkpoint: [0; 32],
            },
            maximum_rung: Rung::Committed,
        },
    );
}

fn issue_key_directory(root: &Path) -> PathBuf {
    let keys = root.join(format!("keys-{}", hex::encode(random::<4>())));
    must(fs::create_dir(&keys));
    must(fs::set_permissions(
        &keys,
        fs::Permissions::from_mode(0o700),
    ));
    keys
}
