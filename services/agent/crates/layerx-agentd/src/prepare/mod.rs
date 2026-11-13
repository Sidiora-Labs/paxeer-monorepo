//! Preparation of EIP-1559 calls against sequencer pending state.

pub mod disclose;

pub mod evm {
    use std::time::Duration;

    use layerx_client::sequencer::{CallRequest, SequencerEndpoint, SequencerError};
    use layerx_crypto::disclosure::{Disclosure, DisclosureError};
    use layerx_crypto::evm_transaction::Eip1559Call;
    use serde_json::{json, Value};

    /// Chain inputs observed before estimating one pending-state call.
    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub struct EvmPreparationState {
        pub chain_id: u64,
        pub pending_nonce: u64,
        pub base_fee_per_gas: u128,
        pub head_height: u64,
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub enum PrecompileTarget {
        Ledger,
        Pay,
        Auth,
        Programs,
    }

    impl PrecompileTarget {
        #[must_use]
        pub const fn address(self) -> [u8; 20] {
            let mut address = [0; 20];
            address[18] = 0x10;
            address[19] = match self {
                Self::Ledger => 0x1a,
                Self::Pay => 0x1b,
                Self::Auth => 0x1c,
                Self::Programs => 0x1d,
            };
            address
        }
    }

    /// ABI words and dynamic byte arguments, in the selected method's ABI order.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub enum AbiArgument {
        Word([u8; 32]),
        Bytes(Vec<u8>),
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct PrecompileCall {
        pub target: PrecompileTarget,
        pub selector: [u8; 4],
        pub arguments: Vec<AbiArgument>,
    }

    impl PrecompileCall {
        #[must_use]
        pub fn ledger_transfer(payload: Vec<u8>) -> Self {
            Self {
                target: PrecompileTarget::Ledger,
                selector: [0x2b, 0xb1, 0xae, 0x7c],
                arguments: vec![AbiArgument::Bytes(payload)],
            }
        }

        fn encoded_len(&self) -> Result<usize, PrepareError> {
            let head = self
                .arguments
                .len()
                .checked_mul(32)
                .and_then(|length| length.checked_add(4))
                .ok_or(PrepareError::InvalidDefaults)?;
            self.arguments
                .iter()
                .try_fold(head, |length, argument| match argument {
                    AbiArgument::Word(_) => Ok(length),
                    AbiArgument::Bytes(bytes) => bytes
                        .len()
                        .checked_add(31)
                        .map(|length| length & !31)
                        .and_then(|padded| padded.checked_add(32))
                        .and_then(|tail| length.checked_add(tail))
                        .ok_or(PrepareError::InvalidDefaults),
                })
        }

        /// Encodes head offsets relative to the ABI argument block, excluding the selector.
        ///
        /// # Errors
        /// Returns `PrepareError::InvalidDefaults` if ABI head or tail sizing overflows.
        pub fn encode(&self) -> Result<Vec<u8>, PrepareError> {
            let head_length = self
                .arguments
                .len()
                .checked_mul(32)
                .ok_or(PrepareError::InvalidDefaults)?;
            let mut head = Vec::with_capacity(head_length);
            let mut tail = Vec::new();
            for argument in &self.arguments {
                match argument {
                    AbiArgument::Word(word) => head.extend_from_slice(word),
                    AbiArgument::Bytes(bytes) => {
                        let offset = head_length
                            .checked_add(tail.len())
                            .ok_or(PrepareError::InvalidDefaults)?;
                        head.extend_from_slice(&abi_size(offset));
                        tail.extend_from_slice(&abi_size(bytes.len()));
                        tail.extend_from_slice(bytes);
                        let padded = tail
                            .len()
                            .checked_add(31)
                            .ok_or(PrepareError::InvalidDefaults)?
                            & !31;
                        tail.resize(padded, 0);
                    }
                }
            }
            let mut data = self.selector.to_vec();
            data.extend_from_slice(&head);
            data.extend_from_slice(&tail);
            Ok(data)
        }
    }

    fn abi_size(value: usize) -> [u8; 32] {
        let mut word = [0; 32];
        let bytes = value.to_be_bytes();
        word[32 - bytes.len()..].copy_from_slice(&bytes);
        word
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub struct PreparationDefaults {
        pub max_priority_fee_per_gas: u128,
        pub max_fee_per_gas: u128,
        pub maximum_payload_bytes: usize,
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct PrepareRequest {
        pub from: [u8; 20],
        pub call: PrecompileCall,
        pub expected_nonce: Option<u64>,
        pub max_priority_fee_per_gas: Option<u128>,
        pub max_fee_per_gas: Option<u128>,
        pub declared_payload_limit: usize,
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct Prepared {
        pub call: Eip1559Call,
        pub from: [u8; 20],
        pub disclosure: Disclosure,
        pub disclosure_digest: [u8; 32],
        pub signing_hash: [u8; 32],
        pub observed_head_height: u64,
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub enum PrepareError {
        Sequencer(SequencerError),
        Disclosure(DisclosureError),
        StaleNonce { expected: u64, pending: u64 },
        InvalidDefaults,
        PayloadLimitExceeded { actual: usize, maximum: usize },
    }

    /// Uses the same endpoint and pinned roots for typed pending reads and the head header.
    pub struct ProductionEvmPreparationBoundary {
        endpoint: SequencerEndpoint,
        header_agent: ureq::Agent,
        url: String,
    }

    impl ProductionEvmPreparationBoundary {
        /// # Errors
        /// Returns the sequencer endpoint error for invalid URLs or TLS trust roots.
        pub fn new(url: &str, trust_anchors: &[&[u8]]) -> Result<Self, SequencerError> {
            let endpoint = SequencerEndpoint::new(url, trust_anchors)?;
            let roots: Vec<_> = trust_anchors
                .iter()
                .map(|anchor| ureq::tls::Certificate::from_der(anchor).to_owned())
                .collect();
            let header_agent = ureq::Agent::config_builder()
                .timeout_global(Some(Duration::from_secs(30)))
                .http_status_as_error(false)
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
                endpoint,
                header_agent,
                url: url.to_owned(),
            })
        }

        /// # Errors
        /// Returns transport, TLS or RPC failures, `Unavailable` for a missing head,
        /// and `Malformed` when the header fields or verified block hash disagree.
        pub fn preparation_state(
            &self,
            from: &[u8; 20],
        ) -> Result<EvmPreparationState, SequencerError> {
            let mut response = self
                .header_agent
                .post(&self.url)
                .send_json(json!({"jsonrpc": "2.0", "id": 1,
                    "method": "eth_getBlockByNumber", "params": ["latest", false]}))
                .map_err(|value| map_transport(&value))?;
            let status = response.status().as_u16();
            match status {
                200 => {}
                502..=504 => return Err(SequencerError::Unavailable),
                other => return Err(SequencerError::HttpStatus(other)),
            }
            let answer: Value = response
                .body_mut()
                .with_config()
                .limit(16 * 1_048_576)
                .read_json()
                .map_err(|_| SequencerError::Malformed)?;
            if answer.get("jsonrpc").and_then(Value::as_str) != Some("2.0")
                || answer.get("id").and_then(Value::as_u64) != Some(1)
                || answer.get("error").is_some() == answer.get("result").is_some()
            {
                return Err(SequencerError::Malformed);
            }
            if let Some(error) = answer.get("error") {
                return Err(SequencerError::Rpc(layerx_client::sequencer::RpcRefusal {
                    code: error
                        .get("code")
                        .and_then(Value::as_i64)
                        .ok_or(SequencerError::Malformed)?,
                    message: error
                        .get("message")
                        .and_then(Value::as_str)
                        .ok_or(SequencerError::Malformed)?
                        .to_owned(),
                    data: error.get("data").cloned(),
                }));
            }
            let header = answer.get("result").ok_or(SequencerError::Malformed)?;
            let head_height = u64::try_from(quantity(header.get("number"))?)
                .map_err(|_| SequencerError::Malformed)?;
            let base_fee_per_gas = quantity(header.get("baseFeePerGas"))?;
            let block = self
                .endpoint
                .block_by_number(head_height)?
                .ok_or(SequencerError::Unavailable)?;
            let hash = header
                .get("hash")
                .and_then(Value::as_str)
                .ok_or(SequencerError::Malformed)?;
            if hash.len() != 66 || !hash.starts_with("0x") {
                return Err(SequencerError::Malformed);
            }
            for (index, expected) in block.hash.iter().enumerate() {
                let byte = hash
                    .get(2 + index * 2..4 + index * 2)
                    .and_then(|byte| u8::from_str_radix(byte, 16).ok())
                    .ok_or(SequencerError::Malformed)?;
                if byte != *expected {
                    return Err(SequencerError::Malformed);
                }
            }
            Ok(EvmPreparationState {
                chain_id: self.endpoint.chain_id()?,
                pending_nonce: self.endpoint.pending_nonce(from)?,
                base_fee_per_gas,
                head_height,
            })
        }
    }

    fn map_transport(error: &ureq::Error) -> SequencerError {
        match error {
            ureq::Error::Timeout(_)
            | ureq::Error::Io(_)
            | ureq::Error::HostNotFound
            | ureq::Error::ConnectionFailed
            | ureq::Error::Protocol(_)
            | ureq::Error::BodyStalled => SequencerError::Unavailable,
            ureq::Error::Tls(_)
            | ureq::Error::Pem(_)
            | ureq::Error::Rustls(_)
            | ureq::Error::TlsRequired => SequencerError::Tls,
            ureq::Error::BodyExceedsLimit(_) | ureq::Error::LargeResponseHeader(..) => {
                SequencerError::Malformed
            }
            _ => SequencerError::Endpoint,
        }
    }

    fn quantity(value: Option<&Value>) -> Result<u128, SequencerError> {
        let digits = value
            .and_then(Value::as_str)
            .and_then(|text| text.strip_prefix("0x"))
            .ok_or(SequencerError::Malformed)?;
        if digits.is_empty()
            || digits.len() > 32
            || (digits.len() > 1 && digits.starts_with('0'))
            || !digits.bytes().all(|byte| byte.is_ascii_hexdigit())
        {
            return Err(SequencerError::Malformed);
        }
        u128::from_str_radix(digits, 16).map_err(|_| SequencerError::Malformed)
    }

    /// # Errors
    /// Returns sequencer read or gas-estimation failures, invalid fee or payload limits,
    /// a stale nonce, or a disclosure error when the resulting call cannot be bound.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public preparation API that accepts an owned request."
    )]
    pub fn prepare_activity(
        boundary: &ProductionEvmPreparationBoundary,
        defaults: PreparationDefaults,
        request: PrepareRequest,
    ) -> Result<Prepared, PrepareError> {
        let state = boundary
            .preparation_state(&request.from)
            .map_err(PrepareError::Sequencer)?;
        let mut call = build_call(state, defaults, &request)?;
        call.gas_limit = boundary
            .endpoint
            .estimate_gas(&CallRequest {
                from: Some(request.from),
                to: Some(call.to),
                value: 0,
                data: call.data.clone(),
            })
            .map_err(PrepareError::Sequencer)?;
        finish_call(call, request.from, state.head_height)
    }

    /// Deterministic construction from an observed state and its pending gas estimate.
    ///
    /// # Errors
    /// Returns invalid defaults, a stale nonce, an exceeded payload limit, or a
    /// disclosure error when the constructed call cannot be bound.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn prepare_bound(
        state: EvmPreparationState,
        defaults: PreparationDefaults,
        request: PrepareRequest,
        gas_limit: u64,
    ) -> Result<Prepared, PrepareError> {
        let mut call = build_call(state, defaults, &request)?;
        call.gas_limit = gas_limit;
        finish_call(call, request.from, state.head_height)
    }

