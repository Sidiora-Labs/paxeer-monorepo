use layerx_client::lni::framing::{read_frame, write_frame};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_types::clock::Deadline;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use std::error::Error;
use std::fs;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

struct Runtime {
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<std::io::Result<()>>>,
}

impl Drop for Runtime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const MAX: usize = 2_097_152;
struct Host {
    root: PathBuf,
    address: SocketAddr,
    child: Option<Child>,
    runtime: Option<Runtime>,
    binding: ClockBinding,
    clock: Arc<RuntimeClock>,
}
impl Drop for Host {
    fn drop(&mut self) {
        self.stop();
        drop(self.runtime.take());
        let _ = fs::remove_dir_all(&self.root);
    }
}
impl Host {
    fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut nonce = [0; 8];
        getrandom::fill(&mut nonce).map_err(|error| std::io::Error::other(error.to_string()))?;
        let root = std::env::temp_dir().join(format!(
            "lxkp-{}-{:x}",
            std::process::id(),
            u64::from_be_bytes(nonce)
        ));
        fs::create_dir(&root)?;
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
        let mut clock_server = runtime_clock_server::Server::bind(&root)?;
        let binding = ClockBinding {
            socket: clock_server.socket().to_path_buf(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let runtime = Runtime {
            stop,
            worker: Some(std::thread::spawn(move || {
                while !stopping.load(Ordering::Acquire) {
                    clock_server.poll()?;
                    std::thread::sleep(Duration::from_millis(1));
                }
                Ok(())
            })),
        };
        let clock = RuntimeClock::connect(binding.clone())?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        drop(listener);
        let mut host = Self {
            root,
            address,
            child: None,
            runtime: Some(runtime),
            binding,
            clock,
        };
        host.provision_tls()?;
        let mut seal = [0; 32];
        getrandom::fill(&mut seal).map_err(|error| std::io::Error::other(error.to_string()))?;
        fs::write(host.root.join("seal"), seal)?;
        fs::write(host.root.join("registry.json"), br#"{"chain_id":125}"#)?;
        for entry in fs::read_dir(&host.root)? {
            let entry = entry?;
            if entry.file_type()?.is_file() {
                fs::set_permissions(entry.path(), fs::Permissions::from_mode(0o600))?;
            }
        }
        host.start()?;
        Ok(host)
    }
    fn provision_tls(&self) -> Result<()> {
        self.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "ca.key",
            "-out",
            "ca.pem",
            "-days",
            "1",
            "-subj",
            "/CN=LXKP test CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
        ])?;
        for name in ["server", "client", "foreign"] {
            self.openssl(&[
                "req",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.csr"),
                "-subj",
                &format!("/CN={name}"),
            ])?;
            fs::write(
                self.root.join("extensions"),
                if name == "server" {
                    "subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
                } else {
                    "extendedKeyUsage=clientAuth\n"
                },
            )?;
            self.openssl(&[
                "x509",
                "-req",
                "-in",
                &format!("{name}.csr"),
                "-CA",
                "ca.pem",
                "-CAkey",
                "ca.key",
                "-CAcreateserial",
                "-out",
                &format!("{name}.pem"),
                "-days",
                "1",
                "-extfile",
                "extensions",
            ])?;
            self.openssl(&[
                "x509",
                "-in",
                &format!("{name}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.der"),
            ])?;
            self.openssl(&[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                &format!("{name}.key"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}-key.der"),
            ])?;
        }
        self.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
        Ok(())
    }
    fn openssl(&self, arguments: &[&str]) -> Result<()> {
        let result = Command::new("openssl")
            .args(arguments)
            .current_dir(&self.root)
            .output()?;
        if !result.status.success() {
            return Err(format!(
                "openssl failed: {}",
                String::from_utf8_lossy(&result.stderr)
            )
            .into());
        }
        Ok(())
    }
    fn launch(&self) -> Result<Child> {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.binding.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", self.binding.process.to_string())
            .env("LAYERX_RUNTIME_CLOCK_UID", self.binding.user.to_string())
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", "beta-kms")
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "2")
            .env(
                "LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER",
                self.root.join("foreign.der"),
            );
        for (suffix, file) in [
            ("REGISTRY_FILE", "registry.json"),
            ("CLIENT_CA_DER", "ca.der"),
            ("TLS_CERT_DER", "server.der"),
            ("TLS_KEY_DER", "server-key.der"),
            ("CLIENT_CERT_DER", "client.der"),
            ("SEAL_SECRET_FILE", "seal"),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.root.join(file));
        }
        Ok(command
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()?)
    }
    fn start(&mut self) -> Result<()> {
        self.child = Some(self.launch()?);
        let mut deadline = Deadline::start(self.clock.as_ref(), Duration::from_secs(10))?;
        loop {
            if self
                .call(&request(0, [0; 32], &[], None)?)
                .is_ok_and(|response| response == b"LXKP\0\x01\0\0")
            {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing process")?
                .try_wait()?
                .is_some()
            {
                return Err("KMS exited at startup".into());
            }
            if deadline.remaining(self.clock.as_ref())?.is_zero() {
                return Err("KMS startup deadline".into());
            }
            std::thread::sleep(Duration::from_millis(20));
        }
    }
    fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
    fn roots(&self) -> Result<RootCertStore> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(self.root.join("ca.der"))?))?;
        Ok(roots)
    }
    fn identity(
        &self,
        name: &str,
    ) -> Result<(Vec<CertificateDer<'static>>, PrivateKeyDer<'static>)> {
        Ok((
            vec![CertificateDer::from(fs::read(
                self.root.join(format!("{name}.der")),
            )?)],
            PrivateKeyDer::try_from(fs::read(self.root.join(format!("{name}-key.der")))?)?,
        ))
    }
    fn connection(&self, name: Option<&str>) -> Result<StreamOwned<ClientConnection, TcpStream>> {
        let builder = ClientConfig::builder().with_root_certificates(self.roots()?);
        let config = if let Some(name) = name {
            let (cert, key) = self.identity(name)?;
            builder.with_client_auth_cert(cert, key)?
        } else {
            builder.with_no_client_auth()
        };
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(3)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(3)))?;
        Ok(StreamOwned::new(
            ClientConnection::new(Arc::new(config), ServerName::try_from("localhost")?)?,
            tcp,
        ))
    }
    fn call(&self, request: &[u8]) -> Result<Vec<u8>> {
        let mut tls = self.connection(Some("client"))?;
        checked(write_frame(&mut tls, request, MAX))?;
        checked(read_frame(&mut tls, MAX))
    }
}
fn blob(out: &mut Vec<u8>, value: &[u8]) -> Result<()> {
    out.extend(u32::try_from(value.len())?.to_be_bytes());
    out.extend(value);
    Ok(())
}
fn request(
    op: u8,
    binding: [u8; 32],
    reference: &[u8],
    expected: Option<[u8; 32]>,
) -> Result<Vec<u8>> {
    let mut out = b"LXKP".to_vec();
    out.extend(if expected.is_some() { 2_u16 } else { 1_u16 }.to_be_bytes());
    out.push(op);
    blob(&mut out, b"beta-kms")?;
    if op != 0 {
        out.extend(binding);
        out.extend(125_u32.to_be_bytes());
        out.push(1);
        blob(&mut out, reference)?;
    }
    if let Some(expected) = expected {
        out.extend(expected);
    }
    Ok(out)
}
fn facts(bytes: &[u8]) -> Result<([u8; 32], [u8; 32])> {
    assert_eq!(&bytes[..4], b"LXKP");
    assert_eq!(bytes[7], 0);
    assert_eq!(bytes.len(), 109);
    assert_eq!(&bytes[8..12], &32_u32.to_be_bytes());
    Ok((bytes[12..44].try_into()?, bytes[44..76].try_into()?))
}

