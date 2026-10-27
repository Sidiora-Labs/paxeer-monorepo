use layerx_client::sequencer::{SequencerEndpoint, SequencerError, SequencerReceipt};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog, SequencerLogRefusal};
use native_tls::{Certificate, TlsConnector};
use sha2::{Digest, Sha256};
use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::sync::Arc;
use std::time::Duration;
use zeroize::Zeroizing;

use crate::payer_tx::{decode_pay_exact, PayerTransactionRefusal};

pub(crate) struct ChainRelay {
    pub(crate) endpoint: Arc<SequencerEndpoint>,
    pub(crate) receipt_keys: Vec<SequencerAuthorization>,
    pub(crate) journal: RelayJournal,
}

pub(crate) struct RelayJournal {
    host: String,
    port: u16,
    connector: TlsConnector,
    username: Zeroizing<String>,
    password: Zeroizing<String>,
}

struct JournalEntry {
    tx_hash: [u8; 32],
    raw: Vec<u8>,
}

impl RelayJournal {
    pub(crate) fn new(
        redis_url: &str,
        ca: Certificate,
        username: Zeroizing<String>,
        password: Zeroizing<String>,
    ) -> Result<Self, RelayError> {
        layerx_platform_gateway::store::RedisEndpoint::parse(redis_url)
            .map_err(RelayError::Journal)?;
        let authority = redis_url
            .strip_prefix("rediss://")
            .ok_or_else(|| RelayError::Journal("Redis TLS endpoint required".to_owned()))?
            .trim_end_matches('/');
        let (host, port) = match authority.rsplit_once(':') {
            Some((host, port)) => (
                host,
                port.parse::<u16>()
                    .map_err(|error| RelayError::Journal(error.to_string()))?,
            ),
            None => (authority, 6379),
        };
        let connector = TlsConnector::builder()
            .add_root_certificate(ca)
            .min_protocol_version(Some(native_tls::Protocol::Tlsv12))
            .build()
            .map_err(|error| RelayError::Journal(error.to_string()))?;
        Ok(Self {
            host: host.to_owned(),
            port,
            connector,
            username,
            password,
        })
    }

    fn key(idempotency_key: &str) -> Result<String, RelayError> {
        if idempotency_key.is_empty() || idempotency_key.len() > 4096 {
            return Err(RelayError::Journal(
                "invalid relay idempotency key".to_owned(),
            ));
        }
        Ok(format!(
            "interop:relay:v1:{:x}",
            Sha256::digest(idempotency_key.as_bytes())
        ))
    }

    fn command(&self, arguments: &[&[u8]]) -> Result<Option<Vec<u8>>, RelayError> {
        let exchange = || -> Result<Option<Vec<u8>>, Box<dyn std::error::Error>> {
            let mut tcp = None;
            for address in (self.host.as_str(), self.port).to_socket_addrs()?.take(8) {
                if let Ok(stream) = TcpStream::connect_timeout(&address, Duration::from_secs(3)) {
                    tcp = Some(stream);
                    break;
                }
            }
            let tcp = tcp.ok_or("relay journal connection unavailable")?;
            tcp.set_read_timeout(Some(Duration::from_secs(8)))?;
            tcp.set_write_timeout(Some(Duration::from_secs(8)))?;
            let mut stream = self.connector.connect(&self.host, tcp)?;
            Self::write_command(
                &mut stream,
                &[b"AUTH", self.username.as_bytes(), self.password.as_bytes()],
            )?;
            if Self::read_line(&mut stream)? != b"+OK" {
                return Err("relay journal authentication refused".into());
            }
            Self::write_command(&mut stream, arguments)?;
            let header = Self::read_line(&mut stream)?;
            if header == b"$-1" {
                return Ok(None);
            }
            let length = header.strip_prefix(b"$").ok_or("invalid journal reply")?;
            let length = std::str::from_utf8(length)?.parse::<usize>()?;
            if length > 2 * 1024 * 1024 {
                return Err("relay journal record exceeds its bound".into());
            }
            let mut bytes = vec![0; length];
            stream.read_exact(&mut bytes)?;
            let mut ending = [0; 2];
            stream.read_exact(&mut ending)?;
            if ending != *b"\r\n" {
                return Err("invalid journal reply ending".into());
            }
            Ok(Some(bytes))
        };
        exchange().map_err(|error| RelayError::Journal(error.to_string()))
    }

