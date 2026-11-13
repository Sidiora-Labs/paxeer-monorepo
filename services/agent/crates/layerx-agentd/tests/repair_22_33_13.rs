//! Audit 22.30.100: graph-restore, native-send-envelope, registration-principal.
fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::collections::{BTreeMap, BTreeSet};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::DirBuilderExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::capability::binding::{BindingError, NativeSendAdmission};
use layerx_agentd::capability::{
    self, AttenuationError, AuthorizationKind, Capability, CapabilityDimensions, CapabilityGraph,
    CapabilityId, Effect, NativeRegistrationContext, RateCeiling, RevocableActivity,
    VerifiedLedgerPrincipal,
};
use layerx_agentd::prepare::{
    prepare_bound, AbiArgument, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
    PrecompileCall, PrecompileTarget,
};
use layerx_agentd::read::{
    checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
    CommitValidator, CommitVote, CommittedReads, ServedCheckpoint,
};
use layerx_agentd::sign::{verify_before_submit, SigningPreparation};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::account_binding::{plan_bind, BindPlan};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::account_derivation::{derive_from_seed, DerivedAccount};
use layerx_crypto::disclosure::call::{encode_payment_call, ledger_selector};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::payments::{asset_id, Payment, Registration};
use layerx_crypto::send::{send_context_hash, SendDebit};
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use layerx_wire::hash::{account_id_for_protocol, did_id_for_protocol, Domain};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

static NEXT: AtomicU64 = AtomicU64::new(0);
const ASSET: [u8; 32] = [0x35; 32];
const FEE_ASSET: [u8; 32] = [0x36; 32];
const PAYEE: [u8; 32] = [0x37; 32];

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-13-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        require_ok(
            std::fs::DirBuilder::new().mode(0o700).create(&path),
            "required test value",
        );
        Self(path)
    }
    fn open(&self) -> Store {
        require_ok(Store::open(&self.0), "required test value")
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        require_ok(std::fs::remove_dir_all(&self.0), "required test value");
    }
}
fn tenant() -> TenantId {
    require_ok(TenantId::new("repair-22-33-13"), "required test value")
}
fn graph_key() -> TenantKey {
    require_ok(
        TenantKey::new(
            tenant(),
            ObjectKind::Configuration,
            b"capability-derivation-graph".to_vec(),
        ),
        "required test value",
    )
}
fn capability(id: u8, amount: u128) -> Capability {
    require_ok(
        Capability::new(
            CapabilityId([id; 32]),
            tenant(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([5]),
                counterparties: BTreeSet::from([PAYEE]),
                assets: BTreeSet::from([ASSET]),
                amount_ceiling: amount,
                rate_ceiling: RateCeiling {
                    maximum_uses: 10,
                    window_sequences: 20,
                },
                purposes: BTreeSet::from(["rent".to_owned()]),
                expiry_sequence: 100,
            },
        ),
        "required test value",
    )
}
fn graph_bytes(graph: &CapabilityGraph) -> Vec<u8> {
    let directory = Directory::new();
    let mut store = directory.open();
    require_ok(graph.persist(&mut store), "required test value");
    require_some(store.get(&graph_key()), "required test value")
        .bytes()
        .to_vec()
}
fn reopen_graph(bytes: Vec<u8>) -> Result<Option<CapabilityGraph>, AttenuationError> {
    let directory = Directory::new();
    let mut store = directory.open();
    require_ok(store.put_local(graph_key(), bytes), "required test value");
    drop(store);
    let reopened = directory.open();
    CapabilityGraph::restore(&reopened, tenant())
}
// Split only records already emitted by CapabilityGraph::persist.
fn records(bytes: &[u8]) -> Vec<Vec<u8>> {
    let count = u16::from_be_bytes(require_ok(bytes[..2].try_into(), "required test value"));
    let mut offset = 2;
    let mut result = Vec::new();
    for _ in 0..count {
        let start = offset;
        let length = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset += 4 + length;
        let parent = bytes[offset];
        offset += 1 + if parent == 1 { 32 } else { 0 };
        offset += 1;
        result.push(bytes[start..offset].to_vec());
    }
    assert_eq!(offset, bytes.len());
    result
}
fn packed(records: &[Vec<u8>]) -> Vec<u8> {
    let mut bytes = require_ok(u16::try_from(records.len()), "required test value")
        .to_be_bytes()
        .to_vec();
    bytes.extend(records.concat());
    bytes
}
fn parent(record: &mut Vec<u8>, id: [u8; 32]) {
    let length =
        u32::from_be_bytes(require_ok(record[..4].try_into(), "required test value")) as usize;
    let end = 4 + length;
    let revoked = *require_some(record.last(), "required test value");
    record.truncate(end);
    record.push(1);
    record.extend_from_slice(&id);
    record.push(revoked);
}
fn encoded_capability(value: &Capability) -> Vec<u8> {
    let directory = Directory::new();
    let mut store = directory.open();
    require_ok(value.persist(&mut store), "required test value");
    let key = require_ok(
        TenantKey::new(
            value.tenant.clone(),
            ObjectKind::Capability,
            value.id.0.to_vec(),
        ),
        "required test value",
    );
    require_some(store.get(&key), "required test value")
        .bytes()
        .to_vec()
}
fn replace_capability(record: &mut Vec<u8>, value: &Capability) {
    let old =
        u32::from_be_bytes(require_ok(record[..4].try_into(), "required test value")) as usize;
    let suffix = record[4 + old..].to_vec();
    let value = encoded_capability(value);
    *record = (require_ok(u32::try_from(value.len()), "test value fits u32"))
        .to_be_bytes()
        .to_vec();
    record.extend(value);
    record.extend(suffix);
}

