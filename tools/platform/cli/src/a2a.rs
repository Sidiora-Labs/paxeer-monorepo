use std::collections::BTreeMap;
use std::fs;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::Path;
use std::process::{Command, Stdio};
use std::sync::{
    mpsc::{sync_channel, TrySendError},
    Arc, Mutex,
};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use layerx_mcp::server::{DeploymentMode, ToolDefinition};
use serde::{Deserialize, Serialize};
use serde_json::{json, Map, Value};
use sha2::{Digest as _, Sha256};
use subtle::ConstantTimeEq as _;
use zeroize::{Zeroize as _, Zeroizing};

use crate::config::Configuration;
use crate::encoding::hex_encode;
use crate::install::a2a::agent_card;
use crate::toolset;

const MAX_HEADER_BYTES: usize = 32 * 1024;
const MAX_REQUEST_BYTES: usize = 256 * 1024;
const MAX_TRACKED_TASKS: usize = 128;
const MAX_CONTEXT_ID_BYTES: usize = 256;
const WORKER_COUNT: usize = 8;
const ADMISSION_CAPACITY: usize = 32;
const CARD_ROUTES: [&str; 2] = ["/.well-known/agent-card.json", "/.well-known/agent.json"];
const STATE_FILE: &str = "service.json";

/// Durable task identities survive ambiguous submission and service restarts.
pub(crate) struct TaskLedger {
    entries: BTreeMap<String, Value>,
    path: Option<std::path::PathBuf>,
    _lock: Option<fs::File>,
}
impl std::ops::Deref for TaskLedger {
    type Target = BTreeMap<String, Value>;
    fn deref(&self) -> &Self::Target {
        &self.entries
    }
}
impl std::ops::DerefMut for TaskLedger {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.entries
    }
}
impl TaskLedger {
    pub(crate) fn open(path: &Path) -> Result<Self, String> {
        use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _};
        let mut options = fs::OpenOptions::new();
        options
            .read(true)
            .write(true)
            .create(true)
            .mode(0o600)
            .custom_flags(
                i32::try_from(rustix::fs::OFlags::NOFOLLOW.bits())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            );
        let lock = options
            .open(path.with_extension("lock"))
            .map_err(|e| format!("task ledger lock: {e}"))?;
        let meta = lock.metadata().map_err(|e| e.to_string())?;
        if !meta.is_file()
            || meta.mode() & 0o077 != 0
            || meta.uid() != rustix::process::geteuid().as_raw()
        {
            return Err("task ledger lock must be caller-owned and private".into());
        }
        rustix::fs::flock(&lock, rustix::fs::FlockOperation::NonBlockingLockExclusive)
            .map_err(|e| format!("task ledger is already open: {e}"))?;
        let entries = match fs::symlink_metadata(path) {
            Ok(meta) => {
                if !meta.is_file()
                    || meta.mode() & 0o077 != 0
                    || meta.uid() != rustix::process::geteuid().as_raw()
                {
                    return Err("task ledger must be a caller-owned private regular file".into());
                }
                serde_json::from_slice::<BTreeMap<String, Value>>(
                    &fs::read(path).map_err(|e| e.to_string())?,
                )
                .map_err(|e| format!("invalid task ledger: {e}"))?
            }
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => BTreeMap::new(),
            Err(e) => return Err(e.to_string()),
        };
        if entries.len() > MAX_TRACKED_TASKS
            || entries.iter().any(|(id, task)| {
                task["id"] != *id
                    || task
                        .pointer("/status/state")
                        .and_then(Value::as_str)
                        .is_none()
            })
        {
            return Err("invalid task ledger identities or capacity".into());
        }
        Ok(Self {
            entries,
            path: Some(path.to_owned()),
            _lock: Some(lock),
        })
    }
    #[cfg(test)]
    fn memory() -> Self {
        Self {
            entries: BTreeMap::new(),
            path: None,
            _lock: None,
        }
    }
    fn persist(&self) -> Result<(), String> {
        let Some(path) = &self.path else {
            return Ok(());
        };
        let parent = path.parent().ok_or("task ledger has no parent")?;
        let mut temporary = tempfile::NamedTempFile::new_in(parent).map_err(|e| e.to_string())?;
        serde_json::to_writer(temporary.as_file_mut(), &self.entries).map_err(|e| e.to_string())?;
        temporary.as_file().sync_all().map_err(|e| e.to_string())?;
        temporary.persist(path).map_err(|e| e.to_string())?;
        fs::File::open(parent)
            .and_then(|f| f.sync_all())
            .map_err(|e| e.to_string())
    }
    fn reserve(&mut self, id: &str, task: Value) -> Result<(), String> {
        let previous = self.entries.clone();
        if self.entries.len() >= MAX_TRACKED_TASKS {
            let terminal = self
                .entries
                .iter()
                .find(|(_, task)| {
                    matches!(
                        task.pointer("/status/state").and_then(Value::as_str),
                        Some("completed" | "failed" | "rejected")
                    )
                })
                .map(|(id, _)| id.clone());
            let Some(id) = terminal else {
                return Err("task capacity reached; unresolved transactions are retained".into());
            };
            self.entries.remove(&id);
        }
        self.entries.insert(id.to_owned(), task);
        if let Err(error) = self.persist() {
            self.entries = previous;
            return Err(error);
        }
        Ok(())
    }
    pub(crate) fn reconcile(&mut self, runtime: &Runtime) -> Result<(), String> {
        let Some(policy) = &runtime.policy else {
            return Ok(());
        };
        for task in self.entries.values_mut() {
            if !matches!(
                task.pointer("/status/state").and_then(Value::as_str),
                Some("submitted" | "unknown")
            ) {
                continue;
            }
            let Some(hash) = task
                .get("txHash")
                .and_then(Value::as_str)
                .map(str::to_owned)
            else {
                continue;
            };
            match runtime.rpc("pax_sequencerReceipt", &json!([hash])) {
                Ok(receipt) => {
                    let expected = crate::programs::transaction_hash(&hash)?;
                    if let Ok((verified, _)) = layerx_sdk::rpc_verification::verify_node_receipt(
                        &receipt, expected, policy,
                    ) {
                        let facts = verified
                            .protocol()
                            .ok_or("recovered receipt omitted protocol facts")?;
                        task["status"]["state"] = json!(if facts.result_code() != 0 {
                            "failed"
                        } else {
                            "completed"
                        });
                        task["reconciliation"] = receipt;
                    }
                }
                Err(_) => break, // Retain unresolved work without resubmitting or blocking startup on every task.
            }
        }
        self.persist()
    }
}

