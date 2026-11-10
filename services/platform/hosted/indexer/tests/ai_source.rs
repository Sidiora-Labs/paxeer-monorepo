//! Exercises the source executable, consensus RPC transport and durable AI consumer.
use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{
    atomic::{AtomicBool, AtomicU64, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};
use layerx_indexer::{
    ai_market::{BlockCommitProof, ProjectionState, ViewError},
    ai_runtime::{encode_committed_read, RuntimeConfig},
    ai_source::SourceConfig,
    codec::{hex, unhex},
    transport::{Endpoint, Security},
    IndexError,
};
use layerx_programs_ai_market::{
    codec::{self, decode_envelope, derive_market, encode_envelope, Envelope},
    dispatch,
    errors::ApplicationError,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::read_state_chunk,
    registry::{derive_rewards_account, F01_SECTION_CAP},
    registry_ops::{self, Outcome},
    state::{self, Section, SharedState},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId, ProgramId,
        RequestId, RubricDigest, StateDigest,
    },
    MAX_EVENT_BYTES,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const CHAIN_ID: &str = "ai-source-consensus";
const CHUNK_RESPONSE_MAX: usize = 8244;
const HEIGHT: u64 = 1000;

#[derive(Debug)]
struct Failure(String);
impl std::fmt::Display for Failure {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}
impl std::error::Error for Failure {}
macro_rules! failures {
    ($($error:ty),+ $(,)?) => {$(impl From<$error> for Failure {
        fn from(error: $error) -> Self { Self(format!("{error:?}")) }
    })+};
}
failures!(
    ApplicationError,
    ViewError,
    ProofError,
    IndexError,
    std::io::Error,
    serde_json::Error
);
type Checked<T = ()> = Result<T, Failure>;
fn missing() -> Failure {
    Failure("required test material absent".to_owned())
}

fn chain() -> Checked<ChainDomain> {
    Ok(ChainDomain::new(CHAIN)?)
}

fn program() -> Checked<ProgramId> {
    Ok(ProgramId::new(PROGRAM)?)
}

fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
    Ok(PrincipalId::new(bytes)?)
}

fn digest(byte: u8) -> Checked<Digest32> {
    Ok(Digest32::new([byte; 32])?)
}

fn market() -> Checked<MarketId> {
    Ok(derive_market(chain()?, program()?)?)
}

fn policy() -> Checked<TaskPolicyV1> {
    Ok(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1)?,
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3)?,
            rubric: RubricDigest::new([4; 32])?,
            task_schema: digest(5)?,
            result_schema: digest(6)?,
            service_terms: digest(7)?,
        },
        100,
        1,
    )?)
}

fn encode_shared(shared: &SharedState<'_>) -> Checked<Vec<u8>> {
    let mut out = vec![0; shared.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(shared, &mut out, &mut scratch)?;
    out.truncate(n);
    Ok(out)
}

fn created_state() -> Checked<Vec<u8>> {
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    policy()?.encode(&mut encoded_policy)?;
    let rewards = derive_rewards_account(program()?, AssetId::new(ASSET)?)?;
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&ASSET);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&REFUND);
    payload.push(0);
    payload.extend_from_slice(&encoded_policy);
    payload.extend_from_slice(&[16; 32]);
    let envelope = Envelope {
        operation: dispatch::CREATE,
        chain: chain()?,
        program: program()?,
        market: market()?,
        actor: principal(OWNER)?,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1_000_000,
        request: RequestId::new([1; 32])?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; 16_384];
    let n = encode_envelope(&envelope, &mut encoded)?;
    let validated = decode_envelope(encoded.get(..n).ok_or_else(missing)?)?;
    let ctx = registry_ops::CallContext {
        chain: chain()?,
        program: program()?,
        principal: principal(OWNER)?,
        height: 1000,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    match registry_ops::apply(&ctx, None, &validated, &mut section, &mut event)? {
        Outcome::Applied { state, .. } => encode_shared(&state),
        Outcome::AlreadyApplied(_) => Err(missing()),
    }
}

fn chunk_payload(
    revision: u64,
    pinned: Option<StateDigest>,
    offset: u32,
    requested: u16,
) -> Vec<u8> {
    let mut payload = revision.to_be_bytes().to_vec();
    payload.extend_from_slice(&pinned.map_or([0; 32], StateDigest::bytes));
    payload.extend_from_slice(&offset.to_be_bytes());
    payload.extend_from_slice(&requested.to_be_bytes());
    payload
}

