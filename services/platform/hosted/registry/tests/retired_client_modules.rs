use std::{fs, io::ErrorKind, path::Path};

#[test]
fn no_retired_layerx_client_module_paths() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let mut paths = vec![root.join("src"), root.join("tests")];
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

    while let Some(path) = paths.pop() {
        if path.is_dir() {
            let entries = fs::read_dir(&path)
                .unwrap_or_else(|error| panic!("cannot scan {}: {error}", path.display()));
            for entry in entries {
                let entry = entry.unwrap_or_else(|error| {
                    panic!("cannot read an entry in {}: {error}", path.display())
                });
                paths.push(entry.path());
            }
            continue;
        }

        let source = match fs::read_to_string(&path) {
            Ok(source) => source,
            Err(error) if error.kind() == ErrorKind::NotFound => continue,
            Err(error) => panic!("cannot read {}: {error}", path.display()),
        };
        let path = path.strip_prefix(root).expect("path is inside the crate");
        let source: String = source.split_whitespace().collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            assert!(
                !source.contains(&retired_path),
                "{} still names retired path {retired_path} from wiring.J44.20",
                path.display()
            );
        }
    }
}
