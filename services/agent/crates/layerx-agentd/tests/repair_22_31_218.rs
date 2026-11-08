use std::thread;
use std::time::Duration;

use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::read::{
    AbciResultProof, CommitValidator, HistoryLimits, HistoryPage, NativeProofRequest,
    NativeReadRoute, NativeResultSource,
};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_programs::hex;
use layerx_types::rung::Rung;
use serde_json::{json, Value};

#[test]
fn proof_root_algorithm_uses_node_emitted_abci_bytes() {
    let vector: Value = serde_json::from_str(include_str!(
        "../../../../../consensus/types/testdata/results_hash_v1.json"
    ))
    .unwrap();
    let root = hex::decode_digest(vector["abci_results_hash"].as_str().unwrap()).unwrap();
    // Values are the exact producer vector, converted only from its documented
    // hexadecimal representation into Comet RPC's base64 representation.
    let results = json!([
        {"code":0,"data":"AAH/","gas_wanted":"100","gas_used":"73"},
        {"code":7,"data":"ZGVuaWVk","gas_wanted":"200","gas_used":"150"},
        {"code":0,"data":"","gas_wanted":"0","gas_used":"0"}
    ]);
    for (index, emitted) in vector["results"].as_array().unwrap().iter().enumerate() {
        let proof = NativeResultSource::from_results(&results, index as u64).unwrap();
        assert_eq!(
            proof.result,
            hex::decode(emitted["protobuf"].as_str().unwrap()).unwrap()
        );
        assert_eq!(
            proof.proof.leaf_hash,
            hex::decode_digest(emitted["proof"]["leaf_hash"].as_str().unwrap()).unwrap()
        );
        let aunts = emitted["proof"]["aunts"]
            .as_array()
            .unwrap()
            .iter()
            .map(|aunt| hex::decode_digest(aunt.as_str().unwrap()).unwrap())
            .collect::<Vec<_>>();
        assert_eq!(proof.proof.aunts, aunts);
        proof.verify(root, index as u64).unwrap();
        let mut recovered = AbciResultProof::decode(&proof.encode().unwrap()).unwrap();
        assert_eq!(proof, recovered);
        recovered.result.push(0);
        assert!(recovered.verify(root, index as u64).is_err());
        assert!(proof.verify(root, (index + 1) as u64).is_err());
    }
}

#[test]
fn proof_root_commits_recorded_node_results_at_successor_height() {
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::TcpListener;
    let recording: Value = serde_json::from_str(include_str!(
        "../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json"
    ))
    .unwrap();
    let settlement = &recording["settlement"];
    let validators = settlement["validators"]
        .as_array()
        .unwrap()
        .iter()
        .map(|v| CommitValidator {
            address: hex::decode(v["address"].as_str().unwrap())
                .unwrap()
                .try_into()
                .unwrap(),
            public_key: openssl::base64::decode_block(v["pub_key"]["value"].as_str().unwrap())
                .unwrap()
                .try_into()
                .unwrap(),
            voting_power: v["voting_power"].as_str().unwrap().parse().unwrap(),
        })
        .collect::<Vec<_>>();
    let signed =
        json!({"signed_header":{"header":settlement["header"],"commit":settlement["commit"]}});
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let source =
        NativeResultSource::connect(&format!("http://{}", listener.local_addr().unwrap()), None)
            .unwrap();
    let server = thread::spawn(move || {
        let (mut stream, _) = listener.accept().unwrap();
        stream
            .set_read_timeout(Some(Duration::from_secs(3)))
            .unwrap();
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();
        reader.read_line(&mut line).unwrap();
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = 0;
        loop {
            line.clear();
            assert_ne!(reader.read_line(&mut line).unwrap(), 0);
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = value.trim().parse().unwrap();
                }
            }
        }
        let mut body = vec![0; length];
        reader.read_exact(&mut body).unwrap();
        let request: Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(request["method"], "commit");
        assert_eq!(request["params"], json!({"height":"11"}));
        let body = json!({"jsonrpc":"2.0","id":request["id"],"result":signed}).to_string();
        write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()).unwrap();
    });
    let checkpoint = source
        .checkpoint(11, "committed-fixture", &validators)
        .unwrap();
    server.join().unwrap();
    let proof =
        NativeResultSource::from_results(&json!([{}, settlement["exec_tx_result"]]), 1).unwrap();
    assert_eq!(
        proof.result,
        decode_hex(&settlement["exec_tx_result_bytes"])
    );
    assert_eq!(
        proof.proof.leaf_hash.as_slice(),
        openssl::base64::decode_block(
            settlement["exec_tx_result_proof"]["leaf_hash"]
                .as_str()
                .unwrap()
        )
        .unwrap()
    );
    proof
        .verify_committed(10, 1, &checkpoint, "committed-fixture", &validators)
        .unwrap();
    assert!(proof
        .verify_committed(11, 1, &checkpoint, "committed-fixture", &validators)
        .is_err());
    assert_ne!(
        settlement["execution_header"]["last_results_hash"],
        settlement["header"]["last_results_hash"]
    );
    let mut insufficient = checkpoint.commit.clone();
    insufficient.votes.clear();
    assert!(layerx_agentd::read::checkpoint(
        11,
        "committed-fixture",
        &checkpoint.header,
        &insufficient,
        &validators
    )
    .is_err());
}