fn read_all(state_bytes: &[u8]) -> Checked<Vec<Vec<u8>>> {
    let mut out = vec![0; CHUNK_RESPONSE_MAX];
    let n = read_state_chunk(state_bytes, &chunk_payload(0, None, 0, 8192), &mut out)?;
    let first = codec::decode_chunk_response(&out[..n])?;
    let (revision, pinned, total) = (first.revision, first.digest, first.total_bytes);
    let mut bodies = vec![out[..n].to_vec()];
    for offset in (8192..total).step_by(8192) {
        let payload = chunk_payload(revision, Some(pinned), offset, 8192);
        let n = read_state_chunk(state_bytes, &payload, &mut out)?;
        bodies.push(out[..n].to_vec());
    }
    Ok(bodies)
}

fn validators() -> Vec<Validator> {
    (1..=4)
        .map(|value| Validator {
            pub_key: SigningKey::from_bytes(&[value; 32])
                .verifying_key()
                .to_bytes(),
            voting_power: 10,
        })
        .collect()
}

fn signed(header: &Header, signers: &[(u8, bool)]) -> Checked<Commit> {
    let mut commit = Commit {
        height: u64::try_from(header.height).map_err(|_| missing())?,
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xA5; 32],
            },
        },
        signatures: Vec::new(),
    };
    for value in 1..=4 {
        let Some((_, tamper)) = signers.iter().find(|(signer, _)| *signer == value) else {
            commit.signatures.push(CommitSig::absent());
            continue;
        };
        let key = SigningKey::from_bytes(&[value; 32]);
        let timestamp = Timestamp {
            seconds: 1_000,
            nanos: 123,
        };
        let mut vote = Vec::new();
        encode_varint_field(1, 2, &mut vote);
        vote.push(0x11);
        vote.extend_from_slice(&commit.height.to_le_bytes());
        let mut parts = Vec::new();
        encode_varint_field(
            1,
            u64::from(commit.block_id.part_set_header.total),
            &mut parts,
        );
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block);
        encode_length_delimited(2, &parts, &mut block);
        encode_length_delimited(4, &block, &mut vote);
        let mut time = Vec::new();
        encode_varint_field(
            1,
            u64::try_from(timestamp.seconds).map_err(|_| missing())?,
            &mut time,
        );
        encode_varint_field(2, u64::from(timestamp.nanos), &mut time);
        encode_length_delimited(5, &time, &mut vote);
        encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
        let mut sign_bytes = Vec::new();
        encode_varint(vote.len() as u64, &mut sign_bytes);
        sign_bytes.extend_from_slice(&vote);
        let mut signature = key.sign(&sign_bytes).to_bytes();
        if *tamper {
            signature[40] ^= 1;
        }
        commit.signatures.push(CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
            timestamp,
            signature: Some(signature),
        });
    }
    Ok(commit)
}

fn header(height: u64, root: Digest32) -> Checked<Header> {
    Ok(Header {
        chain_id: CHAIN_ID.to_owned(),
        height: i64::try_from(height).map_err(|_| missing())?,
        app_hash: root.bytes().to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1_000,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: vec![2; 32],
        validators_hash: validators_hash(&validators()),
        next_validators_hash: validators_hash(&validators()).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: vec![6; 20],
    })
}

