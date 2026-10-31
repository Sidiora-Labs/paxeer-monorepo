use std::cell::Cell;
use std::time::{Duration, Instant};

use layerx_programs::{
    chain_evidence::commitment_trust_step, hex, AccountStateHead, DeploymentProof, ProgramHeadKind,
    ProgramId, ProgramStateBundle, ProtocolDeploymentVerifier, VerifiedChainHead,
    VerifiedDeploymentEvidence, VerifiedProgramBundle,
};
use serde_json::Value;

use crate::head_attestation::{chain_head_evidence, ChainHeadEvidence};

const ACCOUNT_ACTIVITY: u32 = 0x0009_0006;
const WIND_DOWN_ACTIVITY: u32 = 0x0009_0007;
const MAX_CHANGE_RECORDS: usize = 4_096;
const PROJECTION_STALE: i64 = -903;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum HeadAnswer {
    Head,
    Pending,
    Refused(u16),
}

fn classify_head_answer(status: u16, body: &Value) -> HeadAnswer {
    if (200..300).contains(&status) {
        return HeadAnswer::Head;
    }
    if status == 503
        && body.as_object().is_some_and(|fields| fields.len() == 1)
        && body["error"].as_i64() == Some(PROJECTION_STALE)
    {
        return HeadAnswer::Pending;
    }
    HeadAnswer::Refused(status)
}

#[derive(Clone, Copy, Debug, Default, Eq, Ord, PartialEq, PartialOrd)]
pub struct ProgramStateCursor {
    pub sequence: u64,
    pub ordinal: u32,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProgramStateNotice {
    pub cursor: ProgramStateCursor,
    pub program: ProgramId,
    pub activity_type: u32,
    pub event_type: u16,
    pub receipt_digest: [u8; 32],
}

pub struct ProgramStateRecord {
    pub program: ProgramId,
    pub bytes: Vec<u8>,
    pub receipt: AccountStateHead,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HeadAuthority {
    pub header_hash: [u8; 32],
}

#[derive(Clone)]
pub struct RegistryClientIdentity(ureq::tls::ClientCert);

impl std::fmt::Debug for RegistryClientIdentity {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("RegistryClientIdentity([REDACTED])")
    }
}

impl RegistryClientIdentity {
    /// # Errors
    /// Refuses malformed or undecryptable PKCS#12 and a missing or mismatched key pair.
    pub fn from_pkcs12(encoded: &[u8], password: &str) -> Result<Self, String> {
        if encoded.is_empty()
            || encoded.len() > 1024 * 1024
            || password.is_empty()
            || password.len() > 4_096
            || password.as_bytes().contains(&0)
        {
            return Err("registry client identity is outside its bound".to_owned());
        }
        let archive = openssl::pkcs12::Pkcs12::from_der(encoded)
            .and_then(|archive| archive.parse2(password))
            .map_err(|_| "registry client identity cannot be decoded or decrypted".to_owned())?;
        let key = archive
            .pkey
            .ok_or("registry client private key is absent")?;
        let certificate = archive
            .cert
            .ok_or("registry client certificate is absent")?;
        let public = certificate
            .public_key()
            .map_err(|_| "registry client certificate key is invalid".to_owned())?;
        if !public.public_eq(&key) {
            return Err("registry client certificate and private key differ".to_owned());
        }
        let leaf = certificate
            .to_der()
            .map_err(|_| "registry client certificate encoding is invalid".to_owned())?;
        let mut certificates = vec![ureq::tls::Certificate::from_der(&leaf).to_owned()];
        if let Some(chain) = archive.ca {
            for certificate in chain {
                let der = certificate
                    .to_der()
                    .map_err(|_| "registry client certificate chain is invalid".to_owned())?;
                certificates.push(ureq::tls::Certificate::from_der(&der).to_owned());
            }
        }
        let private_pem = zeroize::Zeroizing::new(
            key.private_key_to_pem_pkcs8()
                .map_err(|_| "registry client private key encoding is invalid".to_owned())?,
        );
        let private_key = ureq::tls::PrivateKey::from_pem(private_pem.as_slice())
            .map_err(|_| "registry client private key encoding is invalid".to_owned())?;
        Ok(Self(ureq::tls::ClientCert::new_with_certs(
            &certificates,
            private_key,
        )))
    }
}

pub struct NodeProgramStateSource {
    agent: ureq::Agent,
    endpoint: String,
    authorization: String,
    authority_endpoint: String,
    authority_authorization: String,
    authority_replica_id: [u8; 32],
    deployment_verifier: ProtocolDeploymentVerifier,
    request_deadline: Cell<Option<Instant>>,
}

impl NodeProgramStateSource {
    ///
    /// # Errors
    /// Refuses missing authority configuration and endpoints that are not distinct HTTPS or loopback URLs.
    pub fn connect(
        endpoint: &str,
        authorization: String,
        outbound_ca_der: &[u8],
        authority_endpoint: &str,
        authority_authorization: String,
        authority_replica_id: [u8; 32],
        deployment_verifier: ProtocolDeploymentVerifier,
    ) -> Result<Self, String> {
        Self::connect_with_identity(
            endpoint,
            authorization,
            outbound_ca_der,
            None,
            authority_endpoint,
            authority_authorization,
            authority_replica_id,
            deployment_verifier,
        )
    }

