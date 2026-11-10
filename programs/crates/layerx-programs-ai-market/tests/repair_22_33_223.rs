//! Audit 22.30.125: the existing fuel and close integration suites are the repair gate.
//! Keeping their actual test functions here ensures that all assertions execute.
//! Finding-to-test mappings and external wiring requests: repair_22_33_223.kvx.
#[path = "fuel_budget.rs"]
mod fuel_budget;
#[path = "support/guest.rs"]
mod guest;
#[path = "market_protocol_boundary.rs"]
mod market_protocol_boundary;

#[test]
fn participant_chunks_preserve_other_rosters() {
    use layerx_programs_ai_market::aggregation::{participant_chunk_size, WORKERS_PER_CHUNK};
    assert_eq!(WORKERS_PER_CHUNK, 8);
    for workers in 1..32 {
        assert_eq!(participant_chunk_size(workers), 8);
    }
    assert_eq!(participant_chunk_size(32), 6);
    assert_eq!(32usize.div_ceil(participant_chunk_size(32)), 6);
}

#[test]
fn fuel_guest_build_profile() -> Result<(), Box<dyn std::error::Error>> {
    assert_eq!(guest::BUILD_OPT_LEVEL, "z");
    assert_eq!(
        guest::BUILD_POSTPROCESS,
        "wasm-opt -Oz --flatten --rereloop -Oz --converge --merge-similar-functions -Oz --converge --strip-debug --strip-producers"
    );
    assert_eq!(guest::DEPLOYMENT_ENVELOPE_BYTES, 104);
    assert_eq!(guest::DEPLOYMENT_LIMIT, 524_288);
    assert_eq!(
        layerx_programs_ai_market::MERGED_CHAIN_DOMAIN,
        layerx_programs_ai_market::merged_chain_domain(125)
    );
    let _module = guest::module()?;
    Ok(())
}

#[test]
fn padding_validation_retains_all_bytes() {
    use layerx_programs_ai_market::{codec::Reader, errors::NON_CANONICAL};
    for len in 0..=129 {
        let mut bytes = vec![0; len];
        assert_eq!(Reader::new(&bytes).reserved(len), Ok(()));
        for position in 0..len {
            bytes[position] = 1;
            assert_eq!(Reader::new(&bytes).reserved(len), Err(NON_CANONICAL));
            bytes[position] = 0;
        }
        assert_eq!(Reader::new(&bytes).reserved(len + 1), Err(NON_CANONICAL));
    }
}

#[test]
fn reward_padding_and_counter_invariants_remain_enforced(
) -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{
        rewards::{self, RewardLedger, RewardState},
        AccountId, AssetId,
    };
    let ledger = RewardLedger::new(
        AssetId::new([1; 32])?,
        AccountId::new([2; 32])?,
        AccountId::new([3; 32])?,
    )?;
    let mut valid = vec![0; rewards::REWARD_STATE_BYTES];
    RewardState::init(&ledger, &mut valid)?;
    let dictionary = rewards::LEDGER_BYTES;
    let epochs = dictionary + rewards::DICTIONARY_BYTES;
    for slot in [0, 1, 127, 255] {
        for byte in 0..rewards::SLOT_BYTES {
            let mut malformed = valid.clone();
            malformed[dictionary + slot * rewards::SLOT_BYTES + byte] = 1;
            assert!(rewards::decode_reward_state(&malformed).is_err());
        }
    }
    for row in [0, 1, 16, rewards::MAX_EPOCH_ROWS - 1] {
        for byte in [0, 7, 8, 31, 32, 63, 64, rewards::EPOCH_MAX_BYTES - 1] {
            let mut malformed = valid.clone();
            malformed[epochs + row * rewards::EPOCH_MAX_BYTES + byte] = 1;
            assert!(rewards::decode_reward_state(&malformed).is_err());
        }
    }
    let mut inconsistent = ledger;
    inconsistent.retained_epochs = 1;
    let mut malformed = valid.clone();
    rewards::encode_ledger(&inconsistent, &mut malformed[..rewards::LEDGER_BYTES])?;
    assert!(rewards::decode_reward_state(&malformed).is_err());
    assert!(rewards::decode_reward_state(&valid).is_ok());
    Ok(())
}

