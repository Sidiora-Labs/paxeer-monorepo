use std::sync::{Arc, Mutex};

use layerx_agentd::approval::{ApprovalExpiry, ApprovalExpiryError, ApprovalOutcome};
use layerx_agentd::budget::{self, BudgetLimiter, LimitId, ReservationRequest};
use layerx_agentd::prepare::evm::verify_call_binding;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::decode_unsigned;
use layerx_types::ids::Did;
use serde::{Deserialize, Serialize};

use super::*;

/// The host shares these resources with its durable approval service.
#[derive(Clone)]
pub struct WebApprovalResources {
    pub store: Arc<Mutex<Store>>,
    pub limiter: Arc<BudgetLimiter>,
    pub limits: Vec<LimitId>,
    pub expiry: Arc<ApprovalExpiry>,
}

pub(super) enum HeldAuthorisation {
    Payment(Value),
    ReceiverDraw([u8; 32]),
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct StoredSpend {
    bytes: Vec<u8>,
    signing_hash: [u8; 32],
    digest: [u8; 32],
    sender: [u8; 20],
    commitment: [u8; 32],
    expiry: u64,
    expires_at_height: u64,
    activity_type: u16,
    receiver: String,
    amount: u128,
    asset: String,
    key: [u8; 32],
    selector: [u8; 4],
    arguments: Vec<StoredArgument>,
    grant_id: Option<[u8; 32]>,
}

#[derive(Serialize, Deserialize)]
enum StoredArgument {
    Word([u8; 32]),
    Bytes(Vec<u8>),
}

fn unavailable() -> WebToolError {
    WebToolError::Payer(WebPayerError::Unavailable)
}

fn changed() -> WebToolError {
    WebToolError::ApprovalChanged
}

fn storage_key(tenant: &TenantId, key: [u8; 32]) -> Result<TenantKey, WebToolError> {
    let mut id = b"web-spend-v1:".to_vec();
    id.extend_from_slice(&key);
    TenantKey::new(tenant.clone(), ObjectKind::PreparedActivity, id).map_err(|_| unavailable())
}

impl WebApprovalResources {
    fn load(&self, tenant: &TenantId, key: [u8; 32]) -> Result<Option<StoredSpend>, WebToolError> {
        let store = self.store.lock().map_err(|_| unavailable())?;
        let Some(record) = store.get(&storage_key(tenant, key)?) else {
            return Ok(None);
        };
        let spend: StoredSpend = serde_json::from_slice(record.bytes()).map_err(|_| changed())?;
        if spend.key != key {
            return Err(changed());
        }
        spend.prepared()?;
        Ok(Some(spend))
    }

    /// Returns the retained preparation for the daemon's approve/rebind operation.
    pub fn preparation(
        &self,
        tenant: &TenantId,
        key: [u8; 32],
    ) -> Result<Option<Prepared>, WebToolError> {
        self.load(tenant, key)?
            .map(|spend| spend.prepared())
            .transpose()
    }

    fn save(&self, tenant: &TenantId, spend: &StoredSpend) -> Result<(), WebToolError> {
        let bytes = serde_json::to_vec(spend).map_err(|_| changed())?;
        let mut store = self.store.lock().map_err(|_| unavailable())?;
        let key = storage_key(tenant, spend.key)?;
        if store.get(&key).is_some() {
            return Err(changed());
        }
        store.put_local(key, bytes).map_err(|_| unavailable())
    }
}

impl StoredSpend {
    fn call(&self) -> PrecompileCall {
        PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: self.selector,
            arguments: self
                .arguments
                .iter()
                .map(|arg| match arg {
                    StoredArgument::Word(word) => AbiArgument::Word(*word),
                    StoredArgument::Bytes(bytes) => AbiArgument::Bytes(bytes.clone()),
                })
                .collect(),
        }
    }

    fn receiver_draw(&self) -> bool {
        self.grant_id.is_some()
            && self.selector == layerx_crypto::disclosure::call::ledger_selector(DRAW_SIGNATURE)
    }