pub(crate) struct Runtime {
    client: crate::http::Client,
    network_id: u32,
    environment: String,
    faucet_endpoint: Option<String>,
    policy: Option<layerx_sdk::rpc_verification::ReceiptPolicy>,
}
impl Runtime {
    pub(crate) fn connect(
        endpoint: &str,
        network_id: u32,
        credential: Option<Zeroizing<String>>,
    ) -> Result<Self, String> {
        let endpoint = layerx_platform_cli::chains::node_endpoint(endpoint)?;
        let client = match credential {
            Some(key) => crate::http::Client::new_gateway(endpoint, key)?,
            None => crate::http::Client::new(endpoint, None)?,
        };
        Ok(Self {
            client,
            network_id,
            environment: String::new(),
            faucet_endpoint: None,
            policy: None,
        })
    }
    pub(crate) fn with_policy(
        mut self,
        policy: layerx_sdk::rpc_verification::ReceiptPolicy,
    ) -> Result<Self, String> {
        if policy.network_id != self.network_id || policy.protocol_version != 2 {
            return Err("task receipt policy does not match the node".into());
        }
        self.policy = Some(policy);
        Ok(self)
    }
    #[cfg(test)]
    fn loopback(endpoint: &str, network_id: u32) -> Result<Self, String> {
        Self::connect(endpoint, network_id, None)
    }
    fn rpc(&self, method: &str, params: &Value) -> Result<Value, String> {
        let request = layerx_platform_cli::rpc::request(method, params)?;
        let response = self.client.post("/", &request, None)?;
        layerx_platform_cli::rpc::decode_response(method, &response)
    }
    fn signed_hash(&self, arguments: &Value) -> Result<String, String> {
        use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
        if arguments.as_object().is_none_or(|a| a.len() != 1) {
            return Err("activity.submit requires only raw_transaction".into());
        }
        let raw = arguments["raw_transaction"]
            .as_str()
            .and_then(|r| r.strip_prefix("0x"))
            .ok_or("activity.submit requires signed raw_transaction hex")?;
        let bytes = crate::encoding::hex_decode("raw transaction", raw)?;
        let (call, signature) = decode_signed(&bytes).map_err(|e| e.to_string())?;
        recover_sender(&call, &signature).map_err(|e| e.to_string())?;
        if call.chain_id != u64::from(self.network_id) {
            return Err("raw transaction chain id does not match the active environment".into());
        }
        Ok(format!("0x{}", hex_encode(&transaction_hash(&bytes))))
    }
    #[cfg(test)]
    fn invoke(&self, tool: ToolDefinition, arguments: &Value) -> Result<Value, String> {
        let result = self.invoke_task(tool, arguments)?;
        if result["rung"] == "unknown" {
            return Err(format!("submission {} is indeterminate", result["txHash"]));
        }
        Ok(result)
    }
    fn invoke_task(&self, tool: ToolDefinition, arguments: &Value) -> Result<Value, String> {
        match tool.name {
            "activity.submit" => {
                let hash = self.signed_hash(arguments)?;
                let request = layerx_platform_cli::rpc::request(
                    "eth_sendRawTransaction",
                    &json!([arguments["raw_transaction"]]),
                )?;
                let response = match self.client.post("/", &request, None) {
                    Ok(response) => response,
                    Err(detail) => {
                        return Ok(json!({"txHash":hash,"rung":"unknown","detail":detail}))
                    }
                };
                match layerx_platform_cli::rpc::decode_response("eth_sendRawTransaction", &response)
                {
                    Ok(returned) if returned == hash => {
                        Ok(json!({"txHash":hash,"rung":"submitted"}))
                    }
                    Err(detail)
                        if response["jsonrpc"] == "2.0"
                            && response["id"] == 1
                            && response.get("result").is_none()
                            && response["error"]["code"].as_i64().is_some()
                            && response["error"]["message"].as_str().is_some() =>
                    {
                        Err(detail)
                    }
                    outcome => Ok(
                        json!({"txHash":hash,"rung":"unknown","detail":format!("indeterminate node response: {outcome:?}")}),
                    ),
                }
            }
            "receipt.get" => {
                if arguments.as_object().is_none_or(|a| a.len() != 1) {
                    return Err("receipt.get requires only activity_id".into());
                }
                self.rpc("pax_sequencerReceipt", &json!([arguments["activity_id"]]))
            }
            "faucet.request" => {
                let endpoint = self
                    .faucet_endpoint
                    .as_deref()
                    .ok_or("faucet.request requires an explicit hosted faucet service endpoint")?;
                let token = crate::credential::token(&self.environment)?
                    .ok_or("identity_session_required")?;
                let did = arguments["did"].as_str().ok_or("missing faucet DID")?;
                let key = arguments["public_key"]
                    .as_str()
                    .ok_or("missing faucet public key")?;
                let key = key.to_ascii_lowercase();
                let claim =
                    crate::faucet::decode(&crate::faucet::claim(endpoint, token, did, &key)?)?;
                Ok(json!({"did":did,"public_key":key,"claim":claim}))
            }
            _ => Err("skill is not served by this deployment".into()),
        }
    }
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ServiceState {
    pid: u32,
    process_start_ticks: u64,
    launch_digest: String,
}

struct Request {
    method: String,
    path: String,
    authorization: Option<Zeroizing<String>>,
    body: Vec<u8>,
}

impl Drop for Request {
    fn drop(&mut self) {
        self.body.zeroize();
    }
}

struct Response {
    status: u16,
    body: Vec<u8>,
}

struct Instruction {
    skill: String,
    arguments: Value,
}

pub fn endpoint(listen: &str) -> Result<SocketAddr, String> {
    let address = listen
        .parse::<SocketAddr>()
        .map_err(|_| format!("{listen} is not a host:port endpoint"))?;
    if !address.ip().is_loopback() {
        return Err("the agent-to-agent endpoint must bind a loopback address".into());
    }
    if address.port() == 0 {
        return Err("the agent-to-agent endpoint must declare a fixed port".into());
    }
    Ok(address)
}

#[derive(Clone, Copy)]
pub struct ServeRequest<'a> {
    pub receipt_policy: Option<&'a Path>,
    pub faucet_endpoint: Option<&'a str>,
    pub gateway_credential: &'a str,
    pub key: &'a str,
    pub source: Option<&'a str>,
    pub asset: Option<&'a str>,
    pub listen: &'a str,
    pub authorization_file: &'a Path,
    pub mode: DeploymentMode,
}

pub fn serve(configuration: &Configuration, request: ServeRequest<'_>) -> Result<(), String> {
    let ServeRequest {
        receipt_policy,
        faucet_endpoint,
        gateway_credential,
        key,
        source,
        asset,
        listen,
        authorization_file,
        mode,
    } = request;
    let address = endpoint(listen)?;
    let tools = toolset::surface(mode)?;
    let _validated =
        toolset::Runtime::new(configuration, gateway_credential, key, source, asset, mode)?;
    let (_, profile) = configuration.active_environment()?;
    let mut runtime = Runtime::connect(
        &profile.endpoint,
        profile.network_id,
        Some(
            crate::credential::gateway(gateway_credential)?
                .ok_or("gateway credential is absent")?,
        ),
    )?;
    if let Some(path) = receipt_policy {
        runtime = runtime.with_policy(crate::programs::read_lifecycle_policy(path)?)?;
    }
    configuration
        .current_environment
        .clone_into(&mut runtime.environment);
    runtime.faucet_endpoint = faucet_endpoint.map(str::to_owned);
    let runtime = Arc::new(runtime);
    let card = Arc::new(agent_card(
        &configuration.current_environment,
        &address.to_string(),
        mode,
        &tools,
    ));
    let tools = Arc::new(tools);
    let authorization = Arc::new(crate::install::a2a::read_authorization(authorization_file)?);
    let listener =
        TcpListener::bind(address).map_err(|error| format!("could not bind {listen}: {error}"))?;
    let mut ledger = TaskLedger::open(&authorization_file.with_extension("tasks.json"))?;
    ledger.reconcile(&runtime)?;
    let tasks = Arc::new(Mutex::new(ledger));
    let (sender, receiver) = sync_channel::<TcpStream>(ADMISSION_CAPACITY);
    let receiver = Arc::new(Mutex::new(receiver));
    for index in 0..WORKER_COUNT {
        let runtime = Arc::clone(&runtime);
        let tools = Arc::clone(&tools);
        let card = Arc::clone(&card);
        let tasks = Arc::clone(&tasks);
        let authorization = Arc::clone(&authorization);
        let receiver = Arc::clone(&receiver);
        std::thread::Builder::new()
            .name(format!("layerx-a2a-{index}"))
            .spawn(move || loop {
                let stream = match receiver.lock() {
                    Ok(receiver) => receiver.recv(),
                    Err(_) => return,
                };
                let Ok(mut stream) = stream else {
                    return;
                };
                if stream
                    .set_read_timeout(Some(Duration::from_secs(10)))
                    .and_then(|()| stream.set_write_timeout(Some(Duration::from_secs(10))))
                    .is_err()
                {
                    continue;
                }
                let _ =
                    serve_connection(&runtime, &tools, &card, &tasks, &authorization, &mut stream);
            })
            .map_err(|error| format!("could not start A2A worker {index}: {error}"))?;
    }
    for accepted in listener.incoming() {
        let Ok(stream) = accepted else {
            continue;
        };
        match sender.try_send(stream) {
            Ok(()) => {}
            Err(TrySendError::Full(mut stream)) => {
                let _ = stream.set_write_timeout(Some(Duration::from_secs(2)));
                let _ = write_response(
                    &mut stream,
                    &encode(
                        503,
                        &json!({"error": "agent-to-agent service is at capacity"}),
                    ),
                );
            }
            Err(TrySendError::Disconnected(_)) => {
                return Err("all agent-to-agent workers stopped".into())
            }
        }
    }
    Ok(())
}

