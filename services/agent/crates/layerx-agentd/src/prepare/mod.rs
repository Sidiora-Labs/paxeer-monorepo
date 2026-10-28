//! Preparation of EIP-1559 calls against sequencer pending state.

pub mod evm {
    use std::time::Duration;

    use layerx_client::sequencer::{CallRequest, SequencerEndpoint, SequencerError};
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
        pub signing_hash: [u8; 32],
        pub observed_head_height: u64,
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub enum PrepareError {
        Sequencer(SequencerError),
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

        pub fn preparation_state(
            &self,
            from: &[u8; 20],
        ) -> Result<EvmPreparationState, SequencerError> {
            let mut response = self
                .header_agent
                .post(&self.url)
                .send_json(json!({"jsonrpc": "2.0", "id": 1,
                    "method": "eth_getBlockByNumber", "params": ["latest", false]}))
                .map_err(map_transport)?;
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

    fn map_transport(error: ureq::Error) -> SequencerError {
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
        finish_call(call, state.head_height)
    }

    /// Deterministic construction from an observed state and its pending gas estimate.
    pub fn prepare_bound(
        state: EvmPreparationState,
        defaults: PreparationDefaults,
        request: PrepareRequest,
        gas_limit: u64,
    ) -> Result<Prepared, PrepareError> {
        let mut call = build_call(state, defaults, &request)?;
        call.gas_limit = gas_limit;
        finish_call(call, state.head_height)
    }

    fn finish_call(call: Eip1559Call, observed_head_height: u64) -> Result<Prepared, PrepareError> {
        if call.gas_limit == 0 {
            return Err(PrepareError::InvalidDefaults);
        }
        Ok(Prepared {
            signing_hash: call.signing_hash(),
            call,
            observed_head_height,
        })
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

pub use evm::{
    prepare_bound, AbiArgument, EvmPreparationState, PrecompileCall, PrecompileTarget,
    PreparationDefaults as EvmPreparationDefaults, PrepareError as EvmPrepareError,
    PrepareRequest as EvmPrepareRequest, Prepared as EvmPrepared, ProductionEvmPreparationBoundary,
};

#[path = ""]
pub mod legacy {
    use layerx_types::activity::{
        ActivityBuildError, Authority, EnvelopeBuilder, TimestampBound, UnsignedEnvelope,
    };
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::{ActivityType, ModuleRegistry, Payload, PayloadError};
    use layerx_types::result::ResultCode;
    use layerx_wire::activity::encode_unsigned_envelope;
    use layerx_wire::hash::payload_hash_for;
    use layerx_wire::limits::PROTOCOL_VERSION;
    use layerx_wire::sign::preimage_unsigned;
    use layerx_wire::WireError;

    #[path = "disclose.rs"]
    mod disclosure_binding;
    #[path = "expiry.rs"]
    mod lifecycle;

    pub use disclosure_binding::{DisclosedPreparation, DisclosureBindingError};
    pub(crate) use lifecycle::PreparationAuthorization;
    pub use lifecycle::{
        DurablePreparation, ExpirationReport, LifecycleError, LifecycleState, PayloadRedaction,
        PreparationExtension, PreparationInvalidationReport, PreparationLifecycle, RetentionReport,
        EXTENSION_CAPABILITY, EXTENSION_IDEMPOTENCY, EXTENSION_OUTCOME,
    };

    /// Domain-separated digest of the validated structured disclosure.
    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub struct DisclosureDigest(pub [u8; 32]);

    /// Core-produced inputs required to prepare one activity.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct CorePreparationState {
        pub network_id: u32,
        pub account_sequence: u64,
        pub protocol_timestamp: u64,
        pub observed_head_sequence: u64,
        pub module_registry: ModuleRegistry,
    }

    /// Boundary that obtains preparation state from core without local guessing.
    pub trait CorePreparationBoundary {
        /// Obtains the preparation state for one actor directly from core.
        ///
        /// # Errors
        ///
        /// Returns `Unavailable` when core cannot be reached, `Unverified` when
        /// returned state cannot be trusted, and `Refused` for an authenticated
        /// typed core refusal.
        fn preparation_state(
            &mut self,
            actor: &Did,
        ) -> Result<CorePreparationState, CoreStateError>;
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub enum CoreStateError {
        Unavailable,
        Unverified,
        Refused { class: u8, result: ResultCode },
    }

    /// Production adapter from the authenticated LNI client into preparation.
    /// Correlation identifiers are boundary-local and never supply protocol state.
    pub struct ProductionCorePreparationBoundary<'a> {
        client: &'a mut layerx_client::Client,
        next_correlation_id: u64,
        last_state: Option<CorePreparationState>,
    }

    impl<'a> ProductionCorePreparationBoundary<'a> {
        /// Creates a production boundary with a non-zero first correlation id.
        ///
        /// # Errors
        ///
        /// Refuses zero, which is reserved for the LNI handshake.
        pub fn new(
            client: &'a mut layerx_client::Client,
            first_correlation_id: u64,
        ) -> Result<Self, CoreStateError> {
            if first_correlation_id == 0 {
                return Err(CoreStateError::Unavailable);
            }
            Ok(Self {
                client,
                next_correlation_id: first_correlation_id,
                last_state: None,
            })
        }

        /// Returns the exact node snapshot consumed by the most recent successful
        /// preparation call. Callers retain its registry for later signature
        /// verification instead of querying a second authority.
        #[must_use]
        pub const fn last_state(&self) -> Option<&CorePreparationState> {
            self.last_state.as_ref()
        }
    }

    impl CorePreparationBoundary for ProductionCorePreparationBoundary<'_> {
        fn preparation_state(
            &mut self,
            actor: &Did,
        ) -> Result<CorePreparationState, CoreStateError> {
            let correlation_id = self.next_correlation_id;
            if correlation_id == 0 {
                return Err(CoreStateError::Unavailable);
            }
            self.next_correlation_id = correlation_id.checked_add(1).unwrap_or(0);
            let state = self
                .client
                .preparation_state(actor, correlation_id)
                .map_err(map_preparation_state_error)?;
            let state = CorePreparationState {
                network_id: state.network_id,
                account_sequence: state.account_sequence,
                protocol_timestamp: state.protocol_timestamp,
                observed_head_sequence: state.observed_head_sequence,
                module_registry: state.module_registry,
            };
            self.last_state = Some(state.clone());
            Ok(state)
        }
    }

    fn map_preparation_state_error(
        error: layerx_client::lni::PreparationStateError,
    ) -> CoreStateError {
        use layerx_client::lni::PreparationStateError;
        match error {
            PreparationStateError::Transport(_)
            | PreparationStateError::UnavailableCapability
            | PreparationStateError::Disconnected => CoreStateError::Unavailable,
            PreparationStateError::CoreRefusal { class, result } => {
                CoreStateError::Refused { class, result }
            }
            PreparationStateError::Envelope(_)
            | PreparationStateError::InvalidCorrelation
            | PreparationStateError::InterfaceVersion(_)
            | PreparationStateError::MalformedRequest
            | PreparationStateError::MalformedResponse
            | PreparationStateError::ActorMismatch
            | PreparationStateError::Network { .. }
            | PreparationStateError::StaleSnapshot { .. } => CoreStateError::Unverified,
        }
    }

    /// Explicit configured defaults and hard preparation bounds.
    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub struct PreparationDefaults {
        pub timestamp_span: u64,
        pub fee_limit: Amount,
        pub maximum_payload_bytes: usize,
    }

    /// Complete caller input. Every omitted bounded field has one explicit default.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct PrepareRequest {
        pub actor: Did,
        pub authority: Authority,
        pub activity_type: ActivityType,
        pub expected_account_sequence: Option<u64>,
        pub timestamp_bound: Option<TimestampBound>,
        pub fee_limit: Option<Amount>,
        pub idempotency_key: IdempotencyKey,
        pub payload: Vec<u8>,
        pub declared_payload_limit: usize,
    }

    /// Exact typed envelope plus the only canonical bytes and signer preimage.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct Prepared {
        pub envelope: UnsignedEnvelope,
        pub canonical_bytes: Vec<u8>,
        pub signing_preimage: [u8; 32],
        pub observed_head_sequence: u64,
        pub disclosure: layerx_crypto::disclosure::Disclosure,
        pub disclosure_digest: DisclosureDigest,
        pub audit: PreparationAuditEntry,
    }

