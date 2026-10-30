use std::future::Future;
use std::sync::Arc;
use std::task::{Context, Poll, Wake};

use layerx_crypto::disclosure::{bind, Disclosure};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{encode_payment_envelope, EnvelopeOptions};
use layerx_crypto::signer::{sign_disclosed, Signer};
use layerx_types::activity::{Signature, UnsignedEnvelope};
use layerx_types::payload::{ActivityType, ModuleRegistry};
use layerx_wire::activity::encode_signed_envelope;
use serde_json::{json, Value};

pub struct EvmSigningKey {
    secret: zeroize::Zeroizing<[u8; 32]>,
    address: [u8; 20],
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
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, from).map_err(debug)?;
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
            || layerx_crypto::disclosure::call::bind_call(&self.call, self.from).map_err(debug)?
                != self.disclosure
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
    envelope: UnsignedEnvelope,
    canonical: Vec<u8>,
    registry: ModuleRegistry,
    disclosure: Option<Disclosure>,
    send_disclosure: Option<Value>,
    public_key: [u8; 32],
    identity_next_sequence: u64,
    source_next_sequence: Option<u64>,
}

impl PreparedPayment {
    /// # Errors
    /// Uses the shared canonical encoder and refuses incomplete disclosure before signing.
    pub fn new(payment: &Payment, facts: &SigningFacts<'_>) -> Result<Self, String> {
        let (module, ordinal) = payment.activity_type();
        let kind = ActivityType::new(module, ordinal).map_err(debug)?;
        let encoded = payment.encode(facts.actor.as_bytes()).map_err(debug)?;
        Self::from_encoded(kind, &encoded, facts, None)
    }

    /// # Errors
    /// Requires a valid shared Send payload and full canonical disclosure.
    pub fn from_send(payload: &[u8], facts: &SigningFacts<'_>) -> Result<Self, String> {
        let kind = ActivityType::new(layerx_types::payload::ModuleId::Asset, 5).map_err(debug)?;
        Self::from_encoded(kind, payload, facts, None)
    }

    pub(crate) fn from_encoded(
        kind: ActivityType,
        encoded: &[u8],
        facts: &SigningFacts<'_>,
        send_disclosure: Option<Value>,
    ) -> Result<Self, String> {
        if facts.expires_at_ms <= facts.not_before_ms
            || facts.expires_at_ms - facts.not_before_ms > 300_000
        {
            return Err("wallet validity must be nonempty and at most 300000 milliseconds".into());
        }
        let encoded = encode_payment_envelope(
            kind.module(),
            kind.ordinal(),
            encoded,
            &EnvelopeOptions {
                actor: facts.actor,
                public_key: facts.public_key,
                protocol_version: 3,
                network_id: facts.network_id,
                identity_sequence: facts.identity_next_sequence,
                idempotency_key: facts.idempotency_key,
                fee_limit: facts.fee_limit,
                not_before: facts.not_before_ms,
                not_after: facts.expires_at_ms,
            },
        )
        .map_err(debug)?;
        let source_next_sequence = encoded.disclosure.payload_sequence().map_err(debug)?;
        let envelope = encoded.envelope;
        let canonical = encoded.canonical;
        let registry = encoded.registry;
        let disclosure = if send_disclosure.is_none() {
            Some(encoded.disclosure)
        } else {
            None
        };
        Ok(Self {
            envelope,
            canonical,
            registry,
            disclosure,
            send_disclosure,
            public_key: facts.public_key,
            identity_next_sequence: facts.identity_next_sequence,
            source_next_sequence,
        })
    }

    #[must_use]
    pub fn canonical_unsigned(&self) -> &[u8] {
        &self.canonical
    }

    #[must_use]
    pub fn confirmation(&self) -> Value {
        if let Some(disclosure) = &self.disclosure {
            return json!({
                "protocol_version": self.envelope.protocol_version(),
                "network_id": self.envelope.network_id(),
                "source_account_sequence": self.source_next_sequence.map(|n| n.to_string()),
                "payload_expires_at": disclosure.expiry.payload_expires_at.to_string(),
                "envelope_sequence": self.identity_next_sequence,
                "actor": String::from_utf8_lossy(&disclosure.actor),
                "activity_type": disclosure.activity_type.value(),
                "authority": hex(&disclosure.authority),
                "asset": hex(&disclosure.asset),
                "fee_limit": disclosure.fee_limit.to_string(),
                "not_before_ms": disclosure.expiry.not_before.to_string(),
                "expires_at_ms": disclosure.expiry.not_after.to_string(),
                "idempotency_key": hex(&disclosure.idempotency_key),
                "payment": format!("{:?}", disclosure.payment),
                "canonical_unsigned": hex(&self.canonical),
                "counterparties": disclosure.counterparties.iter().map(|p| json!({"role":format!("{:?}", p.role),"account":hex(&p.account)})).collect::<Vec<_>>(),
                "amounts": disclosure.amounts.iter().map(|a| json!({"role":format!("{:?}", a.role),"value":a.value.to_string()})).collect::<Vec<_>>(),
            });
        }
        json!({
            "protocol_version": 3,
            "network_id": self.envelope.network_id(),
            "envelope_sequence": self.identity_next_sequence,
            "actor": String::from_utf8_lossy(self.envelope.actor_did().as_bytes()),
            "activity_type": self.envelope.activity_type().value(),
            "authority_kind": "owner",
            "authority": hex(self.envelope.authority().as_bytes()),
            "payload_hash": hex(&self.envelope.payload_hash()),
            "fee_limit": self.envelope.fee_limit().value().to_string(),
            "not_before_ms": self.envelope.timestamp_bound().not_before().to_string(),
            "expires_at_ms": self.envelope.timestamp_bound().not_after().to_string(),
            "idempotency_key": hex(&self.envelope.idempotency_key().bytes()),
            "payment": self.send_disclosure,
            "canonical_unsigned": hex(&self.canonical),
        })
    }

