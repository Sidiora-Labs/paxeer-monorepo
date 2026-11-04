use layerx_programs_runtime::admit_abi_version;

pub fn supported_abi_versions() -> Vec<u16> {
    (0..=u16::MAX)
        .filter(|version| admit_abi_version(*version).is_ok())
        .collect()
}
