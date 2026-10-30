use std::time::Duration;

use layerx_crypto::disclosure::call::{bind_call, encode_payment_call};
use layerx_crypto::disclosure::Disclosure;
use layerx_crypto::evm_transaction::{assemble, recover_sender, transaction_hash, Eip1559Call};
pub use layerx_crypto::payments::{Grant, Payment, Registration};
pub use layerx_crypto::send::SendDebit;
use layerx_crypto::send::{ledger_payment_call, CallOptions};
use layerx_crypto::signer::{sign_disclosed_call, SignError, Signer};
use layerx_types::payload::ModuleId;
use serde_json::Value;

use crate::rpc::{BalancesSnapshot, Commitment, RpcClient, RpcError};
use crate::rpc_verification::{ReceiptPolicy, VerifiedRpcReceipt};

pub struct PaymentOptions {
    pub actor: String,
    pub idempotency_key: [u8; 32],
    pub fee_limit: u128,
    pub not_before: u64,
    pub not_after: u64,
    pub commitment: Commitment,
    pub wait_timeout: Duration,
}

pub struct Wallet<'a> {
    pub rpc: &'a RpcClient,
    pub signer: &'a dyn Signer,
    pub policy: &'a ReceiptPolicy,
    pub native_asset: [u8; 32],
}

pub struct PreparedPayment {
    call: Eip1559Call,
    from: [u8; 20],
    canonical: Vec<u8>,
    disclosure: Disclosure,
}

impl PreparedPayment {
    #[must_use]
    pub const fn disclosure(&self) -> &Disclosure {
        &self.disclosure
    }
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical
    }
}

impl<'Wallet> Wallet<'Wallet> {
    #[must_use]
    pub const fn token(&self) -> &Self {
        self
    }
    #[must_use]
    pub const fn grant(&self) -> &Self {
        self
    }
    /// # Errors
    /// Preserves unavailable enumeration and RPC errors.
    pub fn accounts(&self, did: &str) -> Result<BalancesSnapshot, RpcError> {
        self.rpc.get_balances(did)
    }
    /// # Errors
    /// Rejects invalid selectors and preserves RPC read errors.
    pub fn balance(&self, did: &str, asset: [u8; 32]) -> Result<Value, RpcError> {
        self.rpc.wallet(self.native_asset).balance(did, asset)
    }
    /// # Errors
    /// Refuses malformed signed Send authorization, stale source sequence or signer rejection.
    pub async fn send(
        &self,
        canonical_send_payload: &[u8],
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        let prepared = self.prepare_payload(ModuleId::Asset, 5, canonical_send_payload, options)?;
        let hash = self.execute(prepared, options).await?;
        self.wait_for(hash, options.commitment, options.wait_timeout)
    }
    /// # Errors
    /// Refuses scope mismatches, stale account state, signer refusal or missing receipt evidence.
    pub async fn send_debit(
        &self,
        debit: &SendDebit,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        if debit.network_id != self.policy.network_id
            || debit.protocol_version != self.policy.protocol_version
            || debit.idempotency_key != options.idempotency_key
        {
            return Err(RpcError::InvalidRequest);
        }
        if source_sequence(self.rpc, debit.from)? != debit.source_sequence {
            return Err(RpcError::StaleSourceSequence);
        }
        let payload = debit.sign(self.signer).await.map_err(RpcError::Signing)?;
        self.send(&payload, options).await
    }

