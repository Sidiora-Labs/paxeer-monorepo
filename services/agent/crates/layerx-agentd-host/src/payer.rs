use std::sync::Arc;
use std::time::{Duration, Instant};

use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_client::sequencer::{
    CallRequest, ExecutedReceipt, ReceiptKeyEntry, RpcRefusal, SequencerEndpoint, SequencerError,
    SequencerReceipt,
};
use layerx_crypto::evm_transaction::{assemble, transaction_hash, Eip1559Call};
use layerx_crypto::payments::Grant;
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::attestor::{unsigned_evm_transaction, AttestorError, AttestorSigner};
use layerx_mcp::tools::web::{ExactPayment, ExactTerms, GrantTerms, WebPayer, WebPayerError};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const CHAIN_ID: u64 = 125;
const RECEIPT_TIMEOUT: Duration = Duration::from_secs(30);

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PayerError {
    Boundary(WebPayerError),
    AttestorRefused { code: String },
    NodeRefused(RpcRefusal),
}

impl From<WebPayerError> for PayerError {
    fn from(error: WebPayerError) -> Self {
        Self::Boundary(error)
    }
}

impl From<PayerError> for WebPayerError {
    fn from(error: PayerError) -> Self {
        match error {
            PayerError::Boundary(error) => error,
            PayerError::AttestorRefused { .. } | PayerError::NodeRefused(_) => Self::Refused,
        }
    }
}

impl std::fmt::Display for PayerError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(formatter, "{self:?}")
    }
}

impl std::error::Error for PayerError {}

type OwnerCheck = Box<dyn FnMut() -> Result<(), WebPayerError> + Send>;
type SearchQuote = Box<dyn FnMut([u8; 32]) -> Result<u128, WebPayerError> + Send>;

pub struct ProductionWebPayer {
    payer_did: String,
    signer: Arc<AttestorSigner>,
    node: SequencerEndpoint,
    receipt_keys: Vec<ReceiptKeyEntry>,
    rpc: ureq::Agent,
    endpoint: String,
    owner_check: OwnerCheck,
    search_quote: SearchQuote,
    prepared: Vec<PrecompileCall>,
}

