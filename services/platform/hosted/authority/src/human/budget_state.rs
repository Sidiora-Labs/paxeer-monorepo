#[path = "budget_state/native.rs"]
mod native;
#[path = "budget_state/record.rs"]
mod record;

use self::record::{number, Record};
use crate::{
    digest, hex, human_policy::PrincipalPolicy, json, refusal, AuthorityError, AuthorityResult,
    Response,
};
use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;

use layerx_client::rpc::account::{balance, ReadContext, Requested, Rung};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use serde_json::{json as value, Value};
use sha2::{Digest, Sha256};
use std::time::{SystemTime, UNIX_EPOCH};

pub trait Configuration {
    fn chain_rpc_url(&self) -> &str;
    fn protocol_network_id(&self) -> u32;
}

fn unavailable(code: &str) -> Response {
    refusal(503, code, Some(5))
}

const MAX_EXPORT_BYTES: usize = 1_048_576;
const MAX_EXPORT_MATERIAL_BYTES: usize = (MAX_EXPORT_BYTES - 16_384) / 2;
const PAY: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

pub struct Session {
    sequencer: SequencerEndpoint,
    rpc_url: String,
    pub finality: crate::committed::CommittedHeader,
    pub receipt_sequence: u64,
    sequence_proof: Value,
    pub committed_height: u64,
    pub checkpoint_id: [u8; 32],
    pub timestamp: u64,
    state_root: [u8; 32],
}

fn data(signature: &[u8], argument: Option<&[u8; 32]>) -> Vec<u8> {
    let mut bytes = keccak256(signature)[..4].to_vec();
    if let Some(argument) = argument {
        bytes.extend_from_slice(argument);
    }
    bytes
}

/// # Errors
/// Returns an error when the DID length exceeds the encoded length limit.
pub fn identity_key(did: &str) -> AuthorityResult<[u8; 32]> {
    let mut preimage = b"LXP/v1/did-id\0".to_vec();
    preimage.extend_from_slice(
        &u16::try_from(did.len())
            .map_err(|_| AuthorityError)?
            .to_be_bytes(),
    );
    preimage.extend_from_slice(did.as_bytes());
    Ok(digest(&preimage))
}

impl Session {
    fn budget_call(&self, signature: &[u8], id: &[u8; 32]) -> AuthorityResult<Vec<u8>> {
        self.sequencer
            .call_at(self.finality.state_height, &PAY, &data(signature, Some(id)))
            .map_err(|_| AuthorityError)
    }

    /// # Errors
    /// Returns an error when the committed chain state cannot be read or verified.
    pub fn open(config: &impl Configuration) -> AuthorityResult<Self> {
        let sequencer =
            SequencerEndpoint::new(config.chain_rpc_url(), &[]).map_err(|_| AuthorityError)?;
        let finality = crate::committed::CommittedHeader::open(config.chain_rpc_url())?;
        let block = sequencer
            .block_by_number(finality.height)
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        let sequence = native::read(config.chain_rpc_url(), &finality, &[2])?;
        let receipt_sequence = u64::from_be_bytes(
            sequence
                .value
                .as_slice()
                .try_into()
                .map_err(|_| AuthorityError)?,
        );
        if receipt_sequence == 0 || block.number != finality.height {
            return Err(AuthorityError);
        }
        Ok(Self {
            sequencer,
            rpc_url: config.chain_rpc_url().to_owned(),
            committed_height: finality.height,
            checkpoint_id: finality.block_hash,
            timestamp: block.timestamp.checked_mul(1000).ok_or(AuthorityError)?,
            state_root: finality.app_hash,
            finality,
            receipt_sequence,
            sequence_proof: sequence.document,
        })
    }

    fn account(
        &self,
        id: [u8; 32],
    ) -> AuthorityResult<(layerx_proof::state::CanonicalAccount, Value)> {
        let mut key = vec![6];
        key.extend_from_slice(&id);
        let proof = native::read(&self.rpc_url, &self.finality, &key)?;
        let account = native::account(id, &proof.value)?;
        Ok((account, proof.document))
    }

