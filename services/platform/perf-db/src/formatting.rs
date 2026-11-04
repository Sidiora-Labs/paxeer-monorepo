use crate::core::types::FixedI128;

/// Exact 18-decimal financial presentation shared by all CLI commands.
pub fn format_fixed(value: FixedI128) -> String {
    let magnitude = value.unsigned_abs();
    let precision = 1_000_000_000_000_000_000u128;
    let whole = magnitude / precision;
    let fraction = magnitude % precision;
    let sign = if value < 0 { "-" } else { "" };
    if fraction == 0 {
        format!("{sign}{whole}")
    } else {
        let digits = format!("{fraction:018}");
        format!("{sign}{whole}.{}", digits.trim_end_matches('0'))
    }
}