#[path = "../src/wire.rs"]
mod wire;

fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}

fn call() -> layerx_crypto::evm_transaction::Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec();
    data.extend([3; 32]);
    layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data,
    }
}

fn unsigned(call: &layerx_crypto::evm_transaction::Eip1559Call) -> Result<Vec<u8>> {
    // Strip the three signature fields from a transaction produced by the real encoder.
    let signed = call.sign(&[1; 32])?;
    let (decoded, _) = layerx_crypto::evm_transaction::decode_signed(&signed)?;
    assert_eq!(&decoded, call);
    let prefix = signed[1];
    let start = if prefix <= 0xf7 {
        2
    } else {
        2 + usize::from(prefix - 0xf7)
    };
    let mut end = start;
    for _ in 0..9 {
        let prefix = signed[end];
        end += 1;
        match prefix {
            0..=0x7f | 0xc0 => {}
            0x80..=0xb7 => end += usize::from(prefix - 0x80),
            _ => return Err("unexpected RLP field in test call".into()),
        }
    }
    let payload = &signed[start..end];
    let mut out = vec![2];
    if payload.len() < 56 {
        out.push(0xc0 + u8::try_from(payload.len())?);
    } else {
        out.extend([0xf8, u8::try_from(payload.len())?]);
    }
    out.extend(payload);
    Ok(out)
}

