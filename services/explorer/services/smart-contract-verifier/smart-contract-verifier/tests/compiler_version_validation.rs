use smart_contract_verifier::{solidity::standard_json, DetailedVersion, SolcValidator, Version};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    BytecodeType, VerifySolidityStandardJsonRequest,
};

// Compiler --version transcripts enter the same semver codec used by Foundry.
const RELEASE_OUTPUT: &str =
    "solc, the solidity compiler commandline interface\nVersion: 0.1.0+commit.5e4a94a.Linux.g++\n";
const BETA_OUTPUT: &str = "solc, the solidity compiler commandline interface\nVersion: 0.1.0-beta.16+commit.5e4a94a.Linux.g++\n";
const RC_OUTPUT: &str = "solc, the solidity compiler commandline interface\nVersion: 0.1.0-rc.1+commit.5e4a94a.Linux.g++\n";

fn reported_version(output: &str) -> semver::Version {
    output
        .lines()
        .rfind(|line| !line.trim().is_empty())
        .unwrap()
        .trim_start_matches("Version: ")
        .replace(".g++", ".gcc")
        .parse()
        .unwrap()
}

fn requested_version(version: &str) -> DetailedVersion {
    let request = VerifySolidityStandardJsonRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: version.into(),
        input: serde_json::json!({
            "language": "Solidity",
            "sources": {"Contract.sol": {"content": "contract Contract {}"}},
            "settings": {}
        })
        .to_string(),
        ..Default::default()
    };
    standard_json::VerificationRequest::try_from(request)
        .unwrap()
        .compiler_version
}

#[test]
fn matching_release_beta_and_rc_compiler_versions_are_accepted() {
    for (requested, output, prerelease) in [
        ("v0.1.0+commit.5e4a94a", RELEASE_OUTPUT, ""),
        ("v0.1.0-beta.16+commit.5e4a94a", BETA_OUTPUT, "beta.16"),
        ("v0.1.0-rc.1+commit.5e4a94a", RC_OUTPUT, "rc.1"),
    ] {
        let expected = requested_version(requested);
        let reported = reported_version(output);
        assert_eq!(expected.to_semver().pre.as_str(), prerelease);
        assert_eq!(reported.pre.as_str(), prerelease);
        assert!(expected.to_semver().build.is_empty());
        assert_eq!(reported.build.as_str(), "commit.5e4a94a.Linux.gcc");
        SolcValidator::default()
            .validate_version(&expected, &reported)
            .unwrap_or_else(|error| panic!("{requested}: {error}"));
    }
}

#[test]
fn mismatching_release_beta_and_rc_compilers_are_rejected() {
    for prerelease in ["", "-beta.16", "-rc.1"] {
        for requested_core in ["1.1.0", "0.2.0", "0.1.1"] {
            let requested = format!("v{requested_core}{prerelease}+commit.5e4a94a");
            let expected = requested_version(&requested);
            for output in [RELEASE_OUTPUT, BETA_OUTPUT, RC_OUTPUT] {
                let reported = reported_version(output);
                let error = SolcValidator::default()
                    .validate_version(&expected, &reported)
                    .expect_err("a different major, minor or patch must be rejected");
                assert_eq!(
                    error.to_string(),
                    format!("versions don't match: expected={requested_core}, got=0.1.0")
                );
            }
        }
    }
}

#[test]
fn prerelease_and_build_metadata_are_normalized_on_both_sides() {
    for requested in [
        "v0.1.0+commit.5e4a94a",
        "v0.1.0-beta.17+commit.0671b7b",
        "v0.1.0-rc.2+commit.0671b7b",
        "v0.1.0-nightly.2021.9.9+commit.0671b7b",
    ] {
        let expected = requested_version(requested);
        for output in [RELEASE_OUTPUT, BETA_OUTPUT, RC_OUTPUT] {
            let reported = reported_version(output);
            SolcValidator::default()
                .validate_version(&expected, &reported)
                .unwrap_or_else(|error| panic!("{requested}, {output}: {error}"));
        }
    }

    // Version is generic: requested build metadata must also be normalized when
    // the caller supplies semver directly instead of DetailedVersion.
    let expected: semver::Version = "0.1.0-beta.16+commit.0671b7b".parse().unwrap();
    let reported = reported_version(BETA_OUTPUT);
    SolcValidator::default()
        .validate_version(&expected, &reported)
        .unwrap();
    let wrong: semver::Version = "0.1.1-beta.16+commit.0671b7b".parse().unwrap();
    assert!(SolcValidator::default()
        .validate_version(&wrong, &reported)
        .is_err());
}
