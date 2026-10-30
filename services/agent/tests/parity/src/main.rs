use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Barrier};
use std::thread;
use std::time::Duration;

use layerx_agent_api::identity::TenantId as SdkTenantId;
use layerx_agent_api::read::{AccountRef, BalanceValue, Freshness, RelativeTo, VerifiedRead};
use layerx_agent_api::track::{SubmissionRef, TrackRequest};
use layerx_agent_api::verify::Rung;
use layerx_agent_api::{agent_api_schema_v1, Amount, SubmissionState};
use layerx_agentd::idempotency::{
    EconomicResult, IdempotencyError, Outcome, RetentionPolicy, Store as IdempotencyStore,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::store::TenantId;
use layerx_sdk::approval::{
    ApprovalApproveRequest, ApprovalDecisionOutcome, ApprovalEventKind, ApprovalGetRequest,
    ApprovalId, ApprovalListRequest, ApprovalRejectRequest, DecisionKey,
};
use layerx_sdk::{Client as RustSdkClient, Deployment, Operation};
use layerx_types::result::ResultCode;
use serde_json::{json, Value};
use url::Url;

#[derive(Clone, Debug, Eq, PartialEq)]
struct Observation {
    state: String,
    error: String,
    verification: Rung,
    idempotency: String,
    result_code: String,
    receipt_count: usize,
    economic_effects: usize,
}

impl Observation {
    fn encode(&self) -> String {
        format!(
            "state={};error={};verification={};idempotency={};result_code={};receipt_count={};economic_effects={}",
            self.state,
            self.error,
            self.verification.name(),
            self.idempotency,
            self.result_code,
            self.receipt_count,
            self.economic_effects
        )
    }

    fn decode(encoded: &str) -> Result<Self, String> {
        let fields = encoded
            .split(';')
            .map(|field| {
                field
                    .split_once('=')
                    .map(|(key, value)| (key.to_owned(), value.to_owned()))
                    .ok_or_else(|| format!("malformed observation field {field}"))
            })
            .collect::<Result<BTreeMap<_, _>, _>>()?;
        let required = |key: &str| {
            fields
                .get(key)
                .cloned()
                .ok_or_else(|| format!("observation missing {key}"))
        };
        Ok(Self {
            state: required("state")?,
            error: required("error")?,
            verification: parse_rung(&required("verification")?)?,
            idempotency: required("idempotency")?,
            result_code: required("result_code")?,
            receipt_count: required("receipt_count")?
                .parse()
                .map_err(|_| "invalid receipt_count".to_owned())?,
            economic_effects: required("economic_effects")?
                .parse()
                .map_err(|_| "invalid economic_effects".to_owned())?,
        })
    }
}

fn unquote(value: &str) -> Result<String, String> {
    value
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
        .map(str::to_owned)
        .ok_or_else(|| format!("expected quoted value, got {value}"))
}

fn parse_scenarios(path: &Path) -> Result<BTreeMap<String, Observation>, String> {
    let source = fs::read_to_string(path)
        .map_err(|error| format!("could not read {}: {error}", path.display()))?;
    let mut current = None;
    let mut fields: BTreeMap<String, BTreeMap<String, String>> = BTreeMap::new();
    for raw in source.lines() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if let Some(section) = line
            .strip_prefix("[scenario.")
            .and_then(|value| value.strip_suffix(']'))
        {
            current = Some(section.to_owned());
            fields.entry(section.to_owned()).or_default();
            continue;
        }
        if line.starts_with('[') {
            current = None;
            continue;
        }
        let Some(scenario) = current.as_ref() else {
            continue;
        };
        let (key, value) = line
            .split_once('=')
            .ok_or_else(|| format!("malformed scenario declaration {line}"))?;
        if fields
            .get_mut(scenario)
            .ok_or_else(|| "scenario parser lost section".to_owned())?
            .insert(key.trim().to_owned(), value.trim().to_owned())
            .is_some()
        {
            return Err(format!(
                "scenario {scenario}: duplicate field {}",
                key.trim()
            ));
        }
    }
    fields
        .into_iter()
        .map(|(name, values)| {
            let string = |key: &str| {
                values
                    .get(key)
                    .ok_or_else(|| format!("scenario {name} missing {key}"))
                    .and_then(|value| unquote(value))
            };
            let number = |key: &str| {
                values
                    .get(key)
                    .ok_or_else(|| format!("scenario {name} missing {key}"))?
                    .parse::<usize>()
                    .map_err(|_| format!("scenario {name} has invalid {key}"))
            };
            let observation = Observation {
                state: string("state")?,
                error: string("error")?,
                verification: parse_rung(&string("verification")?)
                    .map_err(|error| format!("scenario {name}: {error}"))?,
                idempotency: string("idempotency")?,
                result_code: string("result_code")?,
                receipt_count: number("receipt_count")?,
                economic_effects: number("economic_effects")?,
            };
            Ok((name, observation))
        })
        .collect()
}

