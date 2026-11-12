use std::env;
use std::net::TcpListener;
use std::process::ExitCode;
use std::sync::Arc;

#[path = "kms_gateway/startup.rs"]
mod startup;

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("layerx-human-kms-gateway refused startup: {message}");
            ExitCode::FAILURE
        }
    }
}

fn run() -> Result<(), String> {
    let binding = layerx_client::runtime_clock::ClockBinding::from_environment()
        .map_err(|error| format!("runtime clock unavailable: {error}"))?;
    let clock = startup::runtime_clock(binding)?;
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "the TLS cryptographic provider is already configured".to_owned())?;
    let (listen, gateway) = startup::prepare(clock, &|name| env::var(name).ok())?;
    let listener =
        TcpListener::bind(listen).map_err(|_| "the gateway listener cannot bind".to_owned())?;
    Arc::new(gateway)
        .serve(&listener)
        .map_err(|error| format!("the gateway stopped: {}", error.refusal_code()))
}

#[cfg(test)]
mod tests {
    use layerx_types::clock::Deadline;
    use std::fs;
    use std::net::TcpStream;
    use std::os::unix::fs::PermissionsExt;
    use std::process::{Child, Command, Output, Stdio};

    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, Disclosure};
    use layerx_crypto::ed25519::verify_digest;
    use layerx_human_service::custody::{
        CustodyError, EnvelopeKms, KeyClass, KmsError, KmsProvider, PrincipalKeyBinding,
        GATEWAY_FRAME_LIMIT,
    };
    use layerx_human_service::frame::{read_frame, write_frame};
    use layerx_intents::NativeOwnerBootstrap;
    use layerx_types::ids::Did;
    use layerx_types::intent::PublicKey;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration};
    use rustls::pki_types::ServerName;
    use rustls::{ClientConfig, ClientConnection, StreamOwned};

    use super::startup::protected;
    use super::*;
    use layerx_client::runtime_clock::ClockBinding;
    use layerx_human_service::custody::{
        GatewayLimits, GatewayPolicy, GatewayTls, LxkpGateway, RemoteKmsProvider,
    };
    use layerx_human_service::frame::{Limits, MutualTlsConfig};
    use layerx_types::clock::Clock;
    use layerx_types::payload::ModuleRegistry;
    use rustls::pki_types::{CertificateDer, PrivateKeyDer};
    use rustls::RootCertStore;
    use std::net::SocketAddr;
    use std::path::PathBuf;
    use std::time::Duration;
    use zeroize::Zeroizing;

    const OK: u8 = 0;

    type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

    const PROVIDER: &str = "beta-kms";
    const NETWORK: u32 = 77;
    const BACKEND_BINARY: &str = "LAYERX_HUMAN_KMS_GATEWAY_TEST_BACKEND";
    const GATEWAY_BINARY: &str = "LAYERX_HUMAN_KMS_GATEWAY_TEST_BINARY";
    const IDENTITIES: [(&str, bool); 5] = [
        ("backend", true),
        ("gateway", true),
        ("relay", false),
        ("service", false),
        ("foreign", false),
    ];

    fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> TestResult<T> {
        value.map_err(|error| format!("{error:?}").into())
    }

    struct OwnerEnvelopeContext {
        owner_public_key: [u8; 32],
        network_id: u32,
        account_sequence: u64,
        action_key: [u8; 32],
        fee_limit: u128,
    }

    fn unsigned_native(
        compiled: &layerx_intents::CompiledIntent,
        context: &OwnerEnvelopeContext,
    ) -> TestResult<(Vec<u8>, Disclosure)> {
        let payload = compiled.payload().as_bytes();
        let mut data =
            layerx_crypto::disclosure::call::ledger_selector("recordLifecycle(bytes,bytes32)")
                .to_vec();
        let mut offset = [0; 32];
        offset[31] = 64;
        data.extend_from_slice(&offset);
        data.extend_from_slice(&context.action_key);
        let mut length = [0; 32];
        length[24..].copy_from_slice(&u64::try_from(payload.len())?.to_be_bytes());
        data.extend_from_slice(&length);
        data.extend_from_slice(payload);
        data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
        let mut to = [0; 20];
        to[18..].copy_from_slice(&0x101c_u16.to_be_bytes());
        let call = layerx_crypto::evm_transaction::Eip1559Call {
            chain_id: u64::from(context.network_id),
            nonce: context.account_sequence,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: context.fee_limit,
            gas_limit: 1,
            to,
            data,
        };
        let sender = context.owner_public_key[12..].try_into()?;
        let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&call, sender))?;
        let bytes = call.encode_unsigned();
        Ok((bytes, disclosure))
    }

    fn modules() -> TestResult<ModuleRegistry> {
        checked(ModuleRegistry::new(&[
            checked(ModuleRegistration::new(
                ModuleId::Asset,
                &[
                    checked(ActivityType::new(ModuleId::Asset, 1))?,
                    checked(ActivityType::new(ModuleId::Asset, 4))?,
                    checked(ActivityType::new(ModuleId::Asset, 5))?,
                ],
            ))?,
            checked(ModuleRegistration::new(
                ModuleId::Budget,
                &[checked(ActivityType::new(ModuleId::Budget, 1))?],
            ))?,
            checked(ModuleRegistration::new(
                ModuleId::Governance,
                &[
                    checked(ActivityType::new(ModuleId::Governance, 1))?,
                    checked(ActivityType::new(ModuleId::Governance, 2))?,
                    checked(ActivityType::new(ModuleId::Governance, 3))?,
                    checked(ActivityType::new(ModuleId::Governance, 5))?,
                    checked(ActivityType::new(ModuleId::Governance, 8))?,
                ],
            ))?,
        ]))
    }

    fn limits() -> GatewayLimits {
        GatewayLimits {
            maximum_connections: 8,
            maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
            deadline: Duration::from_secs(5),
            maximum_operations: 1000,
            operation_window: Duration::from_secs(60),
        }
    }

    fn policy(limits: GatewayLimits) -> TestResult<GatewayPolicy> {
        Ok(GatewayPolicy {
            network_id: NETWORK,
            protocol_version: 3,
            registry: modules()?,
            limits,
        })
    }

    fn free_address() -> TestResult<SocketAddr> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        Ok(listener.local_addr()?)
    }

    fn blob(out: &mut Vec<u8>, value: &[u8]) -> TestResult {
        out.extend(u32::try_from(value.len())?.to_be_bytes());
        out.extend(value);
        Ok(())
    }

    fn header(version: u16, operation: u8, provider: &str) -> TestResult<Vec<u8>> {
        let mut out = b"LXKP".to_vec();
        out.extend(version.to_be_bytes());
        out.push(operation);
        blob(&mut out, provider.as_bytes())?;
        Ok(out)
    }

    fn keyed(
        version: u16,
        operation: u8,
        binding: [u8; 32],
        reference: &[u8],
    ) -> TestResult<Vec<u8>> {
        let mut out = header(version, operation, PROVIDER)?;
        out.extend(binding);
        out.extend(NETWORK.to_be_bytes());
        out.push(1);
        blob(&mut out, reference)?;
        Ok(out)
    }

    fn signing(
        binding: [u8; 32],
        reference: &[u8],
        digest: [u8; 32],
        canonical: &[u8],
        disclosure: &[u8],
    ) -> TestResult<Vec<u8>> {
        let mut out = keyed(1, 5, binding, reference)?;
        out.extend(digest);
        blob(&mut out, canonical)?;
        blob(&mut out, disclosure)?;
        Ok(out)
    }

    /// Owner-bootstrap identity activity for one public key with its signing
    /// preimage digest and LXKP wire disclosure.
    struct Activity {
        unsigned: Vec<u8>,
        digest: [u8; 32],
        encoded: Vec<u8>,
    }

    fn owner_activity(public: [u8; 32]) -> TestResult<Activity> {
        let registry = modules()?;
        let did = checked(Did::new(b"did:layerx:gateway-owner"))?;
        let compiled = checked(
            NativeOwnerBootstrap::Identity {
                did: did.clone(),
                primary_key: PublicKey::new(public),
            }
            .compile(&registry),
        )?;
        let context = OwnerEnvelopeContext {
            owner_public_key: public,
            network_id: NETWORK,
            account_sequence: 10,
            action_key: [u8::try_from(compiled.activity_type().ordinal())?; 32],
            fee_limit: 100,
        };
        let (unsigned, disclosure) = checked(unsigned_native(&compiled, &context))?;
        let call = checked(layerx_crypto::evm_transaction::decode_unsigned(&unsigned))?;
        Ok(Activity {
            digest: call.signing_hash(),
            encoded: encoded_disclosure(&disclosure)?,
            unsigned,
        })
    }

    /// LXKP wire disclosure of a native owner operation. Owner bootstrap never
    /// discloses a grant, session, onboarding or payout binding.
    fn encoded_disclosure(disclosure: &Disclosure) -> TestResult<Vec<u8>> {
        if disclosure.authority_grant.is_some()
            || disclosure.session_grant.is_some()
            || disclosure.onboarding.is_some()
            || disclosure.evm_payout_binding.is_some()
        {
            return Err("owner bootstrap disclosed more than a native operation".into());
        }
        let operation = disclosure
            .native_operation
            .as_ref()
            .ok_or("owner bootstrap disclosed no native operation")?;
        let mut out = vec![4];
        out.extend(disclosure.activity_type.value().to_be_bytes());
        blob(&mut out, &disclosure.actor)?;
        blob(&mut out, &disclosure.authority)?;
        out.extend(u32::try_from(disclosure.counterparties.len())?.to_be_bytes());
        for party in &disclosure.counterparties {
            out.push(match party.role {
                CounterpartyRole::Payer => 1,
                CounterpartyRole::Recipient => 2,
            });
            out.extend(party.account);
        }
        out.extend(u32::try_from(disclosure.amounts.len())?.to_be_bytes());
        for amount in &disclosure.amounts {
            out.push(match amount.role {
                AmountRole::Transfer => 1,
                AmountRole::SpendingLimit => 2,
                AmountRole::SupplyCap => 3,
                AmountRole::PerDrawMaximum => 4,
                AmountRole::GrantAllowance => 5,
            });
            out.extend(amount.value.to_be_bytes());
        }
        out.extend(disclosure.asset);
        out.extend(disclosure.fee_limit.to_be_bytes());
        out.extend(disclosure.expiry.not_before.to_be_bytes());
        out.extend(disclosure.expiry.not_after.to_be_bytes());
        out.extend(disclosure.expiry.payload_expires_at.to_be_bytes());
        out.extend(disclosure.idempotency_key);
        out.push(0);
        blob(&mut out, &checked(operation.encode())?)?;
        Ok(out)
    }

    struct Fixture {
        clock: Arc<dyn Clock>,
        root: PathBuf,
        backend: SocketAddr,
        child: Option<Child>,
    }

    impl Drop for Fixture {
        fn drop(&mut self) {
            if let Some(mut child) = self.child.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
            let _ = fs::remove_dir_all(&self.root);
        }
    }

    impl Fixture {
        fn new() -> TestResult<Self> {
            let _ = rustls::crypto::ring::default_provider().install_default();
            let mut unique = [0_u8; 8];
            getrandom::fill(&mut unique).map_err(|error| error.to_string())?;
            let root = std::env::temp_dir().join(format!(
                "lxkp-gateway-{}-{}",
                std::process::id(),
                u64::from_be_bytes(unique)
            ));
            fs::create_dir(&root)?;
            fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
            let fixture = Self {
                clock: layerx_client::runtime_clock::RuntimeClock::from_environment()?,
                root,
                backend: free_address()?,
                child: None,
            };
            fixture.provision()?;
            Ok(fixture)
        }

        fn provision(&self) -> TestResult {
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
                "/CN=LXKP gateway test CA",
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ])?;
            self.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"])?;
            for (name, server) in IDENTITIES {
                let key = format!("{name}.key");
                let request = format!("{name}.csr");
                let certificate = format!("{name}.pem");
                let subject = format!("/CN={name}");
                self.openssl(&[
                    "req", "-newkey", "rsa:2048", "-nodes", "-keyout", &key, "-out", &request,
                    "-subj", &subject,
                ])?;
                fs::write(
                    self.root.join("extensions"),
                    if server {
                        "subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n"
                    } else {
                        "extendedKeyUsage=clientAuth\n"
                    },
                )?;
                self.openssl(&[
                    "x509",
                    "-req",
                    "-in",
                    &request,
                    "-CA",
                    "ca.pem",
                    "-CAkey",
                    "ca.key",
                    "-CAcreateserial",
                    "-out",
                    &certificate,
                    "-days",
                    "1",
                    "-extfile",
                    "extensions",
                ])?;
                self.openssl(&[
                    "x509",
                    "-in",
                    &certificate,
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
                    &key,
                    "-outform",
                    "DER",
                    "-out",
                    &format!("{name}-key.der"),
                ])?;
            }
            let mut seal = [0_u8; 32];
            getrandom::fill(&mut seal).map_err(|error| error.to_string())?;
            fs::write(self.root.join("seal"), seal)?;
            let modules: Vec<_> = self::modules()?
            .registrations()
            .iter()
            .map(|module| {
                let kinds: Vec<_> = module
                    .activity_types()
                    .iter()
                    .map(|kind| kind.value())
                    .collect();
                serde_json::json!({"module_id": module.module() as u16, "activity_types": kinds})
            })
            .collect();
            fs::write(
                self.root.join("registry.json"),
                serde_json::to_vec(
                    &serde_json::json!({"network_id": NETWORK, "protocol_version": 3, "modules": modules}),
                )?,
            )?;
            fs::write(self.root.join("chain.json"), br#"{"chain_id":125}"#)?;
            for entry in fs::read_dir(&self.root)? {
                fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
            }
            Ok(())
        }

        fn openssl(&self, arguments: &[&str]) -> TestResult {
            let output = Command::new("openssl")
                .args(arguments)
                .current_dir(&self.root)
                .output()?;
            if !output.status.success() {
                return Err(format!(
                    "openssl failed: {}",
                    String::from_utf8_lossy(&output.stderr)
                )
                .into());
            }
            Ok(())
        }

        fn path(&self, file: &str) -> PathBuf {
            self.root.join(file)
        }

        fn read(&self, file: &str) -> TestResult<Vec<u8>> {
            Ok(fs::read(self.path(file))?)
        }

        /// Starts the real LXKP KMS backend, which pins the gateway relay identity.
        fn start_backend(&mut self) -> TestResult {
            let binary = std::env::var(BACKEND_BINARY)
                .map_err(|_| format!("{BACKEND_BINARY} must name the layerx-human-kms binary"))?;
            let mut command = Command::new(binary);
            let clock_binding = ClockBinding::from_environment()?;
            startup::forward_clock_binding(&mut command, &clock_binding);
            command
                .env("LAYERX_HUMAN_KMS_LISTEN", self.backend.to_string())
                .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
                .env("LAYERX_HUMAN_KMS_STATE_DIR", self.path("state"))
                .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "5");
            for (suffix, file) in [
                ("REGISTRY_FILE", "chain.json"),
                ("CLIENT_CA_DER", "ca.der"),
                ("TLS_CERT_DER", "backend.der"),
                ("TLS_KEY_DER", "backend-key.der"),
                ("CLIENT_CERT_DER", "relay.der"),
                ("SEAL_SECRET_FILE", "seal"),
            ] {
                command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), self.path(file));
            }
            self.child = Some(
                command
                    .stdout(Stdio::null())
                    .stderr(Stdio::null())
                    .spawn()?,
            );
            let mut deadline = Deadline::start(self.clock.as_ref(), Duration::from_secs(15))?;
            loop {
                if self.provider(self.backend, "relay")?.probe().is_ok() {
                    return Ok(());
                }
                if self
                    .child
                    .as_mut()
                    .ok_or("backend missing")?
                    .try_wait()?
                    .is_some()
                {
                    return Err("the KMS backend exited at startup".into());
                }
                if deadline.remaining(self.clock.as_ref())?.is_zero() {
                    return Err("the KMS backend startup deadline passed".into());
                }
                std::thread::sleep(Duration::from_millis(20));
            }
        }

        fn roots(&self) -> TestResult<RootCertStore> {
            let mut roots = RootCertStore::empty();
            roots.add(CertificateDer::from(self.read("ca.der")?))?;
            Ok(roots)
        }

        fn mutual_tls(&self, identity: &str) -> TestResult<MutualTlsConfig> {
            checked(MutualTlsConfig::new(
                self.roots()?,
                vec![CertificateDer::from(self.read(&format!("{identity}.der"))?)],
                PrivateKeyDer::try_from(self.read(&format!("{identity}-key.der"))?)?,
            ))
        }

        /// The production LXKP client, presenting `identity`, aimed at `endpoint`.
        fn provider(&self, endpoint: SocketAddr, identity: &str) -> TestResult<RemoteKmsProvider> {
            let deadline_tls = ClientConfig::builder()
                .with_root_certificates(self.roots()?)
                .with_client_auth_cert(
                    vec![CertificateDer::from(self.read(&format!("{identity}.der"))?)],
                    PrivateKeyDer::try_from(self.read(&format!("{identity}-key.der"))?)?,
                )?;
            Ok(RemoteKmsProvider::new(
                PROVIDER,
                endpoint,
                "localhost",
                self.mutual_tls(identity)?,
                Limits {
                    maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                    maximum_connections: 4,
                    maximum_streams: 1,
                    maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
                    deadline: Duration::from_secs(5),
                },
                Arc::clone(&self.clock),
            )?
            .with_deadline_transport(Arc::new(deadline_tls)))
        }

        fn tls(&self, pinned: &str) -> TestResult<GatewayTls> {
            Ok(GatewayTls::new(
                &self.read("ca.der")?,
                &self.read("gateway.der")?,
                &Zeroizing::new(self.read("gateway-key.der")?),
                &self.read(&format!("{pinned}.der"))?,
            )?)
        }

        /// Runs an in-process gateway relaying to the backend and pinning the
        /// service identity.
        fn gateway(&self, limits: GatewayLimits) -> TestResult<SocketAddr> {
            let gateway = Arc::new(LxkpGateway::new(
                Arc::new(self.provider(self.backend, "relay")?),
                self.tls("service")?,
                policy(limits)?,
                Arc::clone(&self.clock),
            )?);
            let listener = TcpListener::bind("127.0.0.1:0")?;
            let address = listener.local_addr()?;
            std::thread::spawn(move || gateway.serve(&listener));
            Ok(address)
        }

        fn connection(
            &self,
            address: SocketAddr,
            identity: Option<&str>,
        ) -> TestResult<StreamOwned<ClientConnection, TcpStream>> {
            let builder = ClientConfig::builder().with_root_certificates(self.roots()?);
            let config = match identity {
                Some(identity) => builder.with_client_auth_cert(
                    vec![CertificateDer::from(self.read(&format!("{identity}.der"))?)],
                    PrivateKeyDer::try_from(self.read(&format!("{identity}-key.der"))?)?,
                )?,
                None => builder.with_no_client_auth(),
            };
            let tcp = TcpStream::connect(address)?;
            tcp.set_read_timeout(Some(Duration::from_secs(10)))?;
            tcp.set_write_timeout(Some(Duration::from_secs(10)))?;
            Ok(StreamOwned::new(
                ClientConnection::new(Arc::new(config), ServerName::try_from("localhost")?)?,
                tcp,
            ))
        }

        fn call(
            &self,
            address: SocketAddr,
            identity: Option<&str>,
            frame: &[u8],
        ) -> TestResult<Vec<u8>> {
            let mut stream = self.connection(address, identity)?;
            checked(write_frame(&mut stream, frame, GATEWAY_FRAME_LIMIT))?;
            checked(read_frame(&mut stream, GATEWAY_FRAME_LIMIT))
        }

        fn status(&self, address: SocketAddr, frame: &[u8]) -> TestResult<u8> {
            let response = self.call(address, Some("service"), frame)?;
            if response.len() < 8 || response[..7] != frame[..7] {
                return Err("the gateway answered a different contract".into());
            }
            Ok(response[7])
        }

        fn signature(&self, address: SocketAddr, frame: &[u8]) -> TestResult<[u8; 64]> {
            let response = self.call(address, Some("service"), frame)?;
            if response.len() != 72 || response[..7] != frame[..7] || response[7] != OK {
                return Err("the gateway did not return one signature".into());
            }
            Ok(response[8..].try_into()?)
        }

        fn binary(
            &self,
            listen: SocketAddr,
            kind: &str,
            endpoint: SocketAddr,
        ) -> TestResult<Command> {
            let binary = std::env::var(GATEWAY_BINARY).map_err(|_| {
                format!("{GATEWAY_BINARY} must name the layerx-human-kms-gateway binary")
            })?;
            let mut command = Command::new(binary);
            command
                .env_clear()
                .env("LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND", kind);
            startup::forward_clock_binding(&mut command, &ClockBinding::from_environment()?);
            for (suffix, value) in [
                ("LISTEN", listen.to_string()),
                ("PROVIDER_REFERENCE", PROVIDER.to_owned()),
                ("PROVIDER_ENDPOINT", endpoint.to_string()),
                ("PROVIDER_SERVER_NAME", "localhost".to_owned()),
                ("PROVIDER_MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
                ("PROVIDER_MAX_CONNECTIONS", "4".to_owned()),
                ("PROVIDER_MAX_STREAMS", "1".to_owned()),
                ("PROVIDER_MAX_QUEUED_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
                ("PROVIDER_DEADLINE_SECONDS", "5".to_owned()),
                ("MAX_CONNECTIONS", "8".to_owned()),
                ("MAX_FRAME_BYTES", GATEWAY_FRAME_LIMIT.to_string()),
                ("DEADLINE_SECONDS", "5".to_owned()),
                ("RATE_MAXIMUM", "1000".to_owned()),
                ("RATE_WINDOW_SECONDS", "60".to_owned()),
            ] {
                command.env(format!("LAYERX_HUMAN_KMS_GATEWAY_{suffix}"), value);
            }
            for (suffix, file) in [
                ("REGISTRY_FILE", "chain.json"),
                ("CLIENT_CA_DER", "ca.der"),
                ("TLS_CERT_DER", "gateway.der"),
                ("TLS_KEY_DER", "gateway-key.der"),
                ("CLIENT_CERT_DER", "service.der"),
                ("PROVIDER_ROOT_CERTIFICATE_DER", "ca.der"),
                ("PROVIDER_CLIENT_CERTIFICATE_DER", "relay.der"),
                ("PROVIDER_CLIENT_PRIVATE_KEY_DER", "relay-key.der"),
            ] {
                command.env(
                    format!("LAYERX_HUMAN_KMS_GATEWAY_{suffix}"),
                    self.path(file),
                );
            }
            Ok(command)
        }
    }

    fn refused_startup(output: &Output, reason: &str) -> bool {
        !output.status.success()
            && String::from_utf8_lossy(&output.stderr).contains(&format!(
                "layerx-human-kms-gateway refused startup: {reason}"
            ))
    }

    #[test]
    fn development_envelope_provider_never_backs_the_gateway() -> TestResult {
        let fixture = Fixture::new()?;
        let development = EnvelopeKms::new(PROVIDER, fixture.path("seal"))?;
        let outcome = LxkpGateway::new(
            Arc::new(development),
            fixture.tls("service")?,
            policy(limits())?,
            Arc::clone(&fixture.clock),
        );
        assert!(matches!(
            outcome,
            Err(CustodyError::DevelopmentProviderInProduction)
        ));
        Ok(())
    }

    #[test]
    fn absent_provider_and_invalid_bounds_fail_closed() -> TestResult {
        let fixture = Fixture::new()?;
        let absent = LxkpGateway::new(
            Arc::new(fixture.provider(free_address()?, "relay")?),
            fixture.tls("service")?,
            policy(limits())?,
            Arc::clone(&fixture.clock),
        );
        assert!(matches!(absent, Err(CustodyError::Kms(_))));
        for bounds in [
            GatewayLimits {
                maximum_connections: 0,
                ..limits()
            },
            GatewayLimits {
                maximum_frame_bytes: GATEWAY_FRAME_LIMIT + 1,
                ..limits()
            },
            GatewayLimits {
                deadline: Duration::ZERO,
                ..limits()
            },
            GatewayLimits {
                maximum_operations: 0,
                ..limits()
            },
            GatewayLimits {
                operation_window: Duration::from_secs(3601),
                ..limits()
            },
        ] {
            let outcome = LxkpGateway::new(
                Arc::new(fixture.provider(free_address()?, "relay")?),
                fixture.tls("service")?,
                policy(bounds)?,
                Arc::clone(&fixture.clock),
            );
            assert!(matches!(outcome, Err(CustodyError::InvalidLimits)));
        }
        let wrong_network = LxkpGateway::new(
            Arc::new(fixture.provider(free_address()?, "relay")?),
            fixture.tls("service")?,
            GatewayPolicy {
                network_id: 0,
                ..policy(limits())?
            },
            Arc::clone(&fixture.clock),
        );
        assert!(matches!(wrong_network, Err(CustodyError::InvalidNetwork)));
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn relays_lifecycle_and_verified_signatures_to_the_real_backend() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        let address = fixture.gateway(limits())?;
        let client = fixture.provider(address, "service")?;
        client.probe()?;
        let binding =
            PrincipalKeyBinding::new(b"alice".to_vec(), NETWORK, KeyClass::HumanPrimary, PROVIDER)?;
        let created = client.create_key(&binding)?;
        assert_eq!(created.binding_digest(), binding.digest());
        assert_eq!(client.describe_key(&binding, created.reference())?, created);
        let handle = created.reference().as_bytes();
        let activity = owner_activity(created.public_key())?;
        let frame = signing(
            binding.digest(),
            handle,
            activity.digest,
            &activity.unsigned,
            &activity.encoded,
        )?;
        let signature = fixture.signature(address, &frame)?;
        checked(verify_digest(
            &created.public_key(),
            &signature,
            &activity.digest,
        ))?;
        let rotated =
            client.rotate_key_if_current(&binding, created.reference(), created.public_key())?;
        assert_eq!(rotated.reference(), created.reference());
        assert_ne!(rotated.public_key(), created.public_key());
        assert_eq!(client.describe_key(&binding, created.reference())?, rotated);
        let resigned = fixture.signature(address, &frame)?;
        checked(verify_digest(
            &rotated.public_key(),
            &resigned,
            &activity.digest,
        ))?;
        assert!(verify_digest(&created.public_key(), &resigned, &activity.digest).is_err());
        let other =
            PrincipalKeyBinding::new(b"bob".to_vec(), NETWORK, KeyClass::HumanPrimary, PROVIDER)?;
        assert!(client.describe_key(&other, created.reference()).is_err());
        client.destroy_key(&binding, created.reference())?;
        assert_eq!(
            client.describe_key(&binding, created.reference()),
            Err(KmsError::KeyNotFound)
        );
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn refuses_every_frame_outside_the_served_contracts() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        let address = fixture.gateway(limits())?;
        let binding = [7; 32];
        for (version, operation) in [
            (5, 14),
            (3, 6),
            (3, 7),
            (3, 8),
            (3, 11),
            (3, 12),
            (4, 13),
            (9, 0),
            (1, 6),
        ] {
            assert_eq!(
                fixture.status(address, &keyed(version, operation, binding, b"handle")?)?,
                1
            );
        }
        assert_eq!(fixture.status(address, &header(1, 0, "other-kms")?)?, 1);
        assert_eq!(
            fixture.status(address, &keyed(1, 2, [0; 32], b"handle")?)?,
            1
        );
        let mut network = keyed(1, 2, binding, b"handle")?;
        network[54] ^= 1;
        assert_eq!(fixture.status(address, &network)?, 1);
        let mut class = keyed(1, 2, binding, b"handle")?;
        class[55] = 3;
        assert_eq!(fixture.status(address, &class)?, 1);
        assert_eq!(
            fixture.status(address, &keyed(1, 1, binding, b"handle")?)?,
            1
        );
        assert_eq!(fixture.status(address, &keyed(1, 2, binding, &[])?)?, 1);
        let mut trailing = keyed(1, 2, binding, b"handle")?;
        trailing.push(0);
        assert_eq!(fixture.status(address, &trailing)?, 1);
        let truncated = keyed(1, 2, binding, b"handle")?;
        assert_eq!(
            fixture.status(address, &truncated[..truncated.len() - 1])?,
            1
        );
        assert_eq!(
            fixture.status(address, &keyed(1, 2, binding, &[1; 4097])?)?,
            1
        );
        let mut zero_expected = keyed(2, 3, binding, b"handle")?;
        zero_expected.extend([0; 32]);
        assert_eq!(fixture.status(address, &zero_expected)?, 1);
        assert_eq!(
            fixture.status(
                address,
                &signing(binding, b"handle", [1; 32], &[], b"disclosure")?
            )?,
            1
        );
        assert_eq!(
            fixture.status(
                address,
                &signing(binding, b"handle", [1; 32], b"canonical", &[])?
            )?,
            1
        );
        let mut magic = header(1, 0, PROVIDER)?;
        magic[0] = b'X';
        assert!(fixture.call(address, Some("service"), &magic).is_err());
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn signs_only_the_exactly_disclosed_activity() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        let address = fixture.gateway(limits())?;
        let created = fixture.call(address, Some("service"), &keyed(1, 1, [41; 32], &[])?)?;
        if created.len() != 109 || created[7] != OK || created[8..12] != 32_u32.to_be_bytes() {
            return Err("create did not return a description".into());
        }
        let handle = created[12..44].to_vec();
        let public: [u8; 32] = created[44..76].try_into()?;
        let Activity {
            unsigned,
            digest,
            encoded,
        } = owner_activity(public)?;
        let signature = fixture.signature(
            address,
            &signing([41; 32], &handle, digest, &unsigned, &encoded)?,
        )?;
        checked(verify_digest(&public, &signature, &digest))?;
        let mut tampered = encoded.clone();
        let last = tampered.len() - 1;
        tampered[last] ^= 1;
        assert_eq!(
            fixture.status(
                address,
                &signing([41; 32], &handle, digest, &unsigned, &tampered)?
            )?,
            1
        );
        let mut wrong = digest;
        wrong[0] ^= 1;
        assert_eq!(
            fixture.status(
                address,
                &signing([41; 32], &handle, wrong, &unsigned, &encoded)?
            )?,
            5
        );
        assert_eq!(
            fixture.status(
                address,
                &signing([42; 32], &handle, digest, &unsigned, &encoded)?
            )?,
            2
        );
        for (version, operation) in [(5, 14), (3, 6), (3, 11)] {
            assert_eq!(
                fixture.status(address, &keyed(version, operation, [41; 32], &handle)?)?,
                1
            );
        }
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn authenticates_only_the_pinned_client_and_backend_identity() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        let address = fixture.gateway(limits())?;
        fixture.provider(address, "service")?.probe()?;
        assert!(fixture.provider(address, "foreign")?.probe().is_err());
        assert!(fixture.provider(address, "relay")?.probe().is_err());
        assert!(fixture
            .call(address, None, &header(1, 0, PROVIDER)?)
            .is_err());
        let unauthenticated = LxkpGateway::new(
            Arc::new(fixture.provider(fixture.backend, "foreign")?),
            fixture.tls("service")?,
            policy(limits())?,
            Arc::clone(&fixture.clock),
        );
        assert!(matches!(unauthenticated, Err(CustodyError::Kms(_))));
        let wrong_provider = fixture.call(address, Some("service"), &header(1, 0, "other-kms")?)?;
        assert_eq!(wrong_provider[7], 1);
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn applies_rate_frame_and_connection_bounds() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        let probe = header(1, 0, PROVIDER)?;
        let rated = fixture.gateway(GatewayLimits {
            maximum_operations: 3,
            ..limits()
        })?;
        for _ in 0..3 {
            assert_eq!(fixture.status(rated, &probe)?, OK);
        }
        assert_eq!(fixture.status(rated, &probe)?, 4);
        let framed = fixture.gateway(GatewayLimits {
            maximum_frame_bytes: 1024,
            ..limits()
        })?;
        let oversized = keyed(1, 2, [43; 32], &[5; 1024])?;
        assert!(fixture.call(framed, Some("service"), &oversized).is_err());
        assert_eq!(fixture.status(framed, &probe)?, OK);
        let single = fixture.gateway(GatewayLimits {
            maximum_connections: 1,
            deadline: Duration::from_secs(2),
            ..limits()
        })?;
        let idle = TcpStream::connect(single)?;
        std::thread::sleep(Duration::from_millis(300));
        assert!(fixture.call(single, Some("service"), &probe).is_err());
        std::thread::sleep(Duration::from_secs(3));
        assert_eq!(fixture.status(single, &probe)?, OK);
        drop(idle);
        Ok(())
    }

    #[test]
    #[ignore = "process probe: run only by tools/paxeer-x/gates/104.38.4.sh with --include-ignored"]
    fn production_binary_serves_and_refuses_unbacked_startup() -> TestResult {
        let mut fixture = Fixture::new()?;
        fixture.start_backend()?;
        for kind in ["development", "envelope", ""] {
            let output = fixture
                .binary(free_address()?, kind, fixture.backend)?
                .output()?;
            let reason = if kind.is_empty() {
                "LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND is required"
            } else {
                "LAYERX_HUMAN_KMS_GATEWAY_PROVIDER_KIND must name the non-exportable remote KMS provider remote-lxkp"
            };
            assert!(
                refused_startup(&output, reason),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        let output = fixture
            .binary(free_address()?, "remote-lxkp", free_address()?)?
            .output()?;
        assert!(
            refused_startup(&output, "the gateway was refused: kms-timeout"),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let same = free_address()?;
        assert!(refused_startup(
            &fixture.binary(same, "remote-lxkp", same)?.output()?,
            "the gateway cannot be its own KMS provider"
        ));
        let listen = free_address()?;
        let mut gateway = fixture
            .binary(listen, "remote-lxkp", fixture.backend)?
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()?;
        let served = serve_probe(&fixture, listen, &mut gateway);
        let _ = gateway.kill();
        let _ = gateway.wait();
        served
    }

    fn serve_probe(fixture: &Fixture, listen: SocketAddr, gateway: &mut Child) -> TestResult {
        let client = fixture.provider(listen, "service")?;
        let mut deadline = Deadline::start(fixture.clock.as_ref(), Duration::from_secs(15))?;
        while client.probe().is_err() {
            if gateway.try_wait()?.is_some() {
                return Err("the gateway binary exited at startup".into());
            }
            if deadline.remaining(fixture.clock.as_ref())?.is_zero() {
                return Err("the gateway binary startup deadline passed".into());
            }
            std::thread::sleep(Duration::from_millis(20));
        }
        let snapshot = startup::snapshot(&fixture.path("chain.json"))?;
        let binding = startup::probe_binding(&snapshot, PROVIDER)?;
        let wrong_network =
            PrincipalKeyBinding::new(b"carol".to_vec(), NETWORK, KeyClass::HumanPrimary, PROVIDER)?;
        assert_eq!(client.create_key(&wrong_network), Err(KmsError::Refused));
        let created = client.create_key(&binding)?;
        assert_eq!(created.binding_digest(), binding.digest());
        assert_eq!(client.describe_key(&binding, created.reference())?, created);
        Ok(())
    }

    #[test]
    fn startup_material_must_be_private_owned_and_unlinked() -> TestResult {
        let fixture = Fixture::new()?;
        let secret = fixture.path("relay-key.der");
        assert!(protected(&secret, true).is_ok());
        fs::set_permissions(&secret, fs::Permissions::from_mode(0o640))?;
        assert!(protected(&secret, true).is_err());
        assert!(protected(&secret, false).is_ok());
        let linked = fixture.path("linked");
        std::os::unix::fs::symlink(&secret, &linked)?;
        assert!(protected(&linked, false).is_err());
        let empty = fixture.path("empty");
        fs::write(&empty, b"")?;
        fs::set_permissions(&empty, fs::Permissions::from_mode(0o600))?;
        assert!(protected(&empty, false).is_err());
        Ok(())
    }
}
