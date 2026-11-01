use layerx_programs::chain_evidence::programs_state_key;

#[test]
fn chain_evidence_program_key_uses_module_kv_layout() {
    let _advance = layerx_programs::ChainProofVerifier::advance;

    let program_id: Vec<u8> = (0..32).collect();
    for prefix in [b"program\0".as_slice(), b"wind-down\0s".as_slice()] {
        let mut kernel_key = prefix.to_vec();
        kernel_key.extend_from_slice(&program_id);
        let mut expected = vec![0x01, 0x00, 0x09];
        expected.extend_from_slice(&kernel_key);

        let actual = programs_state_key(&kernel_key);
        assert_eq!(actual, expected);
        assert_eq!(actual.len(), 3 + kernel_key.len());
        assert_eq!(&actual[3..], kernel_key.as_slice());
    }

    assert_eq!(programs_state_key(&[]), vec![0x01, 0x00, 0x09]);
    assert_eq!(
        programs_state_key(&[0x00, 0xff, 0x80, 0x01]),
        vec![0x01, 0x00, 0x09, 0x00, 0xff, 0x80, 0x01]
    );
}
