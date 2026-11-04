use std::collections::{BTreeMap, BTreeSet};
use std::fmt;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::{Mutex, MutexGuard};
use std::time::Duration;

use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::{
    AmountRole, Counterparty, CounterpartyRole, DisclosedAmount, Disclosure,
};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::Payment;
use layerx_crypto::signer::SignError;
use layerx_types::payload::ModuleId;
use rustls::pki_types::CertificateDer;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use zeroize::Zeroizing;

pub const KIND_ACTIVITY: &str = "lx_activity";
pub const MIN_SIGNERS: usize = 3;
pub const APPROVAL_VERSION: u8 = 1;
const PATH_SIGN: &str = "/v1/sign";
const PATH_SIGN_NATIVE: &str = "/v2/sign/native";
const PATH_GENERATE: &str = "/v1/keys/generate";
const PATH_PUBLIC_WALLET: &str = "/v1/keys/public-wallet";
const PATH_HEALTH: &str = "/health";
const PREIMAGE_DOMAIN: &[u8] = b"LXP/v1/signature-preimage\0";
const MAX_RESPONSE_BYTES: u64 = 65_536;
const MAX_IDENTIFIER: usize = 128;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AttestorError {
    Configuration(&'static str),
    Disclosure(SignError),
    WrongNetwork {
        expected: u32,
        actual: u32,
    },
    UnsupportedActivity {
        activity_type: u32,
    },
    Refused {
        node: String,
        status: u16,
        category: String,
        code: String,
        policy_code: Option<String>,
    },
    Timeout {
        node: String,
    },
    Unavailable {
        node: String,
    },
    Authentication {
        node: String,
    },
    MalformedResponse {
        node: String,
    },
    SignatureInvalid {
        node: String,
    },
}

impl fmt::Display for AttestorError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Configuration(what) => {
                write!(formatter, "attestor configuration refused: {what}")
            }
            Self::Disclosure(error) => write!(formatter, "attestor signing refused: {error}"),
            Self::WrongNetwork { expected, actual } => write!(
                formatter,
                "activity names network {actual} but the signer serves network {expected}"
            ),
            Self::UnsupportedActivity { activity_type } => write!(
                formatter,
                "activity type {activity_type} has no attestor disclosure contract"
            ),
            Self::Refused {
                node,
                status,
                category,
                code,
                policy_code,
            } => {
                write!(
                    formatter,
                    "attestor {node} refused with {status} {category}/{code}"
                )?;
                if let Some(policy) = policy_code {
                    write!(formatter, " policy {policy}")?;
                }
                Ok(())
            }
            Self::Timeout { node } => write!(formatter, "attestor {node} timed out"),
            Self::Unavailable { node } => write!(formatter, "attestor {node} is unavailable"),
            Self::Authentication { node } => {
                write!(
                    formatter,
                    "mutual authentication with attestor {node} failed"
                )
            }
            Self::MalformedResponse { node } => {
                write!(formatter, "attestor {node} returned a malformed response")
            }
            Self::SignatureInvalid { node } => {
                write!(formatter, "attestor {node} returned an invalid signature")
            }
        }
    }
}

impl std::error::Error for AttestorError {}

