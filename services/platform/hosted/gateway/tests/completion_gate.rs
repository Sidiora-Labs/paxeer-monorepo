use std::collections::{BTreeMap, BTreeSet};
use std::error::Error;
use std::fs;
use std::io;
use std::path::Path;

use serde_json::Value;

type Sections = BTreeMap<String, BTreeMap<String, String>>;

const WAVE_SIX_TASKS: [&str; 55] = [
    "8.3.1", "8.7.3", "8.7.4", "8.8.4", "8.9.2", "8.11.3", "8.12.3", "8.13.2", "8.14.2", "8.14.4", "8.16.7", "8.18.1", "8.19.1", "8.19.2", "8.19.3", "8.19.4", "8.19.5", "8.19.6", "8.19.7", "8.19.11", "8.20.2", "8.20.4", "8.22.6", "8.26.2", "8.30.3", "8.31.2", "8.32.1", "8.32.2", "8.32.3", "8.32.4",
    "8.41.5", "8.41.6", "8.41.8", "8.42.5", "8.42.6", "8.42.8", "8.43.5", "8.43.6", "8.43.8", "8.44.5", "8.44.6", "8.44.8", "8.45.3", "8.45.4", "8.46.2", "8.46.7", "8.46.8", "8.47.4", "8.47.5", "8.47.6", "8.47.7", "8.48.3", "8.49.2", "8.50.4", "8.51.1",
];

fn sections(source: &str, prefix: &str) -> Sections {
    let mut sections = Sections::new();
    let mut current = None;
    for line in source.lines().map(str::trim) {
        if let Some(name) = line.strip_prefix('[').and_then(|s| s.strip_suffix(']')) {
            current = name.starts_with(prefix).then(|| name.to_owned());
            if let Some(name) = &current {
                assert!(sections.insert(name.clone(), BTreeMap::new()).is_none());
            }
        } else if let (Some(name), Some((key, value))) = (&current, line.split_once('=')) {
            if matches!(key.trim(), "interfaces" | "planned_paths" | "kind" | "path" | "signature" | "status") {
                let fields = sections.entry(name.clone()).or_default();
                assert!(fields.insert(key.trim().to_owned(), value.trim().to_owned()).is_none());
            }
        }
    }
    sections
}

fn field<'a>(section: &'a BTreeMap<String, String>, key: &str) -> &'a str {
    section.get(key).unwrap_or_else(|| panic!("inventory record lacks {key}"))
}

fn quoted(value: &str) -> String {
    value.strip_prefix('"').and_then(|s| s.strip_suffix('"')).unwrap_or_else(|| panic!("invalid KVX string: {value}")).replace("\\\"", "\"")
}

fn compact(value: &str) -> String {
    value.chars().filter(|c| !c.is_whitespace()).collect()
}

fn java_enum_variant_present(id: &str, source: &str) -> Result<Option<bool>, Box<dyn Error>> {
    for (prefix, owner) in [
        ("interface.8.go-sdkjvm-generatedschema-humanmodels-bindingstate-", "BindingState"),
        ("interface.8.go-sdkjvm-generatedschema-humanmodels-errorcode-", "ErrorCode"),
        ("interface.8.go-sdkjvm-generatedschema-humanmodels-journeykind-", "JourneyKind"),
        ("interface.8.go-sdkjvm-generatedschema-humanmodels-verificationlevel-", "VerificationLevel"),
        ("interface.8.go-sdkjvm-schemaerrors-humancode-", "HumanCode"),
        ("interface.8.go-sdkjvm-localverifier-verificationlevel-", "VerificationLevel"),
    ] {
        if let Some(member) = id.strip_prefix(prefix) {
            let declaration = format!("enum {owner}");
            let Some((_, declaration_tail)) = source.split_once(&declaration) else {
                return Ok(Some(false));
            };
            let (_, body) = declaration_tail.split_once('{').ok_or("missing Java enum body")?;
            let mut depth = 1;
            let end = body
                .char_indices()
                .find_map(|(index, character)| {
                    match character {
                        '{' => depth += 1,
                        '}' => depth -= 1,
                        _ => {}
                    }
                    (depth == 0).then_some(index)
                })
                .ok_or("unterminated Java enum body")?;
            let variant = member.replace('-', "_").to_ascii_uppercase();
            return Ok(Some(compact(&body[..end]).split([',', ';', '{', '}']).any(|entry| entry.starts_with(&format!("{variant}(")))));
        }
    }

    Ok(None)
}

