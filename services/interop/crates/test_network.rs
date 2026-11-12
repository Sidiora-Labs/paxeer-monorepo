use std::io;
use std::process::Command;

/// Keep each worker's real loopback clients and listeners in one private network.
pub(crate) fn isolate() -> io::Result<()> {
    let worker = std::fs::read_link("/proc/thread-self/ns/net")?;
    let process = std::fs::read_link("/proc/self/ns/net")?;
    if worker != process {
        return Ok(());
    }
    nix::sched::unshare(nix::sched::CloneFlags::CLONE_NEWNET).map_err(io::Error::from)?;
    let status = Command::new("ip")
        .args(["link", "set", "lo", "up"])
        .status()?;
    if !status.success() {
        return Err(io::Error::other(
            "private test loopback could not be enabled",
        ));
    }
    Ok(())
}
