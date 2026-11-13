//! Operator cosign contract: generated qualification identities run as an
//! independent operator with its own `layerx-mirror-signer bridge` process,
//! its own attestor and fee-payer handles, its own journal, cosign and
//! delivery directories and its own `layerx-bridge-cosign` transport process
//! over pinned mutual TLS. An operator's collection reaches the declared
//! threshold only through shares the transport delivered from the others.
//! Shares are untrusted: wrong-digest, unknown, duplicate and malformed
//! entries never count, an outsider's share is refused by every operator and
//! below the threshold assembly keeps waiting. Killing and restarting one
//! operator keeps every share already published, delivery to a stopped
//! operator resumes on its restart, and republishing never adds a second
//! share.

use std::fs;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

mod support;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    assemble_signatures, ethereum_address, recover_signer, uint256_from_u64, InboundAttestation,
    SignatureError,
};
use layerx_bridge_relayer::config::{RelayerConfig, ATTESTOR_SET_SIZE, ATTESTOR_THRESHOLD};
use layerx_bridge_relayer::cosign::transport::{spki_pin, ENABLE_VARIABLE};
use layerx_bridge_relayer::cosign::transport::{DestinationPolicy, OperatorInventory, Transport};
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{
    Completion, Entry, Journal, JournalError, Observation, Position,
};
use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc};
use layerx_bridge_relayer::signer::{Attestor, ATTEST_INBOUND_DOMAIN, PAXEER_TRANSACTION_DOMAIN};
use layerx_mirror::rpc::RpcCluster;
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use openssl::asn1::Asn1Time;
use openssl::bn::{BigNum, MsbOption};
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkey::{PKey, Private};
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509Name, X509};
use serde::Deserialize;

const HANDLE: &str = "bridge-attestor";
const TRANSPORT: &str = env!("CARGO_BIN_EXE_layerx-bridge-cosign");
const WAIT: Duration = Duration::from_secs(60);

#[path = "support/signer_binary.rs"]
mod signer_binary;
use signer_binary::signer_binary;

/// Qualification uses the declared cardinality; generated keys confer no operator approval.
fn declared_quorum() -> (usize, usize) {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/bridge/deploy/attestors.json");
    let text = fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("declared membership {}: {error}", path.display()));
    let document: serde_json::Value =
        serde_json::from_str(&text).unwrap_or_else(|error| panic!("attestors.json: {error}"));
    let threshold = document["threshold"]
        .as_u64()
        .and_then(|value| usize::try_from(value).ok())
        .or_panic("attestors.json has no threshold");
    let members = document["attestors"].as_array().map_or(0, Vec::len);
    assert_eq!((members, threshold), (1, 1), "governed attestor set");
    (ATTESTOR_SET_SIZE, usize::from(ATTESTOR_THRESHOLD))
}

fn wait_for(what: &str, mut done: impl FnMut() -> bool) {
    let deadline = Instant::now() + WAIT;
    while !done() {
        assert!(Instant::now() < deadline, "timed out waiting for {what}");
        std::thread::sleep(Duration::from_millis(50));
    }
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("port: {error}"))
        .local_addr()
        .unwrap_or_else(|error| panic!("port: {error}"))
        .port()
}

fn ec_key() -> PKey<Private> {
    let group = EcGroup::from_curve_name(Nid::X9_62_PRIME256V1).or_panic("expected Ok value");
    PKey::from_ec_key(EcKey::generate(&group).or_panic("expected Ok value"))
        .or_panic("expected Ok value")
}

