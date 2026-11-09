//! Typed MCP write outcomes over the ordinary daemon write path.

use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::track::{ReceiptRef, SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_types::result::ResultCode;
use layerx_types::rung::Rung;

use crate::server::{DaemonInvocation, InvocationOutcome, Server, ServerError};

/// Mandatory client stages. No MCP-only write stage exists.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum WriteStage {
    Prepare,
    Disclose,
    Policy,
    Sign,
    Submit,
    Track,
}

pub const ORDINARY_WRITE_STAGES: [WriteStage; 6] = [
    WriteStage::Prepare,
    WriteStage::Disclose,
    WriteStage::Policy,
    WriteStage::Sign,
    WriteStage::Submit,
    WriteStage::Track,
];

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum FailureClass {
    Refused,
    Unavailable,
    InvalidEvidence,
    Protocol,
}

/// Machine-readable failure. There is no success-like prose field.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct StageFailure {
    pub stage: WriteStage,
    pub class: FailureClass,
    pub protocol_result_code: Option<ResultCode>,
}

/// Receipt evidence required before an executed outcome can exist.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedReceipt {
    pub receipt_ref: ReceiptRef,
    pub canonical_receipt: CanonicalBytes,
    pub rung: Rung,
    pub evidence_ids: Vec<[u8; 32]>,
}

/// Complete daemon transcript consumed by an MCP write tool.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WriteTranscript {
    pub stages: Vec<WriteStage>,
    pub submission: Result<TrackedSubmission, StageFailure>,
    pub receipt: Option<VerifiedReceipt>,
}

/// The only non-error write outcomes exposed to a model.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum WriteOutcome {
    Executed {
        submission_ref: SubmissionRef,
        rung: Rung,
        receipt: VerifiedReceipt,
    },
    Unknown {
        submission_ref: SubmissionRef,
        rung: Rung,
        age_ms: u64,
    },
    Pending {
        submission_ref: SubmissionRef,
        rung: Rung,
        state: SubmissionState,
    },
}

#[derive(Debug)]
pub enum WriteToolError {
    Server(ServerError),
    Stage(StageFailure),
    InvalidTranscript,
    SuccessWithoutVerifiedReceipt,
    ReceiptMismatch,
}

/// Named payment write tools that reuse the ordinary submit path.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PaymentTool {
    Send,
    Create,
    Mint,
    Transfer,
    IssueGrant,
    DrawGrant,
}

impl PaymentTool {
    #[must_use]
    pub const fn name(self) -> &'static str {
        match self {
            Self::Send => "wallet.send",
            Self::Create => "token.create",
            Self::Mint => "token.mint",
            Self::Transfer => "token.transfer",
            Self::IssueGrant => "grant.issue",
            Self::DrawGrant => "grant.draw",
        }
    }
}

/// # Errors
/// Refuses missing scope, incomplete daemon stages and unverifiable receipt outcomes.
pub fn execute_payment<F>(
    server: &mut Server,
    core_sequence: u64,
    tool: PaymentTool,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    execute_named(
        server,
        core_sequence,
        tool.name(),
        validated_arguments,
        executor,
        unknown_age_ms,
    )
}

/// Executes a new write invocation through the scoped server's daemon-only route.
///
/// # Errors
///
/// Returns typed stage, transcript, evidence, or daemon-routing failures.
pub fn execute<F>(
    server: &mut Server,
    core_sequence: u64,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    execute_named(
        server,
        core_sequence,
        "activity.submit",
        validated_arguments,
        executor,
        unknown_age_ms,
    )
}

