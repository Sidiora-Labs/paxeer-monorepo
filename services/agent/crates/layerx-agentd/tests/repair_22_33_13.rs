//! Audit 22.30.100: graph-restore, native-send-envelope, registration-principal.
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
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&path)
            .unwrap();
        Self(path)
    }
    fn open(&self) -> Store {
        Store::open(&self.0).unwrap()
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).unwrap();
    }
}
fn tenant() -> TenantId {
    TenantId::new("repair-22-33-13").unwrap()
}
fn graph_key() -> TenantKey {
    TenantKey::new(
        tenant(),
        ObjectKind::Configuration,
        b"capability-derivation-graph".to_vec(),
    )
    .unwrap()
}
fn capability(id: u8, amount: u128) -> Capability {
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
    )
    .unwrap()
}
fn graph_bytes(graph: &CapabilityGraph) -> Vec<u8> {
    let directory = Directory::new();
    let mut store = directory.open();
    graph.persist(&mut store).unwrap();
    store.get(&graph_key()).unwrap().bytes().to_vec()
}
fn reopen_graph(bytes: Vec<u8>) -> Result<Option<CapabilityGraph>, AttenuationError> {
    let directory = Directory::new();
    let mut store = directory.open();
    store.put_local(graph_key(), bytes).unwrap();
    drop(store);
    let reopened = directory.open();
    CapabilityGraph::restore(&reopened, tenant())
}
// Split only records already emitted by CapabilityGraph::persist.
fn records(bytes: &[u8]) -> Vec<Vec<u8>> {
    let count = u16::from_be_bytes(bytes[..2].try_into().unwrap());
    let mut offset = 2;
    let mut result = Vec::new();
    for _ in 0..count {
        let start = offset;
        let length = u32::from_be_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize;
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
    let mut bytes = u16::try_from(records.len()).unwrap().to_be_bytes().to_vec();
    bytes.extend(records.concat());
    bytes
}
fn parent(record: &mut Vec<u8>, id: [u8; 32]) {
    let length = u32::from_be_bytes(record[..4].try_into().unwrap()) as usize;
    let end = 4 + length;
    let revoked = *record.last().unwrap();
    record.truncate(end);
    record.push(1);
    record.extend_from_slice(&id);
    record.push(revoked);
}
fn encoded_capability(value: &Capability) -> Vec<u8> {
    let directory = Directory::new();
    let mut store = directory.open();
    value.persist(&mut store).unwrap();
    let key = TenantKey::new(
        value.tenant.clone(),
        ObjectKind::Capability,
        value.id.0.to_vec(),
    )
    .unwrap();
    store.get(&key).unwrap().bytes().to_vec()
}
fn replace_capability(record: &mut Vec<u8>, value: &Capability) {
    let old = u32::from_be_bytes(record[..4].try_into().unwrap()) as usize;
    let suffix = record[4 + old..].to_vec();
    let value = encoded_capability(value);
    *record = (value.len() as u32).to_be_bytes().to_vec();
    record.extend(value);
    record.extend(suffix);
}

#[test]
fn graph_restore_valid_restart_preserves_ancestry_revocation_and_cancellation() {
    let directory = Directory::new();
    let mut store = directory.open();
    let mut graph = CapabilityGraph::new(tenant());
    // Parent identifiers deliberately sort after their children.
    graph.add_root(capability(3, 100)).unwrap();
    capability::attenuate(&mut graph, CapabilityId([3; 32]), capability(1, 50)).unwrap();
    capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 25)).unwrap();
    graph.persist(&mut store).unwrap();
    drop(graph);
    drop(store);
    let mut store = directory.open();
    let mut graph = CapabilityGraph::restore(&store, tenant()).unwrap().unwrap();
    assert_eq!(
        graph.chain(CapabilityId([2; 32])).unwrap(),
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
    let result =
        capability::revoke_subtree(&mut graph, CapabilityId([1; 32]), &mut activities).unwrap();
    assert_eq!(result.cancelled_unsubmitted, 1);
    assert!(activities[0].cancelled);
    assert!(!activities[1].cancelled);
    graph.persist(&mut store).unwrap();
    drop(graph);
    drop(store);
    let store = directory.open();
    let recovered = CapabilityGraph::restore(&store, tenant()).unwrap().unwrap();
    assert!(!recovered.is_enabled(CapabilityId([2; 32])));
    assert!(recovered.is_enabled(CapabilityId([3; 32])));
    assert_eq!(recovered.chain(CapabilityId([2; 32])).unwrap().len(), 3);
}