fn certificate(name: &str, key: &PKey<Private>, issuer: Option<(&X509, &PKey<Private>)>) -> X509 {
    let mut subject = X509Name::builder().or_panic("expected Ok value");
    subject
        .append_entry_by_text("CN", name)
        .or_panic("expected Ok value");
    let subject = subject.build();
    let mut builder = X509::builder().or_panic("expected Ok value");
    builder.set_version(2).or_panic("expected Ok value");
    let mut serial = BigNum::new().or_panic("expected Ok value");
    serial
        .rand(64, MsbOption::MAYBE_ZERO, false)
        .or_panic("expected Ok value");
    builder
        .set_serial_number(&serial.to_asn1_integer().or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    builder
        .set_subject_name(&subject)
        .or_panic("expected Ok value");
    builder
        .set_not_before(&Asn1Time::days_from_now(0).or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    builder
        .set_not_after(&Asn1Time::days_from_now(1).or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    builder.set_pubkey(key).or_panic("expected Ok value");
    if let Some((ca, ca_key)) = issuer {
        builder
            .set_issuer_name(ca.subject_name())
            .or_panic("expected Ok value");
        let san = SubjectAlternativeName::new()
            .dns(name)
            .build(&builder.x509v3_context(Some(ca), None))
            .or_panic("expected Ok value");
        builder.append_extension(san).or_panic("expected Ok value");
        builder
            .append_extension(
                ExtendedKeyUsage::new()
                    .server_auth()
                    .client_auth()
                    .build()
                    .or_panic("expected Ok value"),
            )
            .or_panic("expected Ok value");
        builder
            .sign(ca_key, MessageDigest::sha256())
            .or_panic("expected Ok value");
    } else {
        builder
            .set_issuer_name(&subject)
            .or_panic("expected Ok value");
        builder
            .append_extension(
                BasicConstraints::new()
                    .critical()
                    .ca()
                    .build()
                    .or_panic("expected Ok value"),
            )
            .or_panic("expected Ok value");
        builder
            .append_extension(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build()
                    .or_panic("expected Ok value"),
            )
            .or_panic("expected Ok value");
        builder
            .sign(key, MessageDigest::sha256())
            .or_panic("expected Ok value");
    }
    builder.build()
}

/// The test trust anchor every operator's transport authenticates against.
struct Ca {
    cert: X509,
    key: PKey<Private>,
    path: PathBuf,
}

impl Ca {
    fn new(root: &Path) -> Self {
        let key = ec_key();
        let cert = certificate("operator-test-ca", &key, None);
        let path = root.join("ca.pem");
        fs::write(&path, cert.to_pem().or_panic("expected Ok value")).or_panic("expected Ok value");
        Self { cert, key, path }
    }
}

/// One operator: its own attestor and fee-payer keys, key manifest, signer
/// process, journal, cosign and delivery directories, TLS identity and
/// transport process. Nothing is shared between operators but the CA.
struct Operator {
    name: String,
    key: SigningKey,
    fee_payer: SigningKey,
    fee_payer_handle: String,
    home: PathBuf,
    socket: PathBuf,
    cosign: PathBuf,
    delivery: PathBuf,
    port: u16,
    tls_key: PathBuf,
    tls_cert: PathBuf,
    pin: [u8; 32],
    config: PathBuf,
    log: PathBuf,
    child: Option<Child>,
    transport: Option<Child>,
}

impl Operator {
    fn new(root: &Path, ca: &Ca, index: u8) -> Self {
        let name = format!("operator-{index}");
        let home = root.join(&name);
        let cosign = home.join("cosign");
        let delivery = home.join("delivery");
        let secrets = home.join("secrets");
        for directory in [home.join("journal"), cosign.clone(), delivery.clone()] {
            fs::create_dir_all(&directory).unwrap_or_else(|error| panic!("home: {error}"));
        }
        fs::create_dir_all(&secrets).unwrap_or_else(|error| panic!("secrets: {error}"));
        fs::set_permissions(&secrets, fs::Permissions::from_mode(0o700))
            .unwrap_or_else(|error| panic!("secrets mode: {error}"));
        let key = fixture_key(0x40 + index);
        let fee_payer = fixture_key(0x60 + index);
        let fee_payer_handle = format!("bridge-fee-payer-{index}");
        let key_file = secrets.join("attestor.key");
        let fee_payer_file = secrets.join("fee-payer.key");
        let ethereum_file = secrets.join("ethereum-fee-payer.key");
        for (path, last) in [
            (&key_file, 0x40 + index),
            (&fee_payer_file, 0x60 + index),
            (&ethereum_file, 0x80 + index),
        ] {
            fs::write(path, hex::encode(&fixture_secret(last)))
                .unwrap_or_else(|error| panic!("key file: {error}"));
            fs::set_permissions(path, fs::Permissions::from_mode(0o400))
                .unwrap_or_else(|error| panic!("key mode: {error}"));
        }
        let domain = |domain: &'static [u8]| {
            std::str::from_utf8(domain).unwrap_or_else(|_| panic!("domain"))
        };
        let path = |path: &Path| path.to_str().or_panic("key path").to_owned();
        let manifest = serde_json::json!({"keys": [{
            "handle": HANDLE,
            "algorithm": "secp256k1",
            "key_file": path(&key_file),
            "domains": [domain(ATTEST_INBOUND_DOMAIN)],
        }, {
            "handle": fee_payer_handle,
            "algorithm": "secp256k1",
            "key_file": path(&fee_payer_file),
            "domains": [domain(PAXEER_TRANSACTION_DOMAIN)],
        }, {
            "handle": format!("ethereum-fee-payer-{index}"),
            "algorithm": "secp256k1",
            "key_file": path(&ethereum_file),
            "domains": [domain(layerx_bridge_relayer::signer::ETHEREUM_TRANSACTION_DOMAIN)],
        }]});
        fs::write(home.join("keys.json"), manifest.to_string())
            .unwrap_or_else(|error| panic!("manifest: {error}"));
        let tls_key_pair = ec_key();
        let tls = certificate(&name, &tls_key_pair, Some((&ca.cert, &ca.key)));
        let tls_cert = secrets.join("certificate.pem");
        let tls_key = secrets.join("private-key.pem");
        fs::write(&tls_cert, tls.to_pem().or_panic("expected Ok value"))
            .or_panic("expected Ok value");
        fs::write(
            &tls_key,
            tls_key_pair
                .private_key_to_pem_pkcs8()
                .or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
        fs::set_permissions(&tls_key, fs::Permissions::from_mode(0o400))
            .or_panic("expected Ok value");
        fs::set_permissions(&tls_cert, fs::Permissions::from_mode(0o400))
            .or_panic("expected Ok value");
        let socket = home.join("signer.sock");
        let mut operator = Self {
            name,
            key,
            fee_payer,
            fee_payer_handle,
            config: home.join("transport.json"),
            log: home.join("transport.log"),
            home,
            socket,
            cosign,
            delivery,
            port: free_port(),
            tls_key,
            tls_cert,
            pin: spki_pin(&tls).or_panic("expected Ok value"),
            child: None,
            transport: None,
        };
        operator.start();
        operator
    }

    fn listen(&self) -> SocketAddr {
        SocketAddr::from(([127, 0, 0, 1], self.port))
    }

    /// Writes this operator's transport configuration naming `peers`.
    fn configure(&self, ca: &Ca, peers: &[&Self]) {
        let peers: Vec<_> = peers
            .iter()
            .map(|peer| {
                serde_json::json!({
                    "attestor": hex::prefixed(&peer.address()),
                    "address": peer.listen().to_string(),
                    "server_name": peer.name,
                    "spki_sha256": hex::prefixed(&peer.pin),
                })
            })
            .collect();
        let config = serde_json::json!({
            "attestor": hex::prefixed(&self.address()),
            "listen": self.listen().to_string(),
            "server_name": self.name,
            "trust_anchor": ca.path,
            "certificate": self.tls_cert,
            "private_key": self.tls_key,
            "cosign_directory": self.cosign,
            "delivery_directory": self.delivery,
            "peers": peers,
            "timeout_ms": 2000,
            "sweep_interval_ms": 100,
            "max_connections": 16,
        });
        fs::write(
            &self.config,
            serde_json::to_vec_pretty(&config).unwrap_or_else(|error| panic!("{error}")),
        )
        .unwrap_or_else(|error| panic!("transport config: {error}"));
    }

    fn start(&mut self) {
        let _ = fs::remove_file(&self.socket);
        let child = Command::new(signer_binary())
            .arg("bridge")
            .arg("--socket")
            .arg(&self.socket)
            .arg("--keys")
            .arg(self.home.join("keys.json"))
            .stdin(Stdio::null())
            .spawn()
            .unwrap_or_else(|error| panic!("spawn bridge signer: {error}"));
        self.child = Some(child);
        let started = Instant::now();
        while !self.socket.exists() {
            assert!(
                self.child
                    .as_mut()
                    .or_panic("expected Some value")
                    .try_wait()
                    .or_panic("expected Ok value")
                    .is_none(),
                "bridge signer exited before binding {}",
                self.socket.display()
            );
            assert!(
                started.elapsed() < Duration::from_secs(10),
                "bridge signer did not bind"
            );
            std::thread::sleep(Duration::from_millis(20));
        }
    }

    fn kill(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    /// Starts this operator's real cosign transport process.
    fn start_transport(&mut self) {
        assert!(self.transport.is_none());
        let log = fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log)
            .unwrap_or_else(|error| panic!("transport log: {error}"));
        let child = Command::new(TRANSPORT)
            .env(ENABLE_VARIABLE, &self.config)
            .env_remove(layerx_bridge_relayer::cosign::transport::INVENTORY_VARIABLE)
            .env_remove(layerx_bridge_relayer::cosign::transport::RELAYER_CONFIG_VARIABLE)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(log)
            .spawn()
            .unwrap_or_else(|error| panic!("spawn cosign transport: {error}"));
        self.transport = Some(child);
        let deadline = Instant::now() + WAIT;
        while TcpStream::connect(self.listen()).is_err() {
            assert!(Instant::now() < deadline, "{} never listened", self.name);
            assert!(
                self.transport
                    .as_mut()
                    .and_then(|child| child.try_wait().ok())
                    .is_some_and(|status| status.is_none()),
                "{} transport exited: {}",
                self.name,
                self.transport_log()
            );
            std::thread::sleep(Duration::from_millis(20));
        }
    }

    fn kill_transport(&mut self) {
        if let Some(mut child) = self.transport.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    fn transport_log(&self) -> String {
        fs::read_to_string(&self.log).unwrap_or_default()
    }

    fn remote(&self, handle: &str, key: &SigningKey) -> RemoteChainSigner {
        RemoteChainSigner::new(RemoteSignerConfig {
            endpoint: SignerEndpoint::Uds {
                socket: self.socket.clone(),
            },
            algorithm: SigningAlgorithm::Secp256k1Recoverable,
            key_handle: handle.to_owned(),
            public_key: fixture_public_key(key),
            timeout: Duration::from_secs(5),
        })
        .or_panic("remote signer")
    }

    fn attestor(&self) -> Attestor {
        Attestor::new(self.remote(HANDLE, &self.key)).or_panic("attestor")
    }

    fn address(&self) -> [u8; 20] {
        ethereum_address(self.key.verifying_key())
    }

    fn directory(&self) -> CosignDirectory {
        CosignDirectory::new(self.cosign.clone())
    }

    fn collect(&self, digest: &[u8; 32]) -> Vec<[u8; 65]> {
        self.directory().collect(digest)
    }

    fn holds(&self, digest: &[u8; 32], signer: &[u8; 20]) -> bool {
        digest_directory(&self.cosign, digest)
            .join(format!("{}.sig", hex::encode(signer)))
            .is_file()
    }

    /// Signs through the real signer process and publishes the share into
    /// this operator's own cosign directory only.
    fn publish(&self, attestation: &InboundAttestation) -> [u8; 65] {
        let attestor = self.attestor();
        assert_eq!(attestor.address(), self.address());
        let mut journal = Journal::open(&self.home.join("journal/relay.jsonl"))
            .or_panic("independent production journal");
        let observation = Observation::inbound(
            attestation,
            Position {
                block_number: 1,
                block_hash: attestation.tx_hash,
            },
        );
        let item = observation.key();
        if !journal.state().items.contains_key(&item) {
            journal
                .append(&Entry::Observed {
                    item: item.clone(),
                    observation,
                })
                .or_panic("durable observed item");
        }
        assert_eq!(journal.state().items[&item].observation, observation);
        assert!(
            journal.state().items[&item].is_open(),
            "completed journal item does not republish"
        );
        let signature = if let Some(signature) = journal.state().items[&item].signature {
            signature
        } else {
            let signature = attestor.sign_inbound(attestation).or_panic("sign");
            journal
                .append(&Entry::Signed {
                    item: item.clone(),
                    signature,
                })
                .unwrap_or_else(|error| {
                    panic!("signature persisted before publication: {error:?}")
                });
            signature
        };
        assert!(journal.state().items[&item].submissions.is_empty());
        self.directory()
            .publish(&attestation.digest(), &attestor.address(), &signature)
            .or_panic("publish");
        signature
    }
}

impl Drop for Operator {
    fn drop(&mut self) {
        self.kill_transport();
        self.kill();
    }
}

fn attestation(log_index: u64) -> InboundAttestation {
    let mut recipient = [0_u8; 32];
    recipient[31] = 0x77;
    InboundAttestation {
        chain_id: 1,
        vault: [0x11; 20],
        tx_hash: [0xab; 32],
        log_index,
        recipient,
        asset: [0x22; 20],
        amount: uint256_from_u64(1_000_000),
    }
}

fn digest_directory(shared: &Path, digest: &[u8; 32]) -> PathBuf {
    shared.join(hex::encode(digest))
}

/// Waits until every operator in `receivers` holds `signer`'s share for
/// `digest`, delivered by the transport.
fn wait_delivered(receivers: &[&Operator], digest: &[u8; 32], signer: &[u8; 20]) {
    for receiver in receivers {
        wait_for(
            &format!("delivery of {} to {}", hex::prefixed(signer), receiver.name),
            || receiver.holds(digest, signer),
        );
    }
}

#[test]
fn declared_cardinality_assembles_only_from_distinct_real_operator_shares() {
    let (size, threshold) = declared_quorum();
    assert!(
        size > threshold,
        "a member beyond the quorum stays reachable"
    );
    let root = fixture_directory("operator-cosign");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .unwrap_or_else(|error| panic!("root mode: {error}"));
    let ca = Ca::new(&root);
    let mut operators: Vec<Operator> = (0..=u8::try_from(size).unwrap_or(u8::MAX))
        .map(|index| Operator::new(&root, &ca, index))
        .collect();
    let mut outsider = operators.pop().or_panic("outsider");
    let members: Vec<[u8; 20]> = operators.iter().map(Operator::address).collect();
    assert_operator_independence(&operators, &outsider, &members, size);

    let wanted = attestation(7);
    let digest = wanted.digest();

    assert_fee_payers(&operators, &wanted);

    start_operator_transports(&mut operators, &mut outsider, &ca, &root, threshold);

    assert_below_quorum(&operators, &members, &wanted, &digest, threshold);

    let outsider_share = assert_outsider_rejected(&operators, &outsider, &wanted, &digest);

    assert_hostile_collections(
        &operators,
        &outsider,
        &members,
        &wanted,
        &digest,
        outsider_share,
        threshold,
    );

    let assembled = restart_quorum(&mut operators, &members, &wanted, &digest, threshold);

    assert_delivered_quorum(&mut operators, &members, &digest, threshold, &assembled);

    assert_repeated_publication(
        &mut operators,
        &members,
        &wanted,
        &digest,
        threshold,
        &assembled,
    );

    journal_restart_and_terminal_boundaries(&operators, &wanted);

    assert_private_storage(&operators, &outsider);

    outsider.kill_transport();
    drop(outsider);
}

fn assert_operator_independence(
    operators: &[Operator],
    outsider: &Operator,
    members: &[[u8; 20]],
    size: usize,
) {
    let mut distinct = members.to_vec();
    distinct.sort_unstable();
    distinct.dedup();
    assert_eq!(
        distinct.len(),
        size,
        "operators hold distinct attestor keys"
    );
    assert!(!members.contains(&outsider.address()));
    for (index, operator) in operators.iter().enumerate() {
        assert_ne!(operator.fee_payer_handle, HANDLE);
        assert_ne!(
            fixture_public_key(&operator.fee_payer),
            fixture_public_key(&operator.key),
            "the fee payer is not the attestor key"
        );
        for other in &operators[index + 1..] {
            assert_ne!(operator.home.join("journal"), other.home.join("journal"));
            assert_ne!(operator.socket, other.socket);
            assert_ne!(operator.cosign, other.cosign);
            assert_ne!(operator.delivery, other.delivery);
            assert_ne!(operator.port, other.port);
            assert_ne!(operator.pin, other.pin);
            assert_ne!(operator.fee_payer_handle, other.fee_payer_handle);
            assert_ne!(
                fixture_public_key(&operator.fee_payer),
                fixture_public_key(&other.fee_payer),
                "fee payers are independent"
            );
        }
    }
}

fn assert_fee_payers(operators: &[Operator], wanted: &InboundAttestation) {
    // The fee-payer handle never signs an attestation.
    for operator in operators {
        let fee_payer =
            Attestor::new(operator.remote(&operator.fee_payer_handle, &operator.fee_payer))
                .or_panic("fee payer");
        assert!(
            fee_payer.sign_inbound(wanted).is_err(),
            "{} fee payer refused attestation",
            operator.name
        );
    }
}

fn start_operator_transports(
    operators: &mut [Operator],
    outsider: &mut Operator,
    ca: &Ca,
    root: &Path,
    threshold: usize,
) {
    // Each member delivers to every other member; the outsider is pinned by nobody.
    for operator in &*operators {
        let peers: Vec<&Operator> = operators
            .iter()
            .filter(|peer| peer.name != operator.name)
            .collect();
        operator.configure(ca, &peers);
    }
    let inputs = operator_inputs(root, operators, threshold);
    let inventory =
        OperatorInventory::load(&inputs.inventory).or_panic("generated operator inventory");
    for input in &inputs.operators {
        Transport::load(&input.transport_config)
            .or_panic("expected Ok value")
            .validate_operator(
                &inventory,
                &RelayerConfig::load(&input.relayer_config).or_panic("expected Ok value"),
            )
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "generated signer and transport belong to the in-test inventory"
                )
            });
    }
    outsider.configure(ca, &operators.iter().collect::<Vec<_>>());
    for operator in operators.iter_mut() {
        operator.start_transport();
    }
    outsider.start_transport();
    hostile_transport_deliveries(&operators[0], &operators[1], outsider);
}

fn assert_below_quorum(
    operators: &[Operator],
    members: &[[u8; 20]],
    wanted: &InboundAttestation,
    digest: &[u8; 32],
    threshold: usize,
) {
    // Below the threshold every operator's own collection keeps waiting and
    // never yields a partial set, once the transport delivered every share.
    for operator in &operators[..threshold - 1] {
        operator.publish(wanted);
    }
    for signer in &members[..threshold - 1] {
        wait_delivered(&operators.iter().collect::<Vec<_>>(), digest, signer);
    }
    for operator in operators {
        assert_eq!(
            assemble_signatures(digest, operator.collect(digest), members, threshold),
            Err(SignatureError::BelowThreshold {
                signatures: threshold - 1,
                threshold
            }),
            "{} waits below the threshold",
            operator.name
        );
    }
}

fn assert_outsider_rejected(
    operators: &[Operator],
    outsider: &Operator,
    wanted: &InboundAttestation,
    digest: &[u8; 32],
) -> [u8; 65] {
    // An unknown signer's valid share is refused by every operator's transport.
    let outsider_share = outsider.publish(wanted);
    let outsider_pending =
        |peer: &Operator| format!("delivery to {} pending", hex::prefixed(&peer.address()));
    for operator in operators {
        wait_for(&format!("{} refusing the outsider", operator.name), || {
            operator
                .transport_log()
                .contains("refused inbound share: unpinned client")
                && outsider
                    .transport_log()
                    .contains(&outsider_pending(operator))
        });
        assert!(
            !operator.holds(digest, &outsider.address()),
            "{} refused the outsider's share",
            operator.name
        );
    }
    assert!(
        fs::read_dir(&outsider.delivery)
            .into_iter()
            .flatten()
            .flatten()
            .all(|entry| !entry.file_name().to_string_lossy().ends_with(".ack")),
        "no operator acknowledged the outsider"
    );

    outsider_share
}

fn assert_hostile_collections(
    operators: &[Operator],
    outsider: &Operator,
    members: &[[u8; 20]],
    wanted: &InboundAttestation,
    digest: &[u8; 32],
    outsider_share: [u8; 65],
    threshold: usize,
) {
    // Hostile entries in one operator's collection: the outsider's valid share,
    // a member's share over another digest, a duplicate under another spelling,
    // malformed, misnamed and oversized entries.
    let other = attestation(8);
    let target = &operators[threshold];
    let directory = digest_directory(&target.cosign, digest);
    let unknown = directory.join(format!("{}.sig", hex::encode(&outsider.address())));
    fs::write(&unknown, hex::prefixed(&outsider_share))
        .unwrap_or_else(|error| panic!("unknown share: {error}"));
    assert_eq!(target.collect(digest).len(), threshold);
    assert_eq!(
        assemble_signatures(digest, target.collect(digest), members, threshold),
        Err(SignatureError::BelowThreshold {
            signatures: threshold - 1,
            threshold
        }),
        "an unknown signer's valid share never counts"
    );
    let last = &operators[threshold - 1];
    let foreign = last.publish(&other);
    let first = operators[0].address();
    let first_share = fs::read_to_string(directory.join(format!("{}.sig", hex::encode(&first))))
        .unwrap_or_else(|error| panic!("first share: {error}"));
    let hostile = [
        (
            directory.join(format!("{}.sig", hex::encode(&last.address()))),
            hex::prefixed(&foreign),
        ),
        (
            directory.join(format!("{}.sig", hex::encode(&first).to_uppercase())),
            first_share.clone(),
        ),
        (
            directory.join(format!("{}.sig", hex::encode(&[0x99; 20]))),
            "0xnot-a-signature".to_owned(),
        ),
        (directory.join("garbage.sig"), first_share),
        (
            directory.join(format!("{}.sig", hex::encode(&members[members.len() - 1]))),
            "x".repeat(4096),
        ),
    ];
    for (path, text) in &hostile {
        fs::write(path, text).unwrap_or_else(|error| panic!("hostile share: {error}"));
    }
    assert_eq!(
        assemble_signatures(digest, target.collect(digest), members, threshold),
        Err(SignatureError::BelowThreshold {
            signatures: threshold - 1,
            threshold
        }),
        "wrong-digest, duplicate and malformed shares never count"
    );
    for (path, _) in &hostile {
        fs::remove_file(path).unwrap_or_else(|error| panic!("hostile share: {error}"));
    }

    // Republishing an already delivered member leaves every waiting set exactly.
    operators[1].publish(wanted);
    for operator in operators {
        assert_eq!(
            assemble_signatures(digest, operator.collect(digest), members, threshold),
            Err(SignatureError::BelowThreshold {
                signatures: threshold - 1,
                threshold
            })
        );
    }
}

fn restart_quorum(
    operators: &mut [Operator],
    members: &[[u8; 20]],
    wanted: &InboundAttestation,
    digest: &[u8; 32],
    threshold: usize,
) -> Vec<[u8; 65]> {
    // The last quorum operator dies, signer and transport, while another
    // operator's transport is down; the restarted operator completes the
    // quorum, every share published before the restart is preserved, and the
    // stopped operator reaches the threshold only once delivery resumes.
    let late = members.len() - 1;
    let before: Vec<Vec<[u8; 65]>> = operators
        .iter()
        .map(|operator| operator.collect(digest))
        .collect();
    operators[threshold - 1].kill_transport();
    operators[threshold - 1].kill();
    assert!(
        operators[threshold - 1]
            .attestor()
            .sign_inbound(wanted)
            .is_err(),
        "a dead signer process produces no share"
    );
    operators[late].kill_transport();
    let offset = operators[threshold - 1].transport_log().len();
    operators[threshold - 1].start();
    operators[threshold - 1].start_transport();
    for (operator, shares) in operators.iter().zip(&before) {
        for share in shares {
            assert!(
                operator.collect(digest).contains(share),
                "restart preserves published shares"
            );
        }
    }
    operators[threshold - 1].publish(wanted);
    let completing = members[threshold - 1];
    let reached: Vec<&Operator> = operators
        .iter()
        .filter(|operator| operator.name != operators[late].name)
        .collect();
    wait_delivered(&reached, digest, &completing);
    let late_pending = format!("delivery to {} pending", hex::prefixed(&members[late]));
    wait_for("delivery to the stopped operator pending", || {
        operators[threshold - 1].transport_log()[offset..].contains(&late_pending)
    });
    assert_eq!(
        assemble_signatures(digest, operators[late].collect(digest), members, threshold),
        Err(SignatureError::BelowThreshold {
            signatures: threshold - 1,
            threshold
        }),
        "an operator not yet reached by delivery keeps waiting"
    );
    let assembled = assemble_signatures(digest, operators[0].collect(digest), members, threshold)
        .or_panic("quorum");
    assert_eq!(assembled.len(), threshold);
    assembled
}

fn assert_delivered_quorum(
    operators: &mut [Operator],
    members: &[[u8; 20]],
    digest: &[u8; 32],
    threshold: usize,
    assembled: &[[u8; 65]],
) {
    let late = members.len() - 1;
    let completing = members[threshold - 1];
    let reached: Vec<&Operator> = operators
        .iter()
        .filter(|operator| operator.name != operators[late].name)
        .collect();
    let signers: Vec<[u8; 20]> = assembled
        .iter()
        .map(|signature| {
            recover_signer(digest, signature).unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect();
    assert!(
        signers.windows(2).all(|pair| pair[0] < pair[1]),
        "ascending distinct signers"
    );
    assert!(signers.iter().all(|signer| members.contains(signer)));
    for operator in &reached {
        assert_eq!(
            assemble_signatures(digest, operator.collect(digest), members, threshold),
            Ok(assembled.to_vec()),
            "{} assembles the same authorization",
            operator.name
        );
    }
    operators[late].start_transport();
    wait_delivered(&[&operators[late]], digest, &completing);
    assert_eq!(
        assemble_signatures(digest, operators[late].collect(digest), members, threshold),
        Ok(assembled.to_vec())
    );
}

fn assert_repeated_publication(
    operators: &mut [Operator],
    members: &[[u8; 20]],
    wanted: &InboundAttestation,
    digest: &[u8; 32],
    threshold: usize,
    assembled: &[[u8; 65]],
) {
    // Repeated publication: every quorum operator republishing after a
    // restart yields the identical authorization at every operator, never a
    // second or larger set.
    for operator in &mut operators[..threshold] {
        operator.kill_transport();
        operator.kill();
        operator.start();
        operator.start_transport();
        operator.publish(wanted);
    }
    for operator in &*operators {
        assert_eq!(
            assemble_signatures(digest, operator.collect(digest), members, threshold),
            Ok(assembled.to_vec()),
            "{} keeps the identical authorization",
            operator.name
        );
    }
    // Every remaining member cosigns too; each operator still assembles
    // exactly one threshold-sized set of distinct members.
    for operator in &operators[threshold..] {
        operator.publish(wanted);
    }
    for signer in members {
        wait_delivered(&operators.iter().collect::<Vec<_>>(), digest, signer);
    }
    let complete = assemble_signatures(digest, operators[0].collect(digest), members, threshold)
        .or_panic("complete quorum");
    assert_eq!(complete.len(), threshold);
    for operator in &*operators {
        assert_eq!(
            assemble_signatures(digest, operator.collect(digest), members, threshold),
            Ok(complete.clone())
        );
    }
}

fn assert_private_storage(operators: &[Operator], outsider: &Operator) {
    // Private keys never enter any operator's cosign or delivery storage.
    let mut secrets = Vec::new();
    for operator in operators.iter().chain(std::iter::once(outsider)) {
        secrets.push(hex::encode(&operator.key.to_bytes()[..]));
        secrets.push(hex::encode(&operator.fee_payer.to_bytes()[..]));
        let pem = fs::read_to_string(&operator.tls_key)
            .unwrap_or_else(|error| panic!("tls key: {error}"));
        secrets.extend(
            pem.lines()
                .filter(|line| !line.starts_with("-----") && line.len() >= 32)
                .map(str::to_owned),
        );
    }
    for operator in operators.iter().chain(std::iter::once(outsider)) {
        for entry in walk(&operator.cosign)
            .into_iter()
            .chain(walk(&operator.delivery))
        {
            let text = fs::read(&entry).unwrap_or_default();
            let text = String::from_utf8_lossy(&text);
            for secret in &secrets {
                assert!(
                    !text.contains(secret.as_str()),
                    "{} carries a private key",
                    entry.display()
                );
            }
        }
    }
}

fn walk(directory: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    for entry in fs::read_dir(directory).into_iter().flatten().flatten() {
        let path = entry.path();
        if path.is_dir() {
            files.extend(walk(&path));
        } else {
            files.push(path);
        }
    }
    files
}

fn journal_restart_and_terminal_boundaries(operators: &[Operator], wanted: &InboundAttestation) {
    let observation = Observation::inbound(
        wanted,
        Position {
            block_number: 1,
            block_hash: wanted.tx_hash,
        },
    );
    let item = observation.key();
    for operator in operators {
        let path = operator.home.join("journal/relay.jsonl");
        let mut journal = Journal::open(&path).or_panic("reopen real operator journal");
        let state = journal.state().items[&item].clone();
        assert_eq!(state.observation, observation);
        let signature = state.signature.or_panic("persisted own share");
        assert_eq!(
            recover_signer(&wanted.digest(), &signature),
            Ok(operator.address())
        );
        assert!(state.submissions.is_empty() && state.completion.is_none() && state.is_open());
        let mut wrong = signature;
        wrong[0] ^= 1;
        assert!(matches!(
            journal.append(&Entry::Signed {
                item: item.clone(),
                signature: wrong
            }),
            Err(JournalError::Conflict(_))
        ));
        assert_eq!(journal.state().items[&item].signature, Some(signature));
        journal
            .append(&Entry::Completed {
                item: item.clone(),
                completion: Completion::AlreadyBridged,
            })
            .or_panic("journal terminal transition");
        drop(journal);
        let mut restarted = Journal::open(&path).or_panic("terminal state survives restart");
        assert!(!restarted.state().items[&item].is_open());
        let length = fs::metadata(&path).or_panic("journal metadata").len();
        assert!(matches!(
            restarted.append(&Entry::Completed {
                item: item.clone(),
                completion: Completion::AlreadyBridged
            }),
            Err(JournalError::Conflict(_))
        ));
        assert_eq!(
            fs::metadata(&path).or_panic("journal metadata").len(),
            length
        );
        assert_eq!(restarted.state().items[&item].signature, Some(signature));
        assert!(operator.collect(&wanted.digest()).contains(&signature));
    }
}

#[test]
fn durable_publication_refuses_conflicts_and_concurrent_duplicates() {
    let root = fixture_directory("operator-cosign-publication");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).or_panic("expected Ok value");
    let ca = Ca::new(&root);
    let operator = Operator::new(&root, &ca, 0);
    let wanted = attestation(701);
    let digest = wanted.digest();
    let signature = operator
        .attestor()
        .sign_inbound(&wanted)
        .or_panic("real signer share");
    let address = operator.address();
    let directory = operator.directory();
    std::thread::scope(|scope| {
        for _ in 0..8 {
            let directory = &directory;
            scope.spawn(move || {
                directory
                    .publish(&digest, &address, &signature)
                    .unwrap_or_else(|error| panic!("concurrent immutable publication: {error:?}"));
            });
        }
    });
    assert_eq!(directory.collect(&digest), vec![signature]);
    assert!(directory
        .admit(&attestation(702).digest(), &address, &signature)
        .is_err());
    assert!(directory.admit(&digest, &[0; 20], &signature).is_err());
    let share =
        digest_directory(&operator.cosign, &digest).join(format!("{}.sig", hex::encode(&address)));
    let original = fs::read(&share).or_panic("durable share");
    assert_eq!(
        fs::metadata(&share)
            .or_panic("expected Ok value")
            .permissions()
            .mode()
            & 0o077,
        0
    );
    drop(directory);
    assert_eq!(operator.directory().collect(&digest), vec![signature]);
    fs::write(&share, b"corrupted existing share").or_panic("expected Ok value");
    assert!(matches!(
        operator.directory().publish(&digest, &address, &signature),
        Err(JournalError::Conflict(_))
    ));
    assert_eq!(
        fs::read(&share).or_panic("expected Ok value"),
        b"corrupted existing share"
    );
    fs::remove_file(&share).or_panic("expected Ok value");
    operator
        .directory()
        .publish(&digest, &address, &signature)
        .or_panic("expected Ok value");
    assert_eq!(fs::read(&share).or_panic("expected Ok value"), original);
    let alias = digest_directory(&operator.cosign, &digest)
        .join(format!("{}.sig", hex::encode(&address).to_uppercase()));
    if alias != share {
        std::os::unix::fs::symlink(&share, &alias).or_panic("expected Ok value");
        assert_eq!(operator.directory().collect(&digest), vec![signature]);
    }
    assert!(
        fs::read_dir(digest_directory(&operator.cosign, &digest))
            .or_panic("expected Ok value")
            .all(|entry| !entry
                .or_panic("expected Ok value")
                .file_name()
                .to_string_lossy()
                .starts_with('.')),
        "no staging files after concurrent completion"
    );
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SuppliedInputs {
    inventory: PathBuf,
    operators: Vec<SuppliedOperator>,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SuppliedOperator {
    transport_config: PathBuf,
    relayer_config: PathBuf,
}

#[test]
fn supplied_paxeer_endpoints_build_a_relayer_rpc() {
    let fixture = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/bridge/deploy/relayer-solana.example.json");
    let mut value: serde_json::Value =
        serde_json::from_slice(&fs::read(fixture).or_panic("read relayer fixture config"))
            .or_panic("relayer fixture JSON");
    let root = fixture_directory("supplied-paxeer-endpoints");
    let ca = Ca::new(&root);
    let der = ca.cert.to_der().or_panic("Paxeer trust anchor DER");
    let anchor = root.join("paxeer-ca.der");
    fs::write(&anchor, &der).or_panic("write Paxeer trust anchor");
    value["paxeer"]["receipt_keys"] = serde_json::json!([{
        "sequencer_id": hex::prefixed(&[1; 32]),
        "public_key": hex::prefixed(&[2; 32]),
        "first_height": 0,
        "last_height": u64::MAX,
    }]);
    value["paxeer"]["endpoints"] = serde_json::json!([
        {
            "url": "https://paxeer.example.invalid/",
            "independent_backend": "pinned-test-node",
            "trust_anchor_der": anchor,
            "request_timeout_ms": 1234,
        },
        {
            "url": "http://127.0.0.1:8545",
            "independent_backend": "local-test-node",
            "local_emulator": true,
            "request_timeout_ms": 5678,
        },
    ]);
    let path = root.join("relayer.json");
    fs::write(
        &path,
        serde_json::to_vec(&value).or_panic("relayer config JSON"),
    )
    .or_panic("write relayer config");
    let relayer = RelayerConfig::load(&path).or_panic("load supplied relayer config");
    let endpoints = relayer
        .paxeer_endpoints()
        .or_panic("configured Paxeer endpoints");
    assert_eq!(endpoints.len(), 2);
    for (endpoint, configured) in endpoints.iter().zip(&relayer.paxeer.endpoints) {
        assert_eq!(endpoint.url, configured.url);
        assert_eq!(endpoint.expected_chain_id, relayer.paxeer.chain_id);
        assert_eq!(
            endpoint.request_timeout,
            Duration::from_millis(configured.request_timeout_ms)
        );
    }
    assert_eq!(
        endpoints[0].trust_anchor_der.as_deref(),
        Some(der.as_slice())
    );
    assert_eq!(endpoints[1].trust_anchor_der, None);
    assert!(
        relayer.paxeer_rpc().is_ok(),
        "configured Paxeer RPC adapter"
    );
    fs::remove_dir_all(root).or_panic("remove endpoint config fixture");
}

fn operator_credentials(root: &Path) -> (PathBuf, PathBuf) {
    let ca_der = root.join("ca.der");
    fs::write(
        &ca_der,
        X509::from_pem(&fs::read(root.join("ca.pem")).or_panic("expected Ok value"))
            .or_panic("expected Ok value")
            .to_der()
            .or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    let token = root.join("rpc.token");
    fs::write(&token, "operator-contract-rpc-token").or_panic("expected Ok value");
    fs::set_permissions(&token, fs::Permissions::from_mode(0o600)).or_panic("expected Ok value");
    (ca_der, token)
}

fn operator_inputs(root: &Path, operators: &[Operator], threshold: usize) -> SuppliedInputs {
    let mut members: Vec<_> = operators
        .iter()
        .map(|operator| hex::prefixed(&operator.address()))
        .collect();
    members.sort();
    let (ca_der, token) = operator_credentials(root);
    let membership = root.join("membership.json");
    fs::write(
        &membership,
        serde_json::json!({"attestors": members, "threshold": threshold}).to_string(),
    )
    .or_panic("expected Ok value");
    let mut records = Vec::new();
    let mut inputs = Vec::new();
    for (index, operator) in operators.iter().enumerate() {
        let ethereum_key = fixture_key(0x80 + u8::try_from(index).or_panic("expected Ok value"));
        let ethereum_handle = format!("ethereum-fee-payer-{index}");
        let mut relayer: serde_json::Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../services/bridge/deploy/relayer-solana.example.json"
        )))
        .or_panic("expected Ok value");
        relayer
            .as_object_mut()
            .or_panic("expected Some value")
            .remove("solana");
        relayer["chains"]
            .as_array_mut()
            .or_panic("expected Some value")
            .truncate(1);
        relayer["journal_path"] = serde_json::json!(operator.home.join("journal/relay.jsonl"));
        relayer["cosign_directory"] = serde_json::json!(operator.cosign);
        relayer["signer"]["endpoint"] =
            serde_json::json!({"transport": "uds", "socket": operator.socket});
        relayer["attestor"] = serde_json::json!({"handle": HANDLE, "public_key": hex::prefixed(&fixture_public_key(&operator.key))});
        relayer["attestor_set"] = serde_json::json!({"members": members, "threshold": threshold});
        relayer["paxeer"]["chain_id"] = serde_json::json!(125);
        relayer["paxeer"]["endpoints"] = serde_json::json!([{"url": "http://127.0.0.1:8545", "local_emulator": true, "request_timeout_ms": 2000}]);
        for endpoint in relayer["chains"][0]["rpc"]["endpoints"]
            .as_array_mut()
            .or_panic("expected Some value")
        {
            endpoint["ca_certificate_der"] = serde_json::json!(ca_der);
            endpoint["bearer_token_file"] = serde_json::json!(token);
        }
        relayer["paxeer"]["submitter"] = serde_json::json!({"handle": operator.fee_payer_handle, "public_key": hex::prefixed(&fixture_public_key(&operator.fee_payer))});
        relayer["chains"][0]["chain_id"] = serde_json::json!(1);
        relayer["chains"][0]["vault"] = serde_json::json!(hex::prefixed(&[0x11; 20]));
        relayer["chains"][0]["submitter"] = serde_json::json!({"handle": ethereum_handle, "public_key": hex::prefixed(&fixture_public_key(&ethereum_key))});
        let relayer_config = operator.home.join("relayer.json");
        fs::write(&relayer_config, relayer.to_string()).or_panic("expected Ok value");
        records.push(serde_json::json!({
            "instance": operator.name, "storage": operator.home.to_str().or_panic("expected Some value"),
            "attestor": hex::prefixed(&operator.address()), "signer_handle": HANDLE,
            "signer_public_key": hex::prefixed(&fixture_public_key(&operator.key)),
            "journal_path": operator.home.join("journal/relay.jsonl"), "cosign_directory": operator.cosign,
            "delivery_directory": operator.delivery, "spki_sha256": hex::prefixed(&operator.pin),
            "fee_payers": [
                {"chain_id": 125, "handle": operator.fee_payer_handle, "public_key": hex::prefixed(&fixture_public_key(&operator.fee_payer))},
                {"chain_id": 1, "handle": ethereum_handle, "public_key": hex::prefixed(&fixture_public_key(&ethereum_key))}
            ]
        }));
        inputs.push(SuppliedOperator {
            transport_config: operator.config.clone(),
            relayer_config,
        });
    }
    let inventory = root.join("inventory.json");
    fs::write(&inventory, serde_json::json!({
        "version": 1, "authority": "bridge", "approval": "in-test generated signer membership",
        "membership": membership, "operators": records,
        "destinations": [
            {"chain_id": 125, "vault": hex::prefixed(&layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE), "attestors": members, "threshold": threshold, "observed_block_hash": hex::prefixed(&[0x12; 32])},
            {"chain_id": 1, "vault": hex::prefixed(&[0x11; 20]), "attestors": members, "threshold": threshold, "observed_block_hash": hex::prefixed(&[0x13; 32])}
        ]
    }).to_string()).or_panic("expected Ok value");
    SuppliedInputs {
        inventory,
        operators: inputs,
    }
}

fn recorded_operator_policies(
    root: &Path,
    inventory: &OperatorInventory,
    relayer: &RelayerConfig,
) -> support::Recording {
    use layerx_bridge_relayer::abi;
    use serde_json::json;
    let mut endpoints = serde_json::Map::new();
    for policy in &inventory.destinations {
        let word = |value: u64| uint256_from_u64(value).to_vec();
        let addresses: Vec<u8> = policy
            .attestors
            .iter()
            .flat_map(|address| {
                abi::address_word(&hex::fixed::<20>(address).or_panic("expected Ok value"))
            })
            .collect();
        let list: Vec<u8> = [
            word(32),
            word(policy.attestors.len() as u64),
            addresses.clone(),
        ]
        .concat();
        let native: Vec<u8> = [
            word(96),
            word(128 + 32 * policy.attestors.len() as u64),
            word(policy.threshold as u64),
            word(policy.attestors.len() as u64),
            addresses,
            word(policy.attestors.len() as u64),
            policy.attestors.iter().flat_map(|_| word(1000)).collect(),
        ]
        .concat();
        let call = |data: Vec<u8>, result: Vec<u8>| json!({"method": "eth_call", "params": [{"to": policy.vault, "data": hex::prefixed(&data)}, "latest"], "result": hex::prefixed(&result)});
        let mut rules = vec![
            json!({"method": "eth_chainId", "params": [], "result": hex::quantity(policy.chain_id)}),
            json!({"method": "eth_getBlockByNumber", "params": ["latest", false], "result": {"hash": policy.observed_block_hash}}),
            call(abi::GET_ATTESTORS_SELECTOR.to_vec(), native),
            call(abi::ATTESTORS_SELECTOR.to_vec(), list),
            call(
                abi::THRESHOLD_SELECTOR.to_vec(),
                word(policy.threshold as u64),
            ),
        ];
        if policy.chain_id == relayer.paxeer.chain_id {
            for chain in &relayer.chains {
                rules.push(call(
                    abi::encode_get_chain(chain.chain_id),
                    [
                        word(1),
                        abi::address_word(&chain.vault).to_vec(),
                        word(chain.finality_depth),
                        word(1),
                    ]
                    .concat(),
                ));
            }
        }
        endpoints.insert(policy.chain_id.to_string(), json!({"*": rules}));
    }
    let path = root.join("policies.json");
    fs::write(&path, json!({"endpoints": endpoints}).to_string()).or_panic("expected Ok value");
    support::Recording::load(path.to_str().or_panic("expected Some value"))
}

#[test]
fn supplied_operator_inventory_matches_registered_bridge_configuration() {
    let root = fixture_directory("operator-inventory");
    let ca = Ca::new(&root);
    let (size, threshold) = declared_quorum();
    let operators: Vec<_> = (0..u8::try_from(size).or_panic("expected Ok value"))
        .map(|index| Operator::new(&root, &ca, index))
        .collect();
    for operator in &operators {
        operator.configure(
            &ca,
            &operators
                .iter()
                .filter(|peer| peer.name != operator.name)
                .collect::<Vec<_>>(),
        );
    }
    let inputs = operator_inputs(&root, &operators, threshold);
    let inventory =
        OperatorInventory::load(&inputs.inventory).or_panic("genuine approved bridge inventory");
    assert_eq!(
        inventory.operators.len(),
        size,
        "intended bridge membership cardinality"
    );
    assert_eq!(inputs.operators.len(), size);
    assert!(inventory
        .destinations
        .iter()
        .all(|policy| policy.threshold == threshold));
    let mut observed = std::collections::BTreeSet::new();
    let mut relayers = Vec::new();
    for operator in &inputs.operators {
        assert!(operator.transport_config.is_absolute() && operator.relayer_config.is_absolute());
        let transport = Transport::load(&operator.transport_config).unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "actual production TLS transport configuration"
            )
        });
        let relayer = RelayerConfig::load(&operator.relayer_config)
            .unwrap_or_else(|error| panic!("actual production relayer configuration: {error:?}"));
        transport
            .validate_operator(&inventory, &relayer)
            .unwrap_or_else(|error| {
                panic!("{}: {error:?}",
            "operator conforms to approved roster, independent handles and journal ownership",
        )
            });
        assert!(
            observed.insert(transport.attestor),
            "duplicate supplied operator"
        );
        relayers.push(relayer);
    }
    let relayer = &relayers[0];
    let endpoints = relayer
        .paxeer_endpoints()
        .or_panic("production Paxeer RPC endpoints");
    let _paxeer_transport = PaxeerRpc::new(endpoints).or_panic("production Paxeer RPC adapter");
    let recording = recorded_operator_policies(&root, &inventory, relayer);
    let paxeer = recording.endpoint(&relayer.paxeer.chain_id.to_string());
    let policy = inventory
        .destinations
        .iter()
        .find(|policy| policy.chain_id == relayer.paxeer.chain_id)
        .or_panic("Paxeer policy");
    verify_registered_evm_policy(paxeer.as_ref(), policy, true);
    for chain in &relayer.chains {
        let _quorum = RpcCluster::new(&chain.rpc).or_panic("production destination quorum RPC");
        let rpc = recording.endpoint(&chain.chain_id.to_string());
        let policy = inventory
            .destinations
            .iter()
            .find(|policy| policy.chain_id == chain.chain_id)
            .or_panic("destination policy");
        verify_registered_evm_policy(rpc.as_ref(), policy, false);
        let registration = layerx_bridge_relayer::abi::decode_get_chain(&policy_call(
            paxeer.as_ref(),
            &layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE,
            &layerx_bridge_relayer::abi::encode_get_chain(chain.chain_id),
        ))
        .or_panic("native chain registration");
        assert!(registration.registered && registration.enabled);
        assert_eq!(registration.vault, chain.vault);
        assert_eq!(registration.finality_depth, chain.finality_depth);
    }
    if let Some(config) = &relayer.solana {
        verify_registered_solana_policy(config, &inventory, paxeer.as_ref());
    }
    let raw: serde_json::Value =
        serde_json::from_slice(&fs::read(&inputs.inventory).or_panic("inventory bytes"))
            .or_panic("expected Ok value");
    assert_inventory_violations(&raw);
}