pub(crate) fn serve_connection(
    runtime: &Runtime,
    tools: &[ToolDefinition],
    card: &Value,
    tasks: &Mutex<TaskLedger>,
    authorization: &str,
    stream: &mut TcpStream,
) -> Result<(), String> {
    let response = match read_request_head(stream) {
        Ok(head) if head.request.method == "POST" && !authorized(&head.request, authorization) => {
            encode(
                401,
                &json!({"error": "valid bearer authorization is required"}),
            )
        }
        Ok(head) => match read_request_body(stream, head) {
            Ok(request) => route(runtime, tools, card, tasks, authorization, &request),
            Err(error) => encode(400, &json!({"error":error})),
        },
        Err(error) => encode(400, &json!({"error":error})),
    };
    write_response(stream, &response).map_err(|e| e.to_string())
}

fn route(
    runtime: &Runtime,
    tools: &[ToolDefinition],
    card: &Value,
    tasks: &Mutex<TaskLedger>,
    authorization: &str,
    request: &Request,
) -> Response {
    match request.method.as_str() {
        "GET" if CARD_ROUTES.contains(&request.path.as_str()) => encode(200, card),
        "POST" if !authorized(request, authorization) => encode(
            401,
            &json!({"error": "valid bearer authorization is required"}),
        ),
        "POST" if request.path == "/" => {
            encode(200, &dispatch(runtime, tools, tasks, &request.body))
        }
        "GET" | "POST" => encode(404, &json!({"error": "unknown agent-to-agent route"})),
        _ => encode(405, &json!({"error": "unsupported method"})),
    }
}

fn authorized(request: &Request, expected: &str) -> bool {
    let Some(presented) = request
        .authorization
        .as_deref()
        .and_then(|value| value.strip_prefix("Bearer "))
    else {
        return false;
    };
    presented.len() == expected.len()
        && presented.as_bytes().ct_eq(expected.as_bytes()).unwrap_u8() == 1
}

pub(crate) fn dispatch(
    runtime: &Runtime,
    tools: &[ToolDefinition],
    tasks: &Mutex<TaskLedger>,
    body: &[u8],
) -> Value {
    let Ok(message) = serde_json::from_slice::<Value>(body) else {
        return failure(Value::Null, -32700, "the message is not valid JSON");
    };
    let identifier = message.get("id").cloned().unwrap_or(Value::Null);
    let Some(method) = message.get("method").and_then(Value::as_str) else {
        return failure(identifier, -32600, "the message did not name a method");
    };
    let parameters = message.get("params").cloned().unwrap_or(Value::Null);
    match method {
        "message/send" => send(runtime, tools, tasks, identifier, &parameters),
        "tasks/get" => fetch(tasks, identifier, &parameters),
        "tasks/cancel" => cancel(tasks, identifier, &parameters),
        "message/stream" | "tasks/resubscribe" | "tasks/pushNotificationConfig/set" => failure(
            identifier,
            -32004,
            "the operation is not supported by this deployment",
        ),
        _ => failure(identifier, -32601, "the method is not implemented"),
    }
}

fn send(
    runtime: &Runtime,
    tools: &[ToolDefinition],
    tasks: &Mutex<TaskLedger>,
    identifier: Value,
    parameters: &Value,
) -> Value {
    let Some(message) = parameters.get("message") else {
        return failure(identifier, -32602, "the request carried no message");
    };
    let instruction = match read_instruction(message) {
        Ok(instruction) => instruction,
        Err(error) => return failure(identifier, -32602, &error),
    };
    let Some(tool) = tools
        .iter()
        .find(|tool| tool.name == instruction.skill)
        .copied()
    else {
        return failure(
            identifier,
            -32601,
            &format!(
                "skill {} is not served by this deployment",
                instruction.skill
            ),
        );
    };
    let ids = match (new_identifier(), new_identifier()) {
        (Ok(task), Ok(artifact)) => (task, artifact),
        _ => {
            return failure(
                identifier,
                -32603,
                "operating-system randomness failed while opening a task",
            )
        }
    };
    let context = message
        .get("contextId")
        .and_then(Value::as_str)
        .map_or_else(|| ids.0.clone(), str::to_owned);
    if context.is_empty() || context.len() > MAX_CONTEXT_ID_BYTES {
        return failure(
            identifier,
            -32602,
            "the context identifier is outside the transport limit",
        );
    }
    let timestamp = match timestamp() {
        Ok(value) => value,
        Err(error) => return failure(identifier, -32603, &error),
    };
    if tool.name == "activity.submit"
        && instruction
            .arguments
            .get("raw_transaction")
            .and_then(Value::as_str)
            .is_none()
    {
        return failure(
            identifier,
            -32602,
            "activity.submit requires signed raw_transaction hex",
        );
    }
    let hash = match reserve_signed_submission(
        runtime,
        tasks,
        &SignedSubmission {
            ids: &ids,
            context: &context,
            message,
            tool_name: tool.name,
            timestamp: &timestamp,
            arguments: &instruction.arguments,
            identifier: &identifier,
        },
    ) {
        Ok(hash) => hash,
        Err(response) => return response,
    };
    let mut pending = build_task(
        &ids,
        &context,
        message,
        tool.name,
        &timestamp,
        Ok(json!({"rung":"unknown","txHash":hash})),
    );
    if let Some(hash) = &hash {
        pending["txHash"] = json!(hash);
    }
    {
        let Ok(mut ledger) = tasks.lock() else {
            return failure(identifier, -32603, "the task ledger is unavailable");
        };
        if let Err(error) = ledger.reserve(&ids.0, pending) {
            return failure(identifier, -32005, &error);
        }
    }
    let outcome = runtime.invoke_task(tool, &instruction.arguments);
    let mut task = build_task(&ids, &context, message, tool.name, &timestamp, outcome);
    if let Some(hash) = hash {
        task["txHash"] = json!(hash);
    }
    finish_send(tasks, &ids.0, task, identifier)
}