    /// Preparation audit evidence emitted with every response.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct PreparationAuditEntry {
        pub idempotency_key: [u8; 32],
        pub observed_head_sequence: u64,
        pub disclosure_digest: DisclosureDigest,
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub enum PrepareError {
        Core(CoreStateError),
        StaleSequence { expected: u64, core: u64 },
        InvalidDefaults,
        TimestampDoesNotContainCore,
        TimestampBoundWidened,
        PayloadLimitExceeded { actual: usize, maximum: usize },
        Payload(PayloadError),
        Activity(ActivityBuildError),
        Wire(WireError),
        Disclosure(DisclosureBindingError),
    }

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub enum RestorePreparationError {
        MissingAuthority,
        AuthorityMismatch,
        Actor(layerx_types::ids::LengthError),
        Payload(PayloadError),
        PayloadHashMismatch,
        Activity(ActivityBuildError),
        Wire(WireError),
        Disclosure(DisclosureBindingError),
        CanonicalMismatch,
    }

    pub fn restore_canonical(
        canonical_bytes: &[u8],
        observed_head_sequence: u64,
        registry: &ModuleRegistry,
        retained_authority: Option<&Authority>,
    ) -> Result<Prepared, RestorePreparationError> {
        let authority = retained_authority.ok_or(RestorePreparationError::MissingAuthority)?;
        let activity = layerx_wire::activity::decode_unsigned(canonical_bytes, registry)
            .map_err(RestorePreparationError::Wire)?;
        if authority.as_bytes() != activity.authority() {
            return Err(RestorePreparationError::AuthorityMismatch);
        }
        let actor = Did::new(activity.actor_did()).map_err(RestorePreparationError::Actor)?;
        let payload = Payload::new(registry, activity.activity_type(), activity.payload())
            .map_err(RestorePreparationError::Payload)?;
        let payload_hash = payload_hash_for(&payload).map_err(RestorePreparationError::Wire)?;
        if payload_hash != activity.payload_hash() {
            return Err(RestorePreparationError::PayloadHashMismatch);
        }
        let bound = activity.timestamp_bound();
        let bound = TimestampBound::new(bound.not_before, bound.not_after)
            .map_err(RestorePreparationError::Activity)?;
        let mut builder = EnvelopeBuilder::new();
        builder
            .protocol_version(activity.protocol_version())
            .and_then(|builder| builder.network_id(activity.network_id()))
            .and_then(|builder| builder.activity_type(activity.activity_type()))
            .and_then(|builder| builder.actor_did(actor))
            .and_then(|builder| builder.authority(authority.clone()))
            .and_then(|builder| builder.account_sequence(activity.account_sequence()))
            .and_then(|builder| builder.timestamp_bound(bound))
            .and_then(|builder| {
                builder.idempotency_key(IdempotencyKey::new(activity.idempotency_key()))
            })
            .and_then(|builder| builder.fee_limit(Amount::from_u128(activity.fee_limit())))
            .and_then(|builder| builder.payload_hash(payload_hash))
            .and_then(|builder| builder.payload(payload))
            .map_err(RestorePreparationError::Activity)?;
        let envelope = builder.build().map_err(RestorePreparationError::Activity)?;
        let reproduced =
            encode_unsigned_envelope(&envelope).map_err(RestorePreparationError::Wire)?;
        if reproduced != canonical_bytes {
            return Err(RestorePreparationError::CanonicalMismatch);
        }
        let disclosed = disclosure_binding::decode_and_bind(canonical_bytes, registry, None)
            .map_err(RestorePreparationError::Disclosure)?;
        let signing_preimage = *preimage_unsigned(&envelope)
            .map_err(RestorePreparationError::Wire)?
            .as_bytes();
        let audit = PreparationAuditEntry {
            idempotency_key: envelope.idempotency_key().bytes(),
            observed_head_sequence,
            disclosure_digest: disclosed.digest,
        };
        Ok(Prepared {
            envelope,
            canonical_bytes: reproduced,
            signing_preimage,
            observed_head_sequence,
            disclosure: disclosed.disclosure,
            disclosure_digest: disclosed.digest,
            audit,
        })
    }