#[test]
fn ordered_task_lookup_retains_canonical_validation() -> layerx_programs_ai_market::CodecResult<()>
{
    use layerx_programs_ai_market::{
        errors::NON_CANONICAL,
        tasks::{TaskBinding, TaskSet, TaskStatus},
        Digest32, PrincipalId, TaskId, WorkerId,
    };
    let mut region = 64u16.to_be_bytes().to_vec();
    region.push(0);
    for n in 1..=64u8 {
        region.extend_from_slice(
            &TaskBinding {
                task: TaskId::new([n * 2; 32])?,
                requester: PrincipalId::new([1; 32])?,
                worker: WorkerId::new([2; 32])?,
                input: Digest32::new([3; 32])?,
                deadline: 200,
                status: TaskStatus::Cancelled,
                acknowledgement: None,
                result: None,
                admission: Digest32::new([4; 32])?,
            }
            .encode()?,
        );
    }
    let set = TaskSet::decode(&region)?;
    for n in 1..=129u8 {
        let (index, found) = set.position(TaskId::new([n; 32])?)?;
        assert_eq!(found.is_some(), n % 2 == 0);
        assert_eq!(
            index,
            usize::from(if n % 2 == 0 { n / 2 - 1 } else { n / 2 })
        );
        if let Some(binding) = found {
            assert_eq!(binding.task, TaskId::new([n; 32])?);
            assert_eq!(binding.status, TaskStatus::Cancelled);
        }
    }
    let first = region[3..3 + layerx_programs_ai_market::tasks::TASK_BINDING_BYTES].to_vec();
    region[3 + first.len()..3 + 2 * first.len()].copy_from_slice(&first);
    assert_eq!(TaskSet::decode(&region), Err(NON_CANONICAL));
    Ok(())
}

#[test]
fn dictionary_validation_rejects_duplicate_and_invalid_slots(
) -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{
        errors::{ACCOUNT_BINDING, NON_CANONICAL},
        rewards::{decode_dictionary, RecipientDictionary, RecipientSlot, DICTIONARY_BYTES},
        AccountId, WorkerId,
    };
    let reserve = AccountId::new([9; 32])?;
    let recipient = AccountId::new([2; 32])?;
    let mut bytes = vec![0; DICTIONARY_BYTES];
    for index in 0..256u16 {
        let mut worker = [0; 32];
        worker[30..].copy_from_slice(&(256 - index).to_be_bytes());
        RecipientDictionary::write_slot(
            &mut bytes,
            index,
            Some(&RecipientSlot {
                worker: WorkerId::new(worker)?,
                recipient,
                references: 1,
            }),
        )?;
    }
    let first = decode_dictionary(&bytes, reserve)?.slot(0)?;
    for invalid in [
        first,
        RecipientSlot {
            references: 0,
            ..first
        },
        RecipientSlot {
            recipient: reserve,
            ..first
        },
    ] {
        let mut malformed = bytes.clone();
        RecipientDictionary::write_slot(&mut malformed, 255, Some(&invalid))?;
        let expected = if invalid.recipient == reserve {
            ACCOUNT_BINDING
        } else {
            NON_CANONICAL
        };
        assert_eq!(decode_dictionary(&malformed, reserve), Err(expected));
    }
    assert_eq!(decode_dictionary(&bytes, reserve)?.occupied(), 256);
    Ok(())
}