fn parse_rung(name: &str) -> Result<Rung, String> {
    Rung::from_name(name).ok_or_else(|| {
        format!("invalid verification rung {name:?}; expected submitted, receipted or committed")
    })
}

fn node_json_rpc_url(value: &str) -> Result<Url, String> {
    let invalid = || {
        "expected a node JSON-RPC URL; parity suite requires an http or https node JSON-RPC URL"
            .to_owned()
    };
    if value.trim() != value
        || value.chars().any(char::is_control)
        || !(value.starts_with("http://") || value.starts_with("https://"))
    {
        return Err(invalid());
    }
    let url = Url::parse(value).map_err(|_| invalid())?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.fragment().is_some()
    {
        return Err(invalid());
    }
    Ok(url)
}

fn qualify_node(url: &Url) -> Result<Freshness, String> {
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(2)))
        .max_redirects(0)
        .build()
        .into();
    let mut response = agent
        .post(url.as_str())
        .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []}))
        .map_err(|error| format!("node JSON-RPC chain id request failed: {error}"))?;
    let bytes = response
        .body_mut()
        .with_config()
        .limit(65_536)
        .read_to_vec()
        .map_err(|error| format!("node JSON-RPC chain id response failed: {error}"))?;
    let response: Value = serde_json::from_slice(&bytes)
        .map_err(|error| format!("node JSON-RPC chain id response is invalid: {error}"))?;
    if response["jsonrpc"] != "2.0" || response["id"] != 1 || response.get("error").is_some() {
        return Err("node JSON-RPC refused the chain id request".to_owned());
    }
    let chain_id = response["result"]
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .filter(|digits| {
            !digits.is_empty()
                && (digits.len() == 1 || !digits.starts_with('0'))
                && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
        })
        .and_then(|digits| u64::from_str_radix(digits, 16).ok())
        .ok_or_else(|| "node JSON-RPC returned an invalid chain id".to_owned())?;
    let node = NodeRpc::connect(&NodeRpcConfig {
        url: url.to_string(),
        chain_id,
        deadline: Duration::from_secs(2),
    })
    .map_err(|error| format!("node JSON-RPC connection failed: {error}"))?;
    let height = node
        .block_number()
        .map_err(|error| format!("node JSON-RPC head failed: {error}"))?;
    let block = node
        .block_by_number(height)
        .map_err(|error| format!("node JSON-RPC block failed: {error}"))?
        .ok_or_else(|| "node JSON-RPC head block is unavailable".to_owned())?;
    if block.number != height || block.hash == [0; 32] {
        return Err("node JSON-RPC returned a mismatched head block".to_owned());
    }
    node.receipt_keys()
        .map_err(|error| format!("node JSON-RPC receipt keys failed: {error}"))?;
    Ok(Freshness {
        chain_head: height,
        value_height: height,
        relative_to: RelativeTo::Block {
            height,
            hash: block.hash,
        },
    })
}

fn tenant() -> Result<TenantId, String> {
    TenantId::new("parity-tenant").map_err(|error| format!("tenant: {error}"))
}

