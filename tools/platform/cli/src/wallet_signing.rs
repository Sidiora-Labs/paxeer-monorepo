use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::{bind_call, ledger_payment_call, CallOptions, Disclosure, Payment};
use serde_json::{json, Value};

pub struct EvmSigningKey {
    secret: zeroize::Zeroizing<[u8; 32]>,
    address: [u8; 20],
    public_key: [u8; 32],
}

#[derive(serde::Deserialize)]
struct ExportedKeys {
    evm_address: String,
    evm_private_key: String,
    layerx_seed: String,
    did: String,
    index: u32,
}

impl Drop for ExportedKeys {
    fn drop(&mut self) {
        use zeroize::Zeroize as _;
        self.evm_private_key.zeroize();
        self.layerx_seed.zeroize();
    }
}

fn fixed_key_bytes<const N: usize>(name: &str, text: &str) -> Result<[u8; N], String> {
    if text.len() != N * 2 || !text.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(format!("invalid {name}"));
    }
    let mut bytes = [0; N];
    for (byte, pair) in bytes.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        let pair = std::str::from_utf8(pair).map_err(|_| format!("invalid {name}"))?;
        *byte = u8::from_str_radix(pair, 16).map_err(|_| format!("invalid {name}"))?;
    }
    Ok(bytes)
}

impl EvmSigningKey {
    pub fn read(path: &std::path::Path, actor: &str, public_key: [u8; 32]) -> Result<Self, String> {
        use std::io::Read as _;
        use std::os::unix::fs::PermissionsExt as _;
        let file = std::fs::File::open(path).map_err(|e| format!("evm_key_unavailable: {e}"))?;
        let metadata = file.metadata().map_err(debug)?;
        if !metadata.is_file() || metadata.permissions().mode() & 0o077 != 0 {
            return Err("evm_key_refused: key file must be an owner-only regular file".into());
        }
        let mut text = zeroize::Zeroizing::new(String::new());
        file.take(4097).read_to_string(&mut text).map_err(debug)?;
        if text.len() > 4096 {
            return Err("evm_key_refused: key file exceeds 4096 bytes".into());
        }
        let exported: ExportedKeys = serde_json::from_str(&text)
            .map_err(|_| "evm_key_refused: expected a wallet derive private-key export")?;
        let seed =
            zeroize::Zeroizing::new(fixed_key_bytes::<32>("LayerX seed", &exported.layerx_seed)?);
        if exported.did != actor
            || exported.index >= 0x8000_0000
            || ed25519_dalek::SigningKey::from_bytes(&seed)
                .verifying_key()
                .to_bytes()
                != public_key
        {
            return Err(
                "evm_key_refused: export does not match the selected wallet identity".into(),
            );
        }
        Ok(Self {
            public_key,
            secret: zeroize::Zeroizing::new(fixed_key_bytes(
                "EVM private key",
                &exported.evm_private_key,
            )?),
            address: fixed_key_bytes(
                "EVM address",
                exported
                    .evm_address
                    .strip_prefix("0x")
                    .ok_or("invalid EVM address")?,
            )?,
        })
    }

    #[must_use]
    pub const fn address(&self) -> [u8; 20] {
        self.address
    }
}

pub struct PreparedTransaction {
    call: layerx_crypto::evm_transaction::Eip1559Call,
    from: [u8; 20],
    disclosure: Disclosure,
}

impl PreparedTransaction {
    pub fn new(
        call: layerx_crypto::evm_transaction::Eip1559Call,
        from: [u8; 20],
    ) -> Result<Self, String> {
        if call.chain_id != 125 {
            return Err("wallet writes require chainId 125".into());
        }
        if call.gas_limit == 0
            || call.max_fee_per_gas == 0
            || call.max_priority_fee_per_gas > call.max_fee_per_gas
        {
            return Err("wallet writes require positive gas and valid EIP-1559 fee bounds".into());
        }
        let disclosure = bind_call(&call, from).map_err(debug)?;
        Ok(Self {
            call,
            from,
            disclosure,
        })
    }

    #[must_use]
    pub fn confirmation(&self) -> Value {
        json!({
            "chain_id":self.call.chain_id, "nonce":self.call.nonce.to_string(),
            "from":format!("0x{}", hex(&self.from)), "to":format!("0x{}", hex(&self.call.to)),
            "gas":self.call.gas_limit.to_string(), "max_fee_per_gas":self.call.max_fee_per_gas.to_string(),
            "max_priority_fee_per_gas":self.call.max_priority_fee_per_gas.to_string(),
            "fee_limit":self.disclosure.fee_limit.to_string(), "asset":hex(&self.disclosure.asset),
            "payment":format!("{:?}", self.disclosure.payment), "data":format!("0x{}", hex(&self.call.data)),
        })
    }