impl ProductionWebPayer {
    pub fn from_owner<A: layerx_agentd::human_runtime::HumanAuthorityBoundary + Send + 'static>(
        owner: layerx_agentd::human_runtime::SharedAgentOwner<A>,
        credential: layerx_agentd::session::SessionCredential,
        payer_did: String,
        signer: Arc<AttestorSigner>,
        endpoint: &str,
        trust_anchors: &[&[u8]],
        receipt_keys: Vec<ReceiptKeyEntry>,
    ) -> Result<Self, PayerError> {
        let public_key = signer.public_key();
        owner
            .mcp_native_web_registry(&credential, public_key)
            .map_err(human_error)?;
        let quote_owner = owner.clone();
        let quote_credential = credential.clone();
        Self::new(
            payer_did,
            signer,
            endpoint,
            trust_anchors,
            receipt_keys,
            Box::new(move || {
                owner
                    .mcp_native_web_registry(&credential, public_key)
                    .map(|_| ())
                    .map_err(human_error)
            }),
            Box::new(move |asset| {
                quote_owner
                    .mcp_native_web_search_quote(&quote_credential, public_key, asset)
                    .map_err(human_error)
            }),
        )
    }

    pub fn new(
        payer_did: String,
        signer: Arc<AttestorSigner>,
        endpoint: &str,
        trust_anchors: &[&[u8]],
        receipt_keys: Vec<ReceiptKeyEntry>,
        owner_check: OwnerCheck,
        search_quote: SearchQuote,
    ) -> Result<Self, PayerError> {
        if payer_did != format!("did:layerx:{}", hex(&signer.public_key()))
            || signer.evm_address() == [0; 20]
            || receipt_keys.is_empty()
            || receipt_keys.iter().enumerate().any(|(index, key)| {
                key.key_id == [0; 32]
                    || key.public_key == [0; 32]
                    || key.first_height > key.last_height
                    || receipt_keys[..index]
                        .iter()
                        .any(|other| other.key_id == key.key_id)
            })
        {
            return Err(WebPayerError::Refused.into());
        }
        let node = SequencerEndpoint::new(endpoint, trust_anchors).map_err(node_error)?;
        let roots: Vec<_> = trust_anchors
            .iter()
            .map(|der| ureq::tls::Certificate::from_der(der).to_owned())
            .collect();
        let rpc = ureq::Agent::config_builder()
            .timeout_global(Some(RECEIPT_TIMEOUT))
            .max_redirects(0)
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(ureq::tls::RootCerts::new_with_certs(&roots))
                    .build(),
            )
            .build()
            .into();
        Ok(Self {
            payer_did,
            signer,
            node,
            receipt_keys,
            rpc,
            endpoint: endpoint.to_owned(),
            owner_check,
            search_quote,
            prepared: Vec::new(),
        })
    }

    fn validate(
        &mut self,
        did: &str,
        account: [u8; 32],
        amount: u128,
        key: [u8; 32],
    ) -> Result<(), WebPayerError> {
        (self.owner_check)()?;
        if did != self.payer_did || account == [0; 32] || amount == 0 || key == [0; 32] {
            return Err(WebPayerError::Refused);
        }
        let mut owner = [0; 32];
        owner[12..].copy_from_slice(&self.signer.evm_address());
        let data = [&transaction_hash(b"accountOwner(bytes32)")[..4], &account].concat();
        let expected = self
            .node
            .call_pending(&PrecompileTarget::Ledger.address(), &data)
            .map_err(node_error)
            .map_err(WebPayerError::from)?;
        if expected.as_slice() != owner {
            return Err(WebPayerError::Refused);
        }
        Ok(())
    }

    fn remember(&mut self, call: PrecompileCall) -> Result<PrecompileCall, WebPayerError> {
        if !self.prepared.contains(&call) {
            if self.prepared.len() >= 64 {
                return Err(WebPayerError::Refused);
            }
            self.prepared.push(call.clone());
        }
        Ok(call)
    }

    fn signed_call(&self, call: &PrecompileCall) -> Result<Vec<u8>, PayerError> {
        if !matches!(
            (call.target, call.selector),
            (PrecompileTarget::Ledger, [0x78, 0xea, 0x11, 0xdb])
                | (PrecompileTarget::Ledger, [0x2e, 0x89, 0x49, 0x0f])
                | (PrecompileTarget::Ledger, [0xd0, 0xd5, 0x34, 0x80])
        ) || self.node.chain_id().map_err(node_error)? != CHAIN_ID
        {
            return Err(WebPayerError::Refused.into());
        }
        let data = call.encode().map_err(|_| WebPayerError::Refused)?;
        let nonce = self
            .node
            .pending_nonce(&self.signer.evm_address())
            .map_err(node_error)?;
        let gas_limit = self
            .node
            .estimate_gas(&CallRequest {
                from: Some(self.signer.evm_address()),
                to: Some(call.target.address()),
                value: 0,
                data: data.clone(),
            })
            .map_err(node_error)?;
        let response: Value = self
            .rpc
            .post(&self.endpoint)
            .send_json(json!({"jsonrpc":"2.0", "id":1, "method":"eth_gasPrice", "params":[]}))
            .map_err(|_| WebPayerError::Unavailable)?
            .body_mut()
            .with_config()
            .limit(65536)
            .read_json()
            .map_err(|_| WebPayerError::Unavailable)?;
        if response["jsonrpc"] != "2.0" || response["id"] != 1 || response.get("error").is_some() {
            return Err(WebPayerError::Unavailable.into());
        }
        let fee = response
            .get("result")
            .and_then(Value::as_str)
            .and_then(|value| value.strip_prefix("0x"))
            .and_then(|value| u128::from_str_radix(value, 16).ok())
            .filter(|fee| *fee > 0)
            .ok_or(WebPayerError::Unavailable)?;
        if gas_limit == 0 {
            return Err(WebPayerError::Unavailable.into());
        }
        let transaction = Eip1559Call {
            chain_id: CHAIN_ID,
            nonce,
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: fee,
            gas_limit,
            to: call.target.address(),
            data,
        };
        let mut signature = self
            .signer
            .sign_evm_transaction(&unsigned_evm_transaction(&transaction), CHAIN_ID)
            .map_err(attestor_error)?;
        signature[64] = signature[64]
            .checked_add(27)
            .ok_or(WebPayerError::Refused)?;
        let signature = AgentSignature::new(signature).map_err(|_| WebPayerError::Refused)?;
        Ok(assemble(&transaction, &signature))
    }

    fn send(&self, raw: &[u8], call: &PrecompileCall) -> Result<ExactPayment, PayerError> {
        let tx_hash = self.node.send_raw_transaction(raw).map_err(node_error)?;
        if tx_hash != transaction_hash(raw) {
            return Err(WebPayerError::Refused.into());
        }
        let deadline = Instant::now() + RECEIPT_TIMEOUT;
        loop {
            match self.node.sequencer_receipt(&tx_hash).map_err(node_error)? {
                Some(
                    SequencerReceipt::Receipted(execution)
                    | SequencerReceipt::Committed {
                        receipt: execution, ..
                    },
                ) => {
                    return receipt(
                        execution,
                        tx_hash,
                        self.signer.evm_address(),
                        call,
                        &self.receipt_keys,
                    )
                    .map_err(PayerError::from);
                }
                Some(SequencerReceipt::Submitted { .. }) | None => {}
            }
            if Instant::now() >= deadline {
                return Err(WebPayerError::Unavailable.into());
            }
            std::thread::sleep(Duration::from_millis(25));
        }
    }

    pub fn pay(&mut self, terms: &ExactTerms) -> Result<ExactPayment, PayerError> {
        let call = self.prepare_payment(terms)?;
        self.submit(&call)
    }

    pub fn grant(&mut self, terms: &GrantTerms) -> Result<Vec<u8>, PayerError> {
        let call = self.prepare_grant(terms)?;
        let raw = self.signed_call(&call)?;
        self.send(&raw, &call)?;
        self.prepared.retain(|prepared| prepared != &call);
        Ok(raw)
    }
    pub fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, PayerError> {
        (self.owner_check)()?;
        if !self.prepared.contains(call) {
            return Err(WebPayerError::Refused.into());
        }
        let raw = self.signed_call(call)?;
        let payment = self.send(&raw, call)?;
        self.prepared.retain(|prepared| prepared != call);
        Ok(payment)
    }
}