struct SignedSubmission<'a> {
    ids: &'a (String, String),
    context: &'a str,
    message: &'a Value,
    tool_name: &'a str,
    timestamp: &'a str,
    arguments: &'a Value,
    identifier: &'a Value,
}

fn reserve_signed_submission(
    runtime: &Runtime,
    tasks: &Mutex<TaskLedger>,
    submission: &SignedSubmission<'_>,
) -> Result<Option<String>, Value> {
    if submission.tool_name != "activity.submit" {
        return Ok(None);
    }
    match runtime.signed_hash(submission.arguments) {
        Ok(hash) => Ok(Some(hash)),
        Err(error) => {
            let rejected = build_task(
                submission.ids,
                submission.context,
                submission.message,
                submission.tool_name,
                submission.timestamp,
                Err(error),
            );
            let Ok(mut ledger) = tasks.lock() else {
                return Err(failure(
                    submission.identifier.clone(),
                    -32603,
                    "the task ledger is unavailable",
                ));
            };
            if let Err(error) = ledger.reserve(&submission.ids.0, rejected.clone()) {
                return Err(failure(submission.identifier.clone(), -32005, &error));
            }
            Err(success(submission.identifier.clone(), rejected))
        }
    }
}

fn finish_send(tasks: &Mutex<TaskLedger>, task_id: &str, task: Value, identifier: Value) -> Value {
    let Ok(mut ledger) = tasks.lock() else {
        return failure(identifier, -32603, "the task ledger is unavailable");
    };
    let previous = ledger.insert(task_id.to_owned(), task.clone());
    if let Err(error) = ledger.persist() {
        if let Some(previous) = previous {
            ledger.insert(task_id.to_owned(), previous);
        }
        return failure(
            identifier,
            -32603,
            &format!("task {task_id} retained for recovery; journal update failed: {error}"),
        );
    }
    success(identifier, task)
}

fn build_task(
    ids: &(String, String),
    context: &str,
    message: &Value,
    tool: &str,
    timestamp: &str,
    outcome: Result<Value, String>,
) -> Value {
    match outcome {
        Ok(value) => {
            let state = match gateway_state(&value) {
                Some("refused") => "rejected",
                Some("unknown") => "unknown",
                Some("acknowledged" | "pending" | "submitted") => "submitted",
                _ => "completed",
            };
            json!({
                "id": ids.0,
                "contextId": context,
                "kind": "task",
                "status": {"state": state, "timestamp": timestamp},
                "artifacts": [{
                    "artifactId": ids.1,
                    "name": tool,
                    "parts": [{"kind": "data", "data": {"result": value}}],
                }],
                "history": [message],
            })
        }
        Err(error) => json!({
            "id": ids.0,
            "contextId": context,
            "kind": "task",
            "status": {
                "state": "rejected",
                "timestamp": timestamp,
                "message": {
                    "kind": "message",
                    "role": "agent",
                    "messageId": ids.1,
                    "taskId": ids.0,
                    "contextId": context,
                    "parts": [{"kind": "text", "text": error}],
                },
            },
            "history": [message],
        }),
    }
}

fn gateway_state(value: &Value) -> Option<&str> {
    value
        .get("rung")
        .or_else(|| value.pointer("/gateway/state"))
        .or_else(|| value.pointer("/gateway/result/state"))
        .and_then(Value::as_str)
}

fn cancel(tasks: &Mutex<TaskLedger>, identifier: Value, parameters: &Value) -> Value {
    let Some(name) = parameters.get("id").and_then(Value::as_str) else {
        return failure(identifier, -32602, "the request did not name a task");
    };
    let Ok(tasks) = tasks.lock() else {
        return failure(identifier, -32603, "the task ledger is unavailable");
    };
    let Some(task) = tasks.get(name) else {
        return failure(identifier, -32001, "the task was not found");
    };
    match task.pointer("/status/state").and_then(Value::as_str) {
        Some("submitted" | "unknown") => failure(
            identifier,
            -32004,
            "the submitted transaction cannot be canceled by the transport",
        ),
        _ => failure(
            identifier,
            -32002,
            "the task already reached a terminal state",
        ),
    }
}

fn fetch(tasks: &Mutex<TaskLedger>, identifier: Value, parameters: &Value) -> Value {
    let Some(name) = parameters.get("id").and_then(Value::as_str) else {
        return failure(identifier, -32602, "the request did not name a task");
    };
    let Ok(tasks) = tasks.lock() else {
        return failure(identifier, -32603, "the task ledger is unavailable");
    };
    match tasks.get(name) {
        Some(task) => success(identifier, task.clone()),
        None => failure(identifier, -32001, "the task was not found"),
    }
}

fn read_instruction(message: &Value) -> Result<Instruction, String> {
    let parts = message
        .get("parts")
        .and_then(Value::as_array)
        .ok_or_else(|| "the message carried no parts".to_string())?;
    for part in parts {
        let payload = match part.get("kind").and_then(Value::as_str) {
            Some("data") => part.get("data").cloned(),
            Some("text") => part
                .get("text")
                .and_then(Value::as_str)
                .and_then(|text| serde_json::from_str::<Value>(text).ok()),
            _ => None,
        };
        let Some(payload) = payload else {
            continue;
        };
        let Some(skill) = payload.get("skill").and_then(Value::as_str) else {
            continue;
        };
        return Ok(Instruction {
            skill: skill.to_owned(),
            arguments: payload.get("arguments").cloned().unwrap_or(Value::Null),
        });
    }
    Err("the message did not name a skill and its arguments".into())
}

fn read_headers(stream: &mut TcpStream) -> Result<(Vec<u8>, usize), String> {
    let mut bytes = Vec::with_capacity(4096);
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let read = stream
            .read(&mut chunk)
            .map_err(|error| format!("could not read the request: {error}"))?;
        if read == 0 {
            return Err("the connection closed before the request headers".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            let end = position + 4;
            if end > MAX_HEADER_BYTES {
                return Err("the request headers exceed the transport limit".into());
            }
            break end;
        }
        if bytes.len() > MAX_HEADER_BYTES {
            return Err("the request headers exceed the transport limit".into());
        }
    };
    Ok((bytes, header_end))
}

fn request_line(start: &str) -> Result<(&str, &str), String> {
    let mut start_fields = start.split(' ');
    let method = start_fields.next().unwrap_or_default();
    let path = start_fields.next().unwrap_or_default();
    if method.is_empty()
        || !method.bytes().all(|byte| byte.is_ascii_uppercase())
        || path.is_empty()
        || !path.starts_with('/')
        || path.starts_with("//")
        || path.contains(['?', '#', '\\', '\0'])
        || path.split('/').any(|segment| matches!(segment, "." | ".."))
        || start_fields.next() != Some("HTTP/1.1")
        || start_fields.next().is_some()
    {
        return Err("the request line is not canonical HTTP/1.1".into());
    }
    Ok((method, path))
}

struct RequestHead {
    request: Request,
    bytes: Zeroizing<Vec<u8>>,
    header_end: usize,
    length: usize,
    has_length: bool,
    content_type: Option<String>,
}

#[cfg(test)]
fn read_request(stream: &mut TcpStream) -> Result<Request, String> {
    let head = read_request_head(stream)?;
    read_request_body(stream, head)
}