fn execute_named<F>(
    server: &mut Server,
    core_sequence: u64,
    name: &str,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    server
        .execute_committed(core_sequence, name, validated_arguments, |invocation| {
            let transcript = executor(invocation);
            let result = if transcript.stages == ORDINARY_WRITE_STAGES
                || (transcript_matches(&transcript.stages, &ORDINARY_WRITE_STAGES)
                    && transcript.submission.as_ref().is_err_and(|failure| {
                        matches!(
                            failure.class,
                            FailureClass::Refused | FailureClass::Protocol
                        )
                    })
                    && transcript.receipt.is_none())
            {
                classify_transcript(transcript, unknown_age_ms)
            } else {
                Err(WriteToolError::InvalidTranscript)
            };
            let outcome = invocation_outcome(&result);
            (result, outcome)
        })
        .map_err(WriteToolError::Server)?
}

/// Resolves a prior honest non-terminal result through the same daemon tracking path.
///
/// # Errors
///
/// Returns typed evidence or daemon-routing failures and never manufactures completion.
pub fn track<F>(
    server: &mut Server,
    core_sequence: u64,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    track_named(
        server,
        core_sequence,
        "activity.track",
        validated_arguments,
        executor,
        unknown_age_ms,
    )
}

/// # Errors
/// Refuses missing scope, invalid tracking evidence and success without a verified receipt.
pub fn wait<F>(
    server: &mut Server,
    core_sequence: u64,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    track_named(
        server,
        core_sequence,
        "activity.wait",
        validated_arguments,
        executor,
        unknown_age_ms,
    )
}