    fn write_command(stream: &mut impl Write, arguments: &[&[u8]]) -> std::io::Result<()> {
        write!(stream, "*{}\r\n", arguments.len())?;
        for argument in arguments {
            write!(stream, "${}\r\n", argument.len())?;
            stream.write_all(argument)?;
            stream.write_all(b"\r\n")?;
        }
        stream.flush()
    }

    fn read_line(stream: &mut impl Read) -> std::io::Result<Vec<u8>> {
        let mut line = Vec::new();
        for _ in 0..1024 {
            let mut byte = [0];
            stream.read_exact(&mut byte)?;
            line.push(byte[0]);
            if line.ends_with(b"\r\n") {
                line.truncate(line.len() - 2);
                return Ok(line);
            }
        }
        Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            "journal header exceeds its bound",
        ))
    }

    fn decode(bytes: &[u8]) -> Result<JournalEntry, RelayError> {
        let (hash, raw) = bytes
            .split_at_checked(32)
            .ok_or_else(|| RelayError::Journal("invalid relay journal record".to_owned()))?;
        Ok(JournalEntry {
            tx_hash: hash
                .try_into()
                .map_err(|_| RelayError::TransactionHashMismatch)?,
            raw: raw.to_vec(),
        })
    }

    fn read(&self, key: &str) -> Result<Option<JournalEntry>, RelayError> {
        self.command(&[b"GET", key.as_bytes()])?
            .map(|bytes| Self::decode(&bytes))
            .transpose()
    }

    fn claim(&self, key: &str, raw: &[u8], tx_hash: [u8; 32]) -> Result<bool, RelayError> {
        let mut record = tx_hash.to_vec();
        record.extend_from_slice(raw);
        if record.len() + 1 > 2 * 1024 * 1024 {
            return Err(RelayError::Journal(
                "relay journal record exceeds its bound".to_owned(),
            ));
        }
        let answer = self.command(&[
            b"EVAL",
            b"local old = redis.call('GET', KEYS[1]); if old then return string.char(0) .. old end; redis.call('SET', KEYS[1], ARGV[1]); return string.char(1) .. ARGV[1]",
            b"1", key.as_bytes(), &record,
        ])?.ok_or_else(|| RelayError::Journal("missing relay journal claim".to_owned()))?;
        let (status, bytes) = answer
            .split_first()
            .ok_or_else(|| RelayError::Journal("invalid relay journal claim".to_owned()))?;
        let entry = Self::decode(bytes)?;
        if entry.raw != raw {
            return Err(RelayError::TransactionReplacementRefused);
        }
        if entry.tx_hash != tx_hash {
            return Err(RelayError::TransactionHashMismatch);
        }
        match status {
            0 => Ok(false),
            1 => Ok(true),
            _ => Err(RelayError::Journal(
                "invalid relay journal claim status".to_owned(),
            )),
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum RelayOutcome {
    Pending { tx_hash: [u8; 32] },
    Executed(SequencerEvidence),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum RelayError {
    Transaction(PayerTransactionRefusal),
    Sequencer(SequencerError),
    TransactionHashMismatch,
    TransactionReplacementRefused,
    Journal(String),
    ReceiptRejected(VerificationFailure),
    LogsRejected(SequencerLogRefusal),
    Unknown,
}

impl std::fmt::Display for RelayError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Transaction(error) => error.fmt(formatter),
            Self::TransactionReplacementRefused => {
                formatter.write_str("transaction_replacement_refused")
            }
            error => write!(formatter, "relay refused: {error:?}"),
        }
    }
}

impl std::error::Error for RelayError {}