#[test]
fn graph_restore_valid_restart_preserves_ancestry_revocation_and_cancellation() {
    let directory = Directory::new();
    let mut store = directory.open();
    let mut graph = CapabilityGraph::new(tenant());
    // Parent identifiers deliberately sort after their children.
    require_ok(graph.add_root(capability(3, 100)), "required test value");
    require_ok(
        capability::attenuate(&mut graph, CapabilityId([3; 32]), capability(1, 50)),
        "required test value",
    );
    require_ok(
        capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 25)),
        "required test value",
    );
    require_ok(graph.persist(&mut store), "required test value");
    drop(graph);
    drop(store);
    let mut store = directory.open();
    let mut graph = require_some(
        CapabilityGraph::restore(&store, tenant())
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
        "required test value",
    );
    assert_eq!(
        require_some(graph.chain(CapabilityId([2; 32])), "required test value"),
        [
            CapabilityId([3; 32]),
            CapabilityId([1; 32]),
            CapabilityId([2; 32])
        ]
    );
    assert!(graph.is_enabled(CapabilityId([2; 32])));
    let mut activities = [
        RevocableActivity {
            capability_id: CapabilityId([2; 32]),
            submitted: false,
            cancelled: false,
        },
        RevocableActivity {
            capability_id: CapabilityId([1; 32]),
            submitted: true,
            cancelled: false,
        },
    ];
    let result = require_ok(
        capability::revoke_subtree(&mut graph, CapabilityId([1; 32]), &mut activities),
        "required test value",
    );
    assert_eq!(result.cancelled_unsubmitted, 1);
    assert!(activities[0].cancelled);
    assert!(!activities[1].cancelled);
    require_ok(graph.persist(&mut store), "required test value");
    drop(graph);
    drop(store);
    let store = directory.open();
    let recovered = require_some(
        CapabilityGraph::restore(&store, tenant())
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
        "required test value",
    );
    assert!(!recovered.is_enabled(CapabilityId([2; 32])));
    assert!(recovered.is_enabled(CapabilityId([3; 32])));
    assert_eq!(
        require_some(
            recovered.chain(CapabilityId([2; 32])),
            "required test value"
        )
        .len(),
        3
    );
}

#[test]
fn graph_restore_rejects_duplicate_foreign_missing_and_cyclic_ancestry() {
    let mut graph = CapabilityGraph::new(tenant());
    require_ok(graph.add_root(capability(1, 100)), "required test value");
    require_ok(
        capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 50)),
        "required test value",
    );
    let emitted = records(&graph_bytes(&graph));
    assert_eq!(
        reopen_graph(packed(&[emitted[0].clone(), emitted[0].clone()])),
        Err(AttenuationError::Duplicate)
    );
    let mut foreign = emitted.clone();
    let mut value = capability(2, 50);
    value.tenant = require_ok(TenantId::new("foreign-tenant"), "required test value");
    replace_capability(&mut foreign[1], &value);
    assert_eq!(
        reopen_graph(packed(&foreign)),
        Err(AttenuationError::Tenant)
    );
    let mut missing = emitted.clone();
    parent(&mut missing[1], [9; 32]);
    assert_eq!(
        reopen_graph(packed(&missing)),
        Err(AttenuationError::MissingParent)
    );
    let mut self_cycle = emitted.clone();
    parent(&mut self_cycle[0], [1; 32]);
    assert_eq!(
        reopen_graph(packed(&self_cycle)),
        Err(AttenuationError::Corrupt)
    );
    let mut cycle = emitted.clone();
    replace_capability(&mut cycle[1], &capability(2, 100));
    parent(&mut cycle[0], [2; 32]);
    assert_eq!(reopen_graph(packed(&cycle)), Err(AttenuationError::Corrupt));
    let mut trailing = graph_bytes(&graph);
    trailing.push(0);
    assert_eq!(reopen_graph(trailing), Err(AttenuationError::Corrupt));
    let mut malformed = emitted;
    let mut value = capability(2, 50);
    value.dimensions.activity_types.clear();
    replace_capability(&mut malformed[1], &value);
    assert!(matches!(
        reopen_graph(packed(&malformed)),
        Err(AttenuationError::Capability(_))
    ));
}

