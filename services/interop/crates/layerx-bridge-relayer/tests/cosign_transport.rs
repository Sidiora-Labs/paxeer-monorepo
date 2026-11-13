//! Real-process qualification of the cosign share transport: three child
//! processes of the built binary, an ephemeral CA, generated attestor keys,
//! hostile wire inputs, a byte-flipping relay and kill -9 restarts.

use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    assemble_signatures, ethereum_address, to_attestor_signature, SignatureError,
};
use layerx_bridge_relayer::cosign::transport::{
    encode_wire, spki_pin, ENABLE_VARIABLE, REPLY_ALREADY_HELD,
};
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use openssl::asn1::Asn1Time;
use openssl::bn::{BigNum, MsbOption};
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkey::{PKey, Private};
use openssl::ssl::{SslConnector, SslMethod, SslVerifyMode, SslVersion};
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509Name, X509};
use sha2::{Digest as _, Sha256};

const BIN: &str = env!("CARGO_BIN_EXE_layerx-bridge-cosign");
const WAIT: Duration = Duration::from_secs(60);

fn temp_root(tag: &str) -> PathBuf {
    let root = std::env::temp_dir().join(format!(
        "cosign-transport-{tag}-{}-{}",
        std::process::id(),
        hex::encode(&random::<8>())
    ));
    fs::create_dir_all(&root).or_panic("expected Ok value");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).or_panic("expected Ok value");
    root
}

