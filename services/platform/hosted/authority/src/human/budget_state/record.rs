#[derive(Debug, Eq, PartialEq)]
pub(super) struct Record {
    pub id: [u8; 32],
    pub owner: [u8; 32],
    pub account: [u8; 32],
    pub asset: [u8; 32],
    pub source: Option<[u8; 32]>,
    pub limit: u128,
    pub spent: u128,
    pub period_start: u64,
    pub expiry: u64,
    pub revocation: u64,
    pub closed: bool,
    pub revoked: bool,
}

pub(super) fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], ()> {
    let start = index.checked_mul(32).ok_or(())?;
    bytes
        .get(start..start.checked_add(32).ok_or(())?)
        .ok_or(())?
        .try_into()
        .map_err(|_| ())
}

pub(super) fn number(bytes: &[u8], index: usize, width: usize) -> Result<u128, ()> {
    let word = word(bytes, index)?;
    if width > 16 || word[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(());
    }
    Ok(u128::from_be_bytes(word[16..].try_into().map_err(|_| ())?))
}

impl Record {
    pub fn from_call(id: [u8; 32], bytes: &[u8]) -> Result<Self, ()> {
        if bytes.len() < 20 * 32 || bytes.len() % 32 != 0 || number(bytes, 0, 8)? != 32 {
            return Err(());
        }
        let tuple = &bytes[32..];
        if number(tuple, 16, 8)? != 18 * 32 {
            return Err(());
        }
        let count = usize::try_from(number(tuple, 18, 8)?).map_err(|_| ())?;
        if count > 16 || bytes.len() != (20 + count) * 32 {
            return Err(());
        }
        for index in 4..=8 {
            number(tuple, index, 16)?;
        }
        for index in 9..=12 {
            number(tuple, index, 8)?;
        }
        let rollover = number(tuple, 13, 1)?;
        let closed = number(tuple, 14, 1)?;
        let revoked = number(tuple, 15, 1)?;
        if closed > 1 || revoked > 1 {
            return Err(());
        }
        let owner = word(tuple, 0)?;
        let source = word(tuple, 17)?;
        let record = Self {
            id,
            owner,
            account: word(tuple, 1)?,
            asset: word(tuple, 2)?,
            source: (source != [0; 32] && source != owner).then_some(source),
            limit: number(tuple, 4, 16)?,
            spent: number(tuple, 7, 16)?,
            period_start: u64::try_from(number(tuple, 10, 8)?).map_err(|_| ())?,
            expiry: u64::try_from(number(tuple, 11, 8)?).map_err(|_| ())?,
            revocation: u64::try_from(number(tuple, 12, 8)?).map_err(|_| ())?,
            closed: closed == 1,
            revoked: revoked == 1,
        };
        if id == [0; 32]
            || owner == [0; 32]
            || record.limit == 0
            || number(tuple, 9, 8)? == 0
            || record.expiry <= record.period_start
            || !matches!(rollover, 1 | 2)
            || (rollover == 1 && number(tuple, 6, 16)? != 0)
            || tuple[19 * 32..]
                .chunks_exact(32)
                .collect::<Vec<_>>()
                .windows(2)
                .any(|pair| pair[0] >= pair[1])
        {
            return Err(());
        }
        Ok(record)
    }
}

#[cfg(test)]
mod tests {
    use super::Record;