impl AttestorError {
    /// Names the attestor's own refusal: the policy code when the policy refused, otherwise the
    /// error code, and nothing for failures that are not attestor refusals.
    #[must_use]
    pub fn refusal_code(&self) -> Option<&str> {
        match self {
            Self::Refused {
                code, policy_code, ..
            } => Some(policy_code.as_deref().unwrap_or(code)),
            _ => None,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedSend {
    payload: Vec<u8>,
    canonical: Vec<u8>,
    disclosure: Disclosure,
    authorization: AttestorSignature,
    activity: AttestorSignature,
}

impl SignedSend {
    #[must_use]
    pub fn payload(&self) -> &[u8] {
        &self.payload
    }

    #[must_use]
    pub fn canonical(&self) -> &[u8] {
        &self.canonical
    }

    #[must_use]
    pub const fn disclosure(&self) -> &Disclosure {
        &self.disclosure
    }

    #[must_use]
    pub const fn authorization(&self) -> &AttestorSignature {
        &self.authorization
    }

    #[must_use]
    pub const fn activity(&self) -> &AttestorSignature {
        &self.activity
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AttestorHealth {
    pub node_id: String,
    pub ready: bool,
    pub audit_sequence: u64,
    pub reachable_peers: u32,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GeneratedKey {
    pub key_id: String,
    pub public_key: [u8; 32],
    pub audit: BTreeMap<String, u64>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AttestorSignature {
    signature: [u8; 64],
    digest: [u8; 32],
    audit: BTreeMap<String, u64>,
}

impl AttestorSignature {
    #[must_use]
    pub const fn signature(&self) -> &[u8; 64] {
        &self.signature
    }

    #[must_use]
    pub const fn digest(&self) -> &[u8; 32] {
        &self.digest
    }

    #[must_use]
    pub const fn audit(&self) -> &BTreeMap<String, u64> {
        &self.audit
    }
}

pub struct AttestorClient {
    agent: ureq::Agent,
    nodes: BTreeMap<String, SocketAddr>,
    deadline: Duration,
}

impl fmt::Debug for AttestorClient {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("AttestorClient")
            .field("nodes", &self.nodes)
            .field("identity", &"[gateway client identity]")
            .finish_non_exhaustive()
    }
}

impl AttestorClient {
    /// Builds a mutual-TLS client that trusts only the given attestor roots and presents the
    /// gateway client identity the attestors accept for key generation and signing.
    ///
    /// # Errors
    /// Refuses an empty or duplicated node list, unparsable roots, a missing client identity
    /// and a zero deadline.
    pub fn new(
        nodes: &[(String, SocketAddr)],
        roots: &[Vec<u8>],
        certificate_chain: &[Vec<u8>],
        private_key_pkcs8: &[u8],
        deadline: Duration,
    ) -> Result<Self, AttestorError> {
        let mut table = BTreeMap::new();
        for (id, address) in nodes {
            if !valid_identifier(id) {
                return Err(AttestorError::Configuration("attestor node id"));
            }
            if table.insert(id.clone(), *address).is_some() {
                return Err(AttestorError::Configuration("duplicate attestor node id"));
            }
        }
        if table.len() < MIN_SIGNERS {
            return Err(AttestorError::Configuration("too few attestor nodes"));
        }
        if roots.is_empty() || certificate_chain.is_empty() || private_key_pkcs8.is_empty() {
            return Err(AttestorError::Configuration(
                "attestor trust or client identity",
            ));
        }
        if deadline.is_zero() {
            return Err(AttestorError::Configuration("attestor deadline"));
        }
        let mut store = rustls::RootCertStore::empty();
        for root in roots {
            store
                .add(CertificateDer::from(root.clone()))
                .map_err(|_| AttestorError::Configuration("attestor trust root"))?;
        }
        let key = pkcs8_pem(private_key_pkcs8);
        let private_key = ureq::tls::PrivateKey::from_pem(key.as_bytes())
            .map_err(|_| AttestorError::Configuration("gateway client key"))?;
        let chain: Vec<_> = certificate_chain
            .iter()
            .map(|certificate| ureq::tls::Certificate::from_der(certificate).to_owned())
            .collect();
        let trusted: Vec<_> = roots
            .iter()
            .map(|root| ureq::tls::Certificate::from_der(root).to_owned())
            .collect();
        let identity = ureq::tls::ClientCert::new_with_certs(&chain, private_key);
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(ureq::tls::RootCerts::new_with_certs(&trusted))
                    .client_cert(Some(identity))
                    .build(),
            )
            .timeout_global(Some(deadline))
            .http_status_as_error(false)
            .max_redirects(0)
            .build()
            .into();
        Ok(Self {
            agent,
            nodes: table,
            deadline,
        })
    }

    #[must_use]
    pub fn nodes(&self) -> Vec<String> {
        self.nodes.keys().cloned().collect()
    }

    /// Reads one attestor's health report, ready or not.
    ///
    /// # Errors
    /// Returns transport, authentication and malformed-response failures.
    pub fn health(&self, node: &str) -> Result<AttestorHealth, AttestorError> {
        let address = self.address(node)?;
        let response = self
            .agent
            .get(format!("https://{address}{PATH_HEALTH}"))
            .header("accept", "application/json")
            .call()
            .map_err(|error| transport_error(node, &error))?;
        let (status, body) = read_body(node, response)?;
        if status != 200 && status != 503 {
            return Err(refusal(node, status, &body));
        }
        let report: HealthBody = serde_json::from_slice(&body).map_err(|_| malformed(node))?;
        if report.node_id != node {
            return Err(malformed(node));
        }
        Ok(AttestorHealth {
            node_id: report.node_id,
            ready: report.ready,
            audit_sequence: report.audit_sequence,
            reachable_peers: report.reachable_peers,
        })
    }

    /// Runs a distributed Ed25519 key generation on every configured attestor and returns the
    /// group key they all agree on.
    ///
    /// # Errors
    /// Refuses when any attestor refuses, disagrees on the group key or answers malformed data.
    pub fn generate_ed25519(
        &self,
        key_id: &str,
        owner: &str,
        account: &str,
    ) -> Result<GeneratedKey, AttestorError> {
        if !valid_identifier(key_id) || owner.is_empty() {
            return Err(AttestorError::Configuration("key id or owner"));
        }
        let session = session_id("keygen")?;
        let body = serde_json::to_vec(&GenerateBody {
            session_id: &session,
            key_id,
            curve: "ed25519",
            owner,
            account,
        })
        .map_err(|_| AttestorError::Configuration("generate request"))?;
        let nodes = self.nodes();
        let mut public_key = None;
        let mut audit = BTreeMap::new();
        for (node, result) in self.post_all(&nodes, PATH_GENERATE, &body, None) {
            let raw = result?;
            let response: KeyBody = serde_json::from_slice(&raw).map_err(|_| malformed(&node))?;
            if response.node_id != node
                || response.key_id != key_id
                || response.curve != "ed25519"
                || response.audit_sequence == 0
            {
                return Err(malformed(&node));
            }
            let key = decode_fixed::<32>(&response.public_key).ok_or_else(|| malformed(&node))?;
            match public_key {
                None => public_key = Some(key),
                Some(agreed) if agreed == key => {}
                Some(_) => return Err(malformed(&node)),
            }
            audit.insert(node, response.audit_sequence);
        }
        let public_key = public_key.ok_or(AttestorError::Configuration("no attestor nodes"))?;
        Ok(GeneratedKey {
            key_id: key_id.to_owned(),
            public_key,
            audit,
        })
    }

    pub fn public_wallet_identity(
        &self,
        ed_key_id: &str,
        public_key: &[u8; 32],
        owner: &str,
        assertion: &str,
    ) -> Result<[u8; 20], AttestorError> {
        self.public_wallet_key(ed_key_id, public_key, owner, assertion)
            .map(|(_, address)| address)
    }

    fn public_wallet_key(
        &self,
        ed_key_id: &str,
        public_key: &[u8; 32],
        owner: &str,
        assertion: &str,
    ) -> Result<(String, [u8; 20]), AttestorError> {
        if !valid_identifier(ed_key_id) || owner.is_empty() || assertion.is_empty() {
            return Err(AttestorError::Configuration(
                "public wallet identity request",
            ));
        }
        let session = session_id("public-wallet")?;
        let body = serde_json::to_vec(&PublicWalletRequest {
            session_id: &session,
            key_id: ed_key_id,
            public_key: &encode_hex(public_key),
            owner,
        })
        .map_err(|_| AttestorError::Configuration("public wallet identity request"))?;
        let mut identity = None;
        for (node, result) in
            self.post_all(&self.nodes(), PATH_PUBLIC_WALLET, &body, Some(assertion))
        {
            let raw = result?;
            let response: PublicWalletBody =
                serde_json::from_slice(&raw).map_err(|_| malformed(&node))?;
            let address = decode_fixed::<20>(
                response
                    .address
                    .strip_prefix("0x")
                    .ok_or_else(|| malformed(&node))?,
            )
            .ok_or_else(|| malformed(&node))?;
            let wallet_public =
                decode_fixed::<65>(&response.wallet_public_key).ok_or_else(|| malformed(&node))?;
            if response.node_id != node
                || response.key_id != ed_key_id
                || response.owner != owner
                || decode_fixed::<32>(&response.public_key) != Some(*public_key)
                || !valid_identifier(&response.wallet_key_id)
                || response.audit_sequence == 0
                || wallet_public[0] != 4
                || address == [0; 20]
            {
                return Err(malformed(&node));
            }
            let observed = (
                response.wallet_key_id,
                wallet_public,
                response.wallet_epoch,
                address,
            );
            match &identity {
                None => identity = Some(observed),
                Some(agreed) if agreed == &observed => {}
                Some(_) => return Err(malformed(&node)),
            }
        }
        identity
            .map(|(key_id, _, _, address)| (key_id, address))
            .ok_or(AttestorError::Configuration("no attestor wallet identity"))
    }

    fn address(&self, node: &str) -> Result<SocketAddr, AttestorError> {
        self.nodes
            .get(node)
            .copied()
            .ok_or(AttestorError::Configuration("unknown attestor node"))
    }

    fn post(
        &self,
        node: &str,
        path: &str,
        body: &[u8],
        bearer: Option<&str>,
        budget: Option<Duration>,
    ) -> Result<Vec<u8>, AttestorError> {
        let address = self.address(node)?;
        let mut request = self
            .agent
            .post(format!("https://{address}{path}"))
            .header("content-type", "application/json")
            .header("accept", "application/json");
        if let Some(token) = bearer {
            request = request.header("authorization", format!("Bearer {token}"));
        }
        let response = request
            .config()
            .timeout_global(Some(budget.unwrap_or(self.deadline).min(self.deadline)))
            .build()
            .send(body)
            .map_err(|error| transport_error(node, &error))?;
        let (status, body) = read_body(node, response)?;
        if status == 200 {
            Ok(body)
        } else {
            Err(refusal(node, status, &body))
        }
    }

    fn post_all(
        &self,
        nodes: &[String],
        path: &str,
        body: &[u8],
        bearer: Option<&str>,
    ) -> Vec<(String, Result<Vec<u8>, AttestorError>)> {
        self.post_all_bounded(nodes, path, body, bearer, None)
    }
    fn post_all_bounded(
        &self,
        nodes: &[String],
        path: &str,
        body: &[u8],
        bearer: Option<&str>,
        budget: Option<Duration>,
    ) -> Vec<(String, Result<Vec<u8>, AttestorError>)> {
        std::thread::scope(|scope| {
            let handles: Vec<_> = nodes
                .iter()
                .map(|node| {
                    (
                        node,
                        scope.spawn(move || self.post(node, path, body, bearer, budget)),
                    )
                })
                .collect();
            handles
                .into_iter()
                .map(|(node, handle)| {
                    let result = handle
                        .join()
                        .unwrap_or_else(|_| Err(AttestorError::Unavailable { node: node.clone() }));
                    (node.clone(), result)
                })
                .collect()
        })
    }
}

pub struct AttestorSigner {
    client: AttestorClient,
    key_id: String,
    public_key: [u8; 32],
    evm_key_id: String,
    evm_address: [u8; 20],
    evm_assertion: Option<Zeroizing<String>>,
    signers: Vec<String>,
    network: u32,
    audit: Mutex<BTreeMap<String, u64>>,
    native_replays: Mutex<BTreeMap<String, NativeReplay>>,
    session: Mutex<()>,
    native_journal: Option<NativeJournal>,
    native_clock: Option<std::sync::Arc<dyn layerx_types::clock::Clock>>,
}

#[derive(Clone, Debug, serde::Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AttestorNodeConfig {
    pub id: String,
    pub address: SocketAddr,
}

#[derive(Clone, Debug, serde::Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AttestorSignerConfig {
    pub nodes: Vec<AttestorNodeConfig>,
    pub trust_anchors: Vec<PathBuf>,
    pub client_certificate_chain: Vec<PathBuf>,
    pub client_private_key_pkcs8: PathBuf,
    pub key_id: String,
    pub public_key: [u8; 32],
    pub evm_key_id: String,
    pub evm_address: [u8; 20],
    pub signers: Vec<String>,
    pub network_id: u32,
    pub timeout_seconds: u64,
}

impl fmt::Debug for AttestorSigner {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("AttestorSigner")
            .field("client", &self.client)
            .field("key_id", &self.key_id)
            .field("signers", &self.signers)
            .field("network", &self.network)
            .finish_non_exhaustive()
    }
}

impl AttestorSigner {
    pub fn from_config(config: &AttestorSignerConfig) -> Result<Self, AttestorError> {
        if config.evm_key_id.trim().is_empty() || !valid_identifier(&config.evm_key_id) {
            return Err(AttestorError::Configuration("EVM key id"));
        }
        if config.evm_address == [0; 20] {
            return Err(AttestorError::Configuration("attestor chain or address"));
        }
        if config.timeout_seconds == 0 || config.timeout_seconds > 60 {
            return Err(AttestorError::Configuration("attestor timeout"));
        }
        let read = |path: &std::path::Path, secret| {
            crate::config::protected(path, 1_048_576, secret)
                .map_err(|_| AttestorError::Configuration("protected attestor TLS source"))
        };
        let roots = config
            .trust_anchors
            .iter()
            .map(|path| read(path, false).map(|bytes| bytes.to_vec()))
            .collect::<Result<Vec<_>, _>>()?;
        let certificates = config
            .client_certificate_chain
            .iter()
            .map(|path| read(path, false).map(|bytes| bytes.to_vec()))
            .collect::<Result<Vec<_>, _>>()?;
        let private_key = read(&config.client_private_key_pkcs8, true)?;
        let nodes = config
            .nodes
            .iter()
            .map(|node| (node.id.clone(), node.address))
            .collect::<Vec<_>>();
        let client = AttestorClient::new(
            &nodes,
            &roots,
            &certificates,
            &private_key,
            Duration::from_secs(config.timeout_seconds),
        )?;
        let signers = config
            .signers
            .iter()
            .map(String::as_str)
            .collect::<Vec<_>>();
        let mut signer = Self::new(
            client,
            &config.key_id,
            config.public_key,
            &signers,
            config.network_id,
        )?;
        signer.evm_key_id.clone_from(&config.evm_key_id);
        signer.evm_address = config.evm_address;
        Ok(signer)
    }

    pub fn set_evm_assertion(&mut self, assertion: &str) -> Result<(), AttestorError> {
        if assertion.is_empty() || assertion.len() > 65_536 {
            return Err(AttestorError::Configuration("EVM owner assertion"));
        }
        self.evm_assertion = Some(Zeroizing::new(assertion.to_owned()));
        Ok(())
    }

    pub fn with_public_wallet(
        mut self,
        owner: &str,
        assertion: &str,
    ) -> Result<Self, AttestorError> {
        self.set_evm_assertion(assertion)?;
        let (key_id, address) =
            self.client
                .public_wallet_key(&self.key_id, &self.public_key, owner, assertion)?;
        self.evm_key_id = key_id;
        self.evm_address = address;
        Ok(self)
    }

    #[must_use]
    pub const fn evm_address(&self) -> [u8; 20] {
        self.evm_address
    }

    pub fn sign_evm_transaction(
        &self,
        unsigned: &[u8],
        chain_id: u64,
    ) -> Result<[u8; 65], AttestorError> {
        self.sign_evm_transaction_bounded(unsigned, chain_id, None)
    }
    /// Signs the exact transaction within the smaller of the caller's remaining budget
    /// and the configured attestor timeout, preserving the same quorum and key checks.
    pub fn sign_evm_transaction_bounded(
        &self,
        unsigned: &[u8],
        chain_id: u64,
        budget: Option<Duration>,
    ) -> Result<[u8; 65], AttestorError> {
        if budget.is_some_and(|budget| budget.is_zero()) {
            return Err(AttestorError::Timeout {
                node: self.key_id.clone(),
            });
        }
        if !valid_identifier(&self.evm_key_id) {
            return Err(AttestorError::Configuration("EVM key id"));
        }
        let assertion = self
            .evm_assertion
            .as_ref()
            .ok_or(AttestorError::Configuration("EVM owner assertion"))?;
        self.sign_evm_transaction_with_assertion(
            unsigned,
            chain_id,
            assertion,
            &session_id("evm")?,
            budget,
        )
        .map(|signed| signed.signature)
    }

    fn sign_evm_transaction_with_assertion(
        &self,
        unsigned: &[u8],
        chain_id: u64,
        assertion: &str,
        session: &str,
        budget: Option<Duration>,
    ) -> Result<AttestorCallSignature, AttestorError> {
        self.sign_evm_transaction_for_key(
            unsigned,
            chain_id,
            assertion,
            session,
            budget,
            &self.evm_key_id,
            self.evm_address,
        )
    }
    fn sign_evm_transaction_for_key(
        &self,
        unsigned: &[u8],
        chain_id: u64,
        assertion: &str,
        session: &str,
        budget: Option<Duration>,
        key_id: &str,
        address: [u8; 20],
    ) -> Result<AttestorCallSignature, AttestorError> {
        if !valid_identifier(&key_id) {
            return Err(AttestorError::Configuration("EVM key id"));
        }
        let _session = self
            .session
            .lock()
            .map_err(|_| AttestorError::Configuration("signing session lock"))?;
        let (digest, transaction) = evm_transaction(unsigned, chain_id)?;
        let body = Zeroizing::new(
            serde_json::to_vec(&serde_json::json!({
                "session_id": session,
                "key_id": key_id,
                "kind": "evm_tx",
                "signers": self.signers,
                "transaction": encode_hex(&transaction),
            }))
            .map_err(|_| AttestorError::Configuration("EVM sign request"))?,
        );
        let mut signature = None;
        let mut audit = BTreeMap::new();
        for (node, result) in
            self.client
                .post_all_bounded(&self.signers, PATH_SIGN, &body, Some(assertion), budget)
        {
            let raw = result?;
            let response: SignResponseBody =
                serde_json::from_slice(&raw).map_err(|_| malformed(&node))?;
            let returned =
                decode_fixed::<65>(&response.signature).ok_or_else(|| malformed(&node))?;
            if response.node_id != node
                || response.key_id != key_id
                || response.kind != "evm_tx"
                || response.recovery_id != Some(returned[64])
                || returned[64] > 1
                || decode_fixed::<32>(&response.signed_bytes) != Some(digest)
            {
                return Err(malformed(&node));
            }
            let invalid = || AttestorError::SignatureInvalid { node: node.clone() };
            let compact =
                k256::ecdsa::Signature::from_slice(&returned[..64]).map_err(|_| invalid())?;
            if compact.normalize_s().is_some() {
                return Err(invalid());
            }
            let recovery = k256::ecdsa::RecoveryId::from_byte(returned[64]).ok_or_else(invalid)?;
            let recovered =
                k256::ecdsa::VerifyingKey::recover_from_prehash(&digest, &compact, recovery)
                    .map_err(|_| invalid())?;
            let point = recovered.to_encoded_point(false);
            if sha3::Keccak256::digest(&point.as_bytes()[1..])[12..] != address {
                return Err(invalid());
            }
            if signature.is_some_and(|agreed| agreed != returned) {
                return Err(malformed(&node));
            }
            signature = Some(returned);
            audit.insert(node, response.audit_sequence);
        }
        if audit.len() < MIN_SIGNERS {
            return Err(AttestorError::Configuration("signing quorum"));
        }
        let signature = signature.ok_or(AttestorError::Configuration("signing quorum"))?;
        let mut recorded = self.recorded();
        for (node, sequence) in &audit {
            if *sequence == 0
                || recorded
                    .get(node)
                    .is_some_and(|previous| sequence <= previous)
            {
                return Err(malformed(node));
            }
        }
        recorded.extend(
            audit
                .iter()
                .map(|(node, sequence)| (node.clone(), *sequence)),
        );
        Ok(AttestorCallSignature {
            signature,
            digest,
            audit,
        })
    }

    /// Binds a client to one attestor-held Ed25519 key, its expected group key, the signing
    /// quorum and the network whose activities it signs.
    ///
    /// # Errors
    /// Refuses fewer than three distinct signers, signers outside the client's nodes, an
    /// invalid key id and network zero.
    pub fn new(
        client: AttestorClient,
        key_id: &str,
        public_key: [u8; 32],
        signers: &[&str],
        network: u32,
    ) -> Result<Self, AttestorError> {
        if !valid_identifier(key_id) {
            return Err(AttestorError::Configuration("key id"));
        }
        if network == 0 {
            return Err(AttestorError::Configuration("network"));
        }
        let distinct: BTreeSet<&str> = signers.iter().copied().collect();
        if distinct.len() != signers.len() || distinct.len() < MIN_SIGNERS {
            return Err(AttestorError::Configuration("signing quorum"));
        }
        if distinct
            .iter()
            .any(|node| !client.nodes.contains_key(*node))
        {
            return Err(AttestorError::Configuration(
                "signer outside the attestor nodes",
            ));
        }
        Ok(Self {
            client,
            key_id: key_id.to_owned(),
            public_key,
            evm_key_id: String::new(),
            evm_address: [0; 20],
            evm_assertion: None,
            signers: distinct.into_iter().map(str::to_owned).collect(),
            network,
            audit: Mutex::new(BTreeMap::new()),
            native_replays: Mutex::new(BTreeMap::new()),
            session: Mutex::new(()),
            native_journal: None,
            native_clock: None,
        })
    }

    #[must_use]
    pub const fn public_key(&self) -> [u8; 32] {
        self.public_key
    }

    #[must_use]
    pub fn key_id(&self) -> &str {
        &self.key_id
    }

    #[must_use]
    pub fn client(&self) -> &AttestorClient {
        &self.client
    }

    #[must_use]
    pub fn audit_sequences(&self) -> BTreeMap<String, u64> {
        self.recorded().clone()
    }

    /// Signs a disclosure-bound EIP-1559 ledger call through the attestor quorum.
    pub fn sign_activity(
        &self,
        call: &Eip1559Call,
        disclosure: &Disclosure,
        from: [u8; 20],
        approval: &Approval<'_>,
        assertion: &str,
    ) -> Result<AttestorCallSignature, AttestorError> {
        self.sign_activity_with_clock(call, disclosure, from, approval, assertion, false)
    }

    fn sign_activity_with_clock(
        &self,
        call: &Eip1559Call,
        disclosure: &Disclosure,
        from: [u8; 20],
        approval: &Approval<'_>,
        assertion: &str,
        native_clock: bool,
    ) -> Result<AttestorCallSignature, AttestorError> {
        validate_disclosure(call, disclosure, from)?;
        let Some(ConstructMethod::LedgerSend { payload }) = disclosure
            .construct_method()
            .map_err(|error| AttestorError::Disclosure(SignError::from(error)))?
        else {
            return Err(AttestorError::Disclosure(SignError::InvalidDisclosure));
        };
        // bind_call has validated the payload, including its trailing network and version.
        let scope = &payload[payload.len() - 6..];
        let network = u32::from_be_bytes(scope[..4].try_into().map_err(|_| native_invalid())?);
        let version = u16::from_be_bytes(scope[4..].try_into().map_err(|_| native_invalid())?);
        if network != self.network {
            return Err(AttestorError::WrongNetwork {
                expected: self.network,
                actual: network,
            });
        }
        let chain = u32::try_from(call.chain_id)
            .map_err(|_| AttestorError::Configuration("EVM chain id"))?;
        if chain != self.network {
            return Err(AttestorError::WrongNetwork {
                expected: self.network,
                actual: chain,
            });
        }
        let (key_id, address) = if self.evm_address == [0; 20] {
            self.client.public_wallet_key(
                &self.key_id,
                &self.public_key,
                approval.principal,
                assertion,
            )?
        } else {
            (self.evm_key_id.clone(), self.evm_address)
        };
        if from != address {
            return Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
                "from",
            )));
        }
        if assertion.is_empty() {
            return Err(AttestorError::Configuration("user assertion"));
        }
        if native_clock && version != 3 {
            return Err(AttestorError::Configuration("native signing clock binding"));
        }
        approval.check(
            disclosure
                .expiry
                .not_after
                .min(disclosure.expiry.payload_expires_at),
        )?;
        let signer = AttestorTransactionSigner {
            signer: self,
            key_id: &key_id,
            address,
            assertion,
            session: approval.session_id,
            result: Mutex::new(None),
        };
        let mut future =
            layerx_crypto::signer::sign_disclosed_call(&signer, call, disclosure, from);
        let mut context = std::task::Context::from_waker(std::task::Waker::noop());
        let signature = match future.as_mut().poll(&mut context) {
            std::task::Poll::Ready(result) => result,
            std::task::Poll::Pending => {
                return Err(AttestorError::Configuration("synchronous attestor signer"));
            }
        };
        drop(future);
        let result = signer
            .result
            .into_inner()
            .map_err(|_| AttestorError::Configuration("signing result lock"))?;
        let mut signed = result.ok_or(AttestorError::Configuration("missing signing result"))??;
        signed.signature = *signature.map_err(AttestorError::Disclosure)?.as_bytes();
        Ok(signed)
    }

    pub fn sign_native_preparation_purpose(
        &self,
        purpose: &layerx_agent_api::identity::NativePreparationPurposeV1,
        session_id: &str,
        assertion: &str,
    ) -> Result<AttestorSignature, AttestorError> {
        let canonical = purpose.canonical_bytes().map_err(|_| native_invalid())?;
        let decoded = layerx_agent_api::identity::NativePreparationPurposeV1::from_canonical_bytes(
            &canonical,
        )
        .map_err(|_| native_invalid())?;
        if &decoded != purpose {
            return Err(native_invalid());
        }
        let purpose_expiry = purpose.expires_at_ms;
        let purpose = encode_hex(&canonical);
        self.request_native_signature(
            NativeSignBody {
                profile: 2,
                session_id,
                key_id: &self.key_id,
                operation: "preparation-purpose",
                signers: &self.signers,
                grant: None,
                purpose: Some(&purpose),
                capability: None,
                native_session: None,
                expiry_ms: None,
            },
            Sha256::digest(&canonical).into(),
            purpose_expiry,
            assertion,
        )
    }

    pub fn sign_native_send_purpose(
        &self,
        purpose: &layerx_agent_api::identity::NativeSendPurposeV1,
        session_id: &str,
        assertion: &str,
    ) -> Result<AttestorSignature, AttestorError> {
        let canonical = purpose.canonical_bytes().map_err(|_| native_invalid())?;
        let decoded =
            layerx_agent_api::identity::NativeSendPurposeV1::from_canonical_bytes(&canonical)
                .map_err(|_| native_invalid())?;
        if &decoded != purpose || purpose.owner_public_key != self.public_key {
            return Err(native_invalid());
        }
        let purpose_expiry = purpose.expires_at_ms;
        let purpose = encode_hex(&canonical);
        self.request_native_signature(
            NativeSignBody {
                profile: 2,
                session_id,
                key_id: &self.key_id,
                operation: "send-purpose",
                signers: &self.signers,
                grant: None,
                purpose: Some(&purpose),
                capability: None,
                native_session: None,
                expiry_ms: None,
            },
            Sha256::digest(&canonical).into(),
            purpose_expiry,
            assertion,
        )
    }

    pub fn sign_native_local_grant(
        &self,
        grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
        session_id: &str,
        assertion: &str,
    ) -> Result<AttestorSignature, AttestorError> {
        if grant.owner_public_key != self.public_key || grant.signature != [0; 64] {
            return Err(native_invalid());
        }
        validate_native_local_grant(grant)?;
        let canonical = native_local_grant_signing_bytes(grant)?;
        let capability = encode_hex(&grant.capability);
        let native_session = encode_hex(&grant.session_scope);
        let expiry = grant.expires_at_ms.to_string();
        self.request_native_signature(
            NativeSignBody {
                profile: 2,
                session_id,
                key_id: &self.key_id,
                operation: "local-grant-consent",
                signers: &self.signers,
                grant: None,
                purpose: None,
                capability: Some(&capability),
                native_session: Some(&native_session),
                expiry_ms: Some(&expiry),
            },
            Sha256::digest(&canonical).into(),
            grant.expires_at_ms,
            assertion,
        )
    }

    pub fn sign_ledger_grant(
        &self,
        grant: &layerx_crypto::payments::Grant,
        session_id: &str,
        assertion: &str,
    ) -> Result<AttestorSignature, AttestorError> {
        if grant.public_key != self.public_key || grant.signature != [0; 64] {
            return Err(native_invalid());
        }
        let digest = grant.signing_digest().map_err(|_| native_invalid())?;
        let canonical = [
            grant.id.as_slice(),
            grant.from.as_slice(),
            grant.recipient.as_slice(),
            grant.asset.as_slice(),
            &grant.per_draw_maximum.to_be_bytes(),
            &grant.allowance.to_be_bytes(),
            &[u8::from(grant.recurring)],
            &grant.window_length.to_be_bytes(),
            &grant.expiration.to_be_bytes(),
            &grant.purpose_hash,
            &[u8::from(grant.has_reference)],
            &grant.reference_hash,
            &grant.revocation_sequence.to_be_bytes(),
            &grant.public_key,
            &grant.signature,
        ]
        .concat();
        let expiry = grant.expiration;
        let grant = encode_hex(&canonical);
        self.request_native_signature(
            NativeSignBody {
                profile: 2,
                session_id,
                key_id: &self.key_id,
                operation: "ledger-issue-grant",
                signers: &self.signers,
                grant: Some(&grant),
                purpose: None,
                capability: None,
                native_session: None,
                expiry_ms: None,
            },
            digest,
            expiry,
            assertion,
        )
    }

    pub fn sign_ledger_grant_with_stored_assertion(
        &self,
        grant: &layerx_crypto::payments::Grant,
    ) -> Result<AttestorSignature, AttestorError> {
        let assertion = self
            .evm_assertion
            .as_ref()
            .ok_or(AttestorError::Configuration("EVM owner assertion"))?;
        self.sign_ledger_grant(grant, &session_id("ledger-grant")?, assertion)
    }

    fn request_native_signature(
        &self,
        body: NativeSignBody<'_>,
        digest: [u8; 32],
        expires_at_ms: u64,
        assertion: &str,
    ) -> Result<AttestorSignature, AttestorError> {
        if !valid_identifier(body.session_id) || assertion.is_empty() {
            return Err(AttestorError::Configuration("native assertion/session"));
        }
        let _session = self
            .session
            .lock()
            .map_err(|_| AttestorError::Configuration("signing session lock"))?;
        let runtime;
        let clock = if let Some(clock) = &self.native_clock {
            clock.as_ref()
        } else {
            runtime = layerx_client::runtime_clock::RuntimeClock::from_environment()
                .map_err(|_| AttestorError::Configuration("native replay clock"))?;
            runtime.as_ref()
        };
        let now = clock
            .sample(Duration::from_secs(1))
            .map_err(|_| AttestorError::Configuration("native replay clock"))?
            .unix_seconds();
        let claims = assertion_claims(assertion)?;
        claims.check(now)?;
        let retain_until = claims.retain_until(expires_at_ms);
        if now >= retain_until {
            return Err(assertion_expired());
        }
        let bytes = Zeroizing::new(
            serde_json::to_vec(&body)
                .map_err(|_| AttestorError::Configuration("native sign request"))?,
        );
        let request_digest: [u8; 32] = Sha256::digest(&*bytes).into();
        let mut replays = self
            .native_replays
            .lock()
            .map_err(|_| AttestorError::Configuration("native replay lock"))?;
        let mut staged = replays.clone();
        staged.retain(|_, record| record.retain_until > now);
        let previous = staged.get(body.session_id).cloned();
        if previous
            .as_ref()
            .is_some_and(|record| record.request_digest != request_digest)
        {
            return Err(native_invalid());
        }
        // Without a local journal, the native-v2 attestor's durable key/session request binding
        // is authoritative. This cache only recognizes recent exact replies; evicted stale
        // replies remain refused by the monotonic audit check below.
        if previous.is_none() && self.native_journal.is_none() && staged.len() >= 512 {
            let oldest = staged
                .iter()
                .min_by_key(|(_, record)| record.audit.values().copied().min().unwrap_or(0))
                .map(|(key, _)| key.clone());
            if let Some(oldest) = oldest {
                staged.remove(&oldest);
            }
        }
        staged
            .entry(body.session_id.to_owned())
            .or_insert_with(|| NativeReplay {
                request_digest,
                signature: Vec::new(),
                digest,
                audit: BTreeMap::new(),
                retain_until,
            });
        self.persist_native(&staged)?;
        *replays = staged;
        let results =
            self.client
                .post_all(&self.signers, PATH_SIGN_NATIVE, &bytes, Some(assertion));
        let mut signature = None;
        let mut audit = BTreeMap::new();
        for (node, result) in results {
            let response: NativeSignResponse =
                serde_json::from_slice(&result?).map_err(|_| malformed(&node))?;
            if response.profile != 2
                || response.node_id != node
                || response.key_id != self.key_id
                || response.operation != body.operation
            {
                return Err(malformed(&node));
            }
            let signed =
                decode_fixed::<32>(&response.signed_bytes).ok_or_else(|| malformed(&node))?;
            let returned =
                decode_fixed::<64>(&response.signature).ok_or_else(|| malformed(&node))?;
            if response.signed_bytes != encode_hex(&signed)
                || response.signature != encode_hex(&returned)
                || !layerx_crypto::ct::eq_fixed(&signed, &digest)
            {
                return Err(malformed(&node));
            }
            layerx_crypto::ed25519::verify_digest(&self.public_key, &returned, &digest)
                .map_err(|_| AttestorError::SignatureInvalid { node: node.clone() })?;
            if signature.is_some_and(|previous| previous != returned) {
                return Err(malformed(&node));
            }
            signature = Some(returned);
            audit.insert(node, response.audit_sequence);
        }
        if audit.len() != self.signers.len() {
            return Err(AttestorError::Configuration("native signing quorum"));
        }
        let signature = signature.ok_or(AttestorError::Configuration("native signing quorum"))?;
        let exact_replay = previous.as_ref().is_some_and(|record| {
            record.signature == signature && record.digest == digest && record.audit == audit
        });
        let mut recorded = self.recorded();
        for (node, sequence) in &audit {
            if *sequence == 0
                || (!exact_replay
                    && recorded
                        .get(node)
                        .is_some_and(|previous| sequence <= previous))
            {
                return Err(malformed(node));
            }
        }
        for (node, sequence) in &audit {
            let entry = recorded.entry(node.clone()).or_default();
            *entry = (*entry).max(*sequence);
        }
        replays.insert(
            body.session_id.to_owned(),
            NativeReplay {
                request_digest,
                signature: signature.to_vec(),
                digest,
                audit: audit.clone(),
                retain_until,
            },
        );
        self.persist_native(&replays)?;
        Ok(AttestorSignature {
            signature,
            digest,
            audit,
        })
    }

    /// Retains native-v2 key/session bindings through restart until the earlier of the
    /// consent expiry and the owner assertion's one-hour authorization horizon. The
    /// protected journal is byte bounded; capacity is refused before contacting attestors.
    pub fn with_native_replay_journal(
        mut self,
        path: PathBuf,
        clock: std::sync::Arc<dyn layerx_types::clock::Clock>,
    ) -> Result<Self, AttestorError> {
        use std::os::unix::fs::OpenOptionsExt;
        if !path.is_absolute() {
            return Err(AttestorError::Configuration("native journal path"));
        }
        let lock = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .mode(0o600)
            .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
            .open(path.with_extension("lock"))
            .map_err(|_| AttestorError::Configuration("native journal lock"))?;
        use std::os::unix::fs::MetadataExt;
        let metadata = lock
            .metadata()
            .map_err(|_| AttestorError::Configuration("native journal metadata"))?;
        if !metadata.is_file()
            || metadata.uid() != rustix::process::geteuid().as_raw()
            || metadata.mode() & 0o077 != 0
        {
            return Err(AttestorError::Configuration("native journal ownership"));
        }
        lock.try_lock()
            .map_err(|_| AttestorError::Configuration("native journal already owned"))?;
        let records = if path
            .try_exists()
            .map_err(|_| AttestorError::Configuration("native journal lookup"))?
        {
            let bytes = crate::config::protected(&path, NATIVE_JOURNAL_BYTES, true)
                .map_err(|_| AttestorError::Configuration("native journal source"))?;
            let journal: NativeJournalData = serde_json::from_slice(&bytes)
                .map_err(|_| AttestorError::Configuration("native journal encoding"))?;
            if journal.key_id != self.key_id
                || journal.public_key != self.public_key
                || journal.network != self.network
                || journal.signers != self.signers
            {
                return Err(AttestorError::Configuration("native journal key binding"));
            }
            journal.records
        } else {
            BTreeMap::new()
        };
        let now = clock
            .sample(Duration::from_secs(1))
            .map_err(|_| AttestorError::Configuration("native replay clock"))?
            .unix_seconds();
        let mut records = records;
        records.retain(|_, record| record.retain_until > now);
        for record in records.values() {
            if !record.signature.is_empty() && record.signature.len() != 64 {
                return Err(native_invalid());
            }
            if !record.signature.is_empty() {
                let signature: [u8; 64] = record
                    .signature
                    .as_slice()
                    .try_into()
                    .map_err(|_| native_invalid())?;
                layerx_crypto::ed25519::verify_digest(&self.public_key, &signature, &record.digest)
                    .map_err(|_| native_invalid())?;
                if record.audit.len() != self.signers.len()
                    || record
                        .audit
                        .iter()
                        .any(|(node, sequence)| *sequence == 0 || !self.signers.contains(node))
                {
                    return Err(native_invalid());
                }
            }
            for (node, sequence) in &record.audit {
                let mut audit = self.recorded();
                let value = audit.entry(node.clone()).or_default();
                *value = (*value).max(*sequence);
            }
        }
        self.native_replays = Mutex::new(records);
        self.native_clock = Some(clock);
        self.native_journal = Some(NativeJournal { path, _lock: lock });
        {
            let replays = self.native_replays.lock().map_err(|_| native_invalid())?;
            self.persist_native(&replays)?;
        }
        Ok(self)
    }
    fn persist_native(
        &self,
        records: &BTreeMap<String, NativeReplay>,
    ) -> Result<(), AttestorError> {
        use std::io::Write;
        use std::os::unix::fs::OpenOptionsExt;
        let Some(journal) = &self.native_journal else {
            return Ok(());
        };
        let data = NativeJournalData {
            key_id: self.key_id.clone(),
            public_key: self.public_key,
            network: self.network,
            signers: self.signers.clone(),
            records: records.clone(),
        };
        let bytes = Zeroizing::new(serde_json::to_vec(&data).map_err(|_| native_invalid())?);
        let reserved = records
            .values()
            .filter(|record| record.signature.is_empty())
            .count()
            .saturating_mul(2048);
        if bytes.len().saturating_add(reserved) > NATIVE_JOURNAL_BYTES {
            return Err(AttestorError::Configuration("native journal capacity"));
        }
        let temp = journal.path.with_extension("next");
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .mode(0o600)
            .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
            .open(&temp)
            .map_err(|_| AttestorError::Configuration("native journal write"))?;
        file.write_all(&bytes)
            .and_then(|()| file.sync_all())
            .map_err(|_| AttestorError::Configuration("native journal sync"))?;
        std::fs::rename(temp, &journal.path)
            .and_then(|()| {
                std::fs::File::open(
                    journal
                        .path
                        .parent()
                        .ok_or_else(|| std::io::Error::other("journal parent"))?,
                )
                .and_then(|file| file.sync_all())
            })
            .map_err(|_| AttestorError::Configuration("native journal publish"))
    }
    fn recorded(&self) -> MutexGuard<'_, BTreeMap<String, u64>> {
        match self.audit.lock() {
            Ok(guard) => guard,
            Err(poisoned) => poisoned.into_inner(),
        }
    }
}

