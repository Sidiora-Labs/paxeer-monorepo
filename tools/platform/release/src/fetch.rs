use std::fs;
use std::io::{Read as _, Write as _};
use std::os::fd::OwnedFd;
use std::os::unix::fs::MetadataExt as _;
use std::os::unix::net::UnixStream;
use std::os::unix::process::CommandExt as _;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};

use super::ArtifactManifestEntry;

const GIT_SCHEME: &str = "git+";
const SIMPLE_SCHEME: &str = "simple+";
const TIMEOUT: Duration = Duration::from_secs(300);
const MAX_ARTIFACT_BYTES: u64 = 1 << 30;
const MAX_INDEX_BYTES: u64 = 1 << 24;

/// Fetches the bytes the registry serves for one manifest entry into
/// `destination`.
///
/// Locations are resolved by scheme: `https://` is downloaded as is,
/// `simple+https://` names a PEP 503 project page whose link to the artifact
/// file is followed, and `git+https://<repository>#<tag>` archives the tagged
/// tree the way the Swift package manager consumes it.
///
/// # Errors
///
/// Fails when the location cannot be resolved or downloaded, when the registry
/// answers with anything but HTTP 200, or when the bytes cannot be written.
pub fn fetch(entry: &ArtifactManifestEntry, destination: &Path) -> Result<(), String> {
    fetch_with_git_limits(entry, destination, GitFetchLimits::default())
}

/// Resource budgets for Git retrieval. Callers may tighten the production ceilings.
#[derive(Clone, Copy, Debug)]
pub struct GitFetchLimits {
    pub timeout: Duration,
    pub archive_bytes: u64,
    pub checkout_bytes: u64,
    pub output_bytes: u64,
}

impl Default for GitFetchLimits {
    fn default() -> Self {
        Self {
            timeout: TIMEOUT,
            archive_bytes: MAX_ARTIFACT_BYTES,
            checkout_bytes: MAX_ARTIFACT_BYTES,
            output_bytes: 1 << 16,
        }
    }
}

/// Fetch an artifact with stricter Git budgets, retaining the same scheme dispatch.
///
/// # Errors
/// Returns a refusal on invalid budgets, failed retrieval or exceeded resource limits.
pub fn fetch_with_git_limits(
    entry: &ArtifactManifestEntry,
    destination: &Path,
    limits: GitFetchLimits,
) -> Result<(), String> {
    if let Some(parent) = destination.parent() {
        fs::create_dir_all(parent)
            .map_err(|error| format!("create {}: {error}", parent.display()))?;
    }
    let location = entry.location.as_str();
    if let Some(repository) = location.strip_prefix(GIT_SCHEME) {
        return fetch_git_archive(repository, destination, limits);
    }
    if let Some(index) = location.strip_prefix(SIMPLE_SCHEME) {
        let url = simple_index_link(index, &entry.artifact)?;
        return download(&url, destination);
    }
    if location.starts_with("https://") {
        return download(location, destination);
    }
    Err(format!("location {location} uses no fetchable scheme"))
}

fn agent() -> Result<ureq::Agent, String> {
    let loaded = rustls_native_certs::load_native_certs();
    if loaded.certs.is_empty() || !loaded.errors.is_empty() {
        return Err("system TLS trust roots are unavailable or invalid".into());
    }
    let certificates = loaded
        .certs
        .iter()
        .map(|certificate| ureq::tls::Certificate::from_der(certificate.as_ref()).to_owned())
        .collect::<Vec<_>>();
    Ok(ureq::Agent::config_builder()
        .tls_config(
            ureq::tls::TlsConfig::builder()
                .provider(ureq::tls::TlsProvider::Rustls)
                .root_certs(ureq::tls::RootCerts::new_with_certs(&certificates))
                .build(),
        )
        .timeout_global(Some(TIMEOUT))
        .http_status_as_error(false)
        .build()
        .into())
}

fn get(url: &str, limit: u64, what: &str) -> Result<Vec<u8>, String> {
    let mut response = agent()?
        .get(url)
        .call()
        .map_err(|error| format!("GET {url}: {error}"))?;
    let status = response.status();
    if status != 200 {
        return Err(format!(
            "GET {url} answered HTTP {}, not 200",
            status.as_u16()
        ));
    }
    let mut bytes = Vec::new();
    response
        .body_mut()
        .as_reader()
        .take(limit + 1)
        .read_to_end(&mut bytes)
        .map_err(|error| format!("GET {url}: {error}"))?;
    if bytes.len() as u64 > limit {
        return Err(format!("GET {url}: {what} exceeds {limit} bytes"));
    }
    Ok(bytes)
}

