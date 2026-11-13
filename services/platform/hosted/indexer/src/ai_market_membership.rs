//! Native IAVL and multistore membership for the committed program namespace.
use layerx_committed_proof::proto::encode_varint as varint;
use layerx_committed_proof::ProofError;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::collections::BTreeSet;

type Result<T> = std::result::Result<T, ProofError>;
fn rejected<T>() -> Result<T> {
    Err(ProofError::InvalidProof)
}
fn digest(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn read_varint(input: &mut &[u8]) -> Result<u64> {
    let mut n = 0;
    for shift in (0..70).step_by(7) {
        let (&byte, rest) = input.split_first().ok_or(ProofError::InvalidProof)?;
        *input = rest;
        if shift == 63 && byte > 1 {
            return rejected();
        }
        n |= u64::from(byte & 127) << shift;
        if byte & 128 == 0 {
            if shift != 0 && byte == 0 {
                return rejected();
            }
            return Ok(n);
        }
    }
    rejected()
}
#[derive(Clone, Copy)]
enum Value<'a> {
    Number(u64),
    Bytes(&'a [u8]),
}
fn fields(mut input: &[u8]) -> Result<Vec<(u64, Value<'_>)>> {
    if input.len() > 1_048_576 {
        return rejected();
    }
    let mut out = Vec::new();
    let mut previous = 0;
    while !input.is_empty() {
        let tag = read_varint(&mut input)?;
        let field = tag >> 3;
        if field == 0 || field < previous || field > 0x1fff_ffff || out.len() >= 132 {
            return rejected();
        }
        previous = field;
        let value = match tag & 7 {
            0 => Value::Number(read_varint(&mut input)?),
            2 => {
                let n = usize::try_from(read_varint(&mut input)?)
                    .map_err(|_| ProofError::InvalidProof)?;
                let value = input.get(..n).ok_or(ProofError::InvalidProof)?;
                input = &input[n..];
                Value::Bytes(value)
            }
            _ => return rejected(),
        };
        out.push((field, value));
    }
    Ok(out)
}
fn field_bytes<'a>(field: &(u64, Value<'a>), n: u64) -> Result<&'a [u8]> {
    match field {
        (number, Value::Bytes(value)) if *number == n => Ok(value),
        _ => rejected(),
    }
}
fn one_bytes(input: &[u8], n: u64) -> Result<&[u8]> {
    let f = fields(input)?;
    if f.len() != 1 {
        return rejected();
    }
    field_bytes(&f[0], n)
}
struct Existence<'a> {
    key: &'a [u8],
    value: &'a [u8],
    leaf: &'a [u8],
    path: Vec<&'a [u8]>,
}
fn existence(input: &[u8]) -> Result<Existence<'_>> {
    let f = fields(input)?;
    if f.len() < 3 || f.len() > 131 {
        return rejected();
    }
    Ok(Existence {
        key: field_bytes(&f[0], 1)?,
        value: field_bytes(&f[1], 2)?,
        leaf: field_bytes(&f[2], 3)?,
        path: f[3..]
            .iter()
            .map(|v| field_bytes(v, 4))
            .collect::<Result<_>>()?,
    })
}
fn leaf_prefix(input: &[u8]) -> Result<&[u8]> {
    let f = fields(input)?;
    let mut ops = [0u64; 4];
    let mut prefix = None;
    let mut seen = BTreeSet::new();
    for (n, v) in f {
        if !seen.insert(n) {
            return rejected();
        }
        match (n, v) {
            (1, Value::Number(value)) => ops[0] = value,
            (2, Value::Number(value)) => ops[1] = value,
            (3, Value::Number(value)) => ops[2] = value,
            (4, Value::Number(value)) => ops[3] = value,
            (5, Value::Bytes(value)) => prefix = Some(value),
            _ => return rejected(),
        }
    }
    if ops != [1, 0, 1, 1] {
        return rejected();
    }
    prefix.ok_or(ProofError::InvalidProof)
}
fn metadata(input: &mut &[u8]) -> Result<u64> {
    let mut height = 0;
    for i in 0..3 {
        let n = read_varint(input)?;
        if n & 1 != 0 {
            return rejected();
        }
        if i == 0 {
            height = n >> 1;
        }
    }
    Ok(height)
}
fn existence_root(proof: &Existence<'_>, iavl: bool) -> Result<[u8; 32]> {
    if proof.key.is_empty() || proof.value.is_empty() || proof.path.len() > 128 {
        return rejected();
    }
    let prefix = leaf_prefix(proof.leaf)?;
    if iavl {
        let mut tail = prefix;
        if prefix.first() != Some(&0) || metadata(&mut tail)? != 0 || !tail.is_empty() {
            return rejected();
        }
    } else if prefix != [0] {
        return rejected();
    }
    let mut leaf = prefix.to_vec();
    varint(proof.key.len() as u64, &mut leaf);
    leaf.extend(proof.key);
    varint(32, &mut leaf);
    leaf.extend(digest(proof.value));
    let mut root = digest(&leaf);
    for (depth, op) in proof.path.iter().enumerate() {
        let f = fields(op)?;
        let (prefix, suffix) = match f.as_slice() {
            [(1, Value::Number(1)), (2, Value::Bytes(prefix))] => (*prefix, &[][..]),
            [(1, Value::Number(1)), (2, Value::Bytes(prefix)), (3, Value::Bytes(suffix))] => {
                (*prefix, *suffix)
            }
            _ => return rejected(),
        };
        if iavl {
            let left = match suffix.len() {
                33 => 0,
                0 => 1,
                _ => return rejected(),
            };
            let length = prefix
                .len()
                .checked_sub(left * 33)
                .ok_or(ProofError::InvalidProof)?;
            if !(4..=12).contains(&length) || prefix.first() == Some(&0) {
                return rejected();
            }
            let mut tail = prefix;
            if metadata(&mut tail)? < depth as u64 + 1 {
                return rejected();
            }
            match (left, tail) {
                (0, [32]) if suffix[0] == 32 => {}
                (1, [32, ..]) if tail.len() == 34 && tail[33] == 32 => {}
                _ => return rejected(),
            }
        } else if !((prefix == [1] && suffix.len() == 32)
            || (prefix.len() == 33 && prefix[0] == 1 && suffix.is_empty()))
        {
            return rejected();
        }
        root = digest(&[prefix, &root, suffix].concat());
    }
    Ok(root)
}

