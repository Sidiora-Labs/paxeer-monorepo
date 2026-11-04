mod record;

use self::record::{number, Record};
use super::super::PROTOCOL_VERSION;
use super::{digest, hex, json, refusal, unavailable, Config, PrincipalPolicy, Response};
use layerx_client::px::{PxAccountJoin, PxEndpoint};
use layerx_client::rpc::account::{balance, ReadContext, Requested, Rung};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use serde_json::{json as value, Value};
use sha2::{Digest, Sha256};
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_EXPORT_BYTES: usize = 1_048_576;
const MAX_EXPORT_MATERIAL_BYTES: usize = (MAX_EXPORT_BYTES - 16_384) / 2;
const PAY: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

pub(super) struct Session {
    sequencer: SequencerEndpoint,
    network: PxEndpoint,
    pub(super) committed_height: u64,
    pub(super) checkpoint_id: [u8; 32],
    pub(super) timestamp: u64,
    state_root: [u8; 32],
}

struct AccountJoinExport(PxAccountJoin);

impl serde::Serialize for AccountJoinExport {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        use serde::ser::SerializeStruct as _;

        let join = &self.0;
        let mut state = serializer.serialize_struct("PxAccountJoin", 3)?;
        state.serialize_field("account", &value!({
            "evm_address": join.account.evm_address.map(|address| format!("0x{}", hex::encode(&address))),
            "pax_address": join.account.pax_address,
            "layerx_did": join.account.layerx_did,
            "layerx_account": join.account.layerx_account.map(|id| hex::encode(&id)),
            "bound": join.account.bound,
        }))?;
        state.serialize_field(
            "paxeer",
            &join.paxeer.as_ref().map(|account| {
                value!({
                    "address": format!("0x{}", hex::encode(&account.address)),
                    "balance": account.balance.to_string(),
                    "nonce": account.nonce,
                })
            }),
        )?;
        state.serialize_field("layerx", &join.layerx)?;
        state.end()
    }
}

