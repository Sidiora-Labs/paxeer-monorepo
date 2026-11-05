use layerx_agent_api::identity::Asset;
use serde::{Deserialize, Serialize};

use crate::notify::{ApprovalId, Delivery, NotificationClass, NotifyError};
use crate::store::{PrincipalScope, RowKey, Table};

use super::{InboxError, InboxItem};

const ROW_PREFIX: &str = "approval_notification_";

/// Exact held value carried by an approval notification's stable subject.
/// Asset identifiers are independent of authenticated display currency metadata.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ApprovalNotification {
    approval_id: ApprovalId,
    amount: u128,
    asset: Asset,
    disclosure_digest: [u8; 32],
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StoredNotification {
    version: u8,
    approval_id: String,
    amount: String,
    asset: String,
    disclosure_digest: [u8; 32],
}

impl ApprovalNotification {
    #[must_use]
    pub const fn approval_id(&self) -> &ApprovalId {
        &self.approval_id
    }

    #[must_use]
    pub const fn amount(&self) -> u128 {
        self.amount
    }

    #[must_use]
    pub fn asset(&self) -> &str {
        self.asset.as_str()
    }

    #[must_use]
    pub const fn disclosure_digest(&self) -> [u8; 32] {
        self.disclosure_digest
    }

    /// Loads the exact value bound to a stored notification's approval route.
    ///
    /// # Errors
    /// Refuses another notification class, invalid routes, missing data and
    /// corrupt or substituted approval subjects.
    pub fn for_delivery(
        scope: &PrincipalScope<'_>,
        delivery: &Delivery,
    ) -> Result<Self, NotifyError> {
        if delivery.class() != NotificationClass::ApprovalWaiting {
            return Err(NotifyError::LinkSubjectMismatch);
        }
        let id = ApprovalId::new(
            delivery
                .deep_link()
                .strip_prefix("/app/approvals/")
                .ok_or(NotifyError::InvalidDeepLink)?,
        )?;
        Self::load(scope, &id)
    }

    pub(super) fn persist(
        scope: &mut PrincipalScope<'_>,
        now: u64,
        item: &InboxItem,
    ) -> Result<(), InboxError> {
        let value = Self {
            approval_id: item.approval_id().clone(),
            amount: item.amount(),
            asset: Asset::new(item.asset()).map_err(|_| InboxError::UnsupportedDisclosure)?,
            disclosure_digest: item.disclosure_digest(),
        };
        let key = Self::key(value.approval_id())?;
        if scope.get(Table::Notifications, &key).is_some() {
            if Self::load(scope, value.approval_id())? != value {
                return Err(InboxError::DisclosureChanged);
            }
            return Ok(());
        }
        let stored = StoredNotification {
            version: 1,
            approval_id: value.approval_id.as_str().to_owned(),
            amount: value.amount.to_string(),
            asset: value.asset.as_str().to_owned(),
            disclosure_digest: value.disclosure_digest,
        };
        let bytes = serde_json::to_vec(&stored)
            .map_err(|_| NotifyError::Corrupt("approval notification encoding"))?;
        scope
            .put(Table::Notifications, key, now, bytes)
            .map_err(NotifyError::from)?;
        Ok(())
    }

    fn key(id: &ApprovalId) -> Result<RowKey, NotifyError> {
        RowKey::new(format!("{ROW_PREFIX}{}", id.as_str())).map_err(NotifyError::from)
    }

    fn load(scope: &PrincipalScope<'_>, id: &ApprovalId) -> Result<Self, NotifyError> {
        let key = Self::key(id)?;
        let row = scope
            .get(Table::Notifications, &key)
            .ok_or(NotifyError::NotificationNotFound)?;
        let stored: StoredNotification = serde_json::from_slice(row.bytes())
            .map_err(|_| NotifyError::Corrupt("approval notification record"))?;
        let amount: u128 = stored
            .amount
            .parse()
            .map_err(|_| NotifyError::Corrupt("approval notification amount"))?;
        if stored.version != 1
            || stored.approval_id != id.as_str()
            || stored.amount != amount.to_string()
        {
            return Err(NotifyError::Corrupt("approval notification binding"));
        }
        Ok(Self {
            approval_id: id.clone(),
            amount,
            asset: Asset::new(stored.asset)
                .map_err(|_| NotifyError::Corrupt("approval notification asset"))?,
            disclosure_digest: stored.disclosure_digest,
        })
    }
}