fn signing_request(handle: &[u8], canonical: &[u8], disclosure: &[u8]) -> Result<Vec<u8>> {
    use sha3::{Digest, Keccak256};
    let mut out = request(5, [31; 32], handle, None)?;
    out.extend(Keccak256::digest(canonical));
    blob(&mut out, canonical)?;
    blob(&mut out, disclosure)?;
    Ok(out)
}

fn decode_unsigned_activity(
    canonical: &[u8],
    registry: &layerx_types::payload::ModuleRegistry,
) -> Result<layerx_intents::canonical::Activity> {
    let activity = checked(layerx_intents::canonical::decode_unsigned_activity(
        canonical, registry,
    ))?;
    assert_eq!(
        checked(layerx_intents::canonical::unsigned_activity_bytes(
            &activity
        ))?,
        canonical
    );
    Ok(activity)
}

#[test]
fn lxkp_sign_accepts_an_unsigned_type2_transaction() -> Result<()> {
    let host = Host::new()?;
    let (handle, public) = facts(&host.call(&request(1, [31; 32], &[], None)?)?)?;
    let call = call();
    let canonical = unsigned(&call)?;
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&call, [4; 20]))?;
    let encoded = checked(wire::disclosure(&disclosure))?;
    let sign = signing_request(&handle, &canonical, &encoded)?;
    let response = host.call(&sign)?;
    assert_eq!(&response[..8], b"LXKP\0\x01\x05\0");
    assert_eq!(response.len(), 72);
    checked(
        ring::signature::UnparsedPublicKey::new(&ring::signature::ED25519, public)
            .verify(&call.signing_hash(), &response[8..]),
    )?;

    let mut mismatch = disclosure.clone();
    mismatch.asset[0] ^= 1;
    assert_eq!(
        host.call(&signing_request(
            &handle,
            &canonical,
            &checked(wire::disclosure(&mismatch))?
        )?)?[7],
        1
    );
    mismatch = disclosure.clone();
    mismatch.fee_limit += 1;
    assert_eq!(
        host.call(&signing_request(
            &handle,
            &canonical,
            &checked(wire::disclosure(&mismatch))?
        )?)?[7],
        1
    );
    mismatch = disclosure.clone();
    mismatch.authority[0] ^= 1;
    assert_eq!(
        host.call(&signing_request(
            &handle,
            &canonical,
            &checked(wire::disclosure(&mismatch))?
        )?)?[7],
        1
    );

    let mut changed = call.clone();
    changed.chain_id = 126;
    assert_eq!(
        host.call(&signing_request(&handle, &unsigned(&changed)?, &encoded)?)?[7],
        1
    );
    changed = call.clone();
    changed.to[0] ^= 1;
    assert_eq!(
        host.call(&signing_request(&handle, &unsigned(&changed)?, &encoded)?)?[7],
        1
    );
    changed = call.clone();
    changed.data[0] ^= 1;
    assert_eq!(
        host.call(&signing_request(&handle, &unsigned(&changed)?, &encoded)?)?[7],
        1
    );
    changed = call.clone();
    changed.data[4] ^= 1;
    assert_eq!(
        host.call(&signing_request(&handle, &unsigned(&changed)?, &encoded)?)?[7],
        1
    );

    let mut value = canonical.clone();
    // This fixture's destination is followed by the canonical zero value.
    let destination = value
        .windows(21)
        .position(|item| item[0] == 0x94 && item[1..] == call.to)
        .ok_or("missing destination")?;
    assert_eq!(value[destination + 21], 0x80);
    value[destination + 21] = 1;
    assert_eq!(
        host.call(&signing_request(&handle, &value, &encoded)?)?[7],
        1
    );

    let mut bad_digest = sign.clone();
    let digest_offset = bad_digest.len() - canonical.len() - encoded.len() - 8 - 32;
    bad_digest[digest_offset] ^= 1;
    assert_eq!(host.call(&bad_digest)?[7], 5);
    let mut wrong_network = sign;
    wrong_network[54] ^= 1;
    assert_eq!(host.call(&wrong_network)?[7], 1);
    for invalid in [
        call.sign(&[1; 32])?,
        vec![2, 0xc0],
        {
            let mut trailing = canonical.clone();
            trailing.push(0);
            trailing
        },
        {
            let mut legacy = canonical;
            legacy[0] = 1;
            legacy
        },
    ] {
        assert_eq!(
            host.call(&signing_request(&handle, &invalid, &encoded)?)?[7],
            1
        );
    }
    Ok(())
}