#[test]
fn history_policy_shape_consumes_actual_hosted_business_rows() {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
    ))
    .unwrap();
    let result = &document["result"];
    let hosted = layerx_client::sequencer::HistoryPage {
        account: result["account"]["evm_address"]
            .as_str()
            .unwrap()
            .to_owned(),
        freshness: result["freshness"].clone(),
        items: result["items"].as_array().unwrap().clone(),
        next_cursor: result["next_cursor"].as_str().map(str::to_owned),
        indexed_height: result["indexedHeight"].as_u64(),
        chain_height: result["chainHeight"].as_u64(),
    };
    assert_eq!(hosted.account, result["items"][0]["account"]);
    assert_eq!(hosted.freshness, result["freshness"]);
    let limits = HistoryLimits {
        maximum_items: 100,
        maximum_response_bytes: 240 * 1024,
    };
    let page = HistoryPage::from_hosted(hosted.clone(), 0, u64::MAX, None, limits).unwrap();
    assert_eq!(page.items.len(), 4);
    assert_eq!(page.items[0].position.block_height, 10);
    assert_eq!(page.items[3].position.tx_index, 1);
    assert_eq!(
        serde_json::from_slice::<Value>(&page.items[0].canonical).unwrap(),
        hosted.items[0]
    );
    assert!(HistoryPage::from_hosted(
        hosted.clone(),
        0,
        u64::MAX,
        None,
        HistoryLimits {
            maximum_items: 1,
            ..limits
        }
    )
    .is_err());
    assert!(HistoryPage::from_hosted(
        hosted.clone(),
        0,
        u64::MAX,
        None,
        HistoryLimits {
            maximum_response_bytes: 1,
            ..limits
        }
    )
    .is_err());
    let mut reordered = hosted;
    reordered.items.reverse();
    assert!(HistoryPage::from_hosted(reordered, 0, u64::MAX, None, limits).is_err());
}

// The RPC peer is an in-process committed fixture. Production readers, codecs,
// signature checks, Merkle verification and storage verification run unchanged.
// Receipt bytes are the Go producer's conformance recording. Consensus test
// authorities and in-memory store proofs come from the Programs fixture store;
// they are deliberately not represented as a recording of a live deployment.
#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

const CHAIN: &str = "repair-22-31-218";
const HEIGHT: u64 = 1000;
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const NOW: u64 = 1_700_000_000_123;

fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn quantity(value: u64) -> String {
    format!("{value:#x}")
}
fn decode_hex(value: &Value) -> Vec<u8> {
    hex::decode(value.as_str().unwrap().strip_prefix("0x").unwrap()).unwrap()
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
        u64::from_str_radix(receipt[name].as_str().unwrap().trim_start_matches("0x"), 16).unwrap()
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

fn sign_fixture(fixture: &mut chain::ChainFixture, height: u64) {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_programs::chain_proof::{validators::TrustedValidator, vote::vote_sign_bytes};
    fixture.header.height = height as i64;
    fixture.commit.height = height as i64;
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
    let message = vote_sign_bytes(
        CHAIN,
        fixture.commit.height,
        fixture.commit.round,
        Some(&fixture.commit.block_id),
        fixture.commit.signatures[0].timestamp,
    )
    .unwrap();
    for sig in &mut fixture.commit.signatures {
        let key = SEEDS
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == sig.validator_address
            })
            .unwrap();
        sig.signature = key.sign(&message).to_bytes().to_vec();
    }
}
fn comet_commit(fixture: &chain::ChainFixture) -> Value {
    let header = &fixture.header;
    let commit = &fixture.commit;
    let time = "2023-11-14T22:13:20.123456789Z";
    json!({"signed_header": {
        "header": {
            "version":{"block":header.version.block.to_string(),"app":header.version.app.to_string()},
            "chain_id":CHAIN,"height":header.height.to_string(),"time":time,
            "last_block_id":{"hash":hex::encode(&header.last_block_id.hash),"parts":{"total":header.last_block_id.part_set_total,"hash":hex::encode(&header.last_block_id.part_set_hash)}},
            "last_commit_hash":hex::encode(&header.last_commit_hash),"data_hash":hex::encode(&header.data_hash),
            "validators_hash":hex::encode(&header.validators_hash),"next_validators_hash":hex::encode(&header.next_validators_hash),
            "consensus_hash":hex::encode(&header.consensus_hash),"app_hash":hex::encode(&header.app_hash),
            "last_results_hash":hex::encode(&header.last_results_hash),"evidence_hash":hex::encode(&header.evidence_hash),
            "proposer_address":hex::encode(&header.proposer_address)
        },
        "commit":{"height":commit.height.to_string(),"round":commit.round,
            "block_id":{"hash":hex::encode(&commit.block_id.hash),"parts":{"total":commit.block_id.part_set_header.total,"hash":hex::encode(&commit.block_id.part_set_header.hash)}},
            "signatures":commit.signatures.iter().map(|sig| json!({"block_id_flag":2,"validator_address":hex::encode(&sig.validator_address),"timestamp":time,"signature":openssl::base64::encode_block(&sig.signature)})).collect::<Vec<_>>()}
    }})
}

