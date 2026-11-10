// Fixture recorder: compile the production Rust verifier directly, without a copy.
#[path = "../../services/agent/crates/layerx-proof/src/state_witness.rs"]
mod state_witness;

use serde_json::{json, Value};
use state_witness::{StateProofError, StateWitness};
use std::io::{self, Read};

fn unhex(value: &Value) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let encoded = value.as_str().ok_or("missing hex string")?;
    if encoded.len() % 2 != 0 {
        return Err("odd hex length".into());
    }
    (0..encoded.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&encoded[i..i + 2], 16).map_err(Into::into))
        .collect()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut input = String::new();
    io::stdin().read_to_string(&mut input)?;
    let mut cases: Vec<Value> = serde_json::from_str(&input)?;
    // Preserve the original eleven-leaf witness as a positive regression. The
    // named refusal continues to exercise the first unsupported width, 13.
    if !cases
        .iter()
        .any(|case| case["name"] == "valid-module-tree-width-11")
    {
        let original = cases
            .iter()
            .find(|case| case["name"] == "module-tree-width")
            .ok_or("missing module-tree-width vector")?;
        let wire = unhex(&original["witness"])?;
        let witness = StateWitness::decode(&wire)?;
        assert_eq!(witness.leaf_count_b, 11);
        assert_eq!(witness.encode()?, wire);
        witness.verify(
            unhex(&original["state_root"])?
                .try_into()
                .map_err(|_| "root width")?,
        )?;
        let mut positive = original.clone();
        positive["name"] = json!("valid-module-tree-width-11");
        positive["module_id"] = json!(witness.module_id.to_string());
        positive["key"] = json!(hex(&witness.key));
        positive["value"] = json!(hex(&witness.value));
        positive["valid"] = json!(true);
        positive
            .as_object_mut()
            .ok_or("vector object")?
            .remove("refusal");
        cases.push(positive);
        let negative = cases
            .iter_mut()
            .find(|case| case["name"] == "module-tree-width")
            .ok_or("missing width refusal")?;
        let offset = wire.len() - 5 - witness.siblings_b.len() * 32;
        let mut malformed = wire;
        malformed[offset..offset + 4].copy_from_slice(&13_u32.to_be_bytes());
        assert_eq!(
            StateWitness::decode(&malformed),
            Err(StateProofError::Module)
        );
        negative["witness"] = json!(hex(&malformed));
    }
    for case in &mut cases {
        let wire = unhex(&case["witness"])?;
        let root = unhex(&case["state_root"])?
            .try_into()
            .map_err(|_| "root width")?;
        let result = StateWitness::decode(&wire).and_then(|witness| witness.verify(root));
        // Unrelated historical expectations must remain unchanged.
        assert_eq!(case["valid"], json!(result.is_ok()), "{}", case["name"]);
        if let Err(error) = &result {
            assert_eq!(
                case["refusal"],
                json!(format!("{error:?}")),
                "{}",
                case["name"]
            );
        }
        case["valid"] = json!(result.is_ok());
        if let Err(error) = result {
            case["refusal"] = json!(format!("{error:?}"));
        }
    }
    println!("{}", serde_json::to_string(&cases)?);
    Ok(())
}