fn read_request_head(stream: &mut TcpStream) -> Result<RequestHead, String> {
    let (bytes, header_end) = read_headers(stream)?;
    let bytes = Zeroizing::new(bytes);
    let headers = std::str::from_utf8(&bytes[..header_end])
        .map_err(|_| "the request headers are not UTF-8".to_string())?;
    let mut lines = headers.split("\r\n");
    let start = lines
        .next()
        .ok_or_else(|| "the request has no start line".to_string())?;
    let (method, path) = request_line(start)?;
    let mut length = 0_usize;
    let mut has_length = false;
    let mut has_host = false;
    let mut content_type = None;
    let mut authorization = None;
    for line in lines.filter(|line| !line.is_empty()) {
        let (name, value) = line
            .split_once(':')
            .ok_or_else(|| "the request contains a malformed header".to_string())?;
        if name.is_empty()
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
        {
            return Err("the request contains a malformed header name".into());
        }
        let value = value.trim_matches([' ', '\t']);
        if value
            .bytes()
            .any(|byte| byte.is_ascii_control() && byte != b'\t')
        {
            return Err("the request contains a malformed header value".into());
        }
        if name.eq_ignore_ascii_case("content-length") {
            if has_length || value.is_empty() || !value.bytes().all(|byte| byte.is_ascii_digit()) {
                return Err("the request contains an ambiguous content length".into());
            }
            length = value
                .parse()
                .map_err(|_| "the content length is not a number".to_string())?;
            has_length = true;
        } else if name.eq_ignore_ascii_case("host") {
            if has_host || value.is_empty() {
                return Err("the request contains an ambiguous host".into());
            }
            has_host = true;
        } else if name.eq_ignore_ascii_case("content-type") {
            if content_type.is_some() {
                return Err("the request contains an ambiguous content type".into());
            }
            content_type = Some(value.to_owned());
        } else if name.eq_ignore_ascii_case("authorization") {
            if authorization.is_some() || value.len() > 256 {
                return Err("the request contains ambiguous authorization".into());
            }
            authorization = Some(Zeroizing::new(value.to_owned()));
        } else if name.eq_ignore_ascii_case("transfer-encoding") {
            return Err("transfer encoding is not supported".into());
        }
    }
    if !has_host {
        return Err("the request has no host header".into());
    }
    let request = Request {
        method: method.to_owned(),
        path: path.to_owned(),
        authorization,
        body: Vec::new(),
    };
    Ok(RequestHead {
        request,
        bytes,
        header_end,
        length,
        has_length,
        content_type,
    })
}

fn read_request_body(stream: &mut TcpStream, head: RequestHead) -> Result<Request, String> {
    let RequestHead {
        mut request,
        mut bytes,
        header_end,
        length,
        has_length,
        content_type,
    } = head;
    if request.method == "POST"
        && (!has_length || content_type.as_deref() != Some("application/json"))
    {
        return Err("POST requires one application/json body with an explicit length".into());
    }
    if request.method != "POST" && length != 0 {
        return Err("a non-POST request may not carry a body".into());
    }
    if length > MAX_REQUEST_BYTES {
        return Err("the request body exceeds the transport limit".into());
    }
    let mut chunk = [0_u8; 4096];
    while bytes.len() - header_end < length {
        let read = stream
            .read(&mut chunk)
            .map_err(|error| format!("could not read the request body: {error}"))?;
        if read == 0 {
            return Err("the connection closed before the request body".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
        if bytes.len() > MAX_REQUEST_BYTES.saturating_add(header_end) {
            return Err("the request exceeds the transport limit".into());
        }
    }
    if bytes.len() != header_end + length {
        return Err("the request carries bytes beyond its declared body".into());
    }
    request.body = bytes[header_end..header_end + length].to_vec();
    Ok(request)
}

fn write_response(stream: &mut TcpStream, response: &Response) -> std::io::Result<()> {
    let reason = match response.status {
        200 => "OK",
        400 => "Bad Request",
        401 => "Unauthorized",
        404 => "Not Found",
        405 => "Method Not Allowed",
        503 => "Service Unavailable",
        _ => "Error",
    };
    write!(
        stream,
        "HTTP/1.1 {} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n",
        response.status,
        response.body.len()
    )?;
    stream.write_all(&response.body)
}

fn encode(status: u16, value: &Value) -> Response {
    let body = serde_json::to_vec(value)
        .unwrap_or_else(|_| b"{\"error\":\"result encoding failed\"}".to_vec());
    Response { status, body }
}

fn success(identifier: Value, result: Value) -> Value {
    let mut envelope = Map::new();
    envelope.insert("jsonrpc".to_owned(), Value::String("2.0".to_owned()));
    envelope.insert("id".to_owned(), identifier);
    envelope.insert("result".to_owned(), result);
    Value::Object(envelope)
}

fn failure(identifier: Value, code: i32, detail: &str) -> Value {
    let mut error = Map::new();
    error.insert("code".to_owned(), Value::from(code));
    error.insert("message".to_owned(), Value::String(detail.to_owned()));
    let mut envelope = Map::new();
    envelope.insert("jsonrpc".to_owned(), Value::String("2.0".to_owned()));
    envelope.insert("id".to_owned(), identifier);
    envelope.insert("error".to_owned(), Value::Object(error));
    Value::Object(envelope)
}

fn new_identifier() -> Result<String, String> {
    let mut bytes = [0_u8; 16];
    getrandom::fill(&mut bytes)
        .map_err(|error| format!("operating-system randomness failed: {error}"))?;
    Ok(hex_encode(&bytes))
}

fn timestamp() -> Result<String, String> {
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "the system clock is before the Unix epoch".to_owned())?
        .as_secs();
    let days = seconds / 86_400;
    let remainder = seconds % 86_400;
    let (year, month, day) = civil_date(days);
    let hour = remainder / 3_600;
    let minute = (remainder % 3_600) / 60;
    let second = remainder % 60;
    Ok(format!(
        "{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}Z"
    ))
}

pub fn start_installed(
    command: &str,
    arguments: &[String],
    variables: &BTreeMap<String, String>,
) -> Result<Value, String> {
    if !cfg!(target_os = "linux") {
        return Err("managed A2A lifecycle is currently supported on Linux runtimes".into());
    }
    if variables
        .keys()
        .any(|name| !matches!(name.as_str(), "LAYERX_CONFIG" | "LAYERX_GATEWAY_KEY_ID"))
    {
        return Err("installed A2A runtime environment contains an unsupported variable".into());
    }
    let state_path = state_path()?;
    let digest = launch_digest(command, arguments, variables);
    if let Some(state) = read_state(&state_path)? {
        if process_live(&state) && state.launch_digest == digest {
            return Ok(json!({
                "state": "running",
                "pid": state.pid,
                "changed": false,
                "state_file": state_path.display().to_string(),
            }));
        }
        if process_live(&state) {
            return Err(
                "an A2A process is already running with different installed arguments; stop it before reinstalling"
                    .into(),
            );
        }
        fs::remove_file(&state_path)
            .map_err(|error| format!("could not clear stale {}: {error}", state_path.display()))?;
    }
    let mut process = Command::new(command);
    process
        .args(arguments)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .env_remove("LAYERX_TOKEN")
        .env_remove("LAYERX_API_TOKEN")
        .env_remove("LAYERX_AUTH_TOKEN");
    for (name, value) in variables {
        process.env(name, value);
    }
    let mut child = process
        .spawn()
        .map_err(|error| format!("could not start installed A2A runtime: {error}"))?;
    let process_start_ticks = match process_start_ticks(child.id()) {
        Ok(value) => value,
        Err(error) => {
            let _ = child.kill();
            return Err(error);
        }
    };
    let state = ServiceState {
        pid: child.id(),
        process_start_ticks,
        launch_digest: digest,
    };
    if let Err(error) = wait_installed_ready(&mut child, arguments) {
        let _ = child.kill();
        let _ = child.wait();
        return Err(error);
    }
    if let Err(error) = write_state(&state_path, &state) {
        let _ = child.kill();
        let _ = child.wait();
        return Err(error);
    }
    Ok(json!({
        "state": "running",
        "pid": state.pid,
        "changed": true,
        "state_file": state_path.display().to_string(),
    }))
}

fn wait_installed_ready(
    child: &mut std::process::Child,
    arguments: &[String],
) -> Result<(), String> {
    let argument = |name: &str| {
        arguments
            .windows(2)
            .find(|pair| pair[0] == name)
            .map(|pair| pair[1].as_str())
    };
    let listen = argument("--listen")
        .ok_or_else(|| "installed runtime omitted its listen address".to_owned())?;
    let environment = argument("--environment")
        .ok_or_else(|| "installed runtime omitted its environment".to_owned())?;
    let address = endpoint(listen)?;
    let mode = if arguments.iter().any(|arg| arg == "--read-only") {
        DeploymentMode::ReadOnly
    } else {
        DeploymentMode::Full
    };
    let expected = agent_card(environment, listen, mode, &toolset::surface(mode)?);
    let deadline = std::time::Instant::now() + Duration::from_secs(5);
    while std::time::Instant::now() < deadline {
        if child
            .try_wait()
            .map_err(|error| format!("could not inspect installed runtime: {error}"))?
            .is_some()
        {
            return Err("the installed A2A runtime exited before readiness".to_owned());
        }
        if let Ok(mut stream) = TcpStream::connect_timeout(&address, Duration::from_millis(250)) {
            stream
                .set_read_timeout(Some(Duration::from_millis(250)))
                .map_err(|error| error.to_string())?;
            stream
                .set_write_timeout(Some(Duration::from_millis(250)))
                .map_err(|error| error.to_string())?;
            let request = format!("GET /.well-known/agent-card.json HTTP/1.1\r\nHost: {listen}\r\nConnection: close\r\n\r\n");
            if stream.write_all(request.as_bytes()).is_ok() {
                let mut response = Vec::new();
                if stream.take(65_537).read_to_end(&mut response).is_ok()
                    && response.len() <= 65_536
                {
                    if let Some(end) = response.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
                        let matches = response.starts_with(b"HTTP/1.1 200 ")
                            && serde_json::from_slice::<Value>(&response[end + 4..])
                                .ok()
                                .as_ref()
                                == Some(&expected);
                        if matches
                            && child
                                .try_wait()
                                .map_err(|error| error.to_string())?
                                .is_none()
                        {
                            return Ok(());
                        }
                    }
                }
            }
        }
        std::thread::sleep(Duration::from_millis(50));
    }
    Err("the installed A2A runtime did not publish its exact bound agent card before readiness deadline".to_owned())
}

