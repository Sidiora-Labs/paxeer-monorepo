use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    committed::verify_committed,
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    CommittedProof,
};
use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_x402::facilitator::{
    Facilitator, FacilitatorCommitment, FacilitatorKind, FacilitatorRequest,
    SequencerFacilitatorPlane, SettlementIdentity, SettlementStep, SupportedResponse,
};
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, X402Error, X402_VERSION,
};
use serde_json::{json, Value};

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn pay_exact_words(raw: &[u8]) -> [[u8; 32]; 6] {
    fn take<'a>(input: &mut &'a [u8]) -> &'a [u8] {
        let prefix = input[0];
        let (offset, length) = match prefix {
            0x00..=0x7f => (0, 1),
            0x80..=0xb7 => (1, usize::from(prefix - 0x80)),
            0xb8..=0xbf | 0xf8..=0xff => {
                let width = usize::from(if prefix < 0xc0 {
                    prefix - 0xb7
                } else {
                    prefix - 0xf7
                });
                let length = input[1..1 + width]
                    .iter()
                    .fold(0_usize, |length, byte| (length << 8) | usize::from(*byte));
                (1 + width, length)
            }
            0xc0..=0xf7 => (1, usize::from(prefix - 0xc0)),
        };
        let value = &input[offset..offset + length];
        *input = &input[offset + length..];
        value
    }
    let (mut envelope, data_index) = match raw[0] {
        0x02 => (&raw[1..], 7),
        0xc0..=0xff => (raw, 5),
        _ => panic!("unsupported recorded transaction envelope"),
    };
    let mut fields = take(&mut envelope);
    assert!(envelope.is_empty());
    for _ in 0..data_index {
        take(&mut fields);
    }
    let calldata = take(&mut fields);
    assert_eq!(calldata.len(), 4 + 6 * 32);
    assert_eq!(&calldata[..4], &[0x78, 0xea, 0x11, 0xdb]);
    std::array::from_fn(|index| {
        calldata[4 + index * 32..4 + (index + 1) * 32]
            .try_into()
            .unwrap()
    })
}

fn recorded_case() -> Value {
    let fixture: Value =
        serde_json::from_str(FIXTURE).unwrap_or_else(|error| panic!("recorded fixture: {error}"));
    fixture["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("fixture cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap_or_else(|| panic!("recorded PaymentExact case"))
        .clone()
}

fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("fixture string"))
}

fn bytes(value: &str) -> Vec<u8> {
    let digits = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix"));
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    bytes(value)
        .try_into()
        .unwrap_or_else(|_| panic!("expected {N} bytes"))
}

fn recorded_evidence(case: &Value) -> SequencerEvidence {
    SequencerEvidence {
        transaction_hash: array(text(&case["receiptJson"]["txHash"])),
        signed_receipt: bytes(text(&case["signedReceipt"])),
        logs: case["receiptJson"]["logs"]
            .as_array()
            .unwrap_or_else(|| panic!("logs"))
            .iter()
            .map(|log| {
                SequencerLog::new(
                    array(text(&log["address"])),
                    log["topics"]
                        .as_array()
                        .unwrap_or_else(|| panic!("topics"))
                        .iter()
                        .map(|topic| array(text(topic)))
                        .collect(),
                    bytes(text(&log["data"])),
                    u64::from_str_radix(text(&log["logIndex"]).trim_start_matches("0x"), 16)
                        .unwrap_or_else(|error| panic!("log index: {error}")),
                )
            })
            .collect(),
        rung: SettlementRung::Receipted,
    }
}

fn receipt_keys(case: &Value) -> Vec<SequencerAuthorization> {
    let key = &case["pinnedKey"];
    vec![SequencerAuthorization::new(
        array(text(&key["keyId"])),
        array(text(&key["publicKey"])),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )]
}

fn request(transaction: &str) -> FacilitatorRequest {
    let account = "agent:did:layerx:payee:main";
    let pay_to = account_identifiers(account).unwrap_or_else(|error| panic!("merchant: {error}"))
        [1]
    .iter()
    .map(|byte| format!("{byte:02x}"))
    .collect::<String>();
    let requirements = PaymentRequirements {
        scheme: "exact".to_owned(),
        network: "eip155:125".to_owned(),
        amount: AtomicAmount::from_u128(1_000_000),
        asset: "0x4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28".to_owned(),
        pay_to,
        max_timeout_seconds: 60,
        extra: Some(json!({"layerx": {"account": account, "currency": "LXP"}})),
    };
    FacilitatorRequest {
        x402_version: X402_VERSION,
        payment_payload: PaymentPayload {
            x402_version: X402_VERSION,
            resource: None,
            payload: json!({"transaction": transaction}),
            accepted: requirements.clone(),
            extensions: BTreeMap::new(),
        },
        payment_requirements: requirements,
    }
}