    /// # Errors
    /// Returns an error when the account or identity binding is invalid.
    pub fn identity(&self, did: &str, owner: [u8; 32]) -> AuthorityResult<(Value, Value)> {
        let (account, proof) = self.account(owner)?;
        let name = std::str::from_utf8(&account.name).map_err(|_| AuthorityError)?;
        if account.frozen
            || ![
                format!("agent:{did}:main"),
                format!("agent:{did}:asset:{}", hex::encode(&account.asset_id())),
            ]
            .iter()
            .any(|expected| expected == name)
        {
            return Err(AuthorityError);
        }
        let public = hex::decode32(did.strip_prefix("did:layerx:").ok_or(AuthorityError)?)
            .map_err(|_| AuthorityError)?;
        let mut addr = [0; 20];
        addr[18..].copy_from_slice(&[0x10, 0x04]);
        let call = data(b"getEvmAddrByLayerX(bytes32)", Some(&public));
        let answer = self
            .sequencer
            .call_at(self.finality.state_height, &addr, &call)
            .map_err(|_| AuthorityError)?;
        if answer.len() != 32 || answer[..12] != [0; 12] || answer[12..] == [0; 20] {
            return Err(AuthorityError);
        }
        let row = value!({"bound":true, "layerx_account":hex::encode(&owner), "evm_address":format!("0x{}",hex::encode(&answer[12..])),
            "kind":account.kind,"frozen":account.frozen,"asset_id":hex::encode(&account.asset_id())});
        let evidence = value!({"account":row,"native_account_proof":proof,
            "binding_call":{"to":hex::encode(&addr),"data":hex::encode(&call),"result":hex::encode(&answer),"height":self.finality.state_height},
            "receipt_sequence_proof":self.sequence_proof,"finality":self.finality.document()});
        Ok((row, evidence))
    }

    fn balance(&self, id: [u8; 32], asset: [u8; 32]) -> AuthorityResult<Value> {
        let (account, proof) = self.account(id)?;
        if account.frozen || account.asset_id() != asset {
            return Err(AuthorityError);
        }
        let amount = balance(
            &self.sequencer,
            id,
            asset,
            ReadContext {
                committed_height: self.finality.state_height,
                requested: Requested::new(Rung::Committed).map_err(|_| AuthorityError)?,
            },
        )
        .map_err(|_| AuthorityError)?;
        if amount.value() != account.balance() {
            return Err(AuthorityError);
        }
        Ok(
            value!({"account_id":hex::encode(&id),"asset_id":hex::encode(&asset),"amount":amount.value().to_string(),"proof":proof}),
        )
    }

    /// # Errors
    /// Returns an error when the committed checkpoint has changed.
    pub fn unchanged(&self) -> AuthorityResult<()> {
        let block = self
            .sequencer
            .block_by_number(self.committed_height)
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        if self.sequencer.block_number().map_err(|_| AuthorityError)? != self.committed_height
            || block.hash != self.checkpoint_id
            || self
                .sequencer
                .state_root(self.committed_height)
                .map_err(|_| AuthorityError)?
                != self.state_root
        {
            return Err(AuthorityError);
        }
        Ok(())
    }

    /// # Errors
    /// Returns an error when the clock is unavailable or the checkpoint is too old.
    pub fn fresh(&self, maximum_age_seconds: u64) -> AuthorityResult<()> {
        self.fresh_at(maximum_age_seconds, now_ms()?)
    }

    fn fresh_at(&self, maximum_age_seconds: u64, now_ms: u128) -> AuthorityResult<()> {
        if now_ms
            .checked_sub(u128::from(self.timestamp))
            .is_none_or(|age| age > u128::from(maximum_age_seconds) * 1000)
        {
            return Err(AuthorityError);
        }
        Ok(())
    }
}

