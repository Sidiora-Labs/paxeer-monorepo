use layerx_agentd::human::frame::{
    read_frame, write_frame, FrameViolation, TransportError, LENGTH_PREFIX_BYTES,
};
use std::fmt::Debug;
use std::io::Cursor;

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

#[test]
fn human_frame_round_trips_with_a_length_prefix() {
    const MAXIMUM: usize = 1_048_576;
    assert_eq!(LENGTH_PREFIX_BYTES, 4);
    for payload in [Vec::new(), b"LXHAGT01\x07".to_vec(), vec![0xa5; MAXIMUM]] {
        let mut wire = Vec::new();
        checked(write_frame(&mut wire, &payload, MAXIMUM));
        assert_eq!(
            &wire[..4],
            &checked(u32::try_from(payload.len())).to_be_bytes()
        );
        assert_eq!(&wire[4..], payload);
        assert_eq!(checked(read_frame(&mut wire.as_slice(), MAXIMUM)), payload);
    }

    let oversized = vec![0; MAXIMUM + 1];
    let error = TransportError::Frame(FrameViolation::Oversized {
        declared: checked(u32::try_from(MAXIMUM + 1)),
        maximum: MAXIMUM,
    });
    let mut wire = Vec::new();
    assert_eq!(write_frame(&mut wire, &oversized, MAXIMUM), Err(error));
    assert!(wire.is_empty());
    wire.extend_from_slice(&checked(u32::try_from(MAXIMUM + 1)).to_be_bytes());
    wire.extend_from_slice(&oversized);
    let mut reader = Cursor::new(wire);
    assert_eq!(read_frame(&mut reader, MAXIMUM), Err(error));
    assert_eq!(reader.position(), 4);
    assert_eq!(
        read_frame(&mut u32::MAX.to_be_bytes().as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::Oversized {
            declared: u32::MAX,
            maximum: MAXIMUM,
        }))
    );

    let mut wire = Vec::new();
    checked(write_frame(&mut wire, b"first", MAXIMUM));
    checked(write_frame(&mut wire, b"second", MAXIMUM));
    let mut reader = wire.as_slice();
    assert_eq!(checked(read_frame(&mut reader, MAXIMUM)), b"first");
    assert_eq!(checked(read_frame(&mut reader, MAXIMUM)), b"second");
    assert!(reader.is_empty());
    assert_eq!(
        read_frame(&mut reader, MAXIMUM),
        Err(TransportError::PeerShutdown)
    );
    assert_eq!(
        read_frame(&mut [0, 0, 0].as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::TruncatedPrefix))
    );
    assert_eq!(
        read_frame(&mut [0, 0, 0, 2, 1].as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::TruncatedBody))
    );
}