    fn finish_call(
        call: Eip1559Call,
        from: [u8; 20],
        observed_head_height: u64,
    ) -> Result<Prepared, PrepareError> {
        if call.gas_limit == 0 {
            return Err(PrepareError::InvalidDefaults);
        }
        let bound = super::disclose::bind(&call, from).map_err(PrepareError::Disclosure)?;
        let disclosure = bound.disclosure;
        let disclosure_digest = bound.digest;
        Ok(Prepared {
            from,
            disclosure,
            disclosure_digest,
            signing_hash: call.signing_hash(),
            call,
            observed_head_height,
        })
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub enum CallBindingError {
        Disclosure(DisclosureError),
        DigestMismatch,
        SigningHashMismatch,
    }

    /// # Errors
    /// Returns a binding error when the retained call, disclosure or hashes disagree.
    pub fn verify_call_binding(prepared: &Prepared) -> Result<(), CallBindingError> {
        super::disclose::verify_binding(prepared)
    }

    fn build_call(
        state: EvmPreparationState,
        defaults: PreparationDefaults,
        request: &PrepareRequest,
    ) -> Result<Eip1559Call, PrepareError> {
        if state.chain_id == 0 || defaults.maximum_payload_bytes == 0 {
            return Err(PrepareError::InvalidDefaults);
        }
        if let Some(expected) = request.expected_nonce {
            if expected != state.pending_nonce {
                return Err(PrepareError::StaleNonce {
                    expected,
                    pending: state.pending_nonce,
                });
            }
        }
        let max_priority_fee_per_gas = request
            .max_priority_fee_per_gas
            .unwrap_or(defaults.max_priority_fee_per_gas);
        let max_fee_per_gas = state
            .base_fee_per_gas
            .checked_mul(2)
            .and_then(|fee| fee.checked_add(max_priority_fee_per_gas))
            .ok_or(PrepareError::InvalidDefaults)?;
        if max_fee_per_gas > request.max_fee_per_gas.unwrap_or(defaults.max_fee_per_gas)
            || max_fee_per_gas > defaults.max_fee_per_gas
        {
            return Err(PrepareError::InvalidDefaults);
        }
        let actual = request.call.encoded_len()?;
        let maximum = defaults
            .maximum_payload_bytes
            .min(request.declared_payload_limit);
        if actual > maximum {
            return Err(PrepareError::PayloadLimitExceeded { actual, maximum });
        }
        let data = request.call.encode()?;
        Ok(Eip1559Call {
            chain_id: state.chain_id,
            nonce: state.pending_nonce,
            max_priority_fee_per_gas,
            max_fee_per_gas,
            gas_limit: 0,
            to: request.call.target.address(),
            data,
        })
    }
}

pub use evm::*;
pub use evm::{
    PreparationDefaults as EvmPreparationDefaults, PrepareError as EvmPrepareError,
    PrepareRequest as EvmPrepareRequest,
};

pub type EvmPrepared = evm::Prepared;

#[path = "expiry.rs"]
pub mod lifecycle;

pub(crate) use lifecycle::PreparationAuthorization;
pub use lifecycle::{
    expire_elapsed, sweep_retention, DurablePreparation, ExpirationReport, LifecycleError,
    LifecycleState, PayloadRedaction, PreparationExtension, PreparationInvalidationReport,
    PreparationLifecycle, RetentionReport, EXTENSION_CALL, EXTENSION_CAPABILITY,
    EXTENSION_IDEMPOTENCY, EXTENSION_OUTCOME, EXTENSION_TERMINAL_SEQUENCE,
};
