#[test]
fn no_retired_layerx_client_module_paths() {
    let manifest_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
    let mut directories = vec![manifest_dir.join("src")];
    let mut files = Vec::new();
    while let Some(directory) = directories.pop() {
        for entry in std::fs::read_dir(&directory).expect("read indexer source directory") {
            let entry = entry.expect("read indexer source entry");
            let path = entry.path();
            if path.is_dir() {
                directories.push(path);
            } else if path.extension().is_some_and(|extension| extension == "rs") {
                let source = std::fs::read_to_string(&path).expect("read indexer source file");
                let relative_path = path
                    .strip_prefix(manifest_dir)
                    .expect("indexer source path");
                files.push((relative_path.display().to_string(), source));
            }
        }
    }
    files.sort_by(|left, right| left.0.cmp(&right.0));
    assert!(!files.is_empty(), "no indexer Rust source files found");
    let retired_modules = [
        "lni",
        "client",
        "head",
        "resolver",
        "handover",
        "handover_policy",
        "availability",
        "caps",
        "execution_prestate",
        "arbiter_admission",
        "arbiter_prestate",
        "read",
        "receipt",
        "stream",
        "submit",
        "batch",
        "payments",
        "withdrawal",
        "evidence",
    ];
    let mut violations = Vec::new();
    for (path, source) in files {
        let source: String = source.chars().filter(|c| !c.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            if source.match_indices(&retired_path).any(|(offset, _)| {
                source[offset + retired_path.len()..]
                    .chars()
                    .next()
                    .is_none_or(|c| !c.is_ascii_alphanumeric() && c != '_')
            }) {
                violations.push(format!("{path}: {retired_path}"));
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.20 paths remain:\n{}",
        violations.join("\n")
    );
}
