//! Agent read route backed by the authenticated layerxd Programs service.

use std::time::Duration;

use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, CommitmentProof},
    merkle::SimpleProof,
};
use layerx_programs::state_record::{
    ProtocolAdapterError as StateRecordError, ProtocolProgramStateRead,
};
use layerx_programs::{
    hex, AccountStateHead, ProgramBundleError, ProgramHeadKind, ProgramId, ProgramLifecycle,
    ProgramStateBundle, ProtocolDeploymentVerifier, ProtocolEvidenceError, Registry,
    VerifiedChainHead, VerifiedProgramBundle,
};
use serde_json::Value;

use super::program_balances_impl::{
    CommittedProgramBalanceFreshness, ProgramBalanceRead, ProgramValueBalance,
};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramReadError {
    CoreRefused(i32),
    NonCanonicalView,
    AccountState(layerx_programs::AccountStateError),
    Registry(layerx_programs::RegistryError),
    CorruptRecord,
}

impl From<StateRecordError> for ProgramReadError {
    fn from(error: StateRecordError) -> Self {
        match error {
            StateRecordError::CoreRefused(code) => Self::CoreRefused(code),
            StateRecordError::NonCanonicalView => Self::NonCanonicalView,
            StateRecordError::AccountState(error) => Self::AccountState(error),
            StateRecordError::Registry(error) => Self::Registry(error),
            StateRecordError::CorruptRecord => Self::CorruptRecord,
        }
    }
}

impl From<layerx_programs::RegistryError> for ProgramReadError {
    fn from(error: layerx_programs::RegistryError) -> Self {
        Self::Registry(error)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ChainHeadEvidence {
    commitment: CommitmentProof,
    transaction: Vec<u8>,
    transaction_proof: SimpleProof,
}

/// Refusal of the authenticated chain-head read.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ChainHeadError {
    /// Transport, node/authority disagreement, or non-canonical head document.
    View(ProgramReadError),
    /// Protected-trust refusal of the head as kind-5 coordinates.
    Evidence(ProtocolEvidenceError),
}

impl ChainHeadError {
    /// True only for the protected verifier's freshness/superseded-head refusal.
    #[must_use]
    pub const fn is_stale(&self) -> bool {
        matches!(self, Self::Evidence(ProtocolEvidenceError::Stale))
    }
}

/// Production agent reader connected to layerxd and an independent layerxd
/// receipt-authority replica. Neither endpoint is allowed to be agent-local.
pub struct LayerxdProgramBalanceReader {
    sequencer: SequencerEndpoint,
    agent: ureq::Agent,
    endpoint: String,
    authorization: String,
    authority_endpoint: String,
    authority_authorization: String,
    authority_replica_id: [u8; 32],
    verifier: ProtocolDeploymentVerifier,
    protected_verifier: ProtocolDeploymentVerifier,
    registry: Registry,
    staleness_limit: u64,
}

/// Explicit independent authority identity and outbound trust for both read endpoints.
pub struct ProgramAuthority<'a> {
    pub endpoint: &'a str,
    pub authorization: String,
    pub replica_id: [u8; 32],
    pub ca_der: &'a [u8],
}

pub(crate) struct ProgramArtifacts {
    pub(crate) terminal_payload: Vec<u8>,
    pub(crate) call_graph: Vec<u8>,
}

pub(crate) enum ProgramArtifactsError {
    Unavailable,
    Malformed,
}

impl ProgramArtifactsError {
    pub(crate) fn is_unavailable(&self) -> bool {
        matches!(self, Self::Unavailable)
    }
}

#[derive(serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct ProgramArtifactsWire {
    activity_id: String,
    receipt_digest: String,
    terminal_payload: String,
    call_graph: String,
}