fn word(value: u64) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[24..].copy_from_slice(&value.to_be_bytes());
    bytes
}
fn fixture_did() -> layerx_types::ids::Did {
    layerx_types::ids::Did::new(format!("did:layerx:{}", hex::encode(&[9; 32])).as_bytes()).unwrap()
}

struct LocalNode {
    rpc: String,
    raw: Vec<u8>,
    receipt: Value,
    validators: Vec<CommitValidator>,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl LocalNode {
    fn start() -> Self {
        Self::with_account_state(false)
    }
    fn with_account_state(account_state: bool) -> Self {
        Self::fixture(account_state, 20, 4)
    }
    fn with_funding(balance: u64) -> Self {
        Self::fixture(false, balance, 4)
    }
    fn with_insufficient_quorum() -> Self {
        Self::fixture(false, 20, 2)
    }
    fn fixture(account_state: bool, funded_balance: u64, signed_votes: usize) -> Self {
        use layerx_programs::chain_proof::merkle::hash_from_byte_slices;
        use sha2::{Digest, Sha256};
        use std::io::{BufRead, BufReader, Read, Write};
        use std::net::TcpListener;
        use std::sync::{
            atomic::{AtomicBool, Ordering},
            Arc,
        };

        let vectors: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap();
        let vector = &vectors["cases"][1];
        let raw = decode_hex(&vector["rawTransaction"]);
        let receipt = vector["receiptJson"].clone();
        let key = vector["pinnedKey"].clone();
        let older_receipt = vectors["cases"][0]["receiptJson"].clone();
        let recorded_history: Value = serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        ))
        .unwrap();
        let mut history_rows = recorded_history["result"]["items"]
            .as_array()
            .unwrap()
            .clone();
        for (index, row) in history_rows.iter_mut().enumerate() {
            row["blockHeight"] = json!(HEIGHT);
            row["txIndex"] = json!(if index < 2 { 1 } else { 2 });
            row["logIndex"] = json!(0);
            row["tx_hash"] = if index < 2 {
                older_receipt["txHash"].clone()
            } else {
                receipt["txHash"].clone()
            };
        }
        let account_vector: Value = serde_json::from_str(include_str!(
            "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        ))
        .unwrap();
        let account_vector = account_vector["entries"][0].clone();
        let mut execution =
            chain::ChainFixture::new(CHAIN, &SEEDS, vec![(vec![2], 2_u64.to_be_bytes().to_vec())])
                .unwrap();
        let mut successor =
            chain::ChainFixture::new(CHAIN, &SEEDS, vec![(vec![2], 2_u64.to_be_bytes().to_vec())])
                .unwrap();
        // The account peer uses the actual EVM store vector, including its
        // recorded multistore root. Its independent test authority commits it.
        if account_state {
            successor.header.app_hash =
                hex::decode(account_vector["app_hash"].as_str().unwrap()).unwrap();
        }
        let mut batch = b"PAXEER/v1/sequence-batch\0".to_vec();
        batch.extend_from_slice(&[0; 216]);
        let offset = b"PAXEER/v1/sequence-batch\0".len();
        batch[offset..offset + 8].copy_from_slice(&HEIGHT.to_be_bytes());
        batch[offset + 8..offset + 16].copy_from_slice(&1_u64.to_be_bytes());
        batch[offset + 16..offset + 24].copy_from_slice(&2_u64.to_be_bytes());
        execution.header.data_hash = hash_from_byte_slices(&[&Sha256::digest(&batch)]).to_vec();
        sign_fixture(&mut execution, HEIGHT);
        successor.header.last_block_id.hash = execution.header.hash().unwrap().to_vec();
        let response = abci_response(&receipt);
        let mut result = Vec::new();
        protobuf_bytes(2, &response, &mut result);
        protobuf_integer(5, 60_000, &mut result);
        protobuf_integer(6, 46_220, &mut result);
        successor.header.last_results_hash = hash_from_byte_slices(&[&[], &[], &result]).to_vec();
        let mut next_batch = batch.clone();
        next_batch[offset..offset + 8].copy_from_slice(&(HEIGHT + 1).to_be_bytes());
        successor.header.data_hash =
            hash_from_byte_slices(&[&Sha256::digest(&next_batch)]).to_vec();
        sign_fixture(&mut successor, HEIGHT + 1);
        let validators = execution
            .verifier
            .validators
            .validators()
            .iter()
            .map(|v| CommitValidator {
                address: v.address,
                public_key: v.public_key,
                voting_power: v.voting_power,
            })
            .collect::<Vec<_>>();
        let mut execution_commit = comet_commit(&execution);
        for sig in &mut execution_commit["signed_header"]["commit"]["signatures"]
            .as_array_mut()
            .unwrap()[signed_votes..]
        {
            *sig = json!({"block_id_flag":1,"validator_address":"","timestamp":"0001-01-01T00:00:00Z","signature":null});
        }
        let successor_commit = comet_commit(&successor);
        let block = |fixture: &chain::ChainFixture, results: &[u8], txs: Vec<Value>| {
            json!({
                "number":quantity(fixture.header.height as u64),"hash":rpc_hex(&fixture.header.hash().unwrap()),
                "parentHash":rpc_hex(&fixture.header.last_block_id.hash),"timestamp":quantity(fixture.header.time.seconds as u64),
                "transactionsRoot":rpc_hex(&fixture.header.data_hash),"receiptsRoot":rpc_hex(results),"stateRoot":rpc_hex(&fixture.header.app_hash),"transactions":txs
            })
        };
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw).unwrap();
        let signature = signature.as_bytes();
        let integer_hex = |bytes: &[u8]| {
            let text = hex::encode(bytes);
            format!("0x{}", text.trim_start_matches('0'))
        };
        let tx = json!({
            "blockHash":rpc_hex(&execution.header.hash().unwrap()),"blockNumber":quantity(HEIGHT),"from":receipt["from"],
            "gas":quantity(call.gas_limit),"gasPrice":"0x77359400","hash":rpc_hex(&transaction_hash(&raw)),
            "input":rpc_hex(&call.data),"nonce":quantity(call.nonce),"to":rpc_hex(&call.to),"transactionIndex":"0x0",
            "value":"0x0","type":"0x2","v":quantity(u64::from(signature[64]-27)),"r":integer_hex(&signature[..32]),"s":integer_hex(&signature[32..64]),
            "chainId":"0x7d","accessList":[],"yParity":quantity(u64::from(signature[64]-27)),"maxFeePerGas":format!("{:#x}",call.max_fee_per_gas),"maxPriorityFeePerGas":format!("{:#x}",call.max_priority_fee_per_gas)
        });
        let execution_block = block(&execution, &successor.header.last_results_hash, vec![tx]);
        let successor_block = block(&successor, &[6; 32], vec![]);
        let host_ops = |fixture: &chain::ChainFixture| {
            fixture.prove(&[2]).unwrap().iter().map(|op|json!({"type":op.op_type,"key":openssl::base64::encode_block(&op.key),"data":openssl::base64::encode_block(&op.data)})).collect::<Vec<_>>()
        };
        let execution_ops = host_ops(&execution);
        let successor_ops = host_ops(&successor);
        let published = validators.iter().map(|v| json!({"address":hex::encode(&v.address),"pub_key":{"type":"tendermint/PubKeyEd25519","value":openssl::base64::encode_block(&v.public_key)},"voting_power":v.voting_power.to_string()})).collect::<Vec<_>>();
        let receipt_peer = receipt.clone();
        let did = fixture_did();
        let owner = layerx_wire::hash::did_id_for_protocol(&did, 3).unwrap();
        let asset = [4; 32];
        let account = layerx_types::account::AccountId::for_asset(
            std::str::from_utf8(did.as_bytes()).unwrap(),
            asset,
            asset,
        )
        .unwrap();
        let main_account = layerx_wire::hash::account_id_for_protocol(&account, 3).unwrap();
        let mut address = [0; 32];
        address[12..].fill(5);
        let mut binding = [word(64), word(did.as_bytes().len() as u64)].concat();
        binding.splice(0..0, [9; 32]);
        binding.extend_from_slice(did.as_bytes());
        binding.resize(binding.len().div_ceil(32) * 32, 0);
        let mut metadata = vec![0; 12 * 32];
        metadata[..32].copy_from_slice(&asset);
        metadata[32..64].copy_from_slice(&word(384));
        metadata[64..96].copy_from_slice(&word(448));
        metadata[96..128].copy_from_slice(&word(18));
        metadata[160..192].copy_from_slice(&owner);
        metadata[9 * 32..10 * 32].copy_from_slice(&word(512));
        for name in [b"PAX", b"Pax"] {
            metadata.extend_from_slice(&word(3));
            metadata.extend_from_slice(name);
            metadata.resize(metadata.len().div_ceil(32) * 32, 0);
        }
        metadata.extend_from_slice(&word(0));
        let mut budget = vec![0; 20 * 32];
        for (index, value) in [
            (0, 32),
            (5, 100),
            (6, 100),
            (8, 23),
            (10, 1000),
            (11, 1),
            (12, u64::MAX),
            (14, 1),
            (17, 576),
        ] {
            budget[index * 32..(index + 1) * 32].copy_from_slice(&word(value));
        }
        budget[32..64].copy_from_slice(&owner);
        budget[64..96].fill(3);
        budget[96..128].copy_from_slice(&asset);
        budget[128..160].fill(5);
        let selector = |name: &[u8]| transaction_hash(name)[..4].to_vec();
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let rpc = format!("http://{}", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(e) => panic!("fixture listener: {e}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                let mut reader = BufReader::new(&stream);
                let mut line = String::new();
                reader.read_line(&mut line).unwrap();
                assert_eq!(line, "POST / HTTP/1.1\r\n");
                let mut length = 0;
                loop {
                    line.clear();
                    assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = value.trim().parse().unwrap();
                        }
                    }
                }
                let mut body = vec![0; length];
                reader.read_exact(&mut body).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                let params = &request["params"];
                let method = request["method"].as_str().unwrap();
                let selected = params["height"]
                    .as_str()
                    .and_then(|v| v.parse::<u64>().ok());
                let result = match method {
                    "eth_chainId" => json!("0x7d"),
                    "eth_blockNumber" => json!(quantity(HEIGHT + 1)),
                    "px_getReceiptKeys" => json!({"keys":[key]}),
                    "pax_sequencerReceipt" => {
                        if params[0] == receipt_peer["txHash"] {
                            receipt_peer.clone()
                        } else {
                            assert_eq!(params[0], older_receipt["txHash"]);
                            older_receipt.clone()
                        }
                    }
                    "px_getHistory" => {
                        use layerx_agentd::read::Cursor;
                        let cursor = params[1]
                            .as_str()
                            .map(|text| Cursor::from_hex(text).unwrap());
                        let position = |row: &Value| Cursor {
                            block_height: row["blockHeight"].as_u64().unwrap(),
                            tx_index: row["txIndex"].as_u64().unwrap() as u32,
                            log_index: row["logIndex"].as_u64().unwrap() as u32,
                        };
                        let rows = history_rows
                            .iter()
                            .filter(|row| cursor.is_none_or(|c| position(row) > c))
                            .collect::<Vec<_>>();
                        let limit = params[2].as_u64().unwrap() as usize;
                        assert!(limit > 0 && limit <= 100);
                        let mut count = limit.min(rows.len());
                        while count < rows.len()
                            && position(rows[count]) == position(rows[count - 1])
                        {
                            count += 1;
                        }
                        let next = (count < rows.len()).then(|| position(rows[count - 1]).to_hex());
                        let transcript: Value = serde_json::from_str(include_str!(
                            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
                        ))
                        .unwrap();
                        json!({
                            "account": params[0],
                            "items": rows[..count],
                            "next_cursor": next,
                            "indexedHeight": HEIGHT + 1,
                            "chainHeight": HEIGHT + 1,
                            "freshness": transcript["result"]["freshness"]
                        })
                    }
                    "eth_getTransactionReceipt" => Value::Null,
                    "eth_getBlockByNumber" => match params[0].as_str().unwrap() {
                        "0x3e8" => execution_block.clone(),
                        "0x3e9" => successor_block.clone(),
                        other => panic!("unexpected EVM height {other}"),
                    },
                    "commit" => match selected {
                        Some(HEIGHT) => execution_commit.clone(),
                        Some(h) if h == HEIGHT + 1 => successor_commit.clone(),
                        _ => panic!("unknown commit {params}"),
                    },
                    "validators" => {
                        assert!(selected == Some(HEIGHT) || selected == Some(HEIGHT + 1));
                        json!({"block_height":selected.unwrap().to_string(),"count":"4","total":"4","validators":published})
                    }
                    "block" => {
                        let h = selected.unwrap();
                        assert!(h == HEIGHT || h == HEIGHT + 1);
                        json!({"block":{"header":{"height":h.to_string()},"data":{"txs":[openssl::base64::encode_block(if h==HEIGHT {&batch} else {&next_batch})]}}})
                    }
                    "block_results" => {
                        assert_eq!(selected, Some(HEIGHT));
                        json!({"height":HEIGHT.to_string(),"txs_results":[{}, {}, {"data":openssl::base64::encode_block(&response),"gas_wanted":"60000","gas_used":"46220"}]})
                    }
                    "abci_query" if params["path"] == "/store/lxphost/key" => {
                        assert_eq!(params["data"], "02");
                        let h = selected.unwrap();
                        assert!(h == HEIGHT - 1 || h == HEIGHT);
                        json!({"response":{"code":0,"height":h.to_string(),"key":openssl::base64::encode_block(&[2]),"value":openssl::base64::encode_block(&2_u64.to_be_bytes()),"proof_ops":{"ops":if h==HEIGHT-1 {&execution_ops}else{&successor_ops}}}})
                    }
                    "abci_query" => {
                        json!({"response":{"code":6,"height":selected.unwrap().to_string(),"log":"ProgramState producer is not registered"}})
                    }
                    "eth_call" => {
                        assert_eq!(params[1], quantity(HEIGHT));
                        let data = decode_hex(&params[0]["data"]);
                        let head = &data[..4];
                        let bytes = if head == selector(b"getEvmAddrByLayerX(bytes32)") {
                            assert_eq!(&data[4..], &[9; 32]);
                            address.to_vec()
                        } else if head == selector(b"getUnifiedAccount(address)") {
                            assert_eq!(&data[4..], &address);
                            [address, word(128), [9; 32], main_account, word(0)].concat()
                        } else if head == selector(b"getLayerXDid(address)") {
                            assert_eq!(&data[4..], &address);
                            binding.clone()
                        } else if head == selector(b"getAsset(bytes32)") {
                            assert_eq!(&data[4..], &asset);
                            metadata.clone()
                        } else if head == selector(b"accountId(bytes32,bytes32)") {
                            assert_eq!(&data[4..], &[owner, asset].concat());
                            main_account.to_vec()
                        } else if head == selector(b"accountOwner(bytes32)") {
                            assert_eq!(&data[4..], &main_account);
                            owner.to_vec()
                        } else if head == selector(b"balanceOf(bytes32,bytes32)") {
                            assert_eq!(&data[36..], &asset);
                            if data[4..36] == [3; 32] {
                                word(funded_balance).to_vec()
                            } else {
                                assert_eq!(&data[4..36], &main_account);
                                word(100).to_vec()
                            }
                        } else if head == selector(b"nextSequence(bytes32,bytes32)") {
                            assert_eq!(&data[4..], &[main_account, asset].concat());
                            word(1).to_vec()
                        } else if head == selector(b"getBudget(bytes32)") {
                            assert_eq!(&data[4..], &[1; 32]);
                            budget.clone()
                        } else if head == selector(b"budgetRemaining(bytes32)") {
                            assert_eq!(&data[4..], &[1; 32]);
                            word(funded_balance.min(77)).to_vec()
                        } else {
                            panic!("undeclared committed call {params}")
                        };
                        json!(rpc_hex(&bytes))
                    }
                    "eth_getProof" => {
                        assert!(account_state);
                        assert_eq!(
                            params[0],
                            format!("0x{}", account_vector["address"].as_str().unwrap())
                        );
                        assert_eq!(
                            params[1],
                            json!([format!("0x{}", account_vector["slot"].as_str().unwrap())])
                        );
                        assert_eq!(params[2], quantity(HEIGHT));
                        account_vector["response"].clone()
                    }
                    _ => panic!("undeclared native RPC {request}"),
                };
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()).unwrap();
            }
        });
        Self {
            rpc,
            raw,
            receipt,
            validators,
            stop,
            worker: Some(worker),
        }
    }
    fn validators(&self) -> Vec<CommitValidator> {
        self.validators.clone()
    }
    fn endpoint(&self) -> SequencerEndpoint {
        SequencerEndpoint::new(&self.rpc, &[]).unwrap()
    }
    fn source(&self) -> NativeResultSource {
        NativeResultSource::connect(&self.rpc, None).unwrap()
    }
    fn route(&self) -> NativeReadRoute {
        NativeReadRoute::new(self.endpoint(), 125)
            .with_checkpoint_trust(CHAIN.into(), self.validators())
            .unwrap()
            .with_result_source(self.source())
    }
}
impl Drop for LocalNode {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::SeqCst);
        let result = self.worker.take().unwrap().join();
        if !thread::panicking() {
            result.unwrap();
        }
    }
}

