use layerx_programs_sandbox::{sandbox_state_key, SandboxActivityRefusal};

#[test]
fn sandbox_state_key_matches_kernel_layout() {
    let patterned_lease_id = std::array::from_fn(|index| index as u8);
    for lease_id in [[0xAB; 32], [0; 32], [0xFF; 32], patterned_lease_id] {
        for kind in 0..=2 {
            let mut expected = vec![0x01, 0x00, 0x09, b's', kind];
            expected.extend_from_slice(&lease_id);
            assert_eq!(expected.len(), 37);
            assert_eq!(sandbox_state_key(kind, lease_id), Ok(expected));
        }
        for kind in 3..=u8::MAX {
            assert_eq!(
                sandbox_state_key(kind, lease_id),
                Err(SandboxActivityRefusal::NonCanonical)
            );
        }
    }
}
