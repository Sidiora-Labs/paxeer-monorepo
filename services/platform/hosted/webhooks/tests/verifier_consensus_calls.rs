use ed25519_dalek::{Signer, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
};
use layerx_platform_webhooks::{
    chain::{bytes, merkle, number, varint, ChainPolicy, Membership},
    encoding::{base64_encode, fixed_hex, hex_decode, hex_encode},
    trusted::{ReceiptVerifier, TrustedSources},
};
use native_tls::{Identity, TlsAcceptor};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::{Read, Write},
    net::TcpListener,
    path::PathBuf,
    process::Command,
    thread,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

const CHAIN_ID: &str = "webhook-repair-chain";
const ACTIVITY: &str = "a52f2455a87ccb3b84a90ae4e3c9153e7122b66cd69cb080ae9ecd9732619277";

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn digest(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn validators() -> Vec<Validator> {
    (1..=3)
        .map(|n| Validator {
            pub_key: SigningKey::from_bytes(&[n; 32]).verifying_key().to_bytes(),
            voting_power: 10,
        })
        .collect()
}

fn policy() -> ChainPolicy {
    ChainPolicy {
        chain_id: CHAIN_ID.to_owned(),
        validators: validators()
            .into_iter()
            .map(|validator| layerx_platform_webhooks::chain::ValidatorPin {
                public_key: validator.pub_key,
                voting_power: checked(u64::try_from(validator.voting_power)),
            })
            .collect(),
    }
}

fn header(root: [u8; 32], height: u64, data_hash: [u8; 32]) -> Header {
    Header {
        chain_id: CHAIN_ID.to_owned(),
        height: checked(i64::try_from(height)),
        app_hash: root.to_vec(),
        version: ConsensusVersion { block: 11, app: 1 },
        time: Timestamp {
            seconds: 1_000,
            nanos: 123,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: vec![1; 32],
        data_hash: data_hash.to_vec(),
        validators_hash: validators_hash(&validators()),
        next_validators_hash: validators_hash(&validators()).to_vec(),
        consensus_hash: vec![3; 32],
        last_results_hash: vec![4; 32],
        evidence_hash: vec![5; 32],
        proposer_address: vec![6; 20],
    }
}

fn signed(header: &Header) -> Commit {
    let mut commit = Commit {
        height: checked(u64::try_from(header.height)),
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xa5; 32],
            },
        },
        signatures: Vec::new(),
    };
    for n in 1..=3u8 {
        let key = SigningKey::from_bytes(&[n; 32]);
        let timestamp = Timestamp {
            seconds: 1_000,
            nanos: 123,
        };
        let mut vote = Vec::new();
        encode_varint_field(1, 2, &mut vote);
        vote.push(0x11);
        vote.extend(commit.height.to_le_bytes());
        let mut parts = Vec::new();
        encode_varint_field(1, 1, &mut parts);
        encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
        let mut block = Vec::new();
        encode_length_delimited(1, &commit.block_id.hash, &mut block);
        encode_length_delimited(2, &parts, &mut block);
        encode_length_delimited(4, &block, &mut vote);
        let mut time = Vec::new();
        encode_varint_field(1, 1_000, &mut time);
        encode_varint_field(2, 123, &mut time);
        encode_length_delimited(5, &time, &mut vote);
        encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
        let mut message = Vec::new();
        encode_varint(vote.len() as u64, &mut message);
        message.extend(vote);
        commit.signatures.push(CommitSig {
            block_id_flag: BlockIdFlag::Commit,
            validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
            timestamp,
            signature: Some(key.sign(&message).to_bytes()),
        });
    }
    commit
}

fn membership(store: &[u8], key: &[u8], value: &[u8]) -> (Membership, [u8; 32]) {
    fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
        let mut op = Vec::new();
        number(1, 1, &mut op);
        number(3, 1, &mut op);
        number(4, 1, &mut op);
        bytes(5, prefix, &mut op);
        let mut proof = Vec::new();
        bytes(1, key, &mut proof);
        bytes(2, value, &mut proof);
        bytes(3, &op, &mut proof);
        let mut preimage = prefix.to_vec();
        varint(key.len() as u64, &mut preimage);
        preimage.extend(key);
        varint(32, &mut preimage);
        preimage.extend(digest(value));
        (proof, digest(&preimage))
    }
    let (proof, root) = leaf(key, value, &[0, 2, 2]);
    let (outer, app) = leaf(store, &root, &[0]);
    let mut multistore = Vec::new();
    bytes(1, &outer, &mut multistore);
    (Membership { proof, multistore }, app)
}

