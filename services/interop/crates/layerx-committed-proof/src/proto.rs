//! Protobuf wire encoders for the messages used in header and vote hashes.
//!
//! Field encoders always emit the field, even for zero or empty values. Callers
//! implement their message's default-field omission rules and field ordering.

/// Append the minimal unsigned protobuf varint representation of `value`.
pub fn encode_varint(mut value: u64, output: &mut Vec<u8>) {
    while value >= 0x80 {
        output.push((value as u8 & 0x7f) | 0x80);
        value >>= 7;
    }
    output.push(value as u8);
}

/// Append a field key with wire type 0 followed by an unsigned varint.
///
/// # Panics
/// Panics if `field_number` is zero or exceeds protobuf's 29-bit field limit.
pub fn encode_varint_field(field_number: u32, value: u64, output: &mut Vec<u8>) {
    encode_key(field_number, 0, output);
    encode_varint(value, output);
}

/// Append a field key with wire type 2, its byte length, and its payload.
/// Strings and embedded messages use this same encoding.
///
/// # Panics
/// Panics if `field_number` is zero or exceeds protobuf's 29-bit field limit.
pub fn encode_length_delimited(field_number: u32, bytes: &[u8], output: &mut Vec<u8>) {
    encode_key(field_number, 2, output);
    encode_varint(bytes.len() as u64, output);
    output.extend_from_slice(bytes);
}

fn encode_key(field_number: u32, wire_type: u64, output: &mut Vec<u8>) {
    assert!((1..(1 << 29)).contains(&field_number));
    encode_varint((u64::from(field_number) << 3) | wire_type, output);
}