    /// Decodes a structured disclosure from canonical prepared bytes.
    ///
    /// # Errors
    ///
    /// Returns the disclosure decode failure, or a mismatch when the decoded disclosure does not
    /// re-encode to the exact canonical bytes.
    pub fn disclose(
        canonical_bytes: &[u8],
        registry: &ModuleRegistry,
    ) -> Result<DisclosedPreparation, DisclosureBindingError> {
        disclosure_binding::decode_and_bind(canonical_bytes, registry, None)
    }

    /// Revalidates that a held preparation still matches its disclosure.
    ///
    /// # Errors
    ///
    /// Returns a decode or re-encoding mismatch, or a digest that no longer matches the recorded
    /// preparation and audit digests.
    pub fn verify_disclosure_binding(prepared: &Prepared) -> Result<(), DisclosureBindingError> {
        disclosure_binding::verify_binding(prepared)
    }

    /// Expires every elapsed unsubmitted preparation and releases its reservations.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the lifecycle records cannot be locked, or the reservation refusal
    /// that prevented a release.
    pub fn expire(
        lifecycle: &PreparationLifecycle,
        limiter: &crate::budget::BudgetLimiter,
        core_batch_time_ms: u64,
        current_sequence: u64,
    ) -> Result<ExpirationReport, LifecycleError> {
        lifecycle::expire_elapsed(lifecycle, limiter, core_batch_time_ms, current_sequence)
    }

