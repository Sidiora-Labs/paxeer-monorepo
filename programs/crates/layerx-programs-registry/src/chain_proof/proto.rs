use super::ChainProofError;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProtoValue<'a> {
    Varint(u64),
    Fixed64(u64),
    Bytes(&'a [u8]),
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProtoField<'a> {
    pub number: u32,
    pub value: ProtoValue<'a>,
}

pub struct ProtoReader<'a> {
    bytes: &'a [u8],
    cursor: usize,
    previous_field: u32,
}

impl<'a> ProtoReader<'a> {
    #[must_use]
    pub const fn new(bytes: &'a [u8]) -> Self {
        Self {
            bytes,
            cursor: 0,
            previous_field: 0,
        }
    }

    #[must_use]
    pub const fn cursor(&self) -> usize {
        self.cursor
    }

    pub fn next_field(&mut self) -> Result<Option<ProtoField<'a>>, ChainProofError> {
        if self.cursor == self.bytes.len() {
            return Ok(None);
        }
        let mut cursor = self.cursor;
        let key = read_varint(self.bytes, &mut cursor)?;
        let number = u32::try_from(key >> 3).map_err(|_| ChainProofError::NonCanonical)?;
        if number == 0 || number > 0x1fff_ffff || number < self.previous_field {
            return Err(ChainProofError::NonCanonical);
        }
        let value = match key & 7 {
            0 => ProtoValue::Varint(read_varint(self.bytes, &mut cursor)?),
            1 => {
                let bytes = take(self.bytes, &mut cursor, 8)?;
                let mut fixed = [0; 8];
                fixed.copy_from_slice(bytes);
                ProtoValue::Fixed64(u64::from_le_bytes(fixed))
            }
            2 => {
                let length = read_varint(self.bytes, &mut cursor)?;
                let length = usize::try_from(length).map_err(|_| ChainProofError::Truncated)?;
                ProtoValue::Bytes(take(self.bytes, &mut cursor, length)?)
            }
            _ => return Err(ChainProofError::UnknownField),
        };
        self.cursor = cursor;
        self.previous_field = number;
        Ok(Some(ProtoField { number, value }))
    }

    pub fn finish(&self) -> Result<(), ChainProofError> {
        if self.cursor == self.bytes.len() {
            Ok(())
        } else {
            Err(ChainProofError::NonCanonical)
        }
    }
}

fn read_varint(bytes: &[u8], cursor: &mut usize) -> Result<u64, ChainProofError> {
    let mut value = 0;
    for shift in (0..70).step_by(7) {
        let byte = *bytes.get(*cursor).ok_or(ChainProofError::Truncated)?;
        *cursor += 1;
        if shift == 63 && byte > 1 {
            return Err(ChainProofError::NonCanonical);
        }
        value |= u64::from(byte & 0x7f) << shift;
        if byte & 0x80 == 0 {
            if shift != 0 && byte == 0 {
                return Err(ChainProofError::NonCanonical);
            }
            return Ok(value);
        }
    }
    Err(ChainProofError::NonCanonical)
}

fn take<'a>(
    bytes: &'a [u8],
    cursor: &mut usize,
    length: usize,
) -> Result<&'a [u8], ChainProofError> {
    let end = cursor
        .checked_add(length)
        .ok_or(ChainProofError::Truncated)?;
    let value = bytes.get(*cursor..end).ok_or(ChainProofError::Truncated)?;
    *cursor = end;
    Ok(value)
}