#[test]
fn graph_restore_checks_every_attenuation_dimension_and_handles_deep_valid_chains() {
    use capability::Dimension;
    let mut graph = CapabilityGraph::new(tenant());
    require_ok(graph.add_root(capability(1, 100)), "required test value");
    require_ok(
        capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 50)),
        "required test value",
    );
    let emitted = records(&graph_bytes(&graph));
    for dimension in [
        Dimension::ActivityType,
        Dimension::Counterparty,
        Dimension::Asset,
        Dimension::Amount,
        Dimension::Rate,
        Dimension::Purpose,
        Dimension::Expiry,
    ] {
        let mut value = capability(2, 50);
        match dimension {
            Dimension::ActivityType => {
                value.dimensions.activity_types.insert(6);
            }
            Dimension::Counterparty => {
                value.dimensions.counterparties.insert([99; 32]);
            }
            Dimension::Asset => {
                value.dimensions.assets.insert([99; 32]);
            }
            Dimension::Amount => value.dimensions.amount_ceiling = 101,
            Dimension::Rate => value.dimensions.rate_ceiling.maximum_uses = 11,
            Dimension::Purpose => {
                value.dimensions.purposes.insert("wider".to_owned());
            }
            Dimension::Expiry => value.dimensions.expiry_sequence = 101,
        }
        let mut altered = emitted.clone();
        replace_capability(&mut altered[1], &value);
        assert_eq!(
            reopen_graph(packed(&altered)),
            Err(AttenuationError::Wider(dimension))
        );
    }
    let mut altered = emitted;
    let mut value = capability(2, 50);
    value.dimensions.rate_ceiling.window_sequences = 19;
    replace_capability(&mut altered[1], &value);
    assert_eq!(
        reopen_graph(packed(&altered)),
        Err(AttenuationError::Wider(Dimension::Rate))
    );
    let mut graph = CapabilityGraph::new(tenant());
    require_ok(graph.add_root(capability(1, 100)), "required test value");
    for id in 2..=200 {
        require_ok(
            capability::attenuate(&mut graph, CapabilityId([id - 1; 32]), capability(id, 100)),
            "required test value",
        );
    }
    let restored = require_some(
        reopen_graph(graph_bytes(&graph))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
        "required test value",
    );
    assert_eq!(
        require_some(
            restored.chain(CapabilityId([200; 32])),
            "required test value"
        )
        .len(),
        200
    );
    assert!(restored.is_enabled(CapabilityId([200; 32])));
}

fn signed_checkpoint() -> ServedCheckpoint {
    let key = SigningKey::from_bytes(&[55; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 1,
    };
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height: 8,
        time: CommitTimestamp {
            seconds: 1000,
            nanos: 0,
        },
        last_block_id: None,
        last_commit_hash: None,
        data_hash: None,
        validators_hash: validator_set_hash(&[validator]),
        next_validators_hash: validator_set_hash(&[validator]),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: None,
        evidence_hash: None,
        proposer_address: validator.address,
    };
    let mut commit = BlockCommit {
        height: 8,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: Vec::new(),
    };
    commit.votes.push(CommitVote {
        validator_address: validator.address,
        voting_power: 1,
        timestamp: header.time,
        signature: key
            .sign(&commit.vote_sign_bytes("125", header.time))
            .to_bytes(),
    });
    require_ok(
        checkpoint(8, "125", &header, &commit, &[validator]),
        "required test value",
    )
}