    /// # Errors
    /// Preserves prepare, signing, submission and verification refusals.
    pub async fn open_account(
        &self,
        asset: [u8; 32],
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(Payment::OpenAccount { asset }, options).await
    }
    /// # Errors
    /// Refuses malformed registration and preserves execution errors.
    pub async fn create(
        &self,
        registration: Registration,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(Payment::Register(registration), options).await
    }
    /// # Errors
    /// Preserves canonical encoding, signing and native execution errors.
    pub async fn mint(
        &self,
        asset: [u8; 32],
        to: [u8; 32],
        amount: u128,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(Payment::Mint { asset, to, amount }, options)
            .await
    }
    /// # Errors
    /// Preserves canonical encoding, signing and native execution errors.
    pub async fn burn(
        &self,
        asset: [u8; 32],
        from: [u8; 32],
        amount: u128,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(
            Payment::Burn {
                asset,
                from,
                amount,
            },
            options,
        )
        .await
    }
    /// # Errors
    /// Refuses malformed grants and preserves signing and execution errors.
    pub async fn issue(
        &self,
        grant: Grant,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(Payment::IssueGrant(grant), options).await
    }
    /// # Errors
    /// Preserves prepare, signing and native execution errors.
    pub async fn revoke(
        &self,
        grant: [u8; 32],
        revocation_sequence: u64,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        self.payment(
            Payment::RevokeGrant {
                grant,
                revocation_sequence,
            },
            options,
        )
        .await
    }
    /// # Errors
    /// Requires the canonical Receive variant and preserves prepare/sign/execute errors.
    pub async fn draw(
        &self,
        receive: Payment,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        if !matches!(receive, Payment::Receive { .. }) {
            return Err(RpcError::InvalidRequest);
        }
        self.payment(receive, options).await
    }
    /// # Errors
    /// Returns pending or refuses any missing or invalid commitment evidence.
    pub fn wait_for(
        &self,
        activity: [u8; 32],
        commitment: Commitment,
        timeout: Duration,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        if timeout > Duration::from_secs(300) || activity == [0; 32] {
            return Err(RpcError::InvalidRequest);
        }
        if commitment == Commitment::Finalised {
            return Err(RpcError::MissingFinalityTrust);
        }
        let deadline = std::time::Instant::now() + timeout;
        loop {
            let remaining = deadline.saturating_duration_since(std::time::Instant::now());
            if remaining.is_zero() {
                return Err(RpcError::Pending {
                    activity_id: activity,
                });
            }
            let result = self.rpc.sequencer_receipt_bounded(activity, remaining)?;
            if result.get("txHash").and_then(Value::as_str)
                != Some(&format!("0x{}", crate::rpc::encode_hex(&activity)))
            {
                return Err(RpcError::InvalidResponse);
            }
            let rung = result["rung"].as_str().ok_or(RpcError::InvalidResponse)?;
            if !matches!(
                rung,
                "submitted"
                    | "receipted"
                    | "included"
                    | "executed"
                    | "batched"
                    | "committed"
                    | "finalised"
            ) {
                return Err(RpcError::InvalidResponse);
            }
            if matches!(
                rung,
                "receipted" | "included" | "executed" | "batched" | "committed" | "finalised"
            ) {
                crate::rpc::check_transaction_revert(&result)?;
                if commitment == Commitment::Executed
                    || matches!(rung, "batched" | "committed" | "finalised")
                {
                    return self.rpc.wait_for(
                        activity,
                        commitment,
                        self.policy,
                        deadline.saturating_duration_since(std::time::Instant::now()),
                    );
                }
            }
            std::thread::sleep(
                Duration::from_millis(20)
                    .min(deadline.saturating_duration_since(std::time::Instant::now())),
            );
        }
    }

    /// # Errors
    /// Refuses malformed payment payloads and propagates preparation, signing and verification errors.
    pub async fn payment(
        &self,
        payment: Payment,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        let (module, ordinal) = payment.activity_type();
        let payload = payment
            .encode(options.actor.as_bytes())
            .map_err(|_| RpcError::InvalidRequest)?;
        let prepared = self.prepare_payload(module, ordinal, &payload, options)?;
        let hash = self.execute(prepared, options).await?;
        self.wait_for(hash, options.commitment, options.wait_timeout)
    }

