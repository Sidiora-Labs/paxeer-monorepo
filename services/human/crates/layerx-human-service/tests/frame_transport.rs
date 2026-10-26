use std::io::{self, Read, Write};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::time::Duration;

use layerx_human_service::frame::{
    read_frame, write_frame, ConnectionGate, FrameTransport, FrameViolation, Limits,
    TransportError, Uds,
};

struct SocketDirectory(PathBuf);

impl SocketDirectory {
    fn new() -> io::Result<Self> {
        let mut random = [0_u8; 16];
        getrandom::fill(&mut random).map_err(|error| io::Error::other(error.to_string()))?;
        let path =
            std::env::temp_dir().join(format!("human-frame-{:032x}", u128::from_be_bytes(random)));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for SocketDirectory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn limits() -> Limits {
    Limits {
        maximum_frame_bytes: 256,
        maximum_connections: 1,
        maximum_streams: 1,
        maximum_queued_bytes: 256,
        deadline: Duration::from_secs(1),
    }
}

#[test]
fn frame_transport_round_trips_a_frame_over_uds_within_limits() -> io::Result<()> {
    let directory = SocketDirectory::new()?;
    let path = directory.0.join("frame.sock");
    let listener = UnixListener::bind(&path)?;
    let limits = limits();
    let gate = ConnectionGate::new(limits.maximum_connections);
    let mut client = Uds::connect(&path, &gate, limits)
        .map_err(|e| io::Error::other(format!("connect failed: {e:?}")))?;
    let (mut peer, _) = listener.accept()?;
    peer.set_read_timeout(Some(limits.deadline))?;
    peer.set_write_timeout(Some(limits.deadline))?;
    assert_eq!(gate.active(), 1);

    let payload: Vec<u8> = (0_u8..=u8::MAX).collect();
    assert_eq!(payload.len(), limits.maximum_frame_bytes);
    assert_eq!(client.send(&payload), Ok(()));
    assert_eq!(
        read_frame(&mut peer, limits.maximum_frame_bytes),
        Ok(payload.clone())
    );
    assert_eq!(
        write_frame(&mut peer, &payload, limits.maximum_frame_bytes),
        Ok(())
    );
    assert_eq!(client.receive(), Ok(payload));

    assert_eq!(client.send(&[]), Ok(()));
    assert_eq!(
        read_frame(&mut peer, limits.maximum_frame_bytes),
        Ok(Vec::new())
    );
    drop(client);
    assert_eq!(gate.active(), 0);
    Ok(())
}

#[test]
fn frame_transport_refuses_a_frame_above_max_frame_bytes() -> io::Result<()> {
    let directory = SocketDirectory::new()?;
    let path = directory.0.join("frame.sock");
    let listener = UnixListener::bind(&path)?;
    let mut sender = UnixStream::connect(&path)?;
    let (mut receiver, _) = listener.accept()?;
    let limits = limits();
    receiver.set_read_timeout(Some(limits.deadline))?;
    sender.set_write_timeout(Some(limits.deadline))?;
    let declared = u32::try_from(limits.maximum_frame_bytes + 1).map_err(io::Error::other)?;

    sender.write_all(&declared.to_be_bytes())?;
    assert_eq!(
        read_frame(&mut receiver, limits.maximum_frame_bytes),
        Err(TransportError::Frame(FrameViolation::Oversized {
            declared,
            maximum: limits.maximum_frame_bytes,
        }))
    );

    let body = [0x5a; 257];
    sender.write_all(&body)?;
    let mut unread_body = [0_u8; 257];
    receiver.read_exact(&mut unread_body)?;
    assert_eq!(unread_body, body);
    Ok(())
}