pub fn start_from_manifest() -> Result<Value, String> {
    let manifest_path = crate::install::layerx_directory()?.join("a2a/runtime.json");
    let source = fs::read_to_string(&manifest_path)
        .map_err(|error| format!("could not read {}: {error}", manifest_path.display()))?;
    let document: Value = serde_json::from_str(&source)
        .map_err(|error| format!("could not parse {}: {error}", manifest_path.display()))?;
    let entry = document
        .pointer("/agents/layerx")
        .ok_or_else(|| "installed A2A runtime manifest has no LayerX entry".to_owned())?;
    let command = entry
        .get("command")
        .and_then(Value::as_str)
        .ok_or_else(|| "installed A2A runtime command is missing".to_owned())?;
    let arguments = entry
        .get("args")
        .and_then(Value::as_array)
        .ok_or_else(|| "installed A2A runtime arguments are missing".to_owned())?
        .iter()
        .map(|value| {
            value
                .as_str()
                .map(str::to_owned)
                .ok_or_else(|| "installed A2A runtime argument is not text".to_owned())
        })
        .collect::<Result<Vec<_>, _>>()?;
    let variables = entry
        .get("env")
        .and_then(Value::as_object)
        .ok_or_else(|| "installed A2A runtime environment is missing".to_owned())?
        .iter()
        .map(|(name, value)| {
            value
                .as_str()
                .map(|value| (name.clone(), value.to_owned()))
                .ok_or_else(|| "installed A2A runtime environment value is not text".to_owned())
        })
        .collect::<Result<BTreeMap<_, _>, _>>()?;
    start_installed(command, &arguments, &variables)
}

pub fn stop_installed() -> Result<Value, String> {
    let path = state_path()?;
    let Some(state) = read_state(&path)? else {
        return Ok(json!({"state": "stopped", "changed": false}));
    };
    if process_live(&state) {
        let status = Command::new("kill")
            .arg(state.pid.to_string())
            .status()
            .map_err(|error| format!("could not stop A2A process {}: {error}", state.pid))?;
        if !status.success() {
            return Err(format!(
                "A2A process {} refused the stop request",
                state.pid
            ));
        }
    }
    fs::remove_file(&path)
        .map_err(|error| format!("could not remove {}: {error}", path.display()))?;
    Ok(json!({"state": "stopped", "pid": state.pid, "changed": true}))
}

pub fn installed_status() -> Result<Value, String> {
    let path = state_path()?;
    let Some(state) = read_state(&path)? else {
        return Ok(json!({"state": "stopped"}));
    };
    Ok(json!({
        "state": if process_live(&state) { "running" } else { "stale" },
        "pid": state.pid,
        "state_file": path.display().to_string(),
    }))
}

fn state_path() -> Result<std::path::PathBuf, String> {
    Ok(crate::install::layerx_directory()?
        .join("a2a")
        .join(STATE_FILE))
}

fn process_live(state: &ServiceState) -> bool {
    cfg!(target_os = "linux")
        && process_start_ticks(state.pid).ok() == Some(state.process_start_ticks)
}

fn process_start_ticks(pid: u32) -> Result<u64, String> {
    let path = std::path::Path::new("/proc")
        .join(pid.to_string())
        .join("stat");
    let source = fs::read_to_string(&path)
        .map_err(|error| format!("could not inspect started A2A process {pid}: {error}"))?;
    let (_, fields) = source
        .rsplit_once(") ")
        .ok_or_else(|| format!("{} has an invalid process record", path.display()))?;
    fields
        .split_whitespace()
        .nth(19)
        .ok_or_else(|| format!("{} has no process start identity", path.display()))?
        .parse()
        .map_err(|_| format!("{} has an invalid process start identity", path.display()))
}

fn read_state(path: &std::path::Path) -> Result<Option<ServiceState>, String> {
    match fs::read_to_string(path) {
        Ok(source) => serde_json::from_str(&source)
            .map(Some)
            .map_err(|error| format!("could not parse {}: {error}", path.display())),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(format!("could not read {}: {error}", path.display())),
    }
}

fn write_state(path: &std::path::Path, state: &ServiceState) -> Result<(), String> {
    let encoded = serde_json::to_string(state)
        .map_err(|error| format!("could not encode A2A service state: {error}"))?;
    crate::install::publish(
        path,
        &serde_json::from_str(&encoded)
            .map_err(|error| format!("could not encode A2A service state: {error}"))?,
    )
    .map(|_| ())
}

