//! Task 22.31.163, audit 22.30.109.
//!
//! history-transfer-loss: history_transfer_loss_multiple_logs_overlap_and_sink_reopen.
//! solana-code-offset: solana_code_offset_finalized_capture_matches_deployed_code_pin,
//! solana_code_offset_rejects_invalid_metadata_and_changed_code.
//! Baseline: this test target did not exist in the starting tree. No baseline
//! gate was run: the task's explicit gate order forbids verification before
//! the complete implementation and requires verification through hf-done.
//! Qualification revision and private logs are recorded by hf-build/hf-done.
//! Previous attempt: both Solana cases passed; the history case failed before
//! opening any component because its protected case file was not supplied.
//!
//! Wiring request (domain 11): contracts/bridge/solana/admin/src/main.rs must
//! generate custody_program_data_hash over the full deployed allocation after
//! UpgradeableLoaderState::size_of_programdata_metadata(), including padding.
//! Wiring request (domain 11): services/interop/crates/layerx-migrate/OPERATIONS.md
//! and spec/layerx-merge/interfaces.kvx must document that same pin format.
//! The Ethereum peer transcript, authenticated v1 journal seed and public
//! test-only TLS material are committed under tests/fixtures/repair_22_31_163.
//! The verifier, RPC quorum, history codec and durable journal run in-process;
//! the two TLS peers replay the case and maintain independent journal heads.
//! Wiring request (domain 8): services/platform/hosted/authority must provide
//! layerx_getMigrationJournalHead/layerx_advanceMigrationJournalHead, or identify
//! the external producer; audit finding rollback-authority-unwired records
//! this existing dependency. It cannot be implemented inside this task's scope.