impl LayerxdProgramBalanceReader {
    pub(crate) fn read_program_artifacts(
        &self,
        activity_id: [u8; 32],
        receipt_digest: [u8; 32],
    ) -> Result<ProgramArtifacts, ProgramArtifactsError> {
        let url = format!(
            "{}/v1/programs/activities/{}/artifacts?receipt_digest={}",
            self.endpoint,
            hex::encode(&activity_id),
            hex::encode(&receipt_digest),
        );
        let mut response = self
            .agent
            .get(&url)
            .header("Authorization", &format!("Bearer {}", self.authorization))
            .call()
            .map_err(|_| ProgramArtifactsError::Unavailable)?;
        if !response.status().is_success() {
            return Err(ProgramArtifactsError::Unavailable);
        }
        let body = response
            .body_mut()
            .with_config()
            .limit(4_194_560)
            .read_to_string()
            .map_err(|_| ProgramArtifactsError::Malformed)?;
        let wire: ProgramArtifactsWire =
            serde_json::from_str(&body).map_err(|_| ProgramArtifactsError::Malformed)?;
        if wire.activity_id != hex::encode(&activity_id)
            || wire.receipt_digest != hex::encode(&receipt_digest)
        {
            return Err(ProgramArtifactsError::Malformed);
        }
        Ok(ProgramArtifacts {
            terminal_payload: program_artifact_hex(&wire.terminal_payload)?,
            call_graph: program_artifact_hex(&wire.call_graph)?,
        })
    }