fn native_membership(state: &[u8], program: [u8; 32], cell: &[u8]) -> Checked<(Vec<u8>, [u8; 32])> {
    fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
        let mut operation = Vec::new();
        for (field, value) in [(1, 1), (2, 0), (3, 1), (4, 1)] {
            encode_varint_field(field, value, &mut operation);
        }
        encode_length_delimited(5, prefix, &mut operation);
        let mut proof = Vec::new();
        encode_length_delimited(1, key, &mut proof);
        encode_length_delimited(2, value, &mut proof);
        encode_length_delimited(3, &operation, &mut proof);
        let mut preimage = prefix.to_vec();
        encode_varint(key.len() as u64, &mut preimage);
        preimage.extend(key);
        encode_varint(32, &mut preimage);
        preimage.extend(Sha256::digest(value));
        (proof, Sha256::digest(preimage).into())
    }
    let mut manifest = vec![0, 1];
    manifest.extend(1u32.to_be_bytes());
    manifest.extend(
        u16::try_from(cell.len())
            .map_err(|_| missing())?
            .to_be_bytes(),
    );
    manifest.extend(cell);
    manifest.extend(Sha256::digest(state));
    manifest.extend(
        u32::try_from(state.len())
            .map_err(|_| missing())?
            .to_be_bytes(),
    );
    let mut head = vec![0, 1];
    head.extend(1u32.to_be_bytes());
    head.extend(Sha256::digest(&manifest));
    let mut key = b"\x01\x00\x09progstor".to_vec();
    key.extend(program);
    key.push(1);
    let (proof, store_root) = leaf(&key, &head, &[0, 2, 2]);
    let (outer, app_hash) = leaf(b"lxphost", &store_root, &[0]);
    let mut multistore = Vec::new();
    encode_length_delimited(1, &outer, &mut multistore);
    Ok((
        serde_json::to_vec(
            &json!({"membership": {"proof": proof, "multistore": multistore}, "manifest": manifest}),
        )?,
        app_hash,
    ))
}

fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut output = String::new();
    for part in bytes.chunks(3) {
        let first = part[0];
        let second = part.get(1).copied().unwrap_or(0);
        let third = part.get(2).copied().unwrap_or(0);
        output.push(char::from(ALPHABET[usize::from(first >> 2)]));
        output.push(char::from(
            ALPHABET[usize::from((first & 3) << 4 | second >> 4)],
        ));
        output.push(if part.len() > 1 {
            char::from(ALPHABET[usize::from((second & 15) << 2 | third >> 6)])
        } else {
            '='
        });
        output.push(if part.len() > 2 {
            char::from(ALPHABET[usize::from(third & 63)])
        } else {
            '='
        });
    }
    output
}

fn node_time(time: Timestamp) -> String {
    assert_eq!(time.seconds, 1000);
    format!("1970-01-01T00:16:40.{:09}Z", time.nanos)
}
fn node_block(block: &BlockId) -> Value {
    json!({"hash": hex(&block.hash), "parts": {
        "total": block.part_set_header.total, "hash": hex(&block.part_set_header.hash)
    }})
}
fn node_commit(h: &Header, c: &Commit) -> Value {
    let signatures = c.signatures.iter().map(|sig| json!({
        "block_id_flag": match sig.block_id_flag { BlockIdFlag::Absent => 1, BlockIdFlag::Commit => 2, BlockIdFlag::Nil => 3 },
        "validator_address": hex(&sig.validator_address), "timestamp": node_time(sig.timestamp),
        "signature": sig.signature.map(|bytes| base64(&bytes))
    })).collect::<Vec<_>>();
    json!({"canonical": true, "signed_header": {"header": {
        "version": {"block": h.version.block.to_string(), "app": h.version.app.to_string()},
        "chain_id": h.chain_id, "height": h.height.to_string(), "time": node_time(h.time),
        "last_block_id": node_block(&h.last_block_id), "last_commit_hash": hex(&h.last_commit_hash),
        "data_hash": hex(&h.data_hash), "validators_hash": hex(&h.validators_hash),
        "next_validators_hash": hex(&h.next_validators_hash), "consensus_hash": hex(&h.consensus_hash),
        "app_hash": hex(&h.app_hash), "last_results_hash": hex(&h.last_results_hash),
        "evidence_hash": hex(&h.evidence_hash), "proposer_address": hex(&h.proposer_address)
    }, "commit": {"height": c.height.to_string(), "round": c.round,
        "block_id": node_block(&c.block_id), "signatures": signatures
    }}})
}

