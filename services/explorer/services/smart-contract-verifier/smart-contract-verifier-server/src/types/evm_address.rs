use blockscout_display_bytes::Bytes as DisplayBytes;
use std::str::FromStr;

/// An EVM address is exactly 20 bytes; hex syntax alone is insufficient.
pub(super) struct EvmAddress([u8; 20]);

impl EvmAddress {
    pub(super) fn parse(value: &str) -> Result<Self, tonic::Status> {
        let invalid_address =
            || tonic::Status::invalid_argument("address must be a hex-encoded 20-byte EVM address");
        let decoded = DisplayBytes::from_str(value).map_err(|_| invalid_address())?;
        let bytes = decoded
            .0
            .as_ref()
            .try_into()
            .map_err(|_| invalid_address())?;
        Ok(Self(bytes))
    }

    pub(super) fn into_bytes(self) -> bytes::Bytes {
        bytes::Bytes::copy_from_slice(&self.0)
    }
}
