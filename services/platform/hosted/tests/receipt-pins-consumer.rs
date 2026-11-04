use layerx_platform_gateway::ReceiptKeyList;
use std::io::{self, BufRead, Write};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let path = std::env::var("LAYERX_GATEWAY_RECEIPT_KEYS_FILE")?;
    let keys = ReceiptKeyList::from_config(&std::fs::read_to_string(path)?)?;
    println!("ready:{}", keys.entries().len());
    io::stdout().flush()?;
    for line in io::stdin().lock().lines() {
        let line = line?;
        if let Some(path) = line.strip_prefix("verify ") {
            match keys.verify_receipt(&std::fs::read(path)?) {
                Ok(receipt) => {
                    let receipt = receipt.protocol().ok_or("protocol receipt required")?;
                    println!("verified:{}", receipt.global_sequence());
                }
                Err(error) => println!("refused:{error:?}"),
            }
        } else if let Some(command) = line.strip_prefix("header ") {
            let (id, height) = command.split_once(' ').ok_or("header arguments")?;
            let bytes: Result<Vec<_>, _> = (0..id.len())
                .step_by(2)
                .map(|offset| u8::from_str_radix(&id[offset..offset + 2], 16))
                .collect();
            match keys.authorization_for_header(
                bytes?.try_into().map_err(|_| "key ID length")?,
                height.parse()?,
            ) {
                Ok(_) => println!("authorized"),
                Err(error) => println!("refused:{error:?}"),
            }
        } else {
            return Err("unknown consumer command".into());
        }
        io::stdout().flush()?;
    }
    Ok(())
}