impl ProductionWebPayer {
    pub fn into_web_payer(self) -> WebPayerBoundary {
        WebPayerBoundary(self)
    }

    fn validate_search_quote(
        &mut self,
        asset: [u8; 32],
        amount: u128,
    ) -> Result<(), WebPayerError> {
        (self.owner_check)()?;
        if amount == 0 || (self.search_quote)(asset)? != amount {
            return Err(WebPayerError::Refused);
        }
        Ok(())
    }

    fn prepare_payment(&mut self, terms: &ExactTerms) -> Result<PrecompileCall, WebPayerError> {
        self.validate(
            &terms.payer_did,
            terms.payer_account,
            terms.amount,
            terms.idempotency_key,
        )?;
        self.remember(terms.prepare_pay_exact())
    }

    fn prepare_grant(&mut self, terms: &GrantTerms) -> Result<PrecompileCall, WebPayerError> {
        let grant = self.sign_grant(terms)?;
        self.remember(terms.prepare_grant(&grant)?)
    }

    fn sign_grant(&mut self, terms: &GrantTerms) -> Result<Grant, WebPayerError> {
        if terms.purpose_hash == [0; 32] {
            return Err(WebPayerError::Refused);
        }
        self.validate(
            &terms.payer_did,
            terms.payer_account,
            terms.amount,
            terms.idempotency_key,
        )?;
        let mut grant = Grant {
            id: [0; 32],
            from: terms.payer_account,
            recipient: terms.recipient,
            asset: terms.asset,
            per_draw_maximum: terms.amount,
            allowance: terms.amount,
            recurring: false,
            window_length: 0,
            expiration: terms.expires_at_ms,
            purpose_hash: terms.purpose_hash,
            has_reference: true,
            reference_hash: terms.idempotency_key,
            revocation_sequence: 0,
            public_key: self.signer.public_key(),
            signature: [0; 64],
        };
        grant.id = grant.signing_digest().map_err(|_| WebPayerError::Refused)?;
        grant.signature = *self
            .signer
            .sign_ledger_grant_with_stored_assertion(&grant)
            .map_err(attestor_error)
            .map_err(WebPayerError::from)?
            .signature();
        Ok(grant)
    }

