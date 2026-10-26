use layerx_wire::activity::Activity;

use crate::{step::MAX_BYTES, ReplayError};

pub fn signed_activity_bytes(activity: &Activity) -> Result<Vec<u8>, ReplayError> {
    let signature = activity.signature().ok_or(ReplayError::Authority)?;
    if !(1..=3).contains(&activity.protocol_version())
        || activity.actor_did().len() > 255
        || activity.authority().len() > 524_288
        || activity.payload().len() > 524_288
        || signature.len() > 128
    {
        return Err(ReplayError::Authority);
    }
    let length = [
        activity.actor_did().len(),
        activity.authority().len(),
        activity.payload().len(),
        signature.len(),
    ]
    .into_iter()
    .try_fold(155usize, |length, field| length.checked_add(field))
    .filter(|length| *length <= MAX_BYTES)
    .ok_or(ReplayError::Authority)?;
    let mut bytes = Vec::with_capacity(length);
    bytes.extend_from_slice(&activity.protocol_version().to_be_bytes());
    bytes.extend_from_slice(&0x1001u16.to_be_bytes());
    bytes.push(12);
    bytes.push(1);
    bytes.extend_from_slice(&activity.protocol_version().to_be_bytes());
    bytes.push(2);
    bytes.extend_from_slice(&activity.network_id().to_be_bytes());
    bytes.push(3);
    bytes.extend_from_slice(&activity.activity_type().value().to_be_bytes());
    bytes.push(4);
    vector(&mut bytes, activity.actor_did())?;
    bytes.push(5);
    vector(&mut bytes, activity.authority())?;
    bytes.push(6);
    bytes.extend_from_slice(&activity.account_sequence().to_be_bytes());
    bytes.push(7);
    bytes.extend_from_slice(&activity.timestamp_bound().not_before.to_be_bytes());
    bytes.extend_from_slice(&activity.timestamp_bound().not_after.to_be_bytes());
    bytes.push(8);
    vector(&mut bytes, &activity.idempotency_key())?;
    bytes.push(9);
    bytes.extend_from_slice(&activity.fee_limit().to_be_bytes());
    bytes.push(10);
    vector(&mut bytes, &activity.payload_hash())?;
    bytes.push(11);
    vector(&mut bytes, activity.payload())?;
    bytes.push(12);
    vector(&mut bytes, signature)?;
    Ok(bytes)
}

fn vector(bytes: &mut Vec<u8>, value: &[u8]) -> Result<(), ReplayError> {
    let length = u32::try_from(value.len()).map_err(|_| ReplayError::Authority)?;
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(value);
    Ok(())
}
