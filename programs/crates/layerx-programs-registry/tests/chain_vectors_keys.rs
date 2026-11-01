use layerx_programs::chain_evidence::programs_state_key;
use sha2::{Digest, Sha256};

fn vector_field<'a>(record: &'a str, name: &str) -> &'a str {
    record
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(key, _)| key.trim().trim_matches('"') == name)
        .unwrap_or_else(|| panic!("host store key vector is missing {name}"))
        .1
        .trim()
        .trim_end_matches(',')
}

fn vector_string<'a>(record: &'a str, name: &str) -> &'a str {
    vector_field(record, name)
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
        .unwrap_or_else(|| panic!("host store key vector {name} must be a string"))
}

fn decode_hex(value: &str) -> Vec<u8> {
    assert!(value.is_ascii(), "host store key hex must be ASCII");
    assert_eq!(value.len() % 2, 0, "host store key hex must have pairs");
    (0..value.len())
        .step_by(2)
        .map(|offset| {
            u8::from_str_radix(&value[offset..offset + 2], 16)
                .unwrap_or_else(|error| panic!("invalid host store key hex: {error}"))
        })
        .collect()
}

#[test]
fn chain_programs_keys_match_host_vector() {
    let vector = include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../modules/lxphost/testdata/store_keys_v1.json"
    ));
    assert_eq!(
        Sha256::digest(vector.as_bytes()).as_slice(),
        decode_hex("0c5b1d229bab34176cb2c056bf68a3bc3a0e42034f34cc7f5e6f373799daefe0"),
        "host store key vector SHA-256 changed"
    );

    let module_keys = vector
        .split_once("\"module_keys\": [")
        .and_then(|(_, tail)| tail.split_once(']'))
        .unwrap_or_else(|| panic!("host store key vector is missing its module_keys array"))
        .0;
    let mut checked = 0;
    for object in module_keys.split('{').skip(1) {
        let record = object
            .split_once('}')
            .unwrap_or_else(|| panic!("unterminated host store key vector record"))
            .0;
        let name = vector_string(record, "name");
        let module = vector_field(record, "module")
            .parse::<u16>()
            .unwrap_or_else(|error| panic!("invalid module for {name}: {error}"));
        assert_eq!(module, 9, "unexpected module for {name}");
        let kernel_key = decode_hex(vector_string(record, "kernel_key_hex"));
        let store_key = decode_hex(vector_string(record, "store_key_hex"));
        assert_eq!(
            programs_state_key(&kernel_key),
            store_key,
            "programs state key differs from host moduleKVKey for {name}"
        );
        checked += 1;
    }
    assert_eq!(checked, 10, "must check every pinned module 9 key");
}