#[test]
fn graph_restore_rejects_duplicate_foreign_missing_and_cyclic_ancestry() {
    let mut graph = CapabilityGraph::new(tenant());
    graph.add_root(capability(1, 100)).unwrap();
    capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 50)).unwrap();
    let emitted = records(&graph_bytes(&graph));
    assert_eq!(
        reopen_graph(packed(&[emitted[0].clone(), emitted[0].clone()])),
        Err(AttenuationError::Duplicate)
    );
    let mut foreign = emitted.clone();
    let mut value = capability(2, 50);
    value.tenant = TenantId::new("foreign-tenant").unwrap();
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
    graph.add_root(capability(1, 100)).unwrap();
    capability::attenuate(&mut graph, CapabilityId([1; 32]), capability(2, 50)).unwrap();
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
    graph.add_root(capability(1, 100)).unwrap();
    for id in 2..=200 {
        capability::attenuate(&mut graph, CapabilityId([id - 1; 32]), capability(id, 100)).unwrap();
    }
    let restored = reopen_graph(graph_bytes(&graph)).unwrap().unwrap();
    assert_eq!(restored.chain(CapabilityId([200; 32])).unwrap().len(), 200);
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
    checkpoint(8, "125", &header, &commit, &[validator]).unwrap()
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
        let account = derive_from_seed(&[42; 64], 0).unwrap();
        let BindPlan::Bind(binding) = plan_bind(&account, 125, None, 0).unwrap() else {
            panic!("new account must bind");
        };
        binding.signed_binding().verify().unwrap();
        let identity = account.layerx_public_key();
        let sender = account.evm_address();
        let did = Did::new(account.did().as_bytes()).unwrap();
        let owner = did_id_for_protocol(&did, 3).unwrap();
        let main = account_id_for_protocol(
            &AccountId::parse(&format!("agent:{}:main", account.did())).unwrap(),
            3,
        )
        .unwrap();
        let source = account_id_for_protocol(
            &AccountId::for_asset(&account.did(), ASSET, FEE_ASSET).unwrap(),
            3,
        )
        .unwrap();
        let mut address = [0; 32];
        address[12..].copy_from_slice(&sender);
        // Use the production ABI encoder for the committed account-join response.
        let unified = PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: [0; 4],
            arguments: vec![
                AbiArgument::Word(address),
                AbiArgument::Bytes(b"pax-bound-account".to_vec()),
                AbiArgument::Word(identity),
                AbiArgument::Word(main),
            ],
        }
        .encode()
        .unwrap()[4..]
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
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let endpoint =
            SequencerEndpoint::new(&format!("http://{}/", listener.local_addr().unwrap()), &[])
                .unwrap();
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
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().unwrap());
                        }
                    }
                }
                let mut body = vec![0; length.unwrap()];
                reader.read_exact(&mut body).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                let result = match request["method"].as_str().unwrap() {
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"][0], "0x8");
                        block.clone()
                    }
                    "eth_sendRawTransaction" => {
                        let raw = hex::decode(
                            request["params"][0]
                                .as_str()
                                .unwrap()
                                .strip_prefix("0x")
                                .unwrap(),
                        )
                        .unwrap();
                        let (call, signature) = decode_signed(&raw).unwrap();
                        let mut state = served.lock().unwrap();
                        assert_eq!(recover_sender(&call, &signature).unwrap(), state.sender);
                        assert_eq!(call.chain_id, 125);
                        assert_eq!(call.to, PrecompileTarget::Ledger.address());
                        let hash = transaction_hash(&raw);
                        state.submissions.push(raw);
                        json!(format!("0x{}", hex::encode(hash)))
                    }
                    "eth_call" => {
                        assert_eq!(request["params"][1], "0x8");
                        let data = hex::decode(
                            request["params"][0]["data"]
                                .as_str()
                                .unwrap()
                                .strip_prefix("0x")
                                .unwrap(),
                        )
                        .unwrap();
                        let mut state = served.lock().unwrap();
                        let selector = &data[..4];
                        let (name, bytes) =
                            if selector == ledger_selector("getUnifiedAccount(address)") {
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
                    method => panic!("unexpected request {method}"),
                };
                let response =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}",response.len()).unwrap();
                reader.get_mut().flush().unwrap();
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
        CommittedReads::at_checkpoint(&self.endpoint, &self.checkpoint).unwrap()
    }
    fn owner(&self) -> [u8; 32] {
        self.state.lock().unwrap().owner
    }
    fn main(&self) -> [u8; 32] {
        self.state.lock().unwrap().main
    }
    fn source(&self) -> [u8; 32] {
        self.state.lock().unwrap().source
    }
    fn prepare(&self, call: PrecompileCall, nonce: u64) -> SigningPreparation {
        SigningPreparation {
            actor_address: self.account.evm_address(),
            transaction: prepare_bound(
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
            )
            .unwrap(),
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
        let canonical = debit.authorization_message().unwrap();
        let message =
            layerx_crypto::SignatureMessage::new(Domain::SignaturePreimage, 3, 125, &canonical)
                .unwrap();
        let payload = debit
            .encode_signed(
                key.verifying_key().to_bytes(),
                key.sign(&message.digest()).to_bytes(),
            )
            .unwrap();
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
        let encoded = encode_payment_call(
            &Payment::Register(registration),
            self.account.did().as_bytes(),
        )
        .unwrap();
        let mut call = PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: encoded[..4].try_into().unwrap(),
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
        assert_eq!(call.encode().unwrap(), encoded);
        self.prepare(call, 7)
    }
    fn raw(&self, preparation: &SigningPreparation) -> Vec<u8> {
        preparation
            .transaction
            .call
            .sign(self.account.evm_secret().unwrap())
            .unwrap()
    }
}
impl Drop for Ledger {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker.take().unwrap().join().unwrap();
    }
}

