use layerx_programs_ai_market::{
    codec::{Reader, Writer},
    errors::CAPACITY,
};

#[test]
fn section_copy_preserves_unaligned_words_tails_and_bounds() {
    for len in 0..=1025 {
        let source: Vec<u8> = (0_usize..len)
            .map(|i| (i * 73 + 19).to_le_bytes()[0])
            .collect();
        let mut target = vec![0xa5; len + 2];
        let mut writer = Writer::new(&mut target[1..=len]);
        assert_eq!(writer.put(&source), Ok(()));
        assert_eq!(writer.len(), len);
        assert_eq!(writer.put(&[7]), Err(CAPACITY));
        assert_eq!(writer.len(), len);
        assert_eq!(target[0], 0xa5);
        assert_eq!(target[len + 1], 0xa5);
        assert_eq!(&target[1..=len], source.as_slice());
        assert_eq!(
            Reader::new(&target[1..=len]).take(len),
            Ok(source.as_slice())
        );
    }
}

#[test]
fn section_copy_preserves_large_sections_at_every_word_alignment() {
    use layerx_programs_ai_market::{rewards::REWARD_STATE_BYTES, MAX_STATE_BYTES};

    for len in [
        0,
        1,
        7,
        8,
        9,
        31,
        32,
        33,
        REWARD_STATE_BYTES,
        MAX_STATE_BYTES,
    ] {
        for source_offset in 0..8 {
            let input: Vec<u8> = (0..len + source_offset)
                .map(|index| (index * 73 + 19).to_le_bytes()[0])
                .collect();
            let source = &input[source_offset..];
            for destination_offset in 0..8 {
                let mut output = vec![0xa5; destination_offset + len + 1];
                let mut writer = Writer::new(&mut output[..destination_offset + len]);
                assert_eq!(writer.put(&vec![0xa5; destination_offset]), Ok(()));
                assert_eq!(writer.put(source), Ok(()));
                assert_eq!(writer.len(), destination_offset + len);
                assert_eq!(writer.put(&[7]), Err(CAPACITY));
                assert_eq!(writer.len(), destination_offset + len);
                assert!(output[..destination_offset]
                    .iter()
                    .all(|byte| *byte == 0xa5));
                assert_eq!(
                    &output[destination_offset..destination_offset + len],
                    source
                );
                assert_eq!(output[destination_offset + len], 0xa5);
            }
        }
    }
}

#[test]
fn reader_preserves_offsets_and_refuses_without_consuming() {
    use layerx_programs_ai_market::errors::{ARITHMETIC, NON_CANONICAL};
    let input = [1, 2, 3, 4, 5, 6, 7];
    let mut reader = Reader::new(&input);
    assert_eq!(reader.u16(), Ok(0x0102));
    assert_eq!(reader.offset(), 2);
    assert_eq!(reader.remaining(), 5);
    assert_eq!(reader.take(usize::MAX), Err(ARITHMETIC));
    assert_eq!(reader.take(6), Err(NON_CANONICAL));
    assert_eq!(reader.offset(), 2);
    assert_eq!(reader.remaining(), 5);
    assert_eq!(reader.finish(), Err(NON_CANONICAL));
    assert_eq!(reader.u32(), Ok(0x0304_0506));
    assert_eq!(reader.u8(), Ok(7));
    assert_eq!(reader.take(0), Ok(&[][..]));
    assert_eq!(reader.offset(), input.len());
    assert_eq!(reader.remaining(), 0);
    assert_eq!(reader.finish(), Ok(()));
}
