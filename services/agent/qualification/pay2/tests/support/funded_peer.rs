use super::scenario::{funded_inputs, sequence};
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::{
    account_derivation::derive_from_seed,
    disclosure::call::{bind_call, ConstructMethod},
    disclosure::{CounterpartyRole, Disclosure},
    evm_transaction::{decode_signed, recover_sender, transaction_hash},
    local::LocalSigner,
};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    merkle::SimpleProof,
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, PartSetHeader, Timestamp},
    ChainProofVerifier,
};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_sdk::rpc::RpcClient;
use layerx_types::{clock::ClockReading, clock_protocol};
use layerx_wire::{encode::Encoder, receipt};
use serde_json::{json, Value};
use std::{
    collections::BTreeMap,
    fs,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    os::unix::{
        fs::{DirBuilderExt, MetadataExt, PermissionsExt},
        net::UnixListener,
    },
    sync::{
        atomic::{AtomicBool, AtomicU16, Ordering},
        Arc, Mutex,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("PAY2 fixture: {error:?}"))
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").expect("hex prefix");
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| checked(u8::from_str_radix(checked(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn hex(bytes: &[u8]) -> String {
    format!("0x{}", funded_inputs::hex_encode(bytes))
}

struct Worker {
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl Worker {
    fn start(run: impl FnOnce(Arc<AtomicBool>) + Send + 'static) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&stop);
        Self {
            stop,
            thread: Some(thread::spawn(move || run(flag))),
        }
    }

    fn join(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.thread.take() {
            if let Err(payload) = worker.join() {
                if !thread::panicking() {
                    std::panic::resume_unwind(payload);
                }
            }
        }
    }
}

impl Drop for Worker {
    fn drop(&mut self) {
        self.join();
    }
}

pub struct Peer {
    pub cluster: funded_inputs::Cluster,
    pub funding: super::scenario::funding::Funding,
    pub payer: LocalSigner,
    pub recipient: LocalSigner,
    pub requests: Arc<Mutex<Vec<Value>>>,
    pub receipt_version: Arc<AtomicU16>,
    coverage: Arc<
        Mutex<BTreeMap<[u8; 32], layerx_sdk::rpc_verification::node_commit::NodeCommitCoverage>>,
    >,
    clock: Arc<RuntimeClock>,
    workers: Vec<Worker>,
}

impl Peer {
    pub fn start() -> Self {
        let state: Value = checked(serde_json::from_str(include_str!(
            "../fixtures/funded_state.json"
        )));
        let payer_account = checked(derive_from_seed(
            &unhex(state["payer_seed"].as_str().expect("payer seed")),
            0,
        ));
        let recipient_account = checked(derive_from_seed(
            &unhex(state["recipient_seed"].as_str().expect("recipient seed")),
            0,
        ));
        let payer = LocalSigner::from_derived_account(&payer_account);
        let recipient = LocalSigner::from_derived_account(&recipient_account);
        let payer_did = format!(
            "did:layerx:{}",
            funded_inputs::hex_encode(&payer_account.layerx_public_key())
        );
        let recipient_did = format!(
            "did:layerx:{}",
            funded_inputs::hex_encode(&recipient_account.layerx_public_key())
        );
        let mut entropy = [0; 16];
        checked(getrandom::fill(&mut entropy));
        let root = std::env::temp_dir().join(format!(
            "pay2-funded-{}",
            funded_inputs::hex_encode(&entropy)
        ));
        checked(fs::DirBuilder::new().mode(0o700).create(&root));
        let clock_listener = checked(UnixListener::bind(root.join("clock.sock")));
        checked(fs::set_permissions(
            root.join("clock.sock"),
            fs::Permissions::from_mode(0o600),
        ));
        checked(clock_listener.set_nonblocking(true));
        let clock_worker = Worker::start(move |stop| {
            let start = Instant::now();
            while !stop.load(Ordering::Acquire) {
                let mut stream = match clock_listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("PAY2 clock accept: {error}"),
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(3))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(3))));
                let mut bytes = [0; clock_protocol::REQUEST_BYTES];
                checked(stream.read_exact(&mut bytes));
                let request = checked(clock_protocol::Request::decode(&bytes));
                thread::sleep(Duration::from_nanos(request.wait_nanoseconds));
                let reading = ClockReading {
                    generation: [0x69; 16],
                    unix_milliseconds: funded_inputs::now_ms(),
                    monotonic_nanoseconds: checked(u64::try_from(start.elapsed().as_nanos())),
                };
                checked(stream.write_all(&clock_protocol::response(request.counter, Ok(reading))));
            }
        });
        let clock = checked(RuntimeClock::connect(ClockBinding {
            socket: root.join("clock.sock"),
            process: std::process::id(),
            user: checked(fs::metadata(&root)).uid(),
        }));
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        checked(listener.set_nonblocking(true));
        let url = format!("http://{}", checked(listener.local_addr()));
        let sequencer = SigningKey::from_bytes(&[0x61; 32]);
        let sequencer_id = [0x62; 32];
        let public_key = sequencer.verifying_key().to_bytes();
        let cluster = funded_inputs::Cluster {
            root,
            treasury_seed: *payer_account.layerx_seed(),
            treasury_did: payer_did.clone(),
            asset: [3; 32],
            sequencer_id,
            sequencer_key: public_key,
            chain_id: 125,
            node_url: url,
        };
        let funding = super::scenario::funding::Funding {
            recipient_seed: *recipient_account.layerx_seed(),
            recipient_did: recipient_did.clone(),
        };
        let sender = payer_account.evm_address();
        let recipient_sender = recipient_account.evm_address();
        let bound_identities = BTreeMap::from([
            (payer_account.layerx_public_key(), sender),
            (recipient_account.layerx_public_key(), recipient_sender),
        ]);
        let source = checked(layerx_platform_core::main_account(&payer_did));
        let destination = checked(layerx_platform_core::main_account(&recipient_did));
        let source_sequence = state["source_sequence"].as_u64().expect("source sequence");
        let identity_nonce = state["identity_nonce"].as_u64().expect("identity nonce");
        let balance = state["payer_balance"]
            .as_str()
            .expect("funded balance")
            .parse::<u128>()
            .expect("balance integer");
        assert_ne!(source_sequence, identity_nonce);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let observed = Arc::clone(&requests);
        let receipt_version = Arc::new(AtomicU16::new(0));
        let served_version = Arc::clone(&receipt_version);
        let coverage = Arc::new(Mutex::new(BTreeMap::new()));
        let served_coverage = Arc::clone(&coverage);
        let rpc_worker = Worker::start(move |stop| {
            let mut submitted = BTreeMap::<[u8; 32], (Value, CommittedReceipt, u64)>::new();
            let mut nonces = BTreeMap::from([
                (sender, identity_nonce),
                (
                    recipient_sender,
                    state["recipient_identity_nonce"]
                        .as_u64()
                        .expect("recipient nonce"),
                ),
            ]);
            let mut sequences = BTreeMap::from([
                (source, source_sequence),
                (
                    destination,
                    state["recipient_sequence"]
                        .as_u64()
                        .expect("recipient sequence"),
                ),
            ]);
            let mut grants = BTreeMap::<[u8; 32], (layerx_crypto::payments::Grant, u128)>::new();
            let mut balances = BTreeMap::from([(source, balance), (destination, 0_u128)]);
            let mut previous_root = [0; 32];
            while !stop.load(Ordering::Acquire) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("PAY2 RPC accept: {error}"),
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(3))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(3))));
                let request = request(&mut stream);
                checked(observed.lock()).push(request.clone());
                let params = &request["params"];
                println!(
                    "PAY2 loopback poll method={} params={params}",
                    request["method"]
                );
                let answer = match request["method"].as_str().expect("RPC method") {
                    "eth_chainId" => {
                        assert_eq!(params, &json!([]));
                        json!("0x7d")
                    }
                    "eth_blockNumber" => {
                        assert_eq!(params, &json!([]));
                        let height = submitted
                            .values()
                            .map(|(_, committed, _)| committed.coverage.covering_height)
                            .max()
                            .unwrap_or(1);
                        json!(format!("{height:#x}"))
                    }
                    "eth_call" => {
                        assert_eq!(params.as_array().expect("binding params").len(), 2);
                        assert_eq!(params[1], "pending");
                        assert_eq!(
                            params[0]["to"],
                            "0x0000000000000000000000000000000000001004"
                        );
                        let data = unhex(params[0]["data"].as_str().expect("binding calldata"));
                        assert_eq!(data.len(), 36);
                        assert_eq!(
                            &data[..4],
                            &layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX
                        );
                        let did_key: [u8; 32] = checked(data[4..].try_into());
                        let address = bound_identities
                            .get(&did_key)
                            .expect("committed DID binding");
                        let mut word = [0; 32];
                        word[12..].copy_from_slice(address);
                        json!(hex(&word))
                    }
                    "px_getReceiptKeys" => {
                        assert_eq!(params, &json!([]));
                        json!({"keys":[{"keyId":hex(&sequencer_id), "publicKey":hex(&public_key),
                            "firstHeight":1, "lastHeight":u64::MAX}]})
                    }
                    "px_getAccount" => {
                        assert_eq!(params.as_array().expect("account params").len(), 1);
                        let account: [u8; 32] = checked(
                            unhex(&format!(
                                "0x{}",
                                params[0].as_str().expect("account selector")
                            ))
                            .try_into(),
                        );
                        let sequence = sequences.get(&account).expect("funded account");
                        json!({"account_id":funded_inputs::hex_encode(&account), "next_sequence":sequence.to_string()})
                    }
                    "eth_getTransactionCount" => {
                        assert_eq!(params[1], "pending");
                        let sender: [u8; 20] =
                            checked(unhex(params[0].as_str().expect("nonce address")).try_into());
                        json!(format!(
                            "{:#x}",
                            nonces.get(&sender).expect("funded identity")
                        ))
                    }
                    "eth_gasPrice" => {
                        assert_eq!(params, &json!([]));
                        json!("0x2")
                    }
                    "eth_maxPriorityFeePerGas" => {
                        assert_eq!(params, &json!([]));
                        json!("0x1")
                    }
                    "eth_estimateGas" => {
                        assert_eq!(params[1], "pending");
                        assert!([hex(&sender), hex(&recipient_sender)]
                            .iter()
                            .any(|address| params[0]["from"] == *address));
                        assert_eq!(
                            params[0]["to"],
                            "0x000000000000000000000000000000000000101a"
                        );
                        json!("0x186a0")
                    }
                    "eth_sendRawTransaction" => {
                        let raw = unhex(params[0].as_str().expect("signed transaction"));
                        let (call, signature) = checked(decode_signed(&raw));
                        let actual_sender = checked(recover_sender(&call, &signature));
                        assert_eq!(call.chain_id, 125);
                        let nonce = nonces
                            .get_mut(&actual_sender)
                            .expect("funded transaction signer");
                        assert_eq!(call.nonce, *nonce);
                        *nonce = nonce.checked_add(1).expect("next identity nonce");
                        let disclosure = checked(bind_call(&call, actual_sender));
                        let method = disclosure
                            .construct_method()
                            .expect("native call constructor")
                            .expect("ledger operation");
                        match &method {
                            ConstructMethod::LedgerSend { .. } => {
                                assert!(
                                    submitted.is_empty(),
                                    "the scenario must submit its Send only once"
                                );
                                assert_eq!(actual_sender, sender);
                                assert_eq!(
                                    checked(disclosure.payload_sequence()),
                                    Some(source_sequence)
                                );
                                assert_eq!(disclosure.counterparties[0].account, source);
                                assert_eq!(disclosure.counterparties[1].account, destination);
                                let amount: u128 =
                                    disclosure.amounts.iter().map(|amount| amount.value).sum();
                                assert_eq!(amount, 3);
                                let balance = balances.get_mut(&source).expect("payer balance");
                                *balance = balance
                                    .checked_sub(amount)
                                    .expect("committed funding fixture must cover the Send");
                                *balances.get_mut(&destination).expect("recipient balance") +=
                                    amount;
                                *sequences.get_mut(&source).expect("payer sequence") += 1;
                            }
                            ConstructMethod::LedgerIssueGrant { grant } => {
                                assert_eq!(actual_sender, sender);
                                assert_eq!(grant.from, source);
                                assert_eq!(grant.recipient, destination);
                                assert_eq!(grant.asset, [3; 32]);
                                assert_eq!(grant.id, checked(grant.signing_digest()));
                                checked(layerx_crypto::ed25519::verify_digest(
                                    &grant.public_key,
                                    &grant.signature,
                                    &grant.id,
                                ));
                                assert!(
                                    grants.insert(grant.id, (grant.clone(), 0)).is_none(),
                                    "distinct funded grants"
                                );
                            }
                            ConstructMethod::LedgerDraw {
                                recipient,
                                purpose_hash,
                                amount,
                                grant,
                                receiver_sequence,
                                ..
                            } => {
                                assert_eq!(actual_sender, recipient_sender);
                                assert_eq!(*recipient, destination);
                                assert_eq!(*receiver_sequence, sequences[&destination]);
                                let (terms, used) =
                                    grants.get_mut(grant).expect("previously issued grant");
                                assert_eq!(*purpose_hash, terms.purpose_hash);
                                assert!(*amount <= terms.per_draw_maximum);
                                *used = used.checked_add(*amount).expect("grant usage");
                                assert!(*used <= terms.allowance);
                                let balance = balances.get_mut(&source).expect("payer balance");
                                *balance = balance.checked_sub(*amount).expect("funded draw");
                                *balances.get_mut(&destination).expect("recipient balance") +=
                                    *amount;
                                *sequences.get_mut(&destination).expect("recipient sequence") += 1;
                            }
                            method => panic!("unrecorded funded ledger operation: {method:?}"),
                        }
                        let hash = transaction_hash(&raw);
                        let sequence = checked(u64::try_from(submitted.len()))
                            .checked_mul(2)
                            .expect("activity and maintenance sequences")
                            .checked_add(1)
                            .expect("next receipt sequence");
                        let answer = execution_receipt(
                            &raw,
                            &sequencer,
                            sequence,
                            previous_root,
                            Some(&method),
                            &disclosure,
                            &grants,
                        );
                        let committed = commit_receipt(&answer, &raw, &sequencer);
                        previous_root = checked(receipt::decode(&unhex(
                            answer["receipt"].as_str().expect("receipt bytes"),
                        )))
                        .protocol()
                        .expect("protocol receipt")
                        .resulting_state_root();
                        assert!(checked(served_coverage.lock())
                            .insert(hash, committed.coverage.clone())
                            .is_none());
                        assert!(
                            submitted.insert(hash, (answer, committed, 0)).is_none(),
                            "exactly one submission per activity"
                        );
                        json!(hex(&hash))
                    }
                    "pax_sequencerReceipt" => {
                        let hash: [u8; 32] = checked(
                            unhex(params[0].as_str().expect("receipt identity")).try_into(),
                        );
                        assert_eq!(params, &json!([hex(&hash)]));
                        let (executed, committed, receipt_reads) = submitted
                            .get_mut(&hash)
                            .expect("real signed submission before receipt");
                        let answer = if *receipt_reads == 0 {
                            executed
                        } else {
                            &committed.receipt
                        };
                        *receipt_reads += 1;
                        let canonical = unhex(answer["receipt"].as_str().expect("receipt bytes"));
                        let decoded = checked(receipt::decode(&canonical));
                        served_version.store(
                            decoded
                                .protocol()
                                .expect("protocol receipt")
                                .protocol_version(),
                            Ordering::Release,
                        );
                        answer.clone()
                    }
                    "px_getCommit" => {
                        assert_eq!(params.as_array().expect("commit params").len(), 1);
                        let height = params[0].as_str().expect("commit height");
                        submitted
                            .values()
                            .map(|(_, committed, _)| committed)
                            .find(|committed| committed.commit["height"] == height)
                            .expect("recorded covering height")
                            .commit
                            .clone()
                    }
                    "px_getValidators" => {
                        assert_eq!(params.as_array().expect("validator params").len(), 1);
                        let height = params[0].as_str().expect("validator height");
                        assert!(submitted
                            .values()
                            .any(|(_, committed, _)| committed.commit["height"] == height));
                        let validators: Vec<_> = (1..=3).map(|n| {
                            let public = SigningKey::from_bytes(&[n; 32]).verifying_key().to_bytes();
                            let address = layerx_agentd::read::CommitValidator::address_for_key(&public);
                            json!({
                                "address":funded_inputs::hex_encode(&address),
                                "pubKey":{"type":"tendermint/PubKeyEd25519", "value":base64::engine::general_purpose::STANDARD.encode(public)},
                                "votingPower":"1",
                            })
                        }).collect();
                        json!({"height":height, "total":validators.len().to_string(), "validators":validators})
                    }
                    "px_getSequenceBatch" => {
                        assert_eq!(params.as_array().expect("batch params").len(), 1);
                        let height = params[0].as_str().expect("batch height");
                        submitted
                            .values()
                            .map(|(_, committed, _)| committed)
                            .find(|committed| committed.sequence_batch["height"] == height)
                            .expect("recorded sequence batch height")
                            .sequence_batch
                            .clone()
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(params.as_array().expect("block params").len(), 2);
                        let full = params[1].as_bool().expect("full transaction selector");
                        let number = params[0].as_str().expect("block number");
                        let committed = submitted
                            .values()
                            .map(|(_, committed, _)| committed)
                            .find(|committed| committed.block["number"] == number)
                            .expect("recorded covering block");
                        let mut block = committed.block.clone();
                        if full {
                            block["transactions"] = committed.full_transactions.clone();
                        }
                        block
                    }
                    method => {
                        panic!("PAY2 native receipt workflow requested unrecorded RPC {method}")
                    }
                };
                println!(
                    "PAY2 loopback served method={} rung={}",
                    request["method"],
                    answer.get("rung").and_then(Value::as_str).unwrap_or("read")
                );
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":answer}).to_string();
                checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            }
        });
        Self {
            cluster,
            funding,
            payer,
            recipient,
            requests,
            receipt_version,
            coverage,
            clock,
            workers: vec![rpc_worker, clock_worker],
        }
    }

    pub fn rpc(&self) -> RpcClient {
        checked(RpcClient::connect(&self.cluster.node_url, None)).with_clock(self.clock.clone())
    }

    pub fn committed_rpc(
        &self,
        activity: [u8; 32],
    ) -> Result<RpcClient, layerx_sdk::rpc::RpcError> {
        let pinned = self.identity_authority();
        let recorded = checked(self.coverage.lock())
            .get(&activity)
            .expect("submitted activity commitment coverage")
            .clone();
        funded_inputs::write(
            &self.cluster.root.join(format!(
                "commit-coverage-{}.json",
                funded_inputs::hex_encode(&activity)
            )),
            &checked(serde_json::to_vec(&json!({
                "covering_height":recorded.covering_height,
                "batch_transaction":funded_inputs::hex_encode(&recorded.batch_transaction),
                "transaction_proof":{
                    "total":recorded.transaction_proof.total,
                    "index":recorded.transaction_proof.index,
                    "aunts":recorded.transaction_proof.aunts.iter().map(|aunt| funded_inputs::hex_encode(aunt)).collect::<Vec<_>>(),
                },
                "activity_ids":recorded.activity_ids.iter().map(|id| funded_inputs::hex_encode(id)).collect::<Vec<_>>(),
            }))),
            0o600,
        );
        let coverage = funded_inputs::read_commit_coverage(&self.cluster.root, activity)?;
        let authority = layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority::new(
            pinned.chain_id,
            pinned
                .validators
                .into_iter()
                .map(|v| (v.public_key, v.voting_power))
                .collect(),
            coverage.covering_height,
            coverage.covering_height,
        )?;
        self.rpc().with_node_commit_authority(authority)
    }

    pub fn finish(&mut self) {
        for worker in &self.workers {
            worker.stop.store(true, Ordering::Release);
        }
        for worker in &mut self.workers {
            worker.join();
        }
    }

    pub fn identity_authority(&self) -> sequence::Pay2CommittedAuthority {
        sequence::Pay2CommittedAuthority {
            node: checked(layerx_client::sequencer::SequencerEndpoint::new(
                &self.cluster.node_url,
                &[],
            )),
            chain_id: "125".to_owned(),
            validators: (1..=3)
                .map(|n| {
                    let key = SigningKey::from_bytes(&[n; 32]).verifying_key().to_bytes();
                    layerx_agentd::read::CommitValidator {
                        address: layerx_agentd::read::CommitValidator::address_for_key(&key),
                        public_key: key,
                        voting_power: 1,
                    }
                })
                .collect(),
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.workers.clear();
        checked(fs::remove_dir_all(&self.cluster.root));
    }
}

