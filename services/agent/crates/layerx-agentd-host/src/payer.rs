use std::sync::Arc;
use std::time::{Duration, Instant};

use layerx_agentd::prepare::evm::{PrecompileCall, PrecompileTarget};
use layerx_client::sequencer::{
    CallRequest, RpcRefusal, SequencerEndpoint, SequencerError, SequencerReceipt,
};
use layerx_crypto::evm_transaction::{assemble, transaction_hash, Eip1559Call};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::attestor::{unsigned_evm_transaction, AttestorError, AttestorSigner};
use layerx_mcp::tools::web::{ExactPayment, ExactTerms, GrantTerms, WebPayer, WebPayerError};
use serde_json::{json, Value};

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
        owner_check: OwnerCheck,
        search_quote: SearchQuote,
    ) -> Result<Self, PayerError> {
        if payer_did != format!("did:layerx:{}", hex(&signer.public_key()))
            || signer.evm_address() == [0; 20]
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
        account: [u8; 20],
        amount: u128,
        key: [u8; 32],
    ) -> Result<(), WebPayerError> {
        (self.owner_check)()?;
        if did != self.payer_did
            || account != self.signer.evm_address()
            || amount == 0
            || key == [0; 32]
        {
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
                | (PrecompileTarget::Pay, [0xa8, 0x63, 0x16, 0x2b])
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

    fn send(&self, raw: &[u8], target: PrecompileTarget) -> Result<ExactPayment, PayerError> {
        let tx_hash = self.node.send_raw_transaction(raw).map_err(node_error)?;
        if tx_hash != transaction_hash(raw) {
            return Err(WebPayerError::Refused.into());
        }
        let deadline = Instant::now() + RECEIPT_TIMEOUT;
        loop {
            match self.node.sequencer_receipt(&tx_hash).map_err(node_error)? {
                Some(
                    SequencerReceipt::Receipted(receipt)
                    | SequencerReceipt::Committed { receipt, .. },
                ) => {
                    if !receipt.success
                        || receipt.from != self.signer.evm_address()
                        || receipt.to != target.address()
                    {
                        return Err(WebPayerError::Refused.into());
                    }
                    return Ok(ExactPayment {
                        tx_hash,
                        receipt: receipt.receipt,
                        logs: receipt
                            .logs
                            .into_iter()
                            .map(|log| {
                                layerx_proof::sequencer_logs::SequencerLog::new(
                                    log.address,
                                    log.topics,
                                    log.data,
                                    log.log_index,
                                )
                            })
                            .collect(),
                    });
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
        self.send(&raw, call.target)?;
        self.prepared.retain(|prepared| prepared != &call);
        Ok(raw)
    }
    pub fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, PayerError> {
        (self.owner_check)()?;
        if !self.prepared.contains(call) {
            return Err(WebPayerError::Refused.into());
        }
        let raw = self.signed_call(call)?;
        let payment = self.send(&raw, call.target)?;
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
        self.validate(
            &terms.payer_did,
            terms.payer_account,
            terms.amount,
            terms.idempotency_key,
        )?;
        if terms.purpose_hash == [0; 32] || terms.current_height == u64::MAX {
            return Err(WebPayerError::Refused);
        }
        self.remember(terms.prepare_budget())
    }

    pub fn draw(
        &mut self,
        terms: &GrantTerms,
        grant_id: [u8; 32],
    ) -> Result<PrecompileCall, WebPayerError> {
        self.validate(
            &terms.payer_did,
            terms.payer_account,
            terms.amount,
            terms.idempotency_key,
        )?;
        if grant_id == [0; 32] || terms.purpose_hash == [0; 32] {
            return Err(WebPayerError::Refused);
        }
        self.remember(terms.prepare_draw(grant_id))
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

    fn grant(&mut self, terms: &GrantTerms) -> Result<PrecompileCall, WebPayerError> {
        self.0.prepare_grant(terms)
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