    pub fn sign(self, key: &EvmSigningKey) -> Result<(Vec<u8>, [u8; 32]), String> {
        use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
        if key.address != self.from
            || bind_call(&self.call, self.from).map_err(debug)? != self.disclosure
        {
            return Err("wallet signer does not match the disclosed EVM call".into());
        }
        let raw = self.call.sign(&key.secret).map_err(debug)?;
        let (decoded, signature) = decode_signed(&raw).map_err(debug)?;
        if decoded != self.call || recover_sender(&decoded, &signature).map_err(debug)? != self.from
        {
            return Err("wallet EVM key does not match the disclosed sender".into());
        }
        let hash = transaction_hash(&raw);
        Ok((raw, hash))
    }
}

pub struct SigningFacts<'a> {
    pub actor: &'a str,
    pub public_key: [u8; 32],
    pub network_id: u32,
    pub identity_next_sequence: u64,
    pub not_before_ms: u64,
    pub expires_at_ms: u64,
    pub fee_limit: u128,
    pub idempotency_key: [u8; 32],
}

pub struct PreparedPayment {
    call: Eip1559Call,
    from: [u8; 20],
    disclosure: Disclosure,
    public_key: [u8; 32],
    source_next_sequence: Option<u64>,
}

impl PreparedPayment {
    /// # Errors
    /// Requires a canonical payment and valid EVM transaction fee bounds.
    pub fn new(
        payment: &Payment,
        facts: &SigningFacts<'_>,
        options: &CallOptions,
    ) -> Result<Self, String> {
        let call = crate::wallet_encoding::payment_call(payment, facts.actor, options)?;
        Self::from_call(call, facts, options)
    }

    /// # Errors
    /// Requires a valid authorized Send and independent EVM transaction facts.
    pub fn from_send(
        payload: &[u8],
        facts: &SigningFacts<'_>,
        options: &CallOptions,
    ) -> Result<Self, String> {
        let disclosed = ledger_payment_call(payload, options).map_err(debug)?;
        let authorization = payload
            .get(payload.len().saturating_sub(134)..)
            .ok_or("Send authorization is incomplete")?;
        if authorization.get(..32) != Some(facts.public_key.as_slice())
            || authorization.get(128..132) != Some(facts.network_id.to_be_bytes().as_slice())
            || authorization.get(132..134) != Some(3_u16.to_be_bytes().as_slice())
            || disclosed.disclosure.idempotency_key != facts.idempotency_key
            || disclosed.disclosure.expiry.payload_expires_at != facts.expires_at_ms
        {
            return Err("Send authorization and transaction facts differ".into());
        }
        Self::from_call(disclosed.call, facts, options)
    }

    fn from_call(
        call: Eip1559Call,
        facts: &SigningFacts<'_>,
        options: &CallOptions,
    ) -> Result<Self, String> {
        if facts.expires_at_ms <= facts.not_before_ms
            || facts.expires_at_ms - facts.not_before_ms > 300_000
        {
            return Err("wallet validity must be nonempty and at most 300000 milliseconds".into());
        }
        if options.nonce != facts.identity_next_sequence {
            return Err("wallet transaction nonce differs from the identity sequence".into());
        }
        let prepared = PreparedTransaction::new(call, options.from)?;
        if prepared.disclosure.fee_limit > facts.fee_limit {
            return Err("wallet transaction exceeds the disclosed fee limit".into());
        }
        let source_next_sequence = prepared.disclosure.payload_sequence().map_err(debug)?;
        Ok(Self {
            source_next_sequence,
            call: prepared.call,
            from: prepared.from,
            disclosure: prepared.disclosure,
            public_key: facts.public_key,
        })
    }

    #[must_use]
    pub const fn call(&self) -> &Eip1559Call {
        &self.call
    }

    #[must_use]
    pub fn confirmation(&self) -> Value {
        json!({
            "chain_id": self.call.chain_id,
            "nonce": self.call.nonce.to_string(),
            "from": format!("0x{}", hex(&self.from)),
            "to": format!("0x{}", hex(&self.call.to)),
            "gas": self.call.gas_limit.to_string(),
            "max_fee_per_gas": self.call.max_fee_per_gas.to_string(),
            "max_priority_fee_per_gas": self.call.max_priority_fee_per_gas.to_string(),
            "source_account_sequence": self.source_next_sequence.map(|n| n.to_string()),
            "payload_expires_at": self.disclosure.expiry.payload_expires_at.to_string(),
            "activity_type": self.disclosure.activity_type.value(),
            "asset": hex(&self.disclosure.asset),
            "fee_limit": self.disclosure.fee_limit.to_string(),
            "idempotency_key": hex(&self.disclosure.idempotency_key),
            "payment": format!("{:?}", self.disclosure.payment),
            "data": format!("0x{}", hex(&self.call.data)),
            "counterparties": self.disclosure.counterparties.iter().map(|p| json!({"role":format!("{:?}", p.role),"account":hex(&p.account)})).collect::<Vec<_>>(),
            "amounts": self.disclosure.amounts.iter().map(|a| json!({"role":format!("{:?}", a.role),"value":a.value.to_string()})).collect::<Vec<_>>(),
        })
    }