fn validate_disclosure(
    call: &Eip1559Call,
    disclosure: &Disclosure,
    from: [u8; 20],
) -> Result<(), AttestorError> {
    let refuse =
        |field: &'static str| AttestorError::Disclosure(SignError::DisclosureMismatch(field));
    let actual =
        bind_call(call, from).map_err(|error| AttestorError::Disclosure(SignError::from(error)))?;
    macro_rules! require_field {
        ($($field:ident),+) => {
            $(if disclosure.$field != actual.$field {
                return Err(refuse(stringify!($field)));
            })+
        };
    }
    require_field!(
        activity_type,
        actor,
        authority,
        counterparties,
        amounts,
        asset,
        fee_limit,
        expiry,
        idempotency_key,
        evm_payout_binding,
        withdrawal,
        payment,
        authority_grant,
        session_grant,
        onboarding,
        native_operation
    );
    if *disclosure != actual {
        return Err(refuse("canonical_bytes"));
    }
    Ok(())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AttestorCallSignature {
    signature: [u8; 65],
    digest: [u8; 32],
    audit: BTreeMap<String, u64>,
}

impl AttestorCallSignature {
    #[must_use]
    pub const fn signature(&self) -> &[u8; 65] {
        &self.signature
    }

    #[must_use]
    pub const fn digest(&self) -> &[u8; 32] {
        &self.digest
    }

    #[must_use]
    pub const fn audit(&self) -> &BTreeMap<String, u64> {
        &self.audit
    }
}

struct AttestorTransactionSigner<'a> {
    signer: &'a AttestorSigner,
    assertion: &'a str,
    key_id: &'a str,
    address: [u8; 20],
    session: &'a str,
    result: Mutex<Option<Result<AttestorCallSignature, AttestorError>>>,
}

