use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

use layerx_agentd::policy::{evaluate, EvaluationInput, Outcome, PolicySet, Purpose};
use layerx_agentd::protocol_evidence::{RawActivityReceiptEvidence, RawReceiptEvidence};
use layerx_crypto::disclosure::call::{bind_call, encode_payment_call};
use layerx_crypto::disclosure::Disclosure;
use layerx_crypto::evm_transaction::{assemble, recover_sender, transaction_hash, Eip1559Call};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{ledger_payment_call, CallOptions};
use layerx_crypto::signer::sign_disclosed_call;
use layerx_sdk::rpc::{Commitment, RpcError, RpcValue};
use layerx_sdk::rpc_verification::VerifiedRpcReceipt;
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use serde_json::json;

use crate::catalogue::native_alias_target;

use crate::server::{InvocationOutcome, Server, ServerError};
use crate::tools::write::PaymentTool;

#[derive(Debug)]
pub enum WalletToolError {
    Server(ServerError),
    Rpc(RpcError),
    Policy,
    Scope,
    Arguments,
    Evidence,
    Read(layerx_agent_api::error::ApiError),
}

pub struct PaymentExecution<'a> {
    pub wallet: &'a Wallet<'a>,
    pub options: &'a PaymentOptions,
    pub canonical_payload: &'a [u8],
    pub policy: &'a PolicySet,
    pub evaluation: &'a EvaluationInput<'a>,
}

/// # Errors
/// Preserves daemon scope, policy and budget refusal before any wallet signature or submission.
pub fn execute(
    server: &mut Server,
    core_sequence: u64,
    tool: PaymentTool,
    execution: &PaymentExecution<'_>,
) -> Result<VerifiedRpcReceipt, WalletToolError> {
    let input = execution.evaluation;
    let intent = input.intent();
    let session = input.session();
    if session.request.session_id != server.binding().session_id()
        || &session.request.tenant != server.binding().tenant()
        || input.capability().id() != server.binding().capability_id()
        || session.request.agent.as_bytes() != execution.options.actor.as_bytes()
        || intent.core_sequence != core_sequence
        || !matches!(&intent.purpose, Purpose::Text(text) if text.as_str() == tool.name())
    {
        return Err(WalletToolError::Scope);
    }
    let [effect] = intent.effects.as_slice() else {
        return Err(WalletToolError::Arguments);
    };
    let target = native_alias_target(tool.name()).ok_or(WalletToolError::Arguments)?;
    let ordinal = target.ordinal;
    server
        .execute_committed(
            core_sequence,
            tool.name(),
            payment_arguments(execution)?,
            |_| {
                let result = prepare(tool, execution).and_then(|(call, disclosure, from)| {
                    let (amount, asset, counterparty) =
                        policy_dimensions(tool, &disclosure, execution)?;
                    if effect.amount != amount
                        || effect.asset != asset
                        || effect.counterparty != counterparty
                        || effect.activity_type != ordinal
                    {
                        return Err(WalletToolError::Arguments);
                    }
                    let decision = evaluate(execution.policy, input);
                    if decision.outcome != Outcome::Allow {
                        return Err(WalletToolError::Policy);
                    }
                    let signature = run(sign_disclosed_call(
                        execution.wallet.signer,
                        &call,
                        &disclosure,
                        from,
                    ))
                    .map_err(|error| WalletToolError::Rpc(RpcError::Signing(error)))?;
                    if recover_sender(&call, &signature).map_err(|_| WalletToolError::Evidence)?
                        != from
                    {
                        return Err(WalletToolError::Evidence);
                    }
                    let raw = assemble(&call, &signature);
                    let hash = execution
                        .wallet
                        .rpc
                        .send_raw_transaction(&raw)
                        .map_err(WalletToolError::Rpc)?;
                    if hash != transaction_hash(&raw) {
                        return Err(WalletToolError::Evidence);
                    }
                    execution
                        .wallet
                        .wait_for(
                            hash,
                            execution.options.commitment,
                            execution.options.wait_timeout,
                        )
                        .map_err(WalletToolError::Rpc)
                });
                let outcome = classify(&result);
                (result, outcome)
            },
        )
        .map_err(WalletToolError::Server)?
}

