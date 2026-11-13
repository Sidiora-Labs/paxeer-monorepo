const DIGITS: &[u8; 16] = b"0123456789abcdef";

pub(crate) fn push_lower(output: &mut String, bytes: &[u8]) {
    output.reserve(bytes.len().saturating_mul(2));
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
}

#[must_use]
pub fn lower(bytes: &[u8]) -> String {
    let mut output = String::new();
    push_lower(&mut output, bytes);
    output
}