use std::collections::{BTreeMap, BTreeSet};
use std::error::Error;
use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine as _;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_migrate::ethereum::{EthereumConfig, EthereumHistoryClaim, EthereumVerifier};
use layerx_migrate::history::DurableExternalHistory;
use layerx_migrate::solana::immutable_program_code_hash;
use layerx_migrate::{
    ExternalHistoryKind, ExternalHistoryRecord, ExternalHistorySink, ExternalProvenance,
    JournalConfig, MigrationError, RpcEndpointConfig, RpcQuorumConfig, SourceTransaction,
    SourceVerifier, VerifiedHistoryPage,
};
use rustls::pki_types::{CertificateDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde::Deserialize;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

const PROGRAM_CAPTURE: &str = include_str!("../src/fixtures/repair_22_31_163_program_data.json");
const DEPLOYED_CODE_PIN: &str = "31f0a627dba051a938de650464e55cc5397a4be0fd496929c1f9cf02fe5e9011";

fn program_capture() -> (Value, Vec<u8>, u64) {
    let capture: Value = serde_json::from_str(PROGRAM_CAPTURE).expect("genuine RPC capture");
    let response = &capture["program_data"]["result"];
    let data = BASE64
        .decode(
            response["value"]["data"][0]
                .as_str()
                .expect("base64 account"),
        )
        .expect("captured bytes");
    let slot = response["context"]["slot"]
        .as_u64()
        .expect("finalized slot");
    (capture, data, slot)
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

#[test]
fn solana_code_offset_finalized_capture_matches_deployed_code_pin() {
    let (capture, data, slot) = program_capture();
    assert_eq!(capture["commitment"], "finalized");
    assert_eq!(capture["deployed_code_sha256"], DEPLOYED_CODE_PIN);
    assert_eq!(
        capture["program_data"]["result"]["value"]["executable"],
        false
    );
    assert_eq!(
        capture["program_data"]["result"]["value"]["owner"],
        "BPFLoaderUpgradeab1e11111111111111111111111"
    );
    assert_eq!(capture["program"]["result"]["value"]["executable"], true);
    assert_eq!(&data[..4], &3_u32.to_le_bytes());
    assert_eq!(
        capture["program_data"]["result"]["value"]["space"].as_u64(),
        Some(data.len() as u64),
        "pin covers the complete deployed allocation"
    );
    let deployment_slot = u64::from_le_bytes(data[4..12].try_into().expect("deployment slot"));
    assert!(deployment_slot > 0 && deployment_slot <= slot);
    assert_eq!(data[12], 0, "captured program is immutable");
    assert!(
        data[13..45].iter().any(|byte| *byte != 0),
        "loader retains former authority bytes"
    );
    assert_eq!(&data[45..49], b"\x7fELF");
    assert_eq!(
        hex(&immutable_program_code_hash(&data, slot).expect("immutable deployed account")),
        DEPLOYED_CODE_PIN
    );
    assert_eq!(hex(&Sha256::digest(&data[45..])), DEPLOYED_CODE_PIN);
    assert_ne!(hex(&Sha256::digest(&data[13..])), DEPLOYED_CODE_PIN);
    assert_eq!(
        hex(&immutable_program_code_hash(&data, deployment_slot).expect("deployment boundary")),
        DEPLOYED_CODE_PIN
    );
    assert!(matches!(
        immutable_program_code_hash(&data, deployment_slot - 1),
        Err(MigrationError::CustodyProgramMismatch)
    ));
    // A deployment hash includes allocation padding. It never strips zeros.
    let mut extended = data.clone();
    extended.push(0);
    assert_ne!(
        immutable_program_code_hash(&extended, slot).expect("padded code"),
        immutable_program_code_hash(&data, slot).expect("original code")
    );
}

#[test]
fn solana_code_offset_rejects_invalid_metadata_and_changed_code() {
    let (_, data, slot) = program_capture();
    for length in [0, 4, 12, 13, 44, 45] {
        assert!(
            matches!(
                immutable_program_code_hash(&data[..length], slot),
                Err(MigrationError::CustodyProgramMismatch)
            ),
            "truncated account length {length}"
        );
    }
    for tag in [0_u32, 1, 2, 4] {
        let mut altered = data.clone();
        altered[..4].copy_from_slice(&tag.to_le_bytes());
        assert!(matches!(
            immutable_program_code_hash(&altered, slot),
            Err(MigrationError::CustodyProgramMismatch)
        ));
    }
    for authority_tag in [1, 2, 255] {
        let mut altered = data.clone();
        altered[12] = authority_tag;
        assert!(matches!(
            immutable_program_code_hash(&altered, slot),
            Err(MigrationError::CustodyProgramMismatch)
        ));
    }
    let mut future = data.clone();
    future[4..12].copy_from_slice(&(slot + 1).to_le_bytes());
    assert!(matches!(
        immutable_program_code_hash(&future, slot),
        Err(MigrationError::CustodyProgramMismatch)
    ));
    let pin = immutable_program_code_hash(&data, slot).expect("captured pin");
    for offset in [45, data.len() - 1] {
        let mut altered = data.clone();
        altered[offset] ^= 1;
        assert_ne!(
            immutable_program_code_hash(&altered, slot).expect("valid metadata"),
            pin
        );
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct HistoryCase {
    source: EthereumConfig,
    sink: JournalConfig,
    address: [u8; 20],
    from_block: u64,
    to_block: u64,
    expected_record_count: usize,
    multiple_transfers: ExpectedRecord,
    custody_and_token: ExpectedRecord,
    self_transfer: ExpectedRecord,
    transcript: Vec<RpcExchange>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ExpectedRecord {
    transaction: [u8; 32],
    asset: [u8; 32],
    amount: String,
    distinct_log_indices: Vec<u64>,
}

const HISTORY_CASE: &str = include_str!("fixtures/repair_22_31_163/history-case.json");
const JOURNAL_FIXTURE: &str = include_str!("fixtures/repair_22_31_163/journal.json");
const TLS_CERT: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-cert.der");
const TLS_KEY: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-key.der");
static NEXT_FIXTURE: AtomicU64 = AtomicU64::new(0);

#[derive(Clone, Deserialize)]
#[serde(deny_unknown_fields)]
struct RpcExchange {
    method: String,
    params: Value,
    result: Value,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct JournalFixture {
    authentication_key: String,
    records: BTreeMap<String, String>,
    initial_heads: BTreeMap<String, Value>,
}

fn history_case() -> Result<HistoryCase, serde_json::Error> {
    serde_json::from_str(HISTORY_CASE)
}

struct FixtureDirectory(PathBuf);

impl FixtureDirectory {
    fn new() -> Result<Self, Box<dyn Error>> {
        let path = std::env::temp_dir().join(format!(
            "layerx-repair-22-31-163-{}-{}",
            std::process::id(),
            NEXT_FIXTURE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(fs::canonicalize(path)?))
    }

    fn write(&self, name: impl AsRef<Path>, bytes: &[u8]) -> Result<PathBuf, Box<dyn Error>> {
        let path = self.0.join(name);
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path)?;
        file.write_all(bytes)?;
        file.sync_all()?;
        Ok(path)
    }
}

impl Drop for FixtureDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove private test journal directory");
    }
}

struct TranscriptServer {
    address: SocketAddr,
    stopping: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<(String, Value)>>>,
    worker: Option<JoinHandle<()>>,
}

impl TranscriptServer {
    fn start(
        transcript: Vec<RpcExchange>,
        mut heads: BTreeMap<String, Value>,
    ) -> Result<Self, Box<dyn Error>> {
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(TLS_CERT.to_vec())],
                    PrivatePkcs8KeyDer::from(TLS_KEY.to_vec()).into(),
                )?,
        );
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let stopping = Arc::new(AtomicBool::new(false));
        let worker_stopping = Arc::clone(&stopping);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let worker_requests = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            for tcp in listener.incoming() {
                let tcp = tcp.expect("loopback accept");
                if worker_stopping.load(Ordering::Acquire) {
                    break;
                }
                tcp.set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("fixture read timeout");
                tcp.set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("fixture write timeout");
                let connection = ServerConnection::new(Arc::clone(&tls)).expect("fixture TLS");
                let mut stream = StreamOwned::new(connection, tcp);
                let request = read_rpc_request(&mut stream);
                let method = request["method"].as_str().expect("RPC method");
                let params = &request["params"];
                worker_requests
                    .lock()
                    .expect("request log")
                    .push((method.to_owned(), params.clone()));
                let result = match method {
                    "layerx_getMigrationJournalHead" => heads
                        .get(params[0].as_str().expect("anchor id"))
                        .expect("committed anchor namespace")
                        .clone(),
                    "layerx_advanceMigrationJournalHead" => {
                        let advance = &params[0];
                        let current = heads
                            .get_mut(advance["anchor_id"].as_str().expect("anchor id"))
                            .expect("committed anchor namespace");
                        assert_eq!(current["sequence"], advance["expected_sequence"]);
                        assert_eq!(current["digest"], advance["expected_digest"]);
                        assert_eq!(
                            advance["sequence"].as_u64(),
                            Some(current["sequence"].as_u64().expect("head sequence") + 1)
                        );
                        assert_eq!(advance["digest"].as_str().expect("head digest").len(), 64);
                        *current = json!({
                            "sequence": advance["sequence"],
                            "digest": advance["digest"]
                        });
                        current.clone()
                    }
                    _ => transcript
                        .iter()
                        .find(|exchange| exchange.method == method && exchange.params == *params)
                        .unwrap_or_else(|| panic!("unrecorded RPC request: {method} {params}"))
                        .result
                        .clone(),
                };
                let response = serde_json::to_vec(&json!({
                    "jsonrpc": "2.0", "id": request["id"], "result": result
                }))
                .expect("RPC response JSON");
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    response.len()
                )
                .expect("response header");
                stream.write_all(&response).expect("response body");
                stream.conn.send_close_notify();
                stream.flush().expect("flush TLS response");
            }
        });
        Ok(Self {
            address,
            stopping,
            requests,
            worker: Some(worker),
        })
    }
}