    fn draw_signing_hash(bytes: &[u8], commitment: [u8; 32]) -> [u8; 32] {
        let mut hash = Sha256::new();
        hash.update(b"layerx-web-receiver-draw-v1");
        hash.update(commitment);
        hash.update(bytes);
        hash.finalize().into()
    }

    fn prepared(&self) -> Result<Prepared, WebToolError> {
        let fee_limit = if self.receiver_draw() {
            if self.bytes != self.call().encode().map_err(|_| changed())?
                || Self::draw_signing_hash(&self.bytes, self.commitment) != self.signing_hash
                || <[u8; 32]>::from(Sha256::digest(&self.bytes)) != self.digest
            {
                return Err(changed());
            }
            0
        } else {
            let call = decode_unsigned(&self.bytes).map_err(|_| changed())?;
            bind_call(&call, self.sender).map_err(|_| changed())?;
            if call.to != PrecompileTarget::Ledger.address()
                || call.data != self.call().encode().map_err(|_| changed())?
                || call.signing_hash() != self.signing_hash
                || <[u8; 32]>::from(Sha256::digest(&self.bytes)) != self.digest
            {
                return Err(changed());
            }
            call.max_fee_per_gas
                .checked_mul(u128::from(call.gas_limit))
                .ok_or_else(changed)?
        };
        let receiver = AgentDid::new(self.receiver.clone()).map_err(|_| changed())?;
        Ok(Prepared {
            preparation_ref: PreparationRef::new(format!("web-{}", hex(&self.digest)))
                .map_err(|_| changed())?,
            unsigned_canonical_bytes: CanonicalBytes::new(self.bytes.clone())
                .map_err(|_| changed())?,
            signing_preimage: SigningPreimage::new(self.signing_hash.to_vec())
                .map_err(|_| changed())?,
            disclosure: Disclosure {
                canonical_digest: self.digest,
                activity_type: ActivityType(self.activity_type),
                actor: AgentDid::new(hex(&self.sender)).map_err(|_| changed())?,
                authority: AuthorityRef::new(format!("x402:{}", hex(&self.commitment)))
                    .map_err(|_| changed())?,
                counterparties: ExplicitSet::allow(vec![receiver.clone()]),
                amounts: ExplicitSet::allow(vec![DisclosedAmount {
                    counterparty: receiver,
                    amount: Amount(self.amount),
                }]),
                asset: Asset::new(self.asset.clone()).map_err(|_| changed())?,
                fee_limit: Amount(fee_limit),
                expiry: TimestampSeconds(self.expiry),
                idempotency_key: IdempotencyRef::new(hex(&self.key)).map_err(|_| changed())?,
            },
            expiry: TimestampSeconds(self.expiry),
        })
    }
}

impl WebPlane<'_, '_> {
    fn spend_commitment(&self, expiry: u64) -> Result<[u8; 32], WebToolError> {
        let bytes = serde_json::to_vec(&json!({
            "domain": SPEND_DOMAIN, "resource": self.resource_commitment,
            "tool": self.call.operation.tool_name(), "target": self.target,
            "key": self.call.idempotency_key, "expiry": expiry,
            "payer": self.config.payer_did, "sender": self.config.payer_address,
            "receiver": self.config.receiver_address, "network": self.config.network,
            "session": self.approval.context.session.0,
            "capability": self.approval.context.capability.0,
            "policy": self.approval.context.policy_version,
        }))
        .map_err(|_| changed())?;
        Ok(Sha256::digest(bytes).into())
    }