fn launch_digest(
    command: &str,
    arguments: &[String],
    variables: &BTreeMap<String, String>,
) -> String {
    let mut digest = Sha256::new();
    digest.update((command.len() as u64).to_be_bytes());
    digest.update(command.as_bytes());
    for argument in arguments {
        digest.update((argument.len() as u64).to_be_bytes());
        digest.update(argument.as_bytes());
    }
    for (name, value) in variables {
        digest.update((name.len() as u64).to_be_bytes());
        digest.update(name.as_bytes());
        digest.update((value.len() as u64).to_be_bytes());
        digest.update(value.as_bytes());
    }
    hex_encode(&digest.finalize())
}

fn civil_date(days: u64) -> (u64, u64, u64) {
    let shifted = days + 719_468;
    let era = shifted / 146_097;
    let day_of_era = shifted % 146_097;
    let year_of_era =
        (day_of_era - day_of_era / 1_460 + day_of_era / 36_524 - day_of_era / 146_096) / 365;
    let civil_year = year_of_era + era * 400;
    let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
    let shifted_month = (5 * day_of_year + 2) / 153;
    let day_of_month = day_of_year - (153 * shifted_month + 2) / 5 + 1;
    let month = if shifted_month < 10 {
        shifted_month + 3
    } else {
        shifted_month - 9
    };
    let year = if month <= 2 {
        civil_year + 1
    } else {
        civil_year
    };
    (year, month, day_of_month)
}

#[cfg(test)]
mod submission_outcome_tests {
    use std::io::Read as _;
    use std::net::{Shutdown, TcpListener};
    use std::sync::Mutex;
    use std::thread::{self, JoinHandle};
    use std::time::{Duration, Instant};

    use layerx_crypto::evm_transaction::{decode_signed, transaction_hash, Eip1559Call};
    use layerx_mcp::server::{DeploymentMode, ToolDefinition};
    use serde_json::{json, Value};

    use super::{dispatch, encode, read_headers, write_response, MAX_REQUEST_BYTES};
    use super::{Runtime, TaskLedger};
    use crate::encoding::hex_encode;
    use crate::toolset;

    fn node_response(
        listener: &TcpListener,
        response: Option<Value>,
    ) -> std::io::Result<JoinHandle<Result<Value, String>>> {
        let listener = listener.try_clone()?;
        Ok(thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(5);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        if Instant::now() >= deadline {
                            return Err("timed out waiting for submission".into());
                        }
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => return Err(error.to_string()),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .and_then(|()| stream.set_write_timeout(Some(Duration::from_secs(5))))
                .map_err(|error| error.to_string())?;
            let (mut bytes, header_end) = read_headers(&mut stream)?;
            let headers =
                std::str::from_utf8(&bytes[..header_end]).map_err(|error| error.to_string())?;
            assert_eq!(headers.lines().next(), Some("POST / HTTP/1.1"));
            let fields: Vec<_> = headers
                .lines()
                .filter_map(|line| line.split_once(':'))
                .map(|(name, value)| (name.to_ascii_lowercase(), value.trim()))
                .collect();
            assert!(
                fields.contains(&("content-type".to_owned(), "application/json; charset=utf-8"))
            );
            assert!(!fields.iter().any(|(name, _)| name == "transfer-encoding"));
            let lengths: Vec<_> = fields
                .iter()
                .filter(|(name, _)| name == "content-length")
                .collect();
            assert_eq!(lengths.len(), 1);
            let length: usize = lengths[0]
                .1
                .parse::<usize>()
                .map_err(|error| error.to_string())?;
            assert!(length <= MAX_REQUEST_BYTES);
            let received = bytes.len() - header_end;
            assert!(received <= length);
            bytes.resize(header_end + length, 0);
            stream
                .read_exact(&mut bytes[header_end + received..])
                .map_err(|error| error.to_string())?;
            let body =
                serde_json::from_slice(&bytes[header_end..]).map_err(|error| error.to_string())?;
            if let Some(response) = response {
                write_response(&mut stream, &encode(200, &response))
                    .map_err(|error| error.to_string())?;
            }
            stream
                .shutdown(Shutdown::Both)
                .map_err(|error| error.to_string())?;
            Ok(body)
        }))
    }

    fn call(
        runtime: &Runtime,
        tools: &[ToolDefinition],
        tasks: &Mutex<TaskLedger>,
        method: &str,
        params: &Value,
    ) -> Value {
        dispatch(
            runtime,
            tools,
            tasks,
            &serde_json::to_vec(&json!({
                "jsonrpc": "2.0", "id": 1, "method": method, "params": params,
            }))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "serializable request")),
        )
    }

    fn message(arguments: &Value) -> Value {
        json!({"message": {
            "kind": "message", "role": "user", "messageId": "submission",
            "parts": [{"kind": "data", "data": {
                "skill": "activity.submit", "arguments": arguments,
            }}],
        }})
    }

    struct SubmissionFixture<'a> {
        runtime: &'a Runtime,
        tools: &'a [ToolDefinition],
        tasks: &'a Mutex<TaskLedger>,
        listener: &'a TcpListener,
        submit: ToolDefinition,
        encoded: &'a str,
        hash: &'a str,
        arguments: &'a Value,
    }

    impl SubmissionFixture<'_> {
        fn assert_case(
            &self,
            response: Option<Value>,
            state: &str,
        ) -> Result<(), Box<dyn std::error::Error>> {
            let node = node_response(self.listener, response.clone())?;
            let sent = call(
                self.runtime,
                self.tools,
                self.tasks,
                "message/send",
                &message(self.arguments),
            );
            assert_eq!(
                node.join().map_err(|_| "node reader panicked")??,
                json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":[self.encoded]})
            );
            let task = &sent["result"];
            assert_eq!(task["status"]["state"], state, "response: {response:?}");
            if state == "unknown" || state == "submitted" {
                let result = &task["artifacts"][0]["parts"][0]["data"]["result"];
                assert_eq!(result["txHash"], self.hash);
                assert_eq!(result["rung"], state);
                if state == "unknown" {
                    assert!(result["detail"]
                        .as_str()
                        .is_some_and(|detail| !detail.is_empty()));
                }
                let fetched = call(
                    self.runtime,
                    self.tools,
                    self.tasks,
                    "tasks/get",
                    &json!({"id":task["id"]}),
                );
                assert_eq!(&fetched["result"], task);
                let canceled = call(
                    self.runtime,
                    self.tools,
                    self.tasks,
                    "tasks/cancel",
                    &json!({"id":task["id"]}),
                );
                assert_eq!(canceled["error"]["code"], -32004);
                assert_eq!(
                    call(
                        self.runtime,
                        self.tools,
                        self.tasks,
                        "tasks/get",
                        &json!({"id":task["id"]})
                    )["result"],
                    *task
                );
            } else {
                assert!(task.get("artifacts").is_none());
                assert!(task["status"]["message"]["parts"][0]["text"]
                    .as_str()
                    .is_some_and(|detail| detail.contains("nonce too low")));
                assert_eq!(task["txHash"], self.hash);
                assert_eq!(
                    call(
                        self.runtime,
                        self.tools,
                        self.tasks,
                        "tasks/get",
                        &json!({"id":task["id"]})
                    )["result"],
                    *task
                );
            }
            assert!(
                matches!(self.listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
            );

            let node = node_response(self.listener, response)?;
            let mcp = self.runtime.invoke(self.submit, self.arguments);
            assert_eq!(
                node.join().map_err(|_| "node reader panicked")??,
                json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":[self.encoded]})
            );
            if state == "submitted" {
                assert_eq!(mcp?, json!({"txHash":self.hash, "rung":"submitted"}));
            } else {
                assert!(mcp.is_err());
            }
            assert!(
                matches!(self.listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
            );
            Ok(())
        }
    }

    fn assert_invalid_submissions(
        fixture: &SubmissionFixture<'_>,
        call_data: Eip1559Call,
        raw: &[u8],
    ) -> Result<(), Box<dyn std::error::Error>> {
        let wrong_chain = Eip1559Call {
            chain_id: 126,
            ..call_data
        }
        .sign(&[1; 32])?;
        let mut bad_signature = raw.to_vec();
        let signature_start = bad_signature.len() - 32;
        bad_signature[signature_start..].fill(0xff);
        assert!(decode_signed(&bad_signature).is_err());
        for invalid in [
            json!({"raw_transaction":"0x02zz"}),
            json!({"raw_transaction":"0x02c0"}),
            json!({"raw_transaction":fixture.encoded.trim_start_matches("0x")}),
            json!({"raw_transaction":fixture.encoded, "extra":true}),
            json!({"raw_transaction":format!("0x{}", hex_encode(&bad_signature))}),
            json!({"raw_transaction":format!("0x{}", hex_encode(&wrong_chain))}),
        ] {
            let sent = call(
                fixture.runtime,
                fixture.tools,
                fixture.tasks,
                "message/send",
                &message(&invalid),
            );
            assert_eq!(sent["result"]["status"]["state"], "rejected");
            assert!(sent["result"].get("artifacts").is_none());
            assert!(fixture.runtime.invoke(fixture.submit, &invalid).is_err());
            assert!(
                matches!(fixture.listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
            );
        }
        Ok(())
    }

    #[test]
    fn a2a_response_loss_keeps_unknown_outcome() -> Result<(), Box<dyn std::error::Error>> {
        let call_data = Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
            to: [0x22; 20],
            data: vec![0xab; 512],
        };
        let raw = call_data.sign(&[1; 32])?;
        let encoded = format!("0x{}", hex_encode(&raw));
        let hash = format!("0x{}", hex_encode(&transaction_hash(&raw)));
        let arguments = json!({"raw_transaction": encoded});
        let tools = toolset::surface(DeploymentMode::Full)?;
        let submit = tools
            .iter()
            .find(|tool| tool.name == "activity.submit")
            .copied()
            .ok_or("missing submit tool")?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let runtime = Runtime::loopback(&format!("http://{}", listener.local_addr()?), 125)?;
        let tasks = Mutex::new(TaskLedger::memory());
        let fixture = SubmissionFixture {
            runtime: &runtime,
            tools: &tools,
            tasks: &tasks,
            listener: &listener,
            submit,
            encoded: &encoded,
            hash: &hash,
            arguments: &arguments,
        };
        for (response, state) in [
            (None, "unknown"),
            (
                Some(json!({"jsonrpc":"2.0", "id":1, "result":null})),
                "unknown",
            ),
            (
                Some(json!({"jsonrpc":"2.0", "id":1, "result":"0x12"})),
                "unknown",
            ),
            (
                Some(json!({"jsonrpc":"2.0", "id":1, "result":format!("0x{}", "00".repeat(32))})),
                "unknown",
            ),
            (
                Some(json!({"jsonrpc":"2.0", "id":2, "result":hash})),
                "unknown",
            ),
            (
                Some(
                    json!({"jsonrpc":"1.0", "id":1, "error":{"code":-32000, "message":"nonce too low"}}),
                ),
                "unknown",
            ),
            (
                Some(
                    json!({"jsonrpc":"2.0", "id":2, "error":{"code":-32000, "message":"nonce too low"}}),
                ),
                "unknown",
            ),
            (
                Some(
                    json!({"jsonrpc":"2.0", "id":1, "result":null, "error":{"code":-32000, "message":"nonce too low"}}),
                ),
                "unknown",
            ),
            (
                Some(
                    json!({"jsonrpc":"2.0", "id":1, "error":{"code":"-32000", "message":"nonce too low"}}),
                ),
                "unknown",
            ),
            (
                Some(json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000, "message":null}})),
                "unknown",
            ),
            (
                Some(
                    json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000, "message":"nonce too low"}}),
                ),
                "rejected",
            ),
            (
                Some(json!({"jsonrpc":"2.0", "id":1, "result":hash})),
                "submitted",
            ),
        ] {
            fixture.assert_case(response, state)?;
        }
        assert_invalid_submissions(&fixture, call_data, &raw)?;
        Ok(())
    }
}