fn accounts(
    session: &Session,
    record: &Record,
    policy: &PrincipalPolicy,
    owner_did: Option<&str>,
) -> AuthorityResult<([u8; 32], Value)> {
    if hex::encode(&record.owner) != policy.account_id {
        return Err(AuthorityError);
    }
    let (identity, mut evidence) = if let Some(did) = owner_did {
        let (_, evidence) = session.identity(did, record.owner)?;
        (identity_key(did)?, evidence)
    } else {
        let mut bound = Vec::new();
        for identity in policy.identities.iter().filter(|identity| !identity.frozen) {
            if let Ok((_, evidence)) = session.identity(&identity.did, record.owner) {
                bound.push((identity_key(&identity.did)?, evidence));
            }
        }
        if bound.len() != 1 {
            return Err(AuthorityError);
        }
        bound.pop().ok_or(AuthorityError)?
    };
    let mut balances = vec![
        session.balance(record.owner, record.asset)?,
        session.balance(record.account, record.asset)?,
    ];
    if let Some(source) = record.source {
        balances.push(session.balance(source, record.asset)?);
    }
    evidence["balances"] = value!(balances);
    Ok((identity, evidence))
}

fn now_ms() -> AuthorityResult<u128> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .map_err(|_| AuthorityError)
}

fn produce(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
) -> AuthorityResult<Response> {
    produce_at(config, p, id, export, owner_did, now_ms()?)
}

fn produce_at(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
    now_ms: u128,
) -> AuthorityResult<Response> {
    let session = Session::open(config)?;
    produce_session(config, p, id, export, owner_did, now_ms, &session)
}

fn produce_session(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
    now_ms: u128,
    session: &Session,
) -> AuthorityResult<Response> {
    let budget_call = session.budget_call(b"getBudget(bytes32)", &id)?;
    let record = Record::from_call(id, &budget_call)?;
    if hex::encode(&record.asset) != p.asset_id
        || record.revocation == 0
        || record.revocation > session.receipt_sequence
    {
        return Err(AuthorityError);
    }
    let remaining_call = session.budget_call(b"budgetRemaining(bytes32)", &id)?;
    if remaining_call.len() != 32 {
        return Err(AuthorityError);
    }
    let remaining = number(&remaining_call, 0, 16)?;
    let (identity, account_join) = accounts(session, &record, p, owner_did)?;
    session.fresh_at(p.maximum_age_seconds, now_ms)?;
    if p.maximum_age_sequences == 0 {
        return Err(AuthorityError);
    }
    session.unchanged()?;
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(session.checkpoint_id);
    let join_bytes = serde_json::to_vec(&account_join).map_err(|_| AuthorityError)?;
    let material_bytes = [&budget_call, &remaining_call, &join_bytes]
        .iter()
        .try_fold(0_usize, |total, bytes| {
            total.checked_add(bytes.len()).ok_or(AuthorityError)
        })?;
    if material_bytes > MAX_EXPORT_MATERIAL_BYTES {
        return Err(AuthorityError);
    }
    for bytes in [&budget_call, &remaining_call, &join_bytes] {
        evidence.update(
            u64::try_from(bytes.len())
                .map_err(|_| AuthorityError)?
                .to_be_bytes(),
        );
        evidence.update(bytes);
    }
    let evidence: [u8; 32] = evidence.finalize().into();
    let state = value!({
        "budget_id":hex::encode(&record.id), "asset":hex::encode(&record.asset),
        "revocation_sequence":record.revocation, "observed_head_sequence":session.receipt_sequence,
        "verification":layerx_types::verify::VerificationLevel::COMMITTED.wire_rank(), "evidence_digest":hex::encode(&evidence),
        "receipt_digest":hex::encode(&digest(&budget_call)), "checkpoint_digest":hex::encode(&session.checkpoint_id),
        "age_sequences":0, "maximum_age_sequences":p.maximum_age_sequences, "remaining":remaining.to_string(),
        "closed":record.closed, "revoked":record.revoked, "canonical_core_bytes":hex::encode(&budget_call)
    });
    let document = if export {
        let call = |signature: &[u8], result: &[u8]| {
            value!({
                "to":hex::encode(&PAY), "data":hex::encode(&data(signature, Some(&id))),
                "result":hex::encode(result), "height":session.finality.state_height
            })
        };
        value!({
            "schema":"layerx.human.budget-proof.v2", "tenant":p.tenant, "principal":p.principal,
            "protocol_version":PROTOCOL_VERSION,"network_id":config.protocol_network_id(),"maximum_age_seconds":p.maximum_age_seconds,
            "head":{"committed_height":session.committed_height,"block_hash":hex::encode(&session.checkpoint_id)},
            "budget_state":state,
            "accounts":{"owner":hex::encode(&record.owner),"budget":hex::encode(&record.account),"source":hex::encode(&record.source.unwrap_or(record.owner))},
            "identity_key":hex::encode(&identity),
            "proofs":{"budget_call":call(b"getBudget(bytes32)", &budget_call),"remaining_call":call(b"budgetRemaining(bytes32)", &remaining_call),"account_join":account_join},
            "finality":{"height":session.committed_height,"block_hash":hex::encode(&session.checkpoint_id),"app_hash":hex::encode(&session.state_root),"state_height":session.finality.state_height}
        })
    } else {
        state
    };
    let response = json(200, &document);
    if response.body.len() > MAX_EXPORT_BYTES {
        return Err(AuthorityError);
    }
    Ok(response)
}