fn context() -> (Facilitator, GatewayCore, PrincipalId, TraceId) {
    let facilitator = Facilitator::new(SupportedResponse {
        kinds: vec![FacilitatorKind {
            x402_version: X402_VERSION,
            scheme: "exact".to_owned(),
            network: "eip155:125".to_owned(),
            extra: None,
        }],
        extensions: Vec::new(),
        signers: BTreeMap::from([("eip155:*".to_owned(), vec!["sequencer".to_owned()])]),
    })
    .unwrap_or_else(|error| panic!("facilitator: {error}"));
    let mut gateway = GatewayCore::new();
    let principal = PrincipalId::new("buyer").unwrap_or_else(|error| panic!("principal: {error}"));
    let trace = TraceId::mint([0x11; 16]);
    let adapter = AdapterId::new("x402").unwrap_or_else(|error| panic!("adapter: {error}"));
    let version = SpecVersion::parse("2.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter.clone(), version, [0x51; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let suite = ConformanceSuite::new(
        AdapterId::new("x402-v2-vectors").unwrap_or_else(|error| panic!("suite id: {error}")),
        96,
        [0x52; 32],
    )
    .unwrap_or_else(|error| panic!("suite: {error}"));
    gateway
        .register_adapter(AdapterDescriptor::new(adapter, spec, suite), &trace, 0)
        .unwrap_or_else(|error| panic!("register: {error}"));
    (facilitator, gateway, principal, trace)
}

