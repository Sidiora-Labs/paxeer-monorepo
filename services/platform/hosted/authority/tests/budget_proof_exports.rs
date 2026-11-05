include!("repair_22_33_1471.rs");

#[path = "../src/human/budget_state/record.rs"]
mod record;

fn protected(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = fs::symlink_metadata(path).expect("real corpus metadata");
    assert!(metadata.is_file() && metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    fs::read(path).expect("real protected corpus")
}

fn bytes(value: &Value) -> Result<Vec<u8>, ()> {
    let text = value.as_str().ok_or(())?;
    let decoded = hex::decode(text).map_err(|_| ())?;
    if hex::encode(&decoded) != text {
        return Err(());
    }
    Ok(decoded)
}

fn verify_export(document: &Value, expected_finality: &Value) -> Result<(), ()> {
    use record::{number, Record};
    if document["schema"] != "layerx.human.budget-proof.v2"
        || document["protocol_version"] != 3
        || document.get("lni_interface_version").is_some()
        || document.get("authorization").is_some()
        || document.get("maximum_bytes").is_some()
        || &document["finality"] != expected_finality
    {
        return Err(());
    }
    let state = &document["budget_state"];
    let proofs = &document["proofs"];
    if proofs.as_object().ok_or(())?.len() != 3 {
        return Err(());
    }
    let budget = bytes(&proofs["budget_call"]["result"])?;
    let remaining = bytes(&proofs["remaining_call"]["result"])?;
    let record = Record::from_call(
        bytes(&state["budget_id"])?.try_into().map_err(|_| ())?,
        &budget,
    )?;
    if bytes(&state["canonical_core_bytes"])? != budget
        || bytes(&state["asset"])? != record.asset
        || state["revocation_sequence"] != record.revocation
        || state["closed"] != record.closed
        || state["revoked"] != record.revoked
        || remaining.len() != 32
        || state["remaining"] != number(&remaining, 0, 16)?.to_string()
    {
        return Err(());
    }
    let head = &document["head"];
    let finality = &document["finality"];
    if head.as_object().ok_or(())?.len() != 2 || finality.as_object().ok_or(())?.len() != 3 {
        return Err(());
    }
    let committed = head["committed_height"].as_u64().ok_or(())?;
    let block_hash: [u8; 32] = bytes(&head["block_hash"])?.try_into().map_err(|_| ())?;
    let app_hash: [u8; 32] = bytes(&finality["app_hash"])?.try_into().map_err(|_| ())?;
    if committed == 0
        || finality["height"] != committed
        || block_hash == [0; 32]
        || app_hash == [0; 32]
        || bytes(&finality["block_hash"])? != block_hash
        || bytes(&state["checkpoint_digest"])? != block_hash
        || state["observed_head_sequence"] != committed
        || record.revocation == 0
        || record.revocation > committed
    {
        return Err(());
    }
    for (name, signature) in [
        ("budget_call", b"getBudget(bytes32)".as_slice()),
        ("remaining_call", b"budgetRemaining(bytes32)".as_slice()),
    ] {
        let call = &proofs[name];
        let mut data = layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
        data.extend_from_slice(&record.id);
        if call.as_object().ok_or(())?.len() != 4
            || call["height"] != committed
            || bytes(&call["to"])?
                != hex::decode("000000000000000000000000000000000000101b").map_err(|_| ())?
            || bytes(&call["data"])? != data
        {
            return Err(());
        }
    }
    for (name, id) in [
        ("owner", record.owner),
        ("budget", record.account),
        ("source", record.source.unwrap_or(record.owner)),
    ] {
        if bytes(&document["accounts"][name])? != id {
            return Err(());
        }
    }
    if !proofs["account_join"]["layerx"]["accounts"]
        .as_array()
        .ok_or(())?
        .iter()
        .any(|row| {
            row["bound"] == true
                && bytes(&row["layerx_account"]).is_ok_and(|id| id == record.owner)
                && row["evm_address"]
                    .as_str()
                    .and_then(|s| s.strip_prefix("0x"))
                    .and_then(|s| hex::decode(s).ok())
                    .is_some_and(|id| id.len() == 20 && id != [0; 20])
        })
    {
        return Err(());
    }
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(block_hash);
    let join = serde_json::to_vec(&proofs["account_join"]).map_err(|_| ())?;
    for material in [&budget, &remaining, &join] {
        evidence.update(u64::try_from(material.len()).map_err(|_| ())?.to_be_bytes());
        evidence.update(material);
    }
    let digest: [u8; 32] = evidence.finalize().into();
    if bytes(&state["evidence_digest"])? != digest
        || bytes(&state["receipt_digest"])? != Sha256::digest(&budget).as_slice()
    {
        return Err(());
    }
    Ok(())
}

#[test]
fn real_budget_exports_bind_each_selector_finality_and_tamper_refusals() {
    let corpus = produced_export_corpus();
    let fixture: Value =
        serde_json::from_slice(&protected(&corpus.path, 65_536)).expect("produced corpus JSON");
    assert_export_corpus(&fixture);
}

#[test]
fn budget_abi_matches_node_precompile_types() {
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    ))
    .expect("node ABI");
    let get = abi
        .as_array()
        .expect("ABI")
        .iter()
        .find(|row| row["name"] == "getBudget")
        .expect("getBudget");
    assert_eq!(get["inputs"][0]["type"], "bytes32");
    let fields = get["outputs"][0]["components"].as_array().expect("tuple");
    assert_eq!(fields.len(), 18);
    assert_eq!(fields[16]["type"], "bytes32[]");
    assert_eq!(fields[17]["name"], "sourceAccount");
}

