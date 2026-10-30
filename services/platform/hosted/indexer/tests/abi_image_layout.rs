use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use layerx_indexer::abi::{keccak, precompile_address, AbiRegistry, NESTED_PRECOMPILES};

struct Scratch(PathBuf);

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn image_abi_layout_binds_perps_and_pricefeed() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let precompiles = root.join("precompiles");
    let script = root.join("docker/platform-indexer/package-abis.sh");
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("system time: {error}"))
        .as_nanos();
    let scratch = Scratch(std::env::temp_dir().join(format!(
        "layerx-indexer-image-abis-{}-{nonce}",
        std::process::id()
    )));
    fs::create_dir(&scratch.0).unwrap_or_else(|error| panic!("scratch directory: {error}"));
    let packaged = scratch.0.join("image abis");
    let output = Command::new("sh")
        .arg(&script)
        .arg(&precompiles)
        .arg(&packaged)
        .output()
        .unwrap_or_else(|error| panic!("package-abis.sh: {error}"));
    assert!(
        output.status.success(),
        "package-abis.sh failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    for entry in
        fs::read_dir(&precompiles).unwrap_or_else(|error| panic!("precompiles directory: {error}"))
    {
        let entry = entry.unwrap_or_else(|error| panic!("precompile entry: {error}"));
        let source = entry.path().join("abi.json");
        if source.is_file() {
            let destination = packaged.join(entry.file_name()).join("abi.json");
            assert_eq!(
                fs::read(&source).unwrap_or_else(|error| panic!("{}: {error}", source.display())),
                fs::read(&destination)
                    .unwrap_or_else(|error| panic!("{}: {error}", destination.display()))
            );
        }
    }

    let registry = AbiRegistry::load_dir(&packaged)
        .unwrap_or_else(|error| panic!("packaged ABI registry: {error}"));
    for (contract, relative_path, number) in NESTED_PRECOMPILES {
        let source = precompiles.join(relative_path);
        let destination = packaged.join(relative_path);
        assert_eq!(
            fs::read(&source).unwrap_or_else(|error| panic!("{}: {error}", source.display())),
            fs::read(&destination)
                .unwrap_or_else(|error| panic!("{}: {error}", destination.display()))
        );
        let abi = registry
            .abis()
            .iter()
            .find(|abi| abi.contract == contract)
            .unwrap_or_else(|| panic!("{contract} ABI not packaged"));
        assert_eq!(abi.address, Some(precompile_address(number)), "{contract}");
    }

    for (contract, number, name, signature) in [
        (
            "perps",
            0x1015,
            "CollateralDeposited",
            "CollateralDeposited(address,uint256,uint256)",
        ),
        (
            "pricefeed",
            0x1020,
            "FeedKeyRegistered",
            "FeedKeyRegistered(address,address,bytes,address)",
        ),
    ] {
        let event = registry
            .lookup(precompile_address(number), keccak(signature.as_bytes()))
            .unwrap_or_else(|| panic!("{contract}.{name} not registered at {number:#x}"));
        assert_eq!(event.contract, contract);
        assert_eq!(event.name, name);
        assert_eq!(event.signature, signature);
    }
    assert!(!registry.abis().iter().any(|abi| {
        abi.contract == "layerxexchange" && abi.address == Some(precompile_address(0x1015))
    }));

    for (_, relative_path, _) in NESTED_PRECOMPILES {
        let missing = packaged.join(relative_path);
        fs::remove_file(&missing).unwrap_or_else(|error| panic!("{}: {error}", missing.display()));
        let output = Command::new("sh")
            .arg(&script)
            .arg(&packaged)
            .arg(scratch.0.join("missing abi output"))
            .output()
            .unwrap_or_else(|error| panic!("package-abis.sh: {error}"));
        assert!(
            !output.status.success(),
            "packaging must refuse missing {relative_path}"
        );
        assert!(String::from_utf8_lossy(&output.stderr).contains(relative_path));
        fs::copy(precompiles.join(relative_path), &missing)
            .unwrap_or_else(|error| panic!("restore {}: {error}", missing.display()));
    }
}
