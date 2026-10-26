use sha2::{Digest, Sha256};

const MAX_PROOF_BYTES: usize = 1 << 20;
const MAX_PATH_DEPTH: usize = 128;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ExistenceProof {
    pub key: Vec<u8>,
    pub value: Vec<u8>,
    pub leaf: LeafOp,
    pub path: Vec<InnerOp>,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct LeafOp {
    pub hash: u32,
    pub prehash_key: u32,
    pub prehash_value: u32,
    pub length: u32,
    pub prefix: Vec<u8>,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct InnerOp {
    pub hash: u32,
    pub prefix: Vec<u8>,
    pub suffix: Vec<u8>,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ProofError {
    Malformed,
    RootMismatch,
}

impl std::fmt::Display for ProofError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(match self {
            Self::Malformed => "malformed IAVL existence proof",
            Self::RootMismatch => "IAVL proof does not match the root, key or value",
        })
    }
}

impl std::error::Error for ProofError {}

fn varint(input: &mut &[u8]) -> Result<u64, ProofError> {
    let mut value = 0;
    for i in 0..10 {
        let (&byte, rest) = input.split_first().ok_or(ProofError::Malformed)?;
        *input = rest;
        if i == 9 && byte > 1 {
            return Err(ProofError::Malformed);
        }
        value |= u64::from(byte & 0x7f) << (7 * i);
        if byte & 0x80 == 0 {
            if i > 0 && byte == 0 {
                return Err(ProofError::Malformed);
            }
            return Ok(value);
        }
    }
    Err(ProofError::Malformed)
}

fn take<'a>(input: &mut &'a [u8], len: usize) -> Result<&'a [u8], ProofError> {
    let bytes = input.get(..len).ok_or(ProofError::Malformed)?;
    *input = &input[len..];
    Ok(bytes)
}

fn field(input: &mut &[u8]) -> Result<(u32, u8), ProofError> {
    let tag = varint(input)?;
    let number = tag >> 3;
    if number == 0 || number > 0x1fff_ffff {
        return Err(ProofError::Malformed);
    }
    Ok((number as u32, (tag & 7) as u8))
}

fn bytes_field<'a>(input: &mut &'a [u8], wire: u8) -> Result<&'a [u8], ProofError> {
    if wire != 2 {
        return Err(ProofError::Malformed);
    }
    let len = usize::try_from(varint(input)?).map_err(|_| ProofError::Malformed)?;
    take(input, len)
}

fn enum_field(input: &mut &[u8], wire: u8) -> Result<u32, ProofError> {
    if wire != 0 {
        return Err(ProofError::Malformed);
    }
    let value = varint(input)?;
    if value > i32::MAX as u64 {
        return Err(ProofError::Malformed);
    }
    Ok(value as u32)
}

fn skip(input: &mut &[u8], wire: u8) -> Result<(), ProofError> {
    match wire {
        0 => {
            varint(input)?;
        }
        1 => {
            take(input, 8)?;
        }
        2 => {
            bytes_field(input, wire)?;
        }
        5 => {
            take(input, 4)?;
        }
        _ => return Err(ProofError::Malformed),
    }
    Ok(())
}

fn once(seen: &mut u8, number: u32) -> Result<(), ProofError> {
    let bit = 1 << number;
    if *seen & bit != 0 {
        return Err(ProofError::Malformed);
    }
    *seen |= bit;
    Ok(())
}

fn decode_leaf(mut input: &[u8]) -> Result<LeafOp, ProofError> {
    let mut op = LeafOp::default();
    let mut seen = 0;
    while !input.is_empty() {
        let (number, wire) = field(&mut input)?;
        if (1..=5).contains(&number) {
            once(&mut seen, number)?;
        }
        match number {
            1 => op.hash = enum_field(&mut input, wire)?,
            2 => op.prehash_key = enum_field(&mut input, wire)?,
            3 => op.prehash_value = enum_field(&mut input, wire)?,
            4 => op.length = enum_field(&mut input, wire)?,
            5 => op.prefix = bytes_field(&mut input, wire)?.to_vec(),
            _ => skip(&mut input, wire)?,
        }
    }
    Ok(op)
}