fn track_named<F>(
    server: &mut Server,
    core_sequence: u64,
    name: &str,
    validated_arguments: Vec<u8>,
    executor: F,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError>
where
    F: FnOnce(&DaemonInvocation) -> WriteTranscript,
{
    server
        .execute_committed(core_sequence, name, validated_arguments, |invocation| {
            let mut transcript = executor(invocation);
            let result = if transcript.stages == [WriteStage::Track] {
                transcript.stages = ORDINARY_WRITE_STAGES.to_vec();
                classify_transcript(transcript, unknown_age_ms)
            } else {
                Err(WriteToolError::InvalidTranscript)
            };
            let outcome = invocation_outcome(&result);
            (result, outcome)
        })
        .map_err(WriteToolError::Server)?
}

fn classify_transcript(
    transcript: WriteTranscript,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError> {
    let submission = match transcript.submission {
        Ok(submission) => submission,
        Err(failure) => return Err(WriteToolError::Stage(failure)),
    };
    classify_submission(submission, transcript.receipt, unknown_age_ms)
}

fn invocation_outcome(result: &Result<WriteOutcome, WriteToolError>) -> InvocationOutcome {
    match result {
        Ok(WriteOutcome::Executed { .. }) => InvocationOutcome::Completed,
        Ok(WriteOutcome::Unknown { .. } | WriteOutcome::Pending { .. }) => {
            InvocationOutcome::Unknown
        }
        Err(WriteToolError::Stage(failure))
            if matches!(
                failure.class,
                FailureClass::Refused | FailureClass::Protocol
            ) =>
        {
            InvocationOutcome::Refused
        }
        Err(_) => InvocationOutcome::Failed,
    }
}

fn classify_submission(
    submission: TrackedSubmission,
    receipt: Option<VerifiedReceipt>,
    unknown_age_ms: u64,
) -> Result<WriteOutcome, WriteToolError> {
    let submission_ref = submission.submission_ref.clone();
    let minimum_rung = if matches!(&submission.state, SubmissionState::Committed { .. }) {
        Rung::Committed
    } else {
        Rung::Receipted
    };
    match submission.state {
        SubmissionState::Executed { receipt_ref } | SubmissionState::Committed { receipt_ref } => {
            let receipt = receipt.ok_or(WriteToolError::SuccessWithoutVerifiedReceipt)?;
            if receipt.receipt_ref != receipt_ref {
                return Err(WriteToolError::ReceiptMismatch);
            }
            if submission.rung < minimum_rung
                || receipt.rung != submission.rung
                || receipt.evidence_ids.is_empty()
            {
                return Err(WriteToolError::SuccessWithoutVerifiedReceipt);
            }
            Ok(WriteOutcome::Executed {
                submission_ref,
                rung: submission.rung,
                receipt,
            })
        }
        SubmissionState::Failed { result } => Err(WriteToolError::Stage(StageFailure {
            stage: WriteStage::Track,
            class: FailureClass::Protocol,
            protocol_result_code: Some(result),
        })),
        SubmissionState::Unknown => Ok(WriteOutcome::Unknown {
            submission_ref,
            rung: submission.rung,
            age_ms: unknown_age_ms,
        }),
        state => {
            if receipt.is_some() {
                return Err(WriteToolError::InvalidTranscript);
            }
            Ok(WriteOutcome::Pending {
                submission_ref,
                rung: submission.rung,
                state,
            })
        }
    }
}

fn transcript_matches(actual: &[WriteStage], required: &[WriteStage]) -> bool {
    if actual == required {
        return true;
    }
    let Some(WriteStage::Policy) = actual.last() else {
        return false;
    };
    actual == &required[..3]
}

pub(crate) fn preparation_id(arguments: &serde_json::Value) -> Option<[u8; 32]> {
    let value = arguments
        .get("preparation")?
        .get("purpose")?
        .get("purpose")?
        .get("preparation_id")?
        .as_str()?;
    identifier(value)
}

fn identifier(value: &str) -> Option<[u8; 32]> {
    if value.len() != 64
        || !value
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return None;
    }
    let mut out = [0u8; 32];
    for (i, b) in out.iter_mut().enumerate() {
        *b = u8::from_str_radix(&value[i * 2..i * 2 + 2], 16).ok()?;
    }
    Some(out)
}

fn lower_hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// Encodes the native owner answer from a verified EIP-1559 preparation.
///
/// # Errors
/// Refuses a preparation whose call, sender, signing hash or disclosure binding changed.
pub fn native_call_preparation_answer(
    prepared: &layerx_agentd::prepare::evm::Prepared,
    approval_id: Option<[u8; 32]>,
) -> Result<serde_json::Value, crate::boundary::BoundaryRefusal> {
    use serde_json::json;
    use sha2::{Digest, Sha256};
    layerx_agentd::prepare::evm::verify_call_binding(prepared).map_err(|_| {
        crate::boundary::BoundaryRefusal::Malformed("mcp.native_call_binding".into())
    })?;
    let canonical = prepared.call.encode_unsigned();
    let id: [u8; 32] = Sha256::digest(&canonical).into();
    Ok(json!({
        "version": "1", "preparation_id": lower_hex(&id),
        "canonical_bytes": lower_hex(&canonical),
        "signing_preimage": lower_hex(&prepared.signing_hash),
        "activity": {"version": "1", "module": (prepared.disclosure.activity_type.module() as u16).to_string(),
            "ordinal": prepared.disclosure.activity_type.ordinal().to_string()},
        "approval_required": approval_id.is_some(), "approval_id": approval_id.map(|id| lower_hex(&id)),
    }))
}

/// Encodes disclosure fields bound to the authenticated sender of a prepared call.
///
/// # Errors
/// Refuses an inconsistent preparation or a disclosure that cannot be canonically encoded.
pub fn native_call_disclosure_answer(
    prepared: &layerx_agentd::prepare::evm::Prepared,
) -> Result<serde_json::Value, crate::boundary::BoundaryRefusal> {
    use serde_json::json;
    use sha2::{Digest, Sha256};
    let refuse = || crate::boundary::BoundaryRefusal::Malformed("mcp.native_call_binding".into());
    layerx_agentd::prepare::evm::verify_call_binding(prepared).map_err(|_| refuse())?;
    let canonical = prepared.call.encode_unsigned();
    let id: [u8; 32] = Sha256::digest(&canonical).into();
    let d = &prepared.disclosure;
    Ok(json!({
        "version": "1", "preparation_id": lower_hex(&id), "canonical_bytes": lower_hex(&canonical),
        "disclosure_digest": lower_hex(&d.signing_digest().map_err(|_| refuse())?),
        "activity_type": d.activity_type.value().to_string(), "actor": lower_hex(&d.actor),
        "authority": lower_hex(&d.authority), "asset": lower_hex(&d.asset), "fee_limit": d.fee_limit.to_string(),
        "not_before": d.expiry.not_before.to_string(), "not_after": d.expiry.not_after.to_string(),
        "payload_expires_at": d.expiry.payload_expires_at.to_string(), "idempotency_key": lower_hex(&d.idempotency_key),
    }))
}

/// Executes a native alias against its authenticated owner boundary.
///
/// # Errors
/// Refuses call bytes, intent, sender, preparation or disclosure fields that disagree,
/// and preserves the owner's preparation, signing, submission and tracking refusals.
pub fn execute_native_alias<F>(
    name: &str,
    arguments: &serde_json::Value,
    _registry: &layerx_types::payload::ModuleRegistry,
    key: [u8; 32],
    mut owner: F,
) -> Result<serde_json::Value, crate::boundary::BoundaryRefusal>
where
    F: FnMut(
        layerx_agentd::tenant::Operation,
        &serde_json::Value,
    ) -> Result<serde_json::Value, crate::boundary::BoundaryRefusal>,
{
    use crate::boundary::BoundaryRefusal;
    use layerx_agentd::tenant::Operation;
    use serde_json::json;
    let refuse = || BoundaryRefusal::Malformed("mcp.native_write_binding".into());
    if arguments.get("variant").and_then(serde_json::Value::as_str) != Some("native_write_v1") {
        return Err(BoundaryRefusal::NotServed("native_write_v1"));
    }
    let intent = arguments.get("intent").ok_or_else(refuse)?;
    let preparation = arguments.get("preparation").ok_or_else(refuse)?;
    if intent
        .get("idempotency_key")
        .and_then(serde_json::Value::as_str)
        .and_then(identifier)
        != Some(key)
    {
        return Err(refuse());
    }
    let expected_id = preparation_id(arguments).ok_or_else(refuse)?;
    let canonical_text = arguments
        .get("canonical_bytes")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(refuse)?;
    if canonical_text.is_empty()
        || canonical_text.len() % 2 != 0
        || !canonical_text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(refuse());
    }
    let canonical = (0..canonical_text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&canonical_text[i..i + 2], 16).map_err(|_| refuse()))
        .collect::<Result<Vec<_>, _>>()?;
    let call = layerx_crypto::evm_transaction::decode_unsigned(&canonical).map_err(|_| refuse())?;
    let signer = arguments
        .get("signer_address")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(refuse)?;
    let sender_text = signer.strip_prefix("0x").unwrap_or(signer);
    if sender_text.len() != 40
        || !sender_text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(refuse());
    }
    let mut sender = [0; 20];
    for (index, byte) in sender.iter_mut().enumerate() {
        *byte =
            u8::from_str_radix(&sender_text[index * 2..index * 2 + 2], 16).map_err(|_| refuse())?;
    }
    let signature = arguments.get("signature").ok_or_else(refuse)?;
    let signature_text = signature.as_str().ok_or_else(refuse)?;
    let signature_text = signature_text.strip_prefix("0x").unwrap_or(signature_text);
    if signature_text.len() != 130
        || !signature_text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(refuse());
    }
    let signature_bytes = (0..signature_text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&signature_text[i..i + 2], 16).map_err(|_| refuse()))
        .collect::<Result<Vec<_>, _>>()?;
    let authenticated_signature = layerx_crypto::signer::AgentSignature::new(
        signature_bytes.try_into().map_err(|_| refuse())?,
    )
    .map_err(|_| refuse())?;
    if layerx_crypto::evm_transaction::recover_sender(&call, &authenticated_signature)
        .map_err(|_| refuse())?
        != sender
    {
        return Err(refuse());
    }
    let preview =
        layerx_crypto::disclosure::call::bind_call(&call, sender).map_err(|_| refuse())?;
    use sha2::{Digest, Sha256};
    let signed = preparation.get("purpose").ok_or_else(refuse)?;
    let purpose = signed.get("purpose").ok_or_else(refuse)?;
    let same_decimal = |field: &str, n: u128| {
        preparation.get(field).and_then(serde_json::Value::as_str) == Some(n.to_string().as_str())
    };
    let supplied_activity = preparation.get("activity").ok_or_else(refuse)?;
    if <[u8; 32]>::from(Sha256::digest(&canonical)) != expected_id
        || purpose
            .get("canonical_digest")
            .and_then(serde_json::Value::as_str)
            .and_then(identifier)
            != Some(expected_id)
        || supplied_activity
            .get("module")
            .and_then(serde_json::Value::as_str)
            .and_then(|s| s.parse::<u16>().ok())
            != Some(preview.activity_type.module() as u16)
        || supplied_activity
            .get("ordinal")
            .and_then(serde_json::Value::as_str)
            .and_then(|s| s.parse::<u16>().ok())
            != Some(preview.activity_type.ordinal())
        || preparation
            .get("actor")
            .and_then(serde_json::Value::as_str)
            .filter(|actor| !actor.is_empty())
            != purpose.get("agent_did").and_then(serde_json::Value::as_str)
        || purpose
            .get("agent_did")
            .and_then(serde_json::Value::as_str)
            .is_none()
        || preparation
            .get("authority")
            .and_then(serde_json::Value::as_str)
            .map(|text| text.strip_prefix("0x").unwrap_or(text))
            != Some(lower_hex(&preview.authority).as_str())
        || !same_decimal("account_sequence", u128::from(preview.envelope_sequence()))
        || !same_decimal("fee_limit", preview.fee_limit)
        || !same_decimal("not_before", u128::from(preview.expiry.not_before))
        || !same_decimal("not_after", u128::from(preview.expiry.not_after))
        || preparation
            .get("payload")
            .and_then(serde_json::Value::as_str)
            != Some(lower_hex(preview.canonical_payload()).as_str())
        || preparation
            .get("payload_hash")
            .and_then(serde_json::Value::as_str)
            .and_then(identifier)
            != Some(
                Sha256::new()
                    .chain_update(b"LXP/v1/payload-hash\0")
                    .chain_update(preview.canonical_payload())
                    .finalize()
                    .into(),
            )
        || preparation
            .get("idempotency_key")
            .and_then(serde_json::Value::as_str)
            .and_then(identifier)
            != Some(key)
        || preview.idempotency_key != key
    {
        return Err(refuse());
    }
    verify_native_intent(name, intent, &preview)?;
    let prepared_value = owner(Operation::Prepare, preparation)?;
    let prepared = layerx_sdk::agent_envelope::decode_native_preparation(&prepared_value)
        .ok_or_else(refuse)?;
    if prepared.preparation_id != expected_id
        || prepared.canonical_bytes != canonical
        || prepared.signing_preimage != call.signing_hash()
        || prepared.activity.module != preview.activity_type.module() as u16
        || prepared.activity.ordinal != preview.activity_type.ordinal()
    {
        return Err(refuse());
    }
    let disclosed_value = owner(
        Operation::Prepare,
        &json!({"variant":"native_disclosure_v1","canonical_bytes":lower_hex(&prepared.canonical_bytes)}),
    )?;
    let exported = layerx_sdk::agent_envelope::decode_native_disclosure(&disclosed_value)
        .ok_or_else(refuse)?;
    if exported.preparation_id != prepared.preparation_id
        || exported.canonical_bytes != prepared.canonical_bytes
    {
        return Err(refuse());
    }
    let returned_call = layerx_crypto::evm_transaction::decode_unsigned(&prepared.canonical_bytes)
        .map_err(|_| refuse())?;
    let disclosure =
        layerx_crypto::disclosure::call::bind_call(&returned_call, sender).map_err(|_| refuse())?;
    if disclosure != preview {
        return Err(refuse());
    }
    if <[u8; 32]>::from(Sha256::digest(&prepared.canonical_bytes)) != prepared.preparation_id
        || disclosure.signing_digest().map_err(|_| refuse())? != exported.disclosure_digest
        || disclosure.activity_type.value() != exported.activity_type
        || disclosure.actor != exported.actor
        || disclosure.authority != exported.authority
        || disclosure.asset != exported.asset
        || disclosure.fee_limit != exported.fee_limit
        || disclosure.expiry.not_before != exported.not_before
        || disclosure.expiry.not_after != exported.not_after
        || disclosure.expiry.payload_expires_at != exported.payload_expires_at
        || disclosure.idempotency_key != key
        || disclosure.idempotency_key != exported.idempotency_key
    {
        return Err(refuse());
    }
    verify_native_intent(name, intent, &disclosure)?;
    if prepared.approval_required {
        return Ok(prepared_value);
    }
    let reference = lower_hex(&prepared.preparation_id);
    owner(
        Operation::Sign,
        &json!({"preparation_ref":reference,"signature":signature}),
    )?;
    let mut submit =
        json!({"preparation_ref":reference,"signature":signature,"signer_address":signer});
    if matches!(name, "wallet.send" | "token.transfer") {
        submit["variant"] = json!("native_send_submit_v1");
    }
    let observation = owner(Operation::Submit, &submit)?;
    let submission = observation
        .get("submission")
        .and_then(|value| value.get("submission_ref"))
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| BoundaryRefusal::Unavailable("mcp.native_submit_observation".into()))?;
    owner(Operation::Track, &json!({"submission_ref":submission}))
}