struct BudgetReplay {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    calls: Arc<Mutex<Vec<Value>>>,
    worker: Option<thread::JoinHandle<()>>,
}

impl BudgetReplay {
    fn start(mut answers: Value) -> Self {
        let chain_id: u64 = answers
            .as_array()
            .expect("fixture answers")
            .iter()
            .find(|answer| answer["method"] == "px_getCommit")
            .expect("fixture signed header")["result"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .expect("fixture chain ID")
            .parse()
            .expect("numeric chain ID");
        answers
            .as_array_mut()
            .expect("fixture answers")
            .push(serde_json::json!({
                "method":"eth_chainId", "params":[], "result":format!("{chain_id:#x}")
            }));
        let listener = TcpListener::bind("127.0.0.1:0").expect("fixture RPC listener");
        listener.set_nonblocking(true).expect("nonblocking RPC");
        let address = listener.local_addr().expect("RPC address");
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::clone(&calls);
        let worker = thread::spawn(move || {
            while !stop.load(Ordering::SeqCst) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("RPC accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("read deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("write deadline");
                let mut headers = Vec::new();
                while !headers.ends_with(b"\r\n\r\n") {
                    assert!(headers.len() < 32_768, "bounded HTTP header");
                    let mut byte = [0];
                    stream.read_exact(&mut byte).expect("HTTP header");
                    headers.push(byte[0]);
                }
                let headers = String::from_utf8(headers).expect("HTTP text");
                let length: usize = headers
                    .lines()
                    .find_map(|line| {
                        let (name, value) = line.split_once(':')?;
                        name.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse().expect("HTTP length"))
                    })
                    .expect("content length");
                assert!(length <= 1_048_576, "bounded RPC request");
                let mut body = vec![0; length];
                stream.read_exact(&mut body).expect("RPC body");
                let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                let answer = answers
                    .as_array()
                    .expect("fixture answers")
                    .iter()
                    .find(|answer| {
                        answer["method"] == request["method"]
                            && answer["params"] == request["params"]
                    })
                    .unwrap_or_else(|| panic!("unrecorded RPC: {request}"));
                requests.lock().expect("calls").push(request.clone());
                let body = serde_json::json!({"jsonrpc":"2.0", "id":request["id"], "result":answer["result"]}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).expect("RPC response");
            }
        });
        Self {
            address,
            stopped,
            calls,
            worker: Some(worker),
        }
    }

    fn finish(&mut self) {
        self.stopped.store(true, Ordering::SeqCst);
        self.worker
            .take()
            .expect("RPC worker")
            .join()
            .expect("fixture RPC replay");
    }
}