#[derive(Clone)]
struct LedgerState {
    unified: Vec<u8>,
    main: [u8; 32],
    owner: [u8; 32],
    source: [u8; 32],
    source_owner: [u8; 32],
    identity: [u8; 32],
    sender: [u8; 20],
    queries: BTreeMap<String, usize>,
    submissions: Vec<Vec<u8>>,
}
struct Ledger {
    endpoint: SequencerEndpoint,
    checkpoint: ServedCheckpoint,
    state: Arc<Mutex<LedgerState>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
    account: DerivedAccount,
}
impl Ledger {
    fn new() -> Self {
        let account = require_ok(derive_from_seed(&[42; 64], 0), "required test value");
        let BindPlan::Bind(binding) =
            require_ok(plan_bind(&account, 125, None, 0), "required test value")
        else {
            panic!("new account must bind");
        };
        require_ok(binding.signed_binding().verify(), "required test value");
        let identity = account.layerx_public_key();
        let sender = account.evm_address();
        let did = require_ok(Did::new(account.did().as_bytes()), "required test value");
        let owner = require_ok(did_id_for_protocol(&did, 3), "required test value");
        let main = require_ok(
            account_id_for_protocol(
                &require_ok(
                    AccountId::parse(&format!("agent:{}:main", account.did())),
                    "required test value",
                ),
                3,
            ),
            "required test value",
        );
        let source = require_ok(
            account_id_for_protocol(
                &require_ok(
                    AccountId::for_asset(&account.did(), ASSET, FEE_ASSET),
                    "required test value",
                ),
                3,
            ),
            "required test value",
        );
        let mut address = [0; 32];
        address[12..].copy_from_slice(&sender);
        // Use the production ABI encoder for the committed account-join response.
        let unified = require_ok(
            PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: [0; 4],
                arguments: vec![
                    AbiArgument::Word(address),
                    AbiArgument::Bytes(b"pax-bound-account".to_vec()),
                    AbiArgument::Word(identity),
                    AbiArgument::Word(main),
                ],
            }
            .encode(),
            "required test value",
        )[4..]
            .to_vec();
        let state = Arc::new(Mutex::new(LedgerState {
            unified,
            main,
            owner,
            source,
            source_owner: owner,
            identity,
            sender,
            queries: BTreeMap::new(),
            submissions: Vec::new(),
        }));
        let cp = signed_checkpoint();
        let block = json!({"number":"0x8", "hash":format!("0x{}",hex::encode(cp.block_hash)),
            "parentHash":format!("0x{}",hex::encode([2;32])), "timestamp":"0x3e8",
            "transactionsRoot":format!("0x{}",hex::encode([4;32])), "receiptsRoot":format!("0x{}",hex::encode([7;32])),
            "stateRoot":format!("0x{}",hex::encode(cp.app_hash)), "transactions":[]});
        let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
        require_ok(listener.set_nonblocking(true), "required test value");
        let endpoint = require_ok(
            SequencerEndpoint::new(
                &format!(
                    "http://{}/",
                    require_ok(listener.local_addr(), "required test value")
                ),
                &[],
            ),
            "required test value",
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let served = Arc::clone(&state);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(stream) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                finish_new_2_repair_22_33_13(&block, &served, stream);
            }
        });
        Self {
            endpoint,
            checkpoint: cp,
            state,
            stop,
            worker: Some(worker),
            account,
        }
    }
    fn reads(&self) -> CommittedReads<'_> {
        require_ok(
            CommittedReads::at_checkpoint(&self.endpoint, &self.checkpoint),
            "required test value",
        )
    }
    fn owner(&self) -> [u8; 32] {
        require_ok(self.state.lock(), "required test value").owner
    }
    fn main(&self) -> [u8; 32] {
        require_ok(self.state.lock(), "required test value").main
    }
    fn source(&self) -> [u8; 32] {
        require_ok(self.state.lock(), "required test value").source
    }
    fn prepare(&self, call: PrecompileCall, nonce: u64) -> SigningPreparation {
        SigningPreparation {
            actor_address: self.account.evm_address(),
            transaction: require_ok(
                prepare_bound(
                    EvmPreparationState {
                        chain_id: 125,
                        pending_nonce: nonce,
                        base_fee_per_gas: 2,
                        head_height: 8,
                    },
                    EvmPreparationDefaults {
                        max_priority_fee_per_gas: 1,
                        max_fee_per_gas: 10,
                        maximum_payload_bytes: 8192,
                    },
                    EvmPrepareRequest {
                        from: self.account.evm_address(),
                        call,
                        expected_nonce: Some(nonce),
                        max_priority_fee_per_gas: None,
                        max_fee_per_gas: None,
                        declared_payload_limit: 8192,
                    },
                    100_000,
                ),
                "required test value",
            ),
        }
    }
    fn send(&self, amount: u128, nonce: u64) -> SigningPreparation {
        self.send_with_identity(
            amount,
            nonce,
            &SigningKey::from_bytes(self.account.layerx_seed()),
        )
    }
    fn send_with_identity(&self, amount: u128, nonce: u64, key: &SigningKey) -> SigningPreparation {
        let from = self.source();
        let debit = SendDebit {
            from,
            to: PAYEE,
            asset: ASSET,
            amount,
            source_sequence: 3,
            idempotency_key: [8; 32],
            expires_at: 10_000,
            context_hash: send_context_hash(&from, &PAYEE, &ASSET, amount, &[8; 32]),
            conditions: Vec::new(),
            authorization_kind: 1,
            network_id: 125,
            protocol_version: 3,
        };
        let canonical = require_ok(debit.authorization_message(), "required test value");
        let message = require_ok(
            layerx_crypto::SignatureMessage::new(Domain::SignaturePreimage, 3, 125, &canonical),
            "required test value",
        );
        let payload = require_ok(
            debit.encode_signed(
                key.verifying_key().to_bytes(),
                key.sign(&message.digest()).to_bytes(),
            ),
            "required test value",
        );
        self.prepare(PrecompileCall::ledger_transfer(payload), nonce)
    }
    fn register(&self, supply_cap: u128) -> SigningPreparation {
        let registration = Registration {
            asset: asset_id(&self.owner(), &[9; 32]),
            salt: [9; 32],
            symbol: "NEW".into(),
            name: "Registered asset".into(),
            decimals: 6,
            supply_cap,
            issuer_kind: 1,
            custody_ref: Vec::new(),
        };
        let encoded = require_ok(
            encode_payment_call(
                &Payment::Register(registration),
                self.account.did().as_bytes(),
            ),
            "required test value",
        );
        let mut call = PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: require_ok(encoded[..4].try_into(), "required test value"),
            arguments: Vec::new(),
        };
        // Decode the encoder's canonical arguments without substituting another registration shape.
        let salt = [9; 32];
        let asset = asset_id(&self.owner(), &salt);
        let mut decimals = [0; 32];
        decimals[31] = 6;
        let mut cap = [0; 32];
        cap[16..].copy_from_slice(&supply_cap.to_be_bytes());
        let mut kind = [0; 32];
        kind[31] = 1;
        call.arguments = vec![
            AbiArgument::Word(asset),
            AbiArgument::Word(salt),
            AbiArgument::Bytes(b"NEW".to_vec()),
            AbiArgument::Bytes(b"Registered asset".to_vec()),
            AbiArgument::Word(decimals),
            AbiArgument::Word(cap),
            AbiArgument::Word(kind),
            AbiArgument::Bytes(Vec::new()),
        ];
        assert_eq!(require_ok(call.encode(), "required test value"), encoded);
        self.prepare(call, 7)
    }
    fn raw(&self, preparation: &SigningPreparation) -> Vec<u8> {
        require_ok(
            preparation.transaction.call.sign(require_some(
                self.account.evm_secret(),
                "required test value",
            )),
            "required test value",
        )
    }
}