    /// # Errors
    /// Refuses invalid authority configuration while preserving CA and hostname verification.
    #[allow(clippy::too_many_arguments)]
    pub fn connect_with_identity(
        endpoint: &str,
        authorization: String,
        outbound_ca_der: &[u8],
        client_identity: Option<&RegistryClientIdentity>,
        authority_endpoint: &str,
        authority_authorization: String,
        authority_replica_id: [u8; 32],
        deployment_verifier: ProtocolDeploymentVerifier,
    ) -> Result<Self, String> {
        let endpoint = endpoint.trim_end_matches('/');
        let authority_endpoint = authority_endpoint.trim_end_matches('/');
        if authorization.is_empty()
            || authority_authorization.is_empty()
            || authority_replica_id == [0; 32]
        {
            return Err("node authorities and a configured verifier are required".to_owned());
        }
        if !(endpoint.starts_with("https://") || loopback_http(endpoint))
            || !(authority_endpoint.starts_with("https://") || loopback_http(authority_endpoint))
            || endpoint == authority_endpoint
        {
            return Err(
                "node state and independent receipt authority must be distinct HTTPS or loopback endpoints".to_owned(),
            );
        }
        if outbound_ca_der.is_empty() {
            return Err("an outbound CA certificate is required".to_owned());
        }
        let root = ureq::tls::Certificate::from_der(outbound_ca_der).to_owned();
        let tls = ureq::tls::TlsConfig::builder()
            .provider(ureq::tls::TlsProvider::Rustls)
            .root_certs(ureq::tls::RootCerts::new_with_certs(&[root]))
            .client_cert(client_identity.map(|identity| identity.0.clone()))
            .build();
        let config = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(30)))
            .http_status_as_error(false)
            .max_redirects(0)
            .tls_config(tls)
            .build();
        Ok(Self {
            agent: config.into(),
            endpoint: endpoint.to_owned(),
            authorization,
            authority_endpoint: authority_endpoint.to_owned(),
            authority_authorization,
            authority_replica_id,
            deployment_verifier,
            request_deadline: Cell::new(None),
        })
    }

    pub fn set_request_deadline(&self, deadline: Instant) {
        self.request_deadline.set(Some(deadline));
    }

    #[must_use]
    pub fn request_deadline(&self) -> Option<Instant> {
        self.request_deadline.get()
    }

    #[must_use]
    pub fn request_deadline_expired(&self) -> bool {
        self.request_deadline
            .get()
            .is_some_and(|deadline| Instant::now() >= deadline)
    }

    ///
    /// # Errors
    /// Returns the protocol verifier refusal for invalid historical deployment evidence.
    pub fn verify_stored_deployment(
        &self,
        proof: &DeploymentProof,
        now: u64,
    ) -> Result<VerifiedDeploymentEvidence, String> {
        let document = self.get("/v1/protocol/account-state/head")?;
        self.parse_head_authority(&document, Some(now))?;
        let evidence = chain_head_evidence(&document)?;
        let trust = self
            .deployment_verifier
            .chain_trust()
            .ok_or("chain trust is absent")?;
        let admission = commitment_trust_step(&proof.state.commitment)
            .map_err(|error| format!("deployment trust step: {error:?}"))?;
        let current = commitment_trust_step(&evidence.commitment)
            .map_err(|error| format!("current trust step: {error:?}"))?;
        let verified = trust
            .verify_head(&current.0, &current.1)
            .map_err(|error| format!("current chain head: {error:?}"))?;
        let history = if admission.0.height == current.0.height {
            vec![admission]
        } else {
            vec![admission, current]
        };
        self.deployment_verifier
            .verify_historical_deployment(proof, &verified, &history, now)
            .map_err(|error| format!("stored deployment evidence refused: {error}"))
    }

    ///
    /// # Errors
    /// Refuses unavailable node or replica responses and invalid, unverified or stale head evidence.
    pub fn current_head(&self, now_ms: u64) -> Result<AccountStateHead, String> {
        self.parse_head(&self.get("/v1/protocol/account-state/head")?, Some(now_ms))
    }

    /// Reads the current head together with its independently verified header hash.
    ///
    /// # Errors
    /// Refuses unavailable node or replica responses and invalid, unverified or stale head evidence.
    pub fn current_head_authority(
        &self,
        now_ms: u64,
    ) -> Result<(AccountStateHead, HeadAuthority), String> {
        self.parse_head_authority(&self.get("/v1/protocol/account-state/head")?, Some(now_ms))
    }

    /// Reads the current head, distinguishing a network that has not yet
    /// sequenced its first receipt from every other refusal.
    ///
    /// # Errors
    /// Refuses unavailable node responses other than the stale-projection
    /// answer and invalid, unverified or stale head evidence.
    pub fn current_head_or_pending(&self, now_ms: u64) -> Result<Option<AccountStateHead>, String> {
        let path = "/v1/protocol/account-state/head";
        let (status, body) = self.fetch_from(&self.endpoint, &self.authorization, path)?;
        match classify_head_answer(status, &body) {
            HeadAnswer::Head => self.parse_head(&body, Some(now_ms)).map(Some),
            HeadAnswer::Pending => Ok(None),
            HeadAnswer::Refused(status) => {
                Err(format!("node authority GET {path} returned HTTP {status}"))
            }
        }
    }

    ///
    /// # Errors
    /// Refuses unavailable or invalid evidence and a receipt digest different from the requested digest.
    pub fn receipt_head(&self, digest: [u8; 32]) -> Result<AccountStateHead, String> {
        let path = format!("/v1/receipts/{}/account-state", hex::encode(&digest));
        let head = self.parse_head(&self.get(&path)?, None)?;
        if head.receipt_digest != digest {
            return Err("node receipt lookup returned a different receipt digest".to_owned());
        }
        Ok(head)
    }

    ///
    /// # Errors
    /// Refuses unavailable or malformed records, digest mismatches and records not anchored at the current head.
    pub fn program_state(
        &self,
        program: ProgramId,
        current_head: AccountStateHead,
    ) -> Result<ProgramStateRecord, String> {
        let path = format!(
            "/v1/programs/{}/account-state?at={}",
            hex::encode(&program.bytes()),
            current_head.freshness.observed_sequence
        );
        let document = self.get(&path)?;
        let encoded = field(&document, "record_hex")?;
        let bytes = hex::decode(encoded)
            .map_err(|error| format!("node program-state record is not hexadecimal: {error}"))?;
        if bytes.is_empty() {
            return Err("node program-state record is empty".to_owned());
        }
        let record_digest = digest(&bytes);
        if hex::decode_digest(field(&document, "record_digest")?)
            .map_err(|error| format!("node program-state digest is invalid: {error}"))?
            != record_digest
        {
            return Err("node program-state record does not match its content digest".to_owned());
        }
        let receipt_digest = hex::decode_digest(field(&document, "receipt_digest")?)
            .map_err(|error| format!("node program-state receipt digest is invalid: {error}"))?;
        let receipt = self.receipt_head(receipt_digest)?;
        if receipt != current_head {
            return Err("node program-state record is not anchored at the current head".to_owned());
        }
        Ok(ProgramStateRecord {
            program,
            bytes,
            receipt,
        })
    }

    pub fn current_program_bundle(
        &self,
        program: ProgramId,
        current_head: AccountStateHead,
        authority: &HeadAuthority,
        now_ms: u64,
    ) -> Result<VerifiedProgramBundle, String> {
        let path = format!(
            "/v1/programs/{}/state-proof?at={}",
            hex::encode(&program.bytes()),
            current_head.freshness.observed_sequence
        );
        let document = self.get(&path)?;
        let bytes = hex::decode(field(&document, "record_hex")?)
            .map_err(|error| format!("program state proof encoding refused: {error}"))?;
        if hex::decode_digest(field(&document, "record_digest")?)
            .map_err(|error| error.to_string())?
            != digest(&bytes)
            || hex::decode_digest(field(&document, "receipt_digest")?)
                .map_err(|error| error.to_string())?
                != current_head.receipt_digest
            || hex::decode_digest(field(&document, "program_id")?)
                .map_err(|error| error.to_string())?
                != program.bytes()
        {
            return Err("program state proof document binding mismatch".to_owned());
        }
        let bundle = ProgramStateBundle::decode(&bytes)
            .map_err(|error| format!("program state bundle decoding refused: {error}"))?;
        let head_document = self.get("/v1/protocol/account-state/head")?;
        let (verified_head, verified_authority) =
            self.parse_head_authority(&head_document, Some(now_ms))?;
        if verified_head != current_head || verified_authority != *authority {
            return Err(
                "program proof differs from independently verified current head".to_owned(),
            );
        }
        let evidence = chain_head_evidence(&head_document)?;
        let chain = self.verify_chain_head(&evidence, now_ms)?;
        self.deployment_verifier
            .verify_current_program_bundle(&bundle, &chain, program, now_ms)
            .map_err(|error| format!("program/interface membership refused: {error}"))
    }

    pub fn program_state_profile2(
        &self,
        program: ProgramId,
        current_head: AccountStateHead,
    ) -> Result<ProgramStateRecord, String> {
        let path = format!(
            "/v1/programs/{}/account-state?at={}&profile=2",
            hex::encode(&program.bytes()),
            current_head.freshness.observed_sequence
        );
        let document = self.get(&path)?;
        let encoded = field(&document, "record_hex")?;
        let bytes = hex::decode(encoded)
            .map_err(|error| format!("node program-state record is not hexadecimal: {error}"))?;
        if bytes.is_empty() {
            return Err("node program-state record is empty".to_owned());
        }
        let record_digest = digest(&bytes);
        if hex::decode_digest(field(&document, "record_digest")?)
            .map_err(|error| format!("node program-state digest is invalid: {error}"))?
            != record_digest
        {
            return Err("node program-state record does not match its content digest".to_owned());
        }
        let receipt_digest = hex::decode_digest(field(&document, "receipt_digest")?)
            .map_err(|error| format!("node program-state receipt digest is invalid: {error}"))?;
        let receipt = self.receipt_head(receipt_digest)?;
        if receipt != current_head {
            return Err("node program-state record is not anchored at the current head".to_owned());
        }
        Ok(ProgramStateRecord {
            program,
            bytes,
            receipt,
        })
    }

    ///
    /// # Errors
    /// Refuses unavailable or malformed feeds, invalid cursors, record bounds and inconsistent receipt evidence.
    pub fn changes(
        &self,
        after: ProgramStateCursor,
    ) -> Result<(Vec<ProgramStateNotice>, ProgramStateCursor, u64, bool), String> {
        let path = format!(
            "/v1/programs/account-state/changes?after_sequence={}",
            after.sequence
        );
        if after.ordinal != 0 {
            return Err("durable scan cursors cannot carry an event ordinal".to_owned());
        }
        let document = self.get(&path)?;
        let complete = parse_cursor(&document["complete_through"])?;
        let caught_up = document["caught_up"]
            .as_bool()
            .ok_or_else(|| "node program-state change feed omitted caught_up".to_owned())?;
        let scanned_through_sequence =
            document["scanned_through_sequence"]
                .as_u64()
                .ok_or_else(|| {
                    "node program-state change feed omitted scanned_through_sequence".to_owned()
                })?;
        if scanned_through_sequence < complete.sequence {
            return Err("node change feed cursor is ahead of its canonical scan".to_owned());
        }
        let records = document["records"]
            .as_array()
            .ok_or_else(|| "node program-state change feed omitted records".to_owned())?;
        if records.len() > MAX_CHANGE_RECORDS {
            return Err("node program-state change feed exceeds its record bound".to_owned());
        }
        let mut notices = Vec::with_capacity(records.len());
        let mut prior_notice: Option<ProgramStateCursor> = None;
        for record in records {
            let cursor = parse_cursor(record)?;
            let program = ProgramId::new(
                hex::decode_digest(field(record, "program_id")?)
                    .map_err(|error| format!("change program id is invalid: {error}"))?,
            )
            .map_err(|error| format!("change program id is reserved: {error}"))?;
            let activity_type = u32::try_from(
                record["activity_type"]
                    .as_u64()
                    .ok_or_else(|| "change omitted activity_type".to_owned())?,
            )
            .map_err(|_| "change activity_type is out of range".to_owned())?;
            let event_type = u16::try_from(
                record["event_type"]
                    .as_u64()
                    .ok_or_else(|| "change omitted event_type".to_owned())?,
            )
            .map_err(|_| "change event_type is out of range".to_owned())?;
            let receipt_digest = hex::decode_digest(field(record, "receipt_digest")?)
                .map_err(|error| format!("change receipt digest is invalid: {error}"))?;
            let receipt = self.receipt_head(receipt_digest)?;
            let ordered = prior_notice.is_none_or(|prior| {
                if cursor.sequence == prior.sequence {
                    cursor.ordinal == prior.ordinal.saturating_add(1)
                } else {
                    cursor.sequence > prior.sequence && cursor.ordinal == 0
                }
            });
            if !ordered
                || cursor.sequence <= after.sequence
                || cursor.sequence > complete.sequence
                || receipt.freshness.observed_sequence != cursor.sequence
                || !matches!(activity_type, ACCOUNT_ACTIVITY | WIND_DOWN_ACTIVITY)
                || !(8..=12).contains(&event_type)
            {
                return Err("node program-state change feed is non-canonical".to_owned());
            }
            prior_notice = Some(cursor);
            notices.push(ProgramStateNotice {
                cursor,
                program,
                activity_type,
                event_type,
                receipt_digest,
            });
        }
        if complete.ordinal != 0 || complete.sequence < after.sequence {
            return Err("node program-state scan cursor is non-canonical".to_owned());
        }
        if caught_up != (complete.sequence == scanned_through_sequence) {
            return Err("node program-state caught_up disagrees with its scan head".to_owned());
        }
        Ok((notices, complete, scanned_through_sequence, caught_up))
    }

    fn get(&self, path: &str) -> Result<Value, String> {
        self.get_from(&self.endpoint, &self.authorization, path)
    }

    fn get_authority(&self, path: &str) -> Result<Value, String> {
        self.get_from(
            &self.authority_endpoint,
            &self.authority_authorization,
            path,
        )
    }

    fn get_from(&self, endpoint: &str, authorization: &str, path: &str) -> Result<Value, String> {
        let (status, body) = self.fetch_from(endpoint, authorization, path)?;
        if !(200..300).contains(&status) {
            if let Some(code) = body["error"].as_i64() {
                return Err(format!(
                    "node authority GET {path} returned HTTP {status} with protocol result {code}"
                ));
            }
            return Err(format!("node authority GET {path} returned HTTP {status}"));
        }
        Ok(body)
    }

    fn fetch_from(
        &self,
        endpoint: &str,
        authorization: &str,
        path: &str,
    ) -> Result<(u16, Value), String> {
        let remaining = self
            .request_deadline
            .get()
            .map_or(Some(Duration::from_secs(30)), |deadline| {
                deadline.checked_duration_since(Instant::now())
            })
            .filter(|remaining| !remaining.is_zero())
            .ok_or_else(|| {
                "registry request deadline expired before node authority access".to_owned()
            })?;
        let url = format!("{endpoint}{path}");
        let mut response = self
            .agent
            .get(&url)
            .header("Authorization", &format!("Bearer {authorization}"))
            .config()
            .timeout_global(Some(remaining))
            .build()
            .call()
            .map_err(|error| format!("node authority GET {path} failed: {error}"))?;
        let status = response.status();
        let body = response
            .body_mut()
            .read_to_string()
            .map_err(|error| format!("node authority GET {path} was unreadable: {error}"))?;
        if !status.is_success() {
            return Ok((
                status.as_u16(),
                serde_json::from_str(&body).unwrap_or(Value::Null),
            ));
        }
        serde_json::from_str(&body)
            .map(|value| (status.as_u16(), value))
            .map_err(|error| format!("node authority GET {path} returned invalid JSON: {error}"))
    }

    fn parse_head(&self, value: &Value, now_ms: Option<u64>) -> Result<AccountStateHead, String> {
        self.parse_head_authority(value, now_ms)
            .map(|(head, _)| head)
    }

    fn parse_head_authority(
        &self,
        value: &Value,
        now_ms: Option<u64>,
    ) -> Result<(AccountStateHead, HeadAuthority), String> {
        if now_ms.is_some() && value["current"].as_bool() != Some(true) {
            return Err("node account-state response is not the current head".to_owned());
        }
        let evidence = chain_head_evidence(value)?;
        let sequence = value["observed_sequence"]
            .as_u64()
            .ok_or("head omitted observed_sequence")?;
        let path = format!("/v1/protocol/account-state/head?height={sequence}");
        let independent = self.get_authority(&path)?;
        if evidence != chain_head_evidence(&independent)?
            || hex::decode_digest(field(&independent, "authority_replica_id")?)
                .map_err(|error| error.to_string())?
                != self.authority_replica_id
        {
            return Err("node chain evidence disagrees with independent authority".to_owned());
        }
        let trust = self
            .deployment_verifier
            .chain_trust()
            .ok_or("chain trust is absent")?;
        let step = commitment_trust_step(&evidence.commitment)
            .map_err(|error| format!("chain trust step: {error:?}"))?;
        let header = trust
            .verify_head(&step.0, &step.1)
            .map_err(|error| format!("chain head verification failed: {error:?}"))?;
        let head = match now_ms {
            Some(now) => {
                let chain = self.verify_chain_head(&evidence, now)?;
                AccountStateHead {
                    receipt_digest: chain.receipt_digest(),
                    state_root: chain.state_root(),
                    freshness: chain.freshness(),
                }
            }
            None => {
                let digest = layerx_programs::verify_tx(
                    &header,
                    &evidence.transaction,
                    &evidence.transaction_proof,
                )
                .map_err(|error| format!("historical transaction proof refused: {error:?}"))?;
                let time =
                    u64::try_from(header.time_ms()).map_err(|_| "invalid historical head time")?;
                let freshness =
                    layerx_programs::chain_evidence::freshness::chain_freshness(&header, time, 0)
                        .map_err(|error| error.to_string())?;
                AccountStateHead {
                    receipt_digest: digest,
                    state_root: header.app_hash(),
                    freshness,
                }
            }
        };
        if hex::decode_digest(field(value, "receipt_digest")?).map_err(|error| error.to_string())?
            != head.receipt_digest
            || hex::decode_digest(field(value, "state_root")?).map_err(|error| error.to_string())?
                != head.state_root
            || sequence != head.freshness.observed_sequence
            || value["observed_at"].as_u64() != Some(head.freshness.observed_at)
        {
            return Err("node head claims disagree with verified chain evidence".to_owned());
        }
        Ok((
            head,
            HeadAuthority {
                header_hash: header.header_hash(),
            },
        ))
    }

    fn verify_chain_head(
        &self,
        evidence: &ChainHeadEvidence,
        now: u64,
    ) -> Result<VerifiedChainHead, String> {
        self.deployment_verifier
            .verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &evidence.commitment,
                &evidence.transaction,
                &evidence.transaction_proof,
                now,
            )
            .map_err(|error| format!("chain head evidence refused: {error}"))
    }
}