fn decode_inner(mut input: &[u8]) -> Result<InnerOp, ProofError> {
    let mut op = InnerOp::default();
    let mut seen = 0;
    while !input.is_empty() {
        let (number, wire) = field(&mut input)?;
        if (1..=3).contains(&number) {
            once(&mut seen, number)?;
        }
        match number {
            1 => op.hash = enum_field(&mut input, wire)?,
            2 => op.prefix = bytes_field(&mut input, wire)?.to_vec(),
            3 => op.suffix = bytes_field(&mut input, wire)?.to_vec(),
            _ => skip(&mut input, wire)?,
        }
    }
    Ok(op)
}

pub fn decode_existence(mut bytes: &[u8]) -> Result<ExistenceProof, ProofError> {
    if bytes.len() > MAX_PROOF_BYTES {
        return Err(ProofError::Malformed);
    }
    let (mut key, mut value, mut leaf) = (None, None, None);
    let mut path = Vec::new();
    let mut seen = 0;
    while !bytes.is_empty() {
        let (number, wire) = field(&mut bytes)?;
        if (1..=3).contains(&number) {
            once(&mut seen, number)?;
        }
        match number {
            1 => key = Some(bytes_field(&mut bytes, wire)?.to_vec()),
            2 => value = Some(bytes_field(&mut bytes, wire)?.to_vec()),
            3 => leaf = Some(decode_leaf(bytes_field(&mut bytes, wire)?)?),
            4 => {
                if path.len() == MAX_PATH_DEPTH {
                    return Err(ProofError::Malformed);
                }
                path.push(decode_inner(bytes_field(&mut bytes, wire)?)?);
            }
            _ => skip(&mut bytes, wire)?,
        }
    }
    Ok(ExistenceProof {
        key: key.ok_or(ProofError::Malformed)?,
        value: value.ok_or(ProofError::Malformed)?,
        leaf: leaf.ok_or(ProofError::Malformed)?,
        path,
    })
}

fn node_header(input: &mut &[u8]) -> Result<(u64, u64, u64), ProofError> {
    let height = varint(input)?;
    let size = varint(input)?;
    let version = varint(input)?;
    // IAVL uses Go's signed (zigzag) varints for node metadata.
    if (height | size | version) & 1 != 0 || height >> 1 > 127 {
        return Err(ProofError::Malformed);
    }
    Ok((height >> 1, size >> 1, version >> 1))
}

fn hash_length(hash: &mut Sha256, mut len: usize) {
    while len >= 128 {
        hash.update([(len as u8 & 0x7f) | 0x80]);
        len >>= 7;
    }
    hash.update([len as u8]);
}

pub fn verify_existence(
    proof: &ExistenceProof,
    root: [u8; 32],
    key: &[u8],
    value: &[u8],
) -> Result<(), ProofError> {
    let leaf = &proof.leaf;
    if leaf.hash != 1
        || leaf.prehash_key != 0
        || leaf.prehash_value != 1
        || leaf.length != 1
        || proof.key.is_empty()
        || proof.value.is_empty()
        || proof.path.len() > MAX_PATH_DEPTH
    {
        return Err(ProofError::Malformed);
    }
    let mut header = leaf.prefix.as_slice();
    let (height, size, _) = node_header(&mut header)?;
    if height != 0 || size != 1 || !header.is_empty() {
        return Err(ProofError::Malformed);
    }
    if proof.key != key || proof.value != value {
        return Err(ProofError::RootMismatch);
    }
    let mut hash = Sha256::new();
    hash.update(&leaf.prefix);
    hash_length(&mut hash, key.len());
    hash.update(key);
    hash_length(&mut hash, 32);
    hash.update(Sha256::digest(value));
    let mut child: [u8; 32] = hash.finalize().into();
    let mut previous_height = 0;
    for op in &proof.path {
        if op.hash != 1 {
            return Err(ProofError::Malformed);
        }
        let mut rest = op.prefix.as_slice();
        let (height, size, _) = node_header(&mut rest)?;
        let header_len = op.prefix.len() - rest.len();
        if height <= previous_height || size < 2 || !(3..=11).contains(&header_len) {
            return Err(ProofError::Malformed);
        }
        match (rest.len(), op.suffix.len()) {
            (1, 33) if rest[0] == 32 && op.suffix[0] == 32 => {}
            (34, 0) if rest[0] == 32 && rest[33] == 32 => {}
            _ => return Err(ProofError::Malformed),
        }
        previous_height = height;
        let mut hash = Sha256::new();
        hash.update(&op.prefix);
        hash.update(child);
        hash.update(&op.suffix);
        child = hash.finalize().into();
    }
    if child != root {
        return Err(ProofError::RootMismatch);
    }
    Ok(())
}