impl fmt::Debug for AttestorTransactionSigner<'_> {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("AttestorTransactionSigner")
            .field("signer", self.signer)
            .finish_non_exhaustive()
    }
}

impl layerx_crypto::signer::Signer for AttestorTransactionSigner<'_> {
    fn public_key(&self) -> [u8; 32] {
        self.signer.public_key()
    }

    fn sign<'a>(
        &'a self,
        _request: layerx_crypto::signer::SigningRequest<'a>,
    ) -> layerx_crypto::signer::SignFuture<'a> {
        // This adapter authorizes EVM transactions only; DID signing uses the native endpoints.
        Box::pin(std::future::ready(Err(SignError::RemoteRefused)))
    }

    fn evm_address(&self) -> Option<[u8; 20]> {
        Some(self.address)
    }

    fn sign_transaction<'a>(
        &'a self,
        call: &'a Eip1559Call,
    ) -> layerx_crypto::signer::TransactionSignFuture<'a> {
        Box::pin(async move {
            let result = self.signer.sign_evm_transaction_for_key(
                &unsigned_evm_transaction(call),
                call.chain_id,
                self.assertion,
                self.session,
                None,
                self.key_id,
                self.address,
            );
            let signature = match &result {
                Ok(signed) => {
                    let mut signature = signed.signature;
                    signature[64] += 27;
                    layerx_crypto::signer::AgentSignature::new(signature)
                        .map_err(|_| SignError::ReturnedSignatureInvalid)
                }
                Err(_) => Err(SignError::RemoteRefused),
            };
            *self
                .result
                .lock()
                .map_err(|_| SignError::RemoteUnavailable)? = Some(result);
            signature
        })
    }
}