    /// Connects the running agent route to the production node pair.
    ///
    /// # Errors
    ///
    /// Refuses empty authorizations, a zero replica id, identical or insecure
    /// endpoints as a non-canonical view.
    pub fn connect(
        endpoint: &str,
        authorization: String,
        authority: ProgramAuthority<'_>,
        verifier: ProtocolDeploymentVerifier,
        registry: Registry,
    ) -> Result<Self, ProgramReadError> {
        let ProgramAuthority {
            endpoint: authority_endpoint,
            authorization: authority_authorization,
            replica_id: authority_replica_id,
            ca_der,
        } = authority;
        let endpoint = endpoint.trim_end_matches('/');
        let authority_endpoint = authority_endpoint.trim_end_matches('/');
        if authorization.is_empty()
            || authority_authorization.is_empty()
            || authority_replica_id == [0; 32]
            || endpoint == authority_endpoint
            || !secure_endpoint(endpoint)
            || !secure_endpoint(authority_endpoint)
        {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let tls =
            crate::outbound_tls::private_ca(ca_der).ok_or(ProgramReadError::NonCanonicalView)?;
        let config = ureq::Agent::config_builder()
            .tls_config(tls)
            .timeout_global(Some(Duration::from_secs(30)))
            .http_status_as_error(false)
            .build();
        let sequencer = SequencerEndpoint::new(endpoint, &[ca_der])
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        Ok(Self {
            sequencer,
            agent: config.into(),
            endpoint: endpoint.to_owned(),
            authorization,
            authority_endpoint: authority_endpoint.to_owned(),
            authority_authorization,
            authority_replica_id,
            staleness_limit: verifier.staleness_limit_ms(),
            protected_verifier: verifier.clone(),
            verifier,
            registry,
        })
    }

    /// # Errors
    /// Refuses a projection conflicting with immutable protected policy or revocations.
    pub fn refresh_authority(
        &mut self,
        history: &layerx_proof::signed_authority::SignedAuthorityHistory,
    ) -> Result<(), ProgramReadError> {
        self.verifier = self
            .protected_verifier
            .with_signed_history(history)
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        Ok(())
    }

    /// Reads and locally re-verifies one complete current protocol state.
    ///
    /// # Errors
    ///
    /// Refuses a zero clock as a non-canonical view and surfaces transport,
    /// decode, and verification refusals from the node pair unchanged.
    pub fn read_protocol_state(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        self.read_protocol_state_selected(program, now, false)
    }

    pub fn read_protocol_state_profile2(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        self.read_protocol_state_selected(program, now, true)
    }

    fn read_protocol_state_selected(
        &mut self,
        program: ProgramId,
        now: u64,
        request_profile2: bool,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        if now == 0 {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let head_document = self.get(
            &self.endpoint,
            &self.authorization,
            "/v1/protocol/account-state/head",
        )?;
        let chain = self
            .verify_head(&head_document, now)
            .map_err(|error| match error {
                ChainHeadError::View(error) => error,
                ChainHeadError::Evidence(_) => ProgramReadError::NonCanonicalView,
            })?;
        let head = AccountStateHead {
            receipt_digest: chain.receipt_digest(),
            state_root: chain.state_root(),
            freshness: chain.freshness(),
        };
        let profile2 = request_profile2
            || self
                .registry
                .entry_for_wind_down(program)?
                .versions
                .last()
                .is_some_and(|version| matches!(version.abi_version, 3 | 4));
        let path = format!(
            "/v1/programs/{}/account-state?at={}{}",
            hex::encode(&program.bytes()),
            head.freshness.observed_sequence,
            if profile2 { "&profile=2" } else { "" },
        );
        let document = self.get(&self.endpoint, &self.authorization, &path)?;
        let bytes = hex::decode(field(&document, "record_hex")?)
            .map_err(|_| ProgramReadError::CorruptRecord)?;
        let declared = hex::decode_digest(field(&document, "record_digest")?)
            .map_err(|_| ProgramReadError::CorruptRecord)?;
        if digest(&bytes) != declared
            || hex::decode_digest(field(&document, "receipt_digest")?)
                .map_err(|_| ProgramReadError::CorruptRecord)?
                != head.receipt_digest
        {
            return Err(ProgramReadError::CorruptRecord);
        }
        if profile2 {
            ProtocolProgramStateRead::restore_verified_profile2(
                &bytes,
                &mut self.registry,
                head,
                head,
                now,
                self.staleness_limit,
            )
            .map_err(ProgramReadError::from)
        } else {
            ProtocolProgramStateRead::restore_verified(
                &bytes,
                &mut self.registry,
                head,
                head,
                now,
                self.staleness_limit,
            )
            .map_err(ProgramReadError::from)
        }
    }

    pub fn read_verified_balances(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<layerx_programs::VerifiedProgramBalanceRead, ProgramReadError> {
        self.read_protocol_state(program, now)
            .map(ProtocolProgramStateRead::into_balances)
    }

    pub(crate) fn value_account_ids(
        &self,
        program: ProgramId,
    ) -> Result<Vec<[u8; 32]>, ProgramReadError> {
        self.registry
            .value_account_bindings(program)
            .map(|bindings| bindings.iter().map(|binding| binding.account_id).collect())
            .map_err(ProgramReadError::AccountState)
    }

    /// Reads registered program accounts at the requested committed height.
    ///
    /// # Errors
    /// Refuses unknown programs and every block, transport or ABI refusal.
    pub fn read(
        &mut self,
        program: ProgramId,
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, ProgramReadError> {
        let accounts: Vec<_> = self
            .registry
            .value_account_bindings(program)
            .map_err(|_| ProgramReadError::NonCanonicalView)?
            .iter()
            .map(|binding| binding.account_id)
            .collect();
        Self::read_at(&self.sequencer, program, &accounts, height)
            .map_err(|_| ProgramReadError::NonCanonicalView)
    }

    /// Reads an explicit set of accounts, validating their on-chain identities.
    ///
    /// # Errors
    /// Propagates RPC refusals and rejects absent or changing committed blocks,
    /// duplicate accounts, foreign accounts and non-canonical ABI answers.
    pub fn read_at(
        endpoint: &SequencerEndpoint,
        program: ProgramId,
        accounts: &[[u8; 32]],
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, SequencerError> {
        read_committed_program_balances(endpoint, program, accounts, height)
    }

    /// Reads the current chain head through the same independent
    /// receipt-authority cross-check as the balances path and returns it as the
    /// opaque authenticated kind-5 coordinate head.
    ///
    /// # Errors
    ///
    /// Refuses a zero clock, transport, node/authority and coordinate
    /// disagreement as a non-canonical view, and surfaces protected-trust
    /// refusals of the head (including a superseded ordinary head) unchanged.
    pub fn read_chain_head(&self, now: u64) -> Result<VerifiedChainHead, ChainHeadError> {
        if now == 0 {
            return Err(ChainHeadError::View(ProgramReadError::NonCanonicalView));
        }
        let head_document = self
            .get(
                &self.endpoint,
                &self.authorization,
                "/v1/protocol/account-state/head",
            )
            .map_err(ChainHeadError::View)?;
        self.verify_head(&head_document, now)
    }

    /// Decodes one kind-5 Programs state answer and binds it to the one
    /// authenticated chain head under the refreshed protected trust.
    ///
    /// # Errors
    ///
    /// Refuses non-canonical bytes and every head, signer, program, state,
    /// lifecycle, freshness and interface refusal of the bundle verifier.
    pub fn verify_program_bundle(
        &self,
        payload: &[u8],
        chain: &VerifiedChainHead,
        program: ProgramId,
        now: u64,
    ) -> Result<VerifiedProgramBundle, ProgramBundleError> {
        let bundle = ProgramStateBundle::decode(payload)?;
        self.verifier
            .verify_current_program_bundle(&bundle, chain, program, now)
    }

    #[must_use]
    pub const fn staleness_limit(&self) -> u64 {
        self.staleness_limit
    }

    fn verify_head(&self, value: &Value, now_ms: u64) -> Result<VerifiedChainHead, ChainHeadError> {
        let view = ChainHeadError::View;
        if value["current"].as_bool() != Some(true) {
            return Err(view(ProgramReadError::NonCanonicalView));
        }
        let evidence = chain_head_evidence(value).map_err(view)?;
        let observed_sequence = value["observed_sequence"]
            .as_u64()
            .ok_or(view(ProgramReadError::NonCanonicalView))?;
        let path = format!("/v1/protocol/account-state/head?height={observed_sequence}");
        let authority_document = self
            .get(
                &self.authority_endpoint,
                &self.authority_authorization,
                &path,
            )
            .map_err(view)?;
        if hex::decode_digest(field(&authority_document, "authority_replica_id").map_err(view)?)
            .map_err(|_| view(ProgramReadError::NonCanonicalView))?
            != self.authority_replica_id
            || evidence != chain_head_evidence(&authority_document).map_err(view)?
        {
            return Err(view(ProgramReadError::NonCanonicalView));
        }
        let chain = self
            .verifier
            .verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &evidence.commitment,
                &evidence.transaction,
                &evidence.transaction_proof,
                now_ms,
            )
            .map_err(ChainHeadError::Evidence)?;
        if hex::decode_digest(field(value, "receipt_digest").map_err(view)?)
            .map_err(|_| view(ProgramReadError::NonCanonicalView))?
            != chain.receipt_digest()
            || hex::decode_digest(field(value, "state_root").map_err(view)?)
                .map_err(|_| view(ProgramReadError::NonCanonicalView))?
                != chain.state_root()
            || observed_sequence != chain.freshness().observed_sequence
            || value["observed_at"].as_u64() != Some(chain.freshness().observed_at)
        {
            return Err(view(ProgramReadError::NonCanonicalView));
        }
        Ok(chain)
    }

    fn get(
        &self,
        endpoint: &str,
        authorization: &str,
        path: &str,
    ) -> Result<Value, ProgramReadError> {
        let url = format!("{endpoint}{path}");
        let mut response = self
            .agent
            .get(&url)
            .header("Authorization", &format!("Bearer {authorization}"))
            .call()
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        if !response.status().is_success() {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let body = response
            .body_mut()
            .read_to_string()
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        serde_json::from_str(&body).map_err(|_| ProgramReadError::NonCanonicalView)
    }
}

fn program_artifact_hex(text: &str) -> Result<Vec<u8>, ProgramArtifactsError> {
    if text.len() > 2_097_152
        || text.len() % 2 != 0
        || !text
            .bytes()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramArtifactsError::Malformed);
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let pair = std::str::from_utf8(pair).map_err(|_| ProgramArtifactsError::Malformed)?;
            u8::from_str_radix(pair, 16).map_err(|_| ProgramArtifactsError::Malformed)
        })
        .collect()
}

/// Agent service route that always refreshes from layerxd at request time.
pub struct ProgramBalanceReadRoute {
    reader: LayerxdProgramBalanceReader,
}

impl ProgramBalanceReadRoute {
    #[must_use]
    pub const fn new(reader: LayerxdProgramBalanceReader) -> Self {
        Self { reader }
    }

    /// # Errors
    /// Propagates protected policy and authenticated-history refusals unchanged.
    pub fn refresh_authority(
        &mut self,
        history: &layerx_proof::signed_authority::SignedAuthorityHistory,
    ) -> Result<(), ProgramReadError> {
        self.reader.refresh_authority(history)
    }

    /// Serves one committed balance read through the connected reader.
    ///
    /// # Errors
    ///
    /// Propagates every reader refusal unchanged.
    pub fn read(
        &mut self,
        program: ProgramId,
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, ProgramReadError> {
        self.reader.read(program, height)
    }
}

fn chain_head_evidence(value: &Value) -> Result<ChainHeadEvidence, ProgramReadError> {
    let value = &value["chain_evidence"];
    let commitment = decode_commitment_proof(&chain_hex(field(value, "commitment_hex")?)?)
        .map_err(|_| ProgramReadError::NonCanonicalView)?;
    let transaction = chain_hex(field(value, "transaction_hex")?)?;
    let proof = &value["transaction_proof"];
    let total = proof["total"]
        .as_i64()
        .filter(|total| *total >= 0)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let index = proof["index"]
        .as_i64()
        .filter(|index| *index >= 0)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let leaf_hash = chain_hex(field(proof, "leaf_hash")?)?
        .try_into()
        .map_err(|_| ProgramReadError::NonCanonicalView)?;
    let aunts = proof["aunts"]
        .as_array()
        .filter(|aunts| aunts.len() <= 64)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let aunts = aunts
        .iter()
        .map(|aunt| {
            chain_hex(aunt.as_str().ok_or(ProgramReadError::NonCanonicalView)?)?
                .try_into()
                .map_err(|_| ProgramReadError::NonCanonicalView)
        })
        .collect::<Result<Vec<[u8; 32]>, _>>()?;
    Ok(ChainHeadEvidence {
        commitment,
        transaction,
        transaction_proof: SimpleProof {
            total,
            index,
            leaf_hash,
            aunts,
        },
    })
}

fn chain_hex(text: &str) -> Result<Vec<u8>, ProgramReadError> {
    if !text.len().is_multiple_of(2)
        || !text
            .bytes()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramReadError::NonCanonicalView);
    }
    hex::decode(text).map_err(|_| ProgramReadError::NonCanonicalView)
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, ProgramReadError> {
    value[name]
        .as_str()
        .ok_or(ProgramReadError::NonCanonicalView)
}

fn digest(bytes: &[u8]) -> [u8; 32] {
    use sha2::{Digest as _, Sha256};
    Sha256::digest(bytes).into()
}

fn secure_endpoint(endpoint: &str) -> bool {
    endpoint.starts_with("https://")
        || endpoint
            .strip_prefix("http://")
            .and_then(|value| value.split('/').next())
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
#[path = "program_node_tests.rs"]
mod tests;

/// Reads Programs ABI views at one committed block; never uses pending state.
///
/// # Errors
/// Propagates RPC failures and refuses malformed results, identity mismatches,
/// duplicate accounts, missing blocks and a changed block during the calls.
pub fn read_committed_program_balances(
    endpoint: &SequencerEndpoint,
    program: ProgramId,
    accounts: &[[u8; 32]],
    height: u64,
) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, SequencerError> {
    let malformed = SequencerError::Malformed;
    if height == 0
        || program.bytes() == [0; 32]
        || accounts.len() > layerx_programs::MAX_PROGRAM_VALUE_ACCOUNTS
        || accounts.iter().any(|account| *account == [0; 32])
        || accounts
            .iter()
            .collect::<std::collections::BTreeSet<_>>()
            .len()
            != accounts.len()
    {
        return Err(malformed);
    }
    let block = endpoint
        .block_by_number(height)?
        .ok_or(SequencerError::Malformed)?;
    if block.hash == [0; 32] {
        return Err(malformed);
    }
    let wind_down = programs_call(endpoint, height, "getWindDown(bytes32)", &[program.bytes()])?;
    if wind_down.len() != 192 {
        return Err(malformed);
    }
    let lifecycle = match abi_integer(&wind_down[..32], 1)? {
        1 => ProgramLifecycle::Active,
        2 => ProgramLifecycle::Deprecated,
        3 => ProgramLifecycle::Tombstoned,
        _ => return Err(malformed),
    };
    abi_integer(&wind_down[64..96], 8)?;
    abi_integer(&wind_down[96..128], 8)?;
    let count = abi_integer(&wind_down[128..160], 2)?;
    let live_count = abi_integer(&wind_down[160..192], 2)?;
    if live_count > count {
        return Err(malformed);
    }
    let mut values = Vec::with_capacity(accounts.len());
    for account in accounts {
        let identity = programs_call(
            endpoint,
            height,
            "getProgramAccountById(bytes32)",
            &[*account],
        )?;
        // (bytes32 programId, bytes32 assetId, bytes seed), with one canonical tail.
        if identity.len() < 128
            || identity.len() % 32 != 0
            || identity[..32] != program.bytes()
            || abi_integer(&identity[64..96], 8)? != 96
        {
            return Err(malformed);
        }
        let seed_len = usize::try_from(abi_integer(&identity[96..128], 8)?)
            .map_err(|_| SequencerError::Malformed)?;
        if seed_len > layerx_programs_runtime::MAX_PROGRAM_ACCOUNT_SEED_BYTES
            || identity.len() != 128 + seed_len.div_ceil(32) * 32
            || identity[128 + seed_len..].iter().any(|byte| *byte != 0)
        {
            return Err(malformed);
        }
        let derived = layerx_programs_runtime::derive_program_account(
            program,
            &identity[128..128 + seed_len],
        )
        .map_err(|_| SequencerError::Malformed)?;
        if !derived.matches(account) {
            return Err(malformed);
        }
        let asset = identity[32..64]
            .try_into()
            .map_err(|_| SequencerError::Malformed)?;
        let result = programs_call(
            endpoint,
            height,
            "getValueAccount(bytes32,bytes32)",
            &[program.bytes(), *account],
        )?;
        if result.len() != 64 {
            return Err(malformed);
        }
        let amount = abi_integer(&result[..32], 16)?;
        let exists = match abi_integer(&result[32..], 1)? {
            0 => false,
            1 => true,
            _ => return Err(malformed),
        };
        if !exists && amount != 0 {
            return Err(malformed);
        }
        values.push(ProgramValueBalance {
            account: *account,
            asset,
            amount,
            frozen: None,
        });
    }
    let confirmed = endpoint
        .block_by_number(height)?
        .ok_or(SequencerError::Malformed)?;
    if confirmed != block {
        return Err(malformed);
    }
    Ok(ProgramBalanceRead {
        program: program.bytes(),
        lifecycle,
        accounts: values,
        freshness: CommittedProgramBalanceFreshness {
            height: block.number,
            block_hash: block.hash,
            timestamp: block.timestamp,
        },
    })
}

fn programs_call(
    endpoint: &SequencerEndpoint,
    height: u64,
    signature: &str,
    words: &[[u8; 32]],
) -> Result<Vec<u8>, SequencerError> {
    let selector = layerx_crypto::evm_transaction::transaction_hash(signature.as_bytes());
    let mut data = selector[..4].to_vec();
    for word in words {
        data.extend_from_slice(word);
    }
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1d]);
    endpoint.call_at(height, &address, &data)
}

fn abi_integer(word: &[u8], width: usize) -> Result<u128, SequencerError> {
    if word.len() != 32 || word[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(SequencerError::Malformed);
    }
    let bytes = word[16..]
        .try_into()
        .map_err(|_| SequencerError::Malformed)?;
    Ok(u128::from_be_bytes(bytes))
}