fn retention() -> Result<RetentionPolicy, String> {
    RetentionPolicy::new(100, 50).map_err(|error| format!("retention: {error:?}"))
}

fn economic_result() -> EconomicResult {
    EconomicResult {
        response_bytes: b"executed-once".to_vec(),
        receipt_ref: Some([0x77; 32]),
    }
}

fn qualify_idempotency(
    directory: &Path,
    expected: &BTreeMap<String, Observation>,
) -> Result<(), String> {
    let repeated_root = directory.join("idempotency-repeat");
    let effects = AtomicUsize::new(0);
    let store = IdempotencyStore::open(&repeated_root, tenant()?, retention()?)
        .map_err(|error| format!("repeat store: {error:?}"))?;
    let first = store
        .execute([1; 32], b"same-intent", 10, |_| {
            effects.fetch_add(1, Ordering::SeqCst);
            Ok(economic_result())
        })
        .map_err(|error| format!("first retry scenario: {error:?}"))?;
    let repeated = store
        .execute([1; 32], b"same-intent", 11, |_| {
            Err("duplicate executed".to_owned())
        })
        .map_err(|error| format!("repeated retry scenario: {error:?}"))?;
    if !matches!(first, Outcome::First(_))
        || !matches!(repeated, Outcome::RepeatedOriginal(_))
        || effects.load(Ordering::SeqCst) != 1
    {
        return Err("repeated idempotency produced more than one effect".to_owned());
    }
    assert_idempotency(expected, "idempotency_repeat", "RepeatedOriginal")?;

    let concurrent_root = directory.join("idempotency-concurrent");
    let store = Arc::new(
        IdempotencyStore::open(&concurrent_root, tenant()?, retention()?)
            .map_err(|error| format!("concurrent store: {error:?}"))?,
    );
    let barrier = Arc::new(Barrier::new(16));
    let effects = Arc::new(AtomicUsize::new(0));
    let workers = (0..16)
        .map(|_| {
            let store = Arc::clone(&store);
            let barrier = Arc::clone(&barrier);
            let effects = Arc::clone(&effects);
            thread::spawn(move || {
                barrier.wait();
                store.execute([2; 32], b"concurrent-intent", 20, |_| {
                    effects.fetch_add(1, Ordering::SeqCst);
                    Ok(economic_result())
                })
            })
        })
        .collect::<Vec<_>>();
    let outcomes = workers
        .into_iter()
        .map(|worker| {
            worker
                .join()
                .map_err(|_| "concurrent idempotency worker panicked".to_owned())?
                .map_err(|error| format!("concurrent idempotency: {error:?}"))
        })
        .collect::<Result<Vec<_>, _>>()?;
    if effects.load(Ordering::SeqCst) != 1
        || outcomes
            .iter()
            .filter(|outcome| matches!(outcome, Outcome::First(_)))
            .count()
            != 1
    {
        return Err("concurrent idempotency produced more than one effect".to_owned());
    }
    assert_idempotency(expected, "idempotency_concurrent", "Deduplicated")?;

    let restart_root = directory.join("idempotency-restart");
    {
        let store = IdempotencyStore::open(&restart_root, tenant()?, retention()?)
            .map_err(|error| format!("restart store: {error:?}"))?;
        if !matches!(
            store.execute([3; 32], b"restart-intent", 30, |_| Err(
                "transport lost".to_owned()
            )),
            Err(IdempotencyError::Operation(_))
        ) {
            return Err("pre-restart submission did not remain unresolved".to_owned());
        }
    }
    let effects = AtomicUsize::new(0);
    let store = IdempotencyStore::open(&restart_root, tenant()?, retention()?)
        .map_err(|error| format!("reopened store: {error:?}"))?;
    if store
        .restore(&[[3; 32]])
        .map_err(|error| format!("restore idempotency: {error:?}"))?
        != 1
    {
        return Err("post-restart idempotency record was not restored".to_owned());
    }
    let restored = store
        .execute([3; 32], b"restart-intent", 31, |attempt| {
            if !attempt.retry || attempt.exact_request_bytes != b"restart-intent" {
                return Err("restart changed exact request bytes".to_owned());
            }
            effects.fetch_add(1, Ordering::SeqCst);
            Ok(economic_result())
        })
        .map_err(|error| format!("post-restart retry: {error:?}"))?;
    if !matches!(restored, Outcome::RepeatedOriginal(_)) || effects.load(Ordering::SeqCst) != 1 {
        return Err("post-restart retry duplicated the effect".to_owned());
    }
    assert_idempotency(expected, "idempotency_restart", "RestoredOriginal")
}