/// Computes the domain-separated activity preimage the attestors sign.
#[must_use]
pub fn preimage(canonical: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(PREIMAGE_DOMAIN);
    hash.update(canonical);
    hash.finalize().into()
}

fn evm_rlp_item(raw: &[u8]) -> Option<(&[u8], usize, bool)> {
    let prefix = *raw.first()?;
    if prefix < 0x80 {
        return Some((&raw[..1], 1, false));
    }
    let list = prefix >= 0xc0;
    let short = if list { 0xc0 } else { 0x80 };
    let long = short + 55;
    let (offset, length) = if prefix <= long {
        (1, usize::from(prefix - short))
    } else {
        let count = usize::from(prefix - long);
        let encoded = raw.get(1..1 + count)?;
        if encoded.first() == Some(&0) {
            return None;
        }
        let length = encoded.iter().try_fold(0_usize, |length, byte| {
            length.checked_mul(256)?.checked_add(usize::from(*byte))
        })?;
        if length <= 55 {
            return None;
        }
        (1 + count, length)
    };
    let end = offset.checked_add(length)?;
    let payload = raw.get(offset..end)?;
    if !list && length == 1 && payload[0] < 0x80 {
        return None;
    }
    Some((payload, end, list))
}

fn evm_rlp_list(payload: &[u8]) -> Vec<u8> {
    let mut out = Vec::new();
    if let Ok(length @ 0..=55) = u8::try_from(payload.len()) {
        out.push(0xc0 + length);
    } else {
        let length = payload.len().to_be_bytes();
        let significant = &length[length.iter().take_while(|byte| **byte == 0).count()..];
        out.push(0xf7 + significant.iter().fold(0_u8, |count, _| count + 1));
        out.extend_from_slice(significant);
    }
    out.extend_from_slice(payload);
    out
}

