use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::path::{Path, PathBuf};

use serde_json::json;

use crate::config::ensure_directory_empty;

pub fn create(name: &str, parent: &Path) -> Result<serde_json::Value, String> {
    let sdk = std::env::var_os("LAYERX_PROGRAM_SDK").ok_or(
        "configure LAYERX_PROGRAM_SDK with an available local layerx-program-sdk checkout",
    )?;
    create_with_sdk(name, parent, Path::new(&sdk))
}

/// Create a project against an explicitly selected, locally available SDK.
pub fn create_with_sdk(name: &str, parent: &Path, sdk: &Path) -> Result<serde_json::Value, String> {
    validate_name(name)?;
    let sdk = validated_sdk(sdk)?;
    let project = parent.join(name);
    ensure_directory_empty(&project)?;
    fs::create_dir_all(project.join("src"))
        .map_err(|error| format!("could not create {}: {error}", project.display()))?;
    let crate_name = name.replace('-', "_");
    write_new(
        &project.join("Cargo.toml"),
        &format!(
            "[package]\nname = \"{name}\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[lib]\ncrate-type = [\"cdylib\"]\n\n[dependencies]\nlayerx-program-sdk = {{ path = {sdk} }}\n\n[profile.release]\nopt-level = \"z\"\nlto = true\ncodegen-units = 1\npanic = \"abort\"\nstrip = true\n",
            sdk = toml_edit::Value::from(sdk.to_str().ok_or("SDK path must be valid UTF-8")?)
        ),
    )?;
    write_new(
        &project.join("src/lib.rs"),
        "#![no_std]\n\nuse layerx_program_sdk::{program, trap_on_panic, ProgramError};\n\ntrap_on_panic!();\n\nprogram!(handle);\n\nfn handle(value: i64) -> Result<i64, ProgramError> {\n    Ok(value)\n}\n",
    )?;
    write_new(
        &project.join("LayerX.toml"),
        "schema = 1\nkind = \"program\"\nabi = 1\nenvironment = \"emulator\"\n",
    )?;
    write_new(&project.join(".gitignore"), "/target\n")?;
    Ok(json!({
        "path": absolute(&project)?.display().to_string(),
        "package": name,
        "crate": crate_name,
        "kind": "rust-program",
        "files": ["Cargo.toml", "LayerX.toml", "src/lib.rs", ".gitignore"],
    }))
}

fn write_new(path: &Path, contents: &str) -> Result<(), String> {
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(path)
        .map_err(|error| format!("could not create {}: {error}", path.display()))?;
    file.write_all(contents.as_bytes())
        .and_then(|()| file.sync_all())
        .map_err(|error| format!("could not write {}: {error}", path.display()))
}

fn validate_name(name: &str) -> Result<(), String> {
    if name.is_empty()
        || name.len() > 64
        || name.starts_with('-')
        || !name
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
    {
        return Err("project name must be a lowercase Cargo package name".into());
    }
    Ok(())
}

fn validated_sdk(path: &Path) -> Result<PathBuf, String> {
    let sdk = absolute(path)?;
    sdk.to_str().ok_or("SDK path must be valid UTF-8")?;
    let manifest = sdk.join("Cargo.toml");
    let source = fs::read_to_string(&manifest).map_err(|error| {
        format!(
            "SDK manifest {} is unavailable: {error}",
            manifest.display()
        )
    })?;
    let document = source
        .parse::<toml_edit::DocumentMut>()
        .map_err(|error| format!("invalid SDK manifest: {error}"))?;
    if document
        .get("package")
        .and_then(|package| package.get("name"))
        .and_then(toml_edit::Item::as_str)
        != Some("layerx-program-sdk")
    {
        return Err("local SDK must be the layerx-program-sdk package".into());
    }
    let lib = document
        .get("lib")
        .and_then(|lib| lib.get("path"))
        .and_then(toml_edit::Item::as_str)
        .unwrap_or("src/lib.rs");
    if !sdk.join(lib).is_file() {
        return Err("local SDK library source is unavailable".into());
    }
    Ok(sdk)
}

fn absolute(path: &Path) -> Result<PathBuf, String> {
    path.canonicalize()
        .map_err(|error| format!("could not resolve {}: {error}", path.display()))
}