    pub fn draw(
        &mut self,
        terms: &GrantTerms,
        grant_id: [u8; 32],
    ) -> Result<PrecompileCall, WebPayerError> {
        if grant_id == [0; 32] || terms.purpose_hash == [0; 32] {
            return Err(WebPayerError::Refused);
        }
        self.validate(
            &terms.payer_did,
            terms.payer_account,
            terms.amount,
            terms.idempotency_key,
        )?;
        let call = terms.next_sequence_call();
        let data = call.encode().map_err(|_| WebPayerError::Refused)?;
        let result = self
            .node
            .call_pending(&call.target.address(), &data)
            .map_err(node_error)
            .map_err(WebPayerError::from)?;
        if result.len() != 32 || result[..24] != [0; 24] {
            return Err(WebPayerError::Refused);
        }
        let receiver_sequence = u64::from_be_bytes(
            result[24..]
                .try_into()
                .map_err(|_| WebPayerError::Refused)?,
        );
        self.remember(terms.prepare_draw(grant_id, receiver_sequence))
    }
}

pub struct WebPayerBoundary(ProductionWebPayer);

impl WebPayer for WebPayerBoundary {
    fn validate_search_quote(
        &mut self,
        asset: [u8; 32],
        amount: u128,
    ) -> Result<(), WebPayerError> {
        self.0.validate_search_quote(asset, amount)
    }

    fn pay(&mut self, terms: &ExactTerms) -> Result<PrecompileCall, WebPayerError> {
        self.0.prepare_payment(terms)
    }

    fn sign_grant(&mut self, terms: &GrantTerms) -> Result<Grant, WebPayerError> {
        let grant = self.0.sign_grant(terms)?;
        self.0.remember(terms.prepare_grant(&grant)?)?;
        Ok(grant)
    }

    fn draw(
        &mut self,
        terms: &GrantTerms,
        grant_id: [u8; 32],
    ) -> Result<PrecompileCall, WebPayerError> {
        self.0.draw(terms, grant_id)
    }

    fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, WebPayerError> {
        self.0.submit(call).map_err(WebPayerError::from)
    }
}