impl Drop for TranscriptServer {
    fn drop(&mut self) {
        self.stopping.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        let joined = self.worker.take().expect("fixture worker").join();
        if !thread::panicking() {
            joined.expect("TLS transcript server completed every request");
        }
    }
}

fn read_rpc_request(stream: &mut impl Read) -> Value {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        assert!(header.len() < 32 * 1024, "bounded request header");
        let mut byte = [0];
        stream.read_exact(&mut byte).expect("HTTP request header");
        header.push(byte[0]);
    }
    let header = String::from_utf8(header).expect("ASCII HTTP header");
    assert!(header.starts_with("POST / HTTP/1.1\r\n"));
    assert!(header.contains("Authorization: Bearer public-repair-fixture-token\r\n"));
    let length = header
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .expect("HTTP content length")
        .parse::<usize>()
        .expect("numeric content length");
    assert!(length <= 256 * 1024);
    let mut body = vec![0; length];
    stream.read_exact(&mut body).expect("RPC request body");
    let request: Value = serde_json::from_slice(&body).expect("JSON RPC request");
    assert_eq!(request["jsonrpc"], "2.0");
    assert!(request["id"].as_u64().is_some());
    request
}

struct HistoryFixture {
    case: HistoryCase,
    servers: Vec<TranscriptServer>,
    _directory: FixtureDirectory,
}

