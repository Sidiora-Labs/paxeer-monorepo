use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::fmt::{Display, Formatter};
use std::fs::{self, File, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};

/// Deployment contract for deterministic multi-instance counters.
pub const COUNTER_CONSISTENCY_MODEL: &str = "Every daemon limiter instance serving a tenant must share one linearizable CounterLedger. Windows are selected from authenticated request logical_time_ms, never an instance-local clock, and all applicable counters are checked and incremented in one transaction. Independent ledgers are unsupported because they would permit counter divergence.";

/// Stable operator-facing identifier of one configured limit.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct LimitId(String);

impl LimitId {
    /// Creates a bounded operator-facing limit identifier.
    ///
    /// # Errors
    ///
    /// Returns `InvalidConfiguration` for an empty, over-255-byte, or NUL-bearing value.
    pub fn new(value: impl Into<String>) -> Result<Self, Refusal> {
        let value = value.into();
        if value.is_empty() || value.len() > 255 || value.as_bytes().contains(&0) {
            Err(Refusal::InvalidConfiguration)
        } else {
            Ok(Self(value))
        }
    }

    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

/// Every independently configurable rate-limit dimension.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub enum LimitScope {
    Tenant {
        tenant: String,
    },
    Agent {
        tenant: String,
        agent: String,
    },
    Session {
        tenant: String,
        session: String,
    },
    Capability {
        tenant: String,
        capability: String,
    },
    OperationClass {
        tenant: String,
        operation_class: String,
    },
}

impl LimitScope {
    fn tenant(&self) -> &str {
        match self {
            Self::Tenant { tenant }
            | Self::Agent { tenant, .. }
            | Self::Session { tenant, .. }
            | Self::Capability { tenant, .. }
            | Self::OperationClass { tenant, .. } => tenant,
        }
    }

    fn matches(&self, request: &RateRequest) -> bool {
        match self {
            Self::Tenant { tenant } => tenant == &request.tenant,
            Self::Agent { tenant, agent } => tenant == &request.tenant && agent == &request.agent,
            Self::Session { tenant, session } => {
                tenant == &request.tenant && session == &request.session
            }
            Self::Capability { tenant, capability } => {
                tenant == &request.tenant && capability == &request.capability
            }
            Self::OperationClass {
                tenant,
                operation_class,
            } => tenant == &request.tenant && operation_class == &request.operation_class,
        }
    }

    fn valid(&self) -> bool {
        let valid =
            |value: &str| !value.is_empty() && value.len() <= 255 && !value.as_bytes().contains(&0);
        match self {
            Self::Tenant { tenant } => valid(tenant),
            Self::Agent { tenant, agent } => valid(tenant) && valid(agent),
            Self::Session { tenant, session } => valid(tenant) && valid(session),
            Self::Capability { tenant, capability } => valid(tenant) && valid(capability),
            Self::OperationClass {
                tenant,
                operation_class,
            } => valid(tenant) && valid(operation_class),
        }
    }
}

/// One finite fixed-window rate configuration.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct LimitConfig {
    pub id: LimitId,
    pub scope: LimitScope,
    pub limit: u64,
    pub window_ms: u64,
}

/// Complete authenticated request dimensions and authoritative logical time.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RateRequest {
    pub tenant: String,
    pub agent: String,
    pub session: String,
    pub capability: String,
    pub operation_class: String,
    pub logical_time_ms: u64,
    pub cost: u64,
}

impl RateRequest {
    fn valid(&self) -> bool {
        self.cost > 0
            && [
                self.tenant.as_str(),
                self.agent.as_str(),
                self.session.as_str(),
                self.capability.as_str(),
                self.operation_class.as_str(),
            ]
            .iter()
            .all(|value| !value.is_empty() && value.len() <= 255 && !value.as_bytes().contains(&0))
    }
}

/// Exact fixed window selected deterministically from logical request time.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct Window {
    pub start_ms: u64,
    pub end_ms: u64,
}

/// Successful atomic application of every matching limit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Admission {
    pub applied_limits: Vec<LimitId>,
    pub ledger_revision: u64,
}

/// Current configuration and counter usage for one tenant limit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Utilization {
    pub config: LimitConfig,
    pub window: Window,
    pub used: u64,
    pub remaining: u64,
    pub ledger_revision: u64,
}

/// Typed refusal returned immediately instead of queuing excess work.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Refusal {
    Exceeded {
        limit: Box<LimitConfig>,
        window: Window,
        remaining: u64,
        retry_after_ms: u64,
    },
    StaleWindow {
        limit: Box<LimitConfig>,
        window: Window,
        remaining: u64,
        retry_after_ms: u64,
    },
    InvalidConfiguration,
    InvalidRequest,
    NoApplicableLimits,
    Arithmetic,
    CounterUnavailable,
}