pub fn unsigned_evm_transaction(call: &Eip1559Call) -> Vec<u8> {
    fn bytes(value: &[u8], out: &mut Vec<u8>) {
        if value.len() == 1 && value[0] < 0x80 {
            out.extend_from_slice(value);
        } else {
            let mut encoded = evm_rlp_list(value);
            encoded[0] -= 0x40;
            out.extend(encoded);
        }
    }
    fn integer(value: &[u8], out: &mut Vec<u8>) {
        let start = value
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or(value.len());
        bytes(&value[start..], out);
    }
    let mut fields = Vec::new();
    integer(&call.chain_id.to_be_bytes(), &mut fields);
    integer(&call.nonce.to_be_bytes(), &mut fields);
    integer(&call.max_priority_fee_per_gas.to_be_bytes(), &mut fields);
    integer(&call.max_fee_per_gas.to_be_bytes(), &mut fields);
    integer(&call.gas_limit.to_be_bytes(), &mut fields);
    bytes(&call.to, &mut fields);
    fields.push(0x80);
    bytes(&call.data, &mut fields);
    fields.push(0xc0);
    let mut unsigned = vec![2];
    unsigned.extend(evm_rlp_list(&fields));
    unsigned
}

pub fn decode_unsigned_evm_transaction(unsigned: &[u8]) -> Result<Eip1559Call, AttestorError> {
    let invalid = || AttestorError::Configuration("unsigned EIP-1559 call");
    if unsigned.first() != Some(&2) || unsigned.len() > 1_048_576 {
        return Err(invalid());
    }
    let (mut fields, length, list) = evm_rlp_item(&unsigned[1..]).ok_or_else(invalid)?;
    if !list || length != unsigned.len() - 1 {
        return Err(invalid());
    }
    fn take<'a>(fields: &mut &'a [u8]) -> Option<(&'a [u8], bool)> {
        let (value, length, list) = evm_rlp_item(fields)?;
        *fields = &fields[length..];
        Some((value, list))
    }
    fn integer<const N: usize>(fields: &mut &[u8]) -> Option<[u8; N]> {
        let (value, list) = take(fields)?;
        if list || value.first() == Some(&0) || value.len() > N {
            return None;
        }
        let mut out = [0; N];
        out[N - value.len()..].copy_from_slice(value);
        Some(out)
    }
    let chain_id = u64::from_be_bytes(integer(&mut fields).ok_or_else(invalid)?);
    let nonce = u64::from_be_bytes(integer(&mut fields).ok_or_else(invalid)?);
    let max_priority_fee_per_gas = u128::from_be_bytes(integer(&mut fields).ok_or_else(invalid)?);
    let max_fee_per_gas = u128::from_be_bytes(integer(&mut fields).ok_or_else(invalid)?);
    let gas_limit = u64::from_be_bytes(integer(&mut fields).ok_or_else(invalid)?);
    let (to, list) = take(&mut fields).ok_or_else(invalid)?;
    if list || integer::<32>(&mut fields) != Some([0; 32]) {
        return Err(invalid());
    }
    let to = to.try_into().map_err(|_| invalid())?;
    let (data, list) = take(&mut fields).ok_or_else(invalid)?;
    if list || take(&mut fields) != Some((&[][..], true)) || !fields.is_empty() || chain_id == 0 {
        return Err(invalid());
    }
    let call = Eip1559Call {
        chain_id,
        nonce,
        max_priority_fee_per_gas,
        max_fee_per_gas,
        gas_limit,
        to,
        data: data.to_vec(),
    };
    if unsigned_evm_transaction(&call) != unsigned {
        return Err(invalid());
    }
    Ok(call)
}

fn evm_transaction(unsigned: &[u8], chain_id: u64) -> Result<([u8; 32], Vec<u8>), AttestorError> {
    let invalid = || AttestorError::Configuration("unsigned EIP-1559 transaction");
    if unsigned.first() != Some(&2) || unsigned.len() > 1_048_576 || chain_id == 0 {
        return Err(invalid());
    }
    let (payload, length, list) = evm_rlp_item(&unsigned[1..]).ok_or_else(invalid)?;
    if !list || length != unsigned.len() - 1 {
        return Err(invalid());
    }
    let mut fields = Vec::new();
    let mut offset = 0;
    while offset < payload.len() {
        let (value, length, list) = evm_rlp_item(&payload[offset..]).ok_or_else(invalid)?;
        fields.push((value, list, offset + length));
        offset += length;
        if fields.len() > 12 {
            return Err(invalid());
        }
    }
    if fields.len() != 9 && fields.len() != 12 {
        return Err(invalid());
    }
    for index in [0, 1, 2, 3, 4, 6] {
        let (value, list, _) = fields[index];
        let max = if matches!(index, 0 | 1 | 4) { 8 } else { 32 };
        if list || value.len() > max || value.first() == Some(&0) {
            return Err(invalid());
        }
    }
    let actual = fields[0]
        .0
        .iter()
        .fold(0_u64, |value, byte| (value << 8) | u64::from(*byte));
    if actual != chain_id
        || fields[5].1
        || !matches!(fields[5].0.len(), 0 | 20)
        || fields[7].1
        || !fields[8].1
        || fields[9..]
            .iter()
            .any(|(value, list, _)| *list || !value.is_empty())
    {
        return Err(invalid());
    }
    let signing_fields = &payload[..fields[8].2];
    let mut preimage = vec![2];
    preimage.extend(evm_rlp_list(signing_fields));
    let digest = sha3::Keccak256::digest(preimage).into();
    let mut wire_fields = signing_fields.to_vec();
    wire_fields.extend_from_slice(&[0x80, 0x80, 0x80]);
    let mut wire = vec![2];
    wire.extend(evm_rlp_list(&wire_fields));
    Ok((digest, wire))
}

#[derive(Serialize)]
struct GenerateBody<'a> {
    session_id: &'a str,
    key_id: &'a str,
    curve: &'a str,
    owner: &'a str,
    account: &'a str,
}

#[derive(Serialize)]
struct SignBody<'a> {
    #[serde(skip_serializing_if = "Option::is_none")]
    clock_profile: Option<&'a str>,
    session_id: &'a str,
    key_id: &'a str,
    kind: &'a str,
    signers: &'a [String],
    activity: &'a str,
    disclosure: &'a DisclosureWire,
    approval: ApprovalWire<'a>,
}

/// Wire form of the approved disclosure: every u128 amount and u64 sequence or bound is a
/// canonical unsigned decimal string, the representation the attestor and gateway share.
#[derive(Serialize, Clone, Debug, Eq, PartialEq)]
struct DisclosureWire {
    account: String,
    module: &'static str,
    operation: u16,
    amounts: Vec<AmountWire>,
    destinations: Vec<String>,
    sequence: String,
    not_before: String,
    not_after: String,
}

#[derive(Serialize, Clone, Debug, Eq, PartialEq)]
struct AmountWire {
    asset: String,
    amount: String,
}

#[derive(Serialize)]
struct ApprovalWire<'a> {
    version: u8,
    principal: &'a str,
    key_id: &'a str,
    network_id: u32,
    protocol_version: u16,
    session_id: &'a str,
    activity_digest: String,
    expires_at: String,
}

/// Authorization binding handed over by the original approval boundary: the trusted
/// principal that owns the key, the request session allocated there and the instant after
/// which the approval is void, never later than the approved envelope `not_after`.
#[derive(Clone, Copy, Debug)]
pub struct Approval<'a> {
    pub principal: &'a str,
    pub session_id: &'a str,
    pub expires_at: u64,
}

impl Approval<'_> {
    fn check(&self, not_after: u64) -> Result<(), AttestorError> {
        if !attestor_principal_valid(self.principal) || !valid_identifier(self.session_id) {
            return Err(AttestorError::Configuration("approval binding"));
        }
        if self.expires_at > not_after {
            return Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
                "approval_expiry",
            )));
        }
        Ok(())
    }
}

/// Approval boundary context for both SEND stages: one principal and expiry, and two
/// distinct sessions for the owner authorization and the completed envelope.
#[derive(Clone, Copy, Debug)]
pub struct SendApproval<'a> {
    pub principal: &'a str,
    pub authorization_session: &'a str,
    pub activity_session: &'a str,
    pub expires_at: u64,
}