    pub(super) fn authorise_held(
        &mut self,
        offer: &PaymentRequirements,
        facts: &OfferFacts,
        key: [u8; 32],
    ) -> Result<HeldAuthorisation, WebToolError> {
        let resources = self.payer.approval_resources().ok_or_else(unavailable)?;
        let boundary = self.approval;
        if boundary.context.agent.as_bytes() != self.config.payer_did.as_bytes()
            || self.config.payer_address() != Some(boundary.context.actor_address)
        {
            return Err(changed());
        }
        if matches!(self.call.operation, WebOperation::Search(_))
            && self.call.currency == Currency::Pax
        {
            self.payer
                .validate_search_quote(facts.asset, facts.amount)
                .map_err(WebToolError::Payer)?;
        }
        let ticket = boundary
            .registry
            .ticket(key)
            .map_err(|error| WebToolError::Approval(ApprovalError::Daemon(error)))?;
        let stored = resources.load(&boundary.context.tenant, key)?;
        if ticket.is_some() && stored.is_none() {
            return Err(changed());
        }
        let spend = match stored {
            Some(spend) => spend,
            None => {
                let metadata = spend_disclosure(
                    self.config,
                    self.call,
                    self.target,
                    offer,
                    facts,
                    key,
                    boundary.clock,
                )?;
                let (call, grant_id) = self.approval_call(offer, facts, metadata.clone())?;
                let commitment = self.spend_commitment(metadata.expiry.0)?;
                let (bytes, signing_hash, sender) = if self.call.scheme == Scheme::Metered
                    && call.selector
                        == layerx_crypto::disclosure::call::ledger_selector(DRAW_SIGNATURE)
                    && grant_id.is_some()
                {
                    let bytes = call.encode().map_err(|_| changed())?;
                    let signing_hash = StoredSpend::draw_signing_hash(&bytes, commitment);
                    (bytes, signing_hash, boundary.context.actor_address)
                } else {
                    let transaction = self
                        .payer
                        .prepare_transaction(&call)
                        .map_err(WebToolError::Payer)?;
                    verify_call_binding(&transaction).map_err(|_| changed())?;
                    if transaction.from != boundary.context.actor_address
                        || transaction.call.to != call.target.address()
                        || transaction.call.data != call.encode().map_err(|_| changed())?
                        || self.config.network != format!("eip155:{}", transaction.call.chain_id)
                    {
                        return Err(changed());
                    }
                    (
                        transaction.call.encode_unsigned(),
                        transaction.signing_hash,
                        transaction.from,
                    )
                };
                let spend = StoredSpend {
                    digest: Sha256::digest(&bytes).into(),
                    bytes,
                    signing_hash,
                    sender,
                    commitment,
                    expiry: metadata.expiry.0,
                    expires_at_height: boundary
                        .current_height()
                        .checked_add(APPROVAL_WINDOW)
                        .ok_or_else(changed)?,
                    activity_type: self.call.scheme.activity_type(),
                    receiver: facts.receiver_did.clone(),
                    amount: facts.amount,
                    asset: self.call.currency.code().to_owned(),
                    key,
                    selector: call.selector,
                    arguments: call
                        .arguments
                        .into_iter()
                        .map(|arg| match arg {
                            AbiArgument::Word(word) => StoredArgument::Word(word),
                            AbiArgument::Bytes(bytes) => StoredArgument::Bytes(bytes),
                        })
                        .collect(),
                    grant_id,
                };
                self.validate_approval_call(offer, facts, &spend, &spend.prepared()?)?;
                resources.save(&boundary.context.tenant, &spend)?;
                spend
            }
        };
        if spend.commitment != self.spend_commitment(spend.expiry)?
            || spend.sender != boundary.context.actor_address
            || spend.amount != facts.amount
            || spend.receiver != facts.receiver_did
            || spend.asset != self.call.currency.code()
            || spend.activity_type != self.call.scheme.activity_type()
        {
            return Err(changed());
        }
        let prepared = spend.prepared()?;
        self.validate_approval_call(offer, facts, &spend, &prepared)?;
        if spend.expiry
            <= crate::approval::decision_time(boundary.clock)
                .map_err(WebToolError::Approval)?
                .get()
            || spend.expires_at_height <= boundary.current_height()
        {
            return Err(WebToolError::ApprovalRefused(ApprovalState::Expired));
        }
        let has_ticket = ticket.is_some();
        if let Some(ticket) = ticket {
            if ticket.disclosure != prepared.disclosure {
                return Err(changed());
            }
            match ticket.state {
                ApprovalState::AwaitingApproval => {
                    return Err(WebToolError::ApprovalPending(Box::new(ticket)))
                }
                ApprovalState::Approved => (),
                state => return Err(WebToolError::ApprovalRefused(state)),
            }
        }
        let decision = resources.expiry.recover(
            &boundary.context.tenant,
            key,
            spend.expires_at_height,
            boundary.current_height(),
            &resources.limiter,
        );
        if !has_ticket && matches!(decision, Err(ApprovalExpiryError::NotFound)) {
            let reservation = budget::reserve(
                &resources.limiter,
                &ReservationRequest {
                    id: key,
                    amount: facts.amount,
                    expiry_sequence: spend.expires_at_height,
                    current_sequence: boundary.current_height(),
                    applicable_limits: resources.limits.clone(),
                },
            )
            .map_err(|_| WebToolError::Payer(WebPayerError::Refused))?;
            let mut context = boundary.context.clone();
            context.request_id = key;
            context.agent = Did::new(hex(&spend.sender).as_bytes()).map_err(|_| changed())?;
            let held = layerx_agentd::policy::approval::hold_reserved_at(
                boundary.registry,
                context,
                prepared,
                boundary.current_height(),
                spend.expires_at_height,
                &reservation,
                boundary.clock,
            );
            let held = match held {
                Ok(held) => held,
                Err(error) => {
                    budget::release(
                        &resources.limiter,
                        key,
                        budget::ReleaseKind::Failed,
                        boundary.current_height(),
                    )
                    .map_err(|_| unavailable())?;
                    return Err(WebToolError::Approval(ApprovalError::Daemon(error)));
                }
            };
            return Err(WebToolError::ApprovalRequired(Box::new(held)));
        }
        // A durable granted decision binds the exact canonical bytes, including after restart.
        let decision = decision.map_err(|_| changed())?;
        let mut reference = Sha256::new();
        reference.update(b"layerx-approved-preparation-v1");
        reference.update(boundary.context.tenant.as_str().as_bytes());
        reference.update(key);
        reference.update(prepared.unsigned_canonical_bytes.as_bytes());
        if decision.outcome != ApprovalOutcome::Granted
            || decision.submission_ref != Some(reference.finalize().into())
        {
            return Err(changed());
        }
        if !resources
            .limiter
            .has_reservation(key)
            .map_err(|_| unavailable())?
        {
            return Err(changed());
        }
        if matches!(self.call.operation, WebOperation::Search(_))
            && self.call.currency == Currency::Pax
        {
            self.payer
                .validate_search_quote(facts.asset, facts.amount)
                .map_err(WebToolError::Payer)?;
        }
        if spend.receiver_draw() {
            return Ok(HeldAuthorisation::ReceiverDraw(
                spend.grant_id.ok_or_else(changed)?,
            ));
        }
        let payment = self
            .payer
            .submit_transaction(&prepared)
            .map_err(WebToolError::Payer)?;
        if let Some(grant_id) = spend.grant_id {
            check_grant(self.config, &payment, grant_id)?;
            Ok(HeldAuthorisation::Payment(
                json!({"grantId": hex(&grant_id), "idempotencyKey": hex(&key)}),
            ))
        } else {
            let digest = check_receipt(self.config, &payment, facts, None, key)?;
            Ok(HeldAuthorisation::Payment(
                json!({"idempotencyKey": hex(&key), "receiptDigest": hex(&digest), "txHash": format!("0x{}", hex(&payment.tx_hash)), "verificationLevel": "receipted"}),
            ))
        }
    }