    fn vectors() -> [Vec<u8>; 2] {
        let value: serde_json::Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../tests/vectors/native-budget-records.json"
        )))
        .unwrap_or_else(|error| panic!("native vectors: {error}"));
        ["v1", "v2"].map(|key| {
            let native = crate::hex::decode(
                value[key]
                    .as_str()
                    .unwrap_or_else(|| panic!("{key}"))
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("prefix")),
            )
            .unwrap_or_else(|_| panic!("hex"));
            let mut words = vec![[0_u8; 32]; 22];
            words[0][31] = 32;
            for (index, offset) in [(1, 34), (2, 66), (3, 98), (4, 130)] {
                words[index].copy_from_slice(&native[offset..offset + 32]);
            }
            for (index, offset) in [(5, 162), (6, 178), (7, 194), (8, 210), (9, 226)] {
                words[index][16..].copy_from_slice(&native[offset..offset + 16]);
            }
            for (index, offset) in [(10, 242), (11, 250), (12, 258), (13, 266)] {
                words[index][24..].copy_from_slice(&native[offset..offset + 8]);
            }
            for (index, offset) in [(14, 274), (15, 275), (16, 276)] {
                words[index][31] = native[offset];
            }
            words[17][30..].copy_from_slice(&576_u16.to_be_bytes());
            if key == "v2" {
                words[18].copy_from_slice(&native[342..374]);
            }
            words[19][31] = 2;
            words[20].copy_from_slice(&native[278..310]);
            words[21].copy_from_slice(&native[310..342]);
            words.concat()
        })
    }

    #[test]
    fn actual_native_versions_bind_source_revocation_and_remaining() {
        for (index, bytes) in vectors().iter().enumerate() {
            let record =
                Record::from_call([1; 32], bytes).unwrap_or_else(|()| panic!("native record"));
            assert_eq!(record.id, [1; 32]);
            assert_eq!(record.owner, [2; 32]);
            assert_eq!(record.account, [3; 32]);
            assert_eq!(record.asset, [4; 32]);
            assert_eq!(record.source, (index == 1).then_some([8; 32]));
            assert_eq!(record.revocation, 31);
            assert_eq!(record.limit, 100);
            assert_eq!(record.spent, 23);
            assert_eq!(record.period_start, 1000);
            assert_eq!(record.expiry, 9000);
        }
    }

    #[test]
    fn native_record_decoder_refuses_noncanonical_encodings() {
        for bytes in vectors() {
            for end in 0..bytes.len() {
                assert!(
                    Record::from_call([1; 32], &bytes[..end]).is_err(),
                    "truncation {end}"
                );
            }
            let mut trailing = bytes.clone();
            trailing.push(0);
            assert!(Record::from_call([1; 32], &trailing).is_err());
            for (offset, value) in [
                (31, 64),
                (14 * 32 + 31, 0),
                (14 * 32 + 31, 3),
                (15 * 32 + 31, 2),
                (16 * 32 + 31, 2),
                (19 * 32 + 31, 17),
                (5 * 32, 1),
                (10 * 32, 1),
            ] {
                let mut changed = bytes.clone();
                changed[offset] = value;
                assert!(
                    Record::from_call([1; 32], &changed).is_err(),
                    "field {offset}"
                );
            }
            for (offset, length) in [(32, 32), (5 * 32, 32), (10 * 32, 32), (12 * 32, 32)] {
                let mut changed = bytes.clone();
                changed[offset..offset + length].fill(0);
                assert!(
                    Record::from_call([1; 32], &changed).is_err(),
                    "zero field {offset}"
                );
            }
            let mut duplicate = bytes.clone();
            duplicate.copy_within(20 * 32..21 * 32, 21 * 32);
            assert!(Record::from_call([1; 32], &duplicate).is_err());
            let mut unsorted = bytes.clone();
            unsorted[20 * 32..21 * 32].fill(9);
            assert!(Record::from_call([1; 32], &unsorted).is_err());
            let mut carry = bytes.clone();
            carry[8 * 32 - 1] = 1;
            assert!(Record::from_call([1; 32], &carry).is_err());
            for source in [[0; 32], [2; 32]] {
                let mut changed = bytes.clone();
                changed[18 * 32..19 * 32].copy_from_slice(&source);
                assert_eq!(
                    Record::from_call([1; 32], &changed)
                        .unwrap_or_else(|_| panic!("source"))
                        .source,
                    None
                );
            }
        }
    }

    #[test]
    fn exhausted_closed_and_revoked_budgets_never_report_spendable_value() {
        let [_, bytes] = vectors();
        for (word, field) in [(15, "closed"), (16, "revoked")] {
            let mut changed = bytes.clone();
            changed[word * 32 + 31] = 1;
            let record = Record::from_call([1; 32], &changed).unwrap_or_else(|_| panic!("flags"));
            assert_eq!(record.closed, field == "closed");
            assert_eq!(record.revoked, field == "revoked");
        }
        let mut exhausted = bytes;
        exhausted[8 * 32..9 * 32].fill(0xff);
        exhausted[8 * 32..8 * 32 + 16].fill(0);
        assert_eq!(
            Record::from_call([1; 32], &exhausted)
                .unwrap_or_else(|_| panic!("spent"))
                .spent,
            u128::MAX
        );
    }
}