fn verify_registered_solana_policy(
    config: &layerx_bridge_relayer::config::SolanaConfig,
    inventory: &OperatorInventory,
    paxeer: &dyn JsonRpc,
) {
    use layerx_bridge_relayer::solana::{
        base58_fixed,
        release::{config_address, ConfigRecord},
        rpc::SolanaRpc,
    };
    let settings = config.settings().or_panic("actual Solana settings");
    let quorum = RpcCluster::new(&config.rpc).or_panic("production Solana quorum");
    let rpc = SolanaRpc::new(Box::new(
        RpcCluster::new(&config.rpc)
            .unwrap_or_else(|error| panic!("production Solana account adapter: {error:?}")),
    ));
    let slot = rpc
        .get_slot(settings.commitment)
        .or_panic("genuine Solana commitment slot");
    let block = quorum.call("getBlock", serde_json::json!([slot, {"commitment": settings.commitment.as_str(), "transactionDetails": "none", "rewards": false, "maxSupportedTransactionVersion": 0}])).or_panic("genuine Solana policy block");
    let hash = base58_fixed::<32>(block["blockhash"].as_str().or_panic("Solana blockhash"))
        .or_panic("canonical blockhash");
    let policy = inventory
        .destinations
        .iter()
        .find(|policy| policy.chain_id == config.chain_id)
        .or_panic("Solana policy inventory");
    assert_eq!(
        hash,
        hex::fixed::<32>(&policy.observed_block_hash).or_panic("Solana observation hash")
    );
    let address = config_address(&settings.program_id).or_panic("production config PDA");
    let account = rpc
        .get_account_info(&address, settings.commitment)
        .or_panic("genuine config PDA read")
        .or_panic("registered config PDA");
    assert_eq!(account.owner, settings.program_id);
    assert!(!account.executable);
    let current =
        ConfigRecord::decode(&account.data).or_panic("canonical registered Solana policy");
    assert!(!current.paused);
    assert_eq!(usize::from(current.threshold), policy.threshold);
    assert_eq!(
        current.attestors,
        policy
            .attestors
            .iter()
            .map(|value| hex::fixed::<20>(value).or_panic("approved attestor"))
            .collect::<Vec<_>>()
    );
    assert_eq!(
        rpc.get_slot(settings.commitment)
            .or_panic("stable Solana observation"),
        slot
    );
    let registration = layerx_bridge_relayer::abi::decode_get_chain(&policy_call(
        paxeer,
        &layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE,
        &layerx_bridge_relayer::abi::encode_get_chain(config.chain_id),
    ))
    .or_panic("native Solana registration");
    assert!(registration.registered && registration.enabled);
    assert_eq!(registration.vault, config.vault);
}