fn data(signature: &[u8], argument: Option<&[u8; 32]>) -> Vec<u8> {
    let mut bytes = keccak256(signature)[..4].to_vec();
    if let Some(argument) = argument {
        bytes.extend_from_slice(argument);
    }
    bytes
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

impl Session {
    fn budget_call(&self, signature: &[u8], id: &[u8; 32]) -> Result<Vec<u8>, ()> {
        self.sequencer
            .call_at(self.committed_height, &PAY, &data(signature, Some(id)))
            .map_err(|_| ())
    }

    pub(super) fn open(config: &Config) -> Result<Self, ()> {
        let sequencer = SequencerEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        let network = PxEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        let committed_height = sequencer.block_number().map_err(|_| ())?;
        let block = sequencer
            .block_by_number(committed_height)
            .map_err(|_| ())?
            .ok_or(())?;
        let checkpoint_id = block.hash;
        let state_root = sequencer.state_root(committed_height).map_err(|_| ())?;
        if committed_height == 0
            || block.number != committed_height
            || checkpoint_id == [0; 32]
            || state_root == [0; 32]
        {
            return Err(());
        }
        Ok(Self {
            sequencer,
            network,
            committed_height,
            checkpoint_id,
            timestamp: block.timestamp.checked_mul(1000).ok_or(())?,
            state_root,
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
        let join = self.network.account(did).map_err(|_| ())?;
        if join.layerx != Some(layerx) {
            return Err(());
        }
        let join = serde_json::to_value(AccountJoinExport(join)).map_err(|_| ())?;
        Ok((account, join))
    }

    fn balance(&self, id: [u8; 32], asset: [u8; 32]) -> Result<u128, ()> {
        let account = self.network.account(&hex::encode(&id)).map_err(|_| ())?;
        if account.account.layerx_account != Some(id) {
            return Err(());
        }
        balance(
            &self.sequencer,
            id,
            asset,
            ReadContext {
                committed_height: self.committed_height,
                requested: Requested::new(Rung::Committed).map_err(|_| ())?,
            },
        )
        .map(|amount| amount.value())
        .map_err(|_| ())
    }

    pub(super) fn unchanged(&self) -> Result<(), ()> {
        if self.sequencer.block_number().map_err(|_| ())? != self.committed_height {
            return Err(());
        }
        Ok(())
    }

    pub(super) fn fresh(&self, maximum_age_seconds: u64) -> Result<(), ()> {
        self.fresh_at(maximum_age_seconds, now_ms()?)
    }

    fn fresh_at(&self, maximum_age_seconds: u64, now_ms: u128) -> Result<(), ()> {
        if now_ms
            .checked_sub(u128::from(self.timestamp))
            .is_none_or(|age| age > u128::from(maximum_age_seconds) * 1000)
        {
            return Err(());
        }
        Ok(())
    }
}

fn accounts(
    session: &Session,
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

fn now_ms() -> Result<u128, ()> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .map_err(|_| ())
}

fn produce(
    config: &Config,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
) -> Result<Response, ()> {
    produce_at(config, p, id, export, owner_did, now_ms()?)
}

fn produce_at(
    config: &Config,
    p: &PrincipalPolicy,
    id: [u8; 32],
    export: bool,
    owner_did: Option<&str>,
    now_ms: u128,
) -> Result<Response, ()> {
    let session = Session::open(config)?;
    let budget_call = session.budget_call(b"getBudget(bytes32)", &id)?;
    let record = Record::from_call(id, &budget_call)?;
    if hex::encode(&record.asset) != p.asset_id
        || record.revocation == 0
        || record.revocation > session.committed_height
    {
        return Err(());
    }
    let remaining_call = session.budget_call(b"budgetRemaining(bytes32)", &id)?;
    if remaining_call.len() != 32 {
        return Err(());
    }
    let remaining = number(&remaining_call, 0, 16)?;
    let (identity, account_join) = accounts(&session, &record, p, owner_did)?;
    session.fresh_at(p.maximum_age_seconds, now_ms)?;
    if p.maximum_age_sequences == 0 {
        return Err(());
    }
    session.unchanged()?;
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(session.checkpoint_id);
    let join_bytes = serde_json::to_vec(&account_join).map_err(|_| ())?;
    let material_bytes = [&budget_call, &remaining_call, &join_bytes]
        .iter()
        .try_fold(0_usize, |total, bytes| {
            total.checked_add(bytes.len()).ok_or(())
        })?;
    if material_bytes > MAX_EXPORT_MATERIAL_BYTES {
        return Err(());
    }
    for bytes in [&budget_call, &remaining_call, &join_bytes] {
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
        let call = |signature: &[u8], result: &[u8]| {
            value!({
                "to":hex::encode(&PAY), "data":hex::encode(&data(signature, Some(&id))),
                "result":hex::encode(result), "height":session.committed_height
            })
        };
        value!({
            "schema":"layerx.human.budget-proof.v2", "tenant":p.tenant, "principal":p.principal,
            "protocol_version":PROTOCOL_VERSION,"network_id":config.protocol_network_id,"maximum_age_seconds":p.maximum_age_seconds,
            "head":{"committed_height":session.committed_height,"block_hash":hex::encode(&session.checkpoint_id)},
            "budget_state":state,
            "accounts":{"owner":hex::encode(&record.owner),"budget":hex::encode(&record.account),"source":hex::encode(&record.source.unwrap_or(record.owner))},
            "identity_key":hex::encode(&identity),
            "proofs":{"budget_call":call(b"getBudget(bytes32)", &budget_call),"remaining_call":call(b"budgetRemaining(bytes32)", &remaining_call),"account_join":account_join},
            "finality":{"height":session.committed_height,"block_hash":hex::encode(&session.checkpoint_id),"app_hash":hex::encode(&session.state_root)}
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
    let now = now_ms().map_err(|()| unavailable("budget_proof_export_unavailable"))?;
    export_at(config, p, id, now)
}

fn export_at(
    config: &Config,
    p: &PrincipalPolicy,
    id: &str,
    now_ms: u128,
) -> Result<Response, Response> {
    if !p.budgets.iter().any(|bound| bound == id) {
        return Err(refusal(404, "budget_not_bound", None));
    }
    let id = hex::decode32(id).map_err(|_| refusal(400, "invalid_budget_id", None))?;
    produce_at(config, p, id, true, None, now_ms)
        .map_err(|()| unavailable("budget_proof_export_unavailable"))
}

#[cfg(test)]
impl Config {
    pub(crate) fn fixture_budget_export(
        &self,
        policy: &Value,
        id: &str,
        now_ms: u128,
    ) -> Result<Response, Response> {
        let policy: PrincipalPolicy = serde_json::from_value(policy.clone())
            .map_err(|_| refusal(400, "invalid_fixture_policy", None))?;
        export_at(self, &policy, id, now_ms)
    }
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
