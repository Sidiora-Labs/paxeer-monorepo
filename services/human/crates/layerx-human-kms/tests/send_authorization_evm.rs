#[path = "support/attestor_cluster.rs"]
mod attestor_cluster;
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use attestor_cluster::{checked, scratch, Cluster, Result};
use layerx_client::lni::framing::{read_frame, write_frame};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde_json::{json, Value};
use sha3::{Digest, Keccak256};
use std::fs;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use zeroize::Zeroizing;

const MAX_FRAME: usize = 2_097_152;
const PROVIDER: &str = "evm-attestor-test";
const BINDING: [u8; 32] = [3; 32];
const ACTION: [u8; 32] = [2; 32];
const EVM_KEY: &str = "evm-owner";
const OWNER: &str = "evm-wallet-owner";

struct ClockAuthority {
    socket: PathBuf,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

impl ClockAuthority {
    fn start() -> Result<Self> {
        let mut server = runtime_clock_server::Server::bind(&std::env::temp_dir())?;
        let socket = server.socket().to_owned();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            socket,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

struct Fixture {
    child: Option<Child>,
    clock: ClockAuthority,
    cluster: Cluster,
    root: PathBuf,
    address: SocketAddr,
    wallet: [u8; 20],
    reference: Vec<u8>,
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.stop();
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Fixture {
    fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let mut cluster = Cluster::start(&[])?;
        let public = cluster.generate_key(
            "ed25519-owner",
            "ed25519",
            OWNER,
            "0x1111111111111111111111111111111111111111",
        )?;
        let evm_public = cluster.generate_key(EVM_KEY, "secp256k1", OWNER, "")?;
        let point =
            k256::ecdsa::VerifyingKey::from_sec1_bytes(&evm_public)?.to_encoded_point(false);
        let wallet = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
        cluster.pki.leaf("node-ca", "kms", true)?;
        cluster.pki.leaf("gateway-ca", "service", false)?;
        for name in ["node-ca", "gateway-ca", "gateway", "kms", "service"] {
            cluster.pki.certificate_der(name)?;
        }
        for name in ["gateway", "kms", "service"] {
            cluster.pki.key_der(name)?;
            fs::set_permissions(
                cluster.pki.path(&format!("{name}.pk8")),
                fs::Permissions::from_mode(0o600),
            )?;
        }
        let root = scratch("lxkp-send-authorization")?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let mut seal = Zeroizing::new([0; 32]);
        getrandom::fill(&mut *seal)?;
        fs::write(root.join("seal"), &*seal)?;
        fs::write(root.join("snapshot.json"), br#"{"chain_id":125}"#)?;
        fs::write(root.join("assertion"), cluster.tokens.mint(OWNER)?)?;
        let nodes: Vec<_> = cluster
            .nodes
            .iter()
            .map(|(id, address)| json!({"id":id,"address":address}))
            .collect();
        fs::write(
            root.join("attestor.json"),
            serde_json::to_vec(&json!({
                "nodes": nodes,
                "trust_anchors": [cluster.pki.path("node-ca.der")],
                "client_certificate_chain": [cluster.pki.path("gateway.der")],
                "client_private_key_pkcs8": cluster.pki.path("gateway.pk8"),
                "key_id": "ed25519-owner", "public_key": public,
                "evm_key_id": EVM_KEY, "evm_address": wallet,
                "signers": attestor_cluster::SIGNERS, "network_id":125, "timeout_seconds":60
            }))?,
        )?;
        for entry in fs::read_dir(&root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        let mut fixture = Self {
            child: None,
            clock: ClockAuthority::start()?,
            cluster,
            root,
            address,
            wallet,
            reference: Vec::new(),
        };
        drop(listener);
        fixture.start()?;
        let description = fixture.call(1, 1, &[], None)?;
        let length = usize::try_from(u32::from_be_bytes(description[..4].try_into()?))?;
        assert_eq!(length, 32);
        fixture.reference = description[4..4 + length].to_vec();
        assert_eq!(&description[4 + length + 32..4 + length + 64], BINDING);
        assert_eq!(
            fixture.call(3, 6, &fixture.reference, None)?,
            fixture.wallet
        );
        Ok(fixture)
    }

    fn start(&mut self) -> Result<()> {
        let log = fs::File::create(self.root.join("kms.log"))?;
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "60");
        for (suffix, path) in [
            ("REGISTRY_FILE", self.root.join("snapshot.json")),
            ("SEAL_SECRET_FILE", self.root.join("seal")),
            ("ATTESTOR_CONFIG_FILE", self.root.join("attestor.json")),
            ("EVM_ASSERTION_FILE", self.root.join("assertion")),
            ("CLIENT_CA_DER", self.cluster.pki.path("gateway-ca.der")),
            ("TLS_CERT_DER", self.cluster.pki.path("kms.der")),
            ("TLS_KEY_DER", self.cluster.pki.path("kms.pk8")),
            ("CLIENT_CERT_DER", self.cluster.pki.path("service.der")),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), path);
        }
        command.env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER");
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.clock.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string(),
            );
        self.child = Some(
            command
                .stdin(Stdio::null())
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?,
        );
        let started = Instant::now();
        loop {
            if self.call(1, 0, &[], None).is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing KMS process")?
                .try_wait()?
                .is_some()
                || started.elapsed() > Duration::from_secs(10)
            {
                return Err(format!(
                    "KMS startup failed: {}",
                    fs::read_to_string(self.root.join("kms.log"))?
                )
                .into());
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

    fn exchange(
        &self,
        version: u16,
        operation: u8,
        reference: &[u8],
        payload: Option<&[u8]>,
    ) -> Result<Vec<u8>> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(
            self.cluster.pki.path("node-ca.der"),
        )?))?;
        let config = ClientConfig::builder()
            .with_root_certificates(roots)
            .with_client_auth_cert(
                vec![CertificateDer::from(fs::read(
                    self.cluster.pki.path("service.der"),
                )?)],
                PrivateKeyDer::try_from(fs::read(self.cluster.pki.path("service.pk8"))?)?,
            )?;
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(60)))?;
        let mut tls = StreamOwned::new(
            ClientConnection::new(
                Arc::new(config),
                ServerName::IpAddress(self.address.ip().into()),
            )?,
            tcp,
        );
        let mut request = b"LXKP".to_vec();
        request.extend(version.to_be_bytes());
        request.push(operation);
        blob(&mut request, PROVIDER.as_bytes())?;
        if operation != 0 {
            request.extend(BINDING);
            request.extend(125_u32.to_be_bytes());
            request.push(1);
            blob(&mut request, reference)?;
        }
        if let Some(payload) = payload {
            blob(&mut request, payload)?;
        }
        checked(write_frame(&mut tls, &request, MAX_FRAME))?;
        let response = checked(read_frame(&mut tls, MAX_FRAME))?;
        assert_eq!(&response[..7], &request[..7]);
        Ok(response)
    }

    fn call(
        &self,
        version: u16,
        operation: u8,
        reference: &[u8],
        payload: Option<&[u8]>,
    ) -> Result<Vec<u8>> {
        let response = self.exchange(version, operation, reference, payload)?;
        if response[7] != 0 {
            return Err(format!(
                "LXKP operation {operation} refused with status {}",
                response[7]
            )
            .into());
        }
        Ok(response[8..].to_vec())
    }
}

