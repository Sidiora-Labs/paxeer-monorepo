use serde_json::{json, Value};
use x_websearch::payment::LEDGER_PRECOMPILE;
use x_websearch::watch::{hex0x, keccak, unhex0x};

fn word(value: usize) -> [u8; 32] {
    let mut out = [0; 32];
    out[24..].copy_from_slice(&(value as u64).to_be_bytes());
    out
}

pub fn asset_record(asset_id: &[u8], symbol: &str, paused: bool) -> Vec<u8> {
    let mut record = vec![0; 12 * 32];
    record[..32].copy_from_slice(asset_id);
    record[3 * 32 + 31] = 18;
    record[10 * 32 + 31] = u8::from(paused);
    for (index, value) in [
        (1, symbol.as_bytes()),
        (2, b"Payment asset".as_slice()),
        (9, b"".as_slice()),
    ] {
        let offset = record.len();
        record[index * 32..(index + 1) * 32].copy_from_slice(&word(offset));
        record.extend_from_slice(&word(value.len()));
        record.extend_from_slice(value);
        record.resize(record.len().div_ceil(32) * 32, 0);
    }
    record
}

pub fn get_asset_answer(params: &Value) -> Option<Value> {
    if params[0]["to"] != hex0x(&LEDGER_PRECOMPILE) || params[1] != "pending" {
        return None;
    }
    let data = unhex0x(params[0]["data"].as_str()?)?;
    if data.len() != 36 || data[..4] != keccak(b"getAsset(bytes32)")[..4] {
        return None;
    }
    Some(json!(hex0x(&asset_record(&data[4..], "TEST", false))))
}