fn signature_present(id: &str, kind: &str, signature: &str, path: &Path, source: &str) -> Result<bool, Box<dyn Error>> {
    let source = if path.extension().is_some_and(|extension| extension == "rs") {
        source.split_once("#[cfg(test)]").map_or(source, |(served, _)| served)
    } else {
        source
    };
    if let Some(present) = java_enum_variant_present(id, source)? {
        return Ok(present);
    }

    if kind == "file_format" && signature.starts_with("human ") {
        let words: Vec<_> = signature.split_whitespace().collect();
        let catalogue: Value = serde_json::from_str(source)?;
        let groups = catalogue["operations"].as_object().ok_or("operation catalogue lacks operations")?;
        for group in groups.values() {
            for operation in group.as_array().ok_or("invalid operation group")? {
                if operation["plane"] == words[0] && operation["name"] == words[1] {
                    return Ok(true);
                }
            }
        }
        return Ok(false);
    }

    if kind == "config_key" && signature.starts_with('[') {
        let (section, rest) = signature.split_once(']').ok_or("invalid config signature")?;
        if rest.contains('=') {
            let key = rest.split_once('=').ok_or("missing config key")?.0.trim();
            let mut in_section = false;
            for line in source.lines().map(str::trim) {
                if line.starts_with('[') {
                    in_section = line == format!("{section}]");
                } else if in_section && line.split_once('=').is_some_and(|(k, _)| k.trim() == key) {
                    return Ok(true);
                }
            }
            return Ok(false);
        }
        return Ok(source.lines().any(|line| line.trim() == format!("{section}]")));
    }

    let signature = ["java: ", "csharp: ", "swift: ", "kotlin: "].into_iter().find_map(|prefix| signature.strip_prefix(prefix)).unwrap_or(signature);
    let signature = if kind == "go_exported" {
        let signature = signature.strip_prefix("const ").unwrap_or(signature);
        if signature.contains(" init(")
            || !signature
                .split_whitespace()
                .next()
                .is_some_and(|word| matches!(word, "public" | "private" | "internal" | "default" | "func" | "type" | "var" | "final" | "record" | "enum" | "Task<JsonValue>" | "ValueTask<bool>"))
        {
            signature
        } else {
            signature.split(['(', '{']).next().ok_or("empty declaration")?
        }
    } else if matches!(kind, "rust_pub_item" | "ts_export") {
        signature.split(['(', '{']).next().ok_or("empty declaration")?
    } else {
        signature
    };
    let signature = compact(signature);
    assert!(!signature.is_empty(), "empty retirement signature");
    Ok(compact(source).contains(&signature))
}

#[test]
fn platform_completion_retired_surfaces_absent() -> Result<(), Box<dyn Error>> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).ancestors().nth(4).ok_or("gateway manifest is outside the repository layout")?;
    let tasks = sections(&fs::read_to_string(root.join("spec/layerx-merge/spec.kvx"))?, "task.8.");
    let inventory = sections(&fs::read_to_string(root.join("spec/layerx-merge/interfaces.kvx"))?, "interface.8.");
    let mut interface_ids = BTreeSet::new();
    let mut planned_paths = BTreeSet::new();
    for id in WAVE_SIX_TASKS {
        let name = format!("task.{id}");
        let task = tasks.get(&name).unwrap_or_else(|| panic!("missing {name}"));
        interface_ids.extend(serde_json::from_str::<Vec<String>>(field(task, "interfaces"))?);
        planned_paths.extend(serde_json::from_str::<Vec<String>>(field(task, "planned_paths"))?);
    }
    assert!(planned_paths.len() >= 57, "incomplete planned-path inventory");

    let mut failures = Vec::new();
    let mut sources = BTreeMap::new();
    let mut retired = 0;
    for id in interface_ids {
        let interface = inventory.get(&id).unwrap_or_else(|| panic!("missing interface record {id}"));
        if !matches!(quoted(field(interface, "status")).as_str(), "retire" | "retired") {
            continue;
        }
        retired += 1;
        let relative = quoted(field(interface, "path"));
        let path = root.join(&relative);
        match fs::symlink_metadata(&path) {
            Err(error) if error.kind() == io::ErrorKind::NotFound => continue,
            Err(error) => return Err(format!("cannot inspect {relative}: {error}").into()),
            Ok(_) => {}
        }
        let kind = quoted(field(interface, "kind"));
        let signature = quoted(field(interface, "signature"));
        let present = if signature.starts_with("JSON vector ") {
            true
        } else {
            if !sources.contains_key(&relative) {
                sources.insert(relative.clone(), fs::read_to_string(&path)?);
            }
            signature_present(&id, &kind, &signature, &path, &sources[&relative])?
        };
        if present {
            failures.push(format!("retired surface {id} remains at {relative}"));
        }
    }
    assert!(retired >= 429, "incomplete retired-interface inventory: {retired}");
    for relative in planned_paths {
        if !root.join(&relative).is_file() {
            failures.push(format!("missing planned path: {relative}"));
        }
    }
    assert!(failures.is_empty(), "domain 8 completion blocked:\n{}", failures.join("\n"));
    Ok(())
}