fn assert_inventory_violations(raw: &serde_json::Value) {
    for violation in [
        "wallet-authority",
        "duplicate-attestor",
        "shared-storage",
        "shared-fee-payer",
        "wrong-threshold",
        "unknown-field",
        "missing-operator",
    ] {
        let mut bad = raw.clone();
        match violation {
            "wallet-authority" => bad["authority"] = "wallet".into(),
            "duplicate-attestor" => {
                bad["operators"][1]["attestor"] = bad["operators"][0]["attestor"].clone();
            }
            "shared-storage" => {
                bad["operators"][1]["storage"] = bad["operators"][0]["storage"].clone();
            }
            "shared-fee-payer" => {
                bad["operators"][1]["fee_payers"][0]["public_key"] =
                    bad["operators"][0]["fee_payers"][0]["public_key"].clone();
            }
            "wrong-threshold" => bad["destinations"][0]["threshold"] = 1.into(),
            "unknown-field" => bad["private_key"] = "refused-field".into(),
            "missing-operator" => {
                bad["operators"]
                    .as_array_mut()
                    .or_panic("expected Some value")
                    .pop();
            }
            _ => unreachable!(),
        }
        let refused = match serde_json::from_value::<OperatorInventory>(bad) {
            Ok(inventory) => inventory.validate().is_err(),
            Err(_) => true,
        };
        assert!(refused, "accepted inventory violation {violation}");
    }
}

