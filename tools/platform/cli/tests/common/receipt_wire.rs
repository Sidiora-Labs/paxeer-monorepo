use layerx_wire::encode::Encoder;
use sha2::{Digest as _, Sha256};

pub(super) struct ReceiptFields<'a> {
    pub hash: &'a [u8; 32],
    pub return_data: &'a [u8],
    pub reverted: bool,
    pub body: &'a [u8],
    pub batch_id: &'a [u8; 32],
    pub from: &'a [u8; 32],
    pub to: &'a [u8; 32],
    pub nonce: u64,
    pub key_id: &'a [u8; 32],
}

fn checked<T, E: std::fmt::Debug>(value: Result<T, E>, field: &str) -> T {
    value.unwrap_or_else(|error| panic!("{field}: {error:?}"))
}

pub(super) fn encode(fields: &ReceiptFields<'_>, signature: Option<[u8; 64]>) -> Vec<u8> {
    let digest = |bytes: &[u8]| -> [u8; 32] { Sha256::digest(bytes).into() };
    let mut e = Encoder::new(4096);
    checked(e.structure_header_version(0x5201, 2), "header");
    checked(e.u16(2), "protocol");
    checked(e.bytes(fields.hash, 32), "hash");
    checked(e.u64(10), "sequence");
    checked(e.bytes(&[0x21; 32], 32), "previous root");
    checked(e.bytes(&[0x22; 32], 32), "resulting root");
    checked(e.bytes(&digest(fields.return_data), 32), "return digest");
    checked(e.i32(i32::from(fields.reverted)), "result");
    checked(
        e.sequence_length(1, layerx_wire::limits::MAX_SEQUENCER_EFFECTS),
        "effects",
    );
    checked(e.u16(12), "module");
    checked(e.u16(0), "ordinal");
    checked(e.u16(0), "event");
    checked(e.tag(1, 3), "kind");
    checked(e.u8(0), "monetary");
    checked(e.bytes(&[0; 32], 32), "transfer root");
    checked(e.bytes(fields.body, 256), "summary");
    checked(e.u128(42_000), "fee");
    checked(e.bytes(fields.batch_id, 32), "batch id");
    checked(e.u16(12), "module");
    checked(e.u32(1), "module version");
    checked(e.u32(0), "parameter version");
    checked(e.u8(2), "operation");
    checked(e.bytes(&digest(b"uhpx"), 32), "denom");
    checked(e.u128(0), "amount");
    checked(e.bytes(fields.from, 32), "from");
    checked(e.u128(0), "balance before");
    checked(e.u128(0), "balance after");
    checked(e.u64(fields.nonce), "nonce");
    checked(e.bytes(fields.to, 32), "to");
    checked(e.u128(0), "credit before");
    checked(e.u128(0), "credit after");
    checked(e.bytes(&[0; 32], 32), "transfer root");
    checked(e.bytes(&[0; 32], 32), "authorization");
    checked(e.bytes(fields.key_id, 32), "key id");
    checked(e.u64(1000), "timestamp");
    checked(e.u8(u8::from(signature.is_some())), "signature flag");
    if let Some(signature) = signature {
        checked(e.bytes(&signature, 64), "signature");
    }
    e.finish()
}