struct Evidence {
    receipt: Vec<u8>,
    block_hash: [u8; 32],
    query: Value,
    commit: Value,
    block: Value,
}

fn evidence() -> Evidence {
    let fixture: Value = checked(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    let case = &fixture["cases"][0];
    let receipt = checked(hex_decode(
        case["signedReceipt"]
            .as_str()
            .unwrap_or_default()
            .trim_start_matches("0x"),
    ));
    let activity: [u8; 32] = checked(fixed_hex(ACTIVITY));
    let key = SigningKey::from_bytes(&[0x5e; 32]);
    let mut table = vec![0x6b; 32];
    table.extend(key.verifying_key().to_bytes());
    table.extend(1_000u64.to_be_bytes());
    table.extend(1_999u64.to_be_bytes());
    let (governed_keys, root) = membership(b"sequencer", &[3], &table);
    let mut batch = Vec::new();
    for n in [1_000u64, 1, 1] {
        batch.extend(n.to_be_bytes());
    }
    batch.extend([0; 32]);
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend([0; 32]);
    sequence.extend(1u64.to_be_bytes());
    sequence.extend(activity);
    batch.extend(digest(&sequence));
    batch.extend(digest(&activity));
    let mut signing = b"PAXEER/v1/sequence-batch\0".to_vec();
    signing.extend(CHAIN_ID.as_bytes());
    signing.extend(&batch);
    batch.extend([0x6b; 32]);
    batch.extend(key.sign(&digest(&signing)).to_bytes());
    let mut raw = b"PAXEER/v1/sequence-batch\0".to_vec();
    raw.extend(batch);
    let tx = checked(hex_decode(
        case["rawTransaction"]
            .as_str()
            .unwrap_or_default()
            .trim_start_matches("0x"),
    ));
    let transactions = vec![raw, tx];
    let data_hash = merkle(
        &transactions
            .iter()
            .map(|transaction| digest(transaction).to_vec())
            .collect::<Vec<_>>(),
    );
    let h = header(root, 1_000, data_hash);
    let c = signed(&h);
    let block_hash = h.hash();
    let commit = json!({
        "canonical": true,
        "signed_header": {
            "header": {
                "version": {"block": "11", "app": "1"},
                "chain_id": CHAIN_ID,
                "height": "1000",
                "time": "1970-01-01T00:16:40.000000123Z",
                "last_block_id": {"hash": "", "parts": {"total": "0", "hash": ""}},
                "last_commit_hash": hex_encode(&h.last_commit_hash),
                "data_hash": hex_encode(&h.data_hash),
                "validators_hash": hex_encode(&h.validators_hash),
                "next_validators_hash": hex_encode(&h.next_validators_hash),
                "consensus_hash": hex_encode(&h.consensus_hash),
                "app_hash": hex_encode(&h.app_hash),
                "last_results_hash": hex_encode(&h.last_results_hash),
                "evidence_hash": hex_encode(&h.evidence_hash),
                "proposer_address": hex_encode(&h.proposer_address)
            },
            "commit": {
                "height": "1000",
                "round": "0",
                "block_id": {"hash": hex_encode(&c.block_id.hash), "parts": {"total": "1", "hash": hex_encode(&c.block_id.part_set_header.hash)}},
                "signatures": c.signatures.iter().map(|signature| json!({
                    "block_id_flag": "2",
                    "validator_address": hex_encode(&signature.validator_address),
                    "timestamp": "1970-01-01T00:16:40.000000123Z",
                    "signature": base64_encode(signature.signature.as_ref().map_or(&[], |value| value.as_slice())),
                })).collect::<Vec<_>>()
            }
        }
    });
    let block = json!({
        "block": {
            "header": {"height": "1000"},
            "data": {"txs": transactions.iter().map(|tx| base64_encode(tx)).collect::<Vec<_>>()}
        }
    });
    let inner = {
        let mut value = Vec::new();
        bytes(1, &governed_keys.proof, &mut value);
        value
    };
    let query = json!({
        "response": {
            "height": "999",
            "code": 0,
            "value": base64_encode(&table),
            "proofOps": {"ops": [
                {"type":"ics23:iavl", "key":base64_encode(&[3]), "data":base64_encode(&inner)},
                {"type":"ics23:simple", "key":base64_encode(b"sequencer"), "data":base64_encode(&governed_keys.multistore)}
            ]}
        }
    });
    Evidence {
        receipt,
        block_hash,
        query,
        commit,
        block,
    }
}

struct Material {
    root: PathBuf,
    ca: Vec<u8>,
    server: Identity,
}

impl Drop for Material {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Material {
    fn client_identity(&self) -> Identity {
        checked(Identity::from_pkcs12(
            &checked(fs::read(self.root.join("client.p12"))),
            "test",
        ))
    }
}

fn run(command: &[&str]) {
    let output = checked(Command::new(command[0]).args(&command[1..]).output());
    assert!(
        output.status.success(),
        "{command:?}: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn material() -> Material {
    let root = std::env::temp_dir().join(format!(
        "webhook-consensus-calls-{}-{}",
        std::process::id(),
        checked(SystemTime::now().duration_since(UNIX_EPOCH)).as_nanos()
    ));
    checked(fs::create_dir_all(&root));
    let path = |name: &str| root.join(name).to_string_lossy().into_owned();
    run(&[
        "openssl",
        "req",
        "-x509",
        "-newkey",
        "ed25519",
        "-nodes",
        "-keyout",
        &path("ca-key.pem"),
        "-out",
        &path("ca.pem"),
        "-days",
        "1",
        "-subj",
        "/CN=webhook-consensus-ca",
        "-addext",
        "basicConstraints=critical,CA:TRUE",
        "-addext",
        "keyUsage=critical,keyCertSign,cRLSign",
    ]);
    for (name, usage, san) in [
        ("server", "serverAuth", "DNS:localhost"),
        ("client", "clientAuth", "DNS:localhost"),
    ] {
        run(&[
            "openssl",
            "req",
            "-new",
            "-newkey",
            "ed25519",
            "-nodes",
            "-keyout",
            &path(&format!("{name}-key.pem")),
            "-out",
            &path(&format!("{name}.csr")),
            "-subj",
            &format!("/CN={name}"),
        ]);
        checked(fs::write(root.join(format!("{name}.ext")), format!("basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage={usage}\nsubjectAltName={san}\n")));
        run(&[
            "openssl",
            "x509",
            "-req",
            "-in",
            &path(&format!("{name}.csr")),
            "-CA",
            &path("ca.pem"),
            "-CAkey",
            &path("ca-key.pem"),
            "-CAcreateserial",
            "-out",
            &path(&format!("{name}.pem")),
            "-days",
            "1",
            "-extfile",
            &path(&format!("{name}.ext")),
        ]);
        run(&[
            "openssl",
            "pkcs12",
            "-export",
            "-inkey",
            &path(&format!("{name}-key.pem")),
            "-in",
            &path(&format!("{name}.pem")),
            "-certfile",
            &path("ca.pem"),
            "-out",
            &path(&format!("{name}.p12")),
            "-passout",
            "pass:test",
        ]);
    }
    run(&[
        "openssl",
        "x509",
        "-in",
        &path("ca.pem"),
        "-outform",
        "DER",
        "-out",
        &path("ca.der"),
    ]);
    Material {
        ca: checked(fs::read(root.join("ca.der"))),
        server: checked(Identity::from_pkcs12(
            &checked(fs::read(root.join("server.p12"))),
            "test",
        )),
        root,
    }
}

fn read_request(stream: &mut impl Read) -> (String, Value) {
    let mut bytes = Vec::new();
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let count = checked(stream.read(&mut chunk));
        assert!(count > 0, "request ended before headers");
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(index) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            break index + 4;
        }
    };
    let headers = String::from_utf8_lossy(&bytes[..header_end]);
    let path = headers
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .unwrap_or_default()
        .to_owned();
    let length = headers
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .or_else(|| {
            headers
                .lines()
                .find_map(|line| line.strip_prefix("content-length: "))
        })
        .and_then(|value| value.parse::<usize>().ok())
        .unwrap_or(0);
    while bytes.len() < header_end + length {
        let count = checked(stream.read(&mut chunk));
        assert!(count > 0, "request body truncated");
        bytes.extend_from_slice(&chunk[..count]);
    }
    let body = if length == 0 {
        Value::Null
    } else {
        checked(serde_json::from_slice(
            &bytes[header_end..header_end + length],
        ))
    };
    (path, body)
}

fn serve(
    listener: TcpListener,
    identity: Identity,
    responses: Vec<Value>,
) -> thread::JoinHandle<Vec<(String, Value)>> {
    thread::spawn(move || {
        let acceptor = checked(TlsAcceptor::new(identity));
        let mut calls = Vec::new();
        for response in responses {
            let deadline = Instant::now() + Duration::from_secs(15);
            let socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "request timed out");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            let mut stream = checked(acceptor.accept(socket));
            let (path, request) = read_request(&mut stream);
            calls.push((path, request));
            let body = serde_json::to_vec(&json!({"jsonrpc":"2.0","id":1,"result":response}))
                .unwrap_or_default();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap_or_else(|error| panic!("response headers: {error}"));
            stream
                .write_all(&body)
                .unwrap_or_else(|error| panic!("response body: {error}"));
            stream
                .flush()
                .unwrap_or_else(|error| panic!("response flush: {error}"));
        }
        calls
    })
}