impl Display for Refusal {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Exceeded {
                limit,
                window,
                remaining,
                retry_after_ms,
            } => write!(
                formatter,
                "rate limit {} exceeded in {}..{}; remaining {remaining}; retry after {retry_after_ms}ms",
                limit.id.as_str(),
                window.start_ms,
                window.end_ms
            ),
            Self::StaleWindow { window, .. } => write!(formatter, "rate observation precedes durable window {}..{}", window.start_ms, window.end_ms),
            Self::InvalidConfiguration => formatter.write_str("rate limit configuration is invalid"),
            Self::InvalidRequest => formatter.write_str("rate limit request is invalid"),
            Self::NoApplicableLimits => formatter.write_str("request has no applicable rate limits"),
            Self::Arithmetic => formatter.write_str("rate limit arithmetic overflow"),
            Self::CounterUnavailable => formatter.write_str("shared rate counters are unavailable"),
        }
    }
}

impl std::error::Error for Refusal {}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct Counter {
    config: LimitConfig,
    window: Window,
    used: u64,
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
struct CounterState {
    revision: u64,
    used: BTreeMap<LimitId, Counter>,
}

#[derive(Debug)]
enum CounterAuthority {
    Memory(Mutex<CounterState>),
    Durable(PathBuf),
}

/// A shared authority. Production owners open the same durable root in every instance.
#[derive(Clone, Debug)]
pub struct CounterLedger(Arc<CounterAuthority>);

impl Default for CounterLedger {
    fn default() -> Self {
        Self(Arc::new(CounterAuthority::Memory(Mutex::new(
            CounterState::default(),
        ))))
    }
}

impl CounterLedger {
    #[must_use]
    pub fn shared() -> Self {
        Self::default()
    }

    /// Opens and validates the durable authority before it can admit requests.
    /// A permanent lock inode serializes independent opens, including other processes.
    /// # Errors
    /// Returns an error if the persisted rate-limit state or configuration cannot be restored.
    pub fn open(root: impl AsRef<Path>) -> Result<Self, Refusal> {
        fs::create_dir_all(root.as_ref()).map_err(|_| Refusal::CounterUnavailable)?;
        let ledger = Self(Arc::new(CounterAuthority::Durable(
            root.as_ref().to_path_buf(),
        )));
        ledger.transaction(false, |_| Ok(()))?;
        Ok(ledger)
    }

    fn transaction<T>(
        &self,
        write: bool,
        operation: impl FnOnce(&mut CounterState) -> Result<T, Refusal>,
    ) -> Result<T, Refusal> {
        match self.0.as_ref() {
            CounterAuthority::Memory(state) => {
                let mut state = state.lock().map_err(|_| Refusal::CounterUnavailable)?;
                let mut next = state.clone();
                let result = operation(&mut next)?;
                if write {
                    *state = next;
                }
                Ok(result)
            }
            CounterAuthority::Durable(root) => {
                let lock = OpenOptions::new()
                    .create(true)
                    .truncate(false)
                    .read(true)
                    .write(true)
                    .open(root.join("counter.lock"))
                    .map_err(|_| Refusal::CounterUnavailable)?;
                lock.lock().map_err(|_| Refusal::CounterUnavailable)?;
                let path = root.join("counters.json");
                let mut state: CounterState = match fs::read(&path) {
                    Ok(bytes) => {
                        serde_json::from_slice(&bytes).map_err(|_| Refusal::CounterUnavailable)?
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                        CounterState::default()
                    }
                    Err(_) => return Err(Refusal::CounterUnavailable),
                };
                if state.used.iter().any(|(id, counter)| {
                    id != &counter.config.id
                        || counter.config.limit == 0
                        || counter.config.window_ms == 0
                        || !counter.config.scope.valid()
                        || counter.used == 0
                        || counter.used > counter.config.limit
                        || window(counter.window.start_ms, counter.config.window_ms)
                            != Ok(counter.window)
                }) {
                    return Err(Refusal::CounterUnavailable);
                }
                let result = operation(&mut state)?;
                // Initialize even on open: a missing authority is never confused with a corrupt one.
                if write || !path.exists() {
                    let bytes =
                        serde_json::to_vec(&state).map_err(|_| Refusal::CounterUnavailable)?;
                    let temporary = root.join("counters.next");
                    let mut file =
                        File::create(&temporary).map_err(|_| Refusal::CounterUnavailable)?;
                    file.write_all(&bytes)
                        .and_then(|()| file.sync_all())
                        .map_err(|_| Refusal::CounterUnavailable)?;
                    fs::rename(&temporary, &path).map_err(|_| Refusal::CounterUnavailable)?;
                    File::open(root)
                        .and_then(|file| file.sync_all())
                        .map_err(|_| Refusal::CounterUnavailable)?;
                }
                Ok(result)
            }
        }
    }
}

/// Deterministic layered limiter backed by a shared atomic counter ledger.
#[derive(Clone, Debug)]
pub struct RateLimiter {
    configs: Vec<LimitConfig>,
    ledger: CounterLedger,
}

impl RateLimiter {
    /// Builds a limiter over an id-sorted configuration set sharing one ledger.
    ///
    /// # Errors
    ///
    /// Returns `InvalidConfiguration` for an empty set, a duplicate `LimitId`, a zero limit or
    /// window, or a scope with an empty, oversized or NUL-bearing component.
    pub fn new(configs: Vec<LimitConfig>, ledger: CounterLedger) -> Result<Self, Refusal> {
        let mut configs = configs;
        configs.sort_by(|left, right| left.id.cmp(&right.id));
        if configs.is_empty()
            || configs
                .iter()
                .any(|config| config.limit == 0 || config.window_ms == 0 || !config.scope.valid())
            || configs.windows(2).any(|pair| pair[0].id == pair[1].id)
        {
            return Err(Refusal::InvalidConfiguration);
        }
        Ok(Self { configs, ledger })
    }