fn request(stream: &mut TcpStream) -> Value {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        assert!(header.len() < 16_384);
        let mut byte = [0];
        checked(stream.read_exact(&mut byte));
        header.push(byte[0]);
    }
    let length = checked(std::str::from_utf8(&header))
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| checked(value.trim().parse::<usize>()))
        })
        .expect("content length");
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    checked(stream.read_exact(&mut body));
    checked(serde_json::from_slice(&body))
}

fn ledger_event(name: &str, fields: &[(&str, [u8; 32])]) -> SequencerLog {
    let abi: Value = checked(serde_json::from_str(include_str!(
        "../../../../../../precompiles/layerxledger/abi.json"
    )));
    let event = abi
        .as_array()
        .expect("ledger ABI")
        .iter()
        .find(|entry| entry["type"] == "event" && entry["name"] == name)
        .expect("declared ledger event");
    assert_eq!(event["anonymous"], false);
    let inputs = event["inputs"].as_array().expect("event inputs");
    assert_eq!(inputs.len(), fields.len());
    let signature = format!(
        "{name}({})",
        inputs
            .iter()
            .map(|input| input["type"].as_str().expect("ABI type"))
            .collect::<Vec<_>>()
            .join(",")
    );
    let mut topics = vec![transaction_hash(signature.as_bytes())];
    let mut data = Vec::new();
    for (input, (name, word)) in inputs.iter().zip(fields) {
        assert_eq!(input["name"], *name);
        assert!(matches!(
            input["type"].as_str(),
            Some("bytes32" | "uint128")
        ));
        if input["type"] == "uint128" {
            assert_eq!(&word[..16], &[0; 16]);
        }
        if input["indexed"] == true {
            topics.push(*word);
        } else {
            data.extend_from_slice(word);
        }
    }
    let mut address = [0; 20];
    address[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    SequencerLog::new(address, topics, data, 0)
}

fn execution_receipt(
    raw: &[u8],
    signer: &SigningKey,
    sequence: u64,
    previous_root: [u8; 32],
    method: Option<&ConstructMethod>,
    disclosure: &Disclosure,
    grants: &BTreeMap<[u8; 32], (layerx_crypto::payments::Grant, u128)>,
) -> Value {
    let (call, signature) = checked(decode_signed(raw));
    let sender = checked(recover_sender(&call, &signature));
    let hash = transaction_hash(raw);
    let timestamp = funded_inputs::now_ms();
    let gas = call.gas_limit;
    let price = 5_u64;
    let fee = u128::from(gas) * u128::from(price);
    let empty_digest = funded_inputs::sha256(&[b""]);
    let party = |role| {
        disclosure
            .counterparties
            .iter()
            .find(|party| party.role == role)
            .map(|party| party.account)
    };
    let logs = match method {
        Some(ConstructMethod::LedgerIssueGrant { grant }) => {
            assert_eq!(
                grant.from,
                party(CounterpartyRole::Payer).expect("grant payer disclosure")
            );
            assert_eq!(
                grant.recipient,
                party(CounterpartyRole::Recipient).expect("grant recipient disclosure")
            );
            vec![ledger_event("PayerGrantIssued", &[("grantId", grant.id)])]
        }
        Some(ConstructMethod::LedgerDraw {
            grant,
            amount,
            idempotency_key,
            context_hash,
            ..
        }) => {
            let (terms, _) = grants.get(grant).expect("verified issued grant");
            assert_eq!(
                terms.recipient,
                party(CounterpartyRole::Recipient).expect("draw recipient disclosure")
            );
            let mut amount_word = [0; 32];
            amount_word[16..].copy_from_slice(&amount.to_be_bytes());
            vec![ledger_event(
                "PaymentDrawn",
                &[
                    ("grantId", *grant),
                    ("from", terms.from),
                    ("to", terms.recipient),
                    ("assetId", terms.asset),
                    ("amount", amount_word),
                    ("idempotencyKey", *idempotency_key),
                    ("contextHash", *context_hash),
                ],
            )]
        }
        Some(ConstructMethod::LedgerSend { .. }) | None => Vec::new(),
        Some(method) => panic!("unrecorded receipt constructor: {method:?}"),
    };
    let mut evm_from = [0; 32];
    evm_from[12..].copy_from_slice(&sender);
    let mut evm_to = [0; 32];
    evm_to[12..].copy_from_slice(&call.to);
    let module = 12;
    let operation = raw[0];
    // decode_signed accepts only canonical type-2 calls with zero transaction value.
    let amount = 0_u128;
    assert_eq!(operation, 2);
    let asset = funded_inputs::sha256(&[b"uhpx"]);
    let from = evm_from;
    let to = evm_to;
    let mut summary = Encoder::new(512);
    checked(summary.u64(gas));
    checked(summary.u64(gas));
    checked(summary.u64(price));
    checked(summary.u32(0));
    checked(summary.fixed(&[0; 20]));
    checked(summary.fixed(&[0; 20]));
    checked(summary.fixed(&empty_digest));
    checked(summary.u32(0));
    checked(summary.u32(checked(u32::try_from(logs.len()))));
    checked(summary.u8(4));
    checked(summary.fixed(b"uhpx"));
    let summary = summary.finish();
    let mut e = Encoder::new(layerx_wire::limits::MAX_MESSAGE_BYTES);
    let version = layerx_wire::limits::PROTOCOL_VERSION;
    checked(e.structure_header_version(0x5201, version));
    checked(e.u16(version));
    checked(e.bytes(&hash, 32));
    checked(e.u64(sequence));
    checked(e.bytes(&previous_root, 32));
    checked(e.bytes(
        &funded_inputs::sha256(&[
            b"PAXEER/v1/sequence\0",
            &previous_root,
            &sequence.to_be_bytes(),
            &hash,
        ]),
        32,
    ));
    checked(e.bytes(&empty_digest, 32));
    checked(e.i32(0));
    checked(e.sequence_length(1 + logs.len(), 512));
    checked(e.u16(module));
    checked(e.u16(0));
    checked(e.u16(0));
    checked(e.u8(1));
    checked(e.u8(0));
    checked(e.bytes(&[0; 32], 32));
    checked(e.bytes(&summary, 65536));
    for (index, log) in logs.iter().enumerate() {
        let mut body = Encoder::new(256);
        checked(body.fixed(&log.address()));
        checked(body.u8(checked(u8::try_from(log.topics().len()))));
        for topic in log.topics() {
            checked(body.fixed(topic));
        }
        checked(body.fixed(&funded_inputs::sha256(&[log.data()])));
        checked(e.u16(module));
        checked(e.u16(checked(u16::try_from(index + 1))));
        checked(e.u16(0));
        checked(e.u8(3));
        checked(e.u8(0));
        checked(e.bytes(&[0; 32], 32));
        checked(e.bytes(&body.finish(), 65536));
    }
    checked(e.u128(fee));
    let mut height = [0; 32];
    height[24..].copy_from_slice(&sequence.to_be_bytes());
    checked(e.bytes(&height, 32));
    checked(e.u16(module));
    checked(e.u32(1));
    checked(e.u32(0));
    checked(e.u8(operation));
    checked(e.bytes(&asset, 32));
    checked(e.u128(amount));
    checked(e.bytes(&from, 32));
    checked(e.u128(1_000_000_000));
    checked(
        e.u128(
            1_000_000_000_u128
                .checked_sub(amount)
                .and_then(|balance| balance.checked_sub(fee))
                .expect("funded EVM value and fee"),
        ),
    );
    checked(e.u64(call.nonce));
    checked(e.bytes(&to, 32));
    checked(e.u128(0));
    checked(e.u128(amount));
    checked(e.bytes(&[0; 32], 32));
    checked(e.bytes(&[0; 32], 32));
    checked(e.bytes(&[0x62; 32], 32));
    checked(e.u64(timestamp));
    checked(e.u8(0));
    let decoded = checked(receipt::decode(&e.finish()));
    let unsigned = checked(receipt::encode_unsigned(&decoded));
    let digest = checked(layerx_wire::hash::receipt_digest(&unsigned));
    let signature = signer.sign(&digest);
    let mut e = Encoder::new(layerx_wire::limits::MAX_MESSAGE_BYTES);
    checked(e.fixed(&unsigned[..unsigned.len() - 1]));
    checked(e.u8(1));
    checked(e.bytes(&signature.to_bytes(), 64));
    let canonical = checked(receipt::encode(&checked(receipt::decode(&e.finish()))));
    let response = json!({
        "rung":"receipted", "txHash":hex(&hash), "nonce":format!("{:#x}", call.nonce),
        "seq":format!("{sequence:#x}"), "height":format!("{sequence:#x}"), "index":"0x0", "timestamp":format!("{timestamp:#x}"),
        "status":"0x1", "returnData":"0x", "logs":logs.iter().map(|log| json!({
            "address":hex(&log.address()), "topics":log.topics().iter().map(|topic| hex(topic)).collect::<Vec<_>>(),
            "data":hex(log.data()), "logIndex":format!("{:#x}", log.log_index()),
        })).collect::<Vec<_>>(),
        "gasUsed":format!("{gas:#x}"), "cumulativeGasUsed":format!("{gas:#x}"),
        "effectiveGasPrice":format!("{price:#x}"), "paymentDenom":"uhpx", "feeDenom":"uhpx",
        "feeAmount":format!("{fee:#x}"), "sponsor":hex(&[0;20]),
        "from":hex(&sender), "to":hex(&call.to), "contractAddress":hex(&[0;20]), "vmError":"",
        "keyId":hex(&[0x62;32]), "signingKey":hex(&signer.verifying_key().to_bytes()),
        "receipt":hex(&canonical),
    });
    let vectors: Value = checked(serde_json::from_str(include_str!(
        "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        vectors["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    assert_eq!(
        response
            .as_object()
            .expect("peer response")
            .keys()
            .collect::<Vec<_>>(),
        vectors["cases"][0]["receiptJson"]
            .as_object()
            .expect("recorded node response")
            .keys()
            .collect::<Vec<_>>(),
        "the peer must retain the recorded production node receipt schema",
    );
    let policy = layerx_sdk::rpc_verification::ReceiptPolicy {
        protocol_version: version,
        network_id: 125,
        sequencer: layerx_proof::inclusion::SequencerAuthorization::new(
            [0x62; 32],
            signer.verifying_key().to_bytes(),
            1,
            u64::MAX,
        ),
        trusted_checkpoint_context_digest: None,
    };
    let authenticated = checked(layerx_proof::receipt::verify_sequencer_signature(
        &canonical,
        layerx_proof::receipt::ReceiptKeys::HeightRanges(std::slice::from_ref(&policy.sequencer)),
    ));
    let protocol = authenticated.protocol().expect("signed protocol receipt");
    assert_eq!(protocol.module_id(), module);
    assert_eq!(protocol.operation(), operation);
    assert_eq!(protocol.asset(), asset);
    assert_eq!(protocol.amount(), amount);
    assert_eq!(protocol.from(), from);
    assert_eq!(protocol.to(), to);
    println!(
        "PAY2 loopback peer authenticated receipt module={module} operation={operation} amount={amount} from={} to={} asset={}",
        hex(&from), hex(&to), hex(&asset)
    );
    let (_, verified) = layerx_sdk::rpc_verification::verify_node_receipt(&response, hash, &policy)
        .unwrap_or_else(|error| {
            eprintln!(
                "PAY2 node receipt refusal module={module} operation={operation}: summary/log verification={:?}",
                layerx_proof::sequencer_logs::verify_logs(&authenticated, logs.clone())
            );
            super::record_send_failure("peer_receipt", &error, version, policy.protocol_version)
        });
    assert_eq!(
        verified, canonical,
        "the peer must emit a current SDK-verifiable execution receipt"
    );
    println!("PAY2 loopback peer independently verified current EVM receipt v{version}");
    response
}

struct CommittedReceipt {
    receipt: Value,
    commit: Value,
    block: Value,
    sequence_batch: Value,
    full_transactions: Value,
    coverage: layerx_sdk::rpc_verification::node_commit::NodeCommitCoverage,
}

fn occupancy_maintenance(protocol: &receipt::ProtocolReceipt, batch_number: u64) -> Vec<u8> {
    use layerx_wire::hash::Domain;

    let schedule_version = 1_u32;
    let schedule_prices = [1_u64; 7];
    let occupancy_asset_id = [3; 32];
    let mut schedule = Encoder::new(92);
    checked(schedule.u32(schedule_version));
    for price in schedule_prices {
        checked(schedule.u64(price));
    }
    checked(schedule.fixed(&occupancy_asset_id));
    let schedule_commitment =
        funded_inputs::sha256(&[Domain::ContextHash.tag(), &schedule.finish()]);
    let settlement_evidence = b"PAY2 occupancy settlement: no occupied programs";
    let mut encoder = Encoder::new(1024);
    checked(encoder.fixed(b"LXP/programs/occupancy-receipt/v2\0"));
    checked(encoder.u64(batch_number));
    checked(
        encoder.u64(
            protocol
                .global_sequence()
                .checked_add(1)
                .expect("maintenance sequence"),
        ),
    );
    checked(encoder.u32(1));
    checked(encoder.u32(schedule_version));
    for price in schedule_prices {
        checked(encoder.u64(price));
    }
    checked(encoder.fixed(&occupancy_asset_id));
    for _ in 0..4 {
        checked(encoder.u128(0));
    }
    checked(encoder.u16(0));
    checked(encoder.fixed(&schedule_commitment));
    checked(encoder.bytes(settlement_evidence, 65_536));
    checked(encoder.fixed(&funded_inputs::sha256(&[settlement_evidence])));
    checked(encoder.fixed(&protocol.resulting_state_root()));
    checked(encoder.fixed(&[0; 32]));
    checked(encoder.fixed(&protocol.previous_state_root()));
    checked(encoder.fixed(&protocol.resulting_state_root()));
    encoder.finish()
}

fn commit_receipt(executed: &Value, raw: &[u8], signer: &SigningKey) -> CommittedReceipt {
    use layerx_client::rpc::receipt_commit::{
        verify_receipt_coverage, ReceiptCommitEvidence, SequenceBatch,
    };
    use layerx_proof::{
        inclusion::{verify_receipt, SequencerAuthorization},
        merkle::build_proof,
    };

    let canonical = unhex(executed["receipt"].as_str().expect("execution receipt"));
    let decoded = checked(receipt::decode(&canonical));
    let protocol = decoded.protocol().expect("protocol receipt");
    assert_eq!(protocol.activity_id(), transaction_hash(raw));
    let height = protocol
        .global_sequence()
        .checked_add(1)
        .expect("covering height");
    assert_eq!(protocol.global_sequence() % 2, 1);
    let batch_number = height / 2;
    let maintenance = occupancy_maintenance(protocol, batch_number);
    let (proof, receipt_root) = checked(build_proof(&[&canonical, &maintenance], 0));
    assert_eq!(proof.leaf_count(), 2);
    assert_eq!(proof.leaf_index(), 0);
    let (_, activity_root) = checked(build_proof(&[raw], 0));
    let authorization =
        SequencerAuthorization::new([0x62; 32], signer.verifying_key().to_bytes(), 1, u64::MAX);
    let mut encoder = Encoder::new(1024);
    checked(encoder.structure_header_version(0x1701, funded_inputs::PROTOCOL_VERSION));
    checked(encoder.u8(15));
    for field in 1..=15 {
        checked(encoder.tag(field, 15));
        match field {
            1 => checked(encoder.u16(funded_inputs::PROTOCOL_VERSION)),
            2 => checked(encoder.u32(funded_inputs::NETWORK_ID)),
            3 => checked(encoder.u64(1)),
            4 => checked(encoder.u64(batch_number)),
            5 => checked(encoder.u64(protocol.global_sequence())),
            6 => checked(encoder.u64(height)),
            7 => checked(encoder.bytes(&protocol.previous_state_root(), 32)),
            8 => checked(encoder.bytes(&protocol.resulting_state_root(), 32)),
            9 => checked(encoder.bytes(&activity_root, 32)),
            10 => checked(encoder.bytes(&receipt_root, 32)),
            14 => checked(encoder.u64(protocol.timestamp())),
            15 => checked(encoder.bytes(&authorization.sequencer_id(), 32)),
            _ => checked(encoder.bytes(&[0; 32], 32)),
        }
    }
    let batch_header = encoder.finish();
    let decoded_header = checked(receipt::decode_batch_header(&batch_header));
    let decoded_maintenance = checked(layerx_wire::batch_maintenance::decode_maintenance(
        &maintenance,
    ));
    checked(decoded_maintenance.verify_header(&decoded_header));
    assert_eq!(
        decoded_maintenance.occupancy().previous_state_root,
        decoded_header.previous_state_root(),
    );
    let header_signature = signer
        .sign(&checked(layerx_wire::hash::batch_header_digest(
            &batch_header,
        )))
        .to_bytes();
    checked(verify_receipt(
        &canonical,
        &proof,
        &batch_header,
        &header_signature,
        &authorization,
    ));

    let mut batch = SequenceBatch {
        height,
        from_sequence: protocol.global_sequence(),
        to_sequence: protocol.global_sequence(),
        previous_root: protocol.previous_state_root(),
        resulting_root: protocol.resulting_state_root(),
        activity_root: funded_inputs::sha256(&[&protocol.activity_id()]),
        key_id: authorization.sequencer_id(),
        signature: [0; 64],
    };
    batch.signature = signer
        .sign(&checked(batch.signing_digest("125")))
        .to_bytes();
    let transaction = checked(batch.encode());
    let leaf_hash = funded_inputs::sha256(&[&[0], &funded_inputs::sha256(&[&transaction])]);
    let transaction_proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash,
        aunts: Vec::new(),
    };
    let keys: Vec<_> = (1..=3).map(|n| SigningKey::from_bytes(&[n; 32])).collect();
    let validators = checked(ValidatorSet::new(
        keys.iter()
            .map(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1))
            .collect(),
    ));
    let timestamp = funded_inputs::now_ms();
    let seconds = checked(i64::try_from(timestamp / 1000));
    let nanos = checked(i32::try_from(timestamp % 1000 * 1_000_000));
    let time = utc_timestamp(timestamp);
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 0 },
        chain_id: "125".to_owned(),
        height: checked(i64::try_from(height)),
        time: HeaderTime { seconds, nanos },
        last_block_id: HeaderBlockId {
            hash: protocol.batch_id().to_vec(),
            part_set_total: 1,
            part_set_hash: activity_root.to_vec(),
        },
        last_commit_hash: funded_inputs::sha256(&[b"PAY2 prior commit"]).to_vec(),
        data_hash: leaf_hash.to_vec(),
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: funded_inputs::sha256(&[b"PAY2 consensus parameters"]).to_vec(),
        app_hash: protocol.resulting_state_root().to_vec(),
        last_results_hash: funded_inputs::sha256(&[b"PAY2 prior results"]).to_vec(),
        evidence_hash: funded_inputs::sha256(&[b""]).to_vec(),
        proposer_address: validators.validators()[0].address.to_vec(),
    };
    let block_hash = checked(header.hash());
    let parts_hash = funded_inputs::sha256(&[&transaction]);
    let block_id = BlockId {
        hash: block_hash.to_vec(),
        part_set_header: PartSetHeader {
            total: 1,
            hash: parts_hash.to_vec(),
        },
    };
    let vote = checked(vote_sign_bytes(
        "125",
        checked(i64::try_from(height)),
        0,
        Some(&block_id),
        Timestamp { seconds, nanos },
    ));
    let signatures: Vec<_> = keys
        .iter()
        .zip(validators.validators())
        .map(|(key, validator)| {
            json!({
                "block_id_flag":2, "validator_address":funded_inputs::hex_encode(&validator.address),
                "timestamp":time,
                "signature":base64::engine::general_purpose::STANDARD.encode(key.sign(&vote).to_bytes()),
            })
        })
        .collect();
    let signed_header = json!({
        "header":{
            "version":{"block":"11", "app":"0"}, "chain_id":"125", "height":height.to_string(),
            "time":time,
            "last_block_id":{"hash":funded_inputs::hex_encode(&header.last_block_id.hash),
                "parts":{"total":1,"hash":funded_inputs::hex_encode(&header.last_block_id.part_set_hash)}},
            "last_commit_hash":funded_inputs::hex_encode(&header.last_commit_hash), "data_hash":funded_inputs::hex_encode(&header.data_hash),
            "validators_hash":funded_inputs::hex_encode(&header.validators_hash),
            "next_validators_hash":funded_inputs::hex_encode(&header.next_validators_hash),
            "consensus_hash":funded_inputs::hex_encode(&header.consensus_hash), "app_hash":funded_inputs::hex_encode(&header.app_hash),
            "last_results_hash":funded_inputs::hex_encode(&header.last_results_hash),
            "evidence_hash":funded_inputs::hex_encode(&header.evidence_hash), "proposer_address":funded_inputs::hex_encode(&header.proposer_address),
        },
        "commit":{
            "height":height.to_string(), "round":0,
            "block_id":{"hash":funded_inputs::hex_encode(&block_hash), "parts":{"total":1,"hash":funded_inputs::hex_encode(&parts_hash)}},
            "signatures":signatures,
        },
    });
    let verifier = ChainProofVerifier {
        chain_id: "125".to_owned(),
        validators,
    };
    let head = checked(layerx_client::rpc::commit::verify_commit_material(
        &signed_header,
        &verifier,
    ));
    assert_eq!(head.header_hash(), block_hash);
    let identities = [protocol.activity_id()];
    checked(verify_receipt_coverage(
        protocol,
        protocol.global_sequence(),
        &head,
        &ReceiptCommitEvidence {
            verifier: &verifier,
            covering_height: height,
            batch_transaction: &transaction,
            transaction_proof: &transaction_proof,
            activity_ids: &identities,
        },
        &[layerx_client::sequencer::ReceiptKeyEntry {
            key_id: authorization.sequencer_id(),
            public_key: authorization.public_key(),
            first_height: 1,
            last_height: u64::MAX,
        }],
    ));
    let mut response = executed.clone();
    response["rung"] = json!("committed");
    response["blockNumber"] = json!(format!("{height:#x}"));
    response["blockHash"] = json!(hex(&block_hash));
    response["inclusion"] = json!({
        "kind":"receipt", "activity_id":funded_inputs::hex_encode(&protocol.activity_id()),
        "canonical_value":funded_inputs::hex_encode(&canonical),
        "proof":{"leaf_index":proof.leaf_index(), "leaf_count":proof.leaf_count(),
            "siblings":proof.siblings().iter().map(|s| funded_inputs::hex_encode(s)).collect::<Vec<_>>()},
        "signed_header":{"canonical_header":funded_inputs::hex_encode(&batch_header),
            "signature":funded_inputs::hex_encode(&header_signature)},
    });
    println!("PAY2 loopback peer independently verified receipt inclusion, Comet quorum and signed sequence-batch coverage");
    let (call, signature) = checked(decode_signed(raw));
    let quantity = |bytes: &[u8]| {
        let text = funded_inputs::hex_encode(bytes);
        let digits = text.trim_start_matches('0');
        assert!(!digits.is_empty(), "nonzero signature scalar");
        format!("0x{digits}")
    };
    let parity = signature.as_bytes()[64] - 27;
    let full_transactions = json!([{
        "blockHash":hex(&block_hash), "blockNumber":format!("{height:#x}"),
        "from":hex(&checked(recover_sender(&call, &signature))),
        "gas":format!("{:#x}", call.gas_limit),
        "gasPrice":format!("{:#x}", call.max_fee_per_gas),
        "hash":hex(&transaction_hash(raw)), "input":hex(&call.data),
        "nonce":format!("{:#x}", call.nonce), "to":hex(&call.to),
        "transactionIndex":"0x0", "value":"0x0", "type":"0x2",
        "v":format!("{parity:#x}"), "yParity":format!("{parity:#x}"),
        "r":quantity(&signature.as_bytes()[..32]),
        "s":quantity(&signature.as_bytes()[32..64]),
        "accessList":[], "chainId":format!("{:#x}", call.chain_id),
        "maxFeePerGas":format!("{:#x}", call.max_fee_per_gas),
        "maxPriorityFeePerGas":format!("{:#x}", call.max_priority_fee_per_gas),
    }]);
    CommittedReceipt {
        receipt: response,
        commit: json!({"height":height.to_string(), "signedHeader":signed_header}),
        block: json!({"number":format!("{height:#x}"), "hash":hex(&block_hash),
            "parentHash":hex(&header.last_block_id.hash),
            "timestamp":format!("{seconds:#x}"),
            "transactionsRoot":hex(&header.data_hash),
            "receiptsRoot":hex(&header.last_results_hash),
            "stateRoot":hex(&header.app_hash),
            "transactions":identities.iter().map(|id| hex(id)).collect::<Vec<_>>()}),
        full_transactions,
        sequence_batch: json!({
            "height":height.to_string(), "tx":hex(&transaction),
            "proof":{
                "total":transaction_proof.total, "index":transaction_proof.index,
                "leafHash":hex(&transaction_proof.leaf_hash),
                "aunts":transaction_proof.aunts.iter().map(|aunt| hex(aunt)).collect::<Vec<_>>(),
            },
        }),
        coverage: layerx_sdk::rpc_verification::node_commit::NodeCommitCoverage {
            covering_height: height,
            batch_transaction: transaction,
            transaction_proof: layerx_sdk::rpc_verification::node_commit::NodeTransactionProof {
                total: 1,
                index: 0,
                aunts: Vec::new(),
            },
            activity_ids: identities.to_vec(),
        },
    }
}

fn utc_timestamp(milliseconds: u64) -> String {
    let seconds = milliseconds / 1000;
    let days = checked(i64::try_from(seconds / 86_400)) + 719_468;
    let era = days / 146_097;
    let day_of_era = days - era * 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1460 + day_of_era / 36_524 - day_of_era / 146_096) / 365;
    let mut year = year_of_era + era * 400;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let month_base = (5 * day_of_year + 2) / 153;
    let day = day_of_year - (153 * month_base + 2) / 5 + 1;
    let month = month_base + if month_base < 10 { 3 } else { -9 };
    year += i64::from(month <= 2);
    let hour = seconds % 86_400 / 3600;
    let minute = seconds % 3600 / 60;
    let second = seconds % 60;
    format!(
        "{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}.{:03}Z",
        milliseconds % 1000
    )
}