    fn grant_terms(
        &self,
        facts: &OfferFacts,
        disclosure: &Disclosure,
    ) -> Result<GrantTerms, WebToolError> {
        Ok(GrantTerms {
            payer_did: self.config.payer_did.clone(),
            payer_account: facts.payer_accounts[0],
            recipient: facts.pay_to,
            receiver_did_key: facts
                .receiver_did
                .strip_prefix("did:layerx:")
                .and_then(unhex32)
                .ok_or(WebToolError::OfferMismatch("account"))?,
            asset: facts.asset,
            amount: facts.amount,
            expires_at_ms: expiry_milliseconds(disclosure.expiry.0)?,
            purpose_hash: facts
                .purpose_hash
                .ok_or(WebToolError::OfferMismatch("purpose"))?,
            idempotency_key: self.call.idempotency_key,
        })
    }

    fn approval_call(
        &mut self,
        offer: &PaymentRequirements,
        facts: &OfferFacts,
        disclosure: Disclosure,
    ) -> Result<(PrecompileCall, Option<[u8; 32]>), WebToolError> {
        match self.call.scheme {
            Scheme::Exact => {
                let terms =
                    self.call
                        .exact_terms_for_disclosure(self.config, offer, disclosure, facts)?;
                let call = self.payer.pay(&terms).map_err(WebToolError::Payer)?;
                <dyn WebPayer>::validate_exact_call(&terms, &call)?;
                Ok((call, None))
            }
            Scheme::Metered => {
                let terms = self.grant_terms(facts, &disclosure)?;
                let offered = offer
                    .extra
                    .as_ref()
                    .and_then(|extra| extra.pointer("/layerx/grantId"));
                let existing = if let Some(id) = offered {
                    Some(
                        id.as_str()
                            .and_then(unhex32)
                            .filter(|id| *id != [0; 32])
                            .ok_or_else(changed)?,
                    )
                } else {
                    self.payer
                        .standing_grant(&terms)
                        .map_err(WebToolError::Payer)?
                };
                if let Some(id) = existing {
                    self.validate_existing_grant(&terms, id)?;
                    let result = self
                        .payer
                        .view(&terms.next_sequence_call())
                        .map_err(WebToolError::Payer)?;
                    let sequence = sequence_result(&result).map_err(WebToolError::Payer)?;
                    Ok((terms.prepare_draw(id, sequence), Some(id)))
                } else {
                    let call = self.payer.grant(&terms).map_err(WebToolError::Payer)?;
                    let grant = <dyn WebPayer>::decode_grant_call(&terms, &call)?;
                    Ok((call, Some(grant.id)))
                }
            }
        }
    }

