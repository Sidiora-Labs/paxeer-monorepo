use layerx_interop_gateway::trace::TraceId;
use sha2::{Digest as _, Sha256};

use crate::{CheckoutSubmission, UcpError, UcpPaymentIntent, UcpPaymentPlane, UcpPlaneResult};

/// Deployment-owned equivalence of one currency major unit and one asset unit.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct MerchantCurrencyBinding {
    currency: [u8; 3],
    asset: [u8; 32],
    minor_decimals: u8,
    atomic_decimals: u8,
}

impl MerchantCurrencyBinding {
    /// Binds the merchant's ISO currency exponent and asset exponent explicitly.
    ///
    /// # Errors
    /// Refuses malformed currency, zero asset or exponents beyond u128 precision.
    pub fn new(
        currency: [u8; 3],
        asset: [u8; 32],
        minor_decimals: u8,
        atomic_decimals: u8,
    ) -> Result<Self, UcpError> {
        if !currency.iter().all(u8::is_ascii_uppercase)
            || asset == [0; 32]
            || minor_decimals > 38
            || atomic_decimals > 38
        {
            return Err(UcpError::InvalidCurrencyBinding);
        }
        Ok(Self {
            currency,
            asset,
            minor_decimals,
            atomic_decimals,
        })
    }

    fn atomic_amount(self, submission: &CheckoutSubmission) -> Result<u128, UcpError> {
        submission.validate()?;
        if submission.currency != self.currency || submission.layerx_asset != self.asset {
            return Err(UcpError::CurrencyBindingMismatch);
        }
        if self.atomic_decimals >= self.minor_decimals {
            let multiplier = 10_u128.pow(u32::from(self.atomic_decimals - self.minor_decimals));
            submission
                .total_minor
                .checked_mul(multiplier)
                .ok_or(UcpError::AmountOverflow)
        } else {
            let divisor = 10_u128.pow(u32::from(self.minor_decimals - self.atomic_decimals));
            if submission.total_minor % divisor != 0 {
                return Err(UcpError::InexactAmount);
            }
            Ok(submission.total_minor / divisor)
        }
    }
}

/// Merchant service configuration. An absent binding always refuses payment.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct UcpServiceConfig {
    binding: Option<MerchantCurrencyBinding>,
}

impl UcpServiceConfig {
    #[must_use]
    pub const fn new(binding: MerchantCurrencyBinding) -> Self {
        Self {
            binding: Some(binding),
        }
    }

    /// Converts a checkout without rounding or inferring units from buyer data.
    ///
    /// # Errors
    /// Refuses absent or mismatched bindings, overflow and fractional atomic units.
    pub fn atomic_amount(&self, submission: &CheckoutSubmission) -> Result<u128, UcpError> {
        self.binding
            .ok_or(UcpError::CurrencyBindingRequired)?
            .atomic_amount(submission)
    }

    #[must_use]
    pub fn bind_plane<'a, P: UcpPaymentPlane>(
        &self,
        plane: &'a mut P,
    ) -> ConfiguredUcpPaymentPlane<'a, P> {
        ConfiguredUcpPaymentPlane {
            config: *self,
            plane,
        }
    }

    pub(crate) fn digest(&self) -> [u8; 32] {
        let mut hash = Sha256::new();
        hash.update(b"LayerX/interop/ucp/currency-binding/v1\0");
        if let Some(binding) = self.binding {
            hash.update(binding.currency);
            hash.update(binding.asset);
            hash.update([binding.minor_decimals, binding.atomic_decimals]);
        }
        hash.finalize().into()
    }
}

/// Attaches trusted merchant configuration to an existing real payment plane.
pub struct ConfiguredUcpPaymentPlane<'a, P> {
    config: UcpServiceConfig,
    plane: &'a mut P,
}

impl<P: UcpPaymentPlane> UcpPaymentPlane for ConfiguredUcpPaymentPlane<'_, P> {
    fn service_config(&self) -> Result<UcpServiceConfig, UcpError> {
        Ok(self.config)
    }

    fn execute(
        &mut self,
        intent: &UcpPaymentIntent,
        trace: &TraceId,
    ) -> Result<UcpPlaneResult, UcpError> {
        self.plane.execute(intent, trace)
    }
}
