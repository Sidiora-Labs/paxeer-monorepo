//! Authentication of direct Programs precompile invocations in included node transactions.

use crate::{Lease, LeaseState, SandboxActivityRefusal, SandboxReceiptEvidence, SandboxSubmission};
use layerx_programs::chain_proof::{
    decode::CommitmentProof,
    header::ChainHeader,
    proto::{ProtoReader, ProtoValue},
    results::TxResult,
    VerifiedChainHead,
};
use layerx_programs::verify_state;
use layerx_programs_runtime::{hash_bytes, recover_secp256k1, HashAlgorithm, PrincipalId};

type Result<T> = core::result::Result<T, SandboxActivityRefusal>;
const INVALID: SandboxActivityRefusal = SandboxActivityRefusal::InvalidReceipt;
const EVM_MESSAGE: &[u8] = b"/paxprotocol.paxchain.evm.MsgEVMTransaction";
const PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1d,
];

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SandboxInvocationEvidence {
    /// The signed Ethereum encoding carried by the node's MsgEVMTransaction.
    pub ethereum_tx: Vec<u8>,
    /// Canonical lxphost.Activity bytes, hashed under the native activity-id domain.
    pub native_activity: Vec<u8>,
    /// EVM address to DID key binding at the transaction block's pre-state.
    pub signer_binding: CommitmentProof,
    /// The pre-state lease for activation and execution; funding carries its request.
    pub previous_lease: Option<CommitmentProof>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedSandboxReceiptEvidence {
    pub chain: SandboxReceiptEvidence,
    pub invocation: SandboxInvocationEvidence,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SandboxReceiptProof {
    Authenticated(AuthenticatedSandboxReceiptEvidence),
    Unbound(SandboxReceiptEvidence),
}
impl From<AuthenticatedSandboxReceiptEvidence> for SandboxReceiptProof {
    fn from(value: AuthenticatedSandboxReceiptEvidence) -> Self {
        Self::Authenticated(value)
    }
}
impl From<SandboxReceiptEvidence> for SandboxReceiptProof {
    fn from(value: SandboxReceiptEvidence) -> Self {
        Self::Unbound(value)
    }
}

pub(crate) struct InvocationBinding {
    pub activity_id: [u8; 32],
    pub previous_lease: Option<Lease>,
}

impl SandboxInvocationEvidence {
    pub(crate) fn verify(
        &self,
        head: &VerifiedChainHead,
        submission: &SandboxSubmission,
        chain_tx: &[u8],
        result: &TxResult,
    ) -> Result<InvocationBinding> {
        let tx = EthereumInvocation::decode(&self.ethereum_tx)?;
        tx.verify_node_message(chain_tx)?;
        tx.verify_result(result)?;
        let did_key = proven_value(
            head,
            &self.signer_binding,
            "evm",
            &[&[0x20][..], &tx.signer[..]].concat(),
        )?;
        if did_key.len() != 32 {
            return Err(INVALID);
        }
        let mut did = b"did:layerx:".to_vec();
        did.extend_from_slice(hex(did_key).as_bytes());
        let tenant = PrincipalId::new(domain_hash(
            b"LXP/v1/did-id\0",
            &[&(did.len() as u16).to_be_bytes()[..], &did[..]].concat(),
        )?)
        .map_err(|_| INVALID)?;
        let operation = *submission.payload().get(1).ok_or(INVALID)?;
        let previous_lease = if operation == 2 {
            let payload = submission.payload();
            let length = be_usize(payload.get(248..252).ok_or(INVALID)?)?;
            let lease = Lease::decode_state(
                payload
                    .get(252..252usize.checked_add(length).ok_or(INVALID)?)
                    .ok_or(INVALID)?,
            )
            .map_err(|_| INVALID)?;
            if lease.id() != submission.lease()
                || lease.tenant() != tenant
                || lease.state() != LeaseState::Requested
            {
                return Err(INVALID);
            }
            let did_key = did_key.try_into().map_err(|_| INVALID)?;
            let transfer_start = 256usize.checked_add(length).ok_or(INVALID)?;
            let expected = crate::activity::canonical_fund_for_did(
                crate::SandboxProtocolContext::new(3, 4)?,
                &lease,
                did_key,
                payload.get(transfer_start..).ok_or(INVALID)?,
            )?;
            if expected != *submission {
                return Err(INVALID);
            }
            None
        } else {
            let proof = self.previous_lease.as_ref().ok_or(INVALID)?;
            let key = crate::sandbox_state_key(0, submission.lease().bytes())?;
            let lease = Lease::decode_state(proven_value(head, proof, "lxphost", &key)?)
                .map_err(|_| INVALID)?;
            if lease.id() != submission.lease()
                || lease.tenant() != tenant
                || lease.state_digest().map_err(|_| INVALID)? != submission.expected_lease_digest()
                || (operation == 3
                    && (lease.state() != LeaseState::Funded
                        || lease.history().len() as u64 + 1
                            != submission.expected_usage_sequence()))
                || (operation == 1 && lease.state() != LeaseState::Active)
            {
                return Err(INVALID);
            }
            if operation == 1 {
                let expected = crate::canonical_execute(
                    crate::SandboxProtocolContext::new(3, 4)?,
                    &lease,
                    submission.expected_usage_sequence(),
                    submission.payload().get(236..).ok_or(INVALID)?,
                )?;
                if expected != *submission {
                    return Err(INVALID);
                }
            }
            Some(lease)
        };
        let fee_limit = if operation == 1 {
            let args = tx.calldata.get(4..).ok_or(INVALID)?;
            let fee = args.get(15 * 32..16 * 32).ok_or(INVALID)?;
            if fee[..16] != [0; 16] {
                return Err(INVALID);
            }
            fee[16..].try_into().map_err(|_| INVALID)?
        } else {
            [0; 16]
        };
        let expected_call = canonical_precompile_calldata(submission, fee_limit)?;
        if tx.calldata != expected_call {
            return Err(INVALID);
        }
        let activity_id = verify_native_activity(
            &self.native_activity,
            submission,
            &did,
            did_key,
            tx.hash,
            &expected_call[..4],
            fee_limit,
            head.time_ms(),
        )?;
        Ok(InvocationBinding {
            activity_id,
            previous_lease,
        })
    }
}

fn proven_value<'a>(
    head: &VerifiedChainHead,
    proof: &'a CommitmentProof,
    store: &str,
    key: &[u8],
) -> Result<&'a [u8]> {
    let header = ChainHeader::decode(&proof.signed_header).map_err(|_| INVALID)?;
    if header.hash().map_err(|_| INVALID)? != head.header_hash()
        || proof.store_name != store
        || proof.key != key
    {
        return Err(INVALID);
    }
    let value = proof.value.as_deref().ok_or(INVALID)?;
    verify_state(head, store, key, value, &proof.proof_ops).map_err(|_| INVALID)?;
    Ok(value)
}

