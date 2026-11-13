use super::backend::ApiFailure;
use super::production_components::AccountBalance;
use layerx_types::rung::Rung;

impl AccountBalance {
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn material_rung(bytes: &[u8]) -> Result<Option<Rung>, ApiFailure> {
        let invalid = ApiFailure::upstream_degraded;
        let (rung, offset) = match bytes.get(..5) {
            Some(b"LXHB2") => match bytes.get(5) {
                Some(0) => (None, 6),
                Some(1) => (
                    Some(super::finalization::decode_rung(
                        *bytes.get(6).ok_or_else(invalid)?,
                    )?),
                    7,
                ),
                _ => return Err(invalid()),
            },
            // V1 was emitted by the Rung producer without a presence marker.
            // The missing-rung form starts directly with a bounded timestamp length.
            Some(b"LXHB1") if bytes.get(5..9) == Some(&20u32.to_be_bytes()) => (None, 5),
            Some(b"LXHB1") => (
                Some(super::finalization::decode_rung(
                    *bytes.get(5).ok_or_else(invalid)?,
                )?),
                6,
            ),
            _ => return Err(invalid()),
        };
        let word = |start: usize| -> Result<usize, ApiFailure> {
            let bytes = bytes.get(start..start + 4).ok_or_else(invalid)?;
            Ok(u32::from_be_bytes(bytes.try_into().map_err(|_| invalid())?) as usize)
        };
        let stamp_len = word(offset)?;
        if stamp_len != 20 {
            return Err(invalid());
        }
        let stamp_end = offset + 4 + stamp_len;
        let stamp = std::str::from_utf8(bytes.get(offset + 4..stamp_end).ok_or_else(invalid)?)
            .map_err(|_| invalid())?;
        if crate::time::seconds_from_rfc3339(stamp).is_none() {
            return Err(invalid());
        }
        let canonical_len = word(stamp_end + 8)?;
        let canonical_start = stamp_end + 12;
        let canonical_end = canonical_start
            .checked_add(canonical_len)
            .ok_or_else(invalid)?;
        if canonical_len == 0
            || bytes.get(canonical_start..canonical_end).is_none()
            || canonical_end >= bytes.len()
        {
            return Err(invalid());
        }
        Ok(rung)
    }
}
