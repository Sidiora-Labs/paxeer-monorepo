//! Task 22.33.228 / audit 22.30.126 / acceptance 22998.1.
//! f04-admission-fixture -> f04_admission_fixture_routes_admission_replay_and_aggregation:
//! real ScheduleEvaluator, ApproveAdmission and AdmitEvaluator envelopes establish replay;
//! evidence/commit/reveal sequences follow committed replay, and F05 runs through dispatch.
//! f04-reopen-fixture -> f04_reopen_fixture_recovers_and_retries_the_persisted_reveal:
//! discard the client and market, reopen the preserved journal, reload canonical market and
//! replay bytes, then send and retry the exact persisted reveal without changing state.
//! The shared recovery suite is compiled here so every original assertion also executes.
//! No out-of-path implementation or wiring request is needed for these two findings.
//! Baseline at 4b4896072f927b28618921285b3e3ae758bc467e: both audited fixture gaps
//! were present; this repair target did not exist. No baseline tests ran because the explicit
//! gate order requires complete implementation before hf-build and the sole hf-done verify.
//! First qualification (829b9f917acd5ec9d654cb2efe51321a48a23192) failed all eight
//! cases with BAD_VERSION (/out/logs/verify-1.log): raw task/evidence/slot/report APIs
//! received the joint F07/F08 section. The repair routes the remaining task/evidence
//! producers and uses the production carry_reputation adapter for low-level feature reads.
//! The verified revision, command, exit and private evidence log are recorded by hf-done.

#[path = "f04_recovery.rs"]
mod recovery;

#[test]
fn f04_admission_fixture_routes_admission_replay_and_aggregation() {
    recovery::a12_rollover_binds_epoch_eight_and_historical_retries_recreate_nothing()
        .expect("routed admission, replay sequences, aggregation and rollover");
}

#[test]
fn f04_reopen_fixture_recovers_and_retries_the_persisted_reveal() {
    recovery::a09_crash_after_finalized_commit_reveals_from_the_journal()
        .expect("reopened durable client and committed market replay");
}