    #[must_use]
    pub fn is_durable(&self) -> bool {
        matches!(self.ledger.0.as_ref(), CounterAuthority::Durable(_))
    }

    /// Atomically checks and increments every applicable scope counter.
    ///
    /// # Errors
    ///
    /// Returns `InvalidRequest` for zero cost or a malformed dimension, `NoApplicableLimits`
    /// when no scope matches, `CounterUnavailable` on a poisoned ledger, `Exceeded` with the
    /// window and `retry_after_ms`, or `Arithmetic` on revision, counter or window overflow.
    pub fn admit(&self, request: &RateRequest) -> Result<Admission, Refusal> {
        if !request.valid() {
            return Err(Refusal::InvalidRequest);
        }
        let applicable: Vec<&LimitConfig> = self
            .configs
            .iter()
            .filter(|config| config.scope.matches(request))
            .collect();
        if applicable.is_empty() {
            return Err(Refusal::NoApplicableLimits);
        }
        self.ledger.transaction(true, |ledger| {
            let next_revision = ledger.revision.checked_add(1).ok_or(Refusal::Arithmetic)?;
            for config in &applicable {
                let selected = window(request.logical_time_ms, config.window_ms)?;
                let current = ledger.used.get(&config.id);
                if let Some(counter) = current {
                    if counter.config != **config {
                        return Err(Refusal::InvalidConfiguration);
                    }
                    if selected.start_ms < counter.window.start_ms {
                        return Err(Refusal::StaleWindow {
                            limit: Box::new((*config).clone()),
                            window: counter.window,
                            remaining: config.limit.saturating_sub(counter.used),
                            retry_after_ms: counter
                                .window
                                .end_ms
                                .saturating_sub(request.logical_time_ms),
                        });
                    }
                }
                let used = current
                    .filter(|counter| counter.window == selected)
                    .map_or(0, |counter| counter.used);
                let projected = used.checked_add(request.cost).ok_or(Refusal::Arithmetic)?;
                if projected > config.limit {
                    return Err(Refusal::Exceeded {
                        limit: Box::new((*config).clone()),
                        window: selected,
                        remaining: config.limit.saturating_sub(used),
                        retry_after_ms: selected.end_ms.saturating_sub(request.logical_time_ms),
                    });
                }
            }
            for config in &applicable {
                let selected = window(request.logical_time_ms, config.window_ms)?;
                let used = ledger
                    .used
                    .get(&config.id)
                    .filter(|counter| counter.window == selected)
                    .map_or(0, |counter| counter.used);
                ledger.used.insert(
                    config.id.clone(),
                    Counter {
                        config: (*config).clone(),
                        window: selected,
                        used: used.checked_add(request.cost).ok_or(Refusal::Arithmetic)?,
                    },
                );
            }
            ledger.revision = next_revision;
            Ok(Admission {
                applied_limits: applicable.iter().map(|config| config.id.clone()).collect(),
                ledger_revision: ledger.revision,
            })
        })
    }

    /// Returns live per-limit configuration and current-window utilization for one tenant.
    ///
    /// # Errors
    ///
    /// Returns `CounterUnavailable` when the shared ledger lock is poisoned, or `Arithmetic`
    /// if a configured window end overflows `u64`.
    pub fn utilization(
        &self,
        tenant: &str,
        logical_time_ms: u64,
    ) -> Result<Vec<Utilization>, Refusal> {
        self.ledger.transaction(false, |ledger| {
            self.configs
                .iter()
                .filter(|config| config.scope.tenant() == tenant)
                .map(|config| {
                    let selected = window(logical_time_ms, config.window_ms)?;
                    let current = ledger.used.get(&config.id);
                    if current.is_some_and(|counter| counter.config != *config) {
                        return Err(Refusal::InvalidConfiguration);
                    }
                    let used = current
                        .filter(|counter| counter.window == selected)
                        .map_or(0, |counter| counter.used);
                    Ok(Utilization {
                        config: config.clone(),
                        window: selected,
                        used,
                        remaining: config.limit.saturating_sub(used),
                        ledger_revision: ledger.revision,
                    })
                })
                .collect()
        })
    }
}

fn window(logical_time_ms: u64, window_ms: u64) -> Result<Window, Refusal> {
    let start_ms = logical_time_ms / window_ms * window_ms;
    let end_ms = start_ms.checked_add(window_ms).ok_or(Refusal::Arithmetic)?;
    Ok(Window { start_ms, end_ms })
}
