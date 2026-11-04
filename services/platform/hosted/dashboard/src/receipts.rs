use layerx_platform_webhooks::error::WebhookError;
use layerx_platform_webhooks::events::{EventKind, Principal, ProtocolEvent, Verification};

use crate::gateway::{principal_digest, text, Resp, Store};
use crate::{DashboardError, PaymentView, ReceiptView};

pub struct ReceiptReader {
    store: Store,
}

impl ReceiptReader {
    #[must_use]
    pub fn new(store: Store) -> Self {
        Self { store }
    }

    /// # Errors
    /// Rejects missing or invalid webhook Redis configuration or credentials.
    pub fn from_environment() -> Result<Self, String> {
        Ok(Self::new(Store::from_webhooks_environment()?))
    }

    /// # Errors
    /// Rejects invalid activity identifiers, unavailable or corrupt indexes and
    /// entries without matching principal, activity and settlement evidence.
    pub fn receipt(
        &self,
        principal: &Principal,
        activity_id: &str,
    ) -> Result<ReceiptView, DashboardError> {
        if activity_id.len() != 64 || !activity_id.bytes().all(|byte| byte.is_ascii_hexdigit()) {
            return Err(DashboardError::InvalidRequest);
        }
        let activity_id = activity_id.to_ascii_lowercase();
        let key = format!(
            "webhooks:principal:{}:activities",
            principal_digest(principal)
        );
        let version = self
            .store
            .command(&["GET", &format!("{key}:version")])
            .map_err(|_| WebhookError::Unavailable)?;
        if text(&version).as_deref() != Some("1") {
            return Err(WebhookError::Unavailable.into());
        }
        let response = self
            .store
            .command(&["HGET", &key, &activity_id])
            .map_err(|_| WebhookError::Unavailable)?;
        let bytes = match response {
            Resp::Bulk(None) => return Err(DashboardError::UnknownReceipt),
            Resp::Bulk(Some(bytes)) => bytes,
            _ => return Err(WebhookError::CorruptStore.into()),
        };
        let event: ProtocolEvent =
            serde_json::from_slice(&bytes).map_err(|_| WebhookError::CorruptStore)?;
        event.validate().map_err(|_| WebhookError::CorruptStore)?;
        if event.principal() != principal {
            return Err(WebhookError::CorruptStore.into());
        }
        if event.kind() != EventKind::Payment
            || !event.facts().iter().any(|fact| {
                fact.name() == "activity_id"
                    && fact.value() == activity_id.as_str()
                    && fact.verification().at_least(Verification::ReceiptVerified)
                    && fact.receipt_digest().is_some()
            })
        {
            return Err(DashboardError::UnknownReceipt);
        }
        let payment = PaymentView::of(&event);
        let receipt_digest = payment
            .receipt_digest
            .clone()
            .ok_or(DashboardError::UnknownReceipt)?;
        let activity = event
            .facts()
            .iter()
            .find(|fact| fact.name() == "activity_id" && fact.value() == activity_id.as_str())
            .filter(|fact| fact.receipt_digest() == Some(receipt_digest.as_str()))
            .ok_or(DashboardError::UnknownReceipt)?;
        if !payment.settled {
            return Err(DashboardError::UnknownReceipt);
        }
        let verification = if activity
            .verification()
            .at_least(payment.settlement_verification)
        {
            payment.settlement_verification
        } else {
            activity.verification()
        };
        Ok(ReceiptView {
            activity_id,
            event: event.id().as_str().to_owned(),
            receipt_digest,
            verification,
            settled: true,
        })
    }
}
