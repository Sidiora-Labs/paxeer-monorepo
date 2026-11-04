use super::funded_inputs::{sha256, NETWORK_ID, PROTOCOL_VERSION};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::{
    disclosure::DisclosureError,
    payments::{Grant, Payment, ReceiverAuthorization},
};
use layerx_wire::{encode::Encoder, hash::Domain};

pub struct Funding {
    pub recipient_seed: [u8; 32],
    pub recipient_did: String,
}

pub struct GrantRequest<'a> {
    pub payer_seed: &'a [u8; 32],
    pub payer_did: &'a str,
    pub recipient_did: &'a str,
    pub asset: [u8; 32],
    pub per_draw_maximum: u128,
    pub allowance: u128,
    pub recurring_window: Option<u64>,
    pub expiration: u64,
    pub purpose_hash: [u8; 32],
    pub revocation_sequence: u64,
}

pub fn payer_grant(request: &GrantRequest<'_>) -> Result<Grant, DisclosureError> {
    let key = SigningKey::from_bytes(request.payer_seed);
    let mut grant = Grant {
        id: [0; 32],
        from: layerx_platform_core::main_account(request.payer_did)
            .map_err(|_| DisclosureError::MalformedPayload)?,
        recipient: layerx_platform_core::main_account(request.recipient_did)
            .map_err(|_| DisclosureError::MalformedPayload)?,
        asset: request.asset,
        per_draw_maximum: request.per_draw_maximum,
        allowance: request.allowance,
        recurring: request.recurring_window.is_some(),
        window_length: request.recurring_window.unwrap_or(0),
        expiration: request.expiration,
        purpose_hash: request.purpose_hash,
        has_reference: false,
        reference_hash: [0; 32],
        revocation_sequence: request.revocation_sequence,
        public_key: key.verifying_key().to_bytes(),
        signature: [0; 64],
    };
    grant.id = grant.signing_digest()?;
    grant.signature = key.sign(&grant.id).to_bytes();
    Payment::IssueGrant(grant.clone()).encode(request.payer_did.as_bytes())?;
    Ok(grant)
}

pub fn receive(
    seed: &[u8; 32],
    grant: &Grant,
    sequence: u64,
    idempotency_key: [u8; 32],
    amount: u128,
) -> Result<Payment, DisclosureError> {
    let key = SigningKey::from_bytes(seed);
    let context_hash = if grant.has_reference {
        sha256(&[
            Domain::ContextHash.tag(),
            &grant.purpose_hash,
            &grant.reference_hash,
        ])
    } else {
        sha256(&[Domain::ContextHash.tag(), &grant.purpose_hash])
    };
    let mut e = Encoder::new(512);
    e.fixed(b"LXP:RECEIVE:v1")?;
    e.fixed(&grant.from)?;
    e.fixed(&grant.recipient)?;
    e.fixed(&grant.asset)?;
    e.u128(amount)?;
    e.fixed(&grant.id)?;
    e.u64(sequence)?;
    e.fixed(&idempotency_key)?;
    e.fixed(&context_hash)?;
    e.u8(1)?;
    e.fixed(&grant.recipient)?;
    e.fixed(&context_hash)?;
    e.u32(NETWORK_ID)?;
    e.u16(PROTOCOL_VERSION)?;
    let digest = sha256(&[Domain::SignaturePreimage.tag(), &e.finish()]);
    Ok(Payment::Receive {
        from: grant.from,
        to: grant.recipient,
        asset: grant.asset,
        amount,
        grant: grant.id,
        sequence,
        idempotency_key,
        context_hash,
        receiver_authorization: ReceiverAuthorization {
            kind: 1,
            controller: grant.recipient,
            public_key: key.verifying_key().to_bytes(),
            signature: key.sign(&digest).to_bytes(),
            signed_context_hash: context_hash,
            network_id: NETWORK_ID,
            protocol_version: PROTOCOL_VERSION,
        },
        payer_grant: Box::new(grant.clone()),
    })
}