fn verify_native_intent(
    name: &str,
    intent: &serde_json::Value,
    d: &layerx_crypto::disclosure::Disclosure,
) -> Result<(), crate::boundary::BoundaryRefusal> {
    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};
    use layerx_crypto::payments::Payment;
    let refused =
        || crate::boundary::BoundaryRefusal::Malformed("mcp.native_intent_mismatch".into());
    let id = |field: &str| {
        intent
            .get(field)
            .and_then(serde_json::Value::as_str)
            .and_then(identifier)
            .ok_or_else(refused)
    };
    let amount = |field: &str| {
        intent
            .get(field)
            .and_then(serde_json::Value::as_str)
            .and_then(|v| v.parse::<u128>().ok())
            .ok_or_else(refused)
    };
    let ordinal = crate::catalogue::native_alias_ordinal(name).ok_or_else(refused)?;
    if d.activity_type.module() != layerx_types::payload::ModuleId::Asset
        || d.activity_type.value() != (u32::from(1u16) << 16 | u32::from(ordinal))
    {
        return Err(refused());
    }
    let matches = match name {
        "wallet.send" | "token.transfer" => {
            let recipients = d
                .counterparties
                .iter()
                .filter(|c| c.role == CounterpartyRole::Recipient)
                .collect::<Vec<_>>();
            let amounts = d
                .amounts
                .iter()
                .filter(|a| a.role == AmountRole::Transfer)
                .collect::<Vec<_>>();
            recipients.len() == 1
                && amounts.len() == 1
                && recipients[0].account == id("destination")?
                && amounts[0].value == amount("amount")?
                && d.asset == id("asset")?
        }
        "token.mint" => {
            matches!(&d.payment,Some(Payment::Mint{asset,to,amount:value}) if *asset==id("asset")? && *to==id("destination")? && *value==amount("amount")?)
        }
        "grant.issue" => {
            matches!(&d.payment,Some(Payment::IssueGrant(g)) if g.recipient==id("beneficiary")? && g.asset==id("asset")? && g.allowance==amount("amount")? && u128::from(g.expiration)==amount("expires_at_ms")?)
        }
        "grant.draw" => {
            matches!(d.construct_method().map_err(|_| refused())?,
                Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerDraw { grant, amount: value, .. })
                    if grant == id("grant_id")? && value == amount("amount")?)
        }
        "token.create" => {
            matches!(&d.payment,Some(Payment::Register(r)) if intent.get("symbol").and_then(serde_json::Value::as_str)==Some(r.symbol.as_str()) && amount("decimals")?==u128::from(r.decimals) && amount("supply_cap")?==r.supply_cap)
        }
        _ => false,
    };
    if matches {
        Ok(())
    } else {
        Err(refused())
    }
}