fn assert_idempotency(
    expected: &BTreeMap<String, Observation>,
    scenario: &str,
    idempotency: &str,
) -> Result<(), String> {
    assert_scenario(
        expected,
        scenario,
        &Observation {
            state: "Executed".to_owned(),
            error: "None".to_owned(),
            verification: Rung::Receipted,
            idempotency: idempotency.to_owned(),
            result_code: "0".to_owned(),
            receipt_count: 1,
            economic_effects: 1,
        },
    )
}

fn qualify_state_semantics(expected: &BTreeMap<String, Observation>) -> Result<(), String> {
    if SubmissionState::Unknown.name() != "unknown" {
        return Err("Rust contract collapsed unknown state".to_owned());
    }
    assert_scenario(
        expected,
        "unknown_submission",
        &Observation {
            state: "Unknown".to_owned(),
            error: "None".to_owned(),
            verification: Rung::Submitted,
            idempotency: "Original".to_owned(),
            result_code: "none".to_owned(),
            receipt_count: 0,
            economic_effects: 0,
        },
    )?;
    let result = ResultCode::from_raw(-77_777);
    if result.raw() != -77_777 {
        return Err("Rust contract changed a future result code".to_owned());
    }
    assert_scenario(
        expected,
        "terminal_rejection",
        &Observation {
            state: "Failed".to_owned(),
            error: "CoreRejection".to_owned(),
            verification: Rung::Submitted,
            idempotency: "Original".to_owned(),
            result_code: result.raw().to_string(),
            receipt_count: 0,
            economic_effects: 0,
        },
    )
}

fn assert_scenario(
    expected: &BTreeMap<String, Observation>,
    scenario: &str,
    actual: &Observation,
) -> Result<(), String> {
    let configured = expected
        .get(scenario)
        .ok_or_else(|| format!("scenario {scenario} is missing"))?;
    if configured == actual {
        Ok(())
    } else {
        Err(format!(
            "scenario {scenario} diverged while qualifying daemon/node: expected={} observed={}",
            configured.encode(),
            actual.encode()
        ))
    }
}

fn serve_daemon(
    listener: UnixListener,
    observations: BTreeMap<String, Observation>,
    expected_requests: usize,
) -> Result<BTreeSet<(String, String)>, String> {
    let mut seen = BTreeSet::new();
    for incoming in listener.incoming().take(expected_requests) {
        let mut stream = incoming.map_err(|error| format!("daemon accept failed: {error}"))?;
        let mut request = String::new();
        BufReader::new(
            stream
                .try_clone()
                .map_err(|error| format!("daemon clone failed: {error}"))?,
        )
        .read_line(&mut request)
        .map_err(|error| format!("daemon request failed: {error}"))?;
        let (language, scenario) = request
            .trim_end()
            .split_once('\t')
            .ok_or_else(|| "daemon request omitted language or scenario".to_owned())?;
        let observation = observations
            .get(scenario)
            .ok_or_else(|| format!("unknown parity scenario {scenario}"))?;
        if !seen.insert((language.to_owned(), scenario.to_owned())) {
            return Err(format!("duplicate parity request {language}/{scenario}"));
        }
        writeln!(stream, "{}", observation.encode())
            .map_err(|error| format!("daemon response failed: {error}"))?;
    }
    Ok(seen)
}