fn download(url: &str, destination: &Path) -> Result<(), String> {
    let bytes = get(url, MAX_ARTIFACT_BYTES, "artifact")?;
    fs::write(destination, bytes)
        .map_err(|error| format!("write {}: {error}", destination.display()))
}

fn simple_index_link(index: &str, artifact: &str) -> Result<String, String> {
    let page = get(index, MAX_INDEX_BYTES, "project page")?;
    let page = String::from_utf8(page)
        .map_err(|error| format!("GET {index}: project page is not UTF-8: {error}"))?;
    let mut links = Vec::new();
    let mut rest = page.as_str();
    while let Some(start) = rest.find("href=\"") {
        let after = &rest[start + "href=\"".len()..];
        let Some(end) = after.find('"') else {
            break;
        };
        links.push(after[..end].replace("&amp;", "&"));
        rest = &after[end..];
    }
    let found = links.iter().find(|href| {
        let path = href.split(['#', '?']).next().unwrap_or_default();
        path == artifact || path.ends_with(&format!("/{artifact}"))
    });
    let Some(href) = found else {
        return Err(format!(
            "project page {index} links no file named {artifact}"
        ));
    };
    resolve(index, href)
}

fn resolve(base: &str, href: &str) -> Result<String, String> {
    if href.starts_with("https://") || href.starts_with("http://") {
        return Ok(href.to_owned());
    }
    let (scheme, remainder) = base
        .split_once("://")
        .ok_or_else(|| format!("index {base} has no scheme"))?;
    let (host, path) = remainder.split_once('/').unwrap_or((remainder, ""));
    if let Some(absolute) = href.strip_prefix('/') {
        return Ok(format!("{scheme}://{host}/{absolute}"));
    }
    let directory = path.rsplit_once('/').map_or("", |(directory, _)| directory);
    let mut segments = directory
        .split('/')
        .filter(|segment| !segment.is_empty())
        .map(str::to_owned)
        .collect::<Vec<_>>();
    for segment in href.split('/') {
        match segment {
            "" | "." => {}
            ".." => {
                segments.pop();
            }
            segment => segments.push(segment.to_owned()),
        }
    }
    Ok(format!("{scheme}://{host}/{}", segments.join("/")))
}

static CHECKOUT_SEQUENCE: AtomicU64 = AtomicU64::new(0);
const MAX_CHECKOUT_ENTRIES: usize = 100_000;

struct GitCheckout(PathBuf);