#[cfg(test)]
mod native_disclosure_tests {
    use super::*;
    use layerx_agentd::prepare::evm;
    use layerx_agentd::tenant::Operation;
    use layerx_crypto::evm_transaction::decode_signed;
    use serde_json::{json, Value};
    use sha2::{Digest, Sha256};

    fn served_call() -> evm::Prepared {
        let payload =
            include_str!("../../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim();
        let payload = (0..payload.len())
            .step_by(2)
            .map(|i| u8::from_str_radix(&payload[i..i + 2], 16).unwrap())
            .collect();
        let key = k256::ecdsa::SigningKey::from_slice(&[3; 32]).unwrap();
        let from = layerx_crypto::secp256k1::evm_address(
            key.verifying_key().to_encoded_point(false).as_bytes(),
        )
        .unwrap();
        evm::prepare_bound(
            evm::EvmPreparationState {
                chain_id: 125,
                pending_nonce: 7,
                base_fee_per_gas: 1,
                head_height: 50,
            },
            evm::PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 5,
                maximum_payload_bytes: 65536,
            },
            evm::PrepareRequest {
                from,
                call: evm::PrecompileCall::ledger_transfer(payload),
                expected_nonce: Some(7),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65536,
            },
            100_000,
        )
        .expect("daemon Ledger call producer")
    }