impl HistoryFixture {
    fn new(mut case: HistoryCase) -> Result<Self, Box<dyn Error>> {
        let journal: JournalFixture = serde_json::from_str(JOURNAL_FIXTURE)?;
        let directory = FixtureDirectory::new()?;
        let key = directory.write("journal-key", journal.authentication_key.as_bytes())?;
        let cert = directory.write("ca.der", TLS_CERT)?;
        let token = directory.write("bearer", b"public-repair-fixture-token")?;
        let mut servers = Vec::new();
        for _ in 0..2 {
            servers.push(TranscriptServer::start(
                case.transcript.clone(),
                journal.initial_heads.clone(),
            )?);
        }
        let rpc = RpcQuorumConfig {
            endpoints: servers
                .iter()
                .enumerate()
                .map(|(index, server)| RpcEndpointConfig {
                    url: format!("https://{}/", server.address),
                    ca_certificate_der: cert.clone(),
                    bearer_token_file: token.clone(),
                    independent_backend: format!("repair-fixture-{index}"),
                })
                .collect(),
            quorum: 2,
            connect_timeout_ms: 1000,
            request_timeout_ms: 5000,
            maximum_response_bytes: 1024 * 1024,
        };
        case.source.rpc = rpc.clone();
        let code = case
            .transcript
            .iter()
            .find(|exchange| exchange.method == "eth_getCode")
            .expect("custody deployment bytecode")
            .result
            .as_str()
            .expect("hex code");
        let code = code.strip_prefix("0x").expect("hex code prefix");
        let code: Vec<_> = (0..code.len())
            .step_by(2)
            .map(|offset| u8::from_str_radix(&code[offset..offset + 2], 16).expect("code byte"))
            .collect();
        case.source.custody_code_hash = Keccak256::digest(&code).into();
        for config in [&mut case.source.journal, &mut case.sink] {
            config.directory = directory.0.clone();
            config.authentication_key_file = key.clone();
            config.rollback_anchor_rpc = rpc.clone();
            fs::DirBuilder::new()
                .mode(0o700)
                .create(directory.0.join(&config.namespace))?;
            for (name, bytes) in &journal.records {
                directory.write(Path::new(&config.namespace).join(name), bytes.as_bytes())?;
            }
        }
        Ok(Self {
            case,
            servers,
            _directory: directory,
        })
    }

    fn assert_transcript_used(&self) {
        for server in &self.servers {
            let requests = server.requests.lock().expect("request log");
            for exchange in &self.case.transcript {
                assert!(
                    requests.contains(&(exchange.method.clone(), exchange.params.clone())),
                    "both quorum peers must receive {} {}",
                    exchange.method,
                    exchange.params
                );
            }
            for method in [
                "layerx_getMigrationJournalHead",
                "layerx_advanceMigrationJournalHead",
            ] {
                assert!(requests.iter().any(|(observed, _)| observed == method));
            }
        }
    }
}

