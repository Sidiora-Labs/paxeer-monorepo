use std::io::{self, BufRead, Read as _, Write};
use std::path::Path;

use layerx_mcp::binding::Binding;
use layerx_mcp::server::{DeploymentMode, ToolKind};
use layerx_mcp::stdio::DaemonClientSession;
use serde_json::{json, Value};

const MAX_MESSAGE_BYTES: usize = 1_048_576;

/// A client may narrow a daemon-authorized surface without changing the daemon's binding.
pub(crate) struct InstalledSession {
    daemon: DaemonClientSession,
    read_only: bool,
}

impl InstalledSession {
    pub(crate) fn open(binding: &Binding, read_only: bool) -> Result<Self, String> {
        let daemon = binding
            .open_daemon_client()
            .map_err(|error| error.detail())?;
        Ok(Self {
            daemon,
            read_only: read_only || binding.mode() == DeploymentMode::ReadOnly,
        })
    }

    fn readable(name: &str) -> bool {
        layerx_mcp::catalogue::surface(DeploymentMode::ReadOnly)
            .into_iter()
            .chain(layerx_mcp::catalogue::web_surface())
            .any(|tool| tool.name == name && tool.kind == ToolKind::Read)
    }

    pub(crate) fn handle(&mut self, message: &str) -> Option<Value> {
        if !self.read_only {
            return self.daemon.handle(message);
        }
        let request = serde_json::from_str::<Value>(message).ok();
        let method = request.as_ref().and_then(|value| value["method"].as_str());
        if method == Some("tools/call") {
            if let Some(name) = request
                .as_ref()
                .and_then(|value| value.pointer("/params/name"))
                .and_then(Value::as_str)
            {
                if !Self::readable(name) {
                    let id = request
                        .as_ref()
                        .and_then(|value| value.get("id"))
                        .cloned()
                        .unwrap_or(Value::Null);
                    return (!id.is_null()).then(|| json!({
                        "jsonrpc": "2.0", "id": id,
                        "error": {"code": -32602, "message": "the tool is not served by this read-only deployment"}
                    }));
                }
            }
        }
        let mut response = self.daemon.handle(message)?;
        if method == Some("tools/list") {
            if let Some(tools) = response
                .pointer_mut("/result/tools")
                .and_then(Value::as_array_mut)
            {
                tools.retain(|tool| tool["name"].as_str().is_some_and(Self::readable));
            }
        } else if method == Some("initialize") {
            if let Some(meta) = response
                .pointer_mut("/result/_meta")
                .and_then(Value::as_object_mut)
            {
                meta.insert("layerx/deployment_mode".into(), json!("read-only"));
                meta.insert("layerx/write_tools".into(), json!(0));
                meta.insert("layerx/mutations_reachable".into(), json!(false));
            }
        }
        Some(response)
    }

    fn serve<R: BufRead, W: Write>(
        &mut self,
        reader: &mut R,
        writer: &mut W,
    ) -> Result<(), String> {
        let mut line = String::new();
        loop {
            line.clear();
            let read = reader
                .by_ref()
                .take(MAX_MESSAGE_BYTES as u64)
                .read_line(&mut line)
                .map_err(|error| format!("could not read a protocol message: {error}"))?;
            if read == 0 {
                return Ok(());
            }
            if read >= MAX_MESSAGE_BYTES && !line.ends_with('\n') {
                return Err("a protocol message exceeded the transport limit".into());
            }
            if line.trim().is_empty() {
                continue;
            }
            if let Some(response) = self.handle(line.trim()) {
                let encoded =
                    serde_json::to_string(&response).map_err(|error| error.to_string())?;
                writeln!(writer, "{encoded}")
                    .and_then(|()| writer.flush())
                    .map_err(|error| format!("could not write a protocol message: {error}"))?;
            }
        }
    }
}

/// Serves the live daemon's catalogue, optionally narrowed to read-only tools.
pub fn serve(binding: &Path, read_only: bool) -> Result<(), String> {
    serve_io(
        binding,
        read_only,
        &mut io::stdin().lock(),
        &mut io::stdout().lock(),
    )
}

/// Runs the same installed stdio protocol with caller-owned streams.
pub fn serve_io<R: BufRead, W: Write>(
    binding: &Path,
    read_only: bool,
    reader: &mut R,
    writer: &mut W,
) -> Result<(), String> {
    let declared = Binding::open(binding).map_err(|error| error.detail())?;
    let mut session = InstalledSession::open(&declared, read_only)?;
    session.serve(reader, writer)
}
