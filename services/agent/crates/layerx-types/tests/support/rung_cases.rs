use layerx_types::rung::Rung;

#[test]
fn current_rungs_round_trip_and_preserve_the_exact_order() {
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    for (rank, rung) in rungs.into_iter().enumerate() {
        assert_eq!(usize::from(rung.wire_rank()), rank);
        assert_eq!(Rung::from_name(rung.name()), Some(rung));
        for other in rungs {
            assert_eq!(rung.cmp(&other), rung.wire_rank().cmp(&other.wire_rank()));
        }
    }
    assert!(rungs.windows(2).all(|pair| pair[0] < pair[1]));
}

#[test]
fn current_rungs_refuse_all_legacy_and_malformed_labels() {
    for label in [
        "unverified",
        "sequencer-signed",
        "batch-included",
        "state-proven",
        "checkpoint-finalised",
        "settlement-anchored",
        "checkpoint_finalised",
        "settlement_anchored",
        "Submitted",
        "RECEIPTED",
        " committed",
        "committed ",
        "committed\0",
        "0",
        "1",
        "2",
        "3",
        "5",
        "",
        "unknown",
    ] {
        assert_eq!(Rung::from_name(label), None, "{label:?}");
    }
}

// Historical decoding is serialization compatibility only. No value in this
// module is passed to an authority, budget, read, or tracking consumer.
mod compatibility {
    use layerx_types::rung::Rung;
    use layerx_types::verify::{InvalidVerificationRank, VerificationLevel};
    #[test]
    #[allow(
        deprecated,
        reason = "Test retained historical wire labels without granting current authority."
    )]
    fn verification_levels_form_the_declared_order() {
        let levels = [
            VerificationLevel::UNVERIFIED,
            VerificationLevel::SEQUENCER_SIGNED,
            VerificationLevel::BATCH_INCLUDED,
            VerificationLevel::STATE_PROVEN,
            VerificationLevel::COMMITTED,
        ];
        for pair in levels.windows(2) {
            assert_eq!(pair[0].compare(pair[1]), std::cmp::Ordering::Less);
            assert!(pair[0] < pair[1]);
        }
        assert_eq!(VerificationLevel::UNVERIFIED.wire_rank(), 0);
        assert_eq!(VerificationLevel::SETTLEMENT_ANCHORED.wire_rank(), 5);
    }

    #[test]
    #[allow(
        deprecated,
        reason = "Test retained historical wire labels without granting current authority."
    )]
    fn verification_level_maps_onto_rungs_by_decision() {
        let cases = [
            (VerificationLevel::UNVERIFIED, Rung::Submitted, 0),
            (VerificationLevel::SEQUENCER_SIGNED, Rung::Submitted, 1),
            (VerificationLevel::BATCH_INCLUDED, Rung::Receipted, 2),
            (VerificationLevel::STATE_PROVEN, Rung::Receipted, 3),
            (VerificationLevel::COMMITTED, Rung::Committed, 6),
        ];
        for (level, rung, rank) in cases {
            assert_eq!(level.rung(), Some(rung), "{level:?}");
            assert_eq!(level.wire_rank(), rank);
        }
    }

    #[test]
    #[allow(
        deprecated,
        reason = "Test retained historical wire labels without granting current authority."
    )]
    fn settlement_anchored_level_has_no_rung() {
        const RUNG: Option<Rung> = VerificationLevel::SETTLEMENT_ANCHORED.rung();
        assert_eq!(RUNG, None);
        assert_eq!(VerificationLevel::SETTLEMENT_ANCHORED.wire_rank(), 5);
        for archived in [
            VerificationLevel::CHECKPOINT_FINALISED,
            VerificationLevel::SETTLEMENT_ANCHORED,
        ] {
            assert_eq!(archived.rung(), None);
            for current in [
                VerificationLevel::UNVERIFIED,
                VerificationLevel::COMMITTED,
                archived,
            ] {
                assert_eq!(archived.compare(current), std::cmp::Ordering::Less);
                assert_eq!(current.compare(archived), std::cmp::Ordering::Less);
            }
        }
    }

    #[test]
    #[allow(
        deprecated,
        reason = "Verify the retained historical rank decoder rejects unknown wire labels."
    )]
    fn historical_ranks_round_trip_and_unknown_ranks_are_refused() {
        let levels = [
            VerificationLevel::UNVERIFIED,
            VerificationLevel::SEQUENCER_SIGNED,
            VerificationLevel::BATCH_INCLUDED,
            VerificationLevel::STATE_PROVEN,
            VerificationLevel::CHECKPOINT_FINALISED,
            VerificationLevel::SETTLEMENT_ANCHORED,
        ];
        for (rank, level) in levels.into_iter().enumerate() {
            assert_eq!(
                layerx_types::historical::verify::HistoricalVerificationLevel::from_wire_rank(
                    u8::try_from(rank).unwrap_or_else(|error| panic!("historical rank: {error}"))
                )
                .map(|historical| {
                    assert_eq!(historical.rung(), None);
                    historical.wire_rank()
                }),
                Ok(level.wire_rank())
            );
            assert_eq!(
                level.wire_rank(),
                u8::try_from(rank).unwrap_or_else(|error| panic!("historical rank: {error}"))
            );
            // Even a historical label that maps onto a rung is refused as a
            // current wire label. Decoding does not establish that rung.
            for label in [
                "unverified",
                "sequencer-signed",
                "batch-included",
                "state-proven",
                "checkpoint-finalised",
                "settlement-anchored",
            ] {
                assert_eq!(Rung::from_name(label), None);
            }
        }
        for rank in [4, 5] {
            assert_eq!(
                VerificationLevel::from_wire_rank(rank),
                Err(InvalidVerificationRank(rank))
            );
        }
        assert_eq!(
            VerificationLevel::from_wire_rank(6),
            Ok(VerificationLevel::COMMITTED)
        );
        for rank in 6..=255 {
            assert_eq!(
                layerx_types::historical::verify::HistoricalVerificationLevel::from_wire_rank(rank),
                Err(InvalidVerificationRank(rank))
            );
        }
    }
}