fn assert_expected(
    page: &VerifiedHistoryPage,
    expected: &ExpectedRecord,
    kind: ExternalHistoryKind,
) {
    let records: Vec<_> = page
        .records()
        .iter()
        .filter(|record| {
            record.transaction() == SourceTransaction::Ethereum(expected.transaction)
                && record.source_asset() == expected.asset
                && record.kind() == kind
        })
        .collect();
    assert_eq!(records.len(), 1, "one aggregated declared history identity");
    assert_eq!(
        records[0].source_amount(),
        expected.amount.parse::<u128>().expect("captured total")
    );
    assert_eq!(records[0].provenance(), ExternalProvenance::Ethereum);
}

fn stored_records(
    sink: &DurableExternalHistory,
    principal: &PrincipalId,
) -> Result<Vec<ExternalHistoryRecord>, Box<dyn Error>> {
    let mut records = Vec::new();
    let mut cursor = None;
    loop {
        let page = sink.read(principal, cursor, 1)?;
        for record in page.records() {
            assert!(!records.contains(record));
            records.push(*record);
        }
        assert!(records.len() <= 256);
        match page.next_cursor() {
            Some(next) => {
                assert_ne!(Some(next), cursor);
                cursor = Some(next);
            }
            None => return Ok(records),
        }
    }
}

#[test]
fn history_transfer_loss_multiple_logs_overlap_and_sink_reopen() -> Result<(), Box<dyn Error>> {
    let fixture = HistoryFixture::new(history_case()?)?;
    let case = &fixture.case;
    for expected in [&case.multiple_transfers, &case.custody_and_token] {
        assert!(expected.distinct_log_indices.len() >= 2);
        assert_eq!(
            expected
                .distinct_log_indices
                .iter()
                .collect::<BTreeSet<_>>()
                .len(),
            expected.distinct_log_indices.len()
        );
    }
    assert_eq!(case.self_transfer.distinct_log_indices.len(), 1);
    assert_ne!(
        case.source.journal.rollback_anchor_id,
        case.sink.rollback_anchor_id
    );
    assert_ne!(
        case.source
            .journal
            .directory
            .join(&case.source.journal.namespace),
        case.sink.directory.join(&case.sink.namespace)
    );
    let evidence = EthereumHistoryClaim {
        chain_id: case.source.chain_id,
        address: case.address,
        from_block: case.from_block,
        to_block: case.to_block,
        previous_cursor: None,
    }
    .evidence()?;
    let trace = TraceId::mint([163; 16]);
    let principal = PrincipalId::new("repair-22-31-163")?;
    let verifier = EthereumVerifier::new(case.source.clone())?;
    let page = verifier.verify_history(&evidence, &trace)?;
    assert_eq!(page.evidence_digest(), evidence.digest());
    assert_eq!(page.records().len(), case.expected_record_count);
    assert!(page.next_cursor().is_some());
    assert_ne!(page.next_cursor(), Some([0; 32]));
    assert_expected(
        &page,
        &case.multiple_transfers,
        ExternalHistoryKind::Outgoing,
    );
    assert_expected(
        &page,
        &case.custody_and_token,
        ExternalHistoryKind::Outgoing,
    );
    assert_expected(&page, &case.self_transfer, ExternalHistoryKind::Contract);
    let mut sink = DurableExternalHistory::new(&case.sink)?;
    assert!(
        sink.read(&principal, None, 256)?.records().is_empty(),
        "dedicated fresh qualification namespace required"
    );
    sink.store_external(&principal, &page, &trace)?;
    let stored = stored_records(&sink, &principal)?;
    assert_eq!(stored.len(), page.records().len());
    assert!(page.records().iter().all(|record| stored.contains(record)));
    // Stop both consumers after sink persistence but before source cursor commit.
    drop(sink);
    drop(verifier);
    let verifier = EthereumVerifier::new(case.source.clone())?;
    let replayed = verifier.verify_history(&evidence, &trace)?;
    assert_eq!(replayed, page);
    let mut sink = DurableExternalHistory::new(&case.sink)?;
    assert_eq!(stored_records(&sink, &principal)?, stored);
    sink.store_external(&principal, &replayed, &trace)?;
    sink.store_external(&principal, &replayed, &trace)?;
    assert_eq!(stored_records(&sink, &principal)?, stored);
    verifier.commit_history(&evidence, &replayed, &trace)?;
    drop(sink);
    drop(verifier);
    let verifier = EthereumVerifier::new(case.source.clone())?;
    assert_eq!(verifier.verify_history(&evidence, &trace)?, page);
    let sink = DurableExternalHistory::new(&case.sink)?;
    assert_eq!(stored_records(&sink, &principal)?, stored);
    fixture.assert_transcript_used();
    // Both namespaces contain new records beyond the committed seed, and the
    // quorum peers accepted their authenticated digest progression.
    for config in [&case.source.journal, &case.sink] {
        let journal = config.directory.join(&config.namespace);
        assert!(journal.join("00000000000000000002.record").is_file());
        assert!(journal.join("00000000000000000002.seal").is_file());
    }
    Ok(())
}