    fn arguments(prepared: &evm::Prepared) -> Value {
        let d = &prepared.disclosure;
        let canonical = prepared.call.encode_unsigned();
        let id: [u8; 32] = Sha256::digest(&canonical).into();
        let raw = prepared.call.sign(&[3; 32]).unwrap();
        let (_, signature) = decode_signed(&raw).unwrap();
        let payload_hash: [u8; 32] = Sha256::new()
            .chain_update(b"LXP/v1/payload-hash\0")
            .chain_update(d.canonical_payload())
            .finalize()
            .into();
        json!({
            "variant":"native_write_v1",
            "intent":{"destination":lower_hex(&[2; 32]), "asset":lower_hex(&[3; 32]),
                "amount":"123", "idempotency_key":lower_hex(&d.idempotency_key)},
            "canonical_bytes":lower_hex(&canonical), "signer_address":lower_hex(&prepared.from),
            "signature":lower_hex(signature.as_bytes()),
            "preparation":{
                "variant":"native_send_v1", "activity":{"version":"1","module":"1","ordinal":"5"},
                "actor":"did:layerx:alice", "authority":lower_hex(&d.authority),
                "account_sequence":d.envelope_sequence().to_string(), "fee_limit":d.fee_limit.to_string(),
                "not_before":d.expiry.not_before.to_string(), "not_after":d.expiry.not_after.to_string(),
                "payload":lower_hex(d.canonical_payload()), "payload_hash":lower_hex(&payload_hash),
                "idempotency_key":lower_hex(&d.idempotency_key),
                "purpose":{"purpose":{"preparation_id":lower_hex(&id),
                    "canonical_digest":lower_hex(&id), "agent_did":"did:layerx:alice"}}
            }
        })
    }

