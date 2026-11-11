use layerx_crypto::{ed25519, SignatureMessage};
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::decode::Decoder;
use layerx_wire::hash::{self, Domain};
use layerx_wire::historical::activity::{self, Activity};

const CONSENT_HEADER: [u8; 4] = [0x71, 1, 3, 5];
const REGISTRATION_HEADER: [u8; 4] = [0x71, 1, 2, 1];
const FIELD_PRIME: [u8; 32] = [
    0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
];

#[derive(Clone, Debug, Eq, PartialEq)]
pub(super) struct Consent {
    pub target: Did,
    pub target_public_key: [u8; 32],
    sponsor: [u8; 32],
    action_key: [u8; 32],
    expires_at: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(super) struct SignedRegistration {
    pub consent: Consent,
    network_id: u32,
    not_before: u64,
    payload: Vec<u8>,
}

impl SignedRegistration {
    pub fn decode(payload: &[u8]) -> Result<Self, ()> {
        if payload.len() < 8 || payload.len() > 1024 || payload[..4] != REGISTRATION_HEADER {
            return Err(());
        }
        let mut reader = Decoder::new(&payload[4..], 1024);
        let signed = reader.bytes(1016).map_err(|_| ())?;
        reader.finish().map_err(|_| ())?;
        if signed.len() > 1016 {
            return Err(());
        }
        let activity = activity::decode_signed(signed, &registry()?).map_err(|_| ())?;
        if activity::encode_signed(&activity).map_err(|_| ())? != signed
            || activity.protocol_version() != 3
            || activity.network_id() == 0
            || activity.account_sequence() != 0
            || activity.fee_limit() != 0
            || layerx_wire::historical::hash::payload_hash(&activity).map_err(|_| ())?
                != activity.payload_hash()
        {
            return Err(());
        }
        let key: [u8; 32] = activity.authority().try_into().map_err(|_| ())?;
        let target = Did::new(activity.actor_did()).map_err(|_| ())?;
        let consent = Consent::decode(activity.payload(), target, key)?;
        let bound = activity.timestamp_bound();
        if activity.idempotency_key() != consent.action_key
            || bound.not_after != consent.expires_at
            || bound.not_before >= bound.not_after
        {
            return Err(());
        }
        let unsigned = activity::encode_unsigned(&activity).map_err(|_| ())?;
        let signature = activity.signature().ok_or(())?.try_into().map_err(|_| ())?;
        let message = SignatureMessage::new(
            Domain::SignaturePreimage,
            3,
            activity.network_id(),
            &unsigned,
        )
        .map_err(|_| ())?;
        ed25519::verify(&key, signature, message).map_err(|_| ())?;
        Ok(Self {
            consent,
            network_id: activity.network_id(),
            not_before: bound.not_before,
            payload: payload.to_vec(),
        })
    }

    pub fn validate_outer(&self, outer: &Activity) -> Result<(), ()> {
        let sponsor = Did::new(outer.actor_did()).map_err(|_| ())?;
        let bound = outer.timestamp_bound();
        if outer.protocol_version() != 3
            || outer.network_id() != self.network_id
            || outer.activity_type().value() != 0x0007_0001
            || outer.authority().len() != 32
            || hash::did_id_for_protocol(&sponsor, 3).map_err(|_| ())? != self.consent.sponsor
            || outer.idempotency_key() != self.consent.action_key
            || bound.not_before < self.not_before
            || bound.not_after > self.consent.expires_at
            || bound.not_before >= bound.not_after
            || outer.payload() != self.payload
        {
            return Err(());
        }
        Ok(())
    }
}

impl Consent {
    fn decode(payload: &[u8], target: Did, target_public_key: [u8; 32]) -> Result<Self, ()> {
        if payload.len() != 140 || payload[..4] != CONSENT_HEADER {
            return Err(());
        }
        let mut reader = Decoder::new(&payload[4..], 1024);
        let sponsor: [u8; 32] = reader
            .fixed(32)
            .map_err(|_| ())?
            .try_into()
            .map_err(|_| ())?;
        let native_asset: [u8; 32] = reader
            .fixed(32)
            .map_err(|_| ())?
            .try_into()
            .map_err(|_| ())?;
        let account: [u8; 32] = reader
            .fixed(32)
            .map_err(|_| ())?
            .try_into()
            .map_err(|_| ())?;
        let action_key: [u8; 32] = reader
            .fixed(32)
            .map_err(|_| ())?
            .try_into()
            .map_err(|_| ())?;
        let expires_at = reader.u64().map_err(|_| ())?;
        reader.finish().map_err(|_| ())?;
        let did_id = hash::did_id_for_protocol(&target, 3).map_err(|_| ())?;
        let did = std::str::from_utf8(target.as_bytes()).map_err(|_| ())?;
        let target_account = AccountId::parse(&format!("agent:{did}:main")).map_err(|_| ())?;
        if sponsor == [0; 32]
            || sponsor == did_id
            || native_asset == [0; 32]
            || action_key == [0; 32]
            || expires_at == 0
            || !public_key_is_canonical(&target_public_key)
            || hash::account_id_for_protocol(&target_account, 3).map_err(|_| ())? != account
        {
            return Err(());
        }
        Ok(Self {
            target,
            target_public_key,
            sponsor,
            action_key,
            expires_at,
        })
    }
}

fn public_key_is_canonical(public_key: &[u8; 32]) -> bool {
    let mut y = *public_key;
    y[31] &= 0x7f;
    if y.iter().all(|byte| *byte == 0) || (y[0] == 1 && y[1..].iter().all(|byte| *byte == 0)) {
        return false;
    }
    for index in (0..32).rev() {
        if y[index] < FIELD_PRIME[index] {
            return true;
        }
        if y[index] > FIELD_PRIME[index] {
            return false;
        }
    }
    false
}

fn registry() -> Result<ModuleRegistry, ()> {
    let kind = ActivityType::new(ModuleId::Governance, 1).map_err(|_| ())?;
    let module = ModuleRegistration::new(ModuleId::Governance, &[kind]).map_err(|_| ())?;
    ModuleRegistry::new(&[module]).map_err(|_| ())
}