#[test]
fn strict_report_boundaries_retain_vector_refusals() -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{
        codec::{self, ReportBody, ScoreVector},
        errors::{F03_NONCANONICAL_VECTOR, F03_SCORE_RANGE, NON_CANONICAL},
        types::{
            ChainDomain, EvaluatorBinding, EvaluatorId, EvidenceRoot, FrozenBinding, MarketId,
            ProgramId, RosterDigest, Score, ScoreEntry, Version, WorkerId,
        },
    };
    let cells = [
        ScoreEntry {
            worker: WorkerId::new([1; 32])?,
            score: Score::new(1)?,
        },
        ScoreEntry {
            worker: WorkerId::new([2; 32])?,
            score: Score::new(1_000_000)?,
        },
    ];
    let body = ReportBody {
        binding: EvaluatorBinding {
            frozen: FrozenBinding {
                chain: ChainDomain::new([3; 32])?,
                program: ProgramId::new([4; 32])?,
                market: MarketId::new([5; 32])?,
                epoch: 6,
                config: Version::new(1)?,
                roster: RosterDigest::new([7; 32])?,
            },
            evaluator: EvaluatorId::new([8; 32])?,
            grant: Version::new(1)?,
            key_version: Version::new(1)?,
        },
        evidence: EvidenceRoot::new([9; 32])?,
        scores: ScoreVector::Typed(&cells),
    };
    let mut bytes = vec![0; codec::REPORT_MAX_BYTES];
    let len = codec::encode_report(&body, &mut bytes)?;
    bytes.truncate(len);
    let decoded = layerx_programs_ai_market::evaluators::codec::decode_report_body(&bytes)?;
    assert_eq!(
        codec::report_digest(&decoded)?,
        codec::report_digest(&body)?
    );
    let at = codec::REPORT_FIXED_BYTES;
    let mut duplicate = bytes.clone();
    duplicate[at + 36..at + 68].copy_from_slice(&bytes[at..at + 32]);
    assert_eq!(codec::decode_report(&duplicate), Err(NON_CANONICAL));
    assert_eq!(
        layerx_programs_ai_market::evaluators::codec::decode_report_body(&duplicate),
        Err(F03_NONCANONICAL_VECTOR)
    );
    let mut range = bytes.clone();
    range[at + 32..at + 36].copy_from_slice(&1_000_001u32.to_be_bytes());
    assert_eq!(codec::decode_report(&range), Err(F03_SCORE_RANGE));
    assert_eq!(
        layerx_programs_ai_market::evaluators::codec::decode_report_body(&range),
        Err(F03_SCORE_RANGE)
    );
    let mut short = bytes.clone();
    short.pop();
    assert!(codec::decode_report(&short).is_err());
    assert!(layerx_programs_ai_market::evaluators::codec::decode_report_body(&short).is_err());
    Ok(())
}

#[test]
fn reputation_roster_merge_preserves_bindings_and_refusals(
) -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{
        errors::{F07_BINDING_MISMATCH, F07_UNKNOWN_WORKER, NON_CANONICAL},
        reputation::ReputationState,
        reputation_transition::{open_segments, seal_history, SegmentBinding},
        AccountId, Digest32, MarketId, MetadataDigest, PrincipalId, PublicKey32, Version, WorkerId,
        WorkerRosterEntry,
    };
    let market = MarketId::new([1; 32])?;
    let binding = SegmentBinding {
        config: Version::new(1)?,
        policy: Digest32::new([2; 32])?,
        model: Digest32::new([3; 32])?,
    };
    let worker = |id: u8| -> layerx_programs_ai_market::CodecResult<WorkerRosterEntry> {
        Ok(WorkerRosterEntry {
            worker: WorkerId::new([id; 32])?,
            owner: PrincipalId::new([id + 10; 32])?,
            recipient: AccountId::new([id + 20; 32])?,
            generation: Version::new(1)?,
            key_version: Version::new(1)?,
            public_key: PublicKey32([id + 30; 32]),
            metadata: MetadataDigest::new([id + 40; 32])?,
        })
    };
    let roster = [worker(3)?, worker(1)?];
    let state = open_segments(
        &ReputationState::new(market),
        binding,
        binding,
        &roster,
        |_| false,
        1,
    )?;
    assert_eq!(
        state.records().map(|r| r.worker).collect::<Vec<_>>(),
        vec![worker(1)?.worker, worker(3)?.worker]
    );
    let state = open_segments(
        &state,
        binding,
        binding,
        &[worker(1)?, worker(2)?, worker(3)?],
        |_| false,
        2,
    )?;
    let ids = [worker(1)?.worker, worker(2)?.worker, worker(3)?.worker];
    let seal = seal_history(&state, 1, &ids, 3)?;
    assert!((0..3).all(|i| seal.allows(i)));
    assert_eq!(
        seal_history(&state, 1, &[worker(4)?.worker], 3),
        Err(F07_UNKNOWN_WORKER)
    );
    assert_eq!(
        seal_history(&state, 1, &[ids[1], ids[0]], 3),
        Err(NON_CANONICAL)
    );
    let wrong_owner = WorkerRosterEntry {
        owner: PrincipalId::new([99; 32])?,
        ..worker(2)?
    };
    assert!(matches!(
        open_segments(&state, binding, binding, &[wrong_owner], |_| false, 3),
        Err(F07_BINDING_MISMATCH)
    ));
    Ok(())
}