impl Drop for BudgetReplay {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[test]
fn budget_export_from_the_fixture_answers_equals_the_committed_export() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("repository root");
    let vectors = root.join("tests/vectors/budget-proof-v2");
    let answers: Value = serde_json::from_slice(
        &fs::read(vectors.join("node-answers.json")).expect("committed RPC answers"),
    )
    .expect("fixture JSON");
    let rows = answers.as_array().expect("answers");
    let budget_answer = rows
        .iter()
        .find(|row| row["method"] == "eth_call")
        .expect("budget call");
    let calldata = budget_answer["params"][0]["data"]
        .as_str()
        .expect("budget calldata");
    let id = &calldata[10..];
    let budget = record::Record::from_call(
        hex::decode32(id).expect("budget id"),
        &hex::decode(
            budget_answer["result"]
                .as_str()
                .expect("budget result")
                .strip_prefix("0x")
                .expect("hex prefix"),
        )
        .expect("result bytes"),
    )
    .expect("fixture budget");
    let owner = rows
        .iter()
        .find(|row| row["method"] == "px_getAccount")
        .expect("owner answer")["params"][0]
        .as_str()
        .expect("owner DID");
    let block = &rows
        .iter()
        .find(|row| row["method"] == "eth_getBlockByNumber")
        .expect("committed block")["result"];
    let mut replay = BudgetReplay::start(answers.clone());
    let url = format!("http://{}", replay.address);
    let mut nonce = [0; 16];
    getrandom::fill(&mut nonce).expect("temporary file nonce");
    let keys = std::env::temp_dir().join(format!(
        "budget-export-{}-{}.json",
        std::process::id(),
        hex::encode(&nonce)
    ));
    use std::os::unix::fs::OpenOptionsExt as _;
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&keys)
        .expect("receipt key file");
    let key = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=";
    write!(
        file,
        "{}",
        serde_json::json!([{"keyId":key,"publicKey":key,"firstHeight":0,"lastHeight":u64::MAX}])
    )
    .expect("receipt key list");
    drop(file);
    let chain_receipts = ChainReceipts::load(&url, &keys).expect("receipt source configuration");
    fs::remove_file(keys).expect("remove temporary key list");
    let _ = rustls::crypto::ring::default_provider().install_default();
    let tls = ServerConfig::builder()
        .with_no_client_auth()
        .with_cert_resolver(Arc::new(rustls::server::ResolvesServerCertUsingSni::new()));
    let config = Config {
        listen: replay.address,
        tls: Arc::new(tls),
        tokens: Vec::new(),
        human: None,
        replica_address: replay.address,
        replica_host: replay.address.to_string(),
        replica_token: Zeroizing::new(String::new()),
        evidence_read_token: None,
        replica_id: [1; 32],
        chain_receipts,
        chain_rpc_url: url.clone(),
        protocol_network_id: 125,
        network_id: "fixture".to_owned(),
        wire_version: PROTOCOL_VERSION.to_string(),
        authorization: SequencerAuthorization::new([1; 32], [1; 32], 0, u64::MAX),
        sequencer_public_key: [1; 32],
    };
    let reference = serde_json::json!({"activity_id":hex::encode(&[1;32]), "receipt_digest":hex::encode(&[1;32])});
    let key_policy = serde_json::json!({"policy_revision":1,"required_delay_seconds":1,"maximum_delay_seconds":1,"effective_sequence":1,"evidence":reference});
    let policy = serde_json::json!({
        "tenant":"tenant", "principal":"operator", "account_id":hex::encode(&budget.owner), "asset_id":hex::encode(&budget.asset),
        "activities":[hex::encode(&[1;32])], "budgets":[id], "maximum_age_seconds":60, "maximum_age_sequences":2,
        "identities":[{"did":owner,"authorities":[{"kind":"primary_key","id":hex::encode(&[1;32])}], "revocation_sequence":1,"frozen":false,
            "evidence":reference,"capabilities":[],"rotation":key_policy,"recovery":key_policy}]
    });
    let timestamp_ms = u128::from(
        u64::from_str_radix(
            block["timestamp"]
                .as_str()
                .expect("block timestamp")
                .strip_prefix("0x")
                .expect("timestamp prefix"),
            16,
        )
        .expect("timestamp quantity"),
    ) * 1000;
    let response = config
        .fixture_budget_export(&policy, id, timestamp_ms)
        .unwrap_or_else(|response| {
            panic!(
                "export refused: {}",
                String::from_utf8_lossy(&response.body)
            )
        });
    assert_eq!(response.status, 200);
    let document: Value = serde_json::from_slice(&response.body).expect("export JSON");
    let height = u64::from_str_radix(
        block["number"]
            .as_str()
            .expect("height")
            .strip_prefix("0x")
            .expect("height prefix"),
        16,
    )
    .expect("height quantity");
    let finality = serde_json::json!({"height":height, "block_hash":block["hash"].as_str().expect("hash").strip_prefix("0x").expect("hash prefix"), "app_hash":block["stateRoot"].as_str().expect("root").strip_prefix("0x").expect("root prefix")});
    assert!(verify_export(&document, &finality).is_ok());
    assert!(document["proofs"].get("checkpoint_call").is_none());
    assert_eq!(document["budget_state"]["budget_id"], id);
    assert_eq!(document["accounts"]["owner"], hex::encode(&budget.owner));
    let boundary = config
        .fixture_budget_export(&policy, id, timestamp_ms + 60_000)
        .unwrap_or_else(|response| {
            panic!(
                "freshness boundary refused: {}",
                String::from_utf8_lossy(&response.body)
            )
        });
    assert_eq!(boundary.body, response.body);
    assert!(config
        .fixture_budget_export(&policy, id, timestamp_ms + 60_001)
        .is_err());
    assert!(config
        .fixture_budget_export(&policy, id, timestamp_ms - 1)
        .is_err());
    replay.finish();
    let calls = replay.calls.lock().expect("calls");
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        8
    );
    assert!(calls
        .iter()
        .filter(|call| call["method"] == "eth_call")
        .all(|call| call["params"][1] == format!("{height:#x}")));
    let path = vectors.join("export.json");
    if std::env::var("LAYERX_WRITE_VECTORS").as_deref() == Ok("1") {
        fs::write(&path, &response.body).expect("write authority export");
    }
    assert_eq!(
        fs::read(path).expect("committed authority export"),
        response.body
    );
}