/// Encodes the exact ABI arguments accepted by the native Programs precompile.
/// # Errors
/// Returns a refusal for a malformed submission or a noncanonical funding transfer.
pub fn canonical_precompile_calldata(
    submission: &SandboxSubmission,
    fee_limit: [u8; 16],
) -> Result<Vec<u8>> {
    let p = submission.payload();
    let mut args = Vec::new();
    let signature;
    match p.get(1) {
        Some(3) => {
            signature = "sandboxActivate(bytes32,bytes32,uint64)";
            args.extend_from_slice(&p[4..68]);
            word(&p[68..76], &mut args)?;
        }
        Some(1) => {
            signature = "sandboxExecute(bytes32,uint64,bytes32,bytes32,bytes32,bytes32,uint32,uint64[7],bytes,uint128)";
            args.extend_from_slice(&p[4..36]);
            word(&p[36..44], &mut args)?;
            args.extend_from_slice(&p[44..172]);
            word(&p[172..176], &mut args)?;
            for price in p[176..232].chunks_exact(8) {
                word(price, &mut args)?;
            }
            word(&(16u64 * 32).to_be_bytes(), &mut args)?;
            word(&fee_limit, &mut args)?;
            dynamic(&p[236..], &mut args)?;
        }
        Some(2) => {
            if fee_limit != [0; 16] {
                return Err(INVALID);
            }
            signature = "sandboxFund(bytes32,bytes32,bytes32,bytes32,bytes32,uint128,uint64,uint32,uint64[7],bytes,bytes32,(bytes32,bytes32,bytes32,uint128)[])";
            args.extend_from_slice(&p[4..164]);
            word(&p[164..180], &mut args)?;
            word(&p[180..188], &mut args)?;
            word(&p[188..192], &mut args)?;
            for price in p[192..248].chunks_exact(8) {
                word(price, &mut args)?;
            }
            let n = be_usize(&p[248..252])?;
            let end = 252usize.checked_add(n).ok_or(INVALID)?;
            let request = p.get(252..end).ok_or(INVALID)?;
            let transfer = p.get(end + 4..).ok_or(INVALID)?;
            if transfer.len() != 146 || transfer[32..34] != [0, 1] {
                return Err(INVALID);
            }
            word(&(18u64 * 32).to_be_bytes(), &mut args)?;
            args.extend_from_slice(&transfer[..32]);
            let mut tail = Vec::new();
            dynamic(request, &mut tail)?;
            word(&(18u64 * 32 + tail.len() as u64).to_be_bytes(), &mut args)?;
            args.extend_from_slice(&tail);
            word(&1u64.to_be_bytes(), &mut args)?;
            args.extend_from_slice(&transfer[34..130]);
            word(&transfer[130..146], &mut args)?;
        }
        _ => return Err(INVALID),
    }
    let selector =
        hash_bytes(HashAlgorithm::Keccak256, signature.as_bytes()).map_err(|_| INVALID)?;
    Ok([&selector[..4], &args].concat())
}
fn word(bytes: &[u8], out: &mut Vec<u8>) -> Result<()> {
    if bytes.len() > 32 {
        return Err(INVALID);
    }
    out.resize(out.len() + 32 - bytes.len(), 0);
    out.extend_from_slice(bytes);
    Ok(())
}
fn dynamic(bytes: &[u8], out: &mut Vec<u8>) -> Result<()> {
    word(&(bytes.len() as u64).to_be_bytes(), out)?;
    out.extend_from_slice(bytes);
    out.resize(out.len().div_ceil(32) * 32, 0);
    Ok(())
}
fn domain_hash(domain: &[u8], value: &[u8]) -> Result<[u8; 32]> {
    hash_bytes(HashAlgorithm::Sha256, &[domain, value].concat()).map_err(|_| INVALID)
}
fn be_usize(bytes: &[u8]) -> Result<usize> {
    bytes.iter().try_fold(0usize, |n, b| {
        n.checked_mul(256)
            .and_then(|n| n.checked_add(usize::from(*b)))
            .ok_or(INVALID)
    })
}
fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|b| {
            [
                char::from(DIGITS[usize::from(b >> 4)]),
                char::from(DIGITS[usize::from(b & 15)]),
            ]
        })
        .collect()
}
fn unhex(text: &[u8]) -> Result<Vec<u8>> {
    let text = text.strip_prefix(b"0x").ok_or(INVALID)?;
    if text.len() % 2 != 0 {
        return Err(INVALID);
    }
    text.chunks_exact(2)
        .map(|pair| {
            let digit = |v: u8| match v {
                b'0'..=b'9' => Ok(v - b'0'),
                b'a'..=b'f' => Ok(v - b'a' + 10),
                b'A'..=b'F' => Ok(v - b'A' + 10),
                _ => Err(INVALID),
            };
            Ok(digit(pair[0])? * 16 + digit(pair[1])?)
        })
        .collect()
}