fn query_daemon(socket: &Path, language: &str, scenario: &str) -> Result<Observation, String> {
    let mut stream = UnixStream::connect(socket)
        .map_err(|error| format!("{language} could not connect to daemon: {error}"))?;
    writeln!(stream, "{language}\t{scenario}")
        .map_err(|error| format!("{language} daemon request failed: {error}"))?;
    let mut response = String::new();
    BufReader::new(stream)
        .read_line(&mut response)
        .map_err(|error| format!("{language} daemon response failed: {error}"))?;
    Observation::decode(response.trim_end())
}

fn validate_rust_sdk(
    scenario: &str,
    observation: &Observation,
    freshness: &Freshness,
) -> Result<(), String> {
    let client = RustSdkClient::daemon("parity.sock", agent_api_schema_v1().version)
        .map_err(|error| format!("Rust SDK daemon client: {error:?}"))?;
    if client.deployment() != Deployment::Daemon {
        return Err("Rust SDK did not select daemon deployment".to_owned());
    }
    let sdk_tenant = || {
        SdkTenantId::new("parity-tenant")
            .map_err(|error| format!("Rust SDK approval tenant: {error:?}"))
    };
    let approval_id = ApprovalId::new([7; 32]);
    let operation = match scenario {
        "approval_list" => client
            .approval_list(
                ApprovalListRequest::new(sdk_tenant()?, None, 50)
                    .map_err(|error| format!("Rust SDK approval list: {error:?}"))?,
            )
            .operation(),
        "approval_get" => client
            .approval_get(ApprovalGetRequest {
                tenant: sdk_tenant()?,
                approval_id,
            })
            .operation(),
        "approval_approve" => client
            .approval_approve(ApprovalApproveRequest {
                tenant: sdk_tenant()?,
                approval_id,
                idempotency_key: DecisionKey::new("approve-7")
                    .map_err(|error| format!("Rust SDK approval key: {error:?}"))?,
            })
            .operation(),
        "approval_reject" => client
            .approval_reject(
                ApprovalRejectRequest::new(
                    sdk_tenant()?,
                    approval_id,
                    DecisionKey::new("reject-7")
                        .map_err(|error| format!("Rust SDK rejection key: {error:?}"))?,
                    "not expected",
                )
                .map_err(|error| format!("Rust SDK approval rejection: {error:?}"))?,
            )
            .operation(),
        _ => client
            .track(TrackRequest {
                submission_ref: SubmissionRef::new(scenario)
                    .map_err(|error| format!("Rust SDK submission ref: {error:?}"))?,
            })
            .operation(),
    };
    let expected_operation = match scenario {
        "approval_list" => Operation::ApprovalList,
        "approval_get" => Operation::ApprovalGet,
        "approval_approve" => Operation::ApprovalApprove,
        "approval_reject" => Operation::ApprovalReject,
        _ => Operation::Track,
    };
    if operation != expected_operation {
        return Err("Rust SDK changed parity operation".to_owned());
    }
    match scenario {
        "unknown_submission" if observation.state != "Unknown" => {
            Err("Rust SDK collapsed unknown submission".to_owned())
        }
        "terminal_rejection"
            if ResultCode::from_raw(
                observation
                    .result_code
                    .parse()
                    .map_err(|_| "Rust SDK invalid result code".to_owned())?,
            )
            .raw()
                != -77_777 =>
        {
            Err("Rust SDK changed terminal result code".to_owned())
        }
        "proven_read" => {
            let value = BalanceValue {
                account: AccountRef::new("account-a")
                    .map_err(|error| format!("account: {error:?}"))?,
                asset: layerx_agent_api::identity::Asset::new("LXP")
                    .map_err(|error| format!("asset: {error:?}"))?,
                amount: Amount(1),
                height: freshness.value_height,
                canonical_state: layerx_agent_api::prepare::CanonicalBytes::new(vec![1])
                    .map_err(|error| format!("state: {error:?}"))?,
            };
            RustSdkClient::accept_verified_read(
                Rung::Committed,
                VerifiedRead::new(value, Rung::Committed, freshness.clone()),
            )
            .map(|_| ())
            .map_err(|error| format!("Rust SDK refused proven read: {error:?}"))
        }
        "availability_failure" if observation.error != "UnavailableCapability" => {
            Err("Rust SDK changed availability error".to_owned())
        }
        "subscription_gap" if observation.state != "Gap" => {
            Err("Rust SDK hid subscription gap".to_owned())
        }
        value
            if value.starts_with("idempotency_")
                && (observation.receipt_count != 1 || observation.economic_effects != 1) =>
        {
            Err("Rust SDK observed duplicate economic effects".to_owned())
        }
        value if value.starts_with("approval_event_") => {
            if ApprovalEventKind::ALL
                .iter()
                .any(|kind| kind.name() == observation.state)
            {
                Ok(())
            } else {
                Err("Rust SDK approval event vocabulary diverged".to_owned())
            }
        }
        value if value.starts_with("approval_outcome_") => {
            if ApprovalDecisionOutcome::ALL
                .iter()
                .any(|outcome| outcome.name() == observation.state)
            {
                Ok(())
            } else {
                Err("Rust SDK approval outcome vocabulary diverged".to_owned())
            }
        }
        "approval_list" | "approval_get" | "approval_approve" | "approval_reject"
            if observation.state != operation.name() =>
        {
            Err("Rust SDK approval operation vocabulary diverged".to_owned())
        }
        _ => Ok(()),
    }
}