#[test]
fn aggregation_chunk_merge_preserves_sparse_votes_and_zero_scores(
) -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{
        aggregation::{add_weight, aggregate_chunk, aggregate_worker},
        aggregation_codec::AggregationInputView,
        codec::{self, ReportBody, Roster, ScoreVector},
        errors::{F05_REPORT_INVARIANT, STALE_CURSOR},
        AccountId, ChainDomain, EvaluatorBinding, EvaluatorId, EvaluatorRosterEntry, EvidenceRoot,
        FrozenBinding, MarketId, MetadataDigest, PrincipalId, ProgramId, PublicKey32, RubricDigest,
        Score, ScoreEntry, Version, WorkerId, WorkerRosterEntry,
    };
    let version = Version::new(1)?;
    let workers = (1..=32u8)
        .map(|id| {
            Ok(WorkerRosterEntry {
                worker: WorkerId::new([id; 32])?,
                owner: PrincipalId::new([id + 32; 32])?,
                recipient: AccountId::new([id + 32; 32])?,
                generation: version,
                key_version: version,
                public_key: PublicKey32([id + 64; 32]),
                metadata: MetadataDigest::new([id + 96; 32])?,
            })
        })
        .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
    let evaluators = (0..8u8)
        .map(|id| {
            Ok(EvaluatorRosterEntry {
                evaluator: EvaluatorId::new([id + 128; 32])?,
                owner: PrincipalId::new([id + 144; 32])?,
                grant: version,
                key_version: version,
                public_key: PublicKey32([id + 160; 32]),
                rubric: RubricDigest::new([192; 32])?,
            })
        })
        .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
    let roster = Roster {
        market: MarketId::new([200; 32])?,
        epoch: 3,
        config: version,
        workers: &workers,
        evaluators: &evaluators,
    };
    let frozen = FrozenBinding {
        chain: ChainDomain::new([201; 32])?,
        program: ProgramId::new([202; 32])?,
        market: roster.market,
        epoch: roster.epoch,
        config: version,
        roster: codec::roster_digest(&roster)?,
    };
    for dense in [false, true] {
        let scores = (0..8usize)
            .map(|evaluator| {
                workers
                    .iter()
                    .enumerate()
                    .filter(|(i, _)| dense || (i + evaluator) % 5 != 0)
                    .map(|(i, worker)| {
                        Ok(ScoreEntry {
                            worker: worker.worker,
                            score: Score::new(if i % 7 == 0 {
                                0
                            } else {
                                (i * 1000 + evaluator) as u32
                            })?,
                        })
                    })
                    .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()
            })
            .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
        let reports = evaluators
            .iter()
            .zip(&scores)
            .map(|(evaluator, scores)| {
                Ok(ReportBody {
                    binding: EvaluatorBinding {
                        frozen,
                        evaluator: evaluator.evaluator,
                        grant: version,
                        key_version: version,
                    },
                    evidence: EvidenceRoot::new([203; 32])?,
                    scores: ScoreVector::Typed(scores),
                })
            })
            .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
        let encoded = reports
            .iter()
            .map(|report| {
                let mut bytes = vec![0; codec::REPORT_MAX_BYTES];
                let n = codec::encode_report(report, &mut bytes)?;
                bytes.truncate(n);
                Ok(bytes)
            })
            .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
        let decoded = encoded
            .iter()
            .map(|bytes| codec::decode_report(bytes))
            .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
        for reports in [&reports, &decoded] {
            let view = AggregationInputView::structural(frozen, roster, reports)?;
            let mut running = 0;
            for start in [0, 8, 16, 24] {
                let chunk = aggregate_chunk(&view, start as u16, running)?;
                assert_eq!(chunk.len(), 8);
                for i in 0..8 {
                    let expected = aggregate_worker(&view, workers[start + i])?;
                    assert_eq!(chunk.output(i)?, expected);
                    running = add_weight(running, expected)?;
                }
                assert_eq!(chunk.running_weight(), running);
                assert_eq!(chunk.cursor(), (start + 8) as u16);
            }
            assert!(aggregate_chunk(&view, 32, running)?.is_empty());
            assert!(matches!(
                aggregate_chunk(&view, 33, running),
                Err(STALE_CURSOR)
            ));
            assert!(matches!(
                view.worker_votes(workers[0].worker, Version::new(2)?),
                Err(F05_REPORT_INVARIANT)
            ));
        }
    }

    Ok(())
}