fn verify_native_activity(
    bytes: &[u8],
    submission: &SandboxSubmission,
    did: &[u8],
    key: &[u8],
    tx_hash: [u8; 32],
    selector: &[u8],
    fee: [u8; 16],
    timestamp: i64,
) -> Result<[u8; 32]> {
    let mut c = ActivityCursor { bytes, position: 0 };
    if c.take(5)? != [0, 3, 0x10, 1, 12] {
        return Err(INVALID);
    }
    c.tag(1)?;
    if c.take(2)? != [0, 3] {
        return Err(INVALID);
    }
    c.tag(2)?;
    c.take(4)?;
    c.tag(3)?;
    if c.take(4)? != submission.activity_type().to_be_bytes() {
        return Err(INVALID);
    }
    c.tag(4)?;
    if c.blob()? != did {
        return Err(INVALID);
    }
    c.tag(5)?;
    if c.blob()? != key {
        return Err(INVALID);
    }
    c.tag(6)?;
    c.take(8)?;
    c.tag(7)?;
    let timestamp = u64::try_from(timestamp).map_err(|_| INVALID)?.to_be_bytes();
    if c.take(8)? != timestamp || c.take(8)? != timestamp {
        return Err(INVALID);
    }
    c.tag(8)?;
    let idempotency = domain_hash(
        b"layerxprograms/idempotency/v1",
        &[&tx_hash[..], &[0; 4], selector].concat(),
    )?;
    if c.blob()? != idempotency {
        return Err(INVALID);
    }
    c.tag(9)?;
    if c.take(16)? != fee {
        return Err(INVALID);
    }
    c.tag(10)?;
    if c.blob()? != domain_hash(b"LXP/v1/payload-hash\0", submission.payload())? {
        return Err(INVALID);
    }
    c.tag(11)?;
    if c.blob()? != submission.payload() {
        return Err(INVALID);
    }
    c.tag(12)?;
    if !c.blob()?.is_empty() || c.position != bytes.len() {
        return Err(INVALID);
    }
    domain_hash(b"LXP/v1/activity-id\0", bytes)
}
struct ActivityCursor<'a> {
    bytes: &'a [u8],
    position: usize,
}
impl<'a> ActivityCursor<'a> {
    fn take(&mut self, n: usize) -> Result<&'a [u8]> {
        let end = self.position.checked_add(n).ok_or(INVALID)?;
        let bytes = self.bytes.get(self.position..end).ok_or(INVALID)?;
        self.position = end;
        Ok(bytes)
    }
    fn tag(&mut self, tag: u8) -> Result<()> {
        if self.take(1)? != [tag] {
            return Err(INVALID);
        }
        Ok(())
    }
    fn blob(&mut self) -> Result<&'a [u8]> {
        let n = be_usize(self.take(4)?)?;
        self.take(n)
    }
}