impl Drop for GitCheckout {
    fn drop(&mut self) {
        // The normal return path reports cleanup errors; this also covers unwinding.
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn fetch_git_archive(
    repository: &str,
    destination: &Path,
    limits: GitFetchLimits,
) -> Result<(), String> {
    let ceiling = GitFetchLimits::default();
    if limits.timeout.is_zero()
        || limits.timeout > ceiling.timeout
        || limits.archive_bytes == 0
        || limits.archive_bytes > ceiling.archive_bytes
        || limits.checkout_bytes == 0
        || limits.checkout_bytes > ceiling.checkout_bytes
        || limits.output_bytes == 0
        || limits.output_bytes > ceiling.output_bytes
    {
        return Err("Git fetch budgets must be positive and within production ceilings".into());
    }
    let deadline = Instant::now() + limits.timeout;
    let (url, tag) = repository
        .split_once('#')
        .filter(|(url, tag)| !url.is_empty() && !tag.is_empty())
        .ok_or_else(|| {
            format!("git location {GIT_SCHEME}{repository} must name the tag after `#`")
        })?;
    let destination = std::path::absolute(destination)
        .map_err(|error| format!("resolve {}: {error}", destination.display()))?;
    let checkout_path = PathBuf::from(format!(
        "{}.checkout.{}.{}",
        destination.display(),
        std::process::id(),
        CHECKOUT_SEQUENCE.fetch_add(1, Ordering::Relaxed)
    ));
    fs::create_dir(&checkout_path)
        .map_err(|error| format!("create {}: {error}", checkout_path.display()))?;
    let checkout = GitCheckout(checkout_path);
    let tree = checkout.0.join("repository");
    let archive = checkout.0.join("archive.tar");
    let mut output_bytes = 0;
    let result = (|| {
        run_git(
            git_command(limits)
                .args([
                    "clone",
                    "--quiet",
                    "--bare",
                    "--no-local",
                    "--depth",
                    "1",
                    "--branch",
                    tag,
                    "--",
                    url,
                ])
                .arg(&tree),
            "Git clone",
            &tree,
            None,
            deadline,
            limits,
            &mut output_bytes,
        )?;
        let mut file =
            fs::File::create(&archive).map_err(|error| format!("create archive: {error}"))?;
        run_git(
            git_command(limits)
                .arg("-C")
                .arg(&tree)
                .args(["archive", "--format=tar", "HEAD"]),
            "Git archive",
            &tree,
            Some(&mut file),
            deadline,
            limits,
            &mut output_bytes,
        )?;
        file.sync_all()
            .map_err(|error| format!("sync archive: {error}"))?;
        check_deadline(deadline)?;
        fs::rename(&archive, &destination)
            .map_err(|error| format!("publish {}: {error}", destination.display()))
    })();
    let cleanup = fs::remove_dir_all(&checkout.0)
        .map_err(|error| format!("clean Git checkout {}: {error}", checkout.0.display()));
    match (result, cleanup) {
        (Ok(()), cleanup) => cleanup,
        (Err(error), Ok(())) => Err(error),
        (Err(error), Err(cleanup)) => Err(format!("{error}; {cleanup}")),
    }
}

fn git_command(limits: GitFetchLimits) -> Command {
    // POSIX sh supplies the file-size resource limit without an unsafe pre_exec hook.
    // Git and its transport/index-pack descendants inherit both this limit and group.
    let mut command = Command::new("/bin/sh");
    command
        .args([
            "-c",
            "ulimit -f \"$1\" || exit; shift; exec git \"$@\"",
            "release-git",
        ])
        .arg(limits.checkout_bytes.div_ceil(512).to_string())
        .args(["-c", "core.hooksPath=/dev/null"])
        .env("GIT_TERMINAL_PROMPT", "0")
        .stdin(Stdio::null())
        .process_group(0);
    command
}

fn check_deadline(deadline: Instant) -> Result<(), String> {
    if Instant::now() >= deadline {
        Err("Git fetch total deadline exceeded".into())
    } else {
        Ok(())
    }
}

fn checkout_size(root: &Path, limit: u64, deadline: Instant) -> Result<(), String> {
    let mut pending = vec![root.to_path_buf()];
    let mut bytes = 0_u64;
    let mut entries = 0;
    while let Some(path) = pending.pop() {
        check_deadline(deadline)?;
        let metadata = match fs::symlink_metadata(&path) {
            Ok(metadata) => metadata,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => continue,
            Err(error) => return Err(format!("measure Git checkout: {error}")),
        };
        entries += 1;
        bytes = bytes.saturating_add(metadata.len().max(metadata.blocks().saturating_mul(512)));
        if bytes > limit || entries > MAX_CHECKOUT_ENTRIES {
            return Err(format!(
                "Git checkout exceeds disk budget ({limit} bytes, {MAX_CHECKOUT_ENTRIES} entries)"
            ));
        }
        if metadata.is_dir() {
            for entry in
                fs::read_dir(&path).map_err(|error| format!("read Git checkout: {error}"))?
            {
                check_deadline(deadline)?;
                let entry = entry.map_err(|error| format!("read Git checkout entry: {error}"))?;
                if pending.len() + entries >= MAX_CHECKOUT_ENTRIES {
                    return Err("Git checkout exceeds directory entry budget".into());
                }
                pending.push(entry.path());
            }
        }
    }
    Ok(())
}

fn output_socket() -> Result<(UnixStream, Stdio), String> {
    let (reader, writer) =
        UnixStream::pair().map_err(|error| format!("Git output socket: {error}"))?;
    reader
        .set_nonblocking(true)
        .map_err(|error| format!("Git output nonblocking: {error}"))?;
    Ok((reader, Stdio::from(OwnedFd::from(writer))))
}

fn run_git(
    command: &mut Command,
    what: &str,
    checkout: &Path,
    mut archive: Option<&mut fs::File>,
    deadline: Instant,
    limits: GitFetchLimits,
    output_bytes: &mut u64,
) -> Result<(), String> {
    check_deadline(deadline)?;
    let (mut stdout, child_stdout) = output_socket()?;
    let (mut stderr, child_stderr) = output_socket()?;
    let mut child = command
        .stdout(child_stdout)
        .stderr(child_stderr)
        .spawn()
        .map_err(|error| format!("{what}: {error}"))?;
    // Command retains its copies of Stdio until replaced; close them so EOF is observable.
    command.stdout(Stdio::null()).stderr(Stdio::null());
    let result = (|| {
        let mut stderr_bytes = Vec::new();
        let mut archive_bytes = 0_u64;
        let mut ended = [false; 2];
        let mut status = None;
        let mut next_disk_check = Instant::now();
        loop {
            check_deadline(deadline)?;
            if Instant::now() >= next_disk_check {
                checkout_size(checkout, limits.checkout_bytes, deadline)?;
                next_disk_check = Instant::now() + Duration::from_millis(10);
            }
            let mut progressed = false;
            for (index, reader) in [&mut stdout, &mut stderr].into_iter().enumerate() {
                if ended[index] {
                    continue;
                }
                let mut buffer = [0; 8192];
                match reader.read(&mut buffer) {
                    Ok(0) => ended[index] = true,
                    Ok(count) => {
                        progressed = true;
                        if index == 0 && archive.is_some() {
                            archive_bytes += count as u64;
                            if archive_bytes > limits.archive_bytes {
                                return Err(format!(
                                    "Git archive exceeds {} bytes",
                                    limits.archive_bytes
                                ));
                            }
                            if let Some(file) = archive.as_deref_mut() {
                                file.write_all(&buffer[..count])
                                    .map_err(|error| format!("write Git archive: {error}"))?;
                            }
                        } else {
                            *output_bytes += count as u64;
                            if *output_bytes > limits.output_bytes {
                                return Err(format!(
                                    "Git subprocess output exceeds {} bytes",
                                    limits.output_bytes
                                ));
                            }
                            if index == 1 {
                                stderr_bytes.extend_from_slice(&buffer[..count]);
                            }
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                    Err(error) => return Err(format!("read {what} output: {error}")),
                }
            }
            if status.is_none() {
                status = child
                    .try_wait()
                    .map_err(|error| format!("wait for {what}: {error}"))?;
            }
            if let Some(status) = status.filter(|_| ended.iter().all(|ended| *ended)) {
                checkout_size(checkout, limits.checkout_bytes, deadline)?;
                return if status.success() {
                    Ok(())
                } else {
                    Err(format!(
                        "{what}: git exited with {status}: {}",
                        String::from_utf8_lossy(&stderr_bytes).trim()
                    ))
                };
            }
            if !progressed {
                std::thread::sleep(Duration::from_millis(2));
            }
        }
    })();
    if let Err(error) = &result {
        terminate_git_group(&mut child, error, what)?;
    }
    result
}

fn terminate_git_group(
    child: &mut std::process::Child,
    error: &str,
    what: &str,
) -> Result<(), String> {
    // Kill the whole group: remote-http and index-pack may still own output sockets.
    let killed = Command::new("/bin/sh")
        .args(["-c", "kill -KILL \"-$1\"", "release-git-kill"])
        .arg(child.id().to_string())
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
    let direct_kill = child.kill();
    let reaped = child.wait();
    let exited = matches!(child.try_wait(), Ok(Some(_)));
    if !matches!(killed, Ok(status) if status.success()) && direct_kill.is_err() && !exited {
        return Err(format!("{error}; failed to terminate Git process group"));
    }
    reaped.map_err(|error| format!("reap {what}: {error}"))?;
    Ok(())
}

#[cfg(test)]
#[path = "../../../../services/platform/tests/support/tls_boundary.rs"]
mod tls_boundary;

#[test]
fn artifact_fetch_uses_system_trust_and_checks_the_actual_server_identity() {
    let module = module_path!()
        .split_once("::")
        .map_or("", |(_, module)| module);
    let test_name =
        format!("{module}::artifact_fetch_uses_system_trust_and_checks_the_actual_server_identity");
    tls_boundary::qualify(&test_name, |endpoint| {
        get(&format!("{endpoint}/livez"), 1024, "boundary liveness")
    });
}
