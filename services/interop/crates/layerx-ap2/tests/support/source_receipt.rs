use layerx_ap2::{Ap2Error, AuthorizedPayment, ExecutedPayment, LayerXIntentPlane, PlaneOutcome};
use layerx_intents::{compile, CompiledIntent, Intent, IntentKind, LxpSend};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::trace::TraceId;
use layerx_portable::PortableReceiptV2;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

pub struct IntentKey {
    pub public_key: [u8; 32],
    authorizations: Vec<([u8; 32], [u8; 64])>,
}

impl IntentKey {
    ///
    /// # Panics
    /// Panics when a recorded value fails its required checks.
    #[must_use]
    pub fn new(case: &Value) -> Self {
        Self {
            public_key: array(&case["intentPublicKey"]),
            authorizations: case["intentAuthorizations"]
                .as_array()
                .unwrap_or_else(|| panic!("producer-signed intent authorizations"))
                .iter()
                .map(|entry| (array(&entry["input"]), array(&entry["signature"])))
                .collect(),
        }
    }

    ///
    /// # Panics
    /// Panics when a recorded value fails its required checks.
    #[must_use]
    pub fn sign(&self, message: &[u8]) -> [u8; 64] {
        self.authorizations
            .iter()
            .find(|(input, _)| input.as_slice() == message)
            .unwrap_or_else(|| {
                panic!(
                    "{}",
                    "unchanged producer-signed authorization for this exact request"
                )
            })
            .1
    }
}

pub struct RecordedPlane {
    pub case: Value,
    pub executions: usize,
    intent: Option<Intent>,
}

impl RecordedPlane {
    #[must_use]
    pub fn new(case: Value) -> Self {
        Self {
            case,
            executions: 0,
            intent: None,
        }
    }
}