    /// Discards terminal signed bytes while preserving every unresolved submission.
    ///
    /// # Errors
    ///
    /// Returns `Unavailable` when the lifecycle records cannot be locked.
    pub fn retention_sweep(
        lifecycle: &PreparationLifecycle,
        current_sequence: u64,
        retention_sequences: u64,
    ) -> Result<RetentionReport, LifecycleError> {
        lifecycle::sweep_retention(lifecycle, current_sequence, retention_sequences)
    }

    /// Constructs and canonically encodes an unsigned activity from core state.
    ///
    /// # Errors
    ///
    /// Refuses zero-valued defaults, unavailable or unverified core state, a stale account sequence, a
    /// timestamp bound that excludes core time or widens past the configured span, and an oversized
    /// payload. Also returns the payload, envelope, wire, or disclosure failure raised while building
    /// the canonical form.
    pub fn prepare_activity(
        boundary: &mut dyn CorePreparationBoundary,
        defaults: PreparationDefaults,
        request: PrepareRequest,
    ) -> Result<Prepared, PrepareError> {
        prepare_activity_for_protocol(boundary, defaults, request, PROTOCOL_VERSION)
    }

    /// Prepares an activity for the explicitly selected node protocol.
    ///
    /// # Errors
    ///
    /// Returns the same preparation errors as `prepare_activity`, including unsupported wire versions.
    pub fn prepare_activity_for_protocol(
        boundary: &mut dyn CorePreparationBoundary,
        defaults: PreparationDefaults,
        request: PrepareRequest,
        protocol_version: u16,
    ) -> Result<Prepared, PrepareError> {
        prepare_bound(boundary, defaults, request, protocol_version, None)
    }

    /// Prepares a core budget fund, defund or revoke whose disclosure is bound to the verified
    /// budget state; the context stays in the cached disclosure, so `verify_disclosure_binding`
    /// re-derives the same digest at sign and submit.
    ///
    /// # Errors
    ///
    /// Returns the same preparation errors as `prepare_activity_for_protocol`, and a disclosure
    /// failure when the payload is not a budget mutation consistent with `context`.
    pub fn prepare_budget_mutation_for_protocol(
        boundary: &mut dyn CorePreparationBoundary,
        defaults: PreparationDefaults,
        request: PrepareRequest,
        protocol_version: u16,
        context: &layerx_crypto::disclosure::BudgetStateContext,
    ) -> Result<Prepared, PrepareError> {
        prepare_bound(boundary, defaults, request, protocol_version, Some(context))
    }

