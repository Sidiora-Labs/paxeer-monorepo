#[path = "../funded_inputs.rs"]
mod funded_inputs;
#[path = "../funding.rs"]
mod funding;
#[path = "../required.rs"]
mod required;
#[path = "../sequence.rs"]
mod sequence;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::{
    identity::{self, IdentityResolver},
    read::{
        checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
        CommitValidator, CommitVote,
    },
    store,
};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::{account_derivation::derive_from_seed, local::LocalSigner, signer::Signer};
use layerx_sdk::rpc::{RpcClient, RpcError};
use layerx_types::{ids::Did, verify::VerificationLevel};
use serde_json::{json, Value};
use std::{
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, Mutex,
    },
    thread::JoinHandle,
    time::Duration,
};

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

struct Fixture {
    url: String,
    signer: LocalSigner,
    did: Did,
    height: u64,
    header: CheckpointHeader,
    validators: Vec<CommitValidator>,
    nonce: Arc<AtomicU64>,
    corrupt_commit: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    root: PathBuf,
}

impl Fixture {
    fn start(nonce: u64) -> Self {
        let account = derive_from_seed(&[0x29; 32], 0).expect("real dual-key account");
        let signer = LocalSigner::from_derived_account(&account);
        let did =
            Did::new(format!("did:layerx:{}", hex(&signer.public_key())).as_bytes()).expect("DID");
        let sender = signer.evm_address().expect("derived EVM sender");
        let keys: Vec<_> = (1..=3).map(|n| SigningKey::from_bytes(&[n; 32])).collect();
        let validators: Vec<_> = keys
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
        let height = 8;
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "125".to_owned(),
            height,
            time: CommitTimestamp {
                seconds: 1_700_000_000,
                nanos: 0,
            },
            last_block_id: Some(BlockId {
                hash: [1; 32],
                part_set_total: 1,
                part_set_hash: [2; 32],
            }),
            last_commit_hash: Some([3; 32]),
            data_hash: Some([4; 32]),
            validators_hash: validator_set_hash(&validators),
            next_validators_hash: validator_set_hash(&validators),
            consensus_hash: [5; 32],
            app_hash: [6; 32],
            last_results_hash: Some([7; 32]),
            evidence_hash: Some([8; 32]),
            proposer_address: validators[0].address,
        };
        let mut commit = BlockCommit {
            height,
            round: 1,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [9; 32],
            },
            votes: Vec::new(),
        };
        for (key, validator) in keys.iter().zip(&validators) {
            commit.votes.push(CommitVote {
                validator_address: validator.address,
                voting_power: validator.voting_power,
                timestamp: header.time,
                signature: key
                    .sign(&commit.vote_sign_bytes(&header.chain_id, header.time))
                    .to_bytes(),
            });
        }
        let authenticated = checkpoint(height, "125", &header, &commit, &validators)
            .expect("production checkpoint producer");
        let id = |id: BlockId| json!({"hash":hex(&id.hash), "parts":{"total":id.part_set_total,"hash":hex(&id.part_set_hash)}});
        let signed = json!({"header":{
            "version":{"block":"11","app":"1"},"chain_id":"125","height":height.to_string(),
            "time":"2023-11-14T22:13:20Z","last_block_id":id(header.last_block_id.expect("parent")),
            "last_commit_hash":hex(&[3;32]),"data_hash":hex(&[4;32]),
            "validators_hash":hex(&header.validators_hash),"next_validators_hash":hex(&header.next_validators_hash),
            "consensus_hash":hex(&header.consensus_hash),"app_hash":hex(&header.app_hash),
            "last_results_hash":hex(&[7;32]),"evidence_hash":hex(&[8;32]),"proposer_address":hex(&header.proposer_address)
        },"commit":{"height":height.to_string(),"round":1,"block_id":id(commit.block_id),
            "signatures":commit.votes.iter().map(|vote| json!({"block_id_flag":2,"validator_address":hex(&vote.validator_address),
                "timestamp":"2023-11-14T22:13:20Z","signature":STANDARD.encode(vote.signature)})).collect::<Vec<_>>()}});
        let validator_wire = json!({"height":height.to_string(),"total":validators.len().to_string(),
            "validators":validators.iter().map(|v| json!({"address":hex(&v.address),"pubKey":{"type":"tendermint/PubKeyEd25519","value":STANDARD.encode(v.public_key)},"votingPower":v.voting_power.to_string()})).collect::<Vec<_>>()});
        let block = json!({"number":format!("{height:#x}"),"hash":format!("0x{}",hex(&authenticated.block_hash)),
            "parentHash":format!("0x{}",hex(&[1;32])),"timestamp":format!("{:#x}",header.time.seconds),
            "transactionsRoot":format!("0x{}",hex(&[4;32])),"receiptsRoot":format!("0x{}",hex(&[7;32])),
            "stateRoot":format!("0x{}",hex(&header.app_hash)),"transactions":[]});
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback fixture");
        listener.set_nonblocking(true).expect("nonblocking accept");
        let url = format!("http://{}", listener.local_addr().expect("fixture address"));
        let nonce = Arc::new(AtomicU64::new(nonce));
        let corrupt_commit = Arc::new(AtomicBool::new(false));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let server_nonce = Arc::clone(&nonce);
        let corrupt = Arc::clone(&corrupt_commit);
        let seen = Arc::clone(&requests);
        let shutdown = Arc::clone(&stop);
        let public_key = signer.public_key();
        let worker = std::thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("fixture accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("request deadline");
                let request = read_request(&mut stream);
                let method = request["method"].as_str().expect("method");
                let result = match method {
                    "eth_call" => {
                        assert_eq!(request["params"][1], "pending");
                        assert_eq!(
                            request["params"][0]["to"],
                            "0x0000000000000000000000000000000000001004"
                        );
                        let mut data = layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX.to_vec();
                        data.extend(public_key);
                        assert_eq!(request["params"][0]["data"], format!("0x{}", hex(&data)));
                        json!(format!("0x{}{}", "00".repeat(12), hex(&sender)))
                    }
                    "eth_getTransactionCount" => {
                        assert_eq!(
                            request["params"],
                            json!([format!("0x{}", hex(&sender)), "pending"])
                        );
                        json!(format!("{:#x}", server_nonce.load(Ordering::Acquire)))
                    }
                    "eth_chainId" => {
                        assert_eq!(request["params"], json!([]));
                        json!("0x7d")
                    }
                    "eth_blockNumber" => {
                        assert_eq!(request["params"], json!([]));
                        json!(format!("{height:#x}"))
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"][0], json!(format!("{height:#x}")));
                        assert!(matches!(
                            request["params"][1].as_bool(),
                            Some(true) | Some(false)
                        ));
                        assert_eq!(
                            request["params"].as_array().expect("block selector").len(),
                            2
                        );
                        block.clone()
                    }
                    "px_getCommit" => {
                        assert_eq!(request["params"], json!([height.to_string()]));
                        let mut answer = signed.clone();
                        if corrupt.load(Ordering::Acquire) {
                            answer["header"]["app_hash"] = json!(hex(&[0x99; 32]));
                        }
                        json!({"height":height.to_string(),"signedHeader":answer})
                    }
                    "px_getValidators" => {
                        assert_eq!(request["params"], json!([height.to_string()]));
                        validator_wire.clone()
                    }
                    other => panic!("unexpected real client RPC: {other}"),
                };
                seen.lock().expect("requests").push(request.clone());
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()).expect("fixture response");
            }
        });
        let mut entropy = [0; 16];
        getrandom::fill(&mut entropy).expect("temporary path entropy");
        let root = std::env::temp_dir().join(format!("pay2-repair-{}", hex(&entropy)));
        Self {
            url,
            signer,
            did,
            height,
            header,
            validators,
            nonce,
            corrupt_commit,
            requests,
            stop,
            worker: Some(worker),
            root,
        }
    }

    fn rpc(&self) -> RpcClient {
        RpcClient::connect(&self.url, None).expect("SDK RPC transport")
    }

    fn authority(&self) -> sequence::Pay2CommittedAuthority {
        sequence::Pay2CommittedAuthority {
            node: SequencerEndpoint::new(&self.url, &[]).expect("owner RPC transport"),
            chain_id: "125".to_owned(),
            validators: self.validators.clone(),
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("loopback server assertions");
        }
        if self.root.exists() {
            std::fs::remove_dir_all(&self.root).expect("remove test store");
        }
    }
}