fn free_listener() -> (TcpListener, u16) {
    let listener = checked(TcpListener::bind("127.0.0.1:0"));
    checked(listener.set_nonblocking(true));
    let port = checked(listener.local_addr()).port();
    (listener, port)
}

#[test]
fn verifier_consensus_calls_use_the_fixture_and_preserve_a_comet_path() {
    let calls: Vec<Value> = checked(serde_json::from_str(include_str!(
        "fixtures/verifier-consensus-calls.json"
    )));
    let material = material();
    let evidence = evidence();
    let receipt_json = json!({
        "rung":"committed",
        "receipt":format!("0x{}", hex_encode(&evidence.receipt)),
        "height":"0x3e8",
        "blockHash":format!("0x{}", hex_encode(&evidence.block_hash)),
    });
    let (consensus_listener, consensus_port) = free_listener();
    let consensus = serve(
        consensus_listener,
        material.server.clone(),
        vec![
            json!({"node_info":{"network":CHAIN_ID}}),
            evidence.commit.clone(),
            evidence.block.clone(),
            evidence.query.clone(),
        ],
    );
    let (node_listener, node_port) = free_listener();
    let node = serve(
        node_listener,
        material.server.clone(),
        vec![
            json!("0x7d"),
            receipt_json,
            json!({"transactions":[format!("0x{ACTIVITY}")]}),
        ],
    );
    let (source_listener, source_port) = free_listener();
    let source = serve(
        source_listener,
        material.server.clone(),
        vec![json!({"ready":true}); 4],
    );
    let policy_path = material.root.join("chain.json");
    checked(fs::write(
        &policy_path,
        serde_json::to_vec(&policy()).unwrap_or_default(),
    ));
    let password_path = material.root.join("identity-password");
    let node_token_path = material.root.join("node-token");
    checked(fs::write(&password_path, "test"));
    checked(fs::write(&node_token_path, "token"));
    for stem in ["JOURNEY", "PAYMENT", "APPROVAL", "PROGRAM"] {
        let token_path = material.root.join(format!("{stem}-token"));
        checked(fs::write(&token_path, "source-token"));
        std::env::set_var(
            format!("LAYERX_WEBHOOKS_{stem}_SOURCE_URL"),
            format!("https://localhost:{source_port}"),
        );
        std::env::set_var(
            format!("LAYERX_WEBHOOKS_{stem}_SOURCE_TOKEN_FILE"),
            token_path,
        );
    }
    std::env::set_var(
        "LAYERX_WEBHOOKS_INTERNAL_CA_DER",
        material.root.join("ca.der"),
    );
    std::env::set_var(
        "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12",
        material.root.join("client.p12"),
    );
    std::env::set_var(
        "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE",
        &password_path,
    );
    std::env::set_var("LAYERX_WEBHOOKS_CHAIN_POLICY_FILE", &policy_path);
    std::env::set_var(
        "LAYERX_WEBHOOKS_NODE_URL",
        format!("https://localhost:{node_port}"),
    );
    std::env::set_var(
        "LAYERX_WEBHOOKS_COMET_URL",
        format!("https://localhost:{consensus_port}/comet"),
    );
    std::env::set_var("LAYERX_WEBHOOKS_NODE_TOKEN_FILE", &node_token_path);
    let sources = checked(TrustedSources::from_environment());
    assert!(sources.ready_until(Instant::now() + Duration::from_secs(10)));
    let verifier = checked(ReceiptVerifier::new(
        &format!("https://localhost:{node_port}"),
        &format!("https://localhost:{consensus_port}/comet"),
        "token".to_owned(),
        &material.ca,
        material.client_identity(),
        policy(),
    ));
    let verified = verifier.verify(ACTIVITY);
    assert!(verified.is_ok(), "verification failed");
    let consensus_calls = checked(consensus.join());
    let node_calls = checked(node.join());
    assert_eq!(consensus_calls.len(), calls.len());
    for ((path, request), expected) in consensus_calls.iter().zip(calls) {
        assert_eq!(path, "/comet/");
        assert_eq!(request["method"], expected["method"]);
        assert_eq!(request["params"], expected["params"]);
    }
    assert_eq!(node_calls.len(), 3);
    assert_eq!(node_calls[0].1["method"], "eth_chainId");
    assert_eq!(node_calls[1].1["method"], "pax_sequencerReceipt");
    assert_eq!(node_calls[2].1["method"], "eth_getBlockByNumber");
    assert_eq!(checked(source.join()).len(), 4);
}
