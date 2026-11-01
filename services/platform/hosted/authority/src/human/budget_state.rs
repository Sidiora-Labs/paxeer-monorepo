mod record;

use self::record::{number, word, Record};
use super::super::PROTOCOL_VERSION;
use super::{digest, hex, json, refusal, unavailable, Config, PrincipalPolicy, Response};
use layerx_client::px::PxEndpoint;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use serde_json::{json as value, Value};
use sha2::{Digest, Sha256};
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_EXPORT_BYTES: usize = 1_048_576;
const MAX_EXPORT_MATERIAL_BYTES: usize = (MAX_EXPORT_BYTES - 16_384) / 2;
const ANCHOR: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x14,
];
const PAY: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

pub(super) struct Session<'a> {
    config: &'a Config,
    sequencer: SequencerEndpoint,
    network: PxEndpoint,
    pub(super) committed_height: u64,
    pub(super) batch_number: u64,
    pub(super) checkpoint_id: [u8; 32],
    pub(super) timestamp: u64,
    state_root: [u8; 32],
    finalized_height: u64,
    checkpoint_call: Vec<u8>,
}

fn data(signature: &[u8], argument: Option<&[u8; 32]>) -> Vec<u8> {
    let mut bytes = keccak256(signature)[..4].to_vec();
    if let Some(argument) = argument {
        bytes.extend_from_slice(argument);
    }
    bytes
}

fn u64_word(bytes: &[u8], index: usize) -> Result<u64, ()> {
    u64::try_from(number(bytes, index, 8)?).map_err(|_| ())
}

fn account_id(value: &Value) -> Result<[u8; 32], ()> {
    let text = value.as_str().ok_or(())?;
    hex::decode32(text.strip_prefix("0x").unwrap_or(text)).map_err(|_| ())
}

pub(super) fn identity_key(did: &str) -> Result<[u8; 32], ()> {
    let mut preimage = b"LXP/v1/did-id\0".to_vec();
    preimage.extend_from_slice(&u16::try_from(did.len()).map_err(|_| ())?.to_be_bytes());
    preimage.extend_from_slice(did.as_bytes());
    Ok(digest(&preimage))
}

impl<'a> Session<'a> {
    pub(super) fn open(config: &'a Config) -> Result<Self, ()> {
        let sequencer = SequencerEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        let network = PxEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        let committed_height = sequencer.block_number().map_err(|_| ())?;
        let latest = sequencer
            .call_at(committed_height, &ANCHOR, &data(b"latestFinalized()", None))
            .map_err(|_| ())?;
        if latest.len() != 64 || number(&latest, 1, 1)? != 1 {
            return Err(());
        }
        let batch_number = u64_word(&latest, 0)?;
        let argument = word(&latest, 0)?;
        let checkpoint_call = sequencer
            .call_at(
                committed_height,
                &ANCHOR,
                &data(b"checkpoint(uint64)", Some(&argument)),
            )
            .map_err(|_| ())?;
        if checkpoint_call.len() != 18 * 32
            || u64_word(&checkpoint_call, 0)? != batch_number
            || number(&checkpoint_call, 12, 1)? != 2
        {
            return Err(());
        }
        let checkpoint_id = word(&checkpoint_call, 1)?;
        let state_root = word(&checkpoint_call, 7)?;
        let finalized_height = u64_word(&checkpoint_call, 17)?;
        if state_root == [0; 32] || checkpoint_id == [0; 32] || finalized_height > committed_height
        {
            return Err(());
        }
        Ok(Self {
            config,
            sequencer,
            network,
            committed_height,
            batch_number,
            checkpoint_id,
            timestamp: u64_word(&checkpoint_call, 11)?,
            state_root,
            finalized_height,
            checkpoint_call,
        })
    }