struct Material {
    header: Header,
    commit: Commit,
    proof: Vec<u8>,
    state: Vec<u8>,
}
impl Material {
    fn new() -> Checked<Self> {
        let state = created_state()?;
        let (proof, root) =
            native_membership(&state, PROGRAM, layerx_programs_ai_market::SHARED_STATE_KEY)?;
        let header = header(HEIGHT, Digest32::new(root)?)?;
        let commit = signed(&header, &[(1, false), (2, false), (3, false), (4, false)])?;
        Ok(Self {
            header,
            commit,
            proof,
            state,
        })
    }
    fn body(&self) -> Checked<Vec<u8>> {
        Ok(encode_committed_read(
            chain()?,
            program()?,
            HEIGHT - 1,
            &BlockCommitProof {
                header: &self.header,
                commit: &self.commit,
                store_proof: &self.proof,
            },
            &read_all(&self.state)?,
        )?)
    }
    fn answer(&self, request: &Value) -> Checked<Value> {
        let params = &request["params"];
        match request["method"].as_str() {
            Some("status") => {
                assert_eq!(params, &json!({}));
                Ok(json!({"sync_info": {"latest_block_height": HEIGHT.to_string()}}))
            }
            Some("commit") => {
                assert_eq!(params, &json!({"height": HEIGHT.to_string()}));
                Ok(node_commit(&self.header, &self.commit))
            }
            Some("abci_query") => self.artifact(params),
            _ => Err(missing()),
        }
    }
    fn artifact(&self, params: &Value) -> Checked<Value> {
        assert_eq!(params["path"], "/store/lxphost/key");
        assert_eq!(params["height"], (HEIGHT - 1).to_string());
        let proof: Value = serde_json::from_slice(&self.proof)?;
        let manifest: Vec<u8> = serde_json::from_value(proof["manifest"].clone())?;
        let existence: Vec<u8> = serde_json::from_value(proof["membership"]["proof"].clone())?;
        let outer: Vec<u8> = serde_json::from_value(proof["membership"]["multistore"].clone())?;
        let mut namespace = vec![0, 1];
        namespace.extend(1u32.to_be_bytes());
        namespace.extend(Sha256::digest(&manifest));
        let mut key = b"\x01\x00\x09progstor".to_vec();
        key.extend(PROGRAM);
        key.push(1);
        let requested = unhex(params["data"].as_str().ok_or_else(missing)?)?;
        let (value, ops) = if requested == key {
            assert_eq!(params["prove"], true);
            let mut inner = Vec::new();
            encode_length_delimited(1, &existence, &mut inner);
            (
                namespace,
                json!({"ops": [
                    {"type": "ics23:iavl", "key": base64(&key), "data": base64(&inner)},
                    {"type": "ics23:simple", "key": base64(b"lxphost"), "data": base64(&outer)}
                ]}),
            )
        } else {
            assert_eq!(params["prove"], false);
            let blob_key =
                |bytes: &[u8]| [&[5, 0, 9][..], Sha256::digest(bytes).as_slice()].concat();
            if requested == blob_key(&manifest) {
                (manifest, Value::Null)
            } else {
                assert_eq!(requested, blob_key(&self.state));
                (self.state.clone(), Value::Null)
            }
        };
        Ok(
            json!({"response": {"code": 0, "height": (HEIGHT - 1).to_string(),
                "key": base64(&requested), "value": base64(&value), "proofOps": ops
            }}),
        )
    }
}

fn read_request(stream: &mut TcpStream) -> Checked<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
    let mut head = Vec::new();
    while !head.ends_with(b"\r\n\r\n") {
        assert!(head.len() < 4096);
        let mut byte = [0];
        stream.read_exact(&mut byte)?;
        head.push(byte[0]);
    }
    let text = std::str::from_utf8(&head).map_err(|_| missing())?;
    assert!(text.starts_with("POST / HTTP/1.1\r\n"));
    let length = text
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .and_then(|n| n.parse::<usize>().ok())
        .ok_or_else(missing)?;
    assert!(length < 4096);
    let mut body = vec![0; length];
    stream.read_exact(&mut body)?;
    Ok(serde_json::from_slice(&body)?)
}

struct Node {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<Checked>>,
    calls: Arc<Mutex<Vec<Value>>>,
}
impl Node {
    fn start(material: Option<Material>) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        listener.set_nonblocking(true)?;
        let stop = Arc::new(AtomicBool::new(false));
        let done = Arc::clone(&stop);
        let calls = Arc::new(Mutex::new(Vec::new()));
        let seen = Arc::clone(&calls);
        let join = thread::spawn(move || {
            while !done.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => return Err(error.into()),
                };
                let request = read_request(&mut stream)?;
                seen.lock().map_err(|_| missing())?.push(request.clone());
                let response = if let Some(material) = &material {
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": material.answer(&request)?})
                } else {
                    json!({"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32603, "message": "node unavailable"}})
                };
                let body = serde_json::to_vec(&response)?;
                stream.set_write_timeout(Some(Duration::from_secs(3)))?;
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                )?;
                stream.write_all(&body)?;
            }
            Ok(())
        });
        Ok(Self {
            address,
            stop,
            join: Some(join),
            calls,
        })
    }
    fn finish(&mut self) -> Checked {
        self.stop.store(true, Ordering::Release);
        self.join
            .take()
            .ok_or_else(missing)?
            .join()
            .map_err(|_| missing())?
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            let _ = join.join();
        }
    }
}