    /// # Errors
    /// Requires the EVM key exported for the disclosed wallet identity.
    pub fn sign(self, key: &EvmSigningKey) -> Result<Vec<u8>, String> {
        self.sign_with_id(key).map(|(bytes, _)| bytes)
    }

    /// # Errors
    /// Rebinds the ledger disclosure and verifies the signed transaction and sender.
    pub fn sign_with_id(self, key: &EvmSigningKey) -> Result<(Vec<u8>, [u8; 32]), String> {
        if key.public_key != self.public_key {
            return Err("wallet signer does not match the disclosed owner".into());
        }
        PreparedTransaction {
            call: self.call,
            from: self.from,
            disclosure: self.disclosure,
        }
        .sign(key)
    }
}

fn debug(error: impl std::fmt::Debug) -> String {
    format!("{error:?}")
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        output.push(char::from(DIGITS[usize::from(b >> 4)]));
        output.push(char::from(DIGITS[usize::from(b & 15)]));
    }
    output
}

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_crypto::account_derivation::derive_from_seed;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    use layerx_crypto::payments::{asset_id, Registration};
    use sha2::Digest as _;

    #[test]
    fn shared_payments_sign_and_verify_exact_identity_sequence() -> Result<(), String> {
        let account = derive_from_seed(&[7; 64], 0).map_err(debug)?;
        let actor = account.did();
        let public_key = ed25519_dalek::SigningKey::from_bytes(account.layerx_seed())
            .verifying_key()
            .to_bytes();
        let key = EvmSigningKey {
            secret: zeroize::Zeroizing::new(*account.evm_secret().ok_or("EVM key missing")?),
            address: account.evm_address(),
            public_key,
        };
        let mut h = sha2::Sha256::new();
        h.update(b"LXP/v1/did-id\0");
        h.update(u16::try_from(actor.len()).map_err(debug)?.to_be_bytes());
        h.update(actor.as_bytes());
        let asset = asset_id(&h.finalize().into(), &[9; 32]);
        let facts = SigningFacts {
            actor: &actor,
            public_key,
            network_id: 402,
            identity_next_sequence: 17,
            not_before_ms: 1000,
            expires_at_ms: 2000,
            fee_limit: 42000,
            idempotency_key: [8; 32],
        };
        let options = CallOptions {
            from: key.address,
            chain_id: 125,
            nonce: 17,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 21000,
        };
        for payment in [
            Payment::Register(Registration {
                asset,
                salt: [9; 32],
                symbol: "PAY".into(),
                name: "Payments".into(),
                decimals: 6,
                supply_cap: 1000,
                issuer_kind: 1,
                custody_ref: vec![],
            }),
            Payment::OpenAccount { asset },
            Payment::Mint {
                asset,
                to: [3; 32],
                amount: 500,
            },
            Payment::Burn {
                asset,
                from: [3; 32],
                amount: 1,
            },
            Payment::RevokeGrant {
                grant: [4; 32],
                revocation_sequence: 9,
            },
        ] {
            let prepared = PreparedPayment::new(&payment, &facts, &options)?;
            assert_eq!(prepared.confirmation()["nonce"], "17");
            let call = prepared.call().clone();
            assert_eq!(
                call.data,
                crate::wallet_encoding::payment_calldata(&payment, &actor)?
            );
            let (bytes, id) = prepared.sign_with_id(&key)?;
            let (decoded, signature) = decode_signed(&bytes).map_err(debug)?;
            assert_eq!(decoded, call);
            assert_eq!(
                recover_sender(&decoded, &signature).map_err(debug)?,
                key.address
            );
            assert_eq!(transaction_hash(&bytes), id);
            assert_eq!(
                bind_call(&decoded, key.address).map_err(debug)?.payment,
                Some(payment)
            );
        }
        let wrong_key = EvmSigningKey {
            public_key: [8; 32],
            ..key
        };
        assert!(
            PreparedPayment::new(&Payment::OpenAccount { asset }, &facts, &options)?
                .sign(&wrong_key)
                .is_err()
        );
        Ok(())
    }
}