fn replace(receipt: &mut [u8], old: &[u8], new: &[u8]) {
    assert_eq!(old.len(), new.len());
    let positions: Vec<_> = receipt
        .windows(old.len())
        .enumerate()
        .filter_map(|(index, value)| (value == old).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    receipt[positions[0]..positions[0] + old.len()].copy_from_slice(new);
}

#[test]
fn facilitator_settles_receipted_evidence() {
    let case = recorded_case();
    let transaction = text(&case["receiptJson"]["txHash"]);
    let request = request(transaction);
    let mut evidence = recorded_evidence(&case);
    let recorded_keys = receipt_keys(&case);
    let recorded_receipt =
        verify_sequencer_signature(&evidence.signed_receipt, recorded_keys.as_slice())
            .unwrap_or_else(|error| panic!("recorded signature: {error:?}"));
    assert!(verify_logs(&recorded_receipt, evidence.logs.clone()).is_ok());
    assert_eq!(
        canonical_receipt_digest(&evidence.signed_receipt),
        Ok(array(text(&case["receiptDigest"])))
    );

    let (facilitator, mut gateway, principal, trace) = context();
    let stable = [0x42; 32];
    let identity = SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // Preserve the recorded txHash, amount and asset. Sign the payment for
    // this request's account profile and derived settlement identity, which
    // differ from the standalone conformance vector's payee and payment key.
    let log = &evidence.logs[0];
    let mut topics = log.topics().to_vec();
    let payee = array(&format!("0x{}", request.payment_requirements.pay_to));
    let recorded_topics = topics.concat();
    let words = pay_exact_words(&bytes(text(&case["rawTransaction"])));
    topics[1..].copy_from_slice(&[words[5], words[0], words[1]]);
    topics[1] = identity.idempotency_key;
    topics[3] = payee;
    replace(
        &mut evidence.signed_receipt,
        &recorded_topics,
        &topics.concat(),
    );
    evidence.logs = vec![SequencerLog::new(
        log.address(),
        topics,
        log.data().to_vec(),
        log.log_index(),
    )];
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let receipt_digest = canonical_receipt_digest(&evidence.signed_receipt)
        .unwrap_or_else(|error| panic!("receipt digest: {error}"));
    let signature_start = evidence.signed_receipt.len() - 64;
    evidence.signed_receipt[signature_start..]
        .copy_from_slice(&signer.sign(&receipt_digest).to_bytes());
    let keys = vec![SequencerAuthorization::new(
        array(text(&case["pinnedKey"]["keyId"])),
        signer.verifying_key().to_bytes(),
        1000,
        1999,
    )];
    let mut plane = SequencerFacilitatorPlane::new(evidence.clone(), keys.clone());
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            stable,
            SettlementStep::Single,
            &mut plane,
            &trace,
            1,
        )
        .unwrap_or_else(|error| panic!("recorded transaction settlement: {error}"));
    assert!(response.success);
    assert_eq!(response.transaction, transaction);
    assert_eq!(
        response.payer.as_deref(),
        Some(
            words[0]
                .iter()
                .map(|byte| format!("{byte:02x}"))
                .collect::<String>()
                .as_str()
        )
    );
    assert_eq!(response.amount, Some(request.payment_requirements.amount));
    assert_eq!(
        response.extensions["layerx"]["verificationLevel"],
        "receipted"
    );
    assert_eq!(
        response.extensions["layerx"]["receipt"],
        STANDARD.encode(&evidence.signed_receipt)
    );
    assert_eq!(
        array::<32>(&format!(
            "0x{}",
            text(&response.extensions["layerx"]["receiptDigest"])
        )),
        receipt_digest
    );
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );
    assert_eq!(response.validate_wire(), Ok(()));

    // Untrusted pins, tampering and an unrelated payment key must never
    // produce a successful settlement or advance the gateway record.
    let mut corrupt = evidence.clone();
    corrupt.signed_receipt[signature_start] ^= 1;
    let mut legacy = evidence.clone();
    let log = &legacy.logs[0];
    let native_topics = log.topics().to_vec();
    let mut topics = native_topics.clone();
    topics[1..].copy_from_slice(&[words[0], payee, identity.idempotency_key]);
    replace(
        &mut legacy.signed_receipt,
        &native_topics.concat(),
        &topics.concat(),
    );
    legacy.logs = vec![SequencerLog::new(
        log.address(),
        topics,
        log.data().to_vec(),
        log.log_index(),
    )];
    let digest = canonical_receipt_digest(&legacy.signed_receipt)
        .unwrap_or_else(|error| panic!("legacy receipt digest: {error}"));
    legacy.signed_receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let verified = verify_sequencer_signature(&legacy.signed_receipt, keys.as_slice())
        .unwrap_or_else(|error| panic!("legacy signature: {error:?}"));
    assert!(verify_logs(&verified, legacy.logs.clone()).is_ok());
    for (evidence, keys) in [
        (evidence.clone(), recorded_keys),
        (corrupt, keys.clone()),
        (legacy, keys.clone()),
        (recorded_evidence(&case), receipt_keys(&case)),
    ] {
        let (facilitator, mut gateway, principal, trace) = context();
        let mut plane = SequencerFacilitatorPlane::new(evidence, keys);
        assert!(facilitator
            .settle(
                &mut gateway,
                &principal,
                &request,
                stable,
                SettlementStep::Single,
                &mut plane,
                &trace,
                1
            )
            .is_err());
        assert_eq!(
            gateway.translation(&principal, identity.idempotency_key),
            Some(TranslationStatus::Pending)
        );
    }
}

#[test]
fn facilitator_pending_reports_the_relayed_hash() {
    let case = recorded_case();
    let transaction = text(&case["receiptJson"]["txHash"]);
    let request = request(transaction);
    let mut evidence = recorded_evidence(&case);
    evidence.rung = SettlementRung::Submitted;
    evidence.signed_receipt.clear();
    evidence.logs.clear();
    let mut plane = SequencerFacilitatorPlane::new(evidence, Vec::new());
    let (facilitator, mut gateway, principal, trace) = context();
    let identity =
        SettlementIdentity::derive(&principal, &request, [0x42; 32], SettlementStep::Single)
            .unwrap_or_else(|error| panic!("identity: {error}"));
    let response = facilitator
        .settle(
            &mut gateway,
            &principal,
            &request,
            [0x42; 32],
            SettlementStep::Single,
            &mut plane,
            &trace,
            1,
        )
        .unwrap_or_else(|error| panic!("pending settlement: {error}"));
    assert!(!response.success);
    assert_eq!(response.error_reason.as_deref(), Some("settlement_pending"));
    assert_eq!(response.transaction, transaction);
    assert_ne!(array::<32>(&response.transaction), identity.request_digest);
    assert_ne!(array::<32>(&response.transaction), identity.idempotency_key);
    assert_eq!(
        response.extensions["layerx"]["verificationLevel"],
        "submitted"
    );
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );
    assert!(response.extensions["layerx"].get("receipt").is_none());
    assert_eq!(response.validate_wire(), Ok(()));

    let verified = facilitator
        .verify(&mut gateway, &principal, &request, &mut plane, &trace, 2)
        .unwrap_or_else(|error| panic!("payload verification: {error}"));
    assert!(verified.is_valid);
    assert!(verified.invalid_reason.is_none());
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::Pending)
    );

    for transaction in [
        "lxp:abc",
        "0xabc",
        "",
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    ] {
        let request = self::request(transaction);
        let response = facilitator
            .verify(&mut gateway, &principal, &request, &mut plane, &trace, 2)
            .unwrap_or_else(|error| panic!("transaction form refusal: {error}"));
        assert!(!response.is_valid);
        assert_eq!(response.invalid_reason.as_deref(), Some("transaction_form"));
    }
}

