use std::path::Path;
use std::process::{Command, Output};

fn human_retired_surface_gate(root: &Path) -> std::io::Result<Output> {
    Command::new(env!("CARGO_BIN_EXE_layerx-human-boundary-check"))
        .arg(root)
        .output()
}

#[test]
fn boundary_check_binary_passes_the_merged_human_tree() -> std::io::Result<()> {
    let crates = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../crates")
        .canonicalize()?;
    let output = human_retired_surface_gate(&crates)?;
    assert!(
        output.status.success(),
        "boundary check failed with {}\nstdout: {}\nstderr: {}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(
        String::from_utf8_lossy(&output.stdout),
        "human dependency, boundary, payload-authority and retired-surface policies passed\n"
    );
    Ok(())
}