    fn validate_existing_grant(
        &mut self,
        terms: &GrantTerms,
        id: [u8; 32],
    ) -> Result<(), WebToolError> {
        let grant = getPayerGrant(self.payer, id).map_err(WebToolError::Payer)?;
        if grant.id != id
            || grant.from != terms.payer_account
            || grant.recipient != terms.recipient
            || grant.asset != terms.asset
            || grant.purpose_hash != terms.purpose_hash
            || grant.per_draw_maximum < terms.amount
            || grant.allowance < terms.amount
        {
            return Err(changed());
        }
        Ok(())
    }

    fn validate_approval_call(
        &mut self,
        offer: &PaymentRequirements,
        facts: &OfferFacts,
        spend: &StoredSpend,
        prepared: &Prepared,
    ) -> Result<(), WebToolError> {
        match self.call.scheme {
            Scheme::Exact => {
                let terms = self.call.exact_terms_for_disclosure(
                    self.config,
                    offer,
                    prepared.disclosure.clone(),
                    facts,
                )?;
                if spend.grant_id.is_some() {
                    return Err(changed());
                }
                <dyn WebPayer>::validate_exact_call(&terms, &spend.call())
            }
            Scheme::Metered => {
                let terms = self.grant_terms(facts, &prepared.disclosure)?;
                let id = spend.grant_id.ok_or_else(changed)?;
                if spend.receiver_draw() {
                    terms
                        .validate_draw_call(id, &spend.call())
                        .map_err(|_| changed())?;
                    return self.validate_existing_grant(&terms, id);
                }
                let grant = <dyn WebPayer>::decode_grant_call(&terms, &spend.call())?;
                if grant.id != id {
                    return Err(changed());
                }
                Ok(())
            }
        }
    }
}