#[test]
fn native_checkpoint_proof_acquisition_and_restart_use_real_node() {
    let node = LocalNode::start();
    let endpoint = node.endpoint();
    let validators = node.validators();
    let height = HEIGHT;
    let path = format!("/v1/reads/checkpoint/{height}");
    let mut route = node.route();
    assert!(NativeReadRoute::new(node.endpoint(), 125)
        .read(&path)
        .is_err());
    let served = route
        .read(&path)
        .expect("native route must authenticate committed quorum");
    assert_eq!(served["verified"], true);
    assert_eq!(served["rung"], "committed");
    assert!(
        LocalNode::with_insufficient_quorum()
            .route()
            .read(&path)
            .is_err(),
        "the served checkpoint route must reject half of trusted power"
    );
    let checkpoint = node
        .source()
        .checkpoint(height, CHAIN, &validators)
        .unwrap();
    let mut insufficient = checkpoint.commit.clone();
    insufficient.votes.truncate(2);
    assert!(layerx_agentd::read::checkpoint(
        height,
        CHAIN,
        &checkpoint.header,
        &insufficient,
        &validators
    )
    .is_err());
    let mut damaged = checkpoint.commit.clone();
    damaged.votes[0].signature[0] ^= 1;
    assert!(layerx_agentd::read::checkpoint(
        height,
        CHAIN,
        &checkpoint.header,
        &damaged,
        &validators
    )
    .is_err());
    assert!(
        node.source()
            .program_state_record(
                [1; 32],
                b"unregistered-program-state",
                "lxphost",
                &checkpoint
            )
            .is_err(),
        "a missing native snapshot producer must not become a successful program state"
    );
    let commitment = checkpoint.commitment_proof().unwrap();
    let decoded =
        layerx_programs::chain_proof::header::ChainHeader::decode(&commitment.signed_header)
            .unwrap();
    assert_eq!(decoded.hash().unwrap(), checkpoint.block_hash);
    let (decoded_header, decoded_commit, decoded_validators) =
        layerx_programs::chain_evidence::commitment_trust_step(&commitment).unwrap();
    layerx_programs::chain_proof::ChainProofVerifier {
        chain_id: CHAIN.into(),
        validators: decoded_validators,
    }
    .verify_head(&decoded_header, &decoded_commit)
    .unwrap();
    let program_reader = || {
        layerx_agentd::read::LayerxdProgramBalanceReader::connect_native(
            &node.rpc,
            "fixture-reader".into(),
            None,
            node.source(),
            layerx_programs::ProtocolDeploymentVerifier::from_chain_trust(
                CHAIN,
                validators
                    .iter()
                    .map(|v| {
                        layerx_programs::chain_proof::validators::TrustedValidator::new(
                            v.public_key,
                            v.voting_power,
                        )
                    })
                    .collect(),
                60_000,
            )
            .unwrap(),
            layerx_programs::Registry::new(),
        )
        .unwrap()
    };
    let reader = program_reader();
    let historical = reader
        .read_chain_head_at(height, NOW)
        .expect("native historical chain evidence");
    assert_eq!(historical.freshness().observed_sequence, height);
    let current = reader
        .read_chain_head(NOW)
        .expect("native current chain evidence");
    assert!(current.freshness().observed_sequence >= height);
    drop(reader);
    use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
    let directory = std::env::temp_dir().join(format!(
        "repair-22-31-218-{}-{}",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos()
    ));
    let journal_key = TenantKey::new(
        TenantId::new("native-reader").unwrap(),
        ObjectKind::Configuration,
        b"historical-head".to_vec(),
    )
    .unwrap();
    let mut store = Store::open(&directory).unwrap();
    store
        .put_local(journal_key.clone(), height.to_be_bytes().to_vec())
        .unwrap();
    drop(store);
    let reopened_store = Store::open(&directory).unwrap();
    let retained_height = u64::from_be_bytes(
        reopened_store
            .get(&journal_key)
            .unwrap()
            .bytes()
            .try_into()
            .unwrap(),
    );
    let recovered = program_reader()
        .read_chain_head_at(retained_height, NOW)
        .expect("native historical evidence after reopening reader");
    assert_eq!(historical.receipt_digest(), recovered.receipt_digest());
    assert_eq!(historical.state_root(), recovered.state_root());
    drop(reopened_store);
    std::fs::remove_dir_all(directory).unwrap();
    drop(route);
    let mut reopened = node.route();
    assert_eq!(
        reopened.read(&path).unwrap(),
        served,
        "reopening preserves authenticated historical coordinates"
    );
    let hash = transaction_hash(&node.raw);
    assert_eq!(rpc_hex(&hash), node.receipt["txHash"]);
    let keys = endpoint.receipt_keys().unwrap();
    let Lookup::Verified(receipt) = lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .unwrap() else {
        panic!("recorded receipt must authenticate")
    };
    let received = layerx_agentd::read::acquire_native_receipt(
        &endpoint,
        ProofBundleTarget::Receipt(hash),
        Rung::Receipted,
        &node.raw,
        receipt.canonical_bytes(),
        1,
    )
    .unwrap();
    assert_eq!(received.achieved_rung, Rung::Receipted);
    let proof = reopened
        .read(&format!("/v1/reads/proof/{}", hex::encode(&hash)))
        .expect("native proof route must return full authenticated inclusion bundle");
    assert_eq!(proof["verificationLevel"], "committed");
    assert!(proof["receiptInclusion"]
        .as_str()
        .is_some_and(|bytes| !bytes.is_empty()));
    assert!(proof["commit"]
        .as_str()
        .is_some_and(|bytes| !bytes.is_empty()));
    let account_node = LocalNode::with_account_state(true);
    let mut account = [0; 32];
    account[12..]
        .copy_from_slice(&hex::decode("1122334455667788990011223344556677889900").unwrap());
    let slot =
        hex::decode_digest("0000000000000000000000000000000000000000000000000123456789abcdef")
            .unwrap();
    let account_bundle = NativeProofRequest {
        target: ProofBundleTarget::AccountState {
            activity_id: hash,
            account_id: account,
        },
        requested_rung: Rung::Committed,
        raw_transaction: &account_node.raw,
        served_receipt: receipt.canonical_bytes(),
        chain_id: CHAIN,
        validators: &validators,
        account_slots: &[slot],
    }
    .acquire(&account_node.endpoint(), &account_node.source())
    .expect("committed account acquisition must obtain authenticated storage material");
    assert!(account_bundle.value.account_proof.is_some());
}