    fn prepare_bound(
        boundary: &mut dyn CorePreparationBoundary,
        defaults: PreparationDefaults,
        request: PrepareRequest,
        protocol_version: u16,
        budget_context: Option<&layerx_crypto::disclosure::BudgetStateContext>,
    ) -> Result<Prepared, PrepareError> {
        if defaults.timestamp_span == 0 || defaults.maximum_payload_bytes == 0 {
            return Err(PrepareError::InvalidDefaults);
        }
        let state = boundary
            .preparation_state(&request.actor)
            .map_err(PrepareError::Core)?;
        if let Some(expected) = request.expected_account_sequence {
            if expected != state.account_sequence {
                return Err(PrepareError::StaleSequence {
                    expected,
                    core: state.account_sequence,
                });
            }
        }
        let timestamp_bound = match request.timestamp_bound {
            Some(bound) => {
                if bound.not_before() > state.protocol_timestamp
                    || bound.not_after() < state.protocol_timestamp
                {
                    return Err(PrepareError::TimestampDoesNotContainCore);
                }
                if bound.not_after().saturating_sub(bound.not_before()) > defaults.timestamp_span {
                    return Err(PrepareError::TimestampBoundWidened);
                }
                bound
            }
            None => TimestampBound::new(
                state.protocol_timestamp,
                state
                    .protocol_timestamp
                    .saturating_add(defaults.timestamp_span),
            )
            .map_err(PrepareError::Activity)?,
        };
        let maximum_payload = defaults
            .maximum_payload_bytes
            .min(request.declared_payload_limit);
        if request.payload.len() > maximum_payload {
            return Err(PrepareError::PayloadLimitExceeded {
                actual: request.payload.len(),
                maximum: maximum_payload,
            });
        }
        let payload = Payload::new(
            &state.module_registry,
            request.activity_type,
            &request.payload,
        )
        .map_err(PrepareError::Payload)?;
        let payload_hash = payload_hash_for(&payload).map_err(PrepareError::Wire)?;
        let mut builder = EnvelopeBuilder::new();
        builder
            .protocol_version(protocol_version)
            .and_then(|builder| builder.network_id(state.network_id))
            .and_then(|builder| builder.activity_type(request.activity_type))
            .and_then(|builder| builder.actor_did(request.actor))
            .and_then(|builder| builder.authority(request.authority))
            .and_then(|builder| builder.account_sequence(state.account_sequence))
            .and_then(|builder| builder.timestamp_bound(timestamp_bound))
            .and_then(|builder| builder.idempotency_key(request.idempotency_key))
            .and_then(|builder| builder.fee_limit(request.fee_limit.unwrap_or(defaults.fee_limit)))
            .and_then(|builder| builder.payload_hash(payload_hash))
            .and_then(|builder| builder.payload(payload))
            .map_err(PrepareError::Activity)?;
        let envelope = builder.build().map_err(PrepareError::Activity)?;
        let canonical_bytes = encode_unsigned_envelope(&envelope).map_err(PrepareError::Wire)?;
        let signing_preimage = *preimage_unsigned(&envelope)
            .map_err(PrepareError::Wire)?
            .as_bytes();
        let disclosed = disclosure_binding::decode_and_bind(
            &canonical_bytes,
            &state.module_registry,
            budget_context,
        )
        .map_err(PrepareError::Disclosure)?;
        let audit = PreparationAuditEntry {
            idempotency_key: envelope.idempotency_key().bytes(),
            observed_head_sequence: state.observed_head_sequence,
            disclosure_digest: disclosed.digest,
        };
        Ok(Prepared {
            envelope,
            canonical_bytes,
            signing_preimage,
            observed_head_sequence: state.observed_head_sequence,
            disclosure: disclosed.disclosure,
            disclosure_digest: disclosed.digest,
            audit,
        })
    }

    #[cfg(test)]
    #[path = "restore_tests.rs"]
    mod restore_tests;
}

pub(crate) use legacy::PreparationAuthorization;
pub use legacy::*;