fn finish_new_2_repair_22_33_13(
    block: &Value,
    served: &Arc<Mutex<LedgerState>>,
    stream: std::net::TcpStream,
) {
    require_ok(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "required test value",
    );
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(
            require_ok(reader.read_line(&mut line), "required test value"),
            0
        );
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(require_ok(
                    value.trim().parse::<usize>(),
                    "required test value",
                ));
            }
        }
    }
    let mut body = vec![0; require_some(length, "required test value")];
    require_ok(reader.read_exact(&mut body), "required test value");
    let request: Value = require_ok(serde_json::from_slice(&body), "required test value");
    let result = match require_some(request["method"].as_str(), "required test value") {
        "eth_getBlockByNumber" => {
            assert_eq!(request["params"][0], "0x8");
            block.clone()
        }
        "eth_sendRawTransaction" => {
            let raw = require_ok(
                hex::decode(require_some(
                    request["params"][0]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "required test value"))
                        .strip_prefix("0x"),
                    "required test value",
                )),
                "required test value",
            );
            let (call, signature) = require_ok(decode_signed(&raw), "required test value");
            let mut state = require_ok(served.lock(), "required test value");
            assert_eq!(
                require_ok(recover_sender(&call, &signature), "required test value"),
                state.sender
            );
            assert_eq!(call.chain_id, 125);
            assert_eq!(call.to, PrecompileTarget::Ledger.address());
            let hash = transaction_hash(&raw);
            state.submissions.push(raw);
            json!(format!("0x{}", hex::encode(hash)))
        }
        "eth_call" => {
            assert_eq!(request["params"][1], "0x8");
            finish_new_1_repair_22_33_13(served, &request)
        }
        method => panic!("unexpected request {method}"),
    };
    let response = json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
    require_ok(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}",response.len()), "required test value");
    require_ok(reader.get_mut().flush(), "required test value");
}

