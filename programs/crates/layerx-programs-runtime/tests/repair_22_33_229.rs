//! Repair record for audit 22.30.127, task 22.33.229.
//! Finding 3: `finding_3_multiple_fields_round_trip`,
//! `finding_3_malformed_suffixes_are_rejected`,
//! `finding_3_every_truncated_suffix_is_rejected`, and
//! `finding_3_depth_is_checked_after_a_valid_prefix`.
//! Finding 4: `finding_4_transport_ceiling_dominates_decoded_size` and
//! `production_codec::tests::{decoder_budget_charges_all_top_level_values,`
//! `decoder_budget_is_shared_across_top_level_values,`
//! `decoder_budget_rejects_excess_and_arithmetic_overflow,`
//! `decoder_scalar_accounting_is_bounded_by_encoded_size,`
//! `decoder_evm_accounting_equals_consumed_size}`.
//! Decoder charges only consumed scalar/byte data; container metadata adds no
//! charge or expansion. Thus decoded <= encoded payload <= 1048575 < 16777216
//! for every admitted convention (EVM charges exactly its consumed payload).
//! Baseline verification was not run: the ordered gate instructions require
//! implementation first and prohibit invoking `verify_cmd` by hand.
//! No out-of-path wiring is required by either proposal. Gate evidence names
//! the verified candidate revision; no pre-change failure run is claimed.

use layerx_programs_runtime::abi::{
    Calldata, CodecError, EncodingConvention, DECODED_SIZE_LIMIT, MAX_CALLDATA_BYTES,
    MAX_NESTING_DEPTH,
};
use layerx_programs_runtime::meter;

// Compile the same production source into this integration target so its
// private Decoder tests execute in the declared gate without exposing internals.
#[path = "../src/abi/codec.rs"]
mod production_codec;

#[test]
fn finding_3_multiple_fields_round_trip() -> Result<(), CodecError> {
    let mut calldata = Calldata::new();
    calldata.encode_u8(42)?;
    calldata.encode_u16(256)?;
    calldata.encode_u32(65_536)?;
    calldata.encode_u64(u64::MAX)?;
    calldata.encode_u128(u128::MAX)?;
    calldata.encode_bytes(b"multiple fields")?;
    calldata.begin_fixed_array(2)?;
    calldata.encode_u8(1)?;
    calldata.begin_variable_array(2)?;
    calldata.encode_option_none()?;
    calldata.begin_option_some()?;
    calldata.begin_union(7)?;
    calldata.encode_bytes(b"nested")?;
    calldata.encode_u8(99)?;
    let transport = calldata.as_bytes();
    let parsed = Calldata::from_bytes(&transport)?;
    assert_eq!(parsed.convention(), EncodingConvention::LayerX);
    assert_eq!(parsed.payload(), calldata.payload());
    assert_eq!(parsed.as_bytes(), transport);
    assert!(Calldata::from_bytes(&[])?.payload().is_empty());
    assert!(Calldata::from_bytes(&Calldata::new().as_bytes())?
        .payload()
        .is_empty());
    Ok(())
}

#[test]
fn finding_3_malformed_suffixes_are_rejected() -> Result<(), CodecError> {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut calldata = Calldata::new();
    calldata.encode_u8(1)?;
    let prefix = calldata.as_bytes();
    let mut invalid_tag = prefix.clone();
    invalid_tag.push(0xff);
    assert_eq!(
        match Calldata::from_bytes(&invalid_tag) {
            Err(error) => error,
            Ok(_) => panic!("expected invalid type refusal"),
        },
        CodecError::InvalidType
    );

    calldata.encode_option_none()?;
    let mut invalid_option = calldata.as_bytes();
    *invalid_option.last_mut().assert_success("test value") = 2;
    assert_eq!(
        match Calldata::from_bytes(&invalid_option) {
            Err(error) => error,
            Ok(_) => panic!("expected invalid option refusal"),
        },
        CodecError::InvalidOption
    );
    assert_eq!(Calldata::from_bytes(&prefix)?.as_bytes(), prefix);
    Ok(())
}

