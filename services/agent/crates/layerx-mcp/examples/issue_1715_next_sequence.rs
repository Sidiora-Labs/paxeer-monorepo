use std::error::Error;
use std::io::{self, Write};

use layerx_mcp::tools::web::GrantTerms;

fn issue_1715_next_sequence_call(
    receiver_did_key: [u8; 32],
    recipient: [u8; 32],
    asset: [u8; 32],
) -> Result<([u8; 20], Vec<u8>), Box<dyn Error>> {
    let terms = GrantTerms {
        payer_did: String::new(),
        payer_account: [0; 32],
        recipient,
        receiver_did_key,
        asset,
        expires_at_ms: 0,
        amount: 0,
        purpose_hash: [0; 32],
        idempotency_key: [0; 32],
    };
    let call = terms.next_sequence_call();
    Ok((
        call.target.address(),
        call.encode().map_err(|error| format!("{error:?}"))?,
    ))
}

fn word(text: &str) -> Result<[u8; 32], Box<dyn Error>> {
    if text.len() != 64 || !text.is_ascii() {
        return Err("expected a 32-byte hex identifier".into());
    }
    let mut result = [0; 32];
    for (index, byte) in result.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)?;
    }
    Ok(result)
}

fn main() -> Result<(), Box<dyn Error>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let [did, recipient, asset] = args.as_slice() else {
        return Err("usage: issue_1715_next_sequence DID_KEY ACCOUNT_ID ASSET_ID".into());
    };
    let (target, data) = issue_1715_next_sequence_call(word(did)?, word(recipient)?, word(asset)?)?;
    let mut output = io::stdout().lock();
    output.write_all(&target)?;
    output.write_all(&data)?;
    Ok(())
}