pub fn prepare_alias_call(
    tool: PaymentTool,
    payload: &[u8],
    actor: &[u8],
    options: &CallOptions,
) -> Result<Eip1559Call, WalletToolError> {
    let target = native_alias_target(tool.name()).ok_or(WalletToolError::Arguments)?;
    let call = if target.ordinal == 5 {
        ledger_payment_call(payload, options)
            .map_err(|_| WalletToolError::Arguments)?
            .call
    } else {
        let payment = Payment::decode(target.module, target.ordinal, payload, actor)
            .map_err(|_| WalletToolError::Arguments)?;
        Eip1559Call {
            chain_id: options.chain_id,
            nonce: options.nonce,
            max_priority_fee_per_gas: options.max_priority_fee_per_gas,
            max_fee_per_gas: options.max_fee_per_gas,
            gas_limit: options.gas_limit,
            to: target.address,
            data: encode_payment_call(&payment, actor).map_err(|_| WalletToolError::Arguments)?,
        }
    };
    if call.to != target.address {
        return Err(WalletToolError::Evidence);
    }
    Ok(call)
}

fn prepare(
    tool: PaymentTool,
    execution: &PaymentExecution<'_>,
) -> Result<(Eip1559Call, Disclosure, [u8; 20]), WalletToolError> {
    let wallet = execution.wallet;
    let options = execution.options;
    if options.wait_timeout > Duration::from_secs(300) {
        return Err(WalletToolError::Arguments);
    }
    if options.commitment == Commitment::Finalised
        && wallet.policy.trusted_checkpoint_context_digest.is_none()
    {
        return Err(WalletToolError::Rpc(RpcError::MissingFinalityTrust));
    }
    let from = wallet
        .signer
        .evm_address()
        .ok_or(WalletToolError::Arguments)?;
    let address: String = from.iter().map(|byte| format!("{byte:02x}")).collect();
    let nonce = wallet
        .rpc
        .get_identity_sequence(&format!("0x{address}"))
        .map_err(WalletToolError::Rpc)?;
    let (chain_id, priority, fee) = wallet
        .rpc
        .eip1559_parameters()
        .map_err(WalletToolError::Rpc)?;
    let gas_limit = options
        .fee_limit
        .checked_div(fee)
        .and_then(|gas| u64::try_from(gas).ok())
        .filter(|gas| *gas > 0)
        .ok_or(WalletToolError::Arguments)?;
    let mut call = prepare_alias_call(
        tool,
        execution.canonical_payload,
        options.actor.as_bytes(),
        &CallOptions {
            from,
            chain_id,
            nonce,
            max_priority_fee_per_gas: priority,
            max_fee_per_gas: fee,
            gas_limit,
        },
    )?;
    let gas = wallet
        .rpc
        .estimate_fee_from(&call, from)
        .map_err(WalletToolError::Rpc)?;
    if gas == 0 || gas > gas_limit {
        return Err(WalletToolError::Arguments);
    }
    call.gas_limit = gas;
    let disclosure = bind_call(&call, from).map_err(|_| WalletToolError::Evidence)?;
    if let Some(sequence) = disclosure
        .payload_sequence()
        .map_err(|_| WalletToolError::Evidence)?
    {
        let source = match disclosure
            .construct_method()
            .map_err(|_| WalletToolError::Evidence)?
        {
            Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerDraw {
                recipient,
                ..
            }) => recipient,
            _ => {
                disclosure
                    .counterparties
                    .first()
                    .ok_or(WalletToolError::Arguments)?
                    .account
            }
        };
        let account: String = source.iter().map(|byte| format!("{byte:02x}")).collect();
        let value = wallet
            .rpc
            .get_account(&account)
            .map_err(WalletToolError::Rpc)?;
        if value["account_id"] != account {
            return Err(WalletToolError::Evidence);
        }
        let text = value
            .get("next_sequence")
            .and_then(serde_json::Value::as_str)
            .ok_or(WalletToolError::Evidence)?;
        let observed = text.parse::<u64>().map_err(|_| WalletToolError::Evidence)?;
        if observed.to_string() != text {
            return Err(WalletToolError::Evidence);
        }
        if sequence != observed {
            return Err(WalletToolError::Rpc(RpcError::StaleSourceSequence));
        }
    }
    Ok((call, disclosure, from))
}