fn policy_call(rpc: &dyn JsonRpc, target: &[u8; 20], data: &[u8]) -> Vec<u8> {
    let result = rpc.call("eth_call", serde_json::json!([{"to": hex::prefixed(target), "data": hex::prefixed(data)}, "latest"]))
        .or_panic("authenticated real registered-policy call");
    hex::decode(result.as_str().or_panic("canonical policy response"))
        .or_panic("policy response hex")
}
fn verify_registered_evm_policy(rpc: &dyn JsonRpc, policy: &DestinationPolicy, paxeer: bool) {
    use layerx_bridge_relayer::abi;
    let chain_id = rpc
        .call("eth_chainId", serde_json::json!([]))
        .or_panic("actual destination domain");
    assert_eq!(
        hex::parse_quantity(chain_id.as_str().or_panic("chain id")).or_panic("chain id quantity"),
        policy.chain_id
    );
    let head = rpc
        .call("eth_getBlockByNumber", serde_json::json!(["latest", false]))
        .or_panic("actual policy observation head");
    assert_eq!(
        head["hash"].as_str(),
        Some(policy.observed_block_hash.as_str()),
        "supplied observation must be current"
    );
    let target = hex::fixed::<20>(&policy.vault).or_panic("registered policy target");
    let (members, threshold) = if paxeer {
        let set =
            abi::decode_get_attestors(&policy_call(rpc, &target, &abi::GET_ATTESTORS_SELECTOR))
                .unwrap_or_else(|error| panic!("canonical native attestor policy: {error:?}"));
        (
            set.signers,
            usize::try_from(set.threshold).or_panic("expected Ok value"),
        )
    } else {
        (
            abi::decode_address_list(&policy_call(rpc, &target, &abi::ATTESTORS_SELECTOR))
                .or_panic("canonical vault attestors"),
            usize::try_from(
                abi::decode_threshold(&policy_call(rpc, &target, &abi::THRESHOLD_SELECTOR))
                    .or_panic("canonical vault threshold"),
            )
            .or_panic("expected Ok value"),
        )
    };
    let expected = policy
        .attestors
        .iter()
        .map(|value| hex::fixed::<20>(value).or_panic("approved attestor address"))
        .collect::<Vec<_>>();
    assert_eq!(members, expected);
    assert_eq!(threshold, policy.threshold);
    assert_eq!(
        rpc.call("eth_getBlockByNumber", serde_json::json!(["latest", false]))
            .or_panic("stable policy observation")["hash"],
        head["hash"]
    );
}