type Fields<'a> = Vec<(u32, ProtoValue<'a>)>;
fn fields(bytes: &[u8]) -> Result<Fields<'_>> {
    let mut r = ProtoReader::new(bytes);
    let mut out = Vec::new();
    while let Some(f) = r.next_field().map_err(|_| INVALID)? {
        out.push((f.number, f.value));
    }
    Ok(out)
}
fn bytes_field<'a>(f: &Fields<'a>, n: u32) -> Result<&'a [u8]> {
    let mut values = f.iter().filter(|(i, _)| *i == n);
    let value = match values.next() {
        None => &[][..],
        Some((_, ProtoValue::Bytes(b))) => *b,
        _ => return Err(INVALID),
    };
    if values.next().is_some() {
        return Err(INVALID);
    }
    Ok(value)
}
fn scalar_field(f: &Fields<'_>, n: u32) -> Result<u64> {
    let mut values = f.iter().filter(|(i, _)| *i == n);
    let value = match values.next() {
        None => 0,
        Some((_, ProtoValue::Varint(b))) => *b,
        _ => return Err(INVALID),
    };
    if values.next().is_some() {
        return Err(INVALID);
    }
    Ok(value)
}
fn only_fields(f: &Fields<'_>, max: u32) -> Result<()> {
    if f.iter().any(|(n, _)| *n > max) {
        return Err(INVALID);
    }
    Ok(())
}
fn any(bytes: &[u8], expected: &[u8]) -> Result<Vec<u8>> {
    let f = fields(bytes)?;
    only_fields(&f, 2)?;
    if bytes_field(&f, 1)? != expected {
        return Err(INVALID);
    }
    Ok(bytes_field(&f, 2)?.to_vec())
}

