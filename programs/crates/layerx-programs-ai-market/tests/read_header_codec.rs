use layerx_programs_ai_market::{
    codec::{decode_read_header, encode_read_header, ReadHeader},
    errors::{CodecResult, NON_CANONICAL},
    types::{ChainDomain, MarketId, Presence, ProgramId, RosterDigest, StateDigest, Version},
    SCHEMA_VERSION,
};

#[test]
fn read_header_preserves_ordered_fields_and_rejects_every_truncated_prefix() -> CodecResult<()> {
    let mut header = ReadHeader {
        revision: 0x0102_0304_0506_0708,
        digest: StateDigest::new([0x11; 32])?,
        total_bytes: 0x0001_0203,
        chain: ChainDomain::new([0x22; 32])?,
        program: ProgramId::new([0x33; 32])?,
        market: MarketId::new([0x44; 32])?,
        epoch: Presence::Absent,
        config: Version::new(0x1112_1314_1516_1718)?,
        roster: Presence::Absent,
    };
    for present in [false, true] {
        if present {
            header.epoch = Presence::Present(0x2122_2324_2526_2728);
            header.roster = Presence::Present(RosterDigest::new([0x55; 32])?);
        }
        let mut expected = SCHEMA_VERSION.to_be_bytes().to_vec();
        expected.extend_from_slice(&header.revision.to_be_bytes());
        expected.extend_from_slice(&[0x11; 32]);
        expected.extend_from_slice(&header.total_bytes.to_be_bytes());
        for identity in [0x22, 0x33, 0x44] {
            expected.extend_from_slice(&[identity; 32]);
        }
        expected.push(u8::from(present));
        if present {
            expected.extend_from_slice(&0x2122_2324_2526_2728_u64.to_be_bytes());
        }
        expected.extend_from_slice(&header.config.get().to_be_bytes());
        expected.push(u8::from(present));
        if present {
            expected.extend_from_slice(&[0x55; 32]);
        }

        let mut output = [0; 192];
        let n = encode_read_header(&header, &mut output)?;
        assert_eq!(&output[..n], expected);
        assert_eq!(decode_read_header(&output[..n])?, header);
        for length in 0..n {
            assert_eq!(decode_read_header(&output[..length]), Err(NON_CANONICAL));
        }
    }
    Ok(())
}