    /// # Errors
    /// Requires canonical payload disclosure and fresh source and identity sequences.
    pub fn prepare_payload(
        &self,
        module: ModuleId,
        ordinal: u16,
        payload: &[u8],
        options: &PaymentOptions,
    ) -> Result<PreparedPayment, RpcError> {
        if options.wait_timeout > Duration::from_secs(300) {
            return Err(RpcError::InvalidRequest);
        }
        if options.commitment == Commitment::Finalised
            && self.policy.trusted_checkpoint_context_digest.is_none()
        {
            return Err(RpcError::MissingFinalityTrust);
        }
        let from = self
            .signer
            .evm_address()
            .ok_or(RpcError::Signing(SignError::EvmKeyUnavailable))?;
        let sequence = self
            .rpc
            .get_identity_sequence(&format!("0x{}", crate::rpc::encode_hex(&from)))?;
        let (chain_id, max_priority_fee_per_gas, max_fee_per_gas) =
            self.rpc.eip1559_parameters()?;
        let gas_limit = u64::try_from(options.fee_limit / max_fee_per_gas)
            .map_err(|_| RpcError::InvalidRequest)?;
        if gas_limit == 0 {
            return Err(RpcError::InvalidRequest);
        }
        let mut prepared = prepare_payload(
            module,
            ordinal,
            payload,
            options,
            &CallOptions {
                from,
                chain_id,
                nonce: sequence,
                max_priority_fee_per_gas,
                max_fee_per_gas,
                gas_limit,
            },
        )?;
        let gas = self.rpc.estimate_fee_from(&prepared.call, from)?;
        if gas == 0 || gas > gas_limit {
            return Err(RpcError::InvalidRequest);
        }
        prepared.call.gas_limit = gas;
        prepared.disclosure =
            bind_call(&prepared.call, from).map_err(|_| RpcError::Verification)?;
        if let Some(payload_sequence) = prepared
            .disclosure
            .payload_sequence()
            .map_err(|_| RpcError::Verification)?
        {
            let source =
                if let Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerDraw {
                    recipient,
                    ..
                }) = prepared
                    .disclosure
                    .construct_method()
                    .map_err(|_| RpcError::Verification)?
                {
                    recipient
                } else {
                    prepared
                        .disclosure
                        .counterparties
                        .first()
                        .ok_or(RpcError::InvalidRequest)?
                        .account
                };
            if source_sequence(self.rpc, source)? != payload_sequence {
                return Err(RpcError::StaleSourceSequence);
            }
        }
        Ok(prepared)
    }

    /// # Errors
    /// Refuses signing and submission failures and incomplete receipt evidence.
    pub async fn execute(
        &self,
        prepared: PreparedPayment,
        _options: &PaymentOptions,
    ) -> Result<[u8; 32], RpcError> {
        let signature = sign_disclosed_call(
            self.signer,
            &prepared.call,
            &prepared.disclosure,
            prepared.from,
        )
        .await
        .map_err(RpcError::Signing)?;
        if recover_sender(&prepared.call, &signature).map_err(|_| RpcError::Verification)?
            != prepared.from
        {
            return Err(RpcError::Signing(SignError::ReturnedSignatureInvalid));
        }
        let raw = assemble(&prepared.call, &signature);
        let hash = self.rpc.send_raw_transaction(&raw)?;
        if hash != transaction_hash(&raw) {
            return Err(RpcError::InvalidResponse);
        }
        Ok(hash)
    }

    /// Issues a grant whose purpose is the textual `label`. The grant is signed over the
    /// committed purpose, so its `purpose_hash` must already equal `label.commitment()`; the
    /// grant is then issued exactly as [`Self::issue`] issues it.
    ///
    /// # Errors
    /// Returns `InvalidRequest` when the grant commits to a different purpose, and otherwise
    /// the errors of [`Self::issue`].
    pub async fn issue_with_purpose(
        &self,
        grant: Grant,
        label: &crate::purpose::PurposeLabel,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        require_grant_purpose(&grant, label)?;
        self.issue(grant, options).await
    }
    /// Draws on a grant whose purpose is the textual `label`. The payer grant is signed over
    /// the committed purpose, so its `purpose_hash` must already equal `label.commitment()`;
    /// the draw then proceeds exactly as [`Self::draw`].
    ///
    /// # Errors
    /// Returns `InvalidRequest` for a non-Receive payment or a payer grant committing to a
    /// different purpose, and otherwise the errors of [`Self::draw`].
    pub async fn draw_with_purpose(
        &self,
        receive: Payment,
        label: &crate::purpose::PurposeLabel,
        options: &PaymentOptions,
    ) -> Result<VerifiedRpcReceipt, RpcError> {
        let Payment::Receive { payer_grant, .. } = &receive else {
            return Err(RpcError::InvalidRequest);
        };
        require_grant_purpose(payer_grant, label)?;
        self.draw(receive, options).await
    }
}

fn require_grant_purpose(
    grant: &Grant,
    label: &crate::purpose::PurposeLabel,
) -> Result<(), RpcError> {
    if grant.purpose_hash == label.commitment() {
        Ok(())
    } else {
        Err(RpcError::InvalidRequest)
    }
}

fn source_sequence(rpc: &RpcClient, account: [u8; 32]) -> Result<u64, RpcError> {
    let id = crate::rpc::encode_hex(&account);
    let value = rpc.get_account(&id)?;
    if value["account_id"] != id {
        return Err(RpcError::InvalidResponse);
    }
    decimal(&value, "next_sequence")
}
fn decimal(value: &Value, key: &str) -> Result<u64, RpcError> {
    let text = value[key].as_str().ok_or(RpcError::InvalidResponse)?;
    let number: u64 = text.parse().map_err(|_| RpcError::InvalidResponse)?;
    if number.to_string() != text {
        return Err(RpcError::InvalidResponse);
    }
    Ok(number)
}

