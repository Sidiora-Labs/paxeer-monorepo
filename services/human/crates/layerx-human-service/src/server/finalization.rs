use super::backend::ApiFailure;
use layerx_types::rung::Rung;

pub(crate) fn decode_rung(rank: u8) -> Result<Rung, ApiFailure> {
    match rank {
        0 => Ok(Rung::Submitted),
        1 => Ok(Rung::Receipted),
        2 => Ok(Rung::Committed),
        _ => Err(ApiFailure::upstream_degraded()),
    }
}

pub(crate) fn committed(rank: u8) -> bool {
    matches!(decode_rung(rank), Ok(Rung::Committed))
}