#[derive(Clone)]
struct Rlp<'a> {
    data: &'a [u8],
    encoded: &'a [u8],
    list: bool,
}
fn rlp(bytes: &[u8]) -> Result<(Rlp<'_>, usize)> {
    let first = *bytes.first().ok_or(INVALID)?;
    let (start, len, list) = match first {
        0..=0x7f => (0, 1, false),
        0x80..=0xb7 => (1, usize::from(first - 0x80), false),
        0xc0..=0xf7 => (1, usize::from(first - 0xc0), true),
        _ => {
            let list = first >= 0xf8;
            let n = usize::from(first - if list { 0xf7 } else { 0xb7 });
            let length = bytes.get(1..1 + n).ok_or(INVALID)?;
            if length.first() == Some(&0) {
                return Err(INVALID);
            }
            let len = be_usize(length)?;
            if len < 56 {
                return Err(INVALID);
            }
            (1 + n, len, list)
        }
    };
    let end = start.checked_add(len).ok_or(INVALID)?;
    let data = bytes.get(start..end).ok_or(INVALID)?;
    if !list && start == 1 && len == 1 && data[0] < 0x80 {
        return Err(INVALID);
    }
    Ok((
        Rlp {
            data,
            encoded: &bytes[..end],
            list,
        },
        end,
    ))
}
fn children<'a>(value: &Rlp<'a>) -> Result<Vec<Rlp<'a>>> {
    if !value.list {
        return Err(INVALID);
    }
    let mut bytes = value.data;
    let mut out = Vec::new();
    while !bytes.is_empty() {
        let (v, n) = rlp(bytes)?;
        out.push(v);
        bytes = &bytes[n..];
    }
    Ok(out)
}
fn integer<'a>(value: &Rlp<'a>) -> Result<&'a [u8]> {
    if value.list || value.data.len() > 32 || value.data.first() == Some(&0) {
        return Err(INVALID);
    }
    Ok(value.data)
}
fn rlp_encode(bytes: &[u8], list: bool) -> Vec<u8> {
    if !list && bytes.len() == 1 && bytes[0] < 0x80 {
        return bytes.to_vec();
    }
    let offset = if list { 0xc0u8 } else { 0x80u8 };
    let mut out = Vec::new();
    if bytes.len() < 56 {
        out.push(offset + bytes.len() as u8);
    } else {
        let n = bytes.len().to_be_bytes();
        let n = &n[n.iter().position(|b| *b != 0).unwrap_or(n.len())..];
        out.push(offset + 55 + n.len() as u8);
        out.extend_from_slice(n);
    }
    out.extend_from_slice(bytes);
    out
}
fn decimal(bytes: &[u8]) -> Vec<u8> {
    let mut digits = vec![0u8];
    for b in bytes {
        let mut carry = u16::from(*b);
        for d in &mut digits {
            carry += u16::from(*d) * 256;
            *d = (carry % 10) as u8;
            carry /= 10;
        }
        while carry != 0 {
            digits.push((carry % 10) as u8);
            carry /= 10;
        }
    }
    digits.into_iter().rev().map(|d| b'0' + d).collect()
}