/// Allocates a request session at the approval boundary.
///
/// # Errors
/// Refuses when the platform randomness source fails.
pub fn new_session_id(label: &str) -> Result<String, AttestorError> {
    session_id(label)
}

fn attestor_principal_valid(principal: &str) -> bool {
    !principal.is_empty() && principal.len() <= 1_024 && !principal.chars().any(char::is_control)
}

#[allow(clippy::too_many_arguments)]
fn disclosure_wire(
    account: [u8; 32],
    module: &'static str,
    operation: u16,
    amounts: Vec<([u8; 32], u128)>,
    destinations: Vec<[u8; 32]>,
    sequence: u64,
    not_before: u64,
    not_after: u64,
) -> DisclosureWire {
    DisclosureWire {
        account: encode_hex(&account),
        module,
        operation,
        amounts: amounts
            .into_iter()
            .map(|(asset, amount)| AmountWire {
                asset: encode_hex(&asset),
                amount: amount.to_string(),
            })
            .collect(),
        destinations: destinations.iter().map(|id| encode_hex(id)).collect(),
        sequence: sequence.to_string(),
        not_before: not_before.to_string(),
        not_after: not_after.to_string(),
    }
}

/// Projects an approved disclosure onto the attestor effect contract. Only asset send,
/// budget fund, asset grant issue (the grant allowance) and program transfer (each leg with
/// its own asset) have a contract; every other activity is refused, never approximated.
fn project(d: &Disclosure) -> Result<DisclosureWire, AttestorError> {
    let kind = d.activity_type;
    let mismatch = || AttestorError::Disclosure(SignError::DisclosureMismatch("semantics"));
    let wire = |account, module, amounts, destinations| {
        disclosure_wire(
            account,
            module,
            kind.ordinal(),
            amounts,
            destinations,
            d.envelope_sequence(),
            d.expiry.not_before,
            d.expiry.not_after,
        )
    };
    match (kind.module(), kind.ordinal(), &d.payment) {
        (ModuleId::Asset, 5, None) | (ModuleId::Budget, 2, None) => {
            let module = if kind.module() == ModuleId::Asset {
                "asset"
            } else {
                "budget"
            };
            match (d.counterparties.as_slice(), d.amounts.as_slice()) {
                (
                    [Counterparty {
                        role: CounterpartyRole::Payer,
                        account: from,
                    }, Counterparty {
                        role: CounterpartyRole::Recipient,
                        account: to,
                    }],
                    [DisclosedAmount {
                        role: AmountRole::Transfer,
                        value,
                    }],
                ) => Ok(wire(*from, module, vec![(d.asset, *value)], vec![*to])),
                _ => Err(mismatch()),
            }
        }
        (ModuleId::Asset, 7, Some(Payment::IssueGrant(grant))) => Ok(wire(
            grant.from,
            "asset",
            vec![(grant.asset, grant.allowance)],
            vec![grant.recipient],
        )),
        (ModuleId::Programs, 5, Some(Payment::ProgramTransfer { legs, .. })) => {
            let first = legs.first().ok_or_else(mismatch)?;
            Ok(wire(
                first.from,
                "programs",
                legs.iter().map(|leg| (leg.asset, leg.amount)).collect(),
                legs.iter().map(|leg| leg.to).collect(),
            ))
        }
        _ => Err(AttestorError::UnsupportedActivity {
            activity_type: kind.value(),
        }),
    }
}

#[derive(Deserialize)]
struct KeyBody {
    node_id: String,
    key_id: String,
    curve: String,
    public_key: String,
    audit_sequence: u64,
}

#[derive(Serialize)]
struct PublicWalletRequest<'a> {
    session_id: &'a str,
    key_id: &'a str,
    public_key: &'a str,
    owner: &'a str,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct PublicWalletBody {
    node_id: String,
    key_id: String,
    public_key: String,
    owner: String,
    wallet_key_id: String,
    wallet_public_key: String,
    wallet_epoch: u64,
    address: String,
    audit_sequence: u64,
}

#[derive(Deserialize)]
struct SignResponseBody {
    node_id: String,
    key_id: String,
    kind: String,
    signed_bytes: String,
    signature: String,
    recovery_id: Option<u8>,
    audit_sequence: u64,
}

#[derive(Deserialize)]
struct HealthBody {
    node_id: String,
    ready: bool,
    audit_sequence: u64,
    reachable_peers: u32,
}

#[derive(Deserialize)]
struct ErrorEnvelope {
    error: ErrorBody,
}

#[derive(Deserialize)]
struct ErrorBody {
    category: String,
    code: String,
    policy_code: Option<String>,
}

fn read_body(
    node: &str,
    mut response: ureq::http::Response<ureq::Body>,
) -> Result<(u16, Vec<u8>), AttestorError> {
    let status = response.status().as_u16();
    let body = response
        .body_mut()
        .with_config()
        .limit(MAX_RESPONSE_BYTES)
        .read_to_vec()
        .map_err(|error| transport_error(node, &error))?;
    Ok((status, body))
}

fn refusal(node: &str, status: u16, body: &[u8]) -> AttestorError {
    match serde_json::from_slice::<ErrorEnvelope>(body) {
        Ok(envelope) if !envelope.error.code.is_empty() => AttestorError::Refused {
            node: node.to_owned(),
            status,
            category: envelope.error.category,
            code: envelope.error.code,
            policy_code: envelope.error.policy_code.filter(|code| !code.is_empty()),
        },
        _ => malformed(node),
    }
}

fn transport_error(node: &str, error: &ureq::Error) -> AttestorError {
    let node = node.to_owned();
    match error {
        ureq::Error::Timeout(_) => AttestorError::Timeout { node },
        ureq::Error::Tls(_) | ureq::Error::Pem(_) | ureq::Error::Rustls(_) => {
            AttestorError::Authentication { node }
        }
        ureq::Error::Io(io)
            if io
                .get_ref()
                .is_some_and(<dyn std::error::Error + Send + Sync>::is::<rustls::Error>) =>
        {
            AttestorError::Authentication { node }
        }
        ureq::Error::BodyExceedsLimit(_) | ureq::Error::Protocol(_) => {
            AttestorError::MalformedResponse { node }
        }
        _ => AttestorError::Unavailable { node },
    }
}

fn malformed(node: &str) -> AttestorError {
    AttestorError::MalformedResponse {
        node: node.to_owned(),
    }
}

fn pkcs8_pem(der: &[u8]) -> Zeroizing<String> {
    use base64::Engine as _;
    let encoded = Zeroizing::new(base64::engine::general_purpose::STANDARD.encode(der));
    let mut pem = Zeroizing::new(String::from("-----BEGIN PRIVATE KEY-----\n"));
    for line in encoded.as_bytes().chunks(64) {
        pem.push_str(&String::from_utf8_lossy(line));
        pem.push('\n');
    }
    pem.push_str("-----END PRIVATE KEY-----\n");
    pem
}

fn valid_identifier(value: &str) -> bool {
    !value.is_empty() && value.len() <= MAX_IDENTIFIER && !value.contains('/')
}

fn session_id(label: &str) -> Result<String, AttestorError> {
    let mut nonce = [0_u8; 16];
    getrandom::fill(&mut nonce).map_err(|_| AttestorError::Configuration("session randomness"))?;
    Ok(format!("{label}-{}", encode_hex(&nonce)))
}

fn encode_hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push(char::from(DIGITS[usize::from(byte >> 4)]));
        out.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    out
}

fn decode_fixed<const N: usize>(text: &str) -> Option<[u8; N]> {
    let raw = text.as_bytes();
    if raw.len() != N * 2 {
        return None;
    }
    let mut out = [0_u8; N];
    for (slot, pair) in out.iter_mut().zip(raw.chunks_exact(2)) {
        *slot = (nibble(pair[0])? << 4) | nibble(pair[1])?;
    }
    Some(out)
}

const fn nibble(digit: u8) -> Option<u8> {
    match digit {
        b'0'..=b'9' => Some(digit - b'0'),
        b'a'..=b'f' => Some(digit - b'a' + 10),
        b'A'..=b'F' => Some(digit - b'A' + 10),
        _ => None,
    }
}

#[derive(Serialize)]
struct NativeSignBody<'a> {
    profile: u8,
    session_id: &'a str,
    key_id: &'a str,
    operation: &'a str,
    signers: &'a [String],
    #[serde(skip_serializing_if = "Option::is_none")]
    grant: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    purpose: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    capability: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    native_session: Option<&'a str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    expiry_ms: Option<&'a str>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeSignResponse {
    profile: u8,
    node_id: String,
    key_id: String,
    operation: String,
    signed_bytes: String,
    signature: String,
    audit_sequence: u64,
}

fn native_invalid() -> AttestorError {
    AttestorError::Configuration("native consent binding")
}

pub struct NativeGrantCoordinates {
    pub tenant: String,
    pub agent: String,
    pub session_id: [u8; 32],
    pub generation: u64,
    pub capability_id: [u8; 32],
    pub created_at_ms: u64,
    pub expires_at_ms: u64,
}

