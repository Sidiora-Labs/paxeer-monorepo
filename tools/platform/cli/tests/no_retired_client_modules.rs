use std::fs;
use std::io;
use std::path::{Path, PathBuf};

fn rust_sources(directory: &Path, sources: &mut Vec<PathBuf>) -> io::Result<()> {
    for entry in fs::read_dir(directory)? {
        let entry = entry?;
        let path = entry.path();
        if entry.file_type()?.is_dir() {
            rust_sources(&path, sources)?;
        } else if path.extension().is_some_and(|extension| extension == "rs") {
            sources.push(path);
        }
    }
    Ok(())
}

fn root_modules(path: &str) -> Vec<&str> {
    let Some(group) = path.strip_prefix('{') else {
        return path
            .split(|c: char| !c.is_alphanumeric() && c != '_')
            .take(1)
            .collect();
    };
    let mut modules = Vec::new();
    let mut depth = 0;
    let mut start = 0;
    for (index, character) in group.char_indices() {
        match character {
            ',' | '}' if depth == 0 => {
                modules.extend(root_modules(&group[start..index]));
                start = index + 1;
                if character == '}' {
                    break;
                }
            }
            '{' => depth += 1,
            '}' => depth -= 1,
            _ => {}
        }
    }
    modules
}

#[test]
fn cli_names_no_retired_client_module() -> io::Result<()> {
    let source_root = Path::new(env!("CARGO_MANIFEST_DIR")).join("src");
    let mut sources = Vec::new();
    rust_sources(&source_root, &mut sources)?;
    sources.sort();
    assert!(!sources.is_empty(), "CLI Rust sources must be scanned");

    let mut violations = Vec::new();
    for path in sources {
        let source = fs::read_to_string(&path)?;
        let compact: String = source.chars().filter(|c| !c.is_whitespace()).collect();
        for tail in compact.split("layerx_client::").skip(1) {
            for module in root_modules(tail) {
                if ["lni", "client", "read", "receipt", "submit"].contains(&module) {
                    violations.push(format!("{} names layerx_client::{module}", path.display()));
                }
            }
        }
    }
    assert!(
        violations.is_empty(),
        "CLI sources name retired client modules:\n{}",
        violations.join("\n")
    );
    Ok(())
}