fn policy_dimensions(
    tool: PaymentTool,
    disclosure: &layerx_crypto::disclosure::Disclosure,
    execution: &PaymentExecution<'_>,
) -> Result<(u128, [u8; 32], [u8; 32]), WalletToolError> {
    match (tool, disclosure.payment.as_ref()) {
        (PaymentTool::Create, Some(Payment::Register(_))) => Ok((
            execution.options.fee_limit,
            execution.wallet.native_asset,
            layerx_sdk::rpc::wallet_account(
                &execution.options.actor,
                execution.wallet.native_asset,
                execution.wallet.native_asset,
            )
            .map_err(WalletToolError::Rpc)?,
        )),
        (PaymentTool::Mint, Some(Payment::Mint { asset, to, amount })) => {
            Ok((*amount, *asset, *to))
        }
        (PaymentTool::IssueGrant, Some(Payment::IssueGrant(grant))) => {
            Ok((grant.allowance, grant.asset, grant.recipient))
        }
        (
            PaymentTool::DrawGrant,
            Some(Payment::Receive {
                from,
                asset,
                amount,
                ..
            }),
        ) => Ok((*amount, *asset, *from)),
        (PaymentTool::Send | PaymentTool::Transfer, None) => Ok((
            disclosure
                .amounts
                .iter()
                .try_fold(0_u128, |sum, amount| sum.checked_add(amount.value))
                .ok_or(WalletToolError::Arguments)?,
            disclosure.asset,
            disclosure
                .counterparties
                .last()
                .ok_or(WalletToolError::Arguments)?
                .account,
        )),
        _ => Err(WalletToolError::Arguments),
    }
}

/// Converts a wallet-produced batch-verified receipt into daemon cumulative-use ingress.
///
/// # Errors
/// Refuses independent receipt lookups, executed-only receipts, or missing inclusion material.
pub fn cumulative_evidence(
    receipt: &VerifiedRpcReceipt,
) -> Result<RawActivityReceiptEvidence, WalletToolError> {
    let activity = receipt
        .canonical_activity()
        .ok_or(WalletToolError::Evidence)?;
    let batch = receipt.batch_evidence().ok_or(WalletToolError::Evidence)?;
    Ok(RawActivityReceiptEvidence::from_signed_inclusion(
        activity.to_vec(),
        RawReceiptEvidence::new(
            receipt.canonical_bytes().to_vec(),
            batch.proof().clone(),
            batch.canonical_header().to_vec(),
            batch.header_signature(),
        ),
    ))
}

/// # Errors
/// Refuses another agent's account list or a signer without an EVM address.
pub fn accounts(
    server: &mut Server,
    core_sequence: u64,
    wallet: &Wallet<'_>,
    did: &str,
) -> Result<RpcValue, WalletToolError> {
    let server_actor = server.binding().agent().as_bytes().to_vec();
    server
        .execute_read(
            core_sequence,
            "wallet.accounts",
            did.as_bytes().to_vec(),
            |_| {
                let result = if did.as_bytes() != server_actor {
                    Err(WalletToolError::Scope)
                } else {
                    wallet
                        .signer
                        .evm_address()
                        .ok_or(WalletToolError::Arguments)
                        .map(|address| {
                            let address: String =
                                address.iter().map(|byte| format!("{byte:02x}")).collect();
                            json!({"accounts": [format!("0x{address}")]})
                        })
                };
                let outcome = if result.is_ok() {
                    InvocationOutcome::Completed
                } else {
                    InvocationOutcome::Failed
                };
                (result, outcome)
            },
        )
        .map_err(WalletToolError::Server)?
}

