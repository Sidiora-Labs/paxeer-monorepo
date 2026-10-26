use std::fs;
use std::path::Path;
use std::process::Command;

use layerx_platform_kvx::{parse, string_list, unquote, Document};

fn finality_source(document: &Document) -> Result<(), String> {
    assert_eq!(
        string_list(document.required("binding", "finality_source")?)?,
        ["receipted", "committed"]
    );
    for section in document.sections() {
        for (key, value) in document.section_entries(section) {
            if matches!(key, "finality" | "finality_source") {
                let rungs = if value.starts_with('[') {
                    string_list(value)?
                } else {
                    vec![unquote(value)?]
                };
                assert!(
                    !rungs.is_empty(),
                    "{section}.{key} must name a receipt rung"
                );
                for rung in rungs {
                    assert!(
                        matches!(rung.as_str(), "receipted" | "committed"),
                        "{section}.{key} names an unsupported finality rung: {rung}"
                    );
                }
            }
        }
    }
    Ok(())
}

fn recovery(document: &Document) -> Result<(), String> {
    assert_eq!(
        unquote(document.required("recovery", "finality")?)?,
        "committed"
    );
    assert_eq!(
        unquote(document.required("recovery", "committed_height")?)?,
        "landing-height"
    );
    Ok(())
}

fn approval(document: &Document) -> Result<(), String> {
    for key in ["terms", "mismatch_fields"] {
        let fields = string_list(document.required("approval", key)?)?;
        for field in ["nonce", "gas"] {
            assert!(
                fields.iter().any(|value| value == field),
                "approval.{key} must include the EVM transaction {field}"
            );
        }
    }
    Ok(())
}

fn native_call(document: &Document) -> Result<(), String> {
    assert_eq!(unquote(document.required("native_call", "to")?)?, "0x101D");
    assert_eq!(
        unquote(document.required("native_call", "selector")?)?,
        "0x54bd2b48"
    );
    assert_eq!(document.required("native_call", "chainId")?, "125");
    for retired in [
        "protocol_version",
        "module",
        "module_id",
        "activity_ordinal",
    ] {
        assert!(
            document.get("native_call", retired).is_none(),
            "native_call must not include kernel parameter {retired}"
        );
    }
    Ok(())
}

#[test]
fn paxai_schema_names_only_receipt_rungs() -> Result<(), String> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let output = Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
        .arg("--check-paxai")
        .arg(&root)
        .output()
        .map_err(|error| error.to_string())?;
    assert!(
        output.status.success(),
        "--check-paxai failed ({}):\n{}\n{}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );

    let source = fs::read_to_string(root.join("sdk/platform/schema/paxai-v1.kvx"))
        .map_err(|error| error.to_string())?;
    assert!(
        !source.contains("checkpoint"),
        "checkpoint finality is retired"
    );
    let document = parse(&source)?;
    finality_source(&document)?;
    recovery(&document)
}

#[test]
fn paxai_native_call_targets_layerxprograms() -> Result<(), String> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let source = fs::read_to_string(root.join("sdk/platform/schema/paxai-v1.kvx"))
        .map_err(|error| error.to_string())?;
    let document = parse(&source)?;
    native_call(&document)?;
    approval(&document)
}