fn finish_new_1_repair_22_33_13(served: &Arc<Mutex<LedgerState>>, request: &Value) -> Value {
    let data = require_ok(
        hex::decode(require_some(
            request["params"][0]["data"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "required test value"))
                .strip_prefix("0x"),
            "required test value",
        )),
        "required test value",
    );
    let mut state = require_ok(served.lock(), "required test value");
    let selector = &data[..4];
    let (name, bytes) = if selector == ledger_selector("getUnifiedAccount(address)") {
        assert_eq!(
            request["params"][0]["to"],
            "0x0000000000000000000000000000000000001004"
        );
        assert_eq!(data[4..16], [0; 12]);
        assert_eq!(data[16..], state.sender);
        ("unified", state.unified.clone())
    } else {
        assert_eq!(
            request["params"][0]["to"],
            "0x000000000000000000000000000000000000101a"
        );
        if selector == ledger_selector("accountId(bytes32,bytes32)") {
            assert_eq!(data[4..36], state.identity);
            assert_eq!(data[36..], ASSET);
            ("account", state.source.to_vec())
        } else {
            assert_eq!(selector, ledger_selector("accountOwner(bytes32)"));
            if data[4..] == state.main {
                ("main_owner", state.owner.to_vec())
            } else {
                assert_eq!(data[4..], state.source);
                ("source_owner", state.source_owner.to_vec())
            }
        }
    };
    *state.queries.entry(name.into()).or_default() += 1;
    json!(format!("0x{}", hex::encode(bytes)))
}
impl Drop for Ledger {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        require_ok(
            self.worker
                .take()
                .unwrap_or_else(|| panic!("{}", "required test value"))
                .join(),
            "required test value",
        );
    }
}