    pub(super) fn identity(&self, did: &str, owner: [u8; 32]) -> Result<(Value, Value), ()> {
        let join = self.network.account(did).map_err(|_| ())?;
        let layerx = join.layerx.ok_or(())?;
        let accounts = layerx.get("accounts").and_then(Value::as_array).ok_or(())?;
        let mut matches = accounts.iter().filter(|row| {
            row["bound"] == true
                && account_id(&row["layerx_account"]) == Ok(owner)
                && row["evm_address"].as_str().is_some_and(|address| {
                    address
                        .strip_prefix("0x")
                        .and_then(|address| hex::decode(address).ok())
                        .is_some_and(|bytes| bytes.len() == 20 && bytes != [0; 20])
                })
        });
        let account = matches.next().ok_or(())?.clone();
        if matches.next().is_some() {
            return Err(());
        }
        let raw = layerx_paxeer_verifier::raw_call(
            &self.config.chain_endpoint,
            "px_getAccount",
            &[layerx_paxeer_verifier::Json::Text(did.to_owned())],
        )
        .map_err(|_| ())?;
        let raw: Value = serde_json::from_str(&raw.render()).map_err(|_| ())?;
        if raw["layerx"] != Value::Object(layerx) {
            return Err(());
        }
        Ok((account, raw))
    }

    fn balance(&self, id: [u8; 32], asset: [u8; 32]) -> Result<u128, ()> {
        let balances = self.network.balances(&hex::encode(&id)).map_err(|_| ())?;
        if balances.account.layerx_account != Some(id) {
            return Err(());
        }
        let mut rows = balances.balances.iter().filter(|row| row.asset_id == asset);
        let row = rows.next().ok_or(())?;
        if rows.next().is_some() {
            return Err(());
        }
        let layerx = row.layerx.as_ref().ok_or(())?;
        let amount = layerx.get("amount").and_then(Value::as_str).ok_or(())?;
        amount.parse().map_err(|_| ())
    }

    pub(super) fn unchanged(&self) -> Result<(), ()> {
        if self.sequencer.block_number().map_err(|_| ())? != self.committed_height {
            return Err(());
        }
        Ok(())
    }

    pub(super) fn fresh(&self, maximum_age_seconds: u64) -> Result<(), ()> {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(|_| ())?
            .as_millis();
        if now
            .checked_sub(u128::from(self.timestamp))
            .is_none_or(|age| age > u128::from(maximum_age_seconds) * 1000)
        {
            return Err(());
        }
        Ok(())
    }
}

fn accounts(
    session: &Session<'_>,
    record: &Record,
    policy: &PrincipalPolicy,
    owner_did: Option<&str>,
) -> Result<([u8; 32], Value), ()> {
    if hex::encode(&record.owner) != policy.account_id {
        return Err(());
    }
    if let Some(did) = owner_did {
        let (_, join) = session.identity(did, record.owner)?;
        session.balance(record.owner, record.asset)?;
        session.balance(record.account, record.asset)?;
        if let Some(source) = record.source {
            session.balance(source, record.asset)?;
        }
        return Ok((identity_key(did)?, join));
    }
    let mut bound = Vec::new();
    for identity in policy.identities.iter().filter(|identity| !identity.frozen) {
        if let Ok((_, join)) = session.identity(&identity.did, record.owner) {
            bound.push((identity_key(&identity.did)?, join));
        }
    }
    if bound.len() != 1 {
        return Err(());
    }
    session.balance(record.owner, record.asset)?;
    session.balance(record.account, record.asset)?;
    if let Some(source) = record.source {
        session.balance(source, record.asset)?;
    }
    bound.pop().ok_or(())
}