#[test]
fn resumable_participant_calls_recover_and_retry_once() -> Result<(), Box<dyn std::error::Error>> {
    fuel_budget::resumable_participant_retry_proof()
}

#[test]
fn participant_identity_comparisons_preserve_every_byte() {
    use layerx_programs_ai_market::{Digest32, WorkerId};
    let baseline = [0x80; 32];
    let worker = WorkerId::new(baseline).unwrap();
    let digest = Digest32::new(baseline).unwrap();
    assert_eq!(worker, worker);
    assert_eq!(digest, digest);
    for position in 0..32 {
        for value in 0..=255 {
            let mut bytes = baseline;
            bytes[position] = value;
            let candidate = WorkerId::new(bytes).unwrap();
            let candidate_digest = Digest32::new(bytes).unwrap();
            assert_eq!(worker == candidate, baseline == bytes);
            assert_eq!(digest == candidate_digest, baseline == bytes);
            assert_eq!(worker.cmp(&candidate), baseline.cmp(&bytes));
            assert_eq!(digest.cmp(&candidate_digest), baseline.cmp(&bytes));
            assert_eq!(candidate.cmp(&worker), bytes.cmp(&baseline));
        }
    }
    for position in 0..31 {
        let mut left = baseline;
        let mut right = baseline;
        left[position] = 1;
        right[position + 1] = 0;
        assert_eq!(
            WorkerId::new(left)
                .unwrap()
                .cmp(&WorkerId::new(right).unwrap()),
            left.cmp(&right)
        );
    }
    for position in 0..32 {
        for value in 1..=255 {
            let mut bytes = [0; 32];
            bytes[position] = value;
            assert_eq!(WorkerId::new(bytes).unwrap().bytes(), bytes);
            assert_eq!(Digest32::new(bytes).unwrap().bytes(), bytes);
        }
    }
    assert!(WorkerId::new([0; 32]).is_err());
    assert!(Digest32::new([0; 32]).is_err());
}

