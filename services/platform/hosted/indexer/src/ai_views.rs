//! The AI query consumer shared by authenticated gateway routes and projection recovery.
use crate::ai_market::{
    parse_epoch_query, parse_snapshot_query, Availability, CursorKeyring, EpochStatus, Freshness,
    PageRequest, ParticipantKind, ParticipantRow, Presence, ProjectionState, ProjectionStore,
    QueryLimiter, ScoreStatus, ViewError, Viewer,
};
use crate::codec::hex0x;
use serde_json::{json, Value};
use std::path::Path;

/// Durable projection reader and bounded per-principal query admission.
pub struct AiViews {
    pub store: ProjectionStore,
    pub cursor_keys: CursorKeyring,
    pub limiter: QueryLimiter,
}
impl AiViews {
    pub fn open(path: &Path, generation: u32, cursor_secret: [u8; 32]) -> Result<Self, ViewError> {
        Ok(Self {
            store: ProjectionStore::open(path)?,
            cursor_keys: CursorKeyring::new(generation, cursor_secret),
            limiter: QueryLimiter::new(Some(crate::ai_market::DEFAULT_RATE)),
        })
    }

    /// Invoke the gateway's current credential lookup on every request, before
    /// limiter admission or projection reads. Authentication remains owned by the gateway.
    pub fn authenticated_query<E>(
        &self,
        authenticate: impl FnOnce() -> Result<String, E>,
        view: &str,
        market: &str,
        query: Option<&str>,
        verified_authority_height: Option<u64>,
        now_ms: u64,
    ) -> Result<Result<Value, ViewError>, E> {
        let principal = authenticate()?;
        Ok(Viewer::for_principal(&principal).and_then(|viewer| {
            self.query(
                &viewer,
                view,
                market,
                query,
                verified_authority_height,
                now_ms,
            )
        }))
    }

    /// The caller supplies a viewer only after reauthenticating the current request.
    pub fn query(
        &self,
        viewer: &Viewer,
        view: &str,
        market: &str,
        query: Option<&str>,
        verified_authority_height: Option<u64>,
        now_ms: u64,
    ) -> Result<Value, ViewError> {
        let views = self;
        let _permit = self.limiter.admit(viewer, now_ms)?;
        match view {
            "snapshot" => parse_snapshot_query(market, query).and_then(|(market, selector)| {
                views
                    .store
                    .market_view(
                        viewer,
                        &market,
                        selector.as_ref(),
                        verified_authority_height,
                    )
                    .map(|v| {
                        json!({
                            "snapshot_id": hex0x(v.snapshot_id.as_bytes()),
                            "projection": projection(v.projection),
                            "committed": v.projection.finalized(),
                            "binding": binding(&v.binding),
                            "components": components(&v.availability),
                            "source_activity": hex0x(v.source_activity.as_bytes()),
                            "freshness": freshness(v.freshness),
                        })
                    })
            }),
            "participants" => PageRequest::parse(market, query).and_then(|request| {
                views
                    .store
                    .participants(viewer, &views.cursor_keys, &request, now_ms)
                    .map(|page| {
                        json!({
                            "snapshot_id": hex0x(page.snapshot_id.as_bytes()),
                            "binding": binding(&page.binding),
                            "components": components(&page.availability),
                            "rows": page.rows.iter().map(row).collect::<Vec<_>>(),
                            "cursor": page.cursor,
                        })
                    })
            }),
            "epochs" => parse_epoch_query(market, query).and_then(|(market, from, limit)| {
                views.store.epochs(&market, from, limit).map(|page| {
                    json!({
                        "component": availability(page.component),
                        "entries": page.entries.iter().map(|e| json!({
                            "epoch": e.epoch.to_string(),
                            "status": epoch_status(e.status),
                            "snapshot_id": e.snapshot_id.map(|id| hex0x(id.as_bytes())),
                        })).collect::<Vec<_>>(),
                    })
                })
            }),
            _ => Err(ViewError::InvalidEncoding),
        }
    }
}

fn present<T>(value: Presence<T>, render: impl FnOnce(T) -> Value) -> Value {
    match value {
        Presence::Present(v) => render(v),
        Presence::Absent => Value::Null,
    }
}