fn random<const N: usize>() -> [u8; N] {
    let mut bytes = [0_u8; N];
    openssl::rand::rand_bytes(&mut bytes).or_panic("expected Ok value");
    bytes
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

struct Ca {
    cert: X509,
    key: PKey<Private>,
    path: PathBuf,
}

fn make_ca(root: &Path, name: &str) -> Ca {
    let key = ec_key();
    let cert = certificate(name, &key, None);
    let path = root.join(format!("{name}.pem"));
    fs::write(&path, cert.to_pem().or_panic("expected Ok value")).or_panic("expected Ok value");
    Ca { cert, key, path }
}

struct Identity {
    cert_path: PathBuf,
    key_path: PathBuf,
    pin: [u8; 32],
}

fn issue(root: &Path, ca: &Ca, name: &str, tag: &str) -> Identity {
    let dir = root.join(format!("secrets-{name}-{tag}"));
    fs::create_dir_all(&dir).or_panic("expected Ok value");
    fs::set_permissions(&dir, fs::Permissions::from_mode(0o700)).or_panic("expected Ok value");
    let key = ec_key();
    let cert = certificate(name, &key, Some((&ca.cert, &ca.key)));
    let cert_path = dir.join("cert.pem");
    let key_path = dir.join("key.pem");
    fs::write(&cert_path, cert.to_pem().or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    fs::write(
        &key_path,
        key.private_key_to_pem_pkcs8().or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    fs::set_permissions(&key_path, fs::Permissions::from_mode(0o600)).or_panic("expected Ok value");
    fs::set_permissions(&cert_path, fs::Permissions::from_mode(0o600))
        .or_panic("expected Ok value");
    Identity {
        cert_path,
        key_path,
        pin: spki_pin(&cert).or_panic("expected Ok value"),
    }
}

struct Node {
    name: String,
    signing: SigningKey,
    attestor: [u8; 20],
    port: u16,
    cosign: PathBuf,
    delivery: PathBuf,
    identity: Identity,
    config: PathBuf,
    log: PathBuf,
    child: Option<Child>,
}

impl Node {
    fn new(root: &Path, ca: &Ca, name: &str) -> Self {
        let signing =
            SigningKey::from_slice(&Sha256::digest(random::<32>())).or_panic("expected Ok value");
        let attestor = ethereum_address(signing.verifying_key());
        let base = root.join(name);
        let cosign = base.join("cosign");
        let delivery = base.join("delivery");
        fs::create_dir_all(&cosign).or_panic("expected Ok value");
        fs::create_dir_all(&delivery).or_panic("expected Ok value");
        Self {
            name: name.to_owned(),
            signing,
            attestor,
            port: free_port(),
            cosign,
            delivery,
            identity: issue(root, ca, name, "1"),
            config: base.join("transport.json"),
            log: base.join("stderr.log"),
            child: None,
        }
    }

    fn address(&self) -> SocketAddr {
        SocketAddr::from(([127, 0, 0, 1], self.port))
    }

    fn sign(&self, digest: &[u8; 32]) -> [u8; 65] {
        let (signature, recovery) = self
            .signing
            .sign_prehash_recoverable(digest)
            .or_panic("expected Ok value");
        let mut bytes = [0_u8; 65];
        bytes[..64].copy_from_slice(&signature.to_bytes());
        bytes[64] = recovery.to_byte();
        to_attestor_signature(bytes).or_panic("expected Ok value")
    }

    fn directory(&self) -> CosignDirectory {
        CosignDirectory::new(self.cosign.clone())
    }

    fn publish(&self, digest: &[u8; 32]) -> [u8; 65] {
        let signature = self.sign(digest);
        self.directory()
            .publish(digest, &self.attestor, &signature)
            .or_panic("expected Ok value");
        signature
    }

    fn write_config(&self, ca: &Ca, peers: &[(&Node, [u8; 32])]) {
        let peers: Vec<_> = peers
            .iter()
            .map(|(peer, pin)| {
                serde_json::json!({
                    "attestor": hex::prefixed(&peer.attestor),
                    "address": peer.address().to_string(),
                    "server_name": peer.name,
                    "spki_sha256": hex::prefixed(pin),
                })
            })
            .collect();
        let config = serde_json::json!({
            "attestor": hex::prefixed(&self.attestor),
            "listen": self.address().to_string(),
            "server_name": self.name,
            "trust_anchor": ca.path,
            "certificate": self.identity.cert_path,
            "private_key": self.identity.key_path,
            "cosign_directory": self.cosign,
            "delivery_directory": self.delivery,
            "peers": peers,
            "timeout_ms": 2000,
            "sweep_interval_ms": 100,
            "max_connections": 16,
        });
        fs::write(
            &self.config,
            serde_json::to_vec_pretty(&config).or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
    }

    fn start(&mut self) {
        assert!(self.child.is_none());
        let log = fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log)
            .or_panic("expected Ok value");
        let child = Command::new(BIN)
            .env(ENABLE_VARIABLE, &self.config)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(log)
            .spawn()
            .or_panic("expected Ok value");
        self.child = Some(child);
        let deadline = Instant::now() + WAIT;
        while TcpStream::connect(self.address()).is_err() {
            assert!(Instant::now() < deadline, "{} never listened", self.name);
            assert!(
                self.child
                    .as_mut()
                    .or_panic("expected Some value")
                    .try_wait()
                    .or_panic("expected Ok value")
                    .is_none(),
                "{} exited: {}",
                self.name,
                fs::read_to_string(&self.log).unwrap_or_default()
            );
            std::thread::sleep(Duration::from_millis(20));
        }
    }

    fn kill(&mut self) {
        let mut child = self.child.take().or_panic("expected Some value");
        child.kill().or_panic("expected Ok value");
        child.wait().or_panic("expected Ok value");
    }

    fn holds(&self, digest: &[u8; 32], signer: &[u8; 20]) -> Option<String> {
        fs::read_to_string(
            self.cosign
                .join(hex::encode(digest))
                .join(format!("{}.sig", hex::encode(signer))),
        )
        .ok()
    }

    fn record(
        &self,
        peer: &[u8; 20],
        pin: &[u8; 32],
        digest: &[u8; 32],
        own: &[u8; 65],
        suffix: &str,
    ) -> PathBuf {
        let mut identity = Sha256::new();
        identity.update(self.attestor);
        identity.update(own);
        self.delivery.join(format!(
            "{}-{}-{}-{}.{suffix}",
            hex::encode(peer),
            hex::encode(pin),
            hex::encode(digest),
            hex::encode(&identity.finalize())
        ))
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .or_panic("expected Ok value")
        .local_addr()
        .or_panic("expected Ok value")
        .port()
}

fn wait_for(what: &str, mut done: impl FnMut() -> bool) {
    let deadline = Instant::now() + WAIT;
    while !done() {
        assert!(Instant::now() < deadline, "timed out waiting for {what}");
        std::thread::sleep(Duration::from_millis(50));
    }
}

fn snapshot(root: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    let mut files = BTreeMap::new();
    let mut stack = vec![root.to_path_buf()];
    while let Some(dir) = stack.pop() {
        for entry in fs::read_dir(&dir).or_panic("expected Ok value").flatten() {
            let path = entry.path();
            if path.is_dir() {
                files.insert(path.clone(), Vec::new());
                stack.push(path);
            } else {
                files.insert(path.clone(), fs::read(&path).unwrap_or_default());
            }
        }
    }
    files
}

fn raw_client(ca: &Path, identity: &Identity) -> SslConnector {
    let mut builder = SslConnector::builder(SslMethod::tls_client()).or_panic("expected Ok value");
    builder
        .set_min_proto_version(Some(SslVersion::TLS1_3))
        .or_panic("expected Ok value");
    builder.set_ca_file(ca).or_panic("expected Ok value");
    builder
        .set_certificate_file(&identity.cert_path, openssl::ssl::SslFiletype::PEM)
        .or_panic("expected Ok value");
    builder
        .set_private_key_file(&identity.key_path, openssl::ssl::SslFiletype::PEM)
        .or_panic("expected Ok value");
    builder.set_verify(SslVerifyMode::PEER);
    builder.build()
}

/// Sends raw bytes over mutual TLS and returns the reply byte, if any.
fn exchange(connector: &SslConnector, address: SocketAddr, name: &str, wire: &[u8]) -> Option<u8> {
    let stream = TcpStream::connect(address).ok()?;
    stream
        .set_read_timeout(Some(Duration::from_secs(10)))
        .or_panic("expected Ok value");
    let mut tls = connector.connect(name, stream).ok()?;
    if tls.write_all(wire).and_then(|()| tls.flush()).is_err() {
        return None;
    }
    let mut reply = [0_u8; 1];
    match tls.read(&mut reply) {
        Ok(1) => Some(reply[0]),
        _ => None,
    }
}

/// A TCP relay that flips one byte in every client-to-server chunk after the
/// first (the `ClientHello`), so every protected record is modified.
fn flipping_relay(target: SocketAddr) -> SocketAddr {
    let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
    let address = listener.local_addr().or_panic("expected Ok value");
    std::thread::spawn(move || {
        for client in listener.incoming().flatten() {
            let Ok(server) = TcpStream::connect(target) else {
                continue;
            };
            let (mut client_read, mut server_write) = (
                client.try_clone().or_panic("expected Ok value"),
                server.try_clone().or_panic("expected Ok value"),
            );
            let (mut server_read, mut client_write) = (server, client);
            std::thread::spawn(move || {
                let mut buffer = [0_u8; 16384];
                let mut chunks = 0_usize;
                while let Ok(read) = client_read.read(&mut buffer) {
                    if read == 0 {
                        break;
                    }
                    if chunks > 0 {
                        buffer[read / 2] ^= 0x01;
                    }
                    chunks += 1;
                    if server_write.write_all(&buffer[..read]).is_err() {
                        break;
                    }
                }
                let _ = server_write.shutdown(std::net::Shutdown::Both);
            });
            std::thread::spawn(move || {
                let _ = std::io::copy(&mut server_read, &mut client_write);
                let _ = client_write.shutdown(std::net::Shutdown::Both);
            });
        }
    });
    address
}

fn digest(label: &str) -> [u8; 32] {
    Sha256::digest(label.as_bytes()).into()
}

#[test]
fn three_processes_deliver_authenticated_durable_shares() {
    let root = temp_root("main");
    let ca = make_ca(&root, "cosign-test-ca");
    let mut a = Node::new(&root, &ca, "node-a.cosign.test");
    let mut b = Node::new(&root, &ca, "node-b.cosign.test");
    let mut c = Node::new(&root, &ca, "node-c.cosign.test");
    let attestors = [a.attestor, b.attestor, c.attestor];
    let (pa, pb, pc) = (a.identity.pin, b.identity.pin, c.identity.pin);
    a.write_config(&ca, &[(&b, pb), (&c, pc)]);
    b.write_config(&ca, &[(&a, pa), (&c, pc)]);
    c.write_config(&ca, &[(&a, pa), (&b, pb)]);

    // ac_1/ac_5: two live processes stay below threshold; C is unavailable.
    let d = digest("withdrawal-1");
    let sig_a = a.publish(&d);
    let sig_b = b.publish(&d);
    let sig_c = c.publish(&d);
    // ac_5/ac_4: C already holds different bytes for A's share of d2.
    let d2 = digest("withdrawal-2");
    let held = c.cosign.join(hex::encode(&d2));
    fs::create_dir_all(&held).or_panic("expected Ok value");
    let conflicting = format!("{}.sig", hex::encode(&a.attestor));
    fs::write(held.join(&conflicting), b"0xprevious-bytes").or_panic("expected Ok value");
    let sig_a2 = a.publish(&d2);

    assert_two_node_threshold(&mut a, &mut b, &attestors, &d, &sig_a, &pb, &pc);

    assert_three_way_exchange(
        (&mut a, &b, &mut c),
        &attestors,
        &d,
        (sig_a, sig_b, sig_c),
        &pc,
    );

    assert_durable_conflict(&mut a, &c, &d2, &sig_a2, &held, &conflicting, &pc);

    // ac_4: identical redelivery is idempotent.
    let a_client = raw_client(&ca.path, &a.identity);
    let before = snapshot(&b.cosign);
    assert_eq!(
        exchange(
            &a_client,
            b.address(),
            &b.name,
            &encode_wire(&b.attestor, &d, &a.attestor, &sig_a)
        ),
        Some(REPLY_ALREADY_HELD)
    );
    assert_eq!(snapshot(&b.cosign), before);

    assert_hostile_frames(&root, &ca, &a, &b, &c, &a_client);

    assert_certificate_rotation(&root, &ca, &mut a, &b, &mut c, (pa, pb, pc), (&d, &sig_a));

    assert_fair_sweeps(&a, &b, &pb);

    assert_public_storage(&a, &b, &c);

    drop((a, b, c));
    let _ = fs::remove_dir_all(&root);
}

fn assert_two_node_threshold(
    a: &mut Node,
    b: &mut Node,
    attestors: &[[u8; 20]; 3],
    d: &[u8; 32],
    sig_a: &[u8; 65],
    pb: &[u8; 32],
    pc: &[u8; 32],
) {
    a.start();
    b.start();
    wait_for("A and B exchange", || {
        a.holds(d, &b.attestor).is_some() && b.holds(d, &a.attestor).is_some()
    });
    for node in [&*a, &*b] {
        assert!(matches!(
            assemble_signatures(d, node.directory().collect(d), attestors, 3),
            Err(SignatureError::BelowThreshold {
                signatures: 2,
                threshold: 3
            })
        ));
    }
    wait_for("A ack from B", || {
        a.record(&b.attestor, pb, d, sig_a, "ack").exists()
    });
    assert!(!a.record(&attestors[2], pc, d, sig_a, "ack").exists());
}

fn assert_three_way_exchange(
    nodes: (&mut Node, &Node, &mut Node),
    attestors: &[[u8; 20]; 3],
    d: &[u8; 32],
    signatures: ([u8; 65], [u8; 65], [u8; 65]),
    pc: &[u8; 32],
) {
    let (a, b, c) = nodes;
    let (sig_a, sig_b, sig_c) = signatures;
    // ac_4/ac_5: kill -9 A, bring C up, restart A on the same directories.
    a.kill();
    c.start();
    a.start();
    wait_for("three-way exchange", || {
        attestors.iter().all(|signer| {
            [&*a, b, &*c]
                .iter()
                .all(|node| node.holds(d, signer).is_some())
        })
    });
    for node in [&*a, b, &*c] {
        let assembled = assemble_signatures(d, node.directory().collect(d), attestors, 3)
            .or_panic("expected Ok value");
        assert_eq!(assembled.len(), 3);
        for signature in [sig_a, sig_b, sig_c] {
            assert!(assembled.contains(&signature));
        }
    }
    wait_for("A ack from C after restart", || {
        a.record(&c.attestor, pc, d, &sig_a, "ack").exists()
    });
}

fn assert_durable_conflict(
    a: &mut Node,
    c: &Node,
    d2: &[u8; 32],
    sig_a2: &[u8; 65],
    held: &Path,
    conflicting: &str,
    pc: &[u8; 32],
) {
    // Durable explicit conflict, existing bytes never overwritten.
    wait_for("conflict record", || {
        a.record(&c.attestor, pc, d2, sig_a2, "conflict").exists()
    });
    assert_eq!(
        fs::read(held.join(conflicting)).or_panic("expected Ok value"),
        b"0xprevious-bytes"
    );
    assert!(!a.record(&c.attestor, pc, d2, sig_a2, "ack").exists());
    a.kill();
    a.start();
    std::thread::sleep(Duration::from_millis(600));
    assert!(a.record(&c.attestor, pc, d2, sig_a2, "conflict").exists());
    assert_eq!(
        fs::read(held.join(conflicting)).or_panic("expected Ok value"),
        b"0xprevious-bytes"
    );
}

struct HostileCase {
    digest: [u8; 32],
    good: Vec<u8>,
    frames: Vec<(&'static str, SslConnector, Vec<u8>)>,
}

fn hostile_case(root: &Path, ca: &Ca, a: &Node, b: &Node, c: &Node) -> HostileCase {
    let d3 = digest("withdrawal-3");
    let sig_a3 = a.sign(&d3);
    let good = encode_wire(&b.attestor, &d3, &a.attestor, &sig_a3);
    let foreign_ca = make_ca(root, "foreign-ca");
    let foreign = issue(root, &foreign_ca, "node-a.cosign.test", "foreign");
    let unpinned = issue(root, ca, "node-x.cosign.test", "unpinned");
    let mut hostile: Vec<(&str, SslConnector, Vec<u8>)> = Vec::new();
    let foreign_client = {
        let mut builder =
            SslConnector::builder(SslMethod::tls_client()).or_panic("expected Ok value");
        builder
            .set_min_proto_version(Some(SslVersion::TLS1_3))
            .or_panic("expected Ok value");
        builder.set_ca_file(&ca.path).or_panic("expected Ok value");
        builder
            .set_certificate_file(&foreign.cert_path, openssl::ssl::SslFiletype::PEM)
            .or_panic("expected Ok value");
        builder
            .set_private_key_file(&foreign.key_path, openssl::ssl::SslFiletype::PEM)
            .or_panic("expected Ok value");
        builder.build()
    };
    hostile.push(("foreign CA", foreign_client, good.clone()));
    hostile.push((
        "unpinned client",
        raw_client(&ca.path, &unpinned),
        good.clone(),
    ));
    hostile.push((
        "wrong recipient",
        raw_client(&ca.path, &a.identity),
        encode_wire(&c.attestor, &d3, &a.attestor, &sig_a3),
    ));
    let other_signature = c.sign(&d3);
    hostile.push((
        "wrong signer",
        raw_client(&ca.path, &a.identity),
        encode_wire(&b.attestor, &d3, &c.attestor, &other_signature),
    ));
    let mut flipped = sig_a3;
    flipped[10] ^= 0x55;
    hostile.push((
        "invalid signature",
        raw_client(&ca.path, &a.identity),
        encode_wire(&b.attestor, &d3, &a.attestor, &flipped),
    ));
    hostile.push((
        "invalid digest",
        raw_client(&ca.path, &a.identity),
        encode_wire(&b.attestor, &digest("other"), &a.attestor, &sig_a3),
    ));
    let mut short = good.clone();
    short[..4].copy_from_slice(&142_u32.to_be_bytes());
    short.pop();
    hostile.push(("short length", raw_client(&ca.path, &a.identity), short));
    let mut oversize = good.clone();
    oversize[..4].copy_from_slice(&144_u32.to_be_bytes());
    oversize.push(0);
    hostile.push(("oversize", raw_client(&ca.path, &a.identity), oversize));
    let mut huge = good.clone();
    huge[..4].copy_from_slice(&u32::MAX.to_be_bytes());
    hostile.push(("huge length", raw_client(&ca.path, &a.identity), huge));
    hostile.push((
        "truncated",
        raw_client(&ca.path, &a.identity),
        good[..100].to_vec(),
    ));
    let mut magic = good.clone();
    magic[4] = b'X';
    hostile.push(("bad magic", raw_client(&ca.path, &a.identity), magic));
    let mut version = good.clone();
    version[9] = 2;
    hostile.push(("bad version", raw_client(&ca.path, &a.identity), version));
    let mut surplus = good.clone();
    surplus.push(0);
    hostile.push(("surplus", raw_client(&ca.path, &a.identity), surplus));
    HostileCase {
        digest: d3,
        good,
        frames: hostile,
    }
}

fn assert_hostile_frames(
    root: &Path,
    ca: &Ca,
    a: &Node,
    b: &Node,
    c: &Node,
    a_client: &SslConnector,
) {
    // ac_2/ac_3: hostile inputs leave no share or acknowledgement writes.
    let case = hostile_case(root, ca, a, b, c);
    let d3 = case.digest;
    let good = case.good;
    let hostile = case.frames;
    let shared_before = (
        snapshot(&b.cosign),
        snapshot(&b.delivery),
        snapshot(&a.delivery),
    );
    for (label, connector, wire) in &hostile {
        assert_eq!(
            exchange(connector, b.address(), &b.name, wire),
            None,
            "{label} was answered"
        );
    }
    // ac_3: modified ciphertext through a byte-flipping relay is refused.
    let relay = flipping_relay(b.address());
    assert_eq!(
        exchange(a_client, relay, &b.name, &good),
        None,
        "relay modification answered"
    );
    // Hostname verification: B's certificate does not name node-c.
    assert_eq!(exchange(a_client, b.address(), &c.name, &good), None);
    assert!(b.holds(&d3, &a.attestor).is_none());
    assert!(b.holds(&d3, &c.attestor).is_none());
    assert_eq!(
        (
            snapshot(&b.cosign),
            snapshot(&b.delivery),
            snapshot(&a.delivery)
        ),
        shared_before
    );
    // The untouched good frame is still admitted afterwards.
    assert_eq!(exchange(a_client, b.address(), &b.name, &good), Some(1));
    assert!(b.holds(&d3, &a.attestor).is_some());
}

fn assert_certificate_rotation(
    root: &Path,
    ca: &Ca,
    a: &mut Node,
    b: &Node,
    c: &mut Node,
    pins: ([u8; 32], [u8; 32], [u8; 32]),
    share: (&[u8; 32], &[u8; 65]),
) {
    let (pa, pb, pc) = pins;
    let (d, sig_a) = share;
    // ac_2/ac_5: C rotates its certificate; A and B still pin the old key, so
    // A refuses C as a server and nothing for d4 reaches C.
    let d4 = digest("withdrawal-4");
    let sig_a4 = a.publish(&d4);
    c.kill();
    c.identity = issue(root, ca, "node-c.cosign.test", "2");
    let pc2 = c.identity.pin;
    assert_ne!(pc, pc2);
    c.write_config(ca, &[(&*a, pa), (b, pb)]);
    c.start();
    std::thread::sleep(Duration::from_secs(2));
    assert!(c.holds(&d4, &a.attestor).is_none());
    assert!(!a.record(&c.attestor, &pc, &d4, &sig_a4, "ack").exists());
    assert!(!a.record(&c.attestor, &pc2, &d4, &sig_a4, "ack").exists());
    // A learns the new pin; old-pin acknowledgements are not inherited, so d
    // is re-offered under the new pin and acknowledged afresh.
    a.kill();
    a.write_config(ca, &[(b, pb), (&*c, pc2)]);
    a.start();
    wait_for("delivery under the new pin", || {
        a.record(&c.attestor, &pc2, &d4, &sig_a4, "ack").exists()
            && a.record(&c.attestor, &pc2, d, sig_a, "ack").exists()
    });
    assert!(a.record(&c.attestor, &pc, d, sig_a, "ack").exists());
    assert!(c.holds(&d4, &a.attestor).is_some());
}

fn assert_fair_sweeps(a: &Node, b: &Node, pb: &[u8; 32]) {
    // ac_5: bounded fair sweeps reach entries beyond the first page.
    let mut page = Vec::new();
    for index in 0..300 {
        let entry = digest(&format!("page-{index}"));
        page.push((entry, a.publish(&entry)));
    }
    page.sort_unstable_by_key(|entry| entry.0);
    wait_for("all 300 entries at B", || {
        page.iter()
            .all(|(entry, _)| b.holds(entry, &a.attestor).is_some())
    });
    let (last, last_sig) = page[299];
    wait_for("ack beyond the first page", || {
        a.record(&b.attestor, pb, &last, &last_sig, "ack").exists()
    });
}

fn assert_public_storage(a: &Node, b: &Node, c: &Node) {
    // ac_6: shared directories hold signatures and public metadata only.
    for node in [a, b, c] {
        for dir in [&node.cosign, &node.delivery] {
            for (path, bytes) in snapshot(dir) {
                assert!(
                    !String::from_utf8_lossy(&bytes).contains("PRIVATE KEY"),
                    "{} holds key material",
                    path.display()
                );
            }
        }
    }
}

#[test]
fn enablement_and_bounds_are_enforced() {
    let root = temp_root("config");
    let ca = make_ca(&root, "config-ca");
    let a = Node::new(&root, &ca, "node-a.cosign.test");
    let b = Node::new(&root, &ca, "node-b.cosign.test");
    let run = |variable: Option<&Path>| {
        let mut command = Command::new(BIN);
        command
            .env_remove(ENABLE_VARIABLE)
            .stdout(Stdio::null())
            .stderr(Stdio::null());
        if let Some(path) = variable {
            command.env(ENABLE_VARIABLE, path);
        }
        command.status().or_panic("expected Ok value").code()
    };
    // Not enabled: the binary refuses to act as a transport at all.
    assert_eq!(run(None), Some(2));
    // ac_7: configured but missing, unreadable or invalid is fatal.
    assert_eq!(run(Some(&root.join("missing.json"))), Some(1));
    fs::write(root.join("invalid.json"), b"{not json").or_panic("expected Ok value");
    assert_eq!(run(Some(&root.join("invalid.json"))), Some(1));
    a.write_config(&ca, &[(&b, b.identity.pin)]);
    let valid: serde_json::Value =
        serde_json::from_slice(&fs::read(&a.config).or_panic("expected Ok value"))
            .or_panic("expected Ok value");
    // ac_6: every bound is finite and validated.
    for (field, value) in [
        ("timeout_ms", serde_json::json!(0)),
        ("timeout_ms", serde_json::json!(60_001)),
        ("sweep_interval_ms", serde_json::json!(0)),
        ("max_connections", serde_json::json!(0)),
        ("max_connections", serde_json::json!(65)),
        ("peers", serde_json::json!([])),
        ("unknown_field", serde_json::json!(1)),
    ] {
        let mut config = valid.clone();
        config[field] = value;
        let path = root.join(format!("bad-{field}.json"));
        fs::write(
            &path,
            serde_json::to_vec(&config).or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
        assert_eq!(run(Some(&path)), Some(1), "{field} accepted");
    }
    let mut duplicate = valid.clone();
    let peer = duplicate["peers"][0].clone();
    duplicate["peers"]
        .as_array_mut()
        .or_panic("expected Some value")
        .push(peer);
    let path = root.join("duplicate.json");
    fs::write(
        &path,
        serde_json::to_vec(&duplicate).or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    assert_eq!(run(Some(&path)), Some(1));
    // Private key readable by others, or placed in a shared directory.
    fs::set_permissions(&a.identity.key_path, fs::Permissions::from_mode(0o644))
        .or_panic("expected Ok value");
    assert_eq!(run(Some(&a.config)), Some(1));
    fs::set_permissions(&a.identity.key_path, fs::Permissions::from_mode(0o600))
        .or_panic("expected Ok value");
    let shared_key = a.cosign.join("key.pem");
    fs::copy(&a.identity.key_path, &shared_key).or_panic("expected Ok value");
    fs::set_permissions(&shared_key, fs::Permissions::from_mode(0o600))
        .or_panic("expected Ok value");
    let mut inside = valid;
    inside["private_key"] = serde_json::json!(shared_key);
    let path = root.join("inside.json");
    fs::write(
        &path,
        serde_json::to_vec(&inside).or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    assert_eq!(run(Some(&path)), Some(1));
    drop((a, b));
    let _ = fs::remove_dir_all(&root);
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