#[test]
fn participant_record_codec_keeps_the_canonical_layout() {
    use layerx_programs_ai_market::{
        reputation::{HistoryStatus, Observation, ReputationCurrent, SegmentKey, CURRENT_BYTES},
        reputation_codec::{decode_current, encode_current},
        AccountId, Digest32, MarketId, Presence, PrincipalId, Score, Version, WorkerId,
    };
    let key = SegmentKey {
        market: MarketId::new([1; 32]).unwrap(),
        worker: WorkerId::new([2; 32]).unwrap(),
        config: Version::new(1).unwrap(),
        policy: Digest32::new([3; 32]).unwrap(),
        model: Digest32::new([4; 32]).unwrap(),
        reset_generation: Version::new(1).unwrap(),
    };
    let mut record =
        ReputationCurrent::bootstrap(key, PrincipalId::new([5; 32]).unwrap(), 100).unwrap();
    record.quality = Score::new(600_000).unwrap();
    record.qualifying_count = 1;
    record.coverage = Presence::Present(Score::new(750_000).unwrap());
    record.last_applied = Presence::Present(9);
    record.last_observed = Presence::Present(Observation {
        epoch: 9,
        height: 120,
    });
    record.last_transition_height = 120;
    let mut expected = Vec::new();
    expected.extend_from_slice(record.worker.as_bytes());
    expected.extend_from_slice(record.segment.as_bytes());
    expected.extend_from_slice(record.owner.as_bytes());
    expected.extend_from_slice(&1u64.to_be_bytes());
    for n in [600_000u32, 1, 750_000] {
        expected.extend_from_slice(&n.to_be_bytes());
    }
    for n in [9u64, 9, 120, 120] {
        expected.extend_from_slice(&n.to_be_bytes());
    }
    expected.extend_from_slice(&[0; 32]);
    expected.extend_from_slice(&[HistoryStatus::Active as u8, 7, 0, 0]);
    assert_eq!(expected.len(), CURRENT_BYTES);
    assert_eq!(encode_current(&record).unwrap().as_slice(), expected);
    assert_eq!(decode_current(&expected).unwrap(), record);
    for n in 0..CURRENT_BYTES {
        assert!(decode_current(&expected[..n]).is_err());
    }
    let mut trailing = expected.clone();
    trailing.push(0);
    assert!(decode_current(&trailing).is_err());
    for position in [180, 181, 182, 183] {
        let mut invalid = expected.clone();
        invalid[position] = 0xff;
        assert!(decode_current(&invalid).is_err());
    }
    for start in [0, 32, 64] {
        let mut invalid = expected.clone();
        invalid[start..start + 32].fill(0);
        assert!(decode_current(&invalid).is_err());
    }
    for start in [104, 112] {
        let mut invalid = expected.clone();
        invalid[start..start + 4].copy_from_slice(&1_000_001u32.to_be_bytes());
        assert!(decode_current(&invalid).is_err());
    }
    let mut roster = Vec::new();
    roster.extend_from_slice(key.worker.as_bytes());
    roster.extend_from_slice(record.owner.as_bytes());
    roster.extend_from_slice(AccountId::new([6; 32]).unwrap().as_bytes());
    roster.extend_from_slice(&1u64.to_be_bytes());
    roster.extend_from_slice(&2u64.to_be_bytes());
    roster.extend_from_slice(&[7; 32]);
    roster.extend_from_slice(&[8; 32]);
    let decoded = layerx_programs_ai_market::codec::decode_worker_roster(&roster).unwrap();
    assert_eq!(decoded.worker, key.worker);
    assert_eq!(decoded.owner, record.owner);
    assert_eq!(decoded.recipient.bytes(), [6; 32]);
    assert_eq!(decoded.generation.get(), 1);
    assert_eq!(decoded.key_version.get(), 2);
    assert_eq!(decoded.public_key.0, [7; 32]);
    assert_eq!(decoded.metadata.bytes(), [8; 32]);
    for n in 0..roster.len() {
        assert!(layerx_programs_ai_market::codec::decode_worker_roster(&roster[..n]).is_err());
    }
    for start in [0, 32, 64, 144] {
        let mut invalid = roster.clone();
        invalid[start..start + 32].fill(0);
        assert!(layerx_programs_ai_market::codec::decode_worker_roster(&invalid).is_err());
    }
    for start in [96, 104] {
        let mut invalid = roster.clone();
        invalid[start..start + 8].fill(0);
        assert!(layerx_programs_ai_market::codec::decode_worker_roster(&invalid).is_err());
    }
}