#[test]
fn lxkp_sign_refuses_a_kernel_activity_envelope() -> Result<()> {
    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::{
        ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload,
    };
    let host = Host::new()?;
    let (handle, public) = facts(&host.call(&request(1, [31; 32], &[], None)?)?)?;
    let kind = checked(ActivityType::new(ModuleId::Asset, 4))?;
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Asset,
        &[kind],
    ))?]))?;
    let payment = checked(
        layerx_crypto::payments::Payment::OpenAccount { asset: [3; 32] }
            .encode(b"did:layerx:alice"),
    )?;
    let payload = checked(Payload::new(&registry, kind, &payment))?;
    let mut builder = EnvelopeBuilder::new();
    checked(builder.protocol_version(3))?;
    checked(builder.network_id(125))?;
    checked(builder.activity_type(kind))?;
    checked(builder.actor_did(checked(Did::new(b"did:layerx:alice"))?))?;
    checked(builder.authority(checked(Authority::owner(&public))?))?;
    checked(builder.account_sequence(7))?;
    checked(builder.timestamp_bound(checked(TimestampBound::new(1000, 1010))?))?;
    checked(builder.idempotency_key(IdempotencyKey::new([4; 32])))?;
    checked(builder.fee_limit(Amount::from_u128(1)))?;
    checked(
        builder.payload_hash(checked(layerx_intents::canonical::payload_hash_for(
            &payload,
        ))?),
    )?;
    checked(builder.payload(payload))?;
    let canonical = checked(layerx_intents::canonical::unsigned_envelope_bytes(
        &checked(builder.build())?,
    ))?;
    let activity = decode_unsigned_activity(&canonical, &registry)?;
    assert_eq!(activity.network_id(), 125);
    assert_eq!(activity.activity_type(), kind);
    let disclosure = checked(layerx_crypto::disclosure::bind(&canonical, &registry))?;
    let mut request = signing_request(
        &handle,
        &canonical,
        &checked(wire::disclosure(&disclosure))?,
    )?;
    assert_eq!(host.call(&request)?, b"LXKP\0\x01\x05\x01");
    let offset =
        request.len() - canonical.len() - checked(wire::disclosure(&disclosure))?.len() - 8 - 32;
    request[offset..offset + 32].copy_from_slice(&layerx_human_kms::attestor::preimage(&canonical));
    assert_eq!(host.call(&request)?, b"LXKP\0\x01\x05\x01");
    Ok(())
}