fn prepare_payload(
    module: ModuleId,
    ordinal: u16,
    payload: &[u8],
    options: &PaymentOptions,
    call_options: &CallOptions,
) -> Result<PreparedPayment, RpcError> {
    let (call, disclosure) = if module == ModuleId::Asset && ordinal == 5 {
        let disclosed =
            ledger_payment_call(payload, call_options).map_err(|_| RpcError::InvalidRequest)?;
        (disclosed.call, disclosed.disclosure)
    } else {
        let payment = Payment::decode(module, ordinal, payload, options.actor.as_bytes())
            .map_err(|_| RpcError::InvalidRequest)?;
        let data = encode_payment_call(&payment, options.actor.as_bytes())
            .map_err(|_| RpcError::InvalidRequest)?;
        let mut to = [0; 20];
        to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
        let call = Eip1559Call {
            chain_id: call_options.chain_id,
            nonce: call_options.nonce,
            max_priority_fee_per_gas: call_options.max_priority_fee_per_gas,
            max_fee_per_gas: call_options.max_fee_per_gas,
            gas_limit: call_options.gas_limit,
            to,
            data,
        };
        let disclosure = bind_call(&call, call_options.from).map_err(|_| RpcError::Verification)?;
        (call, disclosure)
    };
    let canonical = call.data.clone();
    Ok(PreparedPayment {
        call,
        from: call_options.from,
        canonical,
        disclosure,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn prepared_receive_retains_independent_identity_and_source_domains() -> Result<(), RpcError> {
        let mut call_options = CallOptions {
            from: [9; 20],
            chain_id: 17,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
        };
        let options = PaymentOptions {
            actor: "did:layerx:alice".into(),
            idempotency_key: [0x71; 32],
            fee_limit: 20,
            not_before: 10,
            not_after: 100,
            commitment: Commitment::Executed,
            wait_timeout: Duration::from_secs(1),
        };
        let payload =
            include_str!("../../layerx-crypto/tests/fixtures/payments/1-6-source-sequence-23.hex")
                .trim()
                .as_bytes()
                .chunks_exact(2)
                .map(|pair| {
                    let text = std::str::from_utf8(pair).map_err(|_| RpcError::InvalidRequest)?;
                    u8::from_str_radix(text, 16).map_err(|_| RpcError::InvalidRequest)
                })
                .collect::<Result<Vec<_>, _>>()?;
        let prepared = prepare_payload(ModuleId::Asset, 6, &payload, &options, &call_options)?;
        assert_eq!(prepared.disclosure().envelope_sequence(), 7);
        assert_eq!(
            prepared
                .disclosure()
                .payload_sequence()
                .map_err(|_| RpcError::Verification)?,
            Some(23)
        );
        assert_eq!(prepared.call.data, prepared.canonical_bytes());
        let mint = Payment::Mint {
            asset: [3; 32],
            to: [2; 32],
            amount: 6,
        }
        .encode(options.actor.as_bytes())
        .map_err(|_| RpcError::InvalidRequest)?;
        call_options.nonce = 8;
        let prepared_mint = prepare_payload(ModuleId::Asset, 10, &mint, &options, &call_options)?;
        assert_eq!(prepared_mint.disclosure().envelope_sequence(), 8);
        assert_eq!(
            prepared_mint
                .disclosure()
                .payload_sequence()
                .map_err(|_| RpcError::Verification)?,
            None
        );
        Ok(())
    }

    fn labelled_grant(purpose_hash: [u8; 32]) -> Grant {
        Grant {
            id: [1; 32],
            from: [2; 32],
            recipient: [3; 32],
            asset: [4; 32],
            per_draw_maximum: 5,
            allowance: 10,
            recurring: false,
            window_length: 0,
            expiration: 100,
            purpose_hash,
            has_reference: false,
            reference_hash: [0; 32],
            revocation_sequence: 0,
            public_key: [5; 32],
            signature: [6; 64],
        }
    }

    #[test]
    fn issue_with_purpose_requires_the_committed_label() -> Result<(), RpcError> {
        let label =
            crate::purpose::PurposeLabel::new("rent").map_err(|_| RpcError::InvalidRequest)?;
        require_grant_purpose(&labelled_grant(label.commitment()), &label)?;
        assert!(matches!(
            require_grant_purpose(&labelled_grant([7; 32]), &label),
            Err(RpcError::InvalidRequest)
        ));
        Ok(())
    }

    #[test]
    fn draw_with_purpose_requires_the_payer_grant_label() -> Result<(), RpcError> {
        let label =
            crate::purpose::PurposeLabel::new("rent").map_err(|_| RpcError::InvalidRequest)?;
        let other =
            crate::purpose::PurposeLabel::new("rent ").map_err(|_| RpcError::InvalidRequest)?;
        let grant = labelled_grant(label.commitment());
        require_grant_purpose(&grant, &label)?;
        assert!(matches!(
            require_grant_purpose(&grant, &other),
            Err(RpcError::InvalidRequest)
        ));
        Ok(())
    }
}