#[test]
fn history_record_codec_keeps_the_canonical_layout() {
    use layerx_programs_ai_market::{
        errors::NON_CANONICAL,
        reputation::{CompletedHistory, HISTORY_BYTES},
        reputation_codec::{decode_history, encode_history},
        Digest32, Version,
    };
    let mut record = CompletedHistory {
        epoch: 9,
        execution_height: 120,
        config: Version::new(1).unwrap(),
        result: Digest32::new([3; 32]).unwrap(),
        root: Digest32::new([4; 32]).unwrap(),
        observed_workers: 2,
        total_workers: 32,
        covered_workers: 8,
    };
    let mut expected = Vec::new();
    for value in [9u64, 120, 1] {
        expected.extend_from_slice(&value.to_be_bytes());
    }
    expected.extend_from_slice(&[3; 32]);
    expected.extend_from_slice(&[4; 32]);
    expected.extend_from_slice(&[2, 32, 8, 0]);
    assert_eq!(expected.len(), HISTORY_BYTES);
    assert_eq!(encode_history(&record).unwrap().as_slice(), expected);
    assert_eq!(decode_history(&expected).unwrap(), record);
    for n in 0..HISTORY_BYTES {
        assert_eq!(decode_history(&expected[..n]), Err(NON_CANONICAL));
    }
    let mut trailing = expected.clone();
    trailing.push(0);
    assert_eq!(decode_history(&trailing), Err(NON_CANONICAL));
    for position in [88, 89, 90, 91] {
        for byte in if position == 91 { 1..=255 } else { 33..=255 } {
            let mut invalid = expected.clone();
            invalid[position] = byte;
            assert_eq!(decode_history(&invalid), Err(NON_CANONICAL));
        }
    }
    for total in 0..=32 {
        for observed in 0..=total {
            record.total_workers = total;
            record.observed_workers = observed;
            record.covered_workers = total - observed;
            let bytes = encode_history(&record).unwrap();
            assert_eq!(&bytes[88..], &[observed, total, total - observed, 0]);
            assert_eq!(decode_history(&bytes).unwrap(), record);
        }
    }
}

#[test]
fn projected_score_medians_preserve_values_and_comparison_counts(
) -> layerx_programs_ai_market::CodecResult<()> {
    use layerx_programs_ai_market::{aggregation::lower_median, errors::CAPACITY, Presence, Score};
    // Every two-valued ordering at every bounded length, including ties and
    // measured zeroes. Count inversions and stopping comparisons independently.
    for pattern in 0..256u16 {
        for length in 0..=8 {
            let raw = (0..length)
                .map(|bit| {
                    if pattern & (1 << bit) == 0 {
                        0
                    } else {
                        1_000_000
                    }
                })
                .collect::<Vec<_>>();
            let scores = raw
                .iter()
                .copied()
                .map(Score::new)
                .collect::<layerx_programs_ai_market::CodecResult<Vec<_>>>()?;
            let expected_comparisons = (1..length)
                .map(|index| {
                    raw[..index]
                        .iter()
                        .filter(|&&value| value > raw[index])
                        .count()
                        + usize::from(raw[..index].iter().any(|&value| value <= raw[index]))
                })
                .sum::<usize>();
            let result = lower_median(&scores)?;
            assert_eq!(usize::from(result.support), length);
            assert_eq!(result.comparisons as usize, expected_comparisons);
            let mut ordered = raw;
            ordered.sort_unstable();
            assert_eq!(
                result.selected,
                if length < 3 {
                    Presence::Absent
                } else {
                    Presence::Present(Score::new(ordered[(length - 1) / 2])?)
                }
            );
        }
    }
    assert_eq!(lower_median(&[Score::new(0)?; 9]), Err(CAPACITY));
    Ok(())
}
