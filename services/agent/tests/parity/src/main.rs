use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};
use std::time::Duration;

use layerx_agent_api::read::{Freshness, RelativeTo};
use layerx_agent_api::verify::Rung;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_sdk::Operation;
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
    let keys = node
        .receipt_keys()
        .map_err(|error| format!("node JSON-RPC receipt keys failed: {error}"))?;
    let repeated_keys = node
        .receipt_keys()
        .map_err(|error| format!("repeated node JSON-RPC receipt keys failed: {error}"))?;
    if keys != repeated_keys {
        return Err("node changed receipt authority during parity qualification".to_owned());
    }
    Ok(Freshness {
        chain_head: height,
        value_height: height,
        relative_to: RelativeTo::Block {
            height,
            hash: block.hash,
        },
    })
}

/// Compares SDK responses from an operator-provisioned daemon, never expected observations.
pub fn agent_sdk_parity_suite(node_url: &str, repository: &Path) -> Result<String, String> {
    let node_url = node_json_rpc_url(node_url)?;
    let expected = parse_scenarios(&repository.join("services/agent/tests/parity/scenarios.kvx"))?;
    if expected.len() != 21 {
        return Err(format!(
            "parity suite expected 21 scenarios, got {}",
            expected.len()
        ));
    }
    let plan_path = repository.join("services/agent/tests/parity/provisioned.json");
    let plan: Value = serde_json::from_slice(&fs::read(&plan_path).map_err(|error| {
        format!(
            "provision a real daemon and envelope plan at {}: {error}",
            plan_path.display()
        )
    })?)
    .map_err(|error| format!("invalid daemon provisioning plan: {error}"))?;
    let cases = plan["cases"]
        .as_object()
        .ok_or("provisioning plan omitted cases")?;
    if cases.keys().collect::<BTreeSet<_>>() != expected.keys().collect::<BTreeSet<_>>() {
        return Err("provisioning plan must exercise all 21 declared parity scenarios".to_owned());
    }
    qualify_node(&node_url)?;
    let daemon = agent_sdk_parity::DaemonParity::from_document(&plan["daemon"], &plan_path)?;
    let mut rust = BTreeMap::new();
    for (name, case) in cases {
        let operation = Operation::ALL
            .iter()
            .copied()
            .find(|operation| Some(operation.name()) == case["operation"].as_str())
            .ok_or_else(|| format!("scenario {name} has no current operation"))?;
        let request_id = case["request_id"]
            .as_str()
            .and_then(|text| text.parse().ok())
            .ok_or_else(|| format!("scenario {name} omitted its canonical request id"))?;
        let key = match &case["idempotency_key"] {
            Value::Null => None,
            value => Some(
                layerx_agent_api::idempotency::Key::new(agent_sdk_parity::digest(value)?)
                    .map_err(|error| format!("scenario {name} key: {error:?}"))?,
            ),
        };
        let observation = daemon.observe(
            operation,
            layerx_agent_api::error::RequestId(request_id),
            &case["request"],
            key,
        )?;
        rust.insert(name.clone(), observation);
    }
    for (language, executable, script) in [
        ("typescript", "node", "typescript.mjs"),
        ("python", "python3", "python.py"),
    ] {
        let mut command = Command::new(executable);
        command
            .arg(repository.join("services/agent/tests/parity").join(script))
            .arg("--envelope-plan")
            .arg(&plan_path)
            .env("PYTHONPATH", repository.join("services/agent/sdk/python"));
        let output = command
            .output()
            .map_err(|error| format!("{language} envelope consumer: {error}"))?;
        if !output.status.success() {
            return Err(format!(
                "{language} current-envelope driver failed: {}",
                String::from_utf8_lossy(&output.stderr).trim()
            ));
        }
        let source = String::from_utf8(output.stdout)
            .map_err(|_| format!("{language} emitted non-UTF-8 evidence"))?;
        let mut observations = BTreeMap::new();
        for line in source.lines() {
            let (scenario, document) = line
                .split_once('\t')
                .ok_or_else(|| format!("{language} emitted malformed envelope evidence"))?;
            let value: Value = serde_json::from_str(document)
                .map_err(|_| format!("{language} emitted invalid envelope evidence"))?;
            if observations.insert(scenario.to_owned(), value).is_some() {
                return Err(format!("{language} repeated scenario {scenario}"));
            }
        }
        agent_sdk_parity::compare_evidence(language, &rust, &observations)?;
    }
    Ok(format!("agent SDK parity passed: {} scenarios, 3 SDKs, provisioned daemon envelopes and node JSON-RPC", cases.len()))
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
