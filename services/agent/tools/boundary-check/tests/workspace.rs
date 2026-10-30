use std::fs;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};

fn agent_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .expect("locate the agent tree")
}

#[test]
fn agent_tree_has_no_lni_artifacts() {
    let root = agent_root();
    assert!(root.join("crates").is_dir(), "expected the real agent tree");
    for artifact in [
        "include/layerx_lni_abi.h",
        "schema/lni",
        "stable-abi-allowlist.toml",
    ] {
        let path = root.join(artifact);
        let error = fs::symlink_metadata(&path).expect_err(&format!(
            "retired artifact must not exist: {}",
            path.display()
        ));
        assert_eq!(
            error.kind(),
            ErrorKind::NotFound,
            "cannot inspect {}: {error}",
            path.display()
        );
    }
}

#[test]
fn unsafe_allowlist_holds_no_exception() {
    let body = fs::read_to_string(agent_root().join("unsafe-allowlist.toml"))
        .expect("read the agent unsafe allowlist");
    let entries: Vec<_> = body
        .lines()
        .map(|line| {
            line.split('#')
                .next()
                .expect("line has a first segment")
                .trim()
        })
        .filter(|line| !line.is_empty())
        .collect();
    assert_eq!(
        entries.len(),
        1,
        "only the version entry may remain: {entries:?}"
    );
    let (key, value) = entries[0].split_once('=').expect("version assignment");
    assert_eq!(key.trim(), "version");
    assert_eq!(value.trim(), "1");
}