fn read_request(stream: &mut TcpStream) -> Value {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte).expect("HTTP request header");
        header.push(byte[0]);
        assert!(header.len() <= 16_384);
    }
    let length = std::str::from_utf8(&header)
        .expect("HTTP header text")
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse::<usize>().expect("HTTP length"))
        })
        .expect("request content length");
    assert!(length <= 1_048_576);
    let mut bytes = vec![0; length];
    stream.read_exact(&mut bytes).expect("HTTP request body");
    serde_json::from_slice(&bytes).expect("JSON-RPC request")
}

#[test]
fn pay2_nonce_snapshot_uses_bound_sender_and_committed_authority() {
    let fixture = Fixture::start(37);
    let rpc = fixture.rpc();
    let did = fixture.did.clone();
    let root = &fixture.root;
    assert!(
        !root.exists(),
        "qualification must start with a fresh real store"
    );
    let tenant = store::TenantId::new("pay2-nonce-snapshot".to_owned()).expect("tenant");
    let capability = [0x29; 32];
    let mut resolver = sequence::Pay2GatewayIdentity::new(&rpc, fixture.authority(), capability);
    let mut store = store::Store::open(root).expect("real identity store");
    let record = identity::register(&mut store, tenant.clone(), did.clone(), &mut resolver)
        .expect("authenticated identity consumer");
    let bytes = record.canonical_core_bytes();
    assert_eq!(bytes.len(), 132);
    let height = u64::from_be_bytes(bytes[32..40].try_into().expect("committed height"));
    let nonce = u64::from_be_bytes(bytes[40..48].try_into().expect("pending nonce"));
    let sender: [u8; 20] = bytes[48..68].try_into().expect("EVM sender");
    let public_key: [u8; 32] = bytes[100..132].try_into().expect("DID key");
    let authority = fixture.authority();
    assert_eq!(
        authority
            .node
            .bound_address(&public_key)
            .expect("addr precompile"),
        Some(sender)
    );
    assert_eq!(Some(sender), fixture.signer.evm_address());
    assert_eq!(record.head_sequence(), height);
    assert_eq!(height, fixture.height);
    assert_eq!(nonce, 37);
    assert_ne!(nonce, height);
    assert_eq!(record.verification_level(), VerificationLevel::COMMITTED);
    assert_eq!(
        record.authorities(),
        &[identity::ProtocolAuthority::CapabilityGrant(capability)]
    );
    let reads = layerx_agentd::read::CommittedReads::open(&authority.node, height)
        .expect("committed owner reads");
    let observed = reads.checkpoint();
    assert_eq!(bytes[..32], observed.app_hash);
    assert_eq!(bytes[68..100], observed.block_hash);
    assert_eq!(observed.app_hash, fixture.header.app_hash);
    assert_eq!(
        checkpoint(
            height,
            &authority.chain_id,
            &observed.header,
            &observed.commit,
            &authority.validators
        )
        .expect("pinned quorum"),
        *observed
    );
    assert_eq!(
        rpc.get_identity_sequence(&format!("0x{}", hex(&sender)))
            .expect("SDK nonce"),
        nonce
    );
    assert_eq!(
        sequence::pay2_signer_nonce(&rpc, &fixture.signer).expect("funded sender nonce"),
        nonce
    );
    assert_eq!(
        authority.node.pending_nonce(&sender).expect("node nonce"),
        nonce
    );
    assert!(matches!(
        rpc.get_identity_sequence(std::str::from_utf8(did.as_bytes()).expect("DID text")),
        Err(RpcError::InvalidRequest)
    ));
    let key = store::TenantKey::new(tenant, store::ObjectKind::Identity, did.as_bytes().to_vec())
        .expect("identity key");
    let persisted = store
        .get(&key)
        .expect("persisted identity")
        .bytes()
        .to_vec();
    drop(store);
    let reopened = store::Store::open(root).expect("reopened identity store");
    assert_eq!(
        reopened.get(&key).expect("recovered identity").bytes(),
        persisted
    );
    let requests = fixture.requests.lock().expect("requests");
    assert!(requests.iter().any(|r| r["method"] == "px_getCommit"));
    assert!(requests.iter().any(|r| r["method"] == "px_getValidators"));
}