fn binding(b: &crate::ai_market::SnapshotBinding) -> Value {
    json!({
        "chain": hex0x(b.chain.as_bytes()),
        "program": hex0x(b.program.as_bytes()),
        "market": hex0x(b.market.as_bytes()),
        "observed_sequence": b.observed_sequence.to_string(),
        "execution_height": b.execution_height.to_string(),
        "batch_id": hex0x(b.batch_id.as_bytes()),
        "native_state_root": hex0x(b.native_state_root.as_bytes()),
        "revision": b.revision.to_string(),
        "state_digest": hex0x(b.state_digest.as_bytes()),
        "epoch": present(b.epoch, |e| json!(e.to_string())),
        "config": b.config.get().to_string(),
        "policy": hex0x(b.policy.as_bytes()),
        "roster": present(b.roster, |r| json!(hex0x(r.as_bytes()))),
        "block_height": b.block_height.to_string(),
        "block_hash": hex0x(b.block_hash.as_bytes()),
        "app_hash": hex0x(b.app_hash.as_bytes()),
        "store_proof_digest": hex0x(b.store_proof_digest.as_bytes()),
        "rank": b.rank,
        "publication_time_ms": b.publication_time_ms.to_string(),
    })
}

fn row(r: &ParticipantRow) -> Value {
    json!({
        "kind": match r.kind {
            ParticipantKind::Worker => "worker",
            ParticipantKind::Evaluator => "evaluator",
        },
        "id": hex0x(&r.id),
        "owner": hex0x(r.owner.as_bytes()),
        "generation": r.generation.to_string(),
        "identity_state": r.identity_state,
        "frozen_member": r.frozen_member,
        "frozen_generation": present(r.frozen_generation, |g| json!(g.to_string())),
        "eligibility": r.eligibility,
        "metadata": present(r.metadata, |m| json!(hex0x(m.as_bytes()))),
        "metadata_revision": r.metadata_revision.to_string(),
        "score": {
            "status": score_status(r.score.status()),
            "epoch": present(r.score.epoch(), |e| json!(e.to_string())),
            "ppm": present(r.score.ppm(), |p| json!(p)),
        },
        "reward": {
            "status": availability(r.reward.status()),
            "asset": present(r.reward.asset(), |a| json!(hex0x(a.as_bytes()))),
            "earned": present(r.reward.earned(), |v| json!(v.to_string())),
            "claimed": present(r.reward.claimed(), |v| json!(v.to_string())),
        },
        "history": {
            "status": availability(r.history_status),
            "digest": present(r.history, |h| json!(hex0x(h.as_bytes()))),
        },
    })
}

const FEATURE_NAMES: [&str; 10] = [
    "F01", "F02", "F03", "F04", "F05", "F06", "F07", "F08", "F09", "F10",
];

fn components(statuses: &[Availability; 10]) -> Value {
    Value::Object(
        FEATURE_NAMES
            .iter()
            .zip(statuses)
            .map(|(name, status)| ((*name).to_owned(), json!(availability(*status))))
            .collect(),
    )
}

const fn availability(status: Availability) -> &'static str {
    match status {
        Availability::Available => "available",
        Availability::NotEnabled => "not-enabled",
        Availability::NotYetProduced => "not-yet-produced",
        Availability::ContentUnavailable => "content-unavailable",
        Availability::UnsupportedVersion => "unsupported-version",
    }
}

const fn score_status(status: ScoreStatus) -> &'static str {
    match status {
        ScoreStatus::Present => "present",
        ScoreStatus::NoAdmissibleScore => "no-admissible-score",
        ScoreStatus::InsufficientCoverage => "insufficient-coverage",
        ScoreStatus::NotProduced => "not-produced",
        ScoreStatus::Unavailable => "unavailable",
        ScoreStatus::Unsupported => "unsupported",
    }
}

const fn projection(state: ProjectionState) -> &'static str {
    match state {
        ProjectionState::ObservedUnverified => "observed-unverified",
        ProjectionState::EvidenceVerifiedUnfinalized => "evidence-verified-unfinalized",
        ProjectionState::FinalizedPublishable => "finalized-publishable",
        ProjectionState::Archived => "archived",
        ProjectionState::Pruned => "pruned",
        ProjectionState::Quarantined => "quarantined",
    }
}

const fn epoch_status(status: EpochStatus) -> &'static str {
    match status {
        EpochStatus::Retained => "retained",
        EpochStatus::NeverOpened => "never-opened",
        EpochStatus::RetainedTerminal => "retained-terminal",
        EpochStatus::ArchiveRequired => "archive-required",
        EpochStatus::ArchiveUnavailable => "archive-unavailable",
        EpochStatus::UnsupportedVersion => "unsupported-version",
    }
}

fn freshness(value: Freshness) -> Value {
    match value {
        Freshness::Current => json!({"label": "current"}),
        Freshness::Stale { lag } => json!({"label": "stale", "lag": lag.to_string()}),
        Freshness::Unknown => json!({"label": "unknown"}),
    }
}
