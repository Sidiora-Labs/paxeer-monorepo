use layerx_programs::chain_proof::{
    proto::{ProtoField, ProtoReader, ProtoValue},
    ChainProofError,
};

#[test]
fn chain_proto_reader_decodes_canonical_fields() -> Result<(), ChainProofError> {
    let header = [
        0x0a, 0x02, 0x08, 0x0b, 0x12, 0x05, b'c', b'h', b'a', b'i', b'n', 0x18, 0xac, 0x02, 0x22,
        0x02, 0x08, 0x01, 0x2a, 0x02, 0x12, 0x00, 0x5a, 0x03, b'a', b'p', b'p',
    ];
    let mut reader = ProtoReader::new(&header);
    assert_eq!(reader.cursor(), 0);
    for (number, value, cursor) in [
        (1, ProtoValue::Bytes(&header[2..4]), 4),
        (2, ProtoValue::Bytes(b"chain"), 11),
        (3, ProtoValue::Varint(300), 14),
        (4, ProtoValue::Bytes(&[0x08, 0x01]), 18),
        (5, ProtoValue::Bytes(&[0x12, 0x00]), 22),
        (11, ProtoValue::Bytes(b"app"), 27),
    ] {
        assert_eq!(reader.next_field()?, Some(ProtoField { number, value }));
        assert_eq!(reader.cursor(), cursor);
    }
    assert_eq!(reader.next_field()?, None);
    assert_eq!(reader.cursor(), header.len());
    reader.finish()?;

    let mut version = ProtoReader::new(&header[2..4]);
    assert_eq!(
        version.next_field()?,
        Some(ProtoField {
            number: 1,
            value: ProtoValue::Varint(11)
        })
    );
    assert_eq!(version.cursor(), 2);
    version.finish()?;

    let vote = [
        0x08, 0x02, 0x11, 0x2c, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x19, 0x01, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x2a, 0x02, 0x08, 0x01, 0x32, 0x05, b'c', b'h', b'a', b'i',
        b'n',
    ];
    let mut reader = ProtoReader::new(&vote);
    for (number, value, cursor) in [
        (1, ProtoValue::Varint(2), 2),
        (2, ProtoValue::Fixed64(300), 11),
        (3, ProtoValue::Fixed64(1), 20),
        (5, ProtoValue::Bytes(&[0x08, 0x01]), 24),
        (6, ProtoValue::Bytes(b"chain"), 31),
    ] {
        assert_eq!(reader.next_field()?, Some(ProtoField { number, value }));
        assert_eq!(reader.cursor(), cursor);
    }
    assert_eq!(reader.next_field()?, None);
    reader.finish()?;

    let mut reader = ProtoReader::new(&[
        0x08, 0x00, 0x08, 0x7f, 0x08, 0x80, 0x01, 0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
        0xff, 0xff, 0x01, 0x82, 0x01, 0x00,
    ]);
    for (value, cursor) in [(0, 2), (127, 4), (128, 7), (u64::MAX, 18)] {
        assert_eq!(
            reader.next_field()?,
            Some(ProtoField {
                number: 1,
                value: ProtoValue::Varint(value)
            })
        );
        assert_eq!(reader.cursor(), cursor);
    }
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 16,
            value: ProtoValue::Bytes(&[])
        })
    );
    assert_eq!(reader.cursor(), 21);
    reader.finish()?;
    let mut empty = ProtoReader::new(&[]);
    assert_eq!(empty.next_field()?, None);
    empty.finish()?;
    Ok(())
}

#[test]
fn chain_proto_reader_refuses_noncanonical_varint_and_order() -> Result<(), ChainProofError> {
    for bytes in [
        &[0x08, 0x80, 0x00][..],
        &[0x08, 0x81, 0x00],
        &[0x88, 0x00, 0x01],
        &[0x0a, 0x80, 0x00],
        &[
            0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02,
        ],
        &[
            0x08, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00,
        ],
        &[0x00],
        &[0x80, 0x80, 0x80, 0x80, 0x10, 0x00],
    ] {
        let mut reader = ProtoReader::new(bytes);
        assert_eq!(
            reader.next_field(),
            Err(ChainProofError::NonCanonical),
            "{bytes:x?}"
        );
        assert_eq!(reader.cursor(), 0);
        assert_eq!(reader.finish(), Err(ChainProofError::NonCanonical));
    }

    let mut reader = ProtoReader::new(&[0x10, 0x01, 0x08, 0x02]);
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 2,
            value: ProtoValue::Varint(1)
        })
    );
    assert_eq!(reader.cursor(), 2);
    assert_eq!(reader.next_field(), Err(ChainProofError::NonCanonical));
    assert_eq!(reader.cursor(), 2);

    for suffix in [&[0x00][..], &[0x80], &[0x10, 0x02]] {
        let mut bytes = vec![0x08, 0x01];
        bytes.extend_from_slice(suffix);
        let mut reader = ProtoReader::new(&bytes);
        assert_eq!(
            reader.next_field()?,
            Some(ProtoField {
                number: 1,
                value: ProtoValue::Varint(1)
            })
        );
        assert_eq!(reader.cursor(), 2);
        assert_eq!(reader.finish(), Err(ChainProofError::NonCanonical));
        assert_eq!(reader.cursor(), 2);
    }

    for bytes in [
        &[0x80][..],
        &[0x08],
        &[0x08, 0x80],
        &[0x09, 0, 0, 0, 0, 0, 0, 0],
        &[0x0a, 0x80],
        &[0x0a, 0x02, 0x01],
        &[
            0x0a, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01,
        ],
    ] {
        let mut reader = ProtoReader::new(bytes);
        assert_eq!(
            reader.next_field(),
            Err(ChainProofError::Truncated),
            "{bytes:x?}"
        );
        assert_eq!(reader.cursor(), 0);
    }
    for wire_type in [3, 4, 5, 6, 7] {
        let bytes = [0x08 | wire_type];
        let mut reader = ProtoReader::new(&bytes);
        assert_eq!(reader.next_field(), Err(ChainProofError::UnknownField));
        assert_eq!(reader.cursor(), 0);
    }
    Ok(())
}