#[test]
fn pay2_nonce_requires_a_real_evm_signer() {
    let rpc = RpcClient::connect("http://127.0.0.1:1", None).expect("local configuration");
    let signer = LocalSigner::new([0x29; 32]);
    assert!(matches!(
        sequence::pay2_signer_nonce(&rpc, &signer),
        Err(RpcError::Signing(
            layerx_crypto::signer::SignError::EvmKeyUnavailable
        ))
    ));
}

#[test]
fn pay2_nonce_does_not_supply_committed_authority() {
    let fixture = Fixture::start(u64::MAX);
    let rpc = fixture.rpc();
    assert_eq!(
        sequence::pay2_signer_nonce(&rpc, &fixture.signer).expect("u64 nonce"),
        u64::MAX
    );
    let mut resolver = sequence::Pay2GatewayIdentity::new(&rpc, fixture.authority(), [0x29; 32]);
    for nonce in [0, u64::MAX] {
        fixture.nonce.store(nonce, Ordering::Release);
        let identity = resolver
            .resolve(&fixture.did)
            .expect("real owner read")
            .expect("bound DID");
        assert_eq!(identity.head_sequence, fixture.height);
        assert_eq!(&identity.canonical_bytes[..32], fixture.header.app_hash);
        assert_eq!(&identity.canonical_bytes[40..48], nonce.to_be_bytes());
    }
    fixture.corrupt_commit.store(true, Ordering::Release);
    assert!(
        resolver.resolve(&fixture.did).is_err(),
        "a valid nonce cannot authenticate a tampered committed root"
    );
    fixture.corrupt_commit.store(false, Ordering::Release);
    let mut untrusted = fixture.authority();
    untrusted.validators[0].public_key = SigningKey::from_bytes(&[0x77; 32])
        .verifying_key()
        .to_bytes();
    untrusted.validators[0].address =
        CommitValidator::address_for_key(&untrusted.validators[0].public_key);
    let mut resolver = sequence::Pay2GatewayIdentity::new(&rpc, untrusted, [0x29; 32]);
    assert!(matches!(
        resolver.resolve(&fixture.did),
        Err(identity::IdentityError::Unverified)
    ));
}

