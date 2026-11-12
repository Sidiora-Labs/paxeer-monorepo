//! Audit 22.30.107 regression paths; finding and evidence record: `repair_22_33_63.kvx`.

pub mod bank {
    include!("support/repair_22_33_63_bank.rs");
}
#[path = "native_send_nonces.rs"]
mod native;
#[path = "support/repair_22_33_63_rotation.rs"]
pub mod rotation;

#[test]
fn journey_faults_producer_not_reopened() {
    bank::producer_outbox_and_consumer_reopen();
}

#[test]
fn move_test_substituted_transaction() {
    bank::exact_custody_bank_transfer();
}

#[test]
fn native_approval_clock_units() {
    clock_regressions::approval();
}

#[test]
fn native_producer_clock_units() {
    clock_regressions::producer();
}

#[test]
fn native_producer_assumes_static_head() {
    native::progressing_head_preserves_exact_action();
}

#[test]
fn session_key_rotation_synthetic_transaction() {
    rotation::current_rotation_reaches_custody("repair-22-33-63-session-rotation");
}

#[test]
fn session_rung_rotation_synthetic_transaction() {
    rotation::current_rotation_reaches_custody("repair-22-33-63-rung-rotation");
}

mod clock_regressions {
    use crate::require_value::RequireValue as _;
    use ed25519_dalek::Signer as _;
    use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
    use layerx_agentd::human::{serve_one, AcceptedTransport, ACCOUNT_SEQUENCE};
    use layerx_agentd::human::{HumanOperations, HumanPeer, HumanSubject};
    use layerx_agentd::human_runtime::{
        HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, SharedAgentOwner,
        UnifiedAgentOwner,
    };
    use layerx_agentd::identity::{
        self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
    };
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRegistry};
    use layerx_agentd::session_keys::SessionKeyRegistry;
    use layerx_agentd::store::{Store, TenantId};
    use layerx_client::paxeer_binding::{Binding, SignedBinding};
    use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
    use layerx_client::sequencer::SequencerEndpoint;

    use layerx_types::account::AccountId;
    use layerx_types::clock::ClockReading;
    use layerx_types::clock_protocol;
    use layerx_types::ids::Did;
    use layerx_types::payload::{ActivityType, ModuleId};
    use native_tls::{Identity, TlsAcceptor};
    use serde_json::{json, Value};
    use sha2::{Digest as _, Sha256};
    use std::collections::{BTreeMap, BTreeSet};
    use std::fs;
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
    use std::os::unix::net::{UnixListener, UnixStream};
    use std::path::{Path, PathBuf};

    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::{Arc, Mutex};
    use std::thread::{self, JoinHandle};
    use std::time::{Duration, Instant};

    use base64::Engine as _;
    #[track_caller]
    fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        match result {
            Ok(value) => value,
            Err(error) => panic!("bootstrap peer: {error:?}"),
        }
    }

    fn random<const N: usize>() -> [u8; N] {
        let mut value = [0; N];
        must(getrandom::fill(&mut value));
        value
    }

    fn directory(label: &str) -> PathBuf {
        let path =
            std::env::temp_dir().join(format!("bootstrap-{label}-{}", hex::encode(random::<16>())));
        must(fs::create_dir(&path));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o700),
        ));
        path
    }

    fn text<'a>(value: &'a Value, field: &str) -> &'a str {
        value[field]
            .as_str()
            .unwrap_or_else(|| panic!("missing {field}: {value}"))
    }

    struct Peer {
        stop: Arc<AtomicBool>,
        worker: Option<JoinHandle<()>>,
        connection: Option<Arc<Mutex<ConnectionState>>>,
    }

    #[derive(Default)]
    struct ConnectionState {
        stream: Option<UnixStream>,
        response: String,
    }

    // Observe the real daemon response before AgentRuntime reduces a peer
    // refusal to AgentBoundaryError::Refused. Never replace or rewrite a frame.
    struct ObservedTransport {
        inner: AcceptedTransport,
        opcode: Option<u8>,
        connection: Arc<Mutex<ConnectionState>>,
    }

    impl layerx_agentd::human::frame::HumanFrameTransport for ObservedTransport {
        fn receive(&mut self) -> Result<Vec<u8>, layerx_agentd::human::frame::TransportError> {
            let bytes = self.inner.receive()?;
            for frame in bytes.windows(9).filter(|part| &part[..8] == b"LXHAGT01") {
                self.opcode = Some(frame[8]);
                eprintln!("clock regression Human request opcode: {}", frame[8]);
            }
            Ok(bytes)
        }

        fn send(
            &mut self,
            bytes: &[u8],
        ) -> Result<(), layerx_agentd::human::frame::TransportError> {
            let offset = if bytes.starts_with(b"LXHAGT02") {
                48
            } else {
                0
            };
            let frame = &bytes[offset..];
            let mut diagnostic = format!(
                "Human opcode {:?}, response status {:?}, payload bytes {}",
                self.opcode,
                frame.get(8),
                frame.len().saturating_sub(9)
            );
            if self.opcode == Some(ACCOUNT_SEQUENCE) && frame.get(8) == Some(&0) {
                assert_eq!(
                    frame.len() - 9,
                    std::mem::size_of::<u64>(),
                    "daemon ACCOUNT_SEQUENCE response contains only its u64 sequence"
                );
                diagnostic.push_str("; ACCOUNT_SEQUENCE contains only its u64 sequence");
            }
            if frame.get(8) == Some(&1) {
                let length = u32::from_be_bytes(
                    frame[9..13]
                        .try_into()
                        .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
                ) as usize;
                std::fmt::Write::write_fmt(
                    &mut diagnostic,
                    format_args!(
                        "; refusal: {}",
                        must(std::str::from_utf8(&frame[13..13 + length]))
                    ),
                )
                .unwrap_or_else(|_| unreachable!("writing to String"));
                eprintln!(
                    "clock regression Human refusal: {}",
                    must(std::str::from_utf8(&frame[13..13 + length]))
                );
            } else if self.opcode == Some(67) && frame.get(9) == Some(&1) && frame.len() >= 16 {
                // Native envelope responses retain their production HTTP status
                // and structured refusal, even though the runtime returns a unit error.
                let status = u16::from_be_bytes(
                    frame[10..12]
                        .try_into()
                        .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
                );
                if status >= 400 {
                    std::fmt::Write::write_fmt(
                        &mut diagnostic,
                        format_args!(
                            "; native envelope refusal ({status}): {}",
                            String::from_utf8_lossy(&frame[16..])
                        ),
                    )
                    .unwrap_or_else(|_| unreachable!("writing to String"));
                    eprintln!(
                        "clock regression native envelope refusal ({status}): {}",
                        String::from_utf8_lossy(&frame[16..])
                    );
                }
            }
            must(self.connection.lock()).response = diagnostic;
            self.inner.send(bytes)
        }
    }

    impl Peer {
        fn spawn(work: impl FnOnce(Arc<AtomicBool>) + Send + 'static) -> Self {
            let stop = Arc::new(AtomicBool::new(false));
            let signal = Arc::clone(&stop);
            Self {
                stop,
                worker: Some(thread::spawn(move || work(signal))),
                connection: None,
            }
        }

        fn diagnostic(&self) -> String {
            must(self.connection.as_ref().or_fail("client connection").lock())
                .response
                .clone()
        }
    }

    impl Drop for Peer {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::Release);
            if let Some(connection) = &self.connection {
                if let Some(stream) = &must(connection.lock()).stream {
                    must(stream.shutdown(std::net::Shutdown::Both));
                }
            }
            if let Some(worker) = self.worker.take() {
                if let Err(failure) = worker.join() {
                    let message = failure
                        .downcast_ref::<String>()
                        .map(String::as_str)
                        .or_else(|| failure.downcast_ref::<&str>().copied())
                        .unwrap_or("non-string panic payload");
                    eprintln!("bootstrap peer failed: {message}");
                    if !thread::panicking() {
                        std::panic::resume_unwind(failure);
                    }
                }
            }
        }
    }

    fn incoming(listener: &TcpListener) -> Option<TcpStream> {
        match listener.accept() {
            Ok((stream, _)) => {
                must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                must(stream.set_write_timeout(Some(Duration::from_secs(3))));
                Some(stream)
            }
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(2));
                None
            }
            Err(error) => panic!("peer accept: {error}"),
        }
    }

    fn http_request(stream: &mut impl Read) -> (String, Vec<u8>) {
        let mut headers = Vec::new();
        while !headers.ends_with(b"\r\n\r\n") {
            assert!(headers.len() < 16_384);
            let mut byte = [0];
            must(stream.read_exact(&mut byte));
            headers.push(byte[0]);
        }
        let headers = must(String::from_utf8(headers));
        let length = headers
            .lines()
            .filter_map(|line| line.split_once(':'))
            .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
            .map_or(0, |(_, value)| must(value.trim().parse::<usize>()));
        assert!(length <= 1_048_576);
        let mut body = vec![0; length];
        must(stream.read_exact(&mut body));
        (headers, body)
    }

    fn http_response(stream: &mut impl Write, body: &Value) {
        let body = must(serde_json::to_vec(body));
        must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
        must(stream.write_all(&body));
        must(stream.flush());
    }

    struct AuthorityIdentity<'a> {
        authority: &'a mut RemoteHumanAuthority,
        peer: &'a HumanPeer,
    }
    impl IdentityResolver for AuthorityIdentity<'_> {
        fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
            self.authority.core_identity(self.peer, did).map(Some)
        }
    }

    fn authority_peer(
        peer: HumanPeer,
        identity: CoreIdentity,
        activity: ActivityType,
    ) -> (RemoteHumanAuthority, Peer) {
        authority_peer_with_identity(peer, Arc::new(Mutex::new(identity)), activity)
    }

    fn authority_peer_with_identity(
        peer: HumanPeer,
        identity: Arc<Mutex<CoreIdentity>>,
        activity: ActivityType,
    ) -> (RemoteHumanAuthority, Peer) {
        let pem = |label: &str, bytes: &[u8]| {
            format!(
                "-----BEGIN {label}-----\n{}\n-----END {label}-----\n",
                base64::engine::general_purpose::STANDARD.encode(bytes)
            )
        };
        let acceptor = must(TlsAcceptor::new(must(Identity::from_pkcs8(
            pem(
                "CERTIFICATE",
                include_bytes!(concat!(
                    env!("CARGO_MANIFEST_DIR"),
                    "/../../../platform/hosted/authority/src/human/fixtures/server.der"
                )),
            )
            .as_bytes(),
            pem(
                "PRIVATE KEY",
                include_bytes!(concat!(
                    env!("CARGO_MANIFEST_DIR"),
                    "/../../../platform/hosted/authority/src/human/fixtures/server-key.der"
                )),
            )
            .as_bytes(),
        ))));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let port = must(listener.local_addr()).port();
        must(listener.set_nonblocking(true));
        let bearer = hex::encode(random::<32>());
        let authority = must(RemoteHumanAuthority::connect(
            &format!("https://localhost:{port}"),
            bearer.clone(),
            Duration::from_secs(3),
            1_048_576,
            include_bytes!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../../platform/hosted/authority/src/human/fixtures/ca.der"
            )),
        ));
        let worker = Peer::spawn(move |stop| {
            let subject = peer.subject.as_ref().or_fail("owner subject");
            let account = must(AccountId::parse(&subject.account));
            let account_id = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
            while !stop.load(Ordering::Acquire) {
                let Some(stream) = incoming(&listener) else {
                    continue;
                };
                let mut stream = must(acceptor.accept(stream));
                let (headers, body) = http_request(&mut stream);
                assert!(body.is_empty());
                assert!(headers
                    .lines()
                    .any(|line| line
                        .eq_ignore_ascii_case(&format!("authorization: Bearer {bearer}"))));
                let path = headers
                    .split_whitespace()
                    .nth(1)
                    .or_fail("authority route")
                    .split('?')
                    .next()
                    .or_fail("route");
                let identity = must(identity.lock());
                eprintln!("clock regression authority read: {path}");
                let value = match path {
                    "/v1/agent/registry" => {
                        json!({"modules":[{"module_id":activity.module() as u16, "activity_types":[activity.value()]}]})
                    }
                    "/v1/agent/subject-context" => {
                        json!({"subject_principal":peer.principal, "tenant":subject.transport_tenant, "agent_tenant":peer.tenant,
                    "owner_did":subject.owner, "account_id":hex::encode(account_id), "asset_id":hex::encode(subject.asset),
                    "verification_level":layerx_types::verify::VerificationLevel::COMMITTED.wire_label(), "observed_head_sequence":identity.head_sequence, "checkpoint_digest":hex::encode(random::<32>())})
                    }
                    "/v1/agent/balance-context" => {
                        json!({"account_id":hex::encode(account_id), "asset_id":hex::encode(subject.asset),
                    "currency":"USD", "observed_at":"loopback committed block", "age_seconds":0,
                    "maximum_age_seconds":315_360_000,
                    "sequencer_id":hex::encode(Sha256::digest(&identity.canonical_bytes)),
                    "sequencer_public_key":hex::encode(&identity.canonical_bytes),
                    "first_batch_number":1, "last_batch_number":HEAD + 1000})
                    }
                    "/v1/agent/identity" => {
                        json!({"canonical_core_bytes":hex::encode(&identity.canonical_bytes), "head_sequence":identity.head_sequence,
                    "revocation_sequence":identity.revocation_sequence, "frozen":identity.frozen, "verification_level": match identity.verification_level { layerx_types::verify::VerificationLevel::COMMITTED => layerx_types::verify::VerificationLevel::COMMITTED.wire_label(), layerx_types::verify::VerificationLevel::STATE_PROVEN => layerx_types::verify::VerificationLevel::STATE_PROVEN.wire_label(), _ => panic!("identity floor case") },
                    "authorities":identity.authorities.iter().map(|authority| match authority { ProtocolAuthority::PrimaryKey(id) => json!({"kind":"primary_key","id":hex::encode(id)}), ProtocolAuthority::SessionKey(id) => json!({"kind":"session_key","id":hex::encode(id)}), ProtocolAuthority::CapabilityGrant(_) => panic!("owner authority") }).collect::<Vec<_>>()})
                    }
                    other => panic!("unexpected authority read: {other}"),
                };
                http_response(&mut stream, &value);
            }
        });
        (authority, worker)
    }

    fn clock_peer_at(root: &Path, unix_milliseconds: u64) -> (Arc<RuntimeClock>, Peer) {
        let path = root.join("clock.sock");
        let listener = must(UnixListener::bind(&path));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o600),
        ));
        must(listener.set_nonblocking(true));
        let generation = random::<16>();
        let worker = Peer::spawn(move |stop| {
            let start = Instant::now();
            while !stop.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                        let mut bytes = [0; clock_protocol::REQUEST_BYTES];
                        must(stream.read_exact(&mut bytes));
                        let request = must(clock_protocol::Request::decode(&bytes));
                        thread::sleep(Duration::from_nanos(request.wait_nanoseconds));
                        let reading = ClockReading {
                            generation,
                            unix_milliseconds: unix_milliseconds
                                .checked_add(must(start.elapsed().as_millis().try_into()))
                                .or_fail("fixture clock time"),
                            monotonic_nanoseconds: must(start.elapsed().as_nanos().try_into()),
                        };
                        must(
                            stream
                                .write_all(&clock_protocol::response(request.counter, Ok(reading))),
                        );
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("clock accept: {error}"),
                }
            }
        });
        let clock = must(RuntimeClock::connect(ClockBinding {
            socket: path,
            process: std::process::id(),
            user: must(fs::metadata(root)).uid(),
        }));
        (clock, worker)
    }

    type CommittedNativeCalls = BTreeMap<(String, Vec<u8>), Vec<u8>>;

    fn word(value: u128) -> [u8; 32] {
        let mut bytes = [0; 32];
        bytes[16..].copy_from_slice(&value.to_be_bytes());
        bytes
    }

    fn budget_call_material(
        owner: [u8; 32],
        asset: [u8; 32],
    ) -> (layerx_agentd::budget::ProtocolBudgetRecord, Vec<u8>) {
        let vectors: Value = must(serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../tests/vectors/native-budget-records.json"
        ))));
        let record = must(layerx_agentd::budget::ProtocolBudgetRecord::decode(&must(
            hex::decode(text(&vectors, "v1").trim_start_matches("0x")),
        )));
        let mut budget = vec![[0; 32]; 20 + record.delegates.len()];
        budget[0] = word(32);
        budget[1] = owner;
        budget[2] = record.budget_account;
        budget[3] = asset;
        budget[4] = record.purpose_hash;
        for (index, value) in [
            (5, record.per_period_limit),
            (6, record.configured_period_limit),
            (7, record.carry_cap),
            (8, record.spent_this_period),
            (9, record.carried),
            (10, u128::from(record.period_length)),
            (11, u128::from(record.period_start)),
            (12, u128::from(record.expiry)),
            (13, u128::from(record.revocation_sequence)),
            (14, u128::from(record.rollover_policy)),
            (15, u128::from(record.closed)),
            (16, u128::from(record.revoked)),
            (17, 576),
            (19, record.delegates.len() as u128),
        ] {
            budget[index] = word(value);
        }
        budget[18] = record.source_account.unwrap_or([0; 32]);
        budget[20..].copy_from_slice(&record.delegates);
        let budget = budget.concat();
        let decoded = must(
            layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
                record.budget_id,
                HEAD,
                &budget,
            ),
        );
        assert_eq!(decoded.record().owner, owner);
        assert_eq!(decoded.record().asset_id, asset);
        assert_eq!(decoded.record().remaining(), record.remaining());

        (record, budget)
    }

    fn committed_native_send_calls(binding: &SignedBinding) -> CommittedNativeCalls {
        let public = binding.public_key();
        let did = must(Did::new(
            format!("did:layerx:{}", hex::encode(public)).as_bytes(),
        ));
        let owner = must(layerx_wire::hash::did_id_for_protocol(&did, 3));
        let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
        let account = must(AccountId::for_asset(
            must(std::str::from_utf8(did.as_bytes())),
            asset,
            asset,
        ));
        let main = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
        let mut address = [0; 32];
        address[12..].copy_from_slice(&binding.binding().evm_address());
        let mut did_binding = [public, word(64), word(did.as_bytes().len() as u128)].concat();
        did_binding.extend_from_slice(did.as_bytes());
        did_binding.resize(did_binding.len().div_ceil(32) * 32, 0);

        // getAsset returns the twelve-word ledger metadata tuple, including three
        // dynamic fields. The fee denomination is the same upax asset as the send.
        let mut metadata = vec![[0; 32]; 12];
        metadata[0] = asset;
        metadata[1] = word(384);
        metadata[2] = word(448);
        metadata[3] = word(6);
        metadata[9] = word(512);
        let mut metadata = metadata.concat();
        for value in [b"PAX".as_slice(), b"Paxeer", b""] {
            metadata.extend_from_slice(&word(value.len() as u128));
            metadata.extend_from_slice(value);
            metadata.resize(metadata.len().div_ceil(32) * 32, 0);
        }

        // Use the production budget codec and the record used by the Human budget
        // checkpoint fixture, with its owner and asset bound to this fixture.
        let (record, budget) = budget_call_material(owner, asset);
        let mut calls = BTreeMap::new();
        let mut add = |target: [u8; 20], signature: &[u8], args: &[u8], answer: Vec<u8>| {
            let selector: [u8; 4] = layerx_crypto::evm_transaction::transaction_hash(signature)
                [..4]
                .try_into()
                .or_fail("four-byte selector");
            let calldata = [selector.as_slice(), args].concat();
            assert!(calls
                .insert((format!("0x{}", hex::encode(target)), calldata), answer)
                .is_none());
        };
        let layerx = layerx_client::account_binding::ADDR_PRECOMPILE;
        let mut ledger = [0; 20];
        ledger[18..].copy_from_slice(&[0x10, 0x1a]);
        let mut budget_precompile = ledger;
        budget_precompile[19] = 0x1b;
        add(
            layerx,
            b"getEvmAddrByLayerX(bytes32)",
            &public,
            address.to_vec(),
        );
        add(
            layerx,
            b"getUnifiedAccount(address)",
            &address,
            [address, word(128), public, main, word(0)].concat(),
        );
        add(layerx, b"getLayerXDid(address)", &address, did_binding);
        add(ledger, b"getAsset(bytes32)", &asset, metadata);
        add(
            ledger,
            b"accountId(bytes32,bytes32)",
            &[owner, asset].concat(),
            main.to_vec(),
        );
        add(ledger, b"accountOwner(bytes32)", &main, owner.to_vec());
        add(
            ledger,
            b"nextSequence(bytes32,bytes32)",
            &[main, asset].concat(),
            word(7).to_vec(),
        );
        add(
            budget_precompile,
            b"getBudget(bytes32)",
            &record.budget_id,
            budget,
        );
        add(
            budget_precompile,
            b"budgetRemaining(bytes32)",
            &record.budget_id,
            word(record.remaining()).to_vec(),
        );
        // balanceOf has two legitimate accounts, selected by the full calldata.
        // The ledger account balance covers both the send and its declared fee.
        add(
            ledger,
            b"balanceOf(bytes32,bytes32)",
            &[main, asset].concat(),
            word(2_000_000_000_000).to_vec(),
        );
        add(
            ledger,
            b"balanceOf(bytes32,bytes32)",
            &[record.budget_account, asset].concat(),
            word(record.remaining()).to_vec(),
        );
        calls
    }

    fn native_send_commit_time(height: u64, head_seconds: i64) -> (i64, String) {
        if height <= HEAD - 3 {
            let before_boundary = HEAD - 3 - height;
            let second_of_hour = 19 * 60 + 3 - before_boundary;
            (
                head_seconds - 60_001 - must(i64::try_from(before_boundary)),
                format!(
                    "2026-09-13T13:{:02}:{:02}Z",
                    second_of_hour / 60,
                    second_of_hour % 60
                ),
            )
        } else {
            let (offset, timestamp) = match height {
                h if h == HEAD - 2 => (-61, "2026-09-14T05:58:03Z"),
                h if h == HEAD - 1 => (-11, "2026-09-14T05:58:53Z"),
                h if h == HEAD => (0, "2026-09-14T05:59:04Z"),
                _ => (1, "2026-09-14T05:59:05Z"),
            };
            (head_seconds + offset, timestamp.to_owned())
        }
    }

    fn recorded_header_digest(h: &Value, name: &str) -> [u8; 32] {
        must(hex::decode(text(h, name)))
            .try_into()
            .or_fail("recorded header digest")
    }

    fn native_send_history_commits(
        recorded: &Value,
        validators: &Value,
        _upstream: &str,
    ) -> BTreeMap<u64, Value> {
        use layerx_agentd::read::{
            BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
        };
        let verifier = layerx_client::rpc::ChainProofVerifier {
            chain_id: "125".to_owned(),
            validators: must(layerx_client::rpc::commit::decode_validators(validators)),
        };
        let original = must(layerx_client::rpc::commit::verify_commit_material(
            &recorded["signedHeader"],
            &verifier,
        ));
        let key = ed25519_dalek::SigningKey::from_bytes(&[1; 32]);
        let public_key = key.verifying_key().to_bytes();
        assert_eq!(verifier.validators.validators().len(), 1);
        assert_eq!(verifier.validators.validators()[0].public_key, public_key);
        let validator = CommitValidator {
            address: CommitValidator::address_for_key(&public_key),
            public_key,
            voting_power: verifier.validators.validators()[0].voting_power,
        };
        let h = &recorded["signedHeader"]["header"];
        let empty_results = <[u8; 32]>::from(Sha256::digest([]));
        let head_seconds = original.time_ms() / 1000;
        assert_eq!(h["time"], "2026-09-14T05:59:04Z");
        let mut parent: [u8; 32] = must(hex::decode(text(&h["last_block_id"], "hash")))
            .try_into()
            .or_fail("recorded predecessor");
        let mut commits = BTreeMap::new();
        for height in 1..=HEAD + 1 {
            let (seconds, timestamp) = native_send_commit_time(height, head_seconds);
            let header = CheckpointHeader {
                version_block: 11,
                version_app: 1,
                chain_id: "125".to_owned(),
                height,
                time: CommitTimestamp { seconds, nanos: 0 },
                last_block_id: Some(BlockId {
                    hash: parent,
                    part_set_total: 1,
                    part_set_hash: [2; 32],
                }),
                last_commit_hash: Some(recorded_header_digest(h, "last_commit_hash")),
                data_hash: Some(recorded_header_digest(h, "data_hash")),
                validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
                next_validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
                consensus_hash: recorded_header_digest(h, "consensus_hash"),
                app_hash: original.app_hash(),
                last_results_hash: Some(empty_results),
                evidence_hash: Some(recorded_header_digest(h, "evidence_hash")),
                proposer_address: validator.address,
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
            let signature = key
                .sign(&commit.vote_sign_bytes("125", header.time))
                .to_bytes();
            commit.votes.push(CommitVote {
                validator_address: validator.address,
                voting_power: validator.voting_power,
                timestamp: header.time,
                signature,
            });
            let checkpoint = must(layerx_agentd::read::checkpoint(
                height,
                "125",
                &header,
                &commit,
                &[validator],
            ));
            let mut signed = recorded["signedHeader"].clone();
            signed["header"]["height"] = json!(height.to_string());
            signed["header"]["time"] = json!(timestamp);
            signed["header"]["last_block_id"]["hash"] = json!(hex::encode(parent));
            signed["header"]["last_results_hash"] = json!(hex::encode(empty_results));
            signed["commit"]["height"] = json!(height.to_string());
            signed["commit"]["block_id"]["hash"] = json!(hex::encode(checkpoint.block_hash));
            signed["commit"]["signatures"][0]["timestamp"] = json!(timestamp);
            signed["commit"]["signatures"][0]["signature"] =
                json!(base64::engine::general_purpose::STANDARD.encode(signature));
            let verified_head = must(layerx_client::rpc::commit::verify_commit_material(
                &signed, &verifier,
            ));
            assert_eq!(verified_head.header_hash(), checkpoint.block_hash);
            assert_eq!(verified_head.app_hash(), original.app_hash());
            assert_eq!(verified_head.time_ms(), seconds * 1000);
            parent = checkpoint.block_hash;
            assert!(commits.insert(height, signed).is_none());
        }
        assert_eq!(commits.len(), must(usize::try_from(HEAD + 1)));
        commits
    }

    fn native_policy_file(
        fixture: &Fixture,
        restart: &str,
        asset: [u8; 32],
        account: [u8; 32],
    ) -> (TenantId, std::path::PathBuf) {
        let commitment = [11_u8; 32];
        let destination = must(layerx_intents::canonical::account_id_for_protocol(
            &must(AccountId::parse(&format!(
                "agent:did:layerx:{}:main",
                hex::encode([9; 32])
            ))),
            3,
        ));
        let policy = json!({
            "schema": "layerx.native-effect-policy.v1", "version": "native-send-test-v1",
            "network_id": 125, "step_limit": 100,
            "rules": [
                {"id": "operation", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
                 "scope": {"kind": "Operation"}, "required_approval": fixture.approval, "purpose_commitment": commitment},
                {"id": "transfer", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
                 "scope": {"kind": "Transfer", "from": account, "to": destination,
                           "asset": asset, "maximum_amount": "10"},
                 "required_approval": fixture.approval, "purpose_commitment": commitment},
                {"id": "fee", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
                 "scope": {"kind": "Fee", "source_account": account, "asset": asset,
                           "maximum_amount": "1000000000000"},
                 "required_approval": fixture.approval, "purpose_commitment": commitment}
            ]
        });
        let policy_tenant = must(TenantId::new(fixture.peer.transport_tenant().to_owned()));
        let policy_path = fixture
            .root
            .join(format!("native-effect-policy-{restart}.json"));
        must(fs::write(
            &policy_path,
            must(serde_json::to_vec(&json!({
                "version": "layerx.native-effect-policy-source.v1",
                "tenant": policy_tenant.as_str(), "policy": policy.to_string(),
            }))),
        ));
        must(fs::set_permissions(
            &policy_path,
            fs::Permissions::from_mode(0o600),
        ));
        (policy_tenant, policy_path)
    }

    fn native_limit_and_denominations(
        fixture: &Fixture,
        restart: &str,
        asset: [u8; 32],
        account: [u8; 32],
    ) -> (LimitConfig, std::path::PathBuf) {
        let limit = LimitConfig {
            id: LimitId([12; 16]),
            name: "preview".to_owned(),
            scope: LimitScope::Tenant(
                Sha256::digest(fixture.credential.tenant().as_str().as_bytes()).into(),
            ),
            ceiling: u128::MAX,
            consumed: 0,
        };
        let stable_id = layerx_agentd::budget::enrolment_limit_id(
            fixture.credential.tenant(),
            &limit.scope,
            &limit.id,
        );
        let denominations_path = fixture
            .root
            .join(format!("native-send-denominations-{restart}.json"));
        must(fs::write(
            &denominations_path,
            must(serde_json::to_vec(&json!({
                "version": 1, "tenant": fixture.credential.tenant().as_str(),
                "limits": [{"stable_id": hex::encode(stable_id),
                    "asset": hex::encode(asset), "source": hex::encode(account)}]
            }))),
        ));
        must(fs::set_permissions(
            &denominations_path,
            fs::Permissions::from_mode(0o600),
        ));
        (limit, denominations_path)
    }

    fn restart_authority(fixture: &Fixture) -> (RemoteHumanAuthority, Peer) {
        let public_key = fixture.account.layerx_public_key();
        let activity = must(ActivityType::new(ModuleId::Asset, 5));
        let (authority, authority_worker) = authority_peer(
            fixture.peer.clone(),
            CoreIdentity {
                canonical_bytes: public_key.to_vec(),
                head_sequence: HEAD,
                revocation_sequence: 1,
                verification_level: layerx_types::verify::VerificationLevel::COMMITTED,
                frozen: false,
                authorities: vec![
                    ProtocolAuthority::PrimaryKey(public_key),
                    ProtocolAuthority::SessionKey(GRANT),
                ],
            },
            activity,
        );
        (authority, authority_worker)
    }

    fn restart_session_keys(fixture: &Fixture, restart: &str) -> SessionKeyRegistry {
        let keys = fixture.root.join(format!("restart-keys-{restart}"));
        must(fs::create_dir(&keys));
        must(fs::set_permissions(
            &keys,
            fs::Permissions::from_mode(0o700),
        ));
        must(SessionKeyRegistry::open(
            keys,
            random::<32>().to_vec(),
            125,
            must(fs::metadata(&fixture.root)).uid(),
        ))
    }

    fn reopen_daemon(fixture: &mut Fixture) {
        let retained = must(fixture.owner.lock()).session_control.store();
        {
            let mut store = must(retained.lock());
            let replacement = must(Store::open(fixture.root.join("closed-store")));
            drop(std::mem::replace(&mut *store, replacement));
            *store = must(Store::open(fixture.root.join("store")));
        }
        let (authority, authority_worker) = restart_authority(fixture);
        let restart = hex::encode(random::<4>());
        let clock_root = fixture.root.join(format!("restart-clock-{restart}"));
        must(fs::create_dir(&clock_root));
        must(fs::set_permissions(
            &clock_root,
            fs::Permissions::from_mode(0o700),
        ));
        let config = must(NodeRpcConfig::from_environment());
        let endpoint = must(SequencerEndpoint::new(&config.url, &[]));
        let checkpoint = must(
            layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
                &endpoint, HEAD, None, "125",
            ),
        );
        let core_time_ms = must(u64::try_from(checkpoint.header.time.seconds))
            .checked_mul(1000)
            .or_fail("checkpoint time in milliseconds");
        let (clock, clock_worker) = clock_peer_at(&clock_root, core_time_ms);
        let trust = must(layerx_agentd::export::ExportTrustSource::new(
            must(endpoint.receipt_keys()),
            checkpoint.validators,
        ));
        let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
        let account = must(layerx_wire::hash::account_id_for_protocol(
            &must(AccountId::for_asset(&fixture.account.did(), asset, asset)),
            3,
        ));
        let (policy_tenant, policy_path) = native_policy_file(fixture, &restart, asset, account);
        let peers = BTreeMap::from([(
            7,
            ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
        )]);
        let mut operations = must(ProductionHumanOperations::new(
            authority,
            must(SequencerEndpoint::new(&config.url, &[])),
            Box::new(must(NodeRpc::connect(&config))),
            Arc::clone(&retained),
            &peers,
            1_048_576,
            30_000,
            clock,
        ));
        must(
            operations
                .attach_native_effect_policies(&BTreeMap::from([(policy_tenant, policy_path)])),
        );
        let (limit, denominations_path) =
            native_limit_and_denominations(fixture, &restart, asset, account);
        must(
            operations.attach_program_budget_denominations(&BTreeMap::from([(
                fixture.credential.tenant().clone(),
                denominations_path,
            )])),
        );
        operations.install_export_trust(trust, Duration::from_secs(3));
        must(operations.authorize_subject(&fixture.peer));
        operations.recover_tenants(std::slice::from_ref(&fixture.peer), u128::MAX);
        assert!(matches!(
            operations.write_admission(&fixture.peer.tenant),
            Some(Ok(()))
        ));
        let registry = restart_session_keys(fixture, &restart);
        let mut owner = must(UnifiedAgentOwner::new(
            operations,
            retained,
            &peers,
            vec![limit],
            registry,
        ));
        layerx_agentd::degraded::enter(
            &mut owner.degraded,
            layerx_agentd::degraded::Observation::Ready {
                reference: layerx_agentd::degraded::Reference {
                    head_sequence: HEAD,
                    checkpoint: [0; 32],
                },
                maximum_rung: layerx_types::rung::Rung::Submitted,
            },
        );
        fixture.owner = must(SharedAgentOwner::new(owner).with_idempotency(
            fixture.root.join("rpc-idempotency"),
            1000,
            1000,
        ));
        fixture.peers.extend([authority_worker, clock_worker]);
    }

    const RP_ID: &str = "paxportwallet.com";
    const ORIGIN: &str = "https://paxportwallet.com";
    const FLAG_UP: u8 = 1;
    const FLAG_UV: u8 = 1 << 2;
    const FLAG_AT: u8 = 1 << 6;

    fn required<T, E: Display>(result: Result<T, E>, label: &str) -> T {
        result.unwrap_or_else(|error| panic!("{label}: {error}"))
    }

    fn auth_config() -> AuthConfig {
        AuthConfig {
            rp_id: RP_ID.to_owned(),
            rp_name: "LayerX".to_owned(),
            origin: ORIGIN.to_owned(),
            ceremony_ttl_secs: 300,
            assertion_ttl_secs: 30,
            session_ttl_secs: 60,
            refresh_ttl_secs: 300,
            step_up_ttl_secs: 10,
            rate_limit: RateLimit {
                attempts: 100,
                window_secs: 60,
            },
        }
    }

    fn decode_ceremony(value: &str) -> Value {
        let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
        required(serde_json::from_slice(&bytes), "parse ceremony")
    }

    fn required_text<'a>(value: &'a Value, pointer: &str) -> &'a str {
        value
            .pointer(pointer)
            .and_then(Value::as_str)
            .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
    }

    fn encode_response(value: &Value) -> String {
        URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
    }

    struct SoftwareAuthenticator {
        signing_key: SigningKey,
        credential_id: Vec<u8>,
        counter: u32,
        user_handle: Option<String>,
    }

    impl SoftwareAuthenticator {
        fn new() -> Self {
            let seed = [0x73_u8; 32];
            let credential_id = vec![0x74_u8; 32];
            Self {
                signing_key: SigningKey::from_bytes(&seed),
                credential_id,
                counter: 0,
                user_handle: None,
            }
        }

        fn register(&mut self, ceremony: &str) -> String {
            let options = decode_ceremony(ceremony);
            let challenge = required_text(&options, "/challenge");
            self.user_handle = Some(required_text(&options, "/user/id").to_owned());
            let client_data = client_data("webauthn.create", challenge);
            encode_response(&json!({
                "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
                "transports": ["internal"],
                "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
                "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            }))
        }

        fn assert(&mut self, ceremony: &str) -> String {
            let options = decode_ceremony(ceremony);
            let challenge = required_text(&options, "/challenge");
            self.counter = self.counter.saturating_add(1);
            let authenticator_data = self.authenticator_data(self.counter, false);
            let client_data = client_data("webauthn.get", challenge);
            let client_hash = Sha256::digest(&client_data);
            let mut signed = Vec::with_capacity(authenticator_data.len() + client_hash.len());
            signed.extend_from_slice(&authenticator_data);
            signed.extend_from_slice(&client_hash);
            let signature = self.signing_key.sign(&signed).to_bytes();
            encode_response(&json!({
                "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
                "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
                "signature": URL_SAFE_NO_PAD.encode(signature),
                "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
                "userHandle": self.user_handle,
            }))
        }

        fn attestation_object(&self) -> Vec<u8> {
            let map = CborValue::Map(vec![
                (
                    CborValue::Text("fmt".to_owned()),
                    CborValue::Text("none".to_owned()),
                ),
                (
                    CborValue::Text("attStmt".to_owned()),
                    CborValue::Map(Vec::new()),
                ),
                (
                    CborValue::Text("authData".to_owned()),
                    CborValue::Bytes(self.authenticator_data(0, true)),
                ),
            ]);
            let mut bytes = Vec::new();
            required(
                ciborium::ser::into_writer(&map, &mut bytes),
                "encode attestation",
            );
            bytes
        }

        fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
            let mut bytes = Vec::new();
            bytes.extend_from_slice(&Sha256::digest(RP_ID.as_bytes()));
            bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
            bytes.extend_from_slice(&counter.to_be_bytes());
            if attested {
                bytes.extend_from_slice(&[0_u8; 16]);
                let credential_length = u16::try_from(self.credential_id.len())
                    .unwrap_or_else(|_| panic!("credential identifier too long"));
                bytes.extend_from_slice(&credential_length.to_be_bytes());
                bytes.extend_from_slice(&self.credential_id);
                bytes.extend_from_slice(&self.cose_public_key());
            }
            bytes
        }

        fn cose_public_key(&self) -> Vec<u8> {
            let public_key = self.signing_key.verifying_key().to_bytes().to_vec();
            let map = CborValue::Map(vec![
                (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
                (
                    CborValue::Integer(3.into()),
                    CborValue::Integer((-8).into()),
                ),
                (
                    CborValue::Integer((-1).into()),
                    CborValue::Integer(6.into()),
                ),
                (
                    CborValue::Integer((-2).into()),
                    CborValue::Bytes(public_key),
                ),
            ]);
            let mut bytes = Vec::new();
            required(
                ciborium::ser::into_writer(&map, &mut bytes),
                "encode public key",
            );
            bytes
        }
    }

    fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
        required(
            serde_json::to_vec(&json!({
                "type": kind,
                "challenge": challenge,
                "origin": ORIGIN,
                "crossOrigin": false,
            })),
            "encode client data",
        )
    }

    fn register_session(
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        authenticator: &mut SoftwareAuthenticator,
        now: u64,
    ) -> SessionGrant {
        let identity = required(
            AccountIdentity::new("mara@example.com", "Mara"),
            "account identity",
        );
        let registration = required(
            passkeys.begin_registration(scope, &identity, "Phone passkey", now),
            "begin registration",
        );
        let response = authenticator.register(&registration.ceremony);
        required(
            passkeys.finish_registration(scope, &registration.registration_id, &response, now + 1),
            "finish registration",
        );
        open_session(
            passkeys,
            scope,
            authenticator,
            "dev_aabbccddeeff00112233445566778891",
            now + 2,
        )
    }

    fn open_session(
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        authenticator: &mut SoftwareAuthenticator,
        device_id: &str,
        now: u64,
    ) -> SessionGrant {
        let challenge = required(passkeys.begin_assertion(scope, now), "begin assertion");
        let response = authenticator.assert(&challenge.ceremony);
        required(
            passkeys.finish_assertion(scope, &challenge.assertion_id, &response, now + 1),
            "finish assertion",
        );
        let device = required(Device::new(device_id, "Phone", "mobile"), "device");
        required(
            passkeys.open_session(scope, &challenge.assertion_id, device, now + 2),
            "open session",
        )
    }

    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use ciborium::value::Value as CborValue;
    use ed25519_dalek::SigningKey;
    use layerx_agent_api::identity::{AgentDid, AuthorityRef};
    use layerx_agent_api::prepare::{IdempotencyRef, PayloadBytes, PrepareRequest, TimestampBound};
    use layerx_human_service::auth::{
        AccountIdentity, AuthConfig, Device, Passkeys, RateLimit, SessionGrant,
    };
    use layerx_human_service::custody::{
        CustodySigner, KeyClass, KeyId, Keystore, NativeConsent, NativeConsentRequest, Operation,
        SignAuthorization, SigningLimits,
    };
    use layerx_human_service::journeys::{
        JourneyEngine, JourneyKind, JourneyLeg, JourneyPhase, JourneyPlan,
    };
    use layerx_human_service::notify::JourneyId;
    use layerx_human_service::server::agent_runtime::{
        AgentRuntime, NativeEffectApprovalFactState,
    };
    use layerx_human_service::server::native_send::{
        access_operation, authorize_access, load_access, preview_send_v2, unsigned_access,
        HumanOwnerNativeContextV1, HumanOwnerNativeSessionV1, NativeSendAccessRequest,
    };
    use layerx_human_service::server::production_components::{AttestorCustodyConfig, AttestorKms};
    use layerx_human_service::store::{PrincipalId, PrincipalScope, PrincipalStore, RowKey, Table};
    use layerx_human_service::trace::TraceId;
    use layerx_human_test_support::{install_and_open, retention_uniform, tenancy};
    use layerx_intents::{Intent, IntentKind, LxpSend};

    use layerx_types::intent::{
        AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey,
        SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
    };
    use layerx_types::payload::{ModuleRegistration, ModuleRegistry};
    use layerx_types::{
        amount::Amount,
        ids::{AssetId, IdempotencyKey},
    };
    use std::fmt::Display;
    use std::future::Future;
    use std::task::{Context, Poll, Wake, Waker};

    const HEAD: u64 = 64;
    const GRANT: [u8; 32] = [0x32; 32];
    const EXPIRY_SECONDS: u64 = 1_789_365_574;
    const EXPIRY_MS: u64 = EXPIRY_SECONDS * 1_000;
    static SERIAL: Mutex<()> = Mutex::new(());

    mod hex {
        pub fn encode(bytes: impl AsRef<[u8]>) -> String {
            layerx_human_service::hex::lower(bytes.as_ref())
        }
        pub fn decode(bytes: impl AsRef<[u8]>) -> Result<Vec<u8>, String> {
            let bytes = bytes.as_ref();
            if bytes.len() % 2 != 0 {
                return Err("odd hex length".into());
            }
            bytes
                .chunks_exact(2)
                .map(|pair| {
                    u8::from_str_radix(std::str::from_utf8(pair).map_err(|e| e.to_string())?, 16)
                        .map_err(|e| e.to_string())
                })
                .collect()
        }
    }
    struct Ready;
    impl Wake for Ready {
        fn wake(self: Arc<Self>) {}
    }
    fn ready<F: Future>(future: F) -> F::Output {
        let mut f = std::pin::pin!(future);
        match f
            .as_mut()
            .poll(&mut Context::from_waker(&Waker::from(Arc::new(Ready))))
        {
            Poll::Ready(value) => value,
            Poll::Pending => panic!("local production operation yielded"),
        }
    }
    struct PublicAccount {
        seed: [u8; 32],
        sender: [u8; 20],
    }
    impl PublicAccount {
        fn new(marker: u8) -> Self {
            let seed = [marker; 32];
            let probe = layerx_crypto::evm_transaction::Eip1559Call {
                chain_id: 125,
                nonce: 9,
                max_priority_fee_per_gas: 0,
                max_fee_per_gas: 1,
                gas_limit: 100_000,
                to: [1; 20],
                data: vec![],
            };
            let (call, signature) = must(layerx_crypto::evm_transaction::decode_signed(&must(
                probe.sign(&seed),
            )));
            Self {
                seed,
                sender: must(layerx_crypto::evm_transaction::recover_sender(
                    &call, &signature,
                )),
            }
        }
        fn layerx_seed(&self) -> &[u8; 32] {
            &self.seed
        }
        fn layerx_public_key(&self) -> [u8; 32] {
            SigningKey::from_bytes(&self.seed)
                .verifying_key()
                .to_bytes()
        }
        fn did(&self) -> String {
            format!("did:layerx:{}", hex::encode(self.layerx_public_key()))
        }
        fn evm_address(&self) -> [u8; 20] {
            self.sender
        }
    }
    struct Fixture {
        owner: SharedAgentOwner<RemoteHumanAuthority>,
        peer: HumanPeer,
        credential: session::SessionCredential,
        account: PublicAccount,
        peers: Vec<Peer>,
        root: PathBuf,
        approval: bool,
        previous_rpc: Option<std::ffi::OsString>,
        previous_chain: Option<std::ffi::OsString>,
        previous_comet: Option<std::ffi::OsString>,
        clock: Arc<RuntimeClock>,
        previous_clock: Vec<(&'static str, Option<std::ffi::OsString>)>,
    }
    fn initial_subject(
        root: &std::path::Path,
        account: &PublicAccount,
    ) -> (
        TenantId,
        HumanPeer,
        session::SessionCredential,
        RemoteHumanAuthority,
        Peer,
        Arc<Mutex<Store>>,
    ) {
        let public_key = account.layerx_public_key();
        let principal = hex::encode(public_key);
        let tenant = must(TenantId::new(must(
            layerx_identity_binding::subject_namespace("bootstrap-service", &principal),
        )));
        let peer = HumanPeer {
            uid: 7,
            principal,
            tenant: tenant.as_str().to_owned(),
            subject: Some(HumanSubject {
                transport_tenant: "bootstrap-service".into(),
                transport_principal: "bootstrap-owner".into(),
                owner: account.did(),
                account: format!("agent:{}:main", account.did()),
                asset: layerx_crypto::evm_transaction::transaction_hash(b"upax"),
                registration: None,
            }),
        };
        let activity = must(ActivityType::new(ModuleId::Asset, 5));
        let identity = CoreIdentity {
            canonical_bytes: public_key.to_vec(),
            head_sequence: HEAD,
            revocation_sequence: 1,
            verification_level: layerx_types::verify::VerificationLevel::COMMITTED,
            frozen: false,
            authorities: vec![
                ProtocolAuthority::PrimaryKey(public_key),
                ProtocolAuthority::SessionKey(GRANT),
            ],
        };
        let (mut authority, authority_worker) = authority_peer(peer.clone(), identity, activity);
        let did = must(Did::new(account.did().as_bytes()));
        let mut store = must(Store::open(root.join("store")));
        let resolved = must(identity::register(
            &mut store,
            tenant.clone(),
            did.clone(),
            &mut AuthorityIdentity {
                authority: &mut authority,
                peer: &peer,
            },
        ));
        let credential = must(session::open(
            &mut store,
            &mut SessionRegistry::default(),
            &resolved,
            OpenRequest {
                session_id: SessionId([8; 32]),
                token_id: [0x71; 32],
                tenant: tenant.clone(),
                agent: did,
                authority: ProtocolAuthority::SessionKey(GRANT),
                permitted_activity_types: BTreeSet::from([5]),
                scopes: BTreeSet::from(["prepare".into()]),
                expiry_sequence: 1000,
                expiry_seconds: Some(EXPIRY_SECONDS),
                opening_client: "human".into(),
                policy_version: "v1".into(),
            },
            HEAD,
        ))
        .credential();
        let retained = Arc::new(Mutex::new(store));
        (
            tenant,
            peer,
            credential,
            authority,
            authority_worker,
            retained,
        )
    }

    impl Fixture {
        fn new(approval: bool) -> Self {
            let root = directory(if approval {
                "approval-clock"
            } else {
                "producer-clock"
            });
            let account = PublicAccount::new(if approval { 0x51 } else { 0x61 });
            let (tenant, peer, credential, authority, authority_worker, retained) =
                initial_subject(&root, &account);
            let (clock, clock_worker) = clock_peer_at(&root, (EXPIRY_SECONDS - 30) * 1000);
            let previous_clock = [
                (
                    "LAYERX_RUNTIME_CLOCK_SOCKET",
                    root.join("clock.sock").into_os_string(),
                ),
                (
                    "LAYERX_RUNTIME_CLOCK_PID",
                    std::process::id().to_string().into(),
                ),
                (
                    "LAYERX_RUNTIME_CLOCK_UID",
                    must(fs::metadata(&root)).uid().to_string().into(),
                ),
            ]
            .into_iter()
            .map(|(name, value)| {
                let previous = std::env::var_os(name);
                std::env::set_var(name, value);
                (name, previous)
            })
            .collect();
            let binding = Binding::new(125, account.evm_address(), 3).sign(account.layerx_seed());
            let (url, node_worker) = recorded_chain(&account, binding);
            let peers = BTreeMap::from([(
                7,
                ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
            )]);
            let mut operations = must(ProductionHumanOperations::new(
                authority,
                must(SequencerEndpoint::new(&url, &[])),
                Box::new(must(NodeRpc::connect(&NodeRpcConfig {
                    url: url.clone(),
                    chain_id: 125,
                    deadline: Duration::from_secs(3),
                }))),
                retained.clone(),
                &peers,
                1_048_576,
                30_000,
                clock.clone(),
            ));
            must(operations.authorize_subject(&peer));
            let keys = root.join("initial-keys");
            must(fs::create_dir(&keys));
            must(fs::set_permissions(
                &keys,
                fs::Permissions::from_mode(0o700),
            ));
            let registry = must(SessionKeyRegistry::open(
                keys,
                vec![0x72; 32],
                125,
                must(fs::metadata(&root)).uid(),
            ));
            let owner = SharedAgentOwner::new(must(UnifiedAgentOwner::new(
                operations,
                retained,
                &peers,
                vec![LimitConfig {
                    id: LimitId([12; 16]),
                    name: "native clock regression".to_owned(),
                    scope: LimitScope::Tenant(Sha256::digest(tenant.as_str().as_bytes()).into()),
                    ceiling: u128::MAX,
                    consumed: 0,
                }],
                registry,
            )));
            let previous_rpc = std::env::var_os("LAYERX_AGENT_NODE_RPC_URL");
            let previous_chain = std::env::var_os("LAYERX_AGENT_NODE_CHAIN_ID");
            let previous_comet = std::env::var_os("LAYERX_AGENT_NODE_COMET_RPC_URL");
            std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
            std::env::set_var("LAYERX_AGENT_NODE_COMET_RPC_URL", &url);
            std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "125");
            let mut fixture = Self {
                owner,
                peer,
                credential,
                account,
                peers: vec![authority_worker, clock_worker, node_worker],
                root,
                approval,
                previous_rpc,
                previous_chain,
                previous_comet,
                clock,
                previous_clock,
            };
            reopen_daemon(&mut fixture);
            fixture
        }
        fn registry() -> ModuleRegistry {
            must(ModuleRegistry::new(&[must(ModuleRegistration::new(
                ModuleId::Asset,
                &[must(ActivityType::new(ModuleId::Asset, 5))],
            ))]))
        }
        fn client(&self) -> (AgentRuntime, Peer) {
            let socket = self
                .root
                .join(format!("agent-{}.sock", hex::encode(random::<4>())));
            let listener = must(UnixListener::bind(&socket));
            must(listener.set_nonblocking(true));
            let mut owner = self.owner.clone();
            let peer = HumanPeer {
                uid: self.peer.uid,
                principal: self
                    .peer
                    .subject
                    .as_ref()
                    .or_fail("owner subject")
                    .transport_principal
                    .clone(),
                tenant: self.peer.transport_tenant().to_owned(),
                subject: None,
            };
            let connection = Arc::new(Mutex::new(ConnectionState::default()));
            let accepted = Arc::clone(&connection);
            let mut worker = Peer::spawn(move |stop| {
                while !stop.load(Ordering::Acquire) {
                    match listener.accept() {
                        Ok((stream, _)) => {
                            must(accepted.lock()).stream = Some(must(stream.try_clone()));
                            let mut transport = ObservedTransport {
                                inner: must(AcceptedTransport::new(
                                    stream,
                                    1_048_576,
                                    Duration::from_secs(5),
                                )),
                                opcode: None,
                                connection: Arc::clone(&accepted),
                            };
                            while !stop.load(Ordering::Acquire) {
                                match serve_one(&mut transport, &peer, &mut owner) {
                                    Ok(()) => {}
                                    Err(layerx_agentd::human::HumanProtocolError::Transport(
                                        layerx_agentd::human::frame::TransportError::ConnectionFailure(
                                            std::io::ErrorKind::UnexpectedEof,
                                        ),
                                    )) => break,
                                    Err(_) if stop.load(Ordering::Acquire) => break,
                                    Err(error) => panic!("daemon frame: {error:?}"),
                                }
                            }
                            must(accepted.lock()).stream = None;
                        }
                        Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(e) => panic!("daemon transport: {e}"),
                    }
                }
            });
            worker.connection = Some(connection);
            let mut runtime = must(
                must(AgentRuntime::new_with_clock(
                    &socket,
                    layerx_human_service::frame::Limits {
                        maximum_frame_bytes: 1_048_576,
                        maximum_connections: 1,
                        maximum_streams: 1,
                        maximum_queued_bytes: 1_048_576,
                        deadline: Duration::from_secs(5),
                    },
                    Self::registry(),
                    self.clock.clone(),
                ))
                .for_subject(
                    &must(PrincipalId::new(&self.peer.principal)),
                    &must(Did::new(self.account.did().as_bytes())),
                    &must(AccountId::parse(
                        &self
                            .peer
                            .subject
                            .as_ref()
                            .unwrap_or_else(|| panic!("missing value"))
                            .account,
                    )),
                    self.peer
                        .subject
                        .as_ref()
                        .unwrap_or_else(|| panic!("missing value"))
                        .asset,
                ),
            );
            must(
                layerx_human_service::journeys::AgentBoundary::bind_transaction_sender(
                    &mut runtime,
                    self.account.evm_address(),
                ),
            );
            (runtime, worker)
        }
        fn custody(&self) -> ClockCustody {
            ClockCustody::new(self)
        }
    }
    impl Drop for Fixture {
        fn drop(&mut self) {
            self.peers.clear();
            for (name, value) in [
                ("LAYERX_AGENT_NODE_RPC_URL", &self.previous_rpc),
                ("LAYERX_AGENT_NODE_CHAIN_ID", &self.previous_chain),
                ("LAYERX_AGENT_NODE_COMET_RPC_URL", &self.previous_comet),
            ] {
                match value {
                    Some(v) => std::env::set_var(name, v),
                    None => std::env::remove_var(name),
                }
            }
            for (name, value) in &self.previous_clock {
                match value {
                    Some(value) => std::env::set_var(name, value),
                    None => std::env::remove_var(name),
                }
            }
            let _ = fs::remove_dir_all(&self.root);
        }
    }

    struct ClockCustody {
        signer: CustodySigner,
        provider: AttestorKms,
        assertion: String,
        owner: String,
        _peers: Vec<Peer>,
    }

    impl std::ops::Deref for ClockCustody {
        type Target = CustodySigner;

        fn deref(&self) -> &Self::Target {
            &self.signer
        }
    }

    fn attestor_tls() -> (Arc<rustls::ServerConfig>, Vec<u8>, Vec<u8>, Vec<u8>) {
        use rustls::pki_types::{CertificateDer, PrivatePkcs8KeyDer};
        use rustls::{RootCertStore, ServerConfig};

        let ca = include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-human-kms/src/fixtures/repair_22_33_25/ca.der"
        ));
        let cert = include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-human-kms/src/fixtures/repair_22_33_25/peer.der"
        ));
        let tls_key = include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-human-kms/src/fixtures/repair_22_33_25/peer.pk8"
        ));
        let mut roots = RootCertStore::empty();
        must(roots.add(CertificateDer::from(ca.to_vec())));
        let verifier = must(rustls::server::WebPkiClientVerifier::builder(Arc::new(roots)).build());
        let tls = Arc::new(must(
            ServerConfig::builder()
                .with_client_cert_verifier(verifier)
                .with_single_cert(
                    vec![CertificateDer::from(cert.to_vec())],
                    PrivatePkcs8KeyDer::from(tls_key.to_vec()).into(),
                ),
        ));
        (tls, ca.to_vec(), cert.to_vec(), tls_key.to_vec())
    }

    fn clock_owner_assertion(fixture: &Fixture, owner: &str) -> String {
        let header = base64::engine::general_purpose::URL_SAFE_NO_PAD
            .encode(br#"{"alg":"EdDSA","typ":"JWT"}"#);
        let claims = base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(must(serde_json::to_vec(&json!({
                "sub":owner,"iat":EXPIRY_SECONDS-60,"exp":EXPIRY_SECONDS,"jti":hex::encode([0x79;32])
            }))));
        let jwt_message = format!("{header}.{claims}");
        let assertion = format!(
            "{jwt_message}.{}",
            base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(
                SigningKey::from_bytes(fixture.account.layerx_seed())
                    .sign(jwt_message.as_bytes())
                    .to_bytes()
            )
        );
        assertion
    }

    struct ClockPeerContext {
        id: String,
        tls: Arc<rustls::ServerConfig>,
        seed: [u8; 32],
        public: [u8; 32],
        sender: [u8; 20],
        owner: String,
        provider_key: String,
        assertion: String,
        signers: Vec<String>,
    }

    fn assert_clock_owner_assertion(key: &SigningKey, headers: &str, assertion: &str, owner: &str) {
        assert!(headers
            .lines()
            .any(|line| line.eq_ignore_ascii_case(&format!("authorization: Bearer {assertion}"))));
        let (message, signature) = assertion.rsplit_once('.').or_fail("signed owner assertion");
        let signature: [u8; 64] =
            must(base64::engine::general_purpose::URL_SAFE_NO_PAD.decode(signature))
                .try_into()
                .or_fail("Ed25519 assertion signature");
        must(key.verifying_key().verify_strict(
            message.as_bytes(),
            &ed25519_dalek::Signature::from_bytes(&signature),
        ));
        let claims = must(layerx_human_kms::attestor::assertion_claims(assertion));
        assert_eq!(claims.subject, owner);
        must(claims.check(EXPIRY_SECONDS - 20));
    }

    fn clock_peer_response(
        context: &ClockPeerContext,
        key: &SigningKey,
        request: (&str, &str, &[u8]),
        sequence: u64,
        sessions: &mut BTreeMap<String, (Value, Value)>,
    ) -> Value {
        let (path, headers, body) = request;
        let id = &context.id;
        let public = context.public;
        let sender = context.sender;
        let owner = context.owner.as_str();
        let provider_key = context.provider_key.as_str();
        let assertion = context.assertion.as_str();
        let signers = &context.signers;
        if path == "/health" {
            assert!(body.is_empty());
            json!({"node_id":id,"ready":true,"audit_sequence":sequence,"reachable_peers":2})
        } else {
            let request: Value = must(serde_json::from_slice(body));
            assert_eq!(request["key_id"], provider_key);
            if path == "/v1/keys/generate" {
                assert_eq!(request["curve"], "ed25519");
                assert_eq!(request["owner"], owner);
                assert_eq!(request["account"], format!("0x{}", hex::encode(sender)));
                json!({"node_id":id,"key_id":provider_key,"curve":"ed25519","public_key":hex::encode(public),"audit_sequence":sequence})
            } else {
                assert_clock_owner_assertion(key, headers, assertion, owner);
                if path == "/v1/keys/public-wallet" {
                    assert_eq!(request["owner"], owner);
                    assert_eq!(request["public_key"], hex::encode(public));
                    let evm = must(k256::ecdsa::SigningKey::from_slice(&context.seed));
                    json!({"node_id":id,"key_id":provider_key,"public_key":hex::encode(public),"owner":owner,"wallet_key_id":"clock-wallet","wallet_public_key":hex::encode(evm.verifying_key().to_encoded_point(false).as_bytes()),"wallet_epoch":1,"address":format!("0x{}",hex::encode(sender)),"audit_sequence":sequence})
                } else {
                    assert_eq!(path, "/v2/sign/native");
                    assert_eq!(request["profile"], 2);
                    assert_eq!(request["signers"], json!(signers));
                    let canonical = match text(&request, "operation") {
                        "local-grant-consent" => {
                            let grant = layerx_agent_api::identity::NativeLocalGrantConsentV1 {
                                capability: must(hex::decode(text(&request, "capability"))),
                                session_scope: must(hex::decode(text(&request, "native_session"))),
                                expires_at_ms: must(text(&request, "expiry_ms").parse()),
                                owner_public_key: public,
                                signature: [0; 64],
                            };
                            let coordinates = must(
                                layerx_human_kms::attestor::validate_native_local_grant(&grant),
                            );
                            assert_eq!(coordinates.tenant, fixture_tenant(&public));
                            assert_eq!(coordinates.agent, owner);
                            must(
                                NativeConsent::LocalGrant(&grant)
                                    .validate_at((EXPIRY_SECONDS - 20) * 1000, public),
                            );
                            must(
                                layerx_human_kms::attestor::native_local_grant_signing_bytes(
                                    &grant,
                                ),
                            )
                        }
                        "send-purpose" => {
                            let canonical = must(hex::decode(text(&request, "purpose")));
                            let purpose = must(layerx_agent_api::identity::NativeSendPurposeV2::from_canonical_bytes(&canonical));
                            assert_eq!(purpose.owner_did.as_str(), owner);
                            assert_eq!(purpose.consent_public_key, public);
                            assert_eq!(purpose.evm_sender, sender);
                            assert_eq!(purpose.expires_at_ms, EXPIRY_MS);
                            must(
                                NativeConsent::SendPurpose(&purpose)
                                    .validate_at(EXPIRY_MS - 1, public),
                            );
                            assert_eq!(must(purpose.canonical_bytes()), canonical);
                            canonical
                        }
                        other => panic!("unexpected native operation: {other}"),
                    };
                    let digest: [u8; 32] = Sha256::digest(canonical).into();
                    let session = text(&request, "session_id").to_owned();
                    if let Some((previous, response)) = sessions.get(&session) {
                        assert_eq!(previous, &request);
                        response.clone()
                    } else {
                        let signature = key.sign(&digest).to_bytes();
                        must(layerx_crypto::ed25519::verify_digest(
                            &public, &signature, &digest,
                        ));
                        let response = json!({"profile":2,"node_id":id,"key_id":provider_key,"operation":request["operation"],"signed_bytes":hex::encode(digest),"signature":hex::encode(signature),"audit_sequence":sequence});
                        sessions.insert(session, (request, response.clone()));
                        response
                    }
                }
            }
        }
    }

    fn serve_clock_peer(stop: &AtomicBool, listener: &TcpListener, context: &ClockPeerContext) {
        use rustls::{ServerConnection, StreamOwned};
        let key = SigningKey::from_bytes(&context.seed);
        let mut sequence = 0;
        let mut sessions: BTreeMap<String, (Value, Value)> = BTreeMap::new();
        while !stop.load(Ordering::Acquire) {
            let Some(stream) = incoming(listener) else {
                continue;
            };
            let mut stream =
                StreamOwned::new(must(ServerConnection::new(context.tls.clone())), stream);
            let (headers, body) = http_request(&mut stream);
            let path = headers.split_whitespace().nth(1).or_fail("attestor path");
            sequence += 1;
            let response = clock_peer_response(
                context,
                &key,
                (path, &headers, &body),
                sequence,
                &mut sessions,
            );
            http_response(&mut stream, &response);
        }
    }

    impl ClockCustody {
        fn new(fixture: &Fixture) -> Self {
            let (tls, ca, cert, tls_key) = attestor_tls();
            let principal = must(PrincipalId::new(&fixture.peer.principal));
            let key = must(KeyId::new("human-primary"));
            let binding = must(layerx_human_service::custody::PrincipalKeyBinding::new(
                format!(
                    "layerx-human-custody/v1\0{}\0human-primary\0human-primary",
                    principal.as_str()
                )
                .into_bytes(),
                125,
                KeyClass::HumanPrimary,
                "attestor-quorum/v1",
            ));
            let provider_key = format!("lx-{}", hex::encode(&binding.digest()[..16]));
            let owner = fixture.account.did();
            let assertion = clock_owner_assertion(fixture, &owner);
            let mut nodes = Vec::new();
            let mut peers = Vec::new();
            let signers: Vec<String> = (1..=3).map(|i| format!("node-{i}")).collect();
            for id in &signers {
                let listener = must(TcpListener::bind("127.0.0.1:0"));
                must(listener.set_nonblocking(true));
                nodes.push((id.clone(), must(listener.local_addr())));
                let context = ClockPeerContext {
                    id: id.clone(),
                    tls: tls.clone(),
                    seed: *fixture.account.layerx_seed(),
                    public: fixture.account.layerx_public_key(),
                    sender: fixture.account.evm_address(),
                    owner: owner.clone(),
                    provider_key: provider_key.clone(),
                    assertion: assertion.clone(),
                    signers: signers.clone(),
                };
                peers.push(Peer::spawn(move |stop| {
                    serve_clock_peer(&stop, &listener, &context);
                }));
            }
            let provider = must(AttestorKms::connect_with_clock(
                must(AttestorCustodyConfig::new(
                    nodes,
                    signers,
                    ca,
                    cert,
                    tls_key,
                    Duration::from_secs(5),
                )),
                fixture.clock.clone(),
            ));
            let root = fixture.root.join("custody");
            let keys = must(Keystore::open_production(
                root.join("keys"),
                125,
                provider.clone(),
            ));
            assert_eq!(
                must(provider.create_owned_key(
                    &keys,
                    &principal,
                    &key,
                    &owner,
                    &format!("0x{}", hex::encode(fixture.account.evm_address()))
                )),
                fixture.account.layerx_public_key()
            );
            let descriptor = must(keys.describe(&principal, &key));
            assert_eq!(descriptor.class, KeyClass::HumanPrimary);
            assert_eq!(descriptor.public_key, fixture.account.layerx_public_key());
            let (audit, _) = install_and_open(
                &root.join("audit"),
                &tenancy(&[(&fixture.peer.principal, &fixture.peer.tenant)]),
                retention_uniform(10_000),
            );
            Self {
                signer: CustodySigner::new(
                    keys,
                    audit,
                    Fixture::registry(),
                    must(SigningLimits::new(100, 60)),
                ),
                provider,
                assertion,
                owner,
                _peers: peers,
            }
        }

        fn admit_owner(&self) {
            must(self.provider.admit_assertion(&self.owner, &self.assertion));
        }
    }

    fn fixture_tenant(public: &[u8; 32]) -> String {
        must(layerx_identity_binding::subject_namespace(
            "bootstrap-service",
            &hex::encode(public),
        ))
    }

    fn abi_string(bytes: &[u8]) -> Vec<u8> {
        let mut out = vec![0; 64 + bytes.len().div_ceil(32) * 32];
        out[31] = 32;
        out[56..64].copy_from_slice(&(bytes.len() as u64).to_be_bytes());
        out[64..64 + bytes.len()].copy_from_slice(bytes);
        out
    }

    fn native_call_result(request: &Value, calls: &CommittedNativeCalls) -> Value {
        let call = &request["params"][0];
        let target = text(call, "to");
        let data = must(hex::decode(
            call.get("data")
                .or_else(|| call.get("input"))
                .unwrap_or_else(|| panic!("missing value"))
                .as_str()
                .unwrap_or_else(|| panic!("missing value"))
                .trim_start_matches("0x"),
        ));
        let answer = if let Some(answer) = calls.get(&(target.to_owned(), data.clone())) {
            answer.clone()
        } else if target == "0x0000000000000000000000000000000000001018" {
            assert_eq!(
                &data[..4],
                &layerx_crypto::evm_transaction::transaction_hash(b"getFeeDenom(address)")[..4]
            );
            abi_string(b"upax")
        } else if target == "0x0000000000000000000000000000000000001001" {
            if data.starts_with(
                &layerx_crypto::evm_transaction::transaction_hash(b"symbol(string)")[..4],
            ) {
                abi_string(b"PAX")
            } else {
                assert!(data.starts_with(
                    &layerx_crypto::evm_transaction::transaction_hash(b"decimals(string)")[..4]
                ));
                let mut out = vec![0; 32];
                out[31] = 6;
                out
            }
        } else if data.starts_with(&layerx_client::account_binding::SELECTOR_LAYERX_BIND_NONCE) {
            let mut word = vec![0; 32];
            word[31] = 3;
            word
        } else {
            panic!("uncaptured native read: {request}")
        };
        json!(format!("0x{}", hex::encode(answer)))
    }

    fn recorded_chain(account: &PublicAccount, binding: SignedBinding) -> (String, Peer) {
        let recorded: Vec<Value> = must(serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../agent/crates/layerx-agentd/tests/support/human_budget_checkpoint.json"
        ))));
        let find = |method: &str| {
            recorded
                .iter()
                .find(|row| row["method"] == method)
                .unwrap_or_else(|| panic!("missing value"))["result"]
                .clone()
        };
        let validators = find("px_getValidators");
        let commits = native_send_history_commits(&find("px_getCommit"), &validators, "");
        let verifier = layerx_client::rpc::ChainProofVerifier {
            chain_id: "125".into(),
            validators: must(layerx_client::rpc::commit::decode_validators(&validators)),
        };
        let head = must(layerx_client::rpc::commit::verify_commit_material(
            &commits[&HEAD],
            &verifier,
        ));
        let calls = committed_native_send_calls(&binding);
        let evm: Value = must(serde_json::from_str(EVM_RECORDING));
        let mut block = evm["eth_getBlockByNumber"].clone();
        block["hash"] = json!(format!("0x{}", hex::encode(head.header_hash())));
        block["stateRoot"] = json!(format!("0x{}", hex::encode(head.app_hash())));
        block["timestamp"] = json!(format!("0x{:x}", head.time_ms() / 1000));
        let public = account.layerx_public_key();
        let sender = account.evm_address();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let key_id: [u8; 32] = Sha256::digest(public).into();
        let worker = Peer::spawn(move |stop| {
            while !stop.load(Ordering::Acquire) {
                let Some(mut stream) = incoming(&listener) else {
                    continue;
                };
                let (_, body) = http_request(&mut stream);
                let request: Value = must(serde_json::from_slice(&body));
                let method = text(&request, "method");
                eprintln!("clock regression node read: {method} {}", request["params"]);
                let result = match method {
                    "eth_chainId" | "eth_blockNumber" | "eth_getBalance" | "eth_estimateGas" => {
                        evm[method].clone()
                    }
                    "eth_getBlockByNumber" => block.clone(),
                    "eth_getTransactionCount" => {
                        assert_eq!(request["params"][0], format!("0x{}", hex::encode(sender)));
                        evm[method].clone()
                    }
                    "px_getCommit" => {
                        json!({"height":HEAD.to_string(),"signedHeader":commits[&HEAD]})
                    }
                    "px_getValidators" => validators.clone(),
                    "px_getReceiptKeys" => {
                        json!({"keys":[{"keyId":format!("0x{}",hex::encode(key_id)),"publicKey":format!("0x{}",hex::encode(public)),"firstHeight":1,"lastHeight":HEAD+1000}]})
                    }
                    "commit" => {
                        let height = must(text(&request["params"], "height").parse::<u64>());
                        let Some(signed) = commits.get(&height) else {
                            http_response(
                                &mut stream,
                                &json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32001,"message":"commit not found","data":{"height":height.to_string()}}}),
                            );
                            continue;
                        };
                        json!({"signed_header":signed})
                    }
                    "block_results" => {
                        json!({"height":request["params"]["height"],"txs_results":[]})
                    }
                    "px_getHistory" => {
                        json!({"account":request["params"][0],"items":[],"next_cursor":null,"indexedHeight":HEAD,"chainHeight":HEAD,"freshness":{"version":1,"database":"ok","status":"ready","sources":[{"source":"comet","source_head":HEAD,"indexed_cursor":HEAD,"finalized_cursor":HEAD,"last_success_at":head.time_ms()/1000,"freshness_secs":0,"last_error_at":null,"reconciled":true,"consecutive_failures":0,"state":"ready"}]}})
                    }
                    "eth_call" => native_call_result(&request, &calls),
                    other => panic!("unexpected production RPC: {other}: {request}"),
                };
                http_response(
                    &mut stream,
                    &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
                );
            }
        });
        (url, worker)
    }

    fn native_access_request(fixture: &Fixture, now: u64) -> NativeSendAccessRequest {
        NativeSendAccessRequest {
            action_key: fixture.credential.session_id().0,
            owner: must(Did::new(fixture.account.did().as_bytes())),
            owner_public_key: fixture.account.layerx_public_key(),
            not_before: now,
            expires_at: EXPIRY_SECONDS,
            native_fee_budget: None,
            counterparty: must(layerx_intents::canonical::account_id_for_protocol(
                &must(AccountId::parse(&format!(
                    "agent:did:layerx:{}:main",
                    hex::encode([9; 32])
                ))),
                3,
            )),
            asset: fixture
                .peer
                .subject
                .as_ref()
                .unwrap_or_else(|| panic!("missing value"))
                .asset,
            maximum_amount: 2_000_000_000_000,
            rate_window_ms: 60_000,
            maximum_uses: 10,
            commitment: [11; 32],
        }
    }

    fn assert_signed_local_consent(fixture: &Fixture, consent: &HumanOwnerNativeContextV1) {
        let mut signed = must(consent.signed_grant());
        let signature = ed25519_dalek::Signature::from_bytes(&signed.signature);
        signed.signature = [0; 64];
        let digest: [u8; 32] = Sha256::digest(must(
            layerx_human_kms::attestor::native_local_grant_signing_bytes(&signed),
        ))
        .into();
        must(
            SigningKey::from_bytes(fixture.account.layerx_seed())
                .verifying_key()
                .verify_strict(&digest, &signature),
        );
    }

    fn retain_consent(
        fixture: &Fixture,
        scope: &mut PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        custody: &ClockCustody,
    ) -> HumanOwnerNativeContextV1 {
        let now = EXPIRY_SECONDS - 30;
        let session: HumanOwnerNativeSessionV1 = must(serde_json::from_value(json!({
            "version":1,"tenant":fixture.peer.tenant,"principal":fixture.peer.principal,"owner":fixture.account.did(),"owner_public_key":fixture.account.layerx_public_key(),
            "session_id":fixture.credential.session_id().0,"generation":fixture.credential.generation(),"grant_id":GRANT,
            "grant_receipt_digest":Sha256::digest(fixture.credential.token_id()).to_vec(),"grant_activity_id":Sha256::digest(fixture.credential.session_id().0).to_vec(),"grant_sequence":HEAD,"expires_at_ms":EXPIRY_MS,"token":fixture.credential.token_id()
        })));
        let request = native_access_request(fixture, now);
        let coordinates = must(
            agent.native_send_owner_context(
                &session,
                u64::from_be_bytes(
                    request.action_key[..8]
                        .try_into()
                        .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
                ),
            ),
        );
        assert_eq!(coordinates.actor, session.owner);
        assert_eq!(coordinates.owner_public_key, session.owner_public_key);
        assert_eq!(coordinates.generation, session.generation);
        assert_eq!(coordinates.expiry_ms, EXPIRY_MS);
        assert!(coordinates.protocol_time_ms < EXPIRY_MS);
        let operation = must(access_operation(scope, &request));
        let passkeys = must(Passkeys::new(auth_config()));
        let mut authenticator = SoftwareAuthenticator::new();
        let browser = register_session(&passkeys, scope, &mut authenticator, now);
        let challenge = must(passkeys.begin_step_up(
            scope,
            browser.access_token().expose(),
            browser.csrf_token().expose(),
            operation,
            now + 7,
        ));
        let assertion = authenticator.assert(&challenge.ceremony);
        let step_up =
            must(passkeys.finish_step_up(scope, &challenge.challenge_id, &assertion, now + 8));
        let grant = must(unsigned_access(&request, &session, &coordinates));
        let native_consent = NativeConsent::LocalGrant(&grant);
        must(native_consent.validate_at((now + 8) * 1000, session.owner_public_key));
        let disclosure = must(native_consent.digest());
        let evidence = must(CustodySigner::bind_authenticated_step_up(
            &passkeys,
            scope,
            &step_up,
            operation,
            Operation::SecuritySettings,
            disclosure,
            [0x76; 32],
            now + 8,
        ));
        let principal = scope.principal().clone();
        let custody_key = must(KeyId::new("human-primary"));
        custody.admit_owner();
        let signed = ready(custody.sign_native_consent_in_scope(
            scope,
            NativeConsentRequest::new(
                &principal,
                &custody_key,
                native_consent,
                SignAuthorization::new(Operation::SecuritySettings, Some(&evidence)),
                now + 8,
                trace(),
            ),
        ))
        .or_fail("human-primary custody must sign the production grant with authenticated SecuritySettings evidence");
        assert_eq!(signed.signer_public_key(), session.owner_public_key);
        assert_eq!(signed.disclosure_digest(), disclosure);
        must(layerx_crypto::ed25519::verify_digest(
            &session.owner_public_key,
            signed.signature(),
            &disclosure,
        ));
        let challenge = must(passkeys.begin_step_up(
            scope,
            browser.access_token().expose(),
            browser.csrf_token().expose(),
            operation,
            now + 9,
        ));
        let assertion = authenticator.assert(&challenge.ceremony);
        let step_up =
            must(passkeys.finish_step_up(scope, &challenge.challenge_id, &assertion, now + 10));
        custody.admit_owner();
        let consent = must(ready(authorize_access(
            scope,
            agent,
            custody,
            &passkeys,
            &step_up,
            operation,
            [0x75; 32],
            &trace(),
            &request,
            session,
            now + 10,
        )));
        assert_eq!(consent.expires_at_ms, EXPIRY_MS);
        assert_eq!(must(consent.signed_grant()).signature.len(), 64);
        assert_signed_local_consent(fixture, &consent);
        consent
    }
    fn trace() -> TraceId {
        must(TraceId::parse("trc_00000000000040008000000000000001"))
    }
    fn consent_row() -> RowKey {
        must(RowKey::new(format!(
            "human-native-send-access-{}",
            hex::encode([8; 32])
        )))
    }
    fn intent(fixture: &Fixture, action: [u8; 32]) -> Intent {
        let asset = fixture
            .peer
            .subject
            .as_ref()
            .unwrap_or_else(|| panic!("missing value"))
            .asset;
        let from = must(AccountId::for_asset(&fixture.account.did(), asset, asset));
        let to = must(AccountId::parse(&format!(
            "agent:did:layerx:{}:main",
            hex::encode([9; 32])
        )));
        let from_id = must(layerx_intents::canonical::account_id_for_protocol(&from, 3));
        let to_id = must(layerx_intents::canonical::account_id_for_protocol(&to, 3));
        let debit = layerx_crypto::send::SendDebit {
            from: from_id,
            to: to_id,
            asset,
            amount: 10,
            source_sequence: 7,
            idempotency_key: action,
            expires_at: EXPIRY_MS,
            context_hash: [11; 32],
            conditions: vec![],
            authorization_kind: 1,
            network_id: 125,
            protocol_version: 3,
        };
        let canonical = must(layerx_intents::canonical::send_authorization_bytes(&debit));
        let message = must(layerx_crypto::SignatureMessage::new(
            layerx_wire::hash::Domain::SignaturePreimage,
            3,
            125,
            &canonical,
        ));
        let signature = SigningKey::from_bytes(fixture.account.layerx_seed())
            .sign(&message.digest())
            .to_bytes();
        Intent::v1(IntentKind::LxpSend(must(LxpSend::new(
            from,
            to,
            AssetId::new(asset),
            Amount::from_u128(10),
            Sequence::from_u64(7),
            IdempotencyKey::new(action),
            TimestampSeconds::from_u64(EXPIRY_MS),
            ContextHash::new([11; 32]),
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new(fixture.account.layerx_public_key()),
                AuthorizationSignature::new(signature),
            ),
            must(NetworkId::new(125)),
            must(ProtocolVersion::new(3)),
        ))))
    }
    fn preparation(fixture: &Fixture, intent: &Intent, action: [u8; 32]) -> PrepareRequest {
        let compiled = must(layerx_intents::compile(intent, &Fixture::registry()));
        PrepareRequest {
            protocol_activity_type: compiled.activity_type().value(),
            actor: must(AgentDid::new(fixture.account.did())),
            authority: must(AuthorityRef::new(hex::encode(
                fixture.account.layerx_public_key(),
            ))),
            account_sequence: layerx_agent_api::Sequence(0),
            timestamp_bound: must(
                TimestampBound {
                    not_before: layerx_agent_api::TimestampSeconds((EXPIRY_SECONDS - 30) * 1000),
                    not_after: layerx_agent_api::TimestampSeconds(EXPIRY_MS),
                }
                .validate(),
            ),
            idempotency_key: must(IdempotencyRef::new(hex::encode(action))),
            fee_limit: layerx_agent_api::Amount(1_000_000_000_000),
            payload: must(PayloadBytes::new(compiled.payload().as_bytes().to_vec())),
            payload_hash: compiled.payload_hash(),
        }
    }
    fn assert_expiry(
        scope: &PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        context: &HumanOwnerNativeContextV1,
        request: &PrepareRequest,
        purpose: &layerx_agent_api::identity::NativeSendPurposeV2,
    ) {
        let original = scope
            .get(Table::Journeys, &consent_row())
            .unwrap_or_else(|| panic!("missing value"))
            .bytes()
            .to_vec();
        must(context.session.validate(scope, EXPIRY_SECONDS - 1));
        let signed_grant = must(context.signed_grant());
        must(signed_grant.validate());
        let mut grant = signed_grant.clone();
        grant.signature = [0; 64];
        let digest = must(NativeConsent::LocalGrant(&grant).digest());
        must(layerx_crypto::ed25519::verify_digest(
            &context.session.owner_public_key,
            &signed_grant.signature,
            &digest,
        ));
        must(
            NativeConsent::LocalGrant(&grant)
                .validate_at(EXPIRY_MS - 1, context.session.owner_public_key),
        );
        must(
            NativeConsent::SendPurpose(purpose)
                .validate_at(EXPIRY_MS - 1, context.session.owner_public_key),
        );
        for (seconds, milliseconds) in [
            (EXPIRY_SECONDS, EXPIRY_MS),
            (EXPIRY_SECONDS + 1, EXPIRY_MS + 1000),
        ] {
            assert!(context.session.validate(scope, seconds).is_err());
            assert!(NativeConsent::LocalGrant(&grant)
                .validate_at(milliseconds, context.session.owner_public_key)
                .is_err());
            assert!(NativeConsent::SendPurpose(purpose)
                .validate_at(milliseconds, context.session.owner_public_key)
                .is_err());
            assert!(load_access(scope, agent, context.capability_id, seconds).is_err());
            assert!(preview_send_v2(
                scope,
                agent,
                request,
                context.capability_id,
                EXPIRY_MS,
                seconds
            )
            .is_err());
        }
        assert_eq!(
            scope
                .get(Table::Journeys, &consent_row())
                .unwrap_or_else(|| panic!("missing value"))
                .bytes(),
            original
        );
    }
    pub(super) fn producer() {
        let _serial = SERIAL
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        let fixture = Fixture::new(false);
        let principal = must(PrincipalId::new(&fixture.peer.principal));
        let root = fixture.root.join("human");
        let retention = retention_uniform(10_000);
        let (mut store, digest) = install_and_open(
            &root,
            &tenancy(&[(&fixture.peer.principal, &fixture.peer.tenant)]),
            retention,
        );
        let (mut agent, transport) = fixture.client();
        let custody = fixture.custody();
        let mut scope = must(store.principal(&principal));
        let consent = retain_consent(&fixture, &mut scope, &mut agent, &custody);
        let original = must(serde_json::to_vec(&consent));
        let request = preparation(&fixture, &intent(&fixture, [0x62; 32]), [0x62; 32]);
        let first = preview_send_v2(
            &scope,
            &mut agent,
            &request,
            consent.capability_id,
            EXPIRY_MS,
            EXPIRY_SECONDS - 1,
        )
        .unwrap_or_else(|error| {
            panic!(
                "production V2 preview failed: {error:?}; {}",
                transport.diagnostic()
            )
        });
        assert_eq!(
            first.purpose.canonical_digest,
            <[u8; 32]>::from(Sha256::digest(&first.canonical_bytes))
        );
        assert_eq!(first.purpose.preparation_id, first.purpose.canonical_digest);
        assert_eq!(first.purpose.idempotency_key, [0x62; 32]);
        assert_eq!(first.purpose.economic_action, consent.capability_id);
        assert_eq!(
            first.purpose.consent_public_key,
            fixture.account.layerx_public_key()
        );
        assert_eq!(first.purpose.evm_sender, fixture.account.evm_address());
        assert_eq!(first.purpose.expires_at_ms, consent.expires_at_ms);
        custody.admit_owner();
        let signed = must(ready(custody.sign_native_consent_in_scope(
            &mut scope,
            NativeConsentRequest::new(
                &principal,
                &must(KeyId::new("human-primary")),
                NativeConsent::SendPurpose(&first.purpose),
                SignAuthorization::new(Operation::ProtocolMutation, None),
                EXPIRY_SECONDS - 1,
                trace(),
            ),
        )));
        must(layerx_crypto::ed25519::verify_digest(
            &first.owner_public_key,
            signed.signature(),
            &Sha256::digest(must(first.purpose.canonical_bytes())).into(),
        ));
        drop(scope);
        drop(store);
        drop(agent);
        drop(transport);
        let mut reopened = must(PrincipalStore::open(&root, retention, digest));
        let scope = must(reopened.principal(&principal));
        let (mut agent, transport) = fixture.client();
        let recovered = must(load_access(
            &scope,
            &mut agent,
            consent.capability_id,
            EXPIRY_SECONDS - 1,
        ));
        assert_eq!(must(serde_json::to_vec(&recovered)), original);
        let second = preview_send_v2(
            &scope,
            &mut agent,
            &request,
            recovered.capability_id,
            EXPIRY_MS,
            EXPIRY_SECONDS - 1,
        )
        .unwrap_or_else(|error| {
            panic!(
                "reopened production V2 preview failed: {error:?}; {}",
                transport.diagnostic()
            )
        });
        assert_eq!(second.canonical_bytes, first.canonical_bytes);
        assert_eq!(second.signing_preimage, first.signing_preimage);
        assert_eq!(second.purpose, first.purpose);
        assert_expiry(&scope, &mut agent, &recovered, &request, &second.purpose);
    }
    struct ApprovalPreview {
        action: [u8; 32],
        original_intent: Intent,
        quote: layerx_human_service::server::native_send::NativeSendPreviewV2,
        engine_request: PrepareRequest,
        fee_limit: u128,
    }

    fn approval_preview(
        fixture: &Fixture,
        scope: &PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        consent: &HumanOwnerNativeContextV1,
    ) -> ApprovalPreview {
        let action = [0x52; 32];
        let original_intent = intent(fixture, action);
        let quote = must(preview_send_v2(
            scope,
            agent,
            &preparation(fixture, &original_intent, action),
            consent.capability_id,
            EXPIRY_MS,
            EXPIRY_SECONDS - 20,
        ));
        let quoted_call = must(
            layerx_human_service::server::agent_runtime::decode_unsigned(&quote.canonical_bytes),
        );
        let fee_limit = u128::from(quoted_call.gas_limit)
            .checked_mul(quoted_call.max_fee_per_gas)
            .or_fail("production quoted transaction fee");
        let mut engine_request = preparation(fixture, &original_intent, action);
        engine_request.fee_limit = layerx_agent_api::Amount(fee_limit);
        let repeated = preview_send_v2(
            scope,
            agent,
            &engine_request,
            consent.capability_id,
            EXPIRY_MS,
            EXPIRY_SECONDS - 20,
        )
        .unwrap_or_else(|error| panic!("approval admission preview preflight: {error:?}"));
        let mut differences = Vec::new();
        macro_rules! compare_purpose_fields {
            ($($field:ident),+ $(,)?) => {
                $(if quote.purpose.$field != repeated.purpose.$field {
                    differences.push(format!(
                        "{}: quoted={:?}, repeated={:?}",
                        stringify!($field), quote.purpose.$field, repeated.purpose.$field
                    ));
                })+
            };
        }
        compare_purpose_fields!(
            tenant,
            agent_did,
            owner_did,
            consent_public_key,
            evm_sender,
            session_id,
            generation,
            expires_at_ms,
            capability_id,
            transaction_type,
            chain_id,
            activity,
            preparation_id,
            canonical_digest,
            economic_action,
            idempotency_key,
            commitment,
        );
        assert!(
            differences.is_empty(),
            "approval purpose drift: {}",
            differences.join("; ")
        );
        assert_eq!(repeated.owner_public_key, quote.purpose.consent_public_key);
        assert_eq!(repeated.canonical_bytes, quote.canonical_bytes);
        assert_eq!(repeated.signing_preimage, quote.signing_preimage);
        ApprovalPreview {
            action,
            original_intent,
            quote,
            engine_request,
            fee_limit,
        }
    }

    struct ApprovalPreflight<'a, 'scope> {
        fixture: &'a Fixture,
        scope: &'a mut PrincipalScope<'scope>,
        custody: &'a ClockCustody,
        principal: &'a PrincipalId,
        quote: &'a layerx_human_service::server::native_send::NativeSendPreviewV2,
        engine_request: &'a PrepareRequest,
        original_intent: &'a Intent,
        consent: &'a HumanOwnerNativeContextV1,
        action: [u8; 32],
    }

    fn assert_approval_preflight(context: ApprovalPreflight<'_, '_>) {
        let ApprovalPreflight {
            fixture,
            scope,
            custody,
            principal,
            quote,
            engine_request,
            original_intent,
            consent,
            action,
        } = context;
        custody.admit_owner();
        let grant = ready(custody.sign_native_consent_in_scope(
            scope,
            NativeConsentRequest::new(
                principal,
                &must(KeyId::new("human-primary")),
                NativeConsent::SendPurpose(&quote.purpose),
                SignAuthorization::new(Operation::ProtocolMutation, None),
                EXPIRY_SECONDS - 20,
                trace(),
            ),
        ))
        .unwrap_or_else(|error| panic!("approval purpose custody preflight: {error:?}"));
        assert_eq!(grant.signer_public_key(), quote.purpose.consent_public_key);
        assert_eq!(
            grant.disclosure_digest(),
            must(NativeConsent::SendPurpose(&quote.purpose).digest())
        );
        layerx_crypto::ed25519::verify_digest(
            &quote.purpose.consent_public_key,
            grant.signature(),
            &Sha256::digest(must(quote.purpose.canonical_bytes())).into(),
        )
        .unwrap_or_else(|error| panic!("approval purpose signature preflight: {error:?}"));
        let signed_purpose = layerx_agent_api::identity::SignedNativeSendPurposeV2 {
            purpose: quote.purpose.clone(),
            signature: *grant.signature(),
        };
        let compiled = must(layerx_intents::compile(
            original_intent,
            &Fixture::registry(),
        ));
        assert_eq!(
            compiled.payload().as_bytes(),
            engine_request.payload.as_bytes()
        );
        assert_eq!(
            must(layerx_wire::hash::payload_hash_for(compiled.payload())),
            engine_request.payload_hash,
            "approval request must preserve the registry-bound compiled payload commitment"
        );
        let body = layerx_human_service::server::native_send::encode_native_send_prepare_v2(
            engine_request,
            &signed_purpose,
            Some(&must(consent.signed_grant())),
        )
        .unwrap_or_else(|error| {
            panic!(
                "approval prepare encoder preflight: {error:?}; \
                 compiled payload_hash={}; request={engine_request:?}",
                hex::encode(engine_request.payload_hash)
            )
        });
        let credential = layerx_sdk::agent_envelope::EnvelopeCredential::new(
            &consent.session.tenant,
            consent.session.session_id,
            fixture.credential.token_id(),
            consent.session.generation,
        )
        .unwrap_or_else(|error| panic!("approval envelope credential preflight: {error:?}"));
        let envelope = layerx_sdk::agent_envelope::encode_envelope(
            layerx_sdk::Operation::Prepare,
            layerx_agent_api::error::RequestId(u64::from_be_bytes(
                action[..8]
                    .try_into()
                    .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            )),
            &body,
            Some(&credential),
            Some(must(layerx_agent_api::idempotency::Key::new(action))),
        )
        .unwrap_or_else(|error| panic!("approval envelope encoder preflight: {error:?}"));
        assert_eq!(envelope["request"], body);
    }

    struct ReopenedApproval<'a> {
        fixture: &'a Fixture,
        root: &'a Path,
        retention: layerx_human_service::store::RetentionPolicy,
        digest: layerx_human_service::store::TenancyDigest,
        principal: &'a PrincipalId,
        row: &'a RowKey,
        original: &'a [u8],
        journal: &'a Value,
        original_consent: &'a [u8],
        action: [u8; 32],
        sender: [u8; 20],
        binding: ([u8; 32], [u8; 32]),
        purpose: layerx_agent_api::identity::NativeSendPurposeV2,
        custody: &'a ClockCustody,
        contract: &'a layerx_sdk::Client,
        registry: &'a ModuleRegistry,
    }

    fn assert_reopened_approval(context: ReopenedApproval<'_>) {
        let ReopenedApproval {
            fixture,
            root,
            retention,
            digest,
            principal,
            row,
            original,
            journal,
            original_consent,
            action,
            sender,
            binding,
            purpose,
            custody,
            contract,
            registry,
        } = context;
        let mut reopened = must(PrincipalStore::open(root, retention, digest));
        let mut scope = must(reopened.principal(principal));
        let (mut agent, transport) = fixture.client();
        let mut engine = must(JourneyEngine::load(
            &scope,
            &must(JourneyId::new("jrn_clockapproval")),
        ))
        .or_fail("reopened real approval journey");
        assert_eq!(
            scope
                .get(Table::Journeys, row)
                .unwrap_or_else(|| panic!("missing value"))
                .bytes(),
            original
        );
        assert_eq!(engine.native_approval_binding(), Some(binding));
        assert_eq!(must(engine.native_context_binding()), Some(purpose.clone()));
        let consent = must(load_access(&scope, &mut agent, [8; 32], EXPIRY_SECONDS - 1));
        assert_eq!(must(serde_json::to_vec(&consent)), original_consent);
        let request = preparation(fixture, &intent(fixture, action), action);
        assert_expiry(&scope, &mut agent, &consent, &request, &purpose);
        assert_eq!(purpose.evm_sender, sender);
        must(
            layerx_human_service::journeys::AgentBoundary::bind_transaction_sender(
                &mut agent, sender,
            ),
        );
        must(agent.bind_native_journey_context(consent, EXPIRY_MS));
        let decided = agent
            .native_effect_approval_decide(
                binding.0,
                binding.1,
                &hex::encode([0x54; 32]),
                true,
                HEAD,
            )
            .unwrap_or_else(|error| {
                panic!(
                    "production approval decision failed: {error:?}; {}",
                    transport.diagnostic()
                )
            });
        assert_eq!(decided.state, NativeEffectApprovalFactState::Granted);
        assert_eq!(decided.approval_id, binding.0);
        assert_eq!(decided.held_digest, binding.1);
        assert!(decided.release_ref.is_some());
        custody.admit_owner();
        ready(engine.advance(
            &mut scope,
            contract,
            &mut agent,
            custody,
            registry,
            &trace(),
            EXPIRY_SECONDS - 1,
        ))
        .unwrap_or_else(|error| {
            panic!(
                "reopened production Journey resume failed: {error:?}; {}",
                transport.diagnostic()
            )
        });
        assert_eq!(must(engine.status()).phases(), [JourneyPhase::Prepared]);
        assert_eq!(must(engine.native_context_binding()), Some(purpose));
        assert_eq!(engine.native_approval_binding(), Some(binding));
        let resumed: Value = must(serde_json::from_slice(
            scope
                .get(Table::Journeys, row)
                .unwrap_or_else(|| panic!("missing value"))
                .bytes(),
        ));
        for field in ["action_key", "payload", "payload_hash", "preparation"] {
            assert_eq!(resumed["legs"][0][field], journal["legs"][0][field]);
        }
        assert!(resumed["legs"][0]["signed"].is_null());
    }

    struct HeldApproval<'a, 'scope> {
        scope: &'a mut PrincipalScope<'scope>,
        agent: &'a mut AgentRuntime,
        custody: &'a ClockCustody,
        principal: &'a PrincipalId,
        quote: &'a layerx_human_service::server::native_send::NativeSendPreviewV2,
        consent: HumanOwnerNativeContextV1,
        transport: &'a Peer,
        plan: &'a JourneyPlan,
        registry: &'a ModuleRegistry,
        contract: &'a layerx_sdk::Client,
        sender: [u8; 20],
        grant_signature_bytes: usize,
    }

    struct HeldState {
        binding: ([u8; 32], [u8; 32]),
        purpose: layerx_agent_api::identity::NativeSendPurposeV2,
        row: RowKey,
        original: Vec<u8>,
        journal: Value,
    }

    fn assert_held_approval(context: HeldApproval<'_, '_>) -> HeldState {
        let HeldApproval {
            scope,
            agent,
            custody,
            principal,
            quote,
            consent,
            transport,
            plan,
            registry,
            contract,
            sender,
            grant_signature_bytes,
        } = context;
        assert_eq!(quote.purpose.evm_sender, sender);
        custody.admit_owner();
        assert_eq!(
            must(custody.evm_wallet(principal, &must(KeyId::new("human-primary")))),
            sender,
            "approval custody wallet must match the signed owner's preview sender"
        );
        must(
            layerx_human_service::journeys::AgentBoundary::bind_transaction_sender(
                &mut *agent,
                sender,
            ),
        );
        must(agent.bind_native_journey_context(consent, EXPIRY_MS));
        let mut engine = must(JourneyEngine::start_native(
            &mut *scope,
            plan,
            registry,
            EXPIRY_SECONDS - 20,
        ));
        for _ in 0..3 {
            if must(engine.status()).phases() == [JourneyPhase::AwaitingApproval] {
                break;
            }
            custody.admit_owner();
            let advanced = ready(engine.advance(
                &mut *scope,
                contract,
                &mut *agent,
                custody,
                registry,
                &trace(),
                EXPIRY_SECONDS - 20,
            ));
            advanced.unwrap_or_else(|error| {
                panic!(
                    "clock regression native Journey advance: {error:?}; \
                     NativeLocalGrantConsentV1 has {grant_signature_bytes} signature bytes; \
                     AgentRuntime::native_send_preview_v2 sends the grant through \
                     daemon NATIVE_SEND_OWNER_PREVIEW_V1; signed grant, bound sender, \
                     owner and prepared transaction passed the production quote; \
                     phase={:?}; retained native purpose={:?}; custody refusal={:?}; {}",
                    engine.status().map(|status| status.phases().to_vec()),
                    engine.native_context_binding(),
                    custody.provider.last_refusal_code(),
                    transport.diagnostic()
                );
            });
        }
        assert_eq!(
            must(engine.status()).phases(),
            [JourneyPhase::AwaitingApproval]
        );
        let binding = engine
            .native_approval_binding()
            .or_fail("production-held approval");
        let purpose = must(engine.native_context_binding()).or_fail("signed production purpose");
        assert_eq!(purpose, quote.purpose);
        let row = must(RowKey::new(format!("journey-{}", hex::encode([0x53; 32]))));
        let original = scope
            .get(Table::Journeys, &row)
            .unwrap_or_else(|| panic!("missing value"))
            .bytes()
            .to_vec();
        let journal: Value = must(serde_json::from_slice(&original));
        assert!(journal["legs"][0]["signed"].is_null());
        let material = must(agent.native_effect_approval_material(binding.0, binding.1));
        assert_eq!(
            <[u8; 32]>::from(Sha256::digest(&material.canonical_unsigned_bytes)),
            purpose.canonical_digest
        );
        HeldState {
            binding,
            purpose,
            row,
            original,
            journal,
        }
    }

    fn approval_plan(
        fixture: &Fixture,
        original_intent: Intent,
        action: [u8; 32],
        fee_limit: u128,
    ) -> JourneyPlan {
        must(JourneyPlan::new(
            must(JourneyId::new("jrn_clockapproval")),
            JourneyKind::Move,
            [0x53; 32],
            must(KeyId::new("human-primary")),
            Operation::ProtocolMutation,
            vec![must(JourneyLeg::new(
                original_intent,
                action,
                must(AgentDid::new(fixture.account.did())),
                must(AuthorityRef::new(hex::encode(
                    fixture.account.layerx_public_key(),
                ))),
                0,
                0,
                (EXPIRY_SECONDS - 30) * 1000,
                EXPIRY_MS,
                fee_limit,
            ))],
        ))
    }

    pub(super) fn approval() {
        let _serial = SERIAL
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        let fixture = Fixture::new(true);
        let principal = must(PrincipalId::new(&fixture.peer.principal));
        let retention = retention_uniform(10_000);
        let root = fixture.root.join("human");
        let (mut store, digest) = install_and_open(
            &root,
            &tenancy(&[(&fixture.peer.principal, &fixture.peer.tenant)]),
            retention,
        );
        let (mut agent, transport) = fixture.client();
        let custody = fixture.custody();
        let mut scope = must(store.principal(&principal));
        let consent = retain_consent(&fixture, &mut scope, &mut agent, &custody);
        let original_consent = must(serde_json::to_vec(&consent));
        let grant_signature_bytes = must(consent.signed_grant()).signature.len();
        assert_eq!(
            grant_signature_bytes, 64,
            "native local grant Ed25519 signature"
        );
        let ApprovalPreview {
            action,
            original_intent,
            quote,
            engine_request,
            fee_limit,
        } = approval_preview(&fixture, &scope, &mut agent, &consent);
        assert_approval_preflight(ApprovalPreflight {
            fixture: &fixture,
            scope: &mut scope,
            custody: &custody,
            principal: &principal,
            quote: &quote,
            engine_request: &engine_request,
            original_intent: &original_intent,
            consent: &consent,
            action,
        });
        let registry = Fixture::registry();
        let plan = approval_plan(&fixture, original_intent, action, fee_limit);
        let sender = fixture.account.evm_address();
        let contract = must(layerx_sdk::Client::daemon(
            "offline-contract",
            layerx_agent_api::agent_api_schema_v1().version,
        ));
        let HeldState {
            binding,
            purpose,
            row,
            original,
            journal,
        } = assert_held_approval(HeldApproval {
            scope: &mut scope,
            agent: &mut agent,
            custody: &custody,
            principal: &principal,
            quote: &quote,
            consent,
            transport: &transport,
            plan: &plan,
            registry: &registry,
            contract: &contract,
            sender,
            grant_signature_bytes,
        });
        drop(scope);
        drop(store);
        drop(agent);
        drop(transport);
        assert_reopened_approval(ReopenedApproval {
            fixture: &fixture,
            root: &root,
            retention,
            digest,
            principal: &principal,
            row: &row,
            original: &original,
            journal: &journal,
            original_consent: &original_consent,
            action,
            sender,
            binding,
            purpose,
            custody: &custody,
            contract: &contract,
            registry: &registry,
        });
    }

    const EVM_RECORDING: &str = r#"{"eth_chainId": "0x7d", "eth_blockNumber": "0x40", "eth_getBlockByNumber": {"hash": "0x70b9d39f64c2bfa613fe696fd3ad7727afe3cd0c8739b5566c5a478b13e04a44", "parentHash": "0xd40c8f8a13c750f792cb947a537ed6fa2c0867c8d79a2407a0ed31b4ab8f6463", "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", "miner": "0x0000000000000000000000000000000000000000", "stateRoot": "0x70a17b1f87d090befed107d9adbf14a09d28ced56df620e873d2a58b011d7971", "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421", "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421", "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", "difficulty": "0x0", "number": "0x40", "gasLimit": "0x1c9c380", "gasUsed": "0x0", "timestamp": "0x6aa78d28", "extraData": "0x", "mixHash": "0x7955d8559d3a6bbab74856a87e5e4000fea50a9d3613acb3b75f4fda626466b4", "nonce": "0x0000000000000000", "baseFeePerGas": "0x36382", "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421", "blobGasUsed": "0x0", "excessBlobGas": "0x0", "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000", "requestsHash": "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "totalDifficulty": "0x0", "size": "0x266", "uncles": [], "transactions": [], "withdrawals": []}, "eth_getTransactionCount": "0x0", "eth_getBalance": "0x0", "eth_estimateGas": "0x5208"}"#;
}

pub mod require_value {
    include!("support/require_value.rs");
}
