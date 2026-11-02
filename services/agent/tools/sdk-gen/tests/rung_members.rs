#[allow(dead_code)]
#[path = "../src/main.rs"]
mod generator;

use std::fs;
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

fn declaration<'a>(source: &'a str, start: &str, end: &str) -> &'a str {
    source
        .split_once(start)
        .and_then(|(_, rest)| rest.split_once(end))
        .map(|(body, _)| body)
        .unwrap_or_else(|| panic!("missing declaration between {start:?} and {end:?}"))
}

#[test]
fn generated_rung_members_are_the_schema_variants() {
    let schema = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/agent-api");
    let errors = fs::read_to_string(schema.join("errors.kvx"))
        .unwrap_or_else(|error| panic!("read canonical errors schema: {error}"));
    let rung_schema = declaration(&errors, "[type.Rung]", "\n[");
    let variants = rung_schema
        .lines()
        .filter_map(|line| line.split_once('='))
        .find_map(|(key, value)| (key.trim() == "variants").then_some(value.trim()))
        .and_then(|value| value.strip_prefix('['))
        .and_then(|value| value.strip_suffix(']'))
        .unwrap_or_else(|| panic!("missing type.Rung.variants list"))
        .split(',')
        .map(|variant| {
            variant
                .trim()
                .strip_prefix('"')
                .and_then(|value| value.strip_suffix('"'))
                .unwrap_or_else(|| panic!("invalid quoted Rung variant: {variant}"))
        })
        .collect::<Vec<_>>();
    assert_eq!(variants, ["Submitted", "Receipted", "Committed"]);

    let generated = generator::agent_sdk_generator(&schema)
        .unwrap_or_else(|error| panic!("generate SDK from canonical schema: {error}"));
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("temporary output timestamp: {error}"))
        .as_nanos();
    let root = std::env::temp_dir().join(format!(
        "agent-sdk-rung-members-{}-{nonce}",
        std::process::id()
    ));
    fs::create_dir(&root).unwrap_or_else(|error| panic!("create temporary SDK directory: {error}"));
    for (relative, source) in &generated.files {
        let path = root.join(relative);
        fs::create_dir_all(
            path.parent()
                .unwrap_or_else(|| panic!("SDK parent missing")),
        )
        .unwrap_or_else(|error| panic!("create generated SDK parent: {error}"));
        fs::write(&path, source)
            .unwrap_or_else(|error| panic!("write generated {}: {error}", path.display()));
    }

    let typescript = fs::read_to_string(root.join("typescript/src/generated/client.ts"))
        .unwrap_or_else(|error| panic!("read generated TypeScript client: {error}"));
    let rungs = declaration(&typescript, "export enum Rung {", "}");
    let expected = variants
        .iter()
        .enumerate()
        .map(|(rank, variant)| format!("{variant} = {rank},"))
        .collect::<Vec<_>>();
    assert_eq!(
        rungs
            .lines()
            .map(str::trim)
            .filter(|line| !line.is_empty())
            .collect::<Vec<_>>(),
        expected,
        "TypeScript Rung members must preserve schema spelling and declaration ranks"
    );
    for lowercase in ["submitted", "receipted", "committed"] {
        assert!(
            !rungs.contains(lowercase),
            "TypeScript Rung must not contain lowercase member {lowercase}"
        );
    }

    for relative in [
        "python/layerx_sdk/generated/client.py",
        "python/layerx_sdk/generated/client.pyi",
    ] {
        let python = fs::read_to_string(root.join(relative))
            .unwrap_or_else(|error| panic!("read generated {relative}: {error}"));
        let rungs = declaration(&python, "class Rung(IntEnum):", "\n\n");
        assert_eq!(
            rungs
                .lines()
                .map(str::trim)
                .filter(|line| !line.is_empty())
                .collect::<Vec<_>>(),
            ["SUBMITTED = 0", "RECEIPTED = 1", "COMMITTED = 2"],
            "{relative}: Python Rung member names and declaration ranks"
        );
    }

    fs::remove_dir_all(&root)
        .unwrap_or_else(|error| panic!("remove temporary SDK directory: {error}"));
}
