use std::path::Path;

use layerx_agent_api::error::RequestId;
use layerx_agent_api::verify::{ApiSuccess, ReasonCode, Rung, RungStatus, VerificationStatus};

fn check_schema_directory(directory: &Path) -> std::io::Result<usize> {
    let mut checked = 0;
    for entry in std::fs::read_dir(directory)? {
        let path = entry?.path();
        if path.is_dir() {
            checked += check_schema_directory(&path)?;
        } else if path.extension().is_some_and(|extension| extension == "kvx") {
            let source = std::fs::read_to_string(&path)?;
            for retired in ["SettlementDomain", "settlement_domain"] {
                assert!(
                    !source.contains(retired),
                    "{} retains {retired}",
                    path.display()
                );
            }
            checked += 1;
        }
    }
    Ok(checked)
}

#[test]
fn settlement_domain_vocabulary_is_absent() -> std::io::Result<()> {
    let directory = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/agent-api");
    assert!(check_schema_directory(&directory)? > 0);
    assert!(check_schema_directory(&directory.join("golden"))? > 0);
    Ok(())
}

#[test]
fn native_send_v1_records_declare_the_explicit_legacy_refusal() {
    for source in [
        layerx_agent_api::AGENT_API_V1_SOURCE,
        include_str!("../../../schema/agent-api/golden/v1.kvx"),
    ] {
        for record in ["NativeSendPurposeV1", "SignedNativeSendPurposeV1"] {
            let header = format!("[record.{record}]\n");
            let section = source
                .split_once(&header)
                .expect("retired V1 record must remain declared")
                .1
                .split("\n[")
                .next()
                .unwrap();
            let retired = section
                .lines()
                .find_map(|line| line.strip_prefix("retired = "))
                .expect("V1 record must declare its retirement");
            assert_eq!(
                retired,
                "\"Retired: validation explicitly refuses with ContractError::Mismatch(native_send.legacy_version); use NativeSendPurposeV2 and SignedNativeSendPurposeV2.\""
            );
            assert!(section.contains(&format!("rust = \"layerx_agent_api::identity::{record}\"")));
            assert!(section.contains(&format!("wire_type = \"{record}Wire\"")));
            assert!(section.lines().any(|line| line.starts_with("fields = [")));
        }
    }
}

#[test]
fn verification_status_is_expressed_in_rungs_only() {
    let identity: fn(RungStatus) -> VerificationStatus = |status| status;
    assert_eq!(
        identity(RungStatus::Achieved(Rung::Submitted)),
        VerificationStatus::Achieved(Rung::Submitted)
    );
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    assert!(rungs.windows(2).all(|pair| pair[0] < pair[1]));
    for rung in rungs {
        let success = ApiSuccess {
            request_id: RequestId(7),
            value: (),
            verification_status: VerificationStatus::Achieved(rung),
        };
        match success.verification_status {
            VerificationStatus::Achieved(achieved) => assert_eq!(achieved, rung),
            VerificationStatus::Below { .. } => panic!("achieved rung reported as unverified"),
        }
    }
    for (requested, achieved) in [
        (Rung::Receipted, Rung::Submitted),
        (Rung::Committed, Rung::Submitted),
        (Rung::Committed, Rung::Receipted),
    ] {
        let reason = ReasonCode::new("verification.below_requested")
            .unwrap_or_else(|error| panic!("invalid reason: {error:?}"));
        let success = ApiSuccess {
            request_id: RequestId(8),
            value: (),
            verification_status: VerificationStatus::Below {
                requested,
                achieved,
                reason: reason.clone(),
            },
        };
        match success.verification_status {
            VerificationStatus::Below {
                requested: actual_requested,
                achieved: actual_achieved,
                reason: actual_reason,
            } => {
                assert_eq!(actual_requested, requested);
                assert_eq!(actual_achieved, achieved);
                assert!(actual_achieved < actual_requested);
                assert_eq!(actual_reason, reason);
            }
            VerificationStatus::Achieved(_) => panic!("shortfall reported as achieved"),
        }
    }
    for source in [
        include_str!("../src/error.rs"),
        include_str!("../src/lib.rs"),
    ] {
        assert!(
            !source
                .split(|character: char| !character.is_alphanumeric() && character != '_')
                .any(|token| matches!(token, "Level" | "VerificationLevel")),
            "retired verification level remains in the API"
        );
    }
}