/// # Errors
/// Returns a refusal response when the budget is unbound or its proof is unavailable.
pub fn read(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: &str,
) -> Result<Response, Response> {
    if !p.budgets.iter().any(|bound| bound == id) {
        return Err(refusal(404, "budget_not_bound", None));
    }
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce(config, p, id, false, None)
        .map_err(|AuthorityError| unavailable("budget_state_proof_unavailable"))
}

/// # Errors
/// Returns a refusal response when the dynamic budget proof is unavailable.
pub fn read_dynamic(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: &str,
    owner_did: &str,
    session: &Session,
) -> Result<Response, Response> {
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce_session(
        config,
        p,
        id,
        false,
        Some(owner_did),
        now_ms().map_err(|AuthorityError| unavailable("clock_unavailable"))?,
        session,
    )
    .map_err(|AuthorityError| unavailable("budget_state_proof_unavailable"))
}

/// # Errors
/// Returns a refusal response when the budget export is unbound or unavailable.
pub fn export(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: &str,
) -> Result<Response, Response> {
    let now = now_ms().map_err(|AuthorityError| unavailable("budget_proof_export_unavailable"))?;
    export_at(config, p, id, now)
}

/// # Errors
/// Returns a refusal response when the budget export is unbound or unavailable.
pub fn export_at(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: &str,
    now_ms: u128,
) -> Result<Response, Response> {
    if !p.budgets.iter().any(|bound| bound == id) {
        return Err(refusal(404, "budget_not_bound", None));
    }
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce_at(config, p, id, true, None, now_ms)
        .map_err(|AuthorityError| unavailable("budget_proof_export_unavailable"))
}

/// # Errors
/// Returns a refusal response when the dynamic budget export is unavailable.
pub fn export_dynamic(
    config: &impl Configuration,
    p: &PrincipalPolicy,
    id: &str,
    owner_did: &str,
    session: &Session,
) -> Result<Response, Response> {
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce_session(
        config,
        p,
        id,
        true,
        Some(owner_did),
        now_ms().map_err(|AuthorityError| unavailable("clock_unavailable"))?,
        session,
    )
    .map_err(|AuthorityError| unavailable("budget_proof_export_unavailable"))
}
