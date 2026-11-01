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
    account_identifiers, AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse,
    X402Error, X402_VERSION,
};
use serde::Deserialize;
use serde_json::{json, Value};

const SEQUENCED_SETTLEMENT_SUITE: &str =
    include_str!("../../../specs/conformance/x402/sequenced-settlement.json");
const RECORDED_RECEIPTS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Vector {
    name: String,
    valid: bool,
    expected: Expected,
    document: SettlementResponse,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Expected {
    verdict: Verdict,
    field: String,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
enum Verdict {
    Accept,
    Pending,
    Refuse,
}

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

fn sequenced_settlement_vectors() -> Vec<Vector> {
    let vectors: Vec<Vector> = serde_json::from_str(SEQUENCED_SETTLEMENT_SUITE)
        .expect("sequenced settlement vectors decode as PAYMENT-RESPONSE bodies");
    assert_eq!(
        vectors
            .iter()
            .map(|vector| vector.name.as_str())
            .collect::<Vec<_>>(),
        [
            "receipted_settlement",
            "submitted_settlement",
            "committed_without_proof",
            "committed_settlement"
        ],
        "the suite must exercise each declared case exactly once"
    );
    vectors
}

#[test]
fn sequenced_settlement_vectors_carry_the_recorded_receipt() {
    let fixture: Value = serde_json::from_str(RECORDED_RECEIPTS)
        .expect("recorded sequencer receipt fixture decodes");
    let recorded = fixture["cases"]
        .as_array()
        .expect("receipt fixture contains cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("receipt fixture contains the recorded payExact case");
    let tx_hash = recorded["receiptJson"]["txHash"]
        .as_str()
        .expect("recorded payExact transaction hash");
    let signed_receipt = recorded["signedReceipt"]
        .as_str()
        .expect("recorded payExact signed receipt");
    let receipt_digest = recorded["receiptDigest"]
        .as_str()
        .expect("recorded payExact receipt digest");
    assert!(!tx_hash.is_empty());
    assert!(!signed_receipt.is_empty());
    assert!(!receipt_digest.is_empty());

    for vector in sequenced_settlement_vectors() {
        let layerx = vector
            .document
            .extensions
            .get("layerx")
            .and_then(Value::as_object)
            .expect("every vector carries a LayerX receipt evidence object");
        if vector.name == "committed_settlement" {
            assert_committed_vector(&vector);
            continue;
        }
        assert_eq!(layerx["receiptDigest"], receipt_digest, "{}", vector.name);
        match vector.name.as_str() {
            "receipted_settlement" | "submitted_settlement" => {
                assert_eq!(vector.document.transaction, tx_hash, "{}", vector.name);
                assert_eq!(layerx["signedReceipt"], signed_receipt, "{}", vector.name);
                assert_eq!(layerx.len(), 2, "{}", vector.name);
            }
            "committed_without_proof" => {
                assert!(vector.document.transaction.is_empty(), "{}", vector.name);
                assert_eq!(layerx.len(), 1, "{}", vector.name);
                assert!(!layerx.contains_key("signedReceipt"), "{}", vector.name);
            }
            name => panic!("unexpected sequenced settlement case: {name}"),
        }
        assert_eq!(vector.document.network, "eip155:125", "{}", vector.name);
        assert_eq!(
            vector.document.network,
            format!("eip155:{}", fixture["chainId"]),
            "{}",
            vector.name
        );
    }
}

#[test]
fn sequenced_settlement_vectors_validate_on_the_wire() {
    for vector in sequenced_settlement_vectors() {
        let (verdict, field) = match vector.name.as_str() {
            "receipted_settlement" | "committed_settlement" => (Verdict::Accept, "/success"),
            "submitted_settlement" => (Verdict::Pending, "/errorReason"),
            "committed_without_proof" => (Verdict::Refuse, "/transaction"),
            name => panic!("unexpected sequenced settlement case: {name}"),
        };
        assert_eq!(vector.expected.verdict, verdict, "{}", vector.name);
        assert_eq!(vector.expected.field, field, "{}", vector.name);
        assert_eq!(vector.valid, verdict != Verdict::Refuse, "{}", vector.name);
        let document = serde_json::to_value(&vector.document)
            .expect("PAYMENT-RESPONSE serializes as a wire document");
        let expected_validation = match verdict {
            Verdict::Accept => {
                assert!(vector.document.success, "{}", vector.name);
                assert_eq!(vector.document.error_reason, None, "{}", vector.name);
                assert_eq!(document.pointer(field), Some(&Value::Bool(true)));
                Ok(())
            }
            Verdict::Pending => {
                assert!(!vector.document.success, "{}", vector.name);
                assert_eq!(
                    document.pointer(field),
                    Some(&Value::from("settlement_pending")),
                    "{}",
                    vector.name
                );
                Ok(())
            }
            Verdict::Refuse => {
                assert!(vector.document.success, "{}", vector.name);
                assert_eq!(vector.document.error_reason, None, "{}", vector.name);
                assert_eq!(
                    document.pointer(field),
                    Some(&Value::from("")),
                    "{}",
                    vector.name
                );
                Err(X402Error::EvidenceMissing)
            }
        };
        assert_eq!(
            vector.document.validate_wire(),
            expected_validation,
            "{}: {:?} decided on {}",
            vector.name,
            vector.expected.verdict,
            vector.expected.field
        );
    }
}

const COMMITTED_RECEIPTS: &str =
    include_str!("../../../specs/conformance/committed/vector-v4-committed.json");

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

fn committed_case() -> Value {
    let fixture: Value =
        serde_json::from_str(COMMITTED_RECEIPTS).expect("committed fixture decodes");
    fixture["cases"]
        .as_array()
        .expect("committed fixture cases")
        .iter()
        .find(|case| case["id"] == "committed-payExact")
        .expect("recorded committed payExact case")
        .clone()
}

fn assert_committed_vector(vector: &Vector) {
    let case = committed_case();
    let layerx = &vector.document.extensions["layerx"];
    assert!(vector.valid);
    assert_eq!(vector.expected.verdict, Verdict::Accept);
    assert_eq!(vector.expected.field, "/success");
    assert!(vector.document.success);
    assert_eq!(vector.document.error_reason, None);
    assert_eq!(
        vector.document.transaction,
        text(&case["receiptJson"]["txHash"])
    );
    assert_eq!(vector.document.network, "eip155:125");
    assert_eq!(vector.document.amount, Some(AtomicAmount::from_u128(3)));
    assert_eq!(
        vector.document.payer.as_deref(),
        Some(
            text(&case["receiptJson"]["logs"][1]["topics"][2])
                .strip_prefix("0x")
                .expect("payer prefix")
        )
    );
    assert_eq!(layerx["signedReceipt"], case["signedReceipt"]);
    assert_eq!(layerx["verificationLevel"], "committed");
    assert_eq!(layerx["blockNumber"], case["receiptJson"]["blockNumber"]);
    assert_eq!(layerx["blockHash"], case["receiptJson"]["blockHash"]);
    assert_eq!(layerx.as_object().expect("LayerX evidence").len(), 5);
    let signed = bytes(text(&layerx["signedReceipt"]));
    let digest = canonical_receipt_digest(&signed).expect("canonical committed receipt digest");
    assert_eq!(array::<32>(text(&layerx["receiptDigest"])), digest);
    let verified = verify_sequencer_signature(&signed, receipt_keys(&case).as_slice())
        .expect("recorded committed receipt signature verifies");
    assert!(verify_logs(&verified, recorded_evidence(&case).logs).is_ok());
    assert_eq!(vector.document.validate_wire(), Ok(()));
}

#[test]
fn x402_committed_vector_settles_as_committed() {
    let vector = sequenced_settlement_vectors()
        .into_iter()
        .find(|vector| vector.name == "committed_settlement")
        .expect("committed settlement vector");
    assert_committed_vector(&vector);
    let fixture: Value = serde_json::from_str(COMMITTED_RECEIPTS).expect("committed fixture");
    let case = committed_case();
    let mut proof = committed_proof(&case);
    let validators: Vec<_> = committed_array(&case, "validators")
        .iter()
        .map(|validator| Validator {
            pub_key: committed_fixed(validator, "pub_key"),
            voting_power: committed_signed(validator, "voting_power"),
        })
        .collect();
    let chain_id = text(&fixture["provenance"]["chainId"]).to_owned();
    let layerx = &vector.document.extensions["layerx"];
    let recorded_digest = array(text(&layerx["receiptDigest"]));
    let block = verify_committed(recorded_digest, &proof, &validators, &chain_id)
        .expect("recorded committed inclusion proof verifies");
    assert_eq!(
        format!("0x{:x}", block.number),
        text(&layerx["blockNumber"])
    );
    assert_eq!(block.hash, array(text(&layerx["blockHash"])));

    let mut evidence = recorded_evidence(&case);
    evidence.rung = SettlementRung::Committed {
        block_number: block.number,
        block_hash: block.hash,
    };
    let mut request = request(&vector.document.transaction);
    let log = &evidence.logs[1];
    request.payment_requirements.asset = format!(
        "0x{}",
        log.data()[..32]
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    );
    request.payment_requirements.amount = vector.document.amount.expect("committed amount");
    request.payment_payload.accepted = request.payment_requirements.clone();
    let (facilitator, mut gateway, principal, trace) = context();
    let stable = [0x42; 32];
    let identity = SettlementIdentity::derive(&principal, &request, stable, SettlementStep::Single)
        .expect("settlement identity");

    // Bind the recorded payment to this request's identity and account profile,
    // retaining its activity, sequence chain and recorded consensus proof.
    let mut topics = log.topics().to_vec();
    let recorded_topics = topics.concat();
    let words = pay_exact_words(&bytes(text(&case["rawTransaction"])));
    topics[1..].copy_from_slice(&[words[5], words[0], words[1]]);
    topics[1] = identity.idempotency_key;
    topics[3] = array(&format!("0x{}", request.payment_requirements.pay_to));
    replace(
        &mut evidence.signed_receipt,
        &recorded_topics,
        &topics.concat(),
    );
    evidence.logs[1] =
        SequencerLog::new(log.address(), topics, log.data().to_vec(), log.log_index());
    let signer = SigningKey::from_bytes(&[0x72; 32]);
    let digest = canonical_receipt_digest(&evidence.signed_receipt).expect("bound receipt digest");
    let signature_start = evidence.signed_receipt.len() - 64;
    evidence.signed_receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let keys = vec![SequencerAuthorization::new(
        array(text(&case["pinnedKey"]["keyId"])),
        signer.verifying_key().to_bytes(),
        block.number,
        block.number,
    )];
    proof.signed_receipt = evidence.signed_receipt.clone();
    let mut plane = SequencerFacilitatorPlane::new(evidence.clone(), keys).with_committed_proof(
        FacilitatorCommitment {
            proof,
            validators,
            chain_id,
        },
    );
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
        .expect("committed vector settles through the facilitator");
    assert_eq!(response.success, vector.document.success);
    assert_eq!(response.error_reason, vector.document.error_reason);
    assert_eq!(response.transaction, vector.document.transaction);
    assert_eq!(response.network, vector.document.network);
    assert_eq!(response.amount, vector.document.amount);
    assert_eq!(response.payer, vector.document.payer);
    let settled = &response.extensions["layerx"];
    for field in ["verificationLevel", "blockNumber", "blockHash"] {
        assert_eq!(settled[field], layerx[field], "{field}");
    }
    assert_eq!(settled["verificationLevel"], "committed");
    assert_eq!(
        settled["receipt"],
        STANDARD.encode(&evidence.signed_receipt)
    );
    assert_eq!(
        array::<32>(&format!("0x{}", text(&settled["receiptDigest"]))),
        digest
    );
    assert_eq!(response.validate_wire(), Ok(()));
    assert_eq!(
        gateway.translation(&principal, identity.idempotency_key),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: digest
        })
    );
}