#[test]
fn history_transfer_loss_rejects_conflicting_log_observations() -> Result<(), Box<dyn Error>> {
    let mut case = history_case()?;
    let outgoing = case
        .transcript
        .iter_mut()
        .find(|exchange| {
            exchange.method == "eth_getLogs"
                && exchange.params[0]["topics"]
                    .as_array()
                    .is_some_and(|topics| topics.len() == 2)
        })
        .expect("outgoing transfer transcript");
    let logs = outgoing.result.as_array_mut().expect("transfer logs");
    let mut conflicting = logs[0].clone();
    conflicting["data"] = logs[1]["data"].clone();
    assert_ne!(conflicting, logs[0]);
    logs.push(conflicting);
    let fixture = HistoryFixture::new(case)?;
    let evidence = EthereumHistoryClaim {
        chain_id: fixture.case.source.chain_id,
        address: fixture.case.address,
        from_block: fixture.case.from_block,
        to_block: fixture.case.to_block,
        previous_cursor: None,
    }
    .evidence()?;
    let verifier = EthereumVerifier::new(fixture.case.source.clone())?;
    assert!(matches!(
        verifier.verify_history(&evidence, &TraceId::mint([164; 16])),
        Err(MigrationError::RpcResponseMismatch)
    ));
    assert!(
        !fixture
            .case
            .source
            .journal
            .directory
            .join("source/00000000000000000002.record")
            .exists(),
        "conflicting observations cannot advance the page cursor"
    );
    Ok(())
}

#[test]
fn history_transfer_loss_rejects_aggregation_overflow() -> Result<(), Box<dyn Error>> {
    let mut case = history_case()?;
    let outgoing = case
        .transcript
        .iter_mut()
        .find(|exchange| {
            exchange.method == "eth_getLogs"
                && exchange.params[0]["topics"]
                    .as_array()
                    .is_some_and(|topics| topics.len() == 2)
        })
        .expect("outgoing transfer transcript");
    let logs = outgoing.result.as_array_mut().expect("transfer logs");
    logs[0]["data"] = Value::String(format!("0x{:064x}", u128::MAX));
    logs[1]["data"] = Value::String(format!("0x{:064x}", 1));
    let fixture = HistoryFixture::new(case)?;
    let evidence = EthereumHistoryClaim {
        chain_id: fixture.case.source.chain_id,
        address: fixture.case.address,
        from_block: fixture.case.from_block,
        to_block: fixture.case.to_block,
        previous_cursor: None,
    }
    .evidence()?;
    let verifier = EthereumVerifier::new(fixture.case.source.clone())?;
    assert!(matches!(
        verifier.verify_history(&evidence, &TraceId::mint([165; 16])),
        Err(MigrationError::InvalidHistory)
    ));
    assert!(
        !fixture
            .case
            .source
            .journal
            .directory
            .join("source/00000000000000000002.record")
            .exists(),
        "overflow cannot advance the page cursor"
    );
    Ok(())
}
