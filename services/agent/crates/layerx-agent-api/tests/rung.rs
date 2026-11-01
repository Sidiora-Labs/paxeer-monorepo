use layerx_agent_api::verify::{ReasonCode, Rung, RungStatus};

#[test]
fn rung_and_rung_status_are_exported_from_the_verify_contract() {
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    assert!(rungs.windows(2).all(|pair| pair[0] < pair[1]));

    for rung in rungs {
        let (rank, name) = match rung {
            Rung::Submitted => (0, "submitted"),
            Rung::Receipted => (1, "receipted"),
            Rung::Committed => (2, "committed"),
        };
        assert_eq!(rung.wire_rank(), rank);
        assert_eq!(rung.name(), name);
        assert_eq!(Rung::from_name(name), Some(rung));

        let status = RungStatus::Achieved(rung);
        match status {
            RungStatus::Achieved(achieved) => assert_eq!(achieved, rung),
            RungStatus::Below { .. } => panic!("achieved rung reported as below"),
        }
    }

    for (requested, achieved) in [
        (Rung::Receipted, Rung::Submitted),
        (Rung::Committed, Rung::Submitted),
        (Rung::Committed, Rung::Receipted),
    ] {
        let reason = ReasonCode::new("verification.below_requested")
            .unwrap_or_else(|error| panic!("reason: {error:?}"));
        let status = RungStatus::Below {
            requested,
            achieved,
            reason: reason.clone(),
        };
        match status {
            RungStatus::Below {
                requested: actual_requested,
                achieved: actual_achieved,
                reason: actual_reason,
            } => {
                assert_eq!(actual_requested, requested);
                assert_eq!(actual_achieved, achieved);
                assert!(actual_achieved < actual_requested);
                assert_eq!(actual_reason, reason);
                assert_eq!(actual_reason.as_str(), "verification.below_requested");
            }
            RungStatus::Achieved(_) => panic!("shortfall reported as achieved"),
        }
    }
}