    fn consume(
        prepared: &evm::Prepared,
        arguments: &Value,
        changed: Option<(&str, Value)>,
    ) -> Result<Value, crate::boundary::BoundaryRefusal> {
        execute_native_alias(
            "wallet.send",
            arguments,
            &layerx_types::payload::ModuleRegistry::new(&[]).unwrap(),
            prepared.disclosure.idempotency_key,
            |operation, request| {
                assert_eq!(
                    operation,
                    Operation::Prepare,
                    "held call must not sign or submit"
                );
                let mut value = if request["variant"] == "native_disclosure_v1" {
                    assert_eq!(
                        request["canonical_bytes"],
                        lower_hex(&prepared.call.encode_unsigned())
                    );
                    let mut value = native_call_disclosure_answer(prepared)?;
                    if let Some((field, replacement)) = &changed {
                        value[*field] = replacement.clone();
                    }
                    value
                } else {
                    native_call_preparation_answer(prepared, Some([7; 32]))?
                };
                let response = layerx_agentd::agent_rpc::mcp_result_response(
                    layerx_agent_api::error::RequestId(1),
                    value.take(),
                );
                let wire = serde_json::from_slice(&response.body).unwrap();
                Ok(
                    layerx_sdk::agent_envelope::decode_response(response.status, &wire)
                        .expect("SDK response codec")
                        .expect("daemon response")
                        .value,
                )
            },
        )
    }