struct EthereumInvocation<'a> {
    values: Vec<Rlp<'a>>,
    kind: u8,
    signer: [u8; 20],
    hash: [u8; 32],
    calldata: &'a [u8],
}
impl<'a> EthereumInvocation<'a> {
    fn decode(raw: &'a [u8]) -> Result<Self> {
        if raw.len() > crate::activity::MAX_CANONICAL_CALL_BYTES + 8192 {
            return Err(INVALID);
        }
        let (kind, body) = match raw.first() {
            Some(1 | 2) => (*raw.first().ok_or(INVALID)?, &raw[1..]),
            Some(0xc0..=0xff) => (0, raw),
            _ => return Err(INVALID),
        };
        let (root, n) = rlp(body)?;
        if n != body.len() {
            return Err(INVALID);
        }
        let v = children(&root)?;
        let unsigned = match kind {
            0 => 6,
            1 => 8,
            2 => 9,
            _ => return Err(INVALID),
        };
        if v.len() != unsigned + 3 {
            return Err(INVALID);
        }
        let to_index = match kind {
            0 => 3,
            1 => 4,
            _ => 5,
        };
        if v[to_index].list
            || v[to_index].data != PRECOMPILE
            || !integer(&v[to_index + 1])?.is_empty()
            || v[to_index + 2].list
        {
            return Err(INVALID);
        }
        let mut sign = v[..unsigned]
            .iter()
            .flat_map(|f| f.encoded.iter().copied())
            .collect::<Vec<_>>();
        let signature_v = be_usize(integer(&v[unsigned])?)?;
        let recovery = if kind == 0 {
            if signature_v < 37 {
                return Err(INVALID);
            }
            let chain = (signature_v - 35) / 2;
            let b = chain.to_be_bytes();
            let b = &b[b.iter().position(|v| *v != 0).ok_or(INVALID)?..];
            sign.extend(rlp_encode(b, false));
            sign.extend_from_slice(&[0x80, 0x80]);
            ((signature_v - 35) % 2) as u8
        } else {
            if signature_v > 1 || integer(&v[0])?.is_empty() {
                return Err(INVALID);
            }
            signature_v as u8
        };
        let mut preimage = rlp_encode(&sign, true);
        if kind != 0 {
            preimage.insert(0, kind);
        }
        let digest = hash_bytes(HashAlgorithm::Keccak256, &preimage).map_err(|_| INVALID)?;
        let mut signature = [0u8; 64];
        for (i, part) in [integer(&v[unsigned + 1])?, integer(&v[unsigned + 2])?]
            .iter()
            .enumerate()
        {
            if part.is_empty() {
                return Err(INVALID);
            }
            signature[(i + 1) * 32 - part.len()..(i + 1) * 32].copy_from_slice(part);
        }
        // EIP-2 rejects the malleable high-s form.
        const HALF_ORDER: [u8; 32] = [
            0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
            0xff, 0xff, 0x5d, 0x57, 0x6e, 0x73, 0x57, 0xa4, 0x50, 0x1d, 0xdf, 0xe9, 0x2f, 0x46,
            0x68, 0x1b, 0x20, 0xa0,
        ];
        if signature[32..] > HALF_ORDER[..] {
            return Err(INVALID);
        }
        let key = recover_secp256k1(&digest, &signature, recovery).map_err(|_| INVALID)?;
        let address = hash_bytes(HashAlgorithm::Keccak256, &key[1..]).map_err(|_| INVALID)?;
        let signer = address[12..].try_into().map_err(|_| INVALID)?;
        let calldata = v[to_index + 2].data;
        let hash = hash_bytes(HashAlgorithm::Keccak256, raw).map_err(|_| INVALID)?;
        Ok(Self {
            values: v,
            kind,
            signer,
            hash,
            calldata,
        })
    }
    fn verify_node_message(&self, tx: &[u8]) -> Result<()> {
        let raw = fields(tx)?;
        only_fields(&raw, 3)?;
        let body = fields(bytes_field(&raw, 1)?)?;
        let message = any(bytes_field(&body, 1)?, EVM_MESSAGE)?;
        let message = fields(&message)?;
        only_fields(&message, 2)?;
        // Derived is an ante-handler cache and must never be supplied on the wire.
        if !bytes_field(&message, 2)?.is_empty() {
            return Err(INVALID);
        }
        let name = match self.kind {
            0 => b"/paxprotocol.paxchain.eth.LegacyTx".as_slice(),
            1 => b"/paxprotocol.paxchain.eth.AccessListTx",
            _ => b"/paxprotocol.paxchain.eth.DynamicFeeTx",
        };
        let data = any(bytes_field(&message, 1)?, name)?;
        let data = fields(&data)?;
        let v = &self.values;
        let (numeric, decimal_fields, to, data_field, access, signature) = match self.kind {
            0 => (vec![(1, 0), (3, 2)], vec![(2, 1), (5, 4)], 4, 6, None, 7),
            1 => (
                vec![(2, 1), (4, 3)],
                vec![(1, 0), (3, 2), (6, 5)],
                5,
                7,
                Some((8, 7)),
                9,
            ),
            _ => (
                vec![(2, 1), (5, 4)],
                vec![(1, 0), (3, 2), (4, 3), (7, 6)],
                6,
                8,
                Some((9, 8)),
                10,
            ),
        };
        only_fields(&data, signature + 2)?;
        for (field, index) in numeric {
            if scalar_field(&data, field)? as u128 != be_usize(integer(&v[index])?)? as u128 {
                return Err(INVALID);
            }
        }
        for (field, index) in decimal_fields {
            if bytes_field(&data, field)? != decimal(integer(&v[index])?) {
                return Err(INVALID);
            }
        }
        if unhex(bytes_field(&data, to)?)? != PRECOMPILE
            || bytes_field(&data, data_field)? != self.calldata
        {
            return Err(INVALID);
        }
        for i in 0..3 {
            if bytes_field(&data, signature + i as u32)? != integer(&v[v.len() - 3 + i])? {
                return Err(INVALID);
            }
        }
        if let Some((field, index)) = access {
            let tuples = children(&v[index])?;
            let encoded = data
                .iter()
                .filter(|(n, _)| *n == field)
                .map(|(_, v)| match v {
                    ProtoValue::Bytes(b) => Ok(*b),
                    _ => Err(INVALID),
                })
                .collect::<Result<Vec<_>>>()?;
            if tuples.len() != encoded.len() {
                return Err(INVALID);
            }
            for (tuple, bytes) in tuples.iter().zip(encoded) {
                let tuple = children(tuple)?;
                if tuple.len() != 2 || tuple[0].list || tuple[0].data.len() != 20 {
                    return Err(INVALID);
                }
                let f = fields(bytes)?;
                only_fields(&f, 2)?;
                if unhex(bytes_field(&f, 1)?)? != tuple[0].data {
                    return Err(INVALID);
                }
                let keys = children(&tuple[1])?;
                let stored = f
                    .iter()
                    .filter(|(n, _)| *n == 2)
                    .map(|(_, v)| match v {
                        ProtoValue::Bytes(b) => unhex(b),
                        _ => Err(INVALID),
                    })
                    .collect::<Result<Vec<_>>>()?;
                if keys.len() != stored.len()
                    || keys
                        .iter()
                        .zip(stored)
                        .any(|(a, b)| a.list || a.data.len() != 32 || a.data != b)
                {
                    return Err(INVALID);
                }
            }
        }
        Ok(())
    }
    fn verify_result(&self, result: &TxResult) -> Result<()> {
        if result.code != 0 {
            return Err(INVALID);
        }
        let outer = fields(&result.data)?;
        only_fields(&outer, 1)?;
        let msg = fields(bytes_field(&outer, 1)?)?;
        only_fields(&msg, 2)?;
        if bytes_field(&msg, 1)? != EVM_MESSAGE {
            return Err(INVALID);
        }
        let response = fields(bytes_field(&msg, 2)?)?;
        only_fields(&response, 5)?;
        if !bytes_field(&response, 2)?.is_empty() || unhex(bytes_field(&response, 4)?)? != self.hash
        {
            return Err(INVALID);
        }
        Ok(())
    }
}

/// Native DID principal and its asset account, using the producer's domains.
pub(crate) fn funding_identity(key: [u8; 32], asset: [u8; 32]) -> Result<(PrincipalId, [u8; 32])> {
    let did = format!("did:layerx:{}", hex(&key));
    let tenant = PrincipalId::new(domain_hash(
        b"LXP/v1/did-id\0",
        &[&(did.len() as u16).to_be_bytes(), did.as_bytes()].concat(),
    )?)
    .map_err(|_| INVALID)?;
    let name = format!("agent:{did}:asset:{}", hex(&asset));
    let source = domain_hash(
        b"LX:ACCOUNT:v1",
        &[&(name.len() as u32).to_be_bytes(), name.as_bytes()].concat(),
    )?;
    Ok((tenant, source))
}