fn blob(out: &mut Vec<u8>, bytes: &[u8]) -> Result<()> {
    out.extend(u32::try_from(bytes.len())?.to_be_bytes());
    out.extend(bytes);
    Ok(())
}

fn authorization(fixture: &Fixture) -> Result<Value> {
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    let recipient = [0x44; 20];
    let mut amount = [0; 32];
    amount[31] = 42;
    let denom = "apax";
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
            .to_vec();
    for address in [fixture.wallet, recipient] {
        data.extend([0; 12]);
        data.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    data.extend(offset);
    data.extend(amount);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    data.extend(denom.as_bytes());
    data.extend([0; 28]);
    Ok(json!({
        "authorization": {
            "plan_id": ([1; 32]), "action_key": ACTION,
            "tenant": "tenant", "principal": OWNER,
            "binding_digest": BINDING, "wallet": fixture.wallet,
            "not_before": now - 5, "not_after": now + 300,
            "transaction": {
                "chain_id": 125, "nonce": 0,
                "max_priority_fee_per_gas": 1, "max_fee_per_gas": 2,
                "gas_limit": 100_000, "to": bank, "value": ([0; 32]),
                "calldata": data,
            }
        },
        "disclosure": { "recipient": recipient, "denom": denom, "amount": amount }
    }))
}

#[test]
fn authorize_send_returns_a_signed_bank_transfer() -> Result<()> {
    let mut fixture = Fixture::new()?;
    let request = authorization(&fixture)?;
    let payload = serde_json::to_vec(&request)?;
    let client = fixture.cluster.client()?;
    let before = fixture.cluster.audit(&client)?;
    let raw = fixture.call(3, 11, &fixture.reference, Some(&payload))?;
    assert_eq!(raw.first(), Some(&2));
    let (call, signature) = decode_signed(&raw)?;
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 0);
    assert_eq!(call.max_priority_fee_per_gas, 1);
    assert_eq!(call.max_fee_per_gas, 2);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(
        json!(call.to),
        request["authorization"]["transaction"]["to"]
    );
    assert_eq!(
        json!(call.data),
        request["authorization"]["transaction"]["calldata"]
    );
    assert_eq!(recover_sender(&call, &signature)?, fixture.wallet);
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(
        &call,
        fixture.wallet,
    ))?;
    let method = checked(disclosure.construct_method())?;
    let Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
        from,
        to,
        denom,
        amount,
    }) = method
    else {
        return Err("signed transaction is not a bank transfer".into());
    };
    assert_eq!(from, fixture.wallet);
    assert_eq!(json!(to), request["disclosure"]["recipient"]);
    assert_eq!(json!(denom), request["disclosure"]["denom"]);
    assert_eq!(json!(amount), request["disclosure"]["amount"]);
    let after = fixture.cluster.audit(&client)?;
    assert_eq!(before.len(), attestor_cluster::SIGNERS.len());
    assert_eq!(after.len(), before.len());
    for (previous, current) in before.iter().zip(&after) {
        assert!(
            current > previous,
            "every attestor must record the EVM signing"
        );
    }
    assert_eq!(
        fixture.call(3, 11, &fixture.reference, Some(&payload))?,
        raw
    );
    assert_eq!(fixture.cluster.audit(&client)?, after);
    let persisted: Value = serde_json::from_slice(&fixture.call(
        3,
        10,
        &fixture.reference,
        Some(&serde_json::to_vec(&ACTION)?),
    )?)?;
    assert_eq!(persisted["raw_transaction"], json!(raw));
    assert_eq!(persisted["transaction_hash"], json!(transaction_hash(&raw)));
    fixture.stop();
    fixture.start()?;
    assert_eq!(
        fixture.call(3, 11, &fixture.reference, Some(&payload))?,
        raw
    );
    assert_eq!(fixture.cluster.audit(&client)?, after);
    let mut conflict = request.clone();
    conflict["authorization"]["transaction"]["nonce"] = json!(1);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&conflict)?)
        )?,
        b"LXKP\0\x03\x0b\x03"
    );
    let config = serde_json::from_slice(&fs::read(fixture.root.join("attestor.json"))?)?;
    let mut signer = checked(layerx_human_kms::attestor::AttestorSigner::from_config(
        &config,
    ))?;
    checked(signer.set_evm_assertion(&fs::read_to_string(fixture.root.join("assertion"))?))?;
    let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
    let mut signature = checked(signer.sign_evm_transaction(&unsigned, call.chain_id))?;
    signature[64] += 27;
    let signature = checked(layerx_crypto::signer::AgentSignature::new(signature))?;
    assert_eq!(recover_sender(&call, &signature)?, fixture.wallet);
    let assembled = layerx_crypto::evm_transaction::assemble(&call, &signature);
    assert_eq!(decode_signed(&assembled)?.0, call);
    Ok(())
}

#[test]
fn authorize_send_with_mismatched_disclosure_is_refused() -> Result<()> {
    let fixture = Fixture::new()?;
    let request = authorization(&fixture)?;
    let client = fixture.cluster.client()?;
    let before = fixture.cluster.audit(&client)?;
    let mut mismatch = request.clone();
    mismatch["disclosure"]["recipient"] = json!(([0x55; 20]));
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["disclosure"]["amount"][31] = json!(43);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["disclosure"]["denom"] = json!("other");
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    assert_eq!(fixture.cluster.audit(&client)?, before);
    assert_eq!(
        fixture.exchange(
            3,
            10,
            &fixture.reference,
            Some(&serde_json::to_vec(&ACTION)?)
        )?,
        b"LXKP\0\x03\x0a\x02"
    );
    Ok(())
}