/// An IAVL existence proof and the multistore path linking it to the app hash.
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Membership {
    pub proof: Vec<u8>,
    pub multistore: Vec<u8>,
}
impl Membership {
    pub fn verify(&self, app_hash: [u8; 32], store: &[u8], key: &[u8], value: &[u8]) -> Result<()> {
        let proof = existence(&self.proof)?;
        if proof.key != key || proof.value != value {
            return rejected();
        }
        let root = existence_root(&proof, true)?;
        let outer = existence(one_bytes(&self.multistore, 1)?)?;
        if outer.key != store || outer.value != root || existence_root(&outer, false)? != app_hash {
            return rejected();
        }
        Ok(())
    }
    pub fn value(&self) -> Result<&[u8]> {
        Ok(existence(&self.proof)?.value)
    }
}

/// Authentic native shared-storage head plus its content-addressed manifest.
#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProgramStateProof {
    membership: Membership,
    manifest: Vec<u8>,
}
impl ProgramStateProof {
    pub fn verify(&self, app_hash: [u8; 32], program: &[u8; 32], state: &[u8]) -> Result<()> {
        let mut key = b"\x01\x00\x09progstor".to_vec();
        key.extend(program);
        key.push(1);
        let value = self.membership.value()?;
        if value.len() != 38
            || value[..2] != [0, 1]
            || self.manifest.len() > 1_048_576
            || digest(&self.manifest) != value[6..]
        {
            return rejected();
        }
        self.membership.verify(app_hash, b"lxphost", &key, value)?;
        let count = u32::from_be_bytes(
            value[2..6]
                .try_into()
                .map_err(|_| ProofError::InvalidProof)?,
        );
        if count == 0 || count > 65_536 || self.manifest.get(..6) != Some(&value[..6]) {
            return rejected();
        }
        let mut input = &self.manifest[6..];
        let mut previous: Option<Vec<u8>> = None;
        let mut found = false;
        for _ in 0..count {
            let n = u16::from_be_bytes(
                take(&mut input, 2)?
                    .try_into()
                    .map_err(|_| ProofError::InvalidProof)?,
            ) as usize;
            if n == 0 || n > 1024 {
                return rejected();
            }
            let key = take(&mut input, n)?;
            if previous.as_ref().is_some_and(|v| v.as_slice() >= key) {
                return rejected();
            }
            previous = Some(key.to_vec());
            let hash = take(&mut input, 32)?;
            let length = u32::from_be_bytes(
                take(&mut input, 4)?
                    .try_into()
                    .map_err(|_| ProofError::InvalidProof)?,
            ) as usize;
            if key == layerx_programs_ai_market::SHARED_STATE_KEY {
                if length != state.len() || hash != digest(state) {
                    return rejected();
                }
                found = true;
            }
        }
        if !input.is_empty() || !found {
            return rejected();
        }
        Ok(())
    }
}
fn take<'a>(input: &mut &'a [u8], n: usize) -> Result<&'a [u8]> {
    let value = input.get(..n).ok_or(ProofError::InvalidProof)?;
    *input = &input[n..];
    Ok(value)
}

pub(super) fn authenticate(
    encoded: &[u8],
    app_hash: [u8; 32],
    program: &[u8; 32],
    state: &[u8],
) -> Result<()> {
    if encoded.len() > 4 * 1_048_576 {
        return rejected();
    }
    let proof: ProgramStateProof =
        serde_json::from_slice(encoded).map_err(|_| ProofError::InvalidProof)?;
    proof.verify(app_hash, program, state)
}