struct Process(Child);
impl Process {
    fn start(node: &Node, address: SocketAddr) -> Checked<Self> {
        let child = Command::new(env!("CARGO_BIN_EXE_layerx-ai-source"))
            .env("LAYERX_AI_SOURCE_LISTEN", address.to_string())
            .env(
                "LAYERX_AI_SOURCE_NODE_URL",
                format!("http://{}", node.address),
            )
            .env_remove("LAYERX_AI_SOURCE_NODE_CA_DER")
            .env("LAYERX_AI_SOURCE_CHAIN", hex(&CHAIN))
            .env("LAYERX_AI_SOURCE_PROGRAM", hex(&PROGRAM))
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .spawn()?;
        let mut process = Self(child);
        let endpoint = endpoint(address)?;
        let deadline = Instant::now() + Duration::from_secs(5);
        loop {
            if endpoint
                .get("/startup")
                .is_ok_and(|(status, _)| status == 404)
            {
                break;
            }
            if process.0.try_wait()?.is_some() || Instant::now() >= deadline {
                return Err(missing());
            }
            thread::sleep(Duration::from_millis(10));
        }
        Ok(process)
    }
}
impl Drop for Process {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}
fn endpoint(address: SocketAddr) -> Checked<Endpoint> {
    Ok(Endpoint::parse(
        &format!("http://{address}"),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(10),
    )?)
}
fn free_address() -> Checked<SocketAddr> {
    Ok(TcpListener::bind("127.0.0.1:0")?.local_addr()?)
}

