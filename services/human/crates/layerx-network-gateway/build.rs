use std::{env, fs, io, path::PathBuf};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let source_path = PathBuf::from(env::var("CARGO_MANIFEST_DIR")?)
        .join("../../../platform/hosted/gateway/src/evm.rs");
    println!("cargo:rerun-if-changed={}", source_path.display());

    let source = fs::read_to_string(source_path)?;
    let plain = "word_u64(123456789)";
    let separated = "word_u64(123_456_789)";
    if source.matches(plain).count() + source.matches(separated).count() != 1 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "hosted gateway ABI quantity vector is absent or ambiguous",
        )
        .into());
    }
    let source = source.replacen(plain, separated, 1);
    let fixture = "\"../tests/fixtures/paxeer-abi-vectors.json\"";
    if source.matches(fixture).count() != 1 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "hosted gateway ABI fixture path is absent or ambiguous",
        )
        .into());
    }
    let source = source.replacen(
        fixture,
        "concat!(env!(\"CARGO_MANIFEST_DIR\"), \"/../../../platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json\")",
        1,
    );
    let source = source
        .lines()
        .map(|line| match line.strip_prefix("//!") {
            Some(text) => format!("//{text}"),
            None => line.to_owned(),
        })
        .collect::<Vec<_>>()
        .join("\n");
    let output = PathBuf::from(env::var("OUT_DIR")?).join("hosted_evm.rs");
    fs::write(output, source)?;
    Ok(())
}