#[test]
fn finding_3_every_truncated_suffix_is_rejected() -> Result<(), CodecError> {
    let mut prefix = Calldata::new();
    prefix.encode_u8(1)?;
    let prefix = prefix.as_bytes();

    let mut suffixes = Vec::new();
    let mut integer = Calldata::new();
    integer.encode_u128(u128::MAX)?;
    suffixes.push(integer);
    let mut bytes = Calldata::new();
    bytes.encode_bytes(b"suffix")?;
    suffixes.push(bytes);
    let mut fixed_array = Calldata::new();
    fixed_array.begin_fixed_array(2)?;
    fixed_array.encode_u8(2)?;
    fixed_array.encode_u16(3)?;
    suffixes.push(fixed_array);
    let mut variable_array = Calldata::new();
    variable_array.begin_variable_array(1)?;
    variable_array.encode_bytes(b"child")?;
    suffixes.push(variable_array);
    let mut option = Calldata::new();
    option.begin_option_some()?;
    option.encode_u8(4)?;
    suffixes.push(option);
    let mut union = Calldata::new();
    union.begin_union(5)?;
    union.encode_u64(6)?;
    suffixes.push(union);

    for suffix in suffixes {
        let mut transport = prefix.clone();
        transport.extend_from_slice(suffix.payload());
        assert_eq!(Calldata::from_bytes(&transport)?.as_bytes(), transport);
        for end in prefix.len() + 1..transport.len() {
            assert_eq!(
                match Calldata::from_bytes(&transport[..end]) {
                    Err(error) => error,
                    Ok(_) => panic!("expected truncated suffix refusal"),
                },
                CodecError::Truncated,
                "suffix {:?}, truncation at {end}",
                suffix.payload()
            );
        }
    }
    Ok(())
}

#[test]
fn finding_3_depth_is_checked_after_a_valid_prefix() -> Result<(), CodecError> {
    for container in 0..4 {
        for depth in [MAX_NESTING_DEPTH, MAX_NESTING_DEPTH + 1] {
            let mut calldata = Calldata::new();
            calldata.encode_u8(1)?;
            for _ in 0..depth {
                match container {
                    0 => calldata.begin_fixed_array(1)?,
                    1 => calldata.begin_variable_array(1)?,
                    2 => calldata.begin_option_some()?,
                    _ => calldata.begin_union(7)?,
                }
            }
            calldata.encode_u8(2)?;
            let transport = calldata.as_bytes();
            if depth == MAX_NESTING_DEPTH {
                assert_eq!(Calldata::from_bytes(&transport)?.as_bytes(), transport);
            } else {
                assert_eq!(
                    match Calldata::from_bytes(&transport) {
                        Err(error) => error,
                        Ok(_) => panic!("expected nesting depth refusal"),
                    },
                    CodecError::NestingTooDeep
                );
            }
        }
    }
    Ok(())
}

#[test]
fn finding_4_transport_ceiling_dominates_decoded_size() -> Result<(), CodecError> {
    assert_eq!(MAX_CALLDATA_BYTES, 1_048_576);
    assert_eq!(DECODED_SIZE_LIMIT, 16_777_216);
    let mut calldata = Calldata::new();
    calldata.encode_u64(u64::MAX)?;
    let remaining = MAX_CALLDATA_BYTES - calldata.as_bytes().len() - 5;
    calldata.encode_bytes(&vec![0xa5; remaining])?;
    let transport = calldata.as_bytes();
    assert_eq!(transport.len(), MAX_CALLDATA_BYTES);
    assert!(transport.len() - 1 < DECODED_SIZE_LIMIT);
    assert!(8 + remaining < transport.len());
    assert_eq!(Calldata::from_bytes(&transport)?.as_bytes(), transport);

    let mut oversized = Calldata::new();
    assert_eq!(
        oversized.encode_bytes(&vec![0; DECODED_SIZE_LIMIT + 1]),
        Err(CodecError::InputTooLarge)
    );
    assert_eq!(oversized.as_bytes(), Calldata::new().as_bytes());
    Ok(())
}