fn produce(
    config: &Config,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
) -> Result<Response, ()> {
    let session = Session::open(config)?;
    let budget_call = session
        .sequencer
        .call_at(
            session.committed_height,
            &PAY,
            &data(b"getBudget(bytes32)", Some(&id)),
        )
        .map_err(|_| ())?;
    let record = Record::from_call(id, &budget_call)?;
    if hex::encode(&record.asset) != p.asset_id
        || record.revocation == 0
        || record.revocation > session.committed_height
    {
        return Err(());
    }
    let remaining_call = session
        .sequencer
        .call_at(
            session.committed_height,
            &PAY,
            &data(b"budgetRemaining(bytes32)", Some(&id)),
        )
        .map_err(|_| ())?;
    if remaining_call.len() != 32 {
        return Err(());
    }
    let remaining = number(&remaining_call, 0, 16)?;
    let (identity, account_join) = accounts(&session, &record, p, owner_did)?;
    session.fresh(p.maximum_age_seconds)?;
    if p.maximum_age_sequences == 0 {
        return Err(());
    }
    session.unchanged()?;
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(session.checkpoint_id);
    let join_bytes = serde_json::to_vec(&account_join).map_err(|_| ())?;
    let material_bytes = [
        &budget_call,
        &remaining_call,
        &session.checkpoint_call,
        &join_bytes,
    ]
    .iter()
    .try_fold(0_usize, |total, bytes| {
        total.checked_add(bytes.len()).ok_or(())
    })?;
    if material_bytes > MAX_EXPORT_MATERIAL_BYTES {
        return Err(());
    }
    for bytes in [
        &budget_call,
        &remaining_call,
        &session.checkpoint_call,
        &join_bytes,
    ] {
        evidence.update(u64::try_from(bytes.len()).map_err(|_| ())?.to_be_bytes());
        evidence.update(bytes);
    }
    let evidence: [u8; 32] = evidence.finalize().into();
    let state = value!({
        "budget_id":hex::encode(&record.id), "asset":hex::encode(&record.asset),
        "revocation_sequence":record.revocation, "observed_head_sequence":session.committed_height,
        "verification":4, "evidence_digest":hex::encode(&evidence),
        "receipt_digest":hex::encode(&digest(&budget_call)), "checkpoint_digest":hex::encode(&session.checkpoint_id),
        "age_sequences":0, "maximum_age_sequences":p.maximum_age_sequences, "remaining":remaining.to_string(),
        "closed":record.closed, "revoked":record.revoked, "canonical_core_bytes":hex::encode(&budget_call)
    });
    let document = if export {
        value!({
            "schema":"layerx.human.budget-proof.v2", "tenant":p.tenant, "principal":p.principal,
            "protocol_version":PROTOCOL_VERSION,"network_id":config.protocol_network_id,"maximum_age_seconds":p.maximum_age_seconds,
            "head":{"batch_number":session.batch_number,"committed_height":session.committed_height,"checkpoint_id":hex::encode(&session.checkpoint_id)},
            "budget_state":state,
            "accounts":{"owner":hex::encode(&record.owner),"budget":hex::encode(&record.account),"source":hex::encode(&record.source.unwrap_or(record.owner))},
            "identity_key":hex::encode(&identity),
            "proofs":{"budget_call":hex::encode(&budget_call),"remaining_call":hex::encode(&remaining_call),"checkpoint_call":hex::encode(&session.checkpoint_call),"account_join":account_join},
            "finality":{"batch_number":session.batch_number,"checkpoint_id":hex::encode(&session.checkpoint_id),"state_root":hex::encode(&session.state_root),"finalized_height":session.finalized_height,"status":2},
            "maximum_bytes":MAX_EXPORT_BYTES
        })
    } else {
        state
    };
    let response = json(200, &document);
    if response.body.len() > MAX_EXPORT_BYTES {
        return Err(());
    }
    Ok(response)
}

pub(super) fn read(config: &Config, p: &PrincipalPolicy, id: &str) -> Result<Response, Response> {
    if !p.budgets.iter().any(|bound| bound == id) {
        return Err(refusal(404, "budget_not_bound", None));
    }
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce(config, p, id, false, None).map_err(|()| unavailable("budget_state_proof_unavailable"))
}

pub(super) fn read_dynamic(
    config: &Config,
    p: &PrincipalPolicy,
    id: &str,
    owner_did: &str,
) -> Result<Response, Response> {
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce(config, p, id, false, Some(owner_did))
        .map_err(|()| unavailable("budget_state_proof_unavailable"))
}

pub(super) fn export(config: &Config, p: &PrincipalPolicy, id: &str) -> Result<Response, Response> {
    if !p.budgets.iter().any(|bound| bound == id) {
        return Err(refusal(404, "budget_not_bound", None));
    }
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce(config, p, id, true, None).map_err(|()| unavailable("budget_proof_export_unavailable"))
}

pub(super) fn export_dynamic(
    config: &Config,
    p: &PrincipalPolicy,
    id: &str,
    owner_did: &str,
) -> Result<Response, Response> {
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce(config, p, id, true, Some(owner_did))
        .map_err(|()| unavailable("budget_proof_export_unavailable"))
}