pub fn receipt(
    execution: ExecutedReceipt,
    tx_hash: [u8; 32],
    sender: [u8; 20],
    call: &PrecompileCall,
    receipt_keys: &[ReceiptKeyEntry],
) -> Result<ExactPayment, WebPayerError> {
    let refuse = || WebPayerError::Refused;
    let keys: Vec<_> = receipt_keys
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    let decoded =
        verify_sequencer_signature(&execution.receipt, keys.as_slice()).map_err(|_| refuse())?;
    let protocol = decoded.protocol().ok_or_else(refuse)?;
    let logs = execution
        .logs
        .iter()
        .map(|log| {
            SequencerLog::new(
                log.address,
                log.topics.clone(),
                log.data.clone(),
                log.log_index,
            )
        })
        .collect();
    let verified = verify_logs(&decoded, logs).map_err(|_| refuse())?;
    let summary = verified.summary();
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.target.address());
    let mut height = [0; 32];
    height[24..].copy_from_slice(&execution.height.to_be_bytes());
    let mut fee = [0; 32];
    fee[16..].copy_from_slice(&protocol.fee_charged().to_be_bytes());
    if !execution.success
        || execution.tx_hash != tx_hash
        || execution.from != sender
        || execution.to != call.target.address()
        || protocol.activity_id() != tx_hash
        || protocol.result_code() != 0
        || protocol.from() != from
        || protocol.to() != to
        || protocol.batch_id() != height
        || protocol.debit_sequence() != execution.nonce
        || protocol.global_sequence() != execution.seq
        || protocol.timestamp() != execution.timestamp
        || protocol.context_hash() != execution.key_id
        || protocol.parameter_version() != 0
        || protocol.asset() != <[u8; 32]>::from(Sha256::digest(execution.fee_denom.as_bytes()))
        || execution.fee_amount != fee
        || summary.gas_used() != execution.gas_used
        || summary.cumulative_gas_used() != execution.cumulative_gas_used
        || summary.effective_gas_price() != execution.effective_gas_price
        || u64::from(summary.tx_index()) != execution.index
        || summary.contract_address() != execution.contract_address
        || summary.sponsor() != execution.sponsor
        || summary.fee_denom() != execution.fee_denom
        || usize::try_from(summary.return_data_len()).ok() != Some(execution.return_data.len())
        || protocol.activity_root() != <[u8; 32]>::from(Sha256::digest(&execution.return_data))
        || summary.vm_error_hash()
            != <[u8; 32]>::from(Sha256::digest(execution.vm_error.as_bytes()))
        || !receipt_keys.iter().any(|key| {
            key.key_id == execution.key_id
                && key.public_key == execution.signing_key
                && key.first_height <= execution.height
                && execution.height <= key.last_height
        })
    {
        return Err(refuse());
    }
    if call.target != PrecompileTarget::Ledger {
        return Err(refuse());
    }
    match (call.selector, call.arguments.as_slice()) {
        (
            [0x78, 0xea, 0x11, 0xdb],
            [AbiArgument::Word(payer), AbiArgument::Word(recipient), AbiArgument::Word(asset), AbiArgument::Word(amount), AbiArgument::Word(_), AbiArgument::Word(reference)],
        ) => {
            let event = transaction_hash(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)");
            if amount[..16] != [0; 16]
                || amount == &[0; 32]
                || verified
                    .logs()
                    .iter()
                    .filter(|log| {
                        log.address() == call.target.address()
                            && log.topics() == [event, *reference, *payer, *recipient]
                            && log.data() == [asset.as_slice(), amount.as_slice()].concat()
                    })
                    .count()
                    != 1
            {
                return Err(refuse());
            }
        }
        (
            [0x2e, 0x89, 0x49, 0x0f],
            [AbiArgument::Word(grant), AbiArgument::Word(recipient), AbiArgument::Word(amount), AbiArgument::Word(reference), AbiArgument::Word(_), AbiArgument::Word(_), AbiArgument::Word(extra)],
        ) => {
            let event = transaction_hash(
                b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
            );
            if amount[..16] != [0; 16]
                || amount == &[0; 32]
                || verified
                    .logs()
                    .iter()
                    .filter(|log| {
                        let topics = log.topics();
                        let data = log.data();
                        log.address() == call.target.address()
                            && topics.len() == 4
                            && topics[0] == event
                            && topics[1] == *grant
                            && topics[3] == *recipient
                            && data.len() == 128
                            && data[32..64] == *amount
                            && data[64..96] == *reference
                            && data[96..128] == *extra
                    })
                    .count()
                    != 1
            {
                return Err(refuse());
            }
        }
        ([0xd0, 0xd5, 0x34, 0x80], [AbiArgument::Bytes(_)]) => {}
        _ => return Err(refuse()),
    }
    Ok(ExactPayment {
        tx_hash,
        receipt: execution.receipt,
        logs: verified.into_logs(),
    })
}

fn attestor_error(error: AttestorError) -> PayerError {
    if let Some(code) = error.refusal_code() {
        return PayerError::AttestorRefused {
            code: code.to_owned(),
        };
    }
    match error {
        AttestorError::Timeout { .. } | AttestorError::Unavailable { .. } => {
            WebPayerError::Unavailable.into()
        }
        _ => WebPayerError::Refused.into(),
    }
}
fn node_error(error: SequencerError) -> PayerError {
    match error {
        SequencerError::Rpc(refusal) => PayerError::NodeRefused(refusal),
        _ => WebPayerError::Unavailable.into(),
    }
}
fn human_error(error: layerx_agentd::human::HumanOperationError) -> WebPayerError {
    match error {
        layerx_agentd::human::HumanOperationError::Unavailable => WebPayerError::Unavailable,
        _ => WebPayerError::Refused,
    }
}
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