#[test]
fn native_history_keeps_position_groups_and_authenticates_reverse_windows() {
    use layerx_agentd::read::Cursor;
    let node = LocalNode::start();
    let mut route = node.route();
    let first = route.read("/v1/reads/history/account?limit=1").unwrap();
    assert_eq!(first["items"].as_array().unwrap().len(), 2);
    assert_eq!(first["items"][0]["txIndex"], 1);
    assert_eq!(first["items"][1]["txIndex"], 1);
    let cursor = first["cursor"].as_str().unwrap();
    let second = route
        .read(&format!(
            "/v1/reads/history/account?limit=1&cursor={cursor}"
        ))
        .unwrap();
    assert_eq!(second["items"].as_array().unwrap().len(), 2);
    assert_eq!(second["items"][0]["txIndex"], 2);
    assert_eq!(second["complete"], true);
    assert!(second["cursor"].is_null());
    let limits = HistoryLimits {
        maximum_items: 1,
        maximum_response_bytes: 240 * 1024,
    };
    let newest = HistoryPage::read_newest(&node.endpoint(), "account", None, limits).unwrap();
    assert_eq!(newest.items.len(), 1);
    assert_eq!(newest.items[0].position.tx_index, 2);
    assert_eq!(
        newest.items[0].canonical,
        decode_hex(&node.receipt["receipt"])
    );
    let next = newest.next_cursor.unwrap();
    assert_eq!((next.block_height, next.tx_index), (HEIGHT, 2));
    let older = HistoryPage::read_newest(&node.endpoint(), "account", Some(next), limits).unwrap();
    assert_eq!(older.items.len(), 1);
    assert_eq!(older.items[0].position.tx_index, 1);
    assert!(older.next_cursor.is_none());
    // A forward page cannot overflow into a later position, even if its byte
    // size is within the response ceiling.
    let invalid = layerx_client::sequencer::HistoryPage {
        account: "account".to_owned(),
        freshness: serde_json::from_str::<Value>(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        ))
        .unwrap()["result"]["freshness"]
            .clone(),
        items: first["items"]
            .as_array()
            .unwrap()
            .iter()
            .chain(second["items"].as_array().unwrap())
            .cloned()
            .collect(),
        next_cursor: None,
        indexed_height: Some(HEIGHT + 1),
        chain_height: Some(HEIGHT + 1),
    };
    assert!(HistoryPage::from_hosted(invalid, 0, u64::MAX, None, limits).is_err());
    assert_eq!(Cursor::from_hex(cursor).unwrap().tx_index, 1);
}

