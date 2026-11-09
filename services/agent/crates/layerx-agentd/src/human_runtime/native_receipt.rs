use super::HumanOperationError;
use layerx_client::rpc::receipt::ReceiptError;
use layerx_types::payload::ModuleRegistry;
use layerx_wire::historical::activity::{decode_signed, encode_signed, Activity};
use layerx_wire::historical::hash::activity_id;

pub(super) struct RetainedNativeOwner {
    canonical: Vec<u8>,
    activity: Activity,
    owner: [u8; 32],
}

impl RetainedNativeOwner {
    pub(super) fn decode(
        canonical: Vec<u8>,
        registry: &ModuleRegistry,
        network_id: u32,
        protocol_version: u16,
        action: [u8; 32],
        identifier: [u8; 32],
    ) -> Result<Option<Self>, HumanOperationError> {
        let activity =
            decode_signed(&canonical, registry).map_err(|_| HumanOperationError::Refused)?;
        if encode_signed(&activity).map_err(|_| HumanOperationError::Refused)? != canonical
            || activity_id(&activity).map_err(|_| HumanOperationError::Refused)? != identifier
            || activity.idempotency_key() != action
            || activity.network_id() != network_id
            || activity.protocol_version() != protocol_version
        {
            return Err(HumanOperationError::Refused);
        }
        if activity.protocol_version() != 3
            || !(2..=7).contains(&(activity.activity_type().module() as u16))
        {
            return Ok(None);
        }
        let owner = activity
            .authority()
            .try_into()
            .map_err(|_| HumanOperationError::Refused)?;
        Ok(Some(Self {
            canonical,
            activity,
            owner,
        }))
    }
}

pub(super) fn map_lookup_error(error: ReceiptError) -> HumanOperationError {
    if error.is_unavailable() {
        HumanOperationError::Unavailable
    } else {
        HumanOperationError::Refused
    }
}

pub(super) fn persist(
    shared: &std::sync::Mutex<crate::store::Store>,
    tenant: crate::store::TenantId,
    action: [u8; 32],
    receipt: &layerx_proof::receipt::VerifiedReceipt,
    batch: &layerx_proof::receipt::AuthorizedBatch,
) -> Result<crate::receipt::ServedReceipt, HumanOperationError> {
    let mut store = shared
        .lock()
        .map_err(|_| HumanOperationError::Unavailable)?;
    let ingress = crate::receipt::store_verified_if_absent(
        &mut store,
        tenant,
        action,
        receipt.canonical_bytes(),
        batch,
    );
    ingress.map_err(|error| match error {
        crate::receipt::ReceiptStoreError::Missing
        | crate::receipt::ReceiptStoreError::Store(_) => HumanOperationError::Unavailable,
        _ => HumanOperationError::Refused,
    })
}