    #[test]
    fn native_disclosure_binds_served_ledger_call_and_signer() {
        let prepared = served_call();
        assert_eq!(&prepared.call.data[..4], &[0x2b, 0xb1, 0xae, 0x7c]);
        let answer = native_call_disclosure_answer(&prepared).unwrap();
        let decoded = layerx_sdk::agent_envelope::decode_native_disclosure(&answer).unwrap();
        assert_eq!(decoded.disclosure_digest, prepared.call.signing_hash());
        let request = layerx_agent_api::identity::NativeDisclosureRequestV1 {
            canonical_bytes: prepared.call.encode_unsigned(),
        };
        layerx_sdk::agent_envelope::validate_call_disclosure(&request, &decoded, prepared.from)
            .expect("SDK and MCP bind the same served call");
        let args = arguments(&prepared);
        let result = consume(&prepared, &args, None).expect("served Ledger transfer accepted");
        assert_eq!(result["approval_required"], true);
        let mut wrong_signer = args.clone();
        wrong_signer["signer_address"] = json!(lower_hex(&[0x22; 20]));
        assert!(consume(&prepared, &wrong_signer, None).is_err());
        let mut wrong_intent = args;
        wrong_intent["intent"]["amount"] = json!("124");
        assert!(consume(&prepared, &wrong_intent, None).is_err());
    }

    #[test]
    fn native_disclosure_rejects_every_changed_answer_field() {
        let prepared = served_call();
        let args = arguments(&prepared);
        let answer = native_call_disclosure_answer(&prepared).unwrap();
        for field in [
            "preparation_id",
            "canonical_bytes",
            "disclosure_digest",
            "activity_type",
            "actor",
            "authority",
            "asset",
            "fee_limit",
            "not_before",
            "not_after",
            "payload_expires_at",
            "idempotency_key",
        ] {
            let text = answer[field].as_str().unwrap();
            let replacement = if matches!(
                field,
                "activity_type" | "fee_limit" | "not_before" | "not_after" | "payload_expires_at"
            ) {
                (text.parse::<u128>().unwrap() ^ 1).to_string()
            } else {
                let mut bytes = text.as_bytes().to_vec();
                bytes[0] = if bytes[0] == b'0' { b'1' } else { b'0' };
                String::from_utf8(bytes).unwrap()
            };
            assert!(
                consume(&prepared, &args, Some((field, json!(replacement)))).is_err(),
                "{field}"
            );
        }
        let audit = prepared.disclosure.audit_digest().unwrap();
        assert_ne!(audit, prepared.signing_hash);
        assert!(consume(
            &prepared,
            &args,
            Some(("disclosure_digest", json!(lower_hex(&audit))))
        )
        .is_err());
    }
}