#[test]
fn committed_inventory_selects_only_checkpoint_funded_budget() {
    use layerx_agentd::read::committed::CommittedReads;
    for balance in [20, 0] {
        let node = LocalNode::with_funding(balance);
        let endpoint = node.endpoint();
        let checkpoint = node
            .source()
            .checkpoint(HEIGHT, CHAIN, &node.validators())
            .unwrap();
        let reads = CommittedReads::at_checkpoint(&endpoint, &checkpoint).unwrap();
        let mut caps = reads
            .inventory(&fixture_did(), &[[4; 32]], [4; 32])
            .unwrap();
        reads.inventory_records(&mut caps, &[[1; 32]], &[]).unwrap();
        let funding = caps.budget_funding(&[1; 32]).unwrap();
        assert_eq!(funding.allowance(), 77);
        assert_eq!(funding.funded_balance(), u128::from(balance));
        assert_eq!(funding.remaining(), u128::from(balance));
        if balance > 0 {
            assert_eq!(
                caps.budget_source(&[1; 32], [4; 32], u128::from(balance))
                    .unwrap()
                    .remaining(),
                u128::from(balance)
            );
        }
        assert!(caps
            .budget_source(&[1; 32], [4; 32], u128::from(balance) + 1)
            .is_err());
        assert!(caps.budget_source(&[1; 32], [4; 32], 0).is_err());
    }
}