#[test]
fn pay2_funded_payloads_use_real_producer_codecs() {
    use layerx_crypto::payments::Payment;
    use layerx_types::payload::ModuleId;
    let payer = derive_from_seed(&[0x29; 32], 0).expect("payer derivation");
    let recipient = derive_from_seed(&[0x30; 32], 0).expect("recipient derivation");
    let grant = funding::payer_grant(&funding::GrantRequest {
        payer_seed: payer.layerx_seed(),
        payer_did: &payer.did(),
        recipient_did: &recipient.did(),
        asset: [0x42; 32],
        per_draw_maximum: 5,
        allowance: 10,
        recurring_window: None,
        expiration: 1_800_000_000_000,
        purpose_hash: [0x43; 32],
        revocation_sequence: 37,
    })
    .expect("signed payer grant");
    let issue = Payment::IssueGrant(grant.clone())
        .encode(payer.did().as_bytes())
        .expect("real issue codec");
    assert_eq!(
        Payment::decode(ModuleId::Asset, 7, &issue, payer.did().as_bytes())
            .expect("grant signature verification"),
        Payment::IssueGrant(grant.clone())
    );
    let payment = funding::receive(recipient.layerx_seed(), &grant, 23, [0x44; 32], 2)
        .expect("signed receive");
    let draw = payment
        .encode(recipient.did().as_bytes())
        .expect("real receive codec");
    assert_eq!(
        Payment::decode(ModuleId::Asset, 6, &draw, recipient.did().as_bytes())
            .expect("both payment signatures"),
        payment
    );
    assert_eq!(issue.len(), 346);
    assert_eq!(draw.len(), 733);
    assert_eq!(&draw[387..], issue);
    let mut tampered = draw;
    tampered[200] ^= 1;
    assert!(Payment::decode(ModuleId::Asset, 6, &tampered, recipient.did().as_bytes()).is_err());
}
