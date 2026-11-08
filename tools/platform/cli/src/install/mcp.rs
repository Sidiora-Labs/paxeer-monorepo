use std::collections::BTreeMap;
use std::env;
use std::fs::Metadata;
use std::io::Read as _;
use std::path::{Path, PathBuf};

use layerx_mcp::binding::Binding;
use layerx_mcp::server::{DeploymentMode, ToolKind};
use serde_json::{json, Value};

use crate::config;
use crate::toolset;

use super::binding_shape::{BindingDocument, MAX_DOCUMENT_BYTES};
use super::{apply, executable, hosts, report, FileTransaction, Registration, SERVER_NAME};

pub struct Request {
    pub hosts: Vec<String>,
    pub read_only: bool,
    pub daemon_binding: Option<PathBuf>,
}

/// Installs and registers the daemon-bound `LayerX` model context protocol server.
///
/// The installed launch line names the binding document that agent-daemon enrolment wrote; the
/// installer opens that document exactly as the served path does and refuses when it is absent,
/// so a registration never points at a catalogue the daemon will not serve. No gateway key,
/// environment, or payment flag takes part: the daemon holds every credential the tools need.
///
/// Returns the agent-daemon endpoint the registration is bound to and the recorded document.
///
/// # Errors
///
/// Returns the operator message when no host is selected, when the binding document cannot be
/// opened, or when a registration cannot be published.
pub fn platform_install_mcp(request: &Request) -> Result<(String, Value), String> {
    let selected_hosts = hosts(&request.hosts)?;
    if selected_hosts.is_empty() {
        return Err("no agent runtime was selected for installation".into());
    }
    let daemon_binding = match &request.daemon_binding {
        Some(explicit) => absolute_binding_path(explicit)?,
        None => daemon_binding_path()?,
    };
    let (binding, declaration) = open_binding(&daemon_binding).map_err(|error| {
        format!(
            "the daemon binding document at {} could not be used: {}; agent-daemon enrolment writes it before the MCP server is installed",
            daemon_binding.display(),
            error
        )
    })?;
    let declared_mode = binding.mode();
    let mode = if request.read_only {
        DeploymentMode::ReadOnly
    } else {
        declared_mode
    };
    let mut session =
        crate::mcp::InstalledSession::open(&binding, request.read_only).map_err(|error| {
            format!(
                "the authenticated agent daemon did not admit this installation binding: {error}"
            )
        })?;
    let description = session
        .handle(r#"{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}"#)
        .ok_or_else(|| "the authenticated daemon returned no tool catalogue".to_owned())?;
    let listings = description
        .pointer("/result/tools")
        .and_then(Value::as_array)
        .ok_or_else(|| "the authenticated daemon refused its tool catalogue".to_owned())?;
    let mut definitions = toolset::daemon_surface(mode)?;
    definitions.extend(
        layerx_mcp::catalogue::web_surface()
            .into_iter()
            .filter(|tool| mode == DeploymentMode::Full || tool.kind == ToolKind::Read),
    );
    let tools = listings
        .iter()
        .map(|listing| {
            let name = listing
                .get("name")
                .and_then(Value::as_str)
                .ok_or_else(|| "the daemon catalogue omitted a tool name".to_owned())?;
            definitions
                .iter()
                .find(|tool| tool.name == name)
                .copied()
                .ok_or_else(|| {
                    "the daemon returned a tool outside its installation catalogue".to_owned()
                })
        })
        .collect::<Result<Vec<_>, String>>()?;
    if mode == DeploymentMode::Full
        && [
            "activity.prepare",
            "activity.disclose",
            "activity.sign",
            "activity.submit",
            "activity.track",
        ]
        .iter()
        .any(|name| !tools.iter().any(|tool| tool.name == *name))
    {
        return Err(
            "the authenticated daemon has not admitted the complete payment journey".to_owned(),
        );
    }
    let command = executable()?;
    let variables: BTreeMap<String, String> = BTreeMap::new();
    let agent_endpoint = binding.agent_endpoint().to_owned();
    let binding_path = path_text(&daemon_binding)?;
    let arguments = launch_arguments(&binding_path, request.read_only);
    let descriptors = tools
        .iter()
        .copied()
        .map(toolset::daemon_descriptor)
        .collect::<Result<Vec<Value>, String>>()?;
    let mut pending = Vec::new();
    for host in selected_hosts {
        let path = host.path()?;
        pending.push((
            host,
            Registration {
                path,
                section: host.section(),
                name: SERVER_NAME.to_owned(),
                entry: host.entry(&command, &arguments, &variables),
            },
        ));
    }
    let (registrations, changed) = publish_registrations(&pending)?;
    let document = json!({
        "component": "mcp",
        "transport": "stdio",
        "authorization": "agent-daemon",
        "deployment_mode": toolset::mode_name(mode),
        "daemon_binding": {
            "path": binding_path,
            "tenant": binding.tenant(),
            "declared_mode": toolset::mode_name(declared_mode),
            "agent_endpoint": &agent_endpoint,
            "store": path_text(binding.store())?,
            "session_generation": binding.session_generation(),
            "core_height": binding.core_height(),
            "readiness": declaration.agent.readiness,
        },
        "server": {
            "name": SERVER_NAME,
            "command": command,
            "args": arguments,
            "env": variables,
        },
        "tools": descriptors,
        "scopes": toolset::scopes(&tools),
        "registrations": registrations,
        "changed": changed,
        "idempotent": true,
    });
    Ok((agent_endpoint, document))
}

fn open_binding(path: &Path) -> Result<(Binding, BindingDocument), String> {
    let canonical = std::fs::canonicalize(path).map_err(|error| error.to_string())?;
    if canonical != path {
        return Err("the binding document path is not canonical".to_owned());
    }
    let before = super::private_file_metadata(path)?;
    let (mut file, opened) = super::open_stable_installation_file(path)?
        .ok_or_else(|| "the binding document is absent".to_owned())?;
    let file_before = file.metadata().map_err(|error| error.to_string())?;
    if !same_binding_snapshot(&before, &opened) || !same_binding_snapshot(&opened, &file_before) {
        return Err("the binding document changed while opening it".to_owned());
    }
    let mut source = String::new();
    file.by_ref()
        .take(MAX_DOCUMENT_BYTES as u64 + 1)
        .read_to_string(&mut source)
        .map_err(|error| format!("could not read the binding document: {error}"))?;
    let file_after = file.metadata().map_err(|error| error.to_string())?;
    let after = super::private_file_metadata(path)?;
    if std::fs::canonicalize(path).map_err(|error| error.to_string())? != path
        || !same_binding_snapshot(&file_before, &file_after)
        || !same_binding_snapshot(&file_after, &after)
        || source.len() > MAX_DOCUMENT_BYTES
    {
        return Err("the binding document changed or exceeded its bound".to_owned());
    }
    let binding = Binding::parse(&source).map_err(|error| error.detail())?;
    let declaration = serde_json::from_str::<BindingDocument>(&source)
        .map_err(|error| format!("the enrolment shape is invalid: {error}"))?;
    Ok((binding, declaration))
}

#[cfg(unix)]
fn same_binding_snapshot(left: &Metadata, right: &Metadata) -> bool {
    use std::os::unix::fs::MetadataExt as _;
    super::stable_installation_metadata(left, right)
        && left.gid() == right.gid()
        && left.nlink() == right.nlink()
        && left.size() == right.size()
        && left.mtime() == right.mtime()
        && left.mtime_nsec() == right.mtime_nsec()
        && left.ctime() == right.ctime()
        && left.ctime_nsec() == right.ctime_nsec()
}

#[cfg(not(unix))]
fn same_binding_snapshot(left: &Metadata, right: &Metadata) -> bool {
    super::stable_installation_metadata(left, right)
        && left.len() == right.len()
        && left.modified().ok() == right.modified().ok()
}

fn publish_registrations(
    pending: &[(super::Host, Registration)],
) -> Result<(Vec<Value>, bool), String> {
    let paths = pending
        .iter()
        .map(|(_, registration)| registration.path.clone())
        .collect::<Vec<_>>();
    let mut transaction = FileTransaction::capture(&paths)?;
    let mut registrations = Vec::new();
    let mut changed = false;
    let applied = (|| {
        for (host, registration) in pending {
            transaction.begin_publication(&registration.path)?;
            let outcome = apply(registration)?;
            transaction.finish_publication(&registration.path, outcome.changed)?;
            if outcome.changed {
                changed = true;
            }
            let mut record = report(
                &registration.path,
                registration.section,
                &registration.name,
                &outcome,
            );
            if let Some(fields) = record.as_object_mut() {
                fields.insert("host".to_owned(), json!(host.name()));
            }
            registrations.push(record);
        }
        Ok::<(), String>(())
    })();
    if let Err(error) = applied {
        let rollback = transaction.rollback();
        return match rollback {
            Ok(()) => Err(error),
            Err(rollback) => Err(format!("{error}; installation rollback failed: {rollback}")),
        };
    }
    Ok((registrations, changed))
}

/// Resolves the daemon binding document the served path reads, beside the CLI configuration.
fn daemon_binding_path() -> Result<PathBuf, String> {
    let configuration = config::path()?;
    let directory = configuration
        .parent()
        .ok_or_else(|| "the CLI configuration path has no parent directory".to_owned())?;
    Ok(directory.join("mcp").join("binding.json"))
}

/// Anchors an operator-supplied binding path so the installed launch line never depends on the
/// working directory of the runtime that starts the server.
fn absolute_binding_path(explicit: &Path) -> Result<PathBuf, String> {
    if explicit.as_os_str().is_empty() {
        return Err("--daemon-binding requires a path".into());
    }
    if explicit.is_absolute() {
        return Ok(explicit.to_path_buf());
    }
    let current = env::current_dir()
        .map_err(|error| format!("the current directory is not available: {error}"))?;
    Ok(current.join(explicit))
}

fn path_text(path: &Path) -> Result<String, String> {
    path.to_str().map(str::to_owned).ok_or_else(|| {
        format!(
            "the path {} is not valid UTF-8 and cannot be written into a launch line",
            path.display()
        )
    })
}

fn launch_arguments(daemon_binding: &str, read_only: bool) -> Vec<String> {
    let mut arguments = vec![
        "mcp".to_owned(),
        "serve".to_owned(),
        "--daemon-binding".to_owned(),
        daemon_binding.to_owned(),
    ];
    if read_only {
        arguments.push("--read-only".to_owned());
    }
    arguments
}