#[test]
fn native_send_envelope_signed_evm_admission_reopens_exact_bytes_and_consumed_nonce() {
    let ledger = Ledger::new();
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
    let admission = require_ok(
        NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()),
        "required test value",
    );
    assert_eq!(admission.exact_bytes(), raw);
    assert_eq!(admission.principal().sender(), ledger.account.evm_address());
    assert_eq!(admission.principal().issuer(), ledger.owner());
    assert_eq!(admission.principal().fee_account(), ledger.main());
    let (call, signature) = require_ok(
        decode_signed(admission.exact_bytes()),
        "required test value",
    );
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.to, PrecompileTarget::Ledger.address());
    assert_eq!(call.data, prepared.transaction.call.data);
    assert_eq!(
        require_ok(recover_sender(&call, &signature), "required test value"),
        ledger.account.evm_address()
    );
    let directory = Directory::new();
    let mut store = directory.open();
    let reference = admission.preparation_id();
    require_ok(admission.admit(&mut store, tenant()), "required test value");
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    let hash = require_ok(
        outbox.enqueue_verified(&mut store, tenant(), admission.submission().clone()),
        "required test value",
    );
    assert_eq!(
        require_ok(outbox.bytes_for_transmission(hash), "required test value"),
        raw
    );
    drop(outbox);
    assert!(matches!(
        admission.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    drop(admission);
    drop(store);
    let store = directory.open();
    let recovered = require_some(
        NativeSendAdmission::restore(&store, tenant(), reference, &registry, &ledger.reads())
            .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value")),
        "required test value",
    );
    assert_eq!(recovered.exact_bytes(), raw);
    assert_eq!(recovered.preparation(), &prepared);
    assert_eq!(
        transaction_hash(recovered.exact_bytes()),
        transaction_hash(&raw)
    );
    complete_native_send_envelope_signed_evm_admission_reopens_exact_bytes_and_consumed_nonce_1_repair_22_33_13(CompleteNativeSendEnvelopeSignedEvmAdmissionReopensExactBytesAndConsumedNonce1Repair223313Context { ledger, raw, registry, reference, hash, store, recovered });
}

struct CompleteNativeSendEnvelopeSignedEvmAdmissionReopensExactBytesAndConsumedNonce1Repair223313Context
{
    ledger: Ledger,
    raw: Vec<u8>,
    registry: ModuleRegistry,
    reference: [u8; 32],
    hash: [u8; 32],
    store: Store,
    recovered: NativeSendAdmission,
}

fn complete_native_send_envelope_signed_evm_admission_reopens_exact_bytes_and_consumed_nonce_1_repair_22_33_13(
    context: CompleteNativeSendEnvelopeSignedEvmAdmissionReopensExactBytesAndConsumedNonce1Repair223313Context,
) {
    let CompleteNativeSendEnvelopeSignedEvmAdmissionReopensExactBytesAndConsumedNonce1Repair223313Context { ledger, raw, registry, reference, hash, mut store, recovered } = context;

    let mut outbox = layerx_agentd::outbox::Outbox::default();
    require_ok(
        outbox.restore(&store, tenant(), hash),
        "required test value",
    );
    let retained = require_ok(outbox.bytes_for_transmission(hash), "required test value");
    assert_eq!(
        require_ok(
            ledger.endpoint.send_raw_transaction(retained),
            "required test value"
        ),
        hash
    );
    assert_eq!(
        require_some(
            ledger
                .state
                .lock()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .submissions
                .last(),
            "required test value"
        ),
        &raw
    );
    assert!(matches!(
        recovered.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    let competing = ledger.send(26, 7);
    let competing = require_ok(
        NativeSendAdmission::verify(
            &competing,
            &ledger.raw(&competing),
            &registry,
            &ledger.reads(),
        ),
        "required test value",
    );
    assert!(matches!(
        competing.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    assert!(require_ok(
        NativeSendAdmission::restore(
            &store,
            require_ok(TenantId::new("other"), "required test value"),
            reference,
            &registry,
            &ledger.reads()
        ),
        "required test value"
    )
    .is_none());
    let queries = &require_ok(ledger.state.lock(), "required test value").queries;
    for name in ["unified", "main_owner", "account", "source_owner"] {
        assert!(queries[name] >= 2);
    }
}

#[test]
fn native_send_envelope_refuses_changed_transaction_binding_and_foreign_ledger_owner() {
    let ledger = Ledger::new();
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
    assert!(NativeSendAdmission::verify(
        &prepared,
        &prepared.transaction.call.encode_unsigned(),
        &registry,
        &ledger.reads()
    )
    .is_err());
    for field in 0..6 {
        let mut changed = prepared.clone();
        match field {
            0 => changed.transaction.call.chain_id = 126,
            1 => changed.transaction.call.nonce += 1,
            2 => changed.transaction.call.to = PrecompileTarget::Pay.address(),
            3 => {
                let last = changed.transaction.call.data.len() - 1;
                changed.transaction.call.data[last] ^= 1;
            }
            4 => changed.actor_address[0] ^= 1,
            _ => changed.transaction.from[0] ^= 1,
        }
        assert!(NativeSendAdmission::verify(&changed, &raw, &registry, &ledger.reads()).is_err());
    }
    let other = require_ok(derive_from_seed(&[43; 64], 0), "required test value");
    let other_raw = require_ok(
        prepared
            .transaction
            .call
            .sign(require_some(other.evm_secret(), "required test value")),
        "required test value",
    );
    assert!(
        NativeSendAdmission::verify(&prepared, &other_raw, &registry, &ledger.reads()).is_err()
    );
    let mut trailing = raw.clone();
    trailing.push(0);
    assert!(NativeSendAdmission::verify(&prepared, &trailing, &registry, &ledger.reads()).is_err());
    let foreign_identity = ledger.send_with_identity(25, 7, &SigningKey::from_bytes(&[77; 32]));
    assert!(NativeSendAdmission::verify(
        &foreign_identity,
        &ledger.raw(&foreign_identity),
        &registry,
        &ledger.reads()
    )
    .is_err());
    require_ok(ledger.state.lock(), "required test value").source_owner = [99; 32];
    assert!(NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()).is_err());
}

#[test]
fn native_send_envelope_restart_rechecks_retained_bytes_and_current_binding() {
    let ledger = Ledger::new();
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let admission = require_ok(
        NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()),
        "required test value",
    );
    let directory = Directory::new();
    let mut store = directory.open();
    require_ok(admission.admit(&mut store, tenant()), "required test value");
    let reference = admission.preparation_id();
    let mut object = b"native-send-evm-v1:".to_vec();
    object.extend_from_slice(&reference);
    let key = require_ok(
        TenantKey::new(tenant(), ObjectKind::PreparedActivity, object),
        "required test value",
    );
    let original = require_some(store.get(&key), "required test value")
        .bytes()
        .to_vec();
    let mut record: Value = require_ok(serde_json::from_slice(&original), "required test value");
    record[5][0] = json!(0x01);
    require_ok(
        store.put_local(
            key.clone(),
            require_ok(serde_json::to_vec(&record), "required test value"),
        ),
        "required test value",
    );
    drop(store);
    let mut store = directory.open();
    assert!(
        NativeSendAdmission::restore(&store, tenant(), reference, &registry, &ledger.reads())
            .is_err()
    );
    require_ok(store.put_local(key, original), "required test value");
    drop(store);
    let store = directory.open();
    require_ok(ledger.state.lock(), "required test value").owner = [91; 32];
    assert!(
        NativeSendAdmission::restore(&store, tenant(), reference, &registry, &ledger.reads())
            .is_err()
    );
}

#[test]
fn registration_principal_uses_verified_evm_ledger_owner_and_same_fee_account() {
    let ledger = Ledger::new();
    let prepared = ledger.register(1_000);
    let raw = ledger.raw(&prepared);
    let registry = require_ok(ModuleRegistry::new(&[]), "required test value");
    assert_eq!(
        require_ok(
            verify_before_submit(&raw, &prepared, &ledger.account.evm_address(), &registry),
            "required test value"
        )
        .exact_bytes(),
        raw
    );
    let principal = require_ok(
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()),
        "required test value",
    );
    let context = NativeRegistrationContext::new(principal, FEE_ASSET);
    let plan = require_ok(
        capability::derive_native_registration(&prepared.transaction.disclosure, context),
        "required test value",
    );
    assert_eq!(
        plan.effects(),
        [Effect::Authorization {
            kind: AuthorizationKind::SupplyCap,
            account: ledger.main(),
            asset: Some(asset_id(&ledger.owner(), &[9; 32])),
            amount: 1_000
        }]
    );
    assert_eq!(principal.fee_account(), ledger.main());
    assert_eq!(principal.issuer(), ledger.owner());
    assert!(plan.gross_per_asset().is_empty());
    assert_eq!(plan.participants(), &BTreeSet::from([ledger.main()]));
    assert_eq!(plan.rate_actions(), 1);
    let digest: [u8; 32] = Sha256::digest(prepared.transaction.call.encode_unsigned()).into();
    let commitment = require_ok(
        capability::native_registration_commitment(
            &prepared.transaction.disclosure,
            3,
            125,
            digest,
            context,
        ),
        "required test value",
    );
    assert_ne!(
        commitment,
        require_ok(
            capability::native_registration_commitment(
                &prepared.transaction.disclosure,
                3,
                125,
                digest,
                NativeRegistrationContext::new(principal, [38; 32])
            ),
            "required test value"
        )
    );
    let directory = Directory::new();
    let mut store = directory.open();
    require_ok(
        prepared.retain(&mut store, tenant(), digest),
        "required test value",
    );
    drop(store);
    let store = directory.open();
    let recovered = require_ok(
        SigningPreparation::restore(&store, tenant(), digest),
        "required test value",
    );
    let principal = require_ok(
        VerifiedLedgerPrincipal::resolve(&recovered.transaction, &ledger.reads()),
        "required test value",
    );
    let recovered_context = NativeRegistrationContext::new(principal, FEE_ASSET);
    assert_eq!(
        plan,
        require_ok(
            capability::derive_native_registration(
                &recovered.transaction.disclosure,
                recovered_context
            ),
            "required test value"
        )
    );
    assert_eq!(
        commitment,
        require_ok(
            capability::native_registration_commitment(
                &recovered.transaction.disclosure,
                3,
                125,
                digest,
                recovered_context
            ),
            "required test value"
        )
    );
}

#[test]
fn registration_principal_refuses_missing_binding_cross_preparation_and_unbounded_supply() {
    let ledger = Ledger::new();
    let prepared = ledger.register(1000);
    let principal = require_ok(
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()),
        "required test value",
    );
    let context = NativeRegistrationContext::new(principal, FEE_ASSET);
    assert!(
        capability::derive_native_registration(&prepared.transaction.disclosure, FEE_ASSET)
            .is_err()
    );
    assert!(capability::derive_native_registration(
        &prepared.transaction.disclosure,
        NativeRegistrationContext::new(principal, [0; 32])
    )
    .is_err());
    for supply in [0, u128::MAX] {
        let changed = ledger.register(supply);
        let principal = require_ok(
            VerifiedLedgerPrincipal::resolve(&changed.transaction, &ledger.reads()),
            "required test value",
        );
        assert!(capability::derive_native_registration(
            &changed.transaction.disclosure,
            NativeRegistrationContext::new(principal, FEE_ASSET)
        )
        .is_err());
    }
    let changed = ledger.register(1001);
    assert!(
        capability::derive_native_registration(&changed.transaction.disclosure, context).is_err()
    );
    let mut altered = prepared.transaction.disclosure.clone();
    if let Some(Payment::Register(value)) = &mut altered.payment {
        value.supply_cap += 1;
    }
    assert!(capability::derive_native_registration(&altered, context).is_err());
    let digest: [u8; 32] = Sha256::digest(prepared.transaction.call.encode_unsigned()).into();
    for (version, network, hash) in [(2, 125, digest), (3, 126, digest), (3, 125, [0; 32])] {
        assert!(capability::native_registration_commitment(
            &prepared.transaction.disclosure,
            version,
            network,
            hash,
            context
        )
        .is_err());
    }
    let mut unbound = require_ok(ledger.state.lock(), "required test value")
        .unified
        .clone();
    unbound[64..96].fill(0);
    require_ok(ledger.state.lock(), "required test value").unified = unbound;
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
}

#[test]
fn registration_principal_refuses_owner_drift_and_unrelated_evm_join() {
    let ledger = Ledger::new();
    let prepared = ledger.register(1_000);
    let original = ledger.owner();
    require_ok(ledger.state.lock(), "required test value").owner = [93; 32];
    let principal = require_ok(
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()),
        "required test value",
    );
    assert!(capability::derive_native_registration(
        &prepared.transaction.disclosure,
        NativeRegistrationContext::new(principal, FEE_ASSET)
    )
    .is_err());
    require_ok(ledger.state.lock(), "required test value").owner = [0; 32];
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
    require_ok(ledger.state.lock(), "required test value").owner = original;
    require_ok(ledger.state.lock(), "required test value").unified[31] ^= 1;
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
}
