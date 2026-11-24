use layerx_agentd::compat::{
    matrix, verify_published, CONTRACT_VERSION, DAEMON_VERSION, PUBLISHED_MATRIX, SDK_VERSION,
};

#[test]
fn published_matrix_embeds_the_moved_document() {
    assert!(!PUBLISHED_MATRIX.trim().is_empty());
    assert_eq!(
        PUBLISHED_MATRIX,
        include_str!("../../../../../docs/agent/COMPATIBILITY.md")
    );
}

#[test]
fn published_matrix_contains_the_current_daemon_row() {
    let row = matrix()[0];
    let expected = format!(
        "| {} | {}.{}+ | {} | {} |",
        DAEMON_VERSION,
        row.node_interface_major,
        row.minimum_node_interface_minor,
        CONTRACT_VERSION,
        SDK_VERSION
    );
    assert!(PUBLISHED_MATRIX.lines().any(|line| line.trim() == expected));
}

#[test]
fn published_matrix_passes_release_validation() {
    assert_eq!(verify_published(), Ok(()));
}