fn committed_string<'a>(value: &'a Value, field: &str) -> &'a str {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing string {field}"))
}

fn committed_number(value: &Value, field: &str) -> u64 {
    value[field]
        .as_u64()
        .unwrap_or_else(|| panic!("missing unsigned number {field}"))
}

fn committed_signed(value: &Value, field: &str) -> i64 {
    value[field]
        .as_i64()
        .unwrap_or_else(|| panic!("missing signed number {field}"))
}

fn committed_array<'a>(value: &'a Value, field: &str) -> &'a [Value] {
    value[field]
        .as_array()
        .unwrap_or_else(|| panic!("missing array {field}"))
}

fn committed_hex_bytes(hex: &str) -> Vec<u8> {
    let hex = hex
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("missing hex prefix"));
    assert!(hex.is_ascii());
    assert_eq!(hex.len() % 2, 0);
    (0..hex.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&hex[i..i + 2], 16)
                .unwrap_or_else(|error| panic!("invalid vector hex: {error}"))
        })
        .collect()
}

fn committed_bytes(value: &Value, field: &str) -> Vec<u8> {
    committed_hex_bytes(committed_string(value, field))
}

fn committed_fixed<const N: usize>(value: &Value, field: &str) -> [u8; N] {
    committed_bytes(value, field)
        .try_into()
        .unwrap_or_else(|_| panic!("incorrect byte length for {field}"))
}

fn committed_timestamp(value: &Value) -> Timestamp {
    Timestamp {
        seconds: committed_signed(value, "seconds"),
        nanos: committed_number(value, "nanos")
            .try_into()
            .unwrap_or_else(|_| panic!("timestamp nanos overflow")),
    }
}

fn committed_block_id(value: &Value) -> BlockId {
    BlockId {
        hash: committed_bytes(value, "hash"),
        part_set_header: PartSetHeader {
            total: committed_number(&value["parts"], "total")
                .try_into()
                .unwrap_or_else(|_| panic!("part count overflow")),
            hash: committed_bytes(&value["parts"], "hash"),
        },
    }
}

fn committed_header(value: &Value) -> Header {
    Header {
        version: ConsensusVersion {
            block: committed_number(&value["version"], "block"),
            app: committed_number(&value["version"], "app"),
        },
        chain_id: committed_string(value, "chain_id").to_owned(),
        height: committed_signed(value, "height"),
        time: committed_timestamp(&value["time"]),
        last_block_id: committed_block_id(&value["last_block_id"]),
        last_commit_hash: committed_bytes(value, "last_commit_hash"),
        data_hash: committed_bytes(value, "data_hash"),
        validators_hash: committed_fixed(value, "validators_hash"),
        next_validators_hash: committed_bytes(value, "next_validators_hash"),
        consensus_hash: committed_bytes(value, "consensus_hash"),
        app_hash: committed_bytes(value, "app_hash"),
        last_results_hash: committed_bytes(value, "last_results_hash"),
        evidence_hash: committed_bytes(value, "evidence_hash"),
        proposer_address: committed_bytes(value, "proposer_address"),
    }
}

fn committed_commit(value: &Value) -> Commit {
    Commit {
        height: committed_number(value, "height"),
        round: committed_signed(value, "round")
            .try_into()
            .unwrap_or_else(|_| panic!("commit round overflow")),
        block_id: committed_block_id(&value["block_id"]),
        signatures: committed_array(value, "signatures")
            .iter()
            .map(|vote| CommitSig {
                block_id_flag: match committed_number(vote, "block_id_flag") {
                    1 => BlockIdFlag::Absent,
                    2 => BlockIdFlag::Commit,
                    3 => BlockIdFlag::Nil,
                    flag => panic!("unknown block id flag {flag}"),
                },
                validator_address: committed_bytes(vote, "validator_address"),
                timestamp: committed_timestamp(&vote["timestamp"]),
                signature: if vote["signature"].is_null() {
                    None
                } else {
                    Some(committed_fixed(vote, "signature"))
                },
            })
            .collect(),
    }
}