impl ChainRelay {
    pub(crate) fn relay(
        &self,
        idempotency_key: &str,
        raw: &[u8],
        expected_sender: [u8; 20],
    ) -> Result<RelayOutcome, RelayError> {
        let key = RelayJournal::key(idempotency_key)?;
        let stored = self.journal.read(&key)?;
        if stored.as_ref().is_some_and(|entry| entry.raw != raw) {
            return Err(RelayError::TransactionReplacementRefused);
        }
        let (_, sender, decoded_hash) = decode_pay_exact(raw).map_err(RelayError::Transaction)?;
        if sender != expected_sender {
            return Err(RelayError::Transaction(
                PayerTransactionRefusal::SignatureRefused,
            ));
        }
        let tx_hash = match stored {
            Some(entry) => {
                if entry.tx_hash != decoded_hash {
                    return Err(RelayError::TransactionHashMismatch);
                }
                entry.tx_hash
            }
            None => {
                if self.journal.claim(&key, raw, decoded_hash)? {
                    let submitted_hash = self
                        .endpoint
                        .send_raw_transaction(raw)
                        .map_err(RelayError::Sequencer)?;
                    if submitted_hash != decoded_hash {
                        return Err(RelayError::TransactionHashMismatch);
                    }
                }
                decoded_hash
            }
        };
        let answer = self
            .endpoint
            .sequencer_receipt(&tx_hash)
            .map_err(RelayError::Sequencer)?;
        let (executed, rung) = match answer {
            Some(SequencerReceipt::Submitted { .. }) => {
                return Ok(RelayOutcome::Pending { tx_hash });
            }
            Some(SequencerReceipt::Receipted(receipt)) => (receipt, SettlementRung::Receipted),
            Some(SequencerReceipt::Committed {
                receipt,
                block_number,
                block_hash,
            }) => {
                let rung = match self.endpoint.block_by_number(block_number) {
                    Ok(Some(block)) if block.hash == block_hash => SettlementRung::Committed {
                        block_number,
                        block_hash,
                    },
                    Ok(_) | Err(_) => SettlementRung::Receipted,
                };
                (receipt, rung)
            }
            None => return Err(RelayError::Unknown),
        };
        let verified = verify_sequencer_signature(
            &executed.receipt,
            ReceiptKeys::HeightRanges(&self.receipt_keys),
        )
        .map_err(RelayError::ReceiptRejected)?;
        if verified.protocol().map(|receipt| receipt.activity_id()) != Some(tx_hash) {
            return Err(RelayError::ReceiptRejected(VerificationFailure {
                check: ReceiptCheck::ActivityId,
            }));
        }
        let logs = executed
            .logs
            .into_iter()
            .map(|log| SequencerLog::new(log.address, log.topics, log.data, log.log_index))
            .collect();
        let logs = verify_logs(&verified, logs)
            .map_err(RelayError::LogsRejected)?
            .into_logs();
        Ok(RelayOutcome::Executed(SequencerEvidence {
            transaction_hash: tx_hash,
            signed_receipt: executed.receipt,
            logs,
            rung,
        }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_platform_gateway::http;
    use serde_json::{json, Value};
    use std::fs;
    use std::net::TcpListener;
    use std::path::PathBuf;
    use std::process::{Child, Command, Stdio};
    use std::thread;
    use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

    type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

    struct RedisProcess {
        child: Child,
        directory: PathBuf,
        url: String,
        certificate: Certificate,
    }

    impl RedisProcess {
        fn start() -> TestResult<Self> {
            let directory = std::env::temp_dir().join(format!(
                "interop-relay-{}-{}",
                std::process::id(),
                SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos(),
            ));
            fs::create_dir(&directory)?;
            let pem = directory.join("server.pem");
            let key = directory.join("server.key");
            let output = Command::new("openssl")
                .args([
                    "req",
                    "-x509",
                    "-newkey",
                    "rsa:2048",
                    "-nodes",
                    "-days",
                    "1",
                    "-subj",
                    "/CN=localhost",
                    "-addext",
                    "subjectAltName=DNS:localhost",
                    "-keyout",
                ])
                .arg(&key)
                .arg("-out")
                .arg(&pem)
                .output()?;
            if !output.status.success() {
                return Err(format!(
                    "test certificate: {}",
                    String::from_utf8_lossy(&output.stderr)
                )
                .into());
            }
            let certificate = Certificate::from_pem(&fs::read(&pem)?)?;
            let listener = TcpListener::bind("127.0.0.1:0")?;
            let port = listener.local_addr()?.port();
            let acl = directory.join("users.acl");
            fs::write(
                &acl,
                "user default off\nuser relay on >relay-test-secret ~* &* +@all\n",
            )?;
            let config = directory.join("redis.conf");
            fs::write(&config, format!(
                "bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                pem.display(), key.display(), pem.display(), acl.display(), directory.display(),
            ))?;
            drop(listener);
            let child = Command::new("redis-server")
                .arg(&config)
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()?;
            let mut process = Self {
                child,
                directory,
                url: format!("rediss://localhost:{port}"),
                certificate,
            };
            let deadline = Instant::now() + Duration::from_secs(5);
            loop {
                if TcpStream::connect(("127.0.0.1", port)).is_ok() {
                    return Ok(process);
                }
                if let Some(status) = process.child.try_wait()? {
                    return Err(format!("test Redis exited: {status}").into());
                }
                if Instant::now() >= deadline {
                    return Err("test Redis did not start".into());
                }
                thread::sleep(Duration::from_millis(10));
            }
        }

        fn journal(&self) -> Result<RelayJournal, RelayError> {
            RelayJournal::new(
                &self.url,
                self.certificate.clone(),
                Zeroizing::new("relay".to_owned()),
                Zeroizing::new("relay-test-secret".to_owned()),
            )
        }
    }

    impl Drop for RedisProcess {
        fn drop(&mut self) {
            let _ = self.child.kill();
            let _ = self.child.wait();
            let _ = fs::remove_dir_all(&self.directory);
        }
    }

    fn fixture() -> TestResult<Value> {
        let fixtures: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        fixtures["cases"]
            .as_array()
            .ok_or("fixture cases must be an array")?
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            .cloned()
            .ok_or_else(|| "missing recorded payExact fixture".into())
    }

    fn hex(value: &Value) -> TestResult<Vec<u8>> {
        let encoded = value
            .as_str()
            .and_then(|value| value.strip_prefix("0x"))
            .ok_or("expected prefixed fixture hex")?;
        if encoded.len() % 2 != 0 {
            return Err("fixture hex must have an even length".into());
        }
        encoded
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
            .collect()
    }

    fn fixed<const N: usize>(value: &Value) -> TestResult<[u8; N]> {
        hex(value)?
            .try_into()
            .map_err(|_| "fixture hex width".into())
    }

    fn pinned_key(fixture: &Value) -> TestResult<SequencerAuthorization> {
        let pin = &fixture["pinnedKey"];
        Ok(SequencerAuthorization::new(
            fixed(&pin["keyId"])?,
            fixed(&pin["publicKey"])?,
            pin["firstHeight"].as_u64().ok_or("first height")?,
            pin["lastHeight"].as_u64().ok_or("last height")?,
        ))
    }

    fn rpc_server(
        fixture: &Value,
        receipt_answers: Vec<Value>,
    ) -> TestResult<(
        Arc<SequencerEndpoint>,
        thread::JoinHandle<Result<(), String>>,
    )> {
        rpc_server_with_blocks(fixture, receipt_answers, Vec::new())
    }

    fn rpc_server_with_blocks(
        fixture: &Value,
        receipt_answers: Vec<Value>,
        block_answers: Vec<(u64, Value)>,
    ) -> TestResult<(
        Arc<SequencerEndpoint>,
        thread::JoinHandle<Result<(), String>>,
    )> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = SequencerEndpoint::new(&format!("http://{}", listener.local_addr()?), &[])
            .map_err(|error| format!("loopback endpoint: {error:?}"))?;
        let raw_json = fixture["rawTransaction"].clone();
        let tx_hash_json = fixture["receiptJson"]["txHash"].clone();
        let server = thread::spawn(move || -> Result<(), String> {
            let deadline = Instant::now() + Duration::from_secs(5);
            let requests = std::iter::once((
                "eth_sendRawTransaction",
                json!([raw_json]),
                tx_hash_json.clone(),
            ))
            .chain(receipt_answers.into_iter().map(|answer| {
                (
                    "pax_sequencerReceipt",
                    json!([tx_hash_json.clone()]),
                    answer,
                )
            }))
            .chain(block_answers.into_iter().map(|(height, answer)| {
                (
                    "eth_getBlockByNumber",
                    json!([format!("{height:#x}"), true]),
                    answer,
                )
            }));
            for (method, params, result) in requests {
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            if Instant::now() >= deadline {
                                return Err(format!("timed out waiting for {method}"));
                            }
                            thread::sleep(Duration::from_millis(5));
                        }
                        Err(error) => return Err(error.to_string()),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                let request = http::read_request(&mut stream, 64 * 1024)?;
                let rpc: Value =
                    serde_json::from_slice(&request.body).map_err(|error| error.to_string())?;
                assert_eq!(request.method, "POST");
                assert_eq!(rpc["jsonrpc"], "2.0");
                assert_eq!(rpc["method"], method);
                assert_eq!(rpc["params"], params);
                let body = serde_json::to_vec(&json!({
                    "jsonrpc": "2.0", "id": rpc["id"], "result": result,
                }))
                .map_err(|error| error.to_string())?;
                http::write_response(
                    &mut stream,
                    &http::OutgoingResponse {
                        content_type: "application/json".to_owned(),
                        headers: Vec::new(),
                        status: 200,
                        body,
                        retry_after: None,
                    },
                )?;
            }
            Ok(())
        });
        Ok((Arc::new(endpoint), server))
    }

    fn run_relay(
        fixture: &Value,
        receipt_answer: Value,
        receipt_keys: &[SequencerAuthorization],
    ) -> TestResult<Result<RelayOutcome, RelayError>> {
        let redis = RedisProcess::start()?;
        let journal = redis.journal()?;
        let (endpoint, server) = rpc_server(fixture, vec![receipt_answer])?;
        let raw = hex(&fixture["rawTransaction"])?;
        let expected_sender = fixed(&fixture["receiptJson"]["from"])?;
        let relay = ChainRelay {
            endpoint,
            receipt_keys: receipt_keys.to_vec(),
            journal,
        };
        assert_eq!(
            relay.relay("recorded-translation", &raw, [0; 20]),
            Err(RelayError::Transaction(
                PayerTransactionRefusal::SignatureRefused
            )),
        );
        assert_eq!(
            RelayError::Transaction(PayerTransactionRefusal::SignatureRefused).to_string(),
            "transaction_signature_refused",
        );
        let outcome = relay.relay("recorded-translation", &raw, expected_sender);
        server
            .join()
            .map_err(|_| "loopback JSON-RPC listener panicked")??;
        Ok(outcome)
    }

    #[test]
    fn a_retry_reads_the_stored_hash_without_resubmitting() -> TestResult {
        let fixture = fixture()?;
        let redis = RedisProcess::start()?;
        let keys = [pinned_key(&fixture)?];
        let raw = hex(&fixture["rawTransaction"])?;
        let sender = fixed(&fixture["receiptJson"]["from"])?;
        let tx_hash = fixed(&fixture["receiptJson"]["txHash"])?;
        let submitted = json!({
            "rung": "submitted", "txHash": fixture["receiptJson"]["txHash"],
            "nonce": fixture["receiptJson"]["nonce"],
        });
        let (endpoint, server) = rpc_server(
            &fixture,
            vec![Value::Null, submitted, fixture["receiptJson"].clone()],
        )?;
        {
            let journal = redis.journal()?;
            let relay = ChainRelay {
                endpoint: Arc::clone(&endpoint),
                receipt_keys: keys.to_vec(),
                journal,
            };
            assert_eq!(
                relay.relay("retry-translation", &raw, sender),
                Err(RelayError::Unknown)
            );
        }
        let journal = redis.journal()?;
        let key = RelayJournal::key("retry-translation")?;
        let stored = journal
            .read(&key)?
            .ok_or("journal must survive reconstruction")?;
        assert_eq!(stored.raw, raw);
        assert_eq!(stored.tx_hash, tx_hash);
        let relay = ChainRelay {
            endpoint,
            receipt_keys: keys.to_vec(),
            journal,
        };
        assert_eq!(
            relay.relay("retry-translation", &raw, sender)?,
            RelayOutcome::Pending { tx_hash }
        );
        let RelayOutcome::Executed(evidence) = relay.relay("retry-translation", &raw, sender)?
        else {
            panic!("retry must read and verify the recorded receipt");
        };
        assert_eq!(evidence.transaction_hash, tx_hash);
        assert_eq!(evidence.signed_receipt, hex(&fixture["signedReceipt"])?);
        server
            .join()
            .map_err(|_| "loopback JSON-RPC listener panicked")??;
        Ok(())
    }

    #[test]
    fn a_replacement_under_one_key_is_refused() -> TestResult {
        let mut fixture = fixture()?;
        let original = hex(&fixture["rawTransaction"])?;
        let (mut call, _) = layerx_crypto::evm_transaction::decode_signed(&original)?;
        let raw = call.sign(&[0x3b; 32])?;
        call.max_fee_per_gas += 1;
        let replacement = call.sign(&[0x3b; 32])?;
        let (_, sender, tx_hash) = decode_pay_exact(&raw)?;
        let (_, replacement_sender, replacement_hash) = decode_pay_exact(&replacement)?;
        assert_eq!(sender, replacement_sender);
        assert_ne!(tx_hash, replacement_hash);
        let (first_call, _) = layerx_crypto::evm_transaction::decode_signed(&raw)?;
        assert_eq!(first_call.nonce, call.nonce);
        let encode = |bytes: &[u8]| -> String {
            format!(
                "0x{}",
                bytes
                    .iter()
                    .map(|byte| format!("{byte:02x}"))
                    .collect::<String>()
            )
        };
        fixture["rawTransaction"] = json!(encode(&raw));
        fixture["receiptJson"]["txHash"] = json!(encode(&tx_hash));
        let submitted = json!({
            "rung": "submitted", "txHash": encode(&tx_hash),
            "nonce": fixture["receiptJson"]["nonce"],
        });
        let redis = RedisProcess::start()?;
        let (endpoint, server) = rpc_server(&fixture, vec![submitted])?;
        {
            let journal = redis.journal()?;
            let relay = ChainRelay {
                endpoint: Arc::clone(&endpoint),
                receipt_keys: Vec::new(),
                journal,
            };
            assert_eq!(
                relay.relay("replacement-translation", &raw, sender)?,
                RelayOutcome::Pending { tx_hash }
            );
        }
        server
            .join()
            .map_err(|_| "loopback JSON-RPC listener panicked")??;
        let journal = redis.journal()?;
        let relay = ChainRelay {
            endpoint,
            receipt_keys: Vec::new(),
            journal,
        };
        assert_eq!(
            relay.relay("replacement-translation", &replacement, sender),
            Err(RelayError::TransactionReplacementRefused)
        );
        assert_eq!(
            relay.relay("replacement-translation", &[0x02], sender),
            Err(RelayError::TransactionReplacementRefused)
        );
        assert_eq!(
            RelayError::TransactionReplacementRefused.to_string(),
            "transaction_replacement_refused"
        );
        let key = RelayJournal::key("replacement-translation")?;
        let stored = relay
            .journal
            .read(&key)?
            .ok_or("original journal entry must remain")?;
        assert_eq!(stored.raw, raw);
        assert_eq!(stored.tx_hash, tx_hash);
        assert_eq!(
            relay.journal.claim(&key, &replacement, replacement_hash),
            Err(RelayError::TransactionReplacementRefused)
        );
        Ok(())
    }

    #[test]
    fn relays_the_recorded_pay_exact_to_receipted_evidence() -> TestResult {
        let fixture = fixture()?;
        let outcome = run_relay(
            &fixture,
            fixture["receiptJson"].clone(),
            &[pinned_key(&fixture)?],
        )??;
        let RelayOutcome::Executed(evidence) = outcome else {
            panic!("the recorded receipted answer must return executed evidence");
        };
        assert_eq!(evidence.rung, SettlementRung::Receipted);
        assert_eq!(
            evidence.transaction_hash,
            fixed(&fixture["receiptJson"]["txHash"])?
        );
        assert_eq!(evidence.signed_receipt, hex(&fixture["signedReceipt"])?);
        assert_eq!(evidence.logs.len(), 1);
        let log = &fixture["receiptJson"]["logs"][0];
        assert_eq!(evidence.logs[0].address(), fixed(&log["address"])?);
        let topics: Vec<[u8; 32]> = log["topics"]
            .as_array()
            .ok_or("log topics")?
            .iter()
            .map(fixed)
            .collect::<TestResult<_>>()?;
        assert_eq!(evidence.logs[0].topics(), topics);
        assert_eq!(evidence.logs[0].data(), hex(&log["data"])?);
        assert_eq!(evidence.logs[0].log_index(), 0);
        Ok(())
    }

    #[test]
    fn a_committed_answer_with_another_block_hash_is_receipted() -> TestResult {
        let fixture = fixture()?;
        let redis = RedisProcess::start()?;
        let raw = hex(&fixture["rawTransaction"])?;
        let sender = fixed(&fixture["receiptJson"]["from"])?;
        let tx_hash = fixed(&fixture["receiptJson"]["txHash"])?;
        let block_number = 1001;
        let block_hash = format!("0x{}", "ab".repeat(32));
        let mut committed = fixture["receiptJson"].clone();
        committed["rung"] = json!("committed");
        committed["blockNumber"] = json!(format!("{block_number:#x}"));
        committed["blockHash"] = json!(block_hash);
        let block = json!({
            "number": format!("{block_number:#x}"),
            "hash": block_hash,
            "parentHash": format!("0x{}", "00".repeat(32)),
            "timestamp": "0x67a1b2c3",
            "transactionsRoot": format!("0x{}", "00".repeat(32)),
            "transactions": [],
        });
        let mut mismatched = block.clone();
        mismatched["hash"] = json!(format!("0x{}", "cd".repeat(32)));
        let mut wrong_height = block.clone();
        wrong_height["number"] = json!("0x3ea");
        for (idempotency_key, block_answer, expected_rung) in [
            ("mismatched-block", mismatched, SettlementRung::Receipted),
            (
                "matching-block",
                block,
                SettlementRung::Committed {
                    block_number,
                    block_hash: fixed(&committed["blockHash"])?,
                },
            ),
            ("missing-block", Value::Null, SettlementRung::Receipted),
            ("wrong-height", wrong_height, SettlementRung::Receipted),
        ] {
            let (endpoint, server) = rpc_server_with_blocks(
                &fixture,
                vec![committed.clone()],
                vec![(block_number, block_answer)],
            )?;
            let relay = ChainRelay {
                endpoint,
                receipt_keys: vec![pinned_key(&fixture)?],
                journal: redis.journal()?,
            };
            let outcome = relay.relay(idempotency_key, &raw, sender);
            server
                .join()
                .map_err(|_| "loopback JSON-RPC listener panicked")??;
            let RelayOutcome::Executed(evidence) = outcome? else {
                panic!("a committed answer must retain its verified receipt");
            };
            assert_eq!(evidence.rung, expected_rung, "{idempotency_key}");
            assert_eq!(evidence.transaction_hash, tx_hash);
            assert_eq!(evidence.signed_receipt, hex(&fixture["signedReceipt"])?);
            assert_eq!(evidence.logs.len(), 1);
            assert_eq!(
                evidence.logs[0].data(),
                hex(&fixture["receiptJson"]["logs"][0]["data"])?
            );
        }
        Ok(())
    }

    #[test]
    fn a_submitted_answer_is_pending_with_the_tx_hash() -> TestResult {
        let fixture = fixture()?;
        let submitted = json!({
            "rung": "submitted",
            "txHash": fixture["receiptJson"]["txHash"],
            "nonce": fixture["receiptJson"]["nonce"],
        });
        let outcome = run_relay(&fixture, submitted, &[pinned_key(&fixture)?])??;
        assert_eq!(
            outcome,
            RelayOutcome::Pending {
                tx_hash: fixed(&fixture["receiptJson"]["txHash"])?,
            }
        );
        Ok(())
    }

    #[test]
    fn a_receipt_signed_by_an_unpinned_key_is_refused() -> TestResult {
        let fixture = fixture()?;
        let pinned = pinned_key(&fixture)?;
        let other_key = SequencerAuthorization::new(
            [0x7a; 32],
            ed25519_dalek::SigningKey::from_bytes(&[0x7a; 32])
                .verifying_key()
                .to_bytes(),
            pinned.first_batch_number(),
            pinned.last_batch_number(),
        );
        let outcome = run_relay(&fixture, fixture["receiptJson"].clone(), &[other_key])?;
        assert_eq!(
            outcome,
            Err(RelayError::ReceiptRejected(VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            }))
        );
        Ok(())
    }
}