impl LayerXIntentPlane for RecordedPlane {
    fn compile(
        &mut self,
        payment: &AuthorizedPayment,
        _: &TraceId,
    ) -> Result<CompiledIntent, Ap2Error> {
        let _ = assert_mandate_reference(&self.case, &evidence(&self.case), payment);
        let key = IntentKey::new(&self.case);
        let send = LxpSend::new(
            AccountId::parse("agent:buyer:main")
                .unwrap_or_else(|error| panic!("payer account: {error:?}")),
            AccountId::parse("agent:merchant:main")
                .unwrap_or_else(|error| panic!("payee account: {error:?}")),
            AssetId::new(payment.asset()),
            Amount::from_u128(payment.amount()),
            Sequence::from_u64(0),
            IdempotencyKey::new(payment.idempotency_key()),
            TimestampSeconds::from_u64(1_700_001_800),
            ContextHash::new(payment.request_digest()),
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new(key.public_key),
                AuthorizationSignature::new(key.sign(&payment.request_digest())),
            ),
            NetworkId::new(125).unwrap_or_else(|error| panic!("network: {error:?}")),
            ProtocolVersion::new(2).unwrap_or_else(|error| panic!("protocol: {error:?}")),
        )
        .map_err(|_| Ap2Error::IntentCompilation)?;
        let activity = ActivityType::new(ModuleId::Asset, 5)
            .unwrap_or_else(|error| panic!("asset send: {error:?}"));
        let registry =
            ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[activity])
                .unwrap_or_else(|error| panic!("asset registration: {error:?}"))])
            .unwrap_or_else(|error| panic!("registry: {error:?}"));
        let intent = Intent::v1(IntentKind::LxpSend(send));
        let compiled = compile(&intent, &registry).map_err(|_| Ap2Error::IntentCompilation)?;
        compiled
            .verify_disclosure(&intent)
            .unwrap_or_else(|error| panic!("real compiler disclosure: {error:?}"));
        assert_eq!(
            compiled.payload().as_bytes(),
            bytes(&self.case["compiledPayload"])
        );
        assert_eq!(
            compiled.payload_hash(),
            array(&self.case["compiledPayloadHash"])
        );
        self.intent = Some(intent);
        Ok(compiled)
    }

    fn execute(
        &mut self,
        payment: &AuthorizedPayment,
        compiled: CompiledIntent,
        _: &TraceId,
    ) -> Result<PlaneOutcome, Ap2Error> {
        compiled
            .verify_disclosure(
                self.intent
                    .as_ref()
                    .unwrap_or_else(|| panic!("compile precedes execution")),
            )
            .unwrap_or_else(|error| panic!("unchanged compiled intent at the consumer: {error:?}"));
        let evidence = evidence(&self.case);
        let _ = assert_mandate_reference(&self.case, &evidence, payment);
        self.executions += 1;
        Ok(PlaneOutcome::Executed(Box::new(ExecutedPayment {
            evidence,
            order_id: payment.checkout_id().to_owned(),
        })))
    }
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn bytes(value: &Value) -> Vec<u8> {
    hex::decode(
        value
            .as_str()
            .unwrap_or_else(|| panic!("fixture hex"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex prefix")),
    )
    .unwrap_or_else(|error| panic!("fixture bytes: {error:?}"))
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|error| panic!("fixture field width: {error:?}"))
}

#[must_use]
pub fn case() -> Value {
    named_case("settlement")
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn named_case(name: &str) -> Value {
    let fixture: Value =
        serde_json::from_str(include_str!("../fixtures/ap2-mandate-pay-exact.json"))
            .unwrap_or_else(|error| panic!("node-produced fixture: {error:?}"));
    assert_eq!(
        fixture["provenance"]["generator"],
        "daemon/committed-fixture"
    );
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("cases"))
        .iter()
        .find(|case| case["id"] == name)
        .unwrap_or_else(|| panic!("node-produced payExact"))
        .clone()
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(&case["receiptJson"]["txHash"]),
        signed_receipt: bytes(&case["signedReceipt"]),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .unwrap_or_else(|| panic!("logs"))
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(&log["address"]),
                    log["topics"]
                        .as_array()
                        .unwrap_or_else(|| panic!("topics"))
                        .iter()
                        .map(array)
                        .collect(),
                    bytes(&log["data"]),
                    quantity(&log["logIndex"]),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn payment_log(evidence: &SequencerEvidence) -> &SequencerLog {
    let topic: [u8; 32] =
        hex::decode("c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1")
            .unwrap_or_else(|error| panic!("PaymentExact topic: {error:?}"))
            .try_into()
            .unwrap_or_else(|error| panic!("topic width: {error:?}"));
    let mut payments = evidence
        .logs
        .iter()
        .filter(|log| log.topics().first() == Some(&topic));
    let payment = payments
        .next()
        .unwrap_or_else(|| panic!("node PaymentExact event"));
    assert!(payments.next().is_none(), "exactly one PaymentExact event");
    payment
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn pin(case: &Value) -> SequencerAuthorization {
    let key = &case["pinnedKey"];
    SequencerAuthorization::new(
        array(&key["keyId"]),
        array(&key["publicKey"]),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )
}

fn quantity(value: &Value) -> u64 {
    u64::from_str_radix(
        value
            .as_str()
            .unwrap_or_else(|| panic!("quantity"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("quantity prefix")),
        16,
    )
    .unwrap_or_else(|error| panic!("quantity value: {error:?}"))
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn calldata(case: &Value) -> [[u8; 32]; 6] {
    let raw = bytes(&case["rawTransaction"]);
    let offsets: Vec<_> = raw
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one recorded payExact selector");
    let start = offsets[0] + 4;
    std::array::from_fn(|word| {
        raw[start + word * 32..start + (word + 1) * 32]
            .try_into()
            .unwrap_or_else(|error| panic!("payExact word: {error:?}"))
    })
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn assert_source(case: &Value, evidence: &SequencerEvidence) -> [u8; 32] {
    assert_eq!(
        evidence,
        &self::evidence(case),
        "keep all node-produced evidence unchanged"
    );
    assert_eq!(
        evidence.signed_receipt,
        bytes(&case["receiptJson"]["receipt"])
    );
    let pins = [pin(case)];
    let raw = bytes(&case["rawTransaction"]);
    let portable =
        PortableReceiptV2::from_sequencer_receipt(&case["receiptJson"], &pins, Some(&raw))
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "node transaction and receipt must share their original identity"
                )
            });
    let verified = portable
        .verify_v2(&pins)
        .unwrap_or_else(|error| panic!("pinned portable evidence: {error:?}"));
    let recorded = PortableReceiptV2::from_json(
        &serde_json::to_vec(&case["portableEvidence"])
            .unwrap_or_else(|error| panic!("recorded portable bytes: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("producer-exported portable evidence: {error:?}"));
    assert_eq!(
        portable, recorded,
        "retain the production encoder's original export"
    );
    assert_eq!(verified.raw_transaction(), Some(raw.as_slice()));
    assert_eq!(verified.signed_receipt(), evidence.signed_receipt);
    assert_eq!(verified.activity_id(), evidence.transaction_hash);
    let digest = array(&case["receiptDigest"]);
    assert_eq!(verified.receipt_digest(), digest);
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(digest)
    );
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, &pins)
        .unwrap_or_else(|error| panic!("original signature: {error:?}"));
    verify_logs(&receipt, evidence.logs.clone())
        .unwrap_or_else(|error| panic!("original signed log commitments: {error:?}"));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"));
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(
        protocol.global_sequence(),
        quantity(&case["receiptJson"]["seq"])
    );
    assert_eq!(protocol.previous_state_root(), array(&case["prevSeqHash"]));
    let mut sequence = Sha256::new();
    sequence.update(b"PAXEER/v1/sequence\0");
    sequence.update(protocol.previous_state_root());
    sequence.update(protocol.global_sequence().to_be_bytes());
    sequence.update(protocol.activity_id());
    let sequence: [u8; 32] = sequence.finalize().into();
    assert_eq!(
        protocol.resulting_state_root(),
        sequence,
        "original transaction sequence chain"
    );
    assert_eq!(
        evidence.logs.len(),
        2,
        "the node emits Transfer and PaymentExact"
    );
    let words = calldata(case);
    let transfer = &evidence.logs[0];
    assert_eq!(&transfer.topics()[1..], &[words[0], words[1], words[2]]);
    let mut reason = [0_u8; 32];
    reason[31] = 5;
    assert_eq!(transfer.data(), [words[3], reason].concat());
    let log = payment_log(evidence);
    assert_eq!(&log.topics()[1..], &[words[5], words[0], words[1]]);
    assert_eq!(log.data(), [words[2], words[3]].concat());
    digest
}

///
/// # Panics
/// Panics when a recorded value fails its required checks.
#[must_use]
pub fn assert_mandate_reference(
    case: &Value,
    evidence: &SequencerEvidence,
    payment: &AuthorizedPayment,
) -> [u8; 32] {
    let digest = assert_source(case, evidence);
    assert_eq!(calldata(case)[5], payment.idempotency_key(),
        "a node-produced payExact fixture for this mandate is required; never rewrite or re-sign a receipt");
    digest
}