#[test]
fn committed_remaining_preserves_allowance_and_funded_spending_power() {
    use layerx_agentd::read::committed::CommittedBudgetFunding;
    use layerx_client::budget::ProtocolBudgetRecord;
    let vector: Value = serde_json::from_str(include_str!(
        "../../../../../tests/vectors/native-budget-records.json"
    ))
    .unwrap();
    for version in ["v1", "v2"] {
        let bytes = hex::decode(
            vector[version]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap(),
        )
        .unwrap();
        let record = ProtocolBudgetRecord::decode(&bytes).unwrap();
        assert_eq!(record.per_period_limit, 100);
        assert_eq!(record.spent_this_period, 23);
        let funded = CommittedBudgetFunding::verify_values(&record, 20, 20).unwrap();
        assert_eq!(funded.allowance(), 77);
        assert_eq!(funded.funded_balance(), 20);
        assert_eq!(funded.remaining(), 20);
        let defunded = CommittedBudgetFunding::verify_values(&record, 0, 0).unwrap();
        assert_eq!(defunded.allowance(), 77);
        assert_eq!(defunded.remaining(), 0);
        assert!(CommittedBudgetFunding::verify_values(&record, 20, 77).is_err());
        assert!(CommittedBudgetFunding::verify_values(&record, 0, 20).is_err());
    }
}