static DIRECTORY: AtomicU64 = AtomicU64::new(0);
struct Scratch(PathBuf);
impl Scratch {
    fn new() -> Checked<Self> {
        let path = std::env::temp_dir().join(format!(
            "ai-source-{}-{}",
            std::process::id(),
            DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path)?;
        Ok(Self(path))
    }
    fn settings(&self, address: SocketAddr) -> Checked<PathBuf> {
        let path = self.0.join("runtime.json");
        let validators = validators()
            .iter()
            .map(|v| json!({"pub_key": v.pub_key.to_vec(), "voting_power": v.voting_power}))
            .collect::<Vec<_>>();
        fs::write(
            &path,
            serde_json::to_vec(&json!({
                "chain": CHAIN.to_vec(), "program": PROGRAM.to_vec(), "consensus_chain_id": CHAIN_ID,
                "validators": validators, "source": format!("http://{address}"), "ca_der": null
            }))?,
        )?;
        Ok(path)
    }
    fn database(&self) -> PathBuf {
        self.0.join("projection.sqlite")
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn open_runtime(
    path: &Path,
    scratch: &Scratch,
) -> Checked<(layerx_indexer::ai_runtime::AiRuntime, Endpoint)> {
    Ok(RuntimeConfig::read(path)?.open(&scratch.database())?)
}

#[test]
fn committed_read_reaches_the_projection() -> Checked {
    let material = Material::new()?;
    let expected = material.body()?;
    let expected_digest = codec::state_digest(&material.state)?;
    let mut node = Node::start(Some(material))?;
    let address = free_address()?;
    let source = Process::start(&node, address)?;
    let scratch = Scratch::new()?;
    let settings = scratch.settings(address)?;
    let (mut runtime, endpoint) = open_runtime(&settings, &scratch)?;
    let target = format!("/v1/ai/programs/{}/committed-read?after=0", hex(&PROGRAM));
    assert_eq!(endpoint.get(&target)?, (200, expected));
    assert!(runtime.projections.stream(&market()?)?.is_none());
    let snapshot = runtime.poll(&endpoint, 5000)?.ok_or_else(missing)?;
    assert_eq!(snapshot.projection, ProjectionState::FinalizedPublishable);
    assert_eq!(snapshot.observed_sequence, HEIGHT - 1);
    assert_eq!(snapshot.execution_height, HEIGHT);
    assert_eq!(snapshot.state_digest, expected_digest);
    assert_eq!(snapshot.market, market()?);
    assert_eq!(
        runtime.projections.snapshot(&snapshot.snapshot_id)?,
        Some(snapshot.clone())
    );
    assert_eq!(
        runtime
            .projections
            .stream(&market()?)?
            .ok_or_else(missing)?
            .finalized_sequence,
        Some(HEIGHT - 1)
    );
    assert!(runtime.poll(&endpoint, 5001)?.is_none());
    let caught_up = format!(
        "/v1/ai/programs/{}/committed-read?after={}",
        hex(&PROGRAM),
        HEIGHT - 1
    );
    assert_eq!(endpoint.get(&caught_up)?, (204, Vec::new()));
    drop(runtime);
    drop(source);
    let _restarted_source = Process::start(&node, address)?;
    let (mut reopened, endpoint) = open_runtime(&settings, &scratch)?;
    assert_eq!(
        reopened.projections.snapshot(&snapshot.snapshot_id)?,
        Some(snapshot)
    );
    assert!(reopened.poll(&endpoint, 5002)?.is_none());
    let calls = node.calls.lock().map_err(|_| missing())?;
    assert_eq!(calls.len(), 25);
    for group in calls.chunks_exact(5) {
        assert_eq!(group[0]["method"], "status");
        assert_eq!(group[1]["method"], "commit");
        assert!(group[2..].iter().all(|call| call["method"] == "abci_query"));
    }
    drop(calls);
    node.finish()
}

#[test]
fn source_refuses_what_it_cannot_serve() -> Checked {
    let mut node = Node::start(None)?;
    let address = free_address()?;
    let _source = Process::start(&node, address)?;
    let endpoint = endpoint(address)?;
    let route = format!("/v1/ai/programs/{}/committed-read", hex(&PROGRAM));
    for target in [
        "/v1/ai/programs/another/committed-read?after=0",
        "/v1/ai/unknown?after=0",
    ] {
        assert_eq!(endpoint.get(target)?, (404, Vec::new()));
    }
    for query in [
        "",
        "after=",
        "after=-1",
        "after=+1",
        "after=18446744073709551616",
        "after=0&after=1",
        "after=1&other=2",
        "cursor=1",
        "after=%31",
    ] {
        assert_eq!(
            endpoint.get(&format!("{route}?{query}"))?,
            (400, Vec::new())
        );
    }
    assert!(node.calls.lock().map_err(|_| missing())?.is_empty());
    for cursor in [0, u64::MAX] {
        assert_eq!(
            endpoint.get(&format!("{route}?after={cursor}"))?,
            (502, Vec::new())
        );
    }
    assert_eq!(node.calls.lock().map_err(|_| missing())?.len(), 2);
    node.finish()
}

#[test]
fn source_settings_enforce_transport_security() -> Checked {
    let mut settings = BTreeMap::from([
        ("LAYERX_AI_SOURCE_LISTEN", "127.0.0.1:8096".to_owned()),
        (
            "LAYERX_AI_SOURCE_NODE_URL",
            "http://127.0.0.1:26657".to_owned(),
        ),
        ("LAYERX_AI_SOURCE_CHAIN", hex(&CHAIN)),
        ("LAYERX_AI_SOURCE_PROGRAM", hex(&PROGRAM)),
    ]);
    let lookup = |settings: &BTreeMap<&str, String>| {
        SourceConfig::from_lookup(&|key| settings.get(key).cloned())
    };
    let config = lookup(&settings)?;
    assert_eq!(config.chain, chain()?);
    assert_eq!(config.program, program()?);
    assert_eq!(
        config.node.security(),
        &Security::Plaintext {
            allow_remote: false
        }
    );
    settings.insert(
        "LAYERX_AI_SOURCE_NODE_URL",
        "http://node.internal:26657".to_owned(),
    );
    assert!(lookup(&settings).is_err());
    settings.insert(
        "LAYERX_AI_SOURCE_NODE_URL",
        "https://node.internal:26657".to_owned(),
    );
    assert!(lookup(&settings).is_err());
    settings.insert(
        "LAYERX_AI_SOURCE_NODE_CA_DER",
        "/missing/ai-source-ca.der".to_owned(),
    );
    assert!(lookup(&settings).is_err());
    Ok(())
}