#[test]
fn native_send_envelope_signed_evm_admission_reopens_exact_bytes_and_consumed_nonce() {
    let ledger = Ledger::new();
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let registry = ModuleRegistry::new(&[]).unwrap();
    let admission =
        NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()).unwrap();
    assert_eq!(admission.exact_bytes(), raw);
    assert_eq!(admission.principal().sender(), ledger.account.evm_address());
    assert_eq!(admission.principal().issuer(), ledger.owner());
    assert_eq!(admission.principal().fee_account(), ledger.main());
    let (call, signature) = decode_signed(admission.exact_bytes()).unwrap();
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.to, PrecompileTarget::Ledger.address());
    assert_eq!(call.data, prepared.transaction.call.data);
    assert_eq!(
        recover_sender(&call, &signature).unwrap(),
        ledger.account.evm_address()
    );
    let directory = Directory::new();
    let mut store = directory.open();
    let reference = admission.preparation_id();
    admission.admit(&mut store, tenant()).unwrap();
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    let hash = outbox
        .enqueue_verified(&mut store, tenant(), admission.submission().clone())
        .unwrap();
    assert_eq!(outbox.bytes_for_transmission(hash).unwrap(), raw);
    drop(outbox);
    assert!(matches!(
        admission.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    drop(admission);
    drop(store);
    let mut store = directory.open();
    let recovered =
        NativeSendAdmission::restore(&store, tenant(), reference, &registry, &ledger.reads())
            .unwrap()
            .unwrap();
    assert_eq!(recovered.exact_bytes(), raw);
    assert_eq!(recovered.preparation(), &prepared);
    assert_eq!(
        transaction_hash(recovered.exact_bytes()),
        transaction_hash(&raw)
    );
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    outbox.restore(&store, tenant(), hash).unwrap();
    let retained = outbox.bytes_for_transmission(hash).unwrap();
    assert_eq!(
        ledger.endpoint.send_raw_transaction(retained).unwrap(),
        hash
    );
    assert_eq!(
        ledger.state.lock().unwrap().submissions.last().unwrap(),
        &raw
    );
    assert!(matches!(
        recovered.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    let competing = ledger.send(26, 7);
    let competing = NativeSendAdmission::verify(
        &competing,
        &ledger.raw(&competing),
        &registry,
        &ledger.reads(),
    )
    .unwrap();
    assert!(matches!(
        competing.admit(&mut store, tenant()),
        Err(BindingError::Conflict)
    ));
    assert!(NativeSendAdmission::restore(
        &store,
        TenantId::new("other").unwrap(),
        reference,
        &registry,
        &ledger.reads()
    )
    .unwrap()
    .is_none());
    let queries = &ledger.state.lock().unwrap().queries;
    for name in ["unified", "main_owner", "account", "source_owner"] {
        assert!(queries[name] >= 2);
    }
}

#[test]
fn native_send_envelope_refuses_changed_transaction_binding_and_foreign_ledger_owner() {
    let ledger = Ledger::new();
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let registry = ModuleRegistry::new(&[]).unwrap();
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
    let other = derive_from_seed(&[43; 64], 0).unwrap();
    let other_raw = prepared
        .transaction
        .call
        .sign(other.evm_secret().unwrap())
        .unwrap();
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
    ledger.state.lock().unwrap().source_owner = [99; 32];
    assert!(NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()).is_err());
}

#[test]
fn native_send_envelope_restart_rechecks_retained_bytes_and_current_binding() {
    let ledger = Ledger::new();
    let registry = ModuleRegistry::new(&[]).unwrap();
    let prepared = ledger.send(25, 7);
    let raw = ledger.raw(&prepared);
    let admission =
        NativeSendAdmission::verify(&prepared, &raw, &registry, &ledger.reads()).unwrap();
    let directory = Directory::new();
    let mut store = directory.open();
    admission.admit(&mut store, tenant()).unwrap();
    let reference = admission.preparation_id();
    let mut object = b"native-send-evm-v1:".to_vec();
    object.extend_from_slice(&reference);
    let key = TenantKey::new(tenant(), ObjectKind::PreparedActivity, object).unwrap();
    let original = store.get(&key).unwrap().bytes().to_vec();
    let mut record: Value = serde_json::from_slice(&original).unwrap();
    record[5][0] = json!(0x01);
    store
        .put_local(key.clone(), serde_json::to_vec(&record).unwrap())
        .unwrap();
    drop(store);
    let mut store = directory.open();
    assert!(
        NativeSendAdmission::restore(&store, tenant(), reference, &registry, &ledger.reads())
            .is_err()
    );
    store.put_local(key, original).unwrap();
    drop(store);
    let store = directory.open();
    ledger.state.lock().unwrap().owner = [91; 32];
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
    let registry = ModuleRegistry::new(&[]).unwrap();
    assert_eq!(
        verify_before_submit(&raw, &prepared, &ledger.account.evm_address(), &registry)
            .unwrap()
            .exact_bytes(),
        raw
    );
    let principal =
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).unwrap();
    let context = NativeRegistrationContext::new(principal, FEE_ASSET);
    let plan =
        capability::derive_native_registration(&prepared.transaction.disclosure, context).unwrap();
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
    let commitment = capability::native_registration_commitment(
        &prepared.transaction.disclosure,
        3,
        125,
        digest,
        context,
    )
    .unwrap();
    assert_ne!(
        commitment,
        capability::native_registration_commitment(
            &prepared.transaction.disclosure,
            3,
            125,
            digest,
            NativeRegistrationContext::new(principal, [38; 32])
        )
        .unwrap()
    );
    let directory = Directory::new();
    let mut store = directory.open();
    prepared.retain(&mut store, tenant(), digest).unwrap();
    drop(store);
    let store = directory.open();
    let recovered = SigningPreparation::restore(&store, tenant(), digest).unwrap();
    let principal =
        VerifiedLedgerPrincipal::resolve(&recovered.transaction, &ledger.reads()).unwrap();
    let recovered_context = NativeRegistrationContext::new(principal, FEE_ASSET);
    assert_eq!(
        plan,
        capability::derive_native_registration(
            &recovered.transaction.disclosure,
            recovered_context
        )
        .unwrap()
    );
    assert_eq!(
        commitment,
        capability::native_registration_commitment(
            &recovered.transaction.disclosure,
            3,
            125,
            digest,
            recovered_context
        )
        .unwrap()
    );
}

#[test]
fn registration_principal_refuses_missing_binding_cross_preparation_and_unbounded_supply() {
    let ledger = Ledger::new();
    let prepared = ledger.register(1000);
    let principal =
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).unwrap();
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
        let principal =
            VerifiedLedgerPrincipal::resolve(&changed.transaction, &ledger.reads()).unwrap();
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
    let mut unbound = ledger.state.lock().unwrap().unified.clone();
    unbound[64..96].fill(0);
    ledger.state.lock().unwrap().unified = unbound;
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
}

#[test]
fn registration_principal_refuses_owner_drift_and_unrelated_evm_join() {
    let ledger = Ledger::new();
    let prepared = ledger.register(1_000);
    let original = ledger.owner();
    ledger.state.lock().unwrap().owner = [93; 32];
    let principal =
        VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).unwrap();
    assert!(capability::derive_native_registration(
        &prepared.transaction.disclosure,
        NativeRegistrationContext::new(principal, FEE_ASSET)
    )
    .is_err());
    ledger.state.lock().unwrap().owner = [0; 32];
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
    ledger.state.lock().unwrap().owner = original;
    ledger.state.lock().unwrap().unified[31] ^= 1;
    assert!(VerifiedLedgerPrincipal::resolve(&prepared.transaction, &ledger.reads()).is_err());
}