    /// # Errors
    /// Requires the exact disclosed owner and validates disclosure again inside the shared signer.
    pub fn sign(self, signer: &dyn Signer) -> Result<Vec<u8>, String> {
        self.sign_with_id(signer).map(|(bytes, _)| bytes)
    }

    /// # Errors
    /// Verifies disclosure and derives the activity identifier from the signed envelope.
    pub fn sign_with_id(self, signer: &dyn Signer) -> Result<(Vec<u8>, [u8; 32]), String> {
        if signer.public_key() != self.public_key {
            return Err("wallet signer does not match the disclosed owner".into());
        }
        let disclosure = bind(&self.canonical, &self.registry).map_err(debug)?;
        let signature = complete(sign_disclosed(
            signer,
            &self.canonical,
            &disclosure,
            &self.registry,
        ))
        .map_err(debug)?;
        let envelope = self
            .envelope
            .attach_signature(Signature::new(signature.as_bytes()).map_err(debug)?);
        let bytes = encode_signed_envelope(&envelope).map_err(debug)?;
        let decoded =
            layerx_wire::activity::decode_signed(&bytes, &self.registry).map_err(debug)?;
        let id = layerx_wire::hash::activity_id(&decoded).map_err(debug)?;
        Ok((bytes, id))
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

struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}

fn complete<F: Future>(future: F) -> F::Output {
    let waker = Arc::new(ThreadWake(std::thread::current())).into();
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
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
    use layerx_crypto::local::LocalSigner;
    use layerx_crypto::payments::{asset_id, Registration};
    use layerx_types::payload::{ModuleId, ModuleRegistration};
    use layerx_wire::activity::decode_signed;
    use sha2::Digest as _;

    #[test]
    fn shared_payments_sign_and_verify_exact_identity_sequence() -> Result<(), String> {
        let signer = LocalSigner::new([7; 32]);
        let actor = "did:layerx:alice";
        let mut h = sha2::Sha256::new();

        h.update(b"LXP/v1/did-id\0");
        h.update(u16::try_from(actor.len()).map_err(debug)?.to_be_bytes());
        h.update(actor.as_bytes());
        let issuer = h.finalize().into();
        let asset = asset_id(&issuer, &[9; 32]);
        let facts = SigningFacts {
            actor,
            public_key: signer.public_key(),
            network_id: 402,
            identity_next_sequence: 17,
            not_before_ms: 1000,
            expires_at_ms: 2000,
            fee_limit: 10,
            idempotency_key: [8; 32],
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
            let prepared = PreparedPayment::new(&payment, &facts)?;
            assert_eq!(prepared.confirmation()["envelope_sequence"], 17);
            let kind =
                ActivityType::new(ModuleId::Asset, payment.activity_type().1).map_err(debug)?;
            let registry =
                ModuleRegistry::new(&[
                    ModuleRegistration::new(ModuleId::Asset, &[kind]).map_err(debug)?
                ])
                .map_err(debug)?;
            let bytes = prepared.sign(&signer)?;
            let decoded = decode_signed(&bytes, &registry).map_err(debug)?;
            assert_eq!(decoded.account_sequence(), 17);
            assert_eq!(
                decoded.payload(),
                payment.encode(actor.as_bytes()).map_err(debug)?
            );
            let preimage = layerx_wire::sign::preimage(&decoded).map_err(debug)?;
            let signature = decoded
                .signature()
                .ok_or("signature missing")?
                .try_into()
                .map_err(debug)?;
            layerx_crypto::ed25519::verify_digest(
                &signer.public_key(),
                &signature,
                preimage.as_bytes(),
            )
            .map_err(debug)?;
        }
        let prepared = PreparedPayment::new(&Payment::OpenAccount { asset }, &facts)?;
        assert!(prepared.sign(&LocalSigner::new([8; 32])).is_err());
        Ok(())
    }
}