#[cfg(test)]
mod request_boundary_tests {
    use std::io::Write as _;
    use std::net::{TcpListener, TcpStream};
    use std::thread;

    use zeroize::Zeroizing;

    use super::{authorized, read_request, Request};

    fn parse(raw: Vec<u8>) -> Result<(), String> {
        let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
        let address = listener.local_addr().map_err(|error| error.to_string())?;
        let writer = thread::spawn(move || -> Result<(), String> {
            let mut stream = TcpStream::connect(address).map_err(|error| error.to_string())?;
            stream.write_all(&raw).map_err(|error| error.to_string())
        });
        let (mut stream, _) = listener.accept().map_err(|error| error.to_string())?;
        let parsed = read_request(&mut stream).map(|_| ());
        writer
            .join()
            .map_err(|_| "request writer panicked".to_owned())??;
        parsed
    }

    #[test]
    fn rejects_ambiguous_http_framing() {
        assert!(parse(b"POST / HTTP/1.0\r\nHost: localhost\r\nContent-Length: 0\r\nContent-Type: application/json\r\n\r\n".to_vec()).is_err());
        assert!(parse(b"POST / HTTP/1.1\r\nHost: localhost\r\nTransfer-Encoding: chunked\r\nContent-Length: 0\r\nContent-Type: application/json\r\n\r\n".to_vec()).is_err());
        assert!(parse(b"POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\nContent-Length: 0\r\nContent-Type: application/json\r\n\r\n".to_vec()).is_err());
        assert!(parse(b"POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\nContent-Type: application/json\r\n\r\nsurplus".to_vec()).is_err());
    }

    #[test]
    fn rejects_query_aliases_and_oversized_headers() {
        assert!(parse(
            b"GET /.well-known/agent-card.json?write=true HTTP/1.1\r\nHost: localhost\r\n\r\n"
                .to_vec()
        )
        .is_err());
        let mut request = b"GET / HTTP/1.1\r\nHost: localhost\r\nX-Fill: ".to_vec();
        request.extend(std::iter::repeat_n(b'a', super::MAX_HEADER_BYTES));
        request.extend_from_slice(b"\r\n\r\n");
        assert!(parse(request).is_err());
    }

    #[test]
    fn binds_mutation_authorization_to_the_exact_installed_bearer() {
        let request = Request {
            method: "POST".to_owned(),
            path: "/".to_owned(),
            authorization: Some(Zeroizing::new("Bearer lxa2a_presented".to_owned())),
            body: Vec::new(),
        };
        assert!(authorized(&request, "lxa2a_presented"));
        assert!(!authorized(&request, "lxa2a_other"));

        let absent = Request {
            method: "POST".to_owned(),
            path: "/".to_owned(),
            authorization: None,
            body: Vec::new(),
        };
        assert!(!authorized(&absent, "lxa2a_presented"));
    }
}
