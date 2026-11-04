use std::env;
use std::fs;
use std::path::{Component, Path, PathBuf};

pub const MANIFEST_PATH: &str = "services/platform/workspace.kvx";

const REQUIRED_DIRECTORIES: [&str; 9] = [
    "sdk-generators",
    "middleware",
    "cli",
    "emulator",
    "gateway",
    "docs",
    "examples",
    "release",
    "tools",
];

const ECOSYSTEM_KEYS: [&str; 5] = ["manifest", "lockfile", "audit", "lint", "policy"];

struct WorkspaceRoot {
    declared: PathBuf,
    canonical: PathBuf,
}

fn normalized_relative(path: &str, declaration: &str) -> Result<PathBuf, String> {
    let mut normalized = PathBuf::new();
    for component in Path::new(path).components() {
        match component {
            Component::Normal(part) => normalized.push(part),
            Component::CurDir => {}
            Component::ParentDir | Component::RootDir | Component::Prefix(_) => {
                return Err(format!(
                    "{declaration} {path} escapes the workspace root: paths must be relative without parent traversal"
                ));
            }
        }
    }
    if normalized.as_os_str().is_empty() {
        return Err(format!("{declaration} must name a nonempty relative path"));
    }
    Ok(normalized)
}

fn workspace_root(repo_root: &Path, path: &str) -> Result<WorkspaceRoot, String> {
    let declared = normalized_relative(path, "workspace root")?;
    let mut canonical = repo_root.to_path_buf();
    for component in declared.components() {
        canonical = fs::canonicalize(canonical.join(component))
            .map_err(|error| format!("workspace root {path} is not a directory: {error}"))?;
        if !canonical.starts_with(repo_root) {
            return Err(format!("workspace root {path} escapes the repository root"));
        }
    }
    if !canonical.is_dir() {
        return Err(format!("workspace root {path} is not a directory"));
    }
    Ok(WorkspaceRoot {
        declared,
        canonical,
    })
}

fn declared_path(
    roots: &[WorkspaceRoot],
    path: &str,
    declaration: &str,
    missing: &str,
) -> Result<PathBuf, String> {
    let normalized = normalized_relative(path, declaration)?;
    let root = roots
        .iter()
        .filter(|root| normalized.starts_with(&root.declared))
        .max_by_key(|root| root.declared.components().count())
        .ok_or_else(|| format!("{declaration} {path} escapes the workspace root"))?;
    let relative = normalized
        .strip_prefix(&root.declared)
        .map_err(|error| format!("{declaration}: {error}"))?;
    let mut resolved = root.canonical.clone();
    for component in relative.components() {
        resolved = fs::canonicalize(resolved.join(component))
            .map_err(|error| format!("{missing}: {path} ({declaration}): {error}"))?;
        if !resolved.starts_with(&root.canonical) {
            return Err(format!(
                "{declaration} {path} escapes the workspace root {}",
                root.declared.display()
            ));
        }
    }
    Ok(resolved)
}

/// Validates the platform workspace manifest against the repository tree.
///
/// # Errors
///
/// Fails when a declared directory or ecosystem policy file is missing, when a
/// mandated directory is undeclared, or when the manifest carries unknown
/// declarations.
pub fn workspace_gate(repo_root: &Path, source: &str) -> Result<(), String> {
    let document = layerx_platform_kvx::parse(source)?;
    let name = layerx_platform_kvx::unquote(document.required("workspace", "name")?)?;
    if name.is_empty() {
        return Err("workspace.name must not be empty".to_owned());
    }
    let repo_root =
        fs::canonicalize(repo_root).map_err(|error| format!("resolve repository root: {error}"))?;
    let root = layerx_platform_kvx::unquote(document.required("workspace", "root")?)?;
    let mut roots = vec![workspace_root(&repo_root, &root)?];
    if let Some(value) = document.get("workspace", "roots") {
        for path in layerx_platform_kvx::string_list(value)? {
            roots.push(workspace_root(&repo_root, &path)?);
        }
    }
    for (key, _) in document.section_entries("workspace") {
        if !matches!(key, "name" | "root" | "roots") {
            return Err(format!("unknown declaration workspace.{key}"));
        }
    }
    let mut directories = Vec::new();
    let mut ecosystems = Vec::new();
    for section in document.sections() {
        if section == "workspace" {
            continue;
        }
        if let Some(directory) = section.strip_prefix("dir.") {
            directories.push(directory.to_owned());
            for (key, value) in document.section_entries(section) {
                if key != "path" {
                    return Err(format!("unknown declaration {section}.{key}"));
                }
                let path = layerx_platform_kvx::unquote(value)?;
                let resolved = declared_path(
                    &roots,
                    &path,
                    &format!("{section}.path"),
                    "declared workspace directory missing",
                )?;
                if !resolved.is_dir() {
                    return Err(format!(
                        "declared workspace directory missing: {path} ({section})"
                    ));
                }
            }
            document.required(section, "path")?;
        } else if let Some(ecosystem) = section.strip_prefix("ecosystem.") {
            ecosystems.push(ecosystem.to_owned());
            for (key, value) in document.section_entries(section) {
                if !ECOSYSTEM_KEYS.contains(&key) {
                    return Err(format!("unknown declaration {section}.{key}"));
                }
                let path = layerx_platform_kvx::unquote(value)?;
                let missing = format!("declared {ecosystem} policy file missing");
                let resolved = declared_path(&roots, &path, &format!("{section}.{key}"), &missing)?;
                if !resolved.is_file() {
                    return Err(format!(
                        "declared {ecosystem} policy file missing: {path} ({section}.{key})"
                    ));
                }
            }
            for key in ECOSYSTEM_KEYS {
                document.required(section, key)?;
            }
        } else {
            return Err(format!("unknown section {section}"));
        }
    }
    for required in REQUIRED_DIRECTORIES {
        if !directories.iter().any(|declared| declared == required) {
            return Err(format!(
                "mandated workspace directory undeclared: {required}"
            ));
        }
    }
    if !ecosystems.iter().any(|declared| declared == "rust") {
        return Err("the rust ecosystem must be declared".to_owned());
    }
    Ok(())
}

/// Reads and validates the checked-in default against the supplied repository.
///
/// # Errors
///
/// Returns manifest read errors or workspace validation failures.
pub fn default_workspace_gate(repo_root: &Path) -> Result<(), String> {
    let manifest_path = repo_root.join(MANIFEST_PATH);
    let source = fs::read_to_string(&manifest_path)
        .map_err(|error| format!("read {}: {error}", manifest_path.display()))?;
    workspace_gate(repo_root, &source)
}

fn run() -> Result<(), String> {
    let arguments = env::args().skip(1).collect::<Vec<_>>();
    let repo_root = arguments
        .get(1)
        .map_or_else(|| PathBuf::from("."), PathBuf::from);
    let Some(manifest_path) = arguments.first().map(PathBuf::from) else {
        return default_workspace_gate(&repo_root);
    };
    let source = fs::read_to_string(&manifest_path)
        .map_err(|error| format!("read {}: {error}", manifest_path.display()))?;
    workspace_gate(&repo_root, &source)
}

fn main() {
    if let Err(error) = run() {
        eprintln!("platform-workspace-gate: {error}");
        std::process::exit(1);
    }
}