/// # Errors
/// Preserves daemon balance read errors and scoped read authorization.
pub fn balance(
    server: &mut Server,
    core_sequence: u64,
    config: &layerx_agentd::node_rpc::NodeRpcConfig,
    account: [u8; 20],
    height: Option<u64>,
) -> Result<RpcValue, WalletToolError> {
    let arguments = serde_json::to_vec(&json!({"account": account, "height": height}))
        .map_err(|_| WalletToolError::Arguments)?;
    server
        .execute_read(core_sequence, "wallet.balance", arguments, |_| {
            let result = layerx_agentd::read::balance(config, account, height)
                .map(|read| json!({
                    "account": format!("0x{}", account.iter().map(|byte| format!("{byte:02x}")).collect::<String>()),
                    "amount": read.amount.value().to_string(),
                    "rung": read.achieved_rung.map(layerx_types::rung::Rung::name),
                    "freshness": {"chain_head": read.freshness.chain_head, "value_height": read.freshness.value_height,
                        "relative_to": if height.is_some() { "Block" } else { "Pending" }}
                }))
                .map_err(WalletToolError::Read);
            let outcome = if result.is_ok() { InvocationOutcome::Completed } else { InvocationOutcome::Failed };
            (result, outcome)
        })
        .map_err(WalletToolError::Server)?
}

/// # Errors
/// Retains scoped tracking authorization and requires the requested verified commitment.
pub fn wait(
    server: &mut Server,
    core_sequence: u64,
    wallet: &Wallet<'_>,
    activity: [u8; 32],
    commitment: Commitment,
    timeout: Duration,
) -> Result<VerifiedRpcReceipt, WalletToolError> {
    let mut arguments = activity.to_vec();
    arguments.extend_from_slice(commitment.as_str().as_bytes());
    server
        .execute_committed(core_sequence, "activity.wait", arguments, |_| {
            let result = wallet
                .wait_for(activity, commitment, timeout)
                .map_err(WalletToolError::Rpc);
            let outcome = classify(&result);
            (result, outcome)
        })
        .map_err(WalletToolError::Server)?
}
fn payment_arguments(execution: &PaymentExecution<'_>) -> Result<Vec<u8>, WalletToolError> {
    let payload_length =
        u64::try_from(execution.canonical_payload.len()).map_err(|_| WalletToolError::Arguments)?;
    let actor_length =
        u64::try_from(execution.options.actor.len()).map_err(|_| WalletToolError::Arguments)?;
    let mut arguments = payload_length.to_be_bytes().to_vec();
    arguments.extend_from_slice(execution.canonical_payload);
    arguments.extend_from_slice(&actor_length.to_be_bytes());
    arguments.extend_from_slice(execution.options.actor.as_bytes());
    arguments.extend_from_slice(&execution.options.idempotency_key);
    arguments.extend_from_slice(&execution.options.fee_limit.to_be_bytes());
    arguments.extend_from_slice(&execution.options.not_before.to_be_bytes());
    arguments.extend_from_slice(&execution.options.not_after.to_be_bytes());
    arguments.extend_from_slice(execution.options.commitment.as_str().as_bytes());
    arguments.extend_from_slice(&execution.options.wait_timeout.as_millis().to_be_bytes());
    Ok(arguments)
}

fn classify(result: &Result<VerifiedRpcReceipt, WalletToolError>) -> InvocationOutcome {
    match result {
        Ok(receipt) => {
            if receipt
                .receipt()
                .protocol()
                .is_some_and(|p| p.result_code() == 0)
            {
                InvocationOutcome::Completed
            } else {
                InvocationOutcome::Refused
            }
        }
        Err(WalletToolError::Rpc(RpcError::Pending { .. } | RpcError::Transport)) => {
            InvocationOutcome::Unknown
        }
        Err(
            WalletToolError::Policy
            | WalletToolError::Scope
            | WalletToolError::Arguments
            | WalletToolError::Evidence,
        ) => InvocationOutcome::Refused,
        Err(_) => InvocationOutcome::Failed,
    }
}
struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
    fn wake_by_ref(self: &Arc<Self>) {
        self.0.unpark();
    }
}
fn run<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn pending_and_refused_wallet_results_never_complete() {
        assert_eq!(
            classify(&Err(WalletToolError::Rpc(RpcError::Pending {
                activity_id: [1; 32]
            }))),
            InvocationOutcome::Unknown
        );
        assert_eq!(
            classify(&Err(WalletToolError::Policy)),
            InvocationOutcome::Refused
        );
        assert_eq!(
            classify(&Err(WalletToolError::Rpc(RpcError::Verification))),
            InvocationOutcome::Failed
        );
    }
}