fn parse_cursor(value: &Value) -> Result<ProgramStateCursor, String> {
    let sequence = value["sequence"]
        .as_u64()
        .ok_or_else(|| "program-state cursor omitted sequence".to_owned())?;
    let ordinal = u32::try_from(
        value["ordinal"]
            .as_u64()
            .ok_or_else(|| "program-state cursor omitted ordinal".to_owned())?,
    )
    .map_err(|_| "program-state cursor ordinal is out of range".to_owned())?;
    Ok(ProgramStateCursor { sequence, ordinal })
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, String> {
    value[name]
        .as_str()
        .ok_or_else(|| format!("node response omitted {name}"))
}

fn digest(bytes: &[u8]) -> [u8; 32] {
    use sha2::{Digest as _, Sha256};
    Sha256::digest(bytes).into()
}

fn loopback_http(endpoint: &str) -> bool {
    endpoint
        .strip_prefix("http://")
        .and_then(|authority| authority.split('/').next())
        .is_some_and(|host| {
            host == "localhost"
                || host.starts_with("localhost:")
                || host == "127.0.0.1"
                || host.starts_with("127.0.0.1:")
                || host == "[::1]"
                || host.starts_with("[::1]:")
        })
}

#[cfg(test)]
mod tests {
    use super::{classify_head_answer, HeadAnswer};
    use serde_json::json;

    #[test]
    fn outbound_client_identity_decrypts_real_pkcs12_and_refuses_invalid_material(
    ) -> Result<(), Box<dyn std::error::Error>> {
        use openssl::asn1::{Asn1Integer, Asn1Time};
        use openssl::bn::BigNum;
        use openssl::hash::MessageDigest;
        use openssl::pkcs12::Pkcs12;
        use openssl::pkey::PKey;
        use openssl::rsa::Rsa;
        use openssl::x509::{X509NameBuilder, X509};

        let key = PKey::from_rsa(Rsa::generate(2048)?)?;
        let mut name = X509NameBuilder::new()?;
        name.append_entry_by_text("CN", "registry-client")?;
        let name = name.build();
        let mut certificate = X509::builder()?;
        certificate.set_version(2)?;
        let serial: Asn1Integer = BigNum::from_u32(1)?.to_asn1_integer()?;
        certificate.set_serial_number(&serial)?;
        certificate.set_subject_name(&name)?;
        certificate.set_issuer_name(&name)?;
        certificate.set_pubkey(&key)?;
        let start = Asn1Time::days_from_now(0)?;
        let end = Asn1Time::days_from_now(1)?;
        certificate.set_not_before(&start)?;
        certificate.set_not_after(&end)?;
        certificate.sign(&key, MessageDigest::sha256())?;
        let certificate = certificate.build();
        let mut builder = Pkcs12::builder();
        builder
            .name("registry-client")
            .pkey(&key)
            .cert(&certificate);
        let archive = builder.build2("test-archive-password")?.to_der()?;
        let identity =
            super::RegistryClientIdentity::from_pkcs12(&archive, "test-archive-password")
                .map_err(std::io::Error::other)?;
        assert_eq!(identity.0.certs()[0].der(), certificate.to_der()?);
        assert_eq!(format!("{:?}", identity.0.private_key().kind()), "Pkcs8");
        assert_eq!(
            format!("{identity:?}"),
            "RegistryClientIdentity([REDACTED])"
        );
        assert!(super::RegistryClientIdentity::from_pkcs12(&archive, "wrong-password").is_err());
        assert!(super::RegistryClientIdentity::from_pkcs12(&archive, "").is_err());
        assert!(super::RegistryClientIdentity::from_pkcs12(&archive, "invalid\0password").is_err());
        assert!(
            super::RegistryClientIdentity::from_pkcs12(b"invalid archive", "password").is_err()
        );
        let mut missing_key = Pkcs12::builder();
        missing_key.cert(&certificate);
        let certificate_only = missing_key.build2("test-archive-password")?.to_der()?;
        assert!(super::RegistryClientIdentity::from_pkcs12(
            &certificate_only,
            "test-archive-password"
        )
        .is_err());
        Ok(())
    }

    #[test]
    fn only_the_stale_projection_answer_is_a_pending_head() {
        assert_eq!(
            classify_head_answer(503, &json!({"error": -903})),
            HeadAnswer::Pending
        );
        assert_eq!(
            classify_head_answer(200, &json!({"current": true})),
            HeadAnswer::Head
        );
        for (status, body) in [
            (503, json!({"error": -903, "detail": "x"})),
            (503, json!({"error": -902})),
            (503, json!({"error": "node_unavailable"})),
            (503, json!(null)),
            (500, json!({"error": -903})),
            (404, json!({"error": -903})),
        ] {
            assert_eq!(
                classify_head_answer(status, &body),
                HeadAnswer::Refused(status)
            );
        }
    }
}
