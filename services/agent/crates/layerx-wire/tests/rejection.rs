use std::collections::BTreeSet;

use layerx_types::result::{KnownResult, ResultCode};
use layerx_types::vectors::CodecVector;
use layerx_wire::decode::Decoder;
use layerx_wire::limits::enforce;
use layerx_wire::{check_ordered_keys, WireError};

fn run_vector(vector: &CodecVector) -> Result<(), WireError> {
    match vector.kind.as_str() {
        "u64" => {
            let mut decoder = Decoder::new(&vector.bytes, 0);
            let _ = decoder.u64()?;
            decoder.finish()
        }
        "tag" => {
            let mut decoder = Decoder::new(&vector.bytes, 0);
            let _ = decoder.tag(3)?;
            Ok(())
        }
        "bytes4" => {
            let mut decoder = Decoder::new(&vector.bytes, 0);
            let _ = decoder.bytes(4)?;
            Ok(())
        }
        "seq" => {
            let mut decoder = Decoder::new(&vector.bytes, 0);
            let first_length = usize::from(decoder.u8()?);
            let first = decoder.fixed(first_length)?;
            let second_length = usize::from(decoder.u8()?);
            let second = decoder.fixed(second_length)?;
            check_ordered_keys(&[first, second])
        }
        _ => Err(WireError {
            result: KnownResult::InvalidTag.into(),
            offset: 0,
        }),
    }
}

/// Executes the published corpus and returns the exact rejection-code coverage.
///
/// # Panics
///
/// Panics when a repository vector is unreadable, accepted, or classified
/// differently from the core-declared result.
#[must_use]
pub fn rejection_corpus() -> BTreeSet<i32> {
    native_submission::codec_vectors(include_str!(
        "../../../../../tests/vectors/codec/adversarial.lxv"
    ))
    .iter()
    .map(|vector| {
        let Err(error) = run_vector(vector) else {
            panic!("adversarial vector accepted: {}", vector.name);
        };
        assert_eq!(
            error.result, vector.expected_result,
            "vector {}",
            vector.name
        );
        error.result.raw()
    })
    .collect()
}

#[test]
fn published_rejection_corpus_matches_core_classes() {
    assert_eq!(rejection_corpus(), BTreeSet::from([-6, -5, -4, -2, -1]));
}

#[test]
fn generated_noncanonical_mutations_are_rejected() {
    let vectors = native_submission::codec_vectors(include_str!(
        "../../../../../tests/vectors/codec/valid.lxv"
    ));
    let canonical = &vectors[0].bytes;
    let mut appended = canonical.clone();
    appended.push(0);
    let mut decoder = Decoder::new(&appended, 0);
    assert!(decoder.u64().is_ok());
    assert_eq!(
        decoder.finish().map_err(|error| error.result),
        Err(ResultCode::from(KnownResult::TrailingBytes))
    );
    for length in 0..canonical.len() {
        let mut decoder = Decoder::new(&canonical[..length], 0);
        assert_eq!(
            decoder.u64().map_err(|error| error.result),
            Err(ResultCode::from(KnownResult::Truncated))
        );
    }
    assert_eq!(
        check_ordered_keys(&[b"b", b"a"]).map_err(|error| error.result),
        Err(ResultCode::from(KnownResult::UnsortedSequence))
    );
    assert_eq!(
        check_ordered_keys(&[b"a", b"a"]).map_err(|error| error.result),
        Err(ResultCode::from(KnownResult::UnsortedSequence))
    );
    let mut indefinite = Decoder::new(&[u8::MAX; 4], 0);
    assert_eq!(
        indefinite.bytes(1024).map_err(|error| error.result),
        Err(ResultCode::from(KnownResult::LengthLimit))
    );

    native_submission::wire_rejection_tests_native_mutations_and_disclosures();
}

#[test]
fn every_declared_bound_accepts_exactly_maximum_and_rejects_one_more() {
    for maximum in [32_usize, 64, 128, 255, 1024, 524_288, 1_048_576] {
        assert!(enforce(maximum, maximum, 7).is_ok());
        assert_eq!(
            enforce(maximum + 1, maximum, 7),
            Err(WireError {
                result: KnownResult::LengthLimit.into(),
                offset: 7,
            })
        );
    }
    for maximum in [4_usize, 32, 64, 256, 512, 65_535] {
        let prefix = u32::try_from(maximum).unwrap_or_default().to_be_bytes();
        let input: Vec<_> = prefix
            .into_iter()
            .chain(std::iter::repeat_n(0, maximum))
            .collect();
        let mut decoder = Decoder::new(&input, maximum);
        assert_eq!(
            decoder.bytes_owned(maximum).map(|bytes| bytes.len()),
            Ok(maximum)
        );

        let over = u32::try_from(maximum + 1).unwrap_or_default().to_be_bytes();
        let mut decoder = Decoder::new(&over, maximum);
        assert_eq!(
            decoder.bytes_owned(maximum).map_err(|error| error.result),
            Err(ResultCode::from(KnownResult::LengthLimit))
        );
        assert_eq!(decoder.allocated(), 0);
    }
}

#[test]
fn hostile_lengths_are_constant_work_and_allocation_bounded() {
    let cases = 100_000;
    let mut completed = 0;
    for _ in 0..cases {
        let mut decoder = Decoder::new(&[u8::MAX; 4], 16);
        assert!(decoder.bytes_owned(16).is_err());
        assert_eq!(decoder.allocated(), 0);
        assert_eq!(decoder.offset(), 4);
        completed += 1;
    }
    assert_eq!(completed, cases);
}

#[test]
fn rejection_taxonomy_coverage_has_no_silent_gap() {
    let mut covered = rejection_corpus();
    let mut invalid_utf8 = Decoder::new(&[0, 0, 0, 1, 0xff], 0);
    let Err(error) = invalid_utf8.text(1) else {
        panic!("invalid UTF-8 was accepted");
    };
    assert_eq!(error.result, KnownResult::NonCanonical.into());
    covered.insert(error.result.raw());
    let mut unknown_field = Decoder::new(&[0, 13], 0);
    let Err(error) = unknown_field.field_id(12) else {
        panic!("unknown field was accepted");
    };
    assert_eq!(error.result, KnownResult::UnknownField.into());
    covered.insert(error.result.raw());
    let mut unknown_version = Decoder::new(&[0, 4, 0x17, 0x01], 0);
    let Err(error) = unknown_version.structure_header_version(0x1701) else {
        panic!("unknown version was accepted");
    };
    assert_eq!(error.result, KnownResult::VersionUnsupported.into());
    covered.insert(error.result.raw());
    let required = BTreeSet::from([
        KnownResult::Truncated.raw(),
        KnownResult::TrailingBytes.raw(),
        KnownResult::NonCanonical.raw(),
        KnownResult::UnsortedSequence.raw(),
        KnownResult::LengthLimit.raw(),
        KnownResult::InvalidTag.raw(),
        KnownResult::UnknownField.raw(),
        KnownResult::VersionUnsupported.raw(),
    ]);
    assert_eq!(covered, required);
}

#[path = "repair_22_33_1523.rs"]
mod native_submission;