struct NativeReader<'a> {
    bytes: &'a [u8],
    at: usize,
}
impl<'a> NativeReader<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], AttestorError> {
        let end = self.at.checked_add(length).ok_or_else(native_invalid)?;
        let value = self.bytes.get(self.at..end).ok_or_else(native_invalid)?;
        self.at = end;
        Ok(value)
    }
    fn array<const N: usize>(&mut self) -> Result<[u8; N], AttestorError> {
        self.take(N)?.try_into().map_err(|_| native_invalid())
    }
    fn u8(&mut self) -> Result<u8, AttestorError> {
        Ok(self.array::<1>()?[0])
    }
    fn u16(&mut self) -> Result<u16, AttestorError> {
        Ok(u16::from_be_bytes(self.array()?))
    }
    fn u32(&mut self) -> Result<u32, AttestorError> {
        Ok(u32::from_be_bytes(self.array()?))
    }
    fn u64(&mut self) -> Result<u64, AttestorError> {
        Ok(u64::from_be_bytes(self.array()?))
    }
    fn u128(&mut self) -> Result<u128, AttestorError> {
        Ok(u128::from_be_bytes(self.array()?))
    }
    fn text(&mut self) -> Result<String, AttestorError> {
        let length = usize::from(self.u16()?);
        String::from_utf8(self.take(length)?.to_vec()).map_err(|_| native_invalid())
    }
    fn end(&self) -> Result<(), AttestorError> {
        if self.at == self.bytes.len() {
            Ok(())
        } else {
            Err(native_invalid())
        }
    }
    fn ids(&mut self) -> Result<BTreeSet<[u8; 32]>, AttestorError> {
        let mut ids = BTreeSet::new();
        for _ in 0..self.u16()? {
            let id = self.array()?;
            if ids.last().is_some_and(|previous| *previous >= id) {
                return Err(native_invalid());
            }
            ids.insert(id);
        }
        Ok(ids)
    }
    fn activities(
        &mut self,
    ) -> Result<BTreeSet<layerx_agent_api::identity::NativeActivity>, AttestorError> {
        let mut values = BTreeSet::new();
        for _ in 0..self.u16()? {
            let value = layerx_agent_api::identity::NativeActivity::decode(self.take(5)?)
                .map_err(|_| native_invalid())?;
            if values.last().is_some_and(|previous| *previous >= value) {
                return Err(native_invalid());
            }
            values.insert(value);
        }
        Ok(values)
    }
}

pub fn native_local_grant_signing_bytes(
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
) -> Result<Vec<u8>, AttestorError> {
    validate_native_local_grant(grant)?;
    let mut bytes = b"LayerX/native/local-grant/v1\0".to_vec();
    bytes.extend_from_slice(&grant.expires_at_ms.to_be_bytes());
    bytes.extend_from_slice(&grant.owner_public_key);
    for record in [&grant.capability, &grant.session_scope] {
        let length = u32::try_from(record.len()).map_err(|_| native_invalid())?;
        bytes.extend_from_slice(&length.to_be_bytes());
        bytes.extend_from_slice(record);
    }
    Ok(bytes)
}

pub fn validate_native_local_grant(
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
) -> Result<NativeGrantCoordinates, AttestorError> {
    grant.validate().map_err(|_| native_invalid())?;
    if grant.signature != [0; 64] {
        return Err(native_invalid());
    }
    let mut outer = NativeReader {
        bytes: &grant.capability,
        at: 0,
    };
    if outer.take(4)? != b"LXNC" || outer.u8()? != 1 {
        return Err(native_invalid());
    }
    let length = usize::try_from(outer.u32()?).map_err(|_| native_invalid())?;
    let mut record = NativeReader {
        bytes: outer.take(length)?,
        at: 0,
    };
    if record.u8()? != 1 {
        return Err(native_invalid());
    }
    let capability_id = record.array::<32>()?;
    match record.u8()? {
        0 => (),
        1 => {
            if record.array::<32>()? == capability_id {
                return Err(native_invalid());
            }
        }
        _ => return Err(native_invalid()),
    }
    let tenant = record.text()?;
    layerx_agent_api::identity::TenantId::new(&tenant).map_err(|_| native_invalid())?;
    if tenant.len() > 255 || tenant.as_bytes().contains(&0) {
        return Err(native_invalid());
    }
    let agent = record.text()?;
    layerx_types::ids::Did::new(agent.as_bytes()).map_err(|_| native_invalid())?;
    if !(1..=3).contains(&record.u8()?) {
        return Err(native_invalid());
    }
    record.array::<32>()?;
    if record.u16()? != 0 {
        return Err(native_invalid());
    }
    record.ids()?;
    let assets = record.ids()?;
    let mut ceilings = BTreeMap::new();
    for _ in 0..record.u16()? {
        let asset = record.array::<32>()?;
        let amount = record.u128()?;
        if !assets.contains(&asset)
            || ceilings
                .last_key_value()
                .is_some_and(|(previous, _)| *previous >= asset)
        {
            return Err(native_invalid());
        }
        ceilings.insert(asset, amount);
    }
    let mut last_window = None;
    for _ in 0..record.u16()? {
        let window = record.u64()?;
        record.u64()?;
        if window == 0 || last_window.is_some_and(|last| last >= window) {
            return Err(native_invalid());
        }
        last_window = Some(window);
    }
    if record.u16()? != 0 {
        return Err(native_invalid());
    }
    let expiry_seconds = record.u64()?;
    let grant_not_after_ms = record.u64()?;
    let created_at_ms = record.u64()?;
    record.u64()?;
    if record.u8()? != 0
        || expiry_seconds == 0
        || grant_not_after_ms != grant.expires_at_ms
        || u128::from(expiry_seconds) * 1000 > u128::from(grant.expires_at_ms)
        || created_at_ms >= grant.expires_at_ms
    {
        return Err(native_invalid());
    }
    record.end()?;
    let activities = outer.activities()?;
    outer.ids()?;
    let mut previous_spend = None;
    for _ in 0..outer.u16()? {
        let source = match outer.u8()? {
            0 => (0_u8, [0; 32], Vec::new(), [0; 32]),
            1 => {
                let program = outer.array::<32>()?;
                let length = usize::from(outer.u16()?);
                let seed = outer.take(length)?.to_vec();
                let account = outer.array::<32>()?;
                (1, program, seed, account)
            }
            _ => return Err(native_invalid()),
        };
        let asset = outer.array::<32>()?;
        let amount = outer.u128()?;
        let key = (source, asset);
        if !assets.contains(&asset)
            || ceilings.get(&asset).is_none_or(|maximum| amount > *maximum)
            || previous_spend.as_ref().is_some_and(|last| last >= &key)
        {
            return Err(native_invalid());
        }
        previous_spend = Some(key);
    }
    outer.end()?;
    let mut session = NativeReader {
        bytes: &grant.session_scope,
        at: 0,
    };
    if session.take(6)? != b"LXNS01" || session.text()? != tenant || session.text()? != agent {
        return Err(native_invalid());
    }
    let session_id = session.array::<32>()?;
    let generation = session.u64()?;
    let permitted = session.activities()?;
    if session_id == [0; 32] || generation == 0 || !activities.is_subset(&permitted) {
        return Err(native_invalid());
    }
    session.end()?;
    Ok(NativeGrantCoordinates {
        tenant,
        agent,
        session_id,
        generation,
        capability_id,
        created_at_ms,
        expires_at_ms: grant.expires_at_ms,
    })
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeReplay {
    request_digest: [u8; 32],
    signature: Vec<u8>,
    digest: [u8; 32],
    audit: BTreeMap<String, u64>,
    retain_until: u64,
}

const NATIVE_JOURNAL_BYTES: usize = 8 * 1024 * 1024;
struct NativeJournal {
    path: PathBuf,
    _lock: std::fs::File,
}
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct NativeJournalData {
    key_id: String,
    public_key: [u8; 32],
    network: u32,
    signers: Vec<String>,
    records: BTreeMap<String, NativeReplay>,
}

fn assertion_expired() -> AttestorError {
    AttestorError::Refused {
        node: "owner-assertion".into(),
        status: 401,
        category: "authorization".into(),
        code: "assertion.expired".into(),
        policy_code: None,
    }
}

/// Owner identity and lifetime claims; attestors verify the assertion's signature.
pub struct AssertionClaims {
    pub subject: String,
    issued: u64,
    expires: u64,
}
impl AssertionClaims {
    pub fn retain_until(&self, consent_expires_at_ms: u64) -> u64 {
        (consent_expires_at_ms / 1000)
            .min(self.expires)
            .min(self.issued.saturating_add(3600))
    }
    pub fn check(&self, now: u64) -> Result<(), AttestorError> {
        if self.issued > now || self.expires <= now || now.saturating_sub(self.issued) >= 3600 {
            return Err(AttestorError::Refused {
                node: "owner-assertion".into(),
                status: 401,
                category: "authorization".into(),
                code: "assertion.expired".into(),
                policy_code: None,
            });
        }
        Ok(())
    }
}
/// Decodes the bounded owner claims used for refresh and replay retention.
pub fn assertion_claims(assertion: &str) -> Result<AssertionClaims, AttestorError> {
    use base64::Engine;
    let invalid = || AttestorError::Configuration("owner assertion claims");
    let mut parts = assertion.split('.');
    let header = parts.next().ok_or_else(invalid)?;
    let payload = parts.next().ok_or_else(invalid)?;
    let signature = parts.next().ok_or_else(invalid)?;
    if header.is_empty() || signature.is_empty() || parts.next().is_some() {
        return Err(invalid());
    }
    let bytes = Zeroizing::new(
        base64::engine::general_purpose::URL_SAFE_NO_PAD
            .decode(payload)
            .map_err(|_| invalid())?,
    );
    let value: serde_json::Value = serde_json::from_slice(&bytes).map_err(|_| invalid())?;
    let subject = value["sub"]
        .as_str()
        .filter(|subject| !subject.is_empty() && subject.len() <= 1024)
        .ok_or_else(invalid)?
        .to_owned();
    let issued = value["iat"].as_u64().ok_or_else(invalid)?;
    let expires = value["exp"].as_u64().ok_or_else(invalid)?;
    if expires <= issued {
        return Err(invalid());
    }
    Ok(AssertionClaims {
        subject,
        issued,
        expires,
    })
}