fn committed_proof(case: &Value) -> CommittedProof {
    let batch = &case["batchProof"];
    assert_eq!(committed_string(batch, "leafEncoding"), "sha256-tx");
    assert_eq!(
        committed_bytes(batch, "rootHash"),
        committed_bytes(&case["executionHeader"], "data_hash")
    );
    CommittedProof {
        header: committed_header(&case["header"]),
        commit: committed_commit(&case["commit"]),
        execution_header: committed_header(&case["executionHeader"]),
        batch_transaction: committed_bytes(batch, "data"),
        batch_total: committed_number(batch, "total"),
        batch_index: committed_number(batch, "index"),
        batch_aunts: committed_array(batch, "aunts")
            .iter()
            .map(|aunt| {
                committed_hex_bytes(aunt.as_str().unwrap())
                    .try_into()
                    .unwrap()
            })
            .collect(),
        signed_receipt: committed_bytes(case, "signedReceipt"),
        activity_ids: vec![committed_fixed(&case["receiptJson"], "txHash")],
    }
}

#[test]
fn facilitator_reports_committed_only_with_a_proof() {
    let vectors: Value = serde_json::from_str(include_str!(
        "../../../specs/conformance/committed/vector-v4-committed.json"
    ))
    .unwrap_or_else(|error| panic!("committed fixture: {error}"));
    let case = committed_array(&vectors, "cases")
        .iter()
        .find(|case| case["id"] == "committed-payExact")
        .unwrap_or_else(|| panic!("recorded committed payment"));
    let mut proof = committed_proof(case);
    let validators: Vec<_> = committed_array(case, "validators")
        .iter()
        .map(|validator| Validator {
            pub_key: committed_fixed(validator, "pub_key"),
            voting_power: committed_signed(validator, "voting_power"),
        })
        .collect();
    let chain_id = text(&vectors["provenance"]["chainId"]).to_owned();
    let mut evidence = recorded_evidence(case);
    let recorded_keys = receipt_keys(case);
    let verified = verify_sequencer_signature(&evidence.signed_receipt, recorded_keys.as_slice())
        .unwrap_or_else(|error| panic!("recorded signature: {error:?}"));
    assert!(verify_logs(&verified, evidence.logs.clone()).is_ok());
    let recorded_digest = canonical_receipt_digest(&evidence.signed_receipt)
        .unwrap_or_else(|error| panic!("recorded digest: {error}"));
    let block = verify_committed(recorded_digest, &proof, &validators, &chain_id)
        .unwrap_or_else(|error| panic!("recorded commitment: {error}"));
    assert_eq!(block.number, 10);
    assert_eq!(block.hash, array(text(&case["receiptJson"]["blockHash"])));
    evidence.rung = SettlementRung::Committed {
        block_number: block.number,
        block_hash: block.hash,
    };

    let transaction = text(&case["receiptJson"]["txHash"]);
    let mut request = request(transaction);
    let log = &evidence.logs[1];
    request.payment_requirements.asset = format!(
        "0x{}",
        log.data()[..32]
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    request.payment_requirements.amount = AtomicAmount::from_u128(u128::from_be_bytes(
        log.data()[48..]
            .try_into()
            .unwrap_or_else(|_| panic!("payment amount")),
    ));
    request.payment_payload.accepted = request.payment_requirements.clone();
    let (_, _, principal, _) = context();
    let stable = [0x42; 32];
    let identity = SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single)
        .unwrap_or_else(|error| panic!("identity: {error}"));

    // Bind the recorded payment output to this facilitator request, preserving
    // the recorded activity, sequence chain, batch and consensus proof.
    let mut topics = log.topics().to_vec();
    let original_topics = topics.concat();
    let words = pay_exact_words(&bytes(text(&case["rawTransaction"])));
    topics[1..].copy_from_slice(&[words[5], words[0], words[1]]);
    topics[1] = identity.idempotency_key;
    topics[3] = array(&format!("0x{}", request.payment_requirements.pay_to));
    replace(
        &mut evidence.signed_receipt,
        &original_topics,
        &topics.concat(),
    );
    evidence.logs[1] =
        SequencerLog::new(log.address(), topics, log.data().to_vec(), log.log_index());
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let digest = canonical_receipt_digest(&evidence.signed_receipt)
        .unwrap_or_else(|error| panic!("bound receipt digest: {error}"));
    let signature_start = evidence.signed_receipt.len() - 64;
    evidence.signed_receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let keys = vec![SequencerAuthorization::new(
        array(text(&case["pinnedKey"]["keyId"])),
        signer.verifying_key().to_bytes(),
        block.number,
        block.number,
    )];
    proof.signed_receipt = evidence.signed_receipt.clone();
    let commitment = FacilitatorCommitment {
        proof,
        validators,
        chain_id,
    };

    for supplied in [None, Some(commitment.clone())] {
        let (facilitator, mut gateway, principal, trace) = context();
        let has_proof = supplied.is_some();
        let mut plane = SequencerFacilitatorPlane::new(evidence.clone(), keys.clone());
        if let Some(supplied) = supplied {
            plane = plane.with_committed_proof(supplied);
        }
        let response = facilitator
            .settle(
                &mut gateway,
                &principal,
                &request,
                stable,
                SettlementStep::Single,
                &mut plane,
                &trace,
                1,
            )
            .unwrap_or_else(|error| panic!("settlement: {error}"));
        assert!(response.success);
        assert_eq!(response.transaction, transaction);
        assert_eq!(response.amount, Some(request.payment_requirements.amount));
        let layerx = &response.extensions["layerx"];
        assert_eq!(layerx["receipt"], STANDARD.encode(&evidence.signed_receipt));
        assert_eq!(
            array::<32>(&format!("0x{}", text(&layerx["receiptDigest"]))),
            digest
        );
        if has_proof {
            assert_eq!(layerx["verificationLevel"], "committed");
            assert_eq!(layerx["blockNumber"], "0xa");
            assert_eq!(array::<32>(text(&layerx["blockHash"])), block.hash);
        } else {
            assert_eq!(layerx["verificationLevel"], "receipted");
            assert!(layerx.get("blockNumber").is_none());
            assert!(layerx.get("blockHash").is_none());
            let wire = serde_json::to_value(&response).unwrap();
            assert!(wire["extensions"]["layerx"].get("blockNumber").is_none());
            assert!(wire["extensions"]["layerx"].get("blockHash").is_none());
        }
        assert_eq!(response.validate_wire(), Ok(()));
        assert_eq!(
            gateway.translation(&principal, identity.idempotency_key),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: digest
            })
        );
    }

    let mut bad_aunt = commitment.clone();
    bad_aunt.proof.batch_aunts[0][0] ^= 1;
    let mut bad_signature = commitment.clone();
    bad_signature.proof.commit.signatures[0]
        .signature
        .as_mut()
        .unwrap()[0] ^= 1;
    let mut unrelated_receipt = commitment.clone();
    unrelated_receipt.proof.signed_receipt = bytes(text(&case["signedReceipt"]));
    let mut wrong_chain = commitment.clone();
    wrong_chain.chain_id.push('x');
    let mut wrong_validators = commitment.clone();
    wrong_validators.validators[0].pub_key[0] ^= 1;
    let mut wrong_hash = evidence.clone();
    wrong_hash.rung = SettlementRung::Committed {
        block_number: block.number,
        block_hash: [0x55; 32],
    };
    let mut wrong_height = evidence.clone();
    wrong_height.rung = SettlementRung::Committed {
        block_number: block.number + 1,
        block_hash: block.hash,
    };
    for (evidence, supplied) in [
        (evidence.clone(), bad_aunt),
        (evidence.clone(), bad_signature),
        (evidence.clone(), unrelated_receipt),
        (evidence.clone(), wrong_chain),
        (evidence, wrong_validators),
        (wrong_hash, commitment.clone()),
        (wrong_height, commitment),
    ] {
        let (facilitator, mut gateway, principal, trace) = context();
        let mut plane =
            SequencerFacilitatorPlane::new(evidence, keys.clone()).with_committed_proof(supplied);
        let error = facilitator
            .settle(
                &mut gateway,
                &principal,
                &request,
                stable,
                SettlementStep::Single,
                &mut plane,
                &trace,
                1,
            )
            .unwrap_err();
        assert_eq!(error.into_error(), X402Error::EvidenceMismatch);
        assert_eq!(
            gateway.translation(&principal, identity.idempotency_key),
            Some(TranslationStatus::Pending)
        );
    }
}