fn tls_delivery(sender: &Operator, receiver: &Operator, wire: &[u8]) -> Option<u8> {
    let config: serde_json::Value =
        serde_json::from_slice(&fs::read(&sender.config).or_panic("real sender configuration"))
            .or_panic("expected Ok value");
    let connector = layerx_mirror::signer::tls_connector(
        &receiver.name,
        Path::new(
            config["trust_anchor"]
                .as_str()
                .or_panic("expected Some value"),
        ),
        &sender.tls_cert,
        &sender.tls_key,
    )
    .or_panic("real pinned TLS connector");
    let stream = TcpStream::connect_timeout(&receiver.listen(), Duration::from_secs(5))
        .or_panic("real transport listener");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .or_panic("expected Ok value");
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .or_panic("expected Ok value");
    let Ok(mut tls) = connector.connect(&receiver.name, stream) else {
        return None;
    };
    assert_eq!(
        spki_pin(&tls.ssl().peer_certificate().or_panic("server certificate"))
            .or_panic("expected Ok value"),
        receiver.pin
    );
    if tls.write_all(wire).and_then(|()| tls.flush()).is_err() {
        return None;
    }
    let mut reply = [0];
    match tls.read(&mut reply) {
        Ok(1) => Some(reply[0]),
        Ok(_) | Err(_) => None,
    }
}
fn hostile_transport_deliveries(sender: &Operator, receiver: &Operator, outsider: &Operator) {
    use layerx_bridge_relayer::cosign::transport::{encode_wire, REPLY_ALREADY_HELD, REPLY_STORED};
    let wanted = attestation(891);
    let digest = wanted.digest();
    let signature = sender
        .attestor()
        .sign_inbound(&wanted)
        .or_panic("actual signer share for wire cases");
    let valid = encode_wire(&receiver.address(), &digest, &sender.address(), &signature);
    assert_eq!(tls_delivery(sender, receiver, &valid), Some(REPLY_STORED));
    assert_eq!(
        tls_delivery(sender, receiver, &valid),
        Some(REPLY_ALREADY_HELD)
    );
    assert_eq!(receiver.collect(&digest), vec![signature]);
    for violation in [
        "wrong-digest",
        "wrong-signer",
        "wrong-recipient",
        "malformed-signature",
        "wrong-magic",
        "wrong-version",
        "oversized",
        "surplus",
    ] {
        let mut wire = valid.clone();
        match violation {
            "wrong-digest" => wire[30] ^= 1,
            "wrong-signer" => wire[62] ^= 1,
            "wrong-recipient" => wire[10] ^= 1,
            "malformed-signature" => wire[82..].fill(0),
            "wrong-magic" => wire[4] ^= 1,
            "wrong-version" => wire[8] ^= 1,
            "oversized" => wire[..4].copy_from_slice(&144_u32.to_be_bytes()),
            "surplus" => wire.push(1),
            _ => unreachable!(),
        }
        assert_eq!(
            tls_delivery(sender, receiver, &wire),
            None,
            "accepted wire violation {violation}"
        );
        assert_eq!(
            receiver.collect(&digest),
            vec![signature],
            "wire violation changed durable valid shares"
        );
    }
    let outsider_signature = outsider
        .attestor()
        .sign_inbound(&wanted)
        .or_panic("actual outsider signer");
    let unknown = encode_wire(
        &receiver.address(),
        &digest,
        &outsider.address(),
        &outsider_signature,
    );
    assert_eq!(tls_delivery(outsider, receiver, &unknown), None);
    assert!(!receiver.holds(&digest, &outsider.address()));
}

fn fixture_secret(last: u8) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[31] = last;
    bytes
}
fn fixture_key(last: u8) -> SigningKey {
    SigningKey::from_slice(&fixture_secret(last)).or_panic("qualification signing key")
}
fn fixture_public_key(key: &SigningKey) -> Vec<u8> {
    key.verifying_key()
        .to_encoded_point(true)
        .as_bytes()
        .to_vec()
}
fn fixture_directory(_name: &str) -> PathBuf {
    let mut unique = [0; 16];
    openssl::rand::rand_bytes(&mut unique).or_panic("private fixture directory identifier");
    let directory = std::env::temp_dir().join(format!(
        "lxbr-{}-{}",
        std::process::id(),
        hex::encode(&unique)
    ));
    fs::DirBuilder::new()
        .mode(0o700)
        .create(&directory)
        .or_panic("new private fixture directory");
    directory
}

#[allow(dead_code)]
#[path = "support/repair_22_33_68.rs"]
mod repair_22_33_68_support;

#[test]
fn repair_22_33_68_configured_root_transport() {
    repair_22_33_68_support::configured_root_transport();
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