fn run_rust(
    socket: &Path,
    scenarios: &[String],
    freshness: &Freshness,
) -> Result<BTreeMap<String, Observation>, String> {
    scenarios
        .iter()
        .map(|scenario| {
            let observation = query_daemon(socket, "rust", scenario)?;
            validate_rust_sdk(scenario, &observation, freshness)?;
            Ok((scenario.clone(), observation))
        })
        .collect()
}

fn run_external(
    language: &str,
    mut command: Command,
    expected: &BTreeMap<String, Observation>,
) -> Result<BTreeMap<String, Observation>, String> {
    let output = command
        .output()
        .map_err(|error| format!("could not run {language} SDK: {error}"))?;
    if !output.status.success() {
        return Err(format!(
            "{language} SDK failed: status={} stderr={}",
            output.status,
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    let stdout = String::from_utf8(output.stdout)
        .map_err(|_| format!("{language} SDK emitted non-UTF-8 output"))?;
    let observations = stdout
        .lines()
        .map(|line| {
            let (scenario, encoded) = line
                .split_once('\t')
                .ok_or_else(|| format!("{language} emitted malformed result {line}"))?;
            Ok((scenario.to_owned(), Observation::decode(encoded)?))
        })
        .collect::<Result<BTreeMap<_, _>, String>>()?;
    if observations.len() != expected.len() {
        return Err(format!(
            "{language} returned {} scenarios, expected {}",
            observations.len(),
            expected.len()
        ));
    }
    Ok(observations)
}

fn compare_language(
    language: &str,
    expected: &BTreeMap<String, Observation>,
    observed: &BTreeMap<String, Observation>,
) -> Result<(), String> {
    for (scenario, expected_value) in expected {
        let observed_value = observed
            .get(scenario)
            .ok_or_else(|| format!("scenario={scenario} language={language} missing result"))?;
        if observed_value != expected_value {
            return Err(format!(
                "scenario={scenario} language={language} expected={} observed={}",
                expected_value.encode(),
                observed_value.encode()
            ));
        }
    }
    Ok(())
}

/// Runs every SDK against one live daemon and one node JSON-RPC endpoint.
///
/// # Errors
///
/// Names the scenario, language and differing observations on any divergence.
pub fn agent_sdk_parity_suite(node_url: &str, repository: &Path) -> Result<String, String> {
    let node_url = node_json_rpc_url(node_url)?;
    let expected = parse_scenarios(&repository.join("services/agent/tests/parity/scenarios.kvx"))?;
    if expected.len() != 21 {
        return Err(format!(
            "parity suite expected 21 scenarios, got {}",
            expected.len()
        ));
    }
    qualify_state_semantics(&expected)?;
    let directory = std::env::temp_dir().join(format!("layerx-sdk-parity-{}", std::process::id()));
    if directory.exists() {
        fs::remove_dir_all(&directory)
            .map_err(|error| format!("could not clear parity directory: {error}"))?;
    }
    fs::create_dir(&directory)
        .map_err(|error| format!("could not create parity directory: {error}"))?;
    qualify_idempotency(&directory, &expected)?;
    let freshness = qualify_node(&node_url)?;

    let daemon_socket = directory.join("agentd-parity.sock");
    let listener = UnixListener::bind(&daemon_socket)
        .map_err(|error| format!("could not bind parity daemon: {error}"))?;
    let daemon_observations = expected.clone();
    let expected_requests = expected.len() * 3;
    let daemon =
        thread::spawn(move || serve_daemon(listener, daemon_observations, expected_requests));
    let scenarios = expected.keys().cloned().collect::<Vec<_>>();
    let joined = scenarios.join(",");
    let rust = run_rust(&daemon_socket, &scenarios, &freshness)?;

    let mut typescript_command = Command::new("node");
    typescript_command
        .arg(repository.join("services/agent/tests/parity/typescript.mjs"))
        .arg(&daemon_socket)
        .arg(&joined);
    let typescript = run_external("typescript", typescript_command, &expected)?;

    let mut python_command = Command::new("python3");
    python_command
        .arg(repository.join("services/agent/tests/parity/python.py"))
        .arg(&daemon_socket)
        .arg(&joined)
        .env("PYTHONPATH", repository.join("services/agent/sdk/python"));
    let python = run_external("python", python_command, &expected)?;

    compare_language("rust", &expected, &rust)?;
    compare_language("typescript", &expected, &typescript)?;
    compare_language("python", &expected, &python)?;
    let seen = daemon
        .join()
        .map_err(|_| "parity daemon panicked".to_owned())??;
    if seen.len() != expected_requests {
        return Err(format!(
            "parity daemon observed {} unique requests, expected {expected_requests}",
            seen.len()
        ));
    }
    fs::remove_dir_all(&directory)
        .map_err(|error| format!("could not clean parity directory: {error}"))?;
    Ok(format!(
        "agent SDK parity passed: {} scenarios, 3 SDKs, one daemon, one node JSON-RPC endpoint",
        expected.len()
    ))
}

fn main() -> ExitCode {
    let mut arguments = std::env::args_os().skip(1);
    let Some(first) = arguments.next() else {
        eprintln!("missing http or https node JSON-RPC URL");
        return ExitCode::FAILURE;
    };
    let result = if first == "--check-scenarios" {
        match (arguments.next(), arguments.next()) {
            (Some(path), None) => parse_scenarios(Path::new(&path)).map(|scenarios| {
                format!(
                    "{} parity scenarios use submitted, receipted or committed",
                    scenarios.len()
                )
            }),
            _ => Err("usage: agent-sdk-parity --check-scenarios <path>".to_owned()),
        }
    } else {
        let node = first
            .to_str()
            .ok_or_else(|| "parity suite requires an http or https node JSON-RPC URL".to_owned());
        node.and_then(|node| {
            node_json_rpc_url(node)?;
            let repository = arguments
                .next()
                .map(PathBuf::from)
                .ok_or_else(|| "missing repository root".to_owned())?;
            if arguments.next().is_some() {
                return Err("unexpected parity-suite argument".to_owned());
            }
            agent_sdk_parity_suite(node, &repository)
        })
    };
    match result {
        Ok(report) => {
            println!("{report}");
            ExitCode::SUCCESS
        }
        Err(error) => {
            eprintln!("SDK parity failed: {error}");
            ExitCode::FAILURE
        }
    }
}
