use std::io::{self, Read, Write};
use std::net::{IpAddr, SocketAddr, TcpStream, ToSocketAddrs};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::mpsc;
use std::time::{Duration, Instant};

static RESOLVERS: AtomicUsize = AtomicUsize::new(0);
const MAX_RESOLVERS: usize = 32;

#[derive(Clone, Copy)]
pub(crate) struct Deadline(Instant);

impl Deadline {
    pub(crate) fn new(budget: Duration) -> Self {
        Self(Instant::now() + budget)
    }

    pub(crate) fn remaining(self) -> io::Result<Duration> {
        self.0
            .checked_duration_since(Instant::now())
            .filter(|left| !left.is_zero())
            .ok_or_else(|| io::Error::new(io::ErrorKind::TimedOut, "migration deadline exceeded"))
    }

    pub(crate) fn resolve(self, host: &str, port: u16) -> io::Result<Vec<SocketAddr>> {
        self.remaining()?;
        if let Ok(ip) = host.parse::<IpAddr>() {
            return Ok(vec![SocketAddr::new(ip, port)]);
        }
        // libc resolution cannot be cancelled. Bound both the caller's wait and
        // the total number of outstanding resolver workers, including timed-out ones.
        RESOLVERS
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |count| {
                (count < MAX_RESOLVERS).then_some(count + 1)
            })
            .map_err(|_| {
                io::Error::new(io::ErrorKind::WouldBlock, "resolver capacity exhausted")
            })?;
        let host = host.to_owned();
        let (sender, receiver) = mpsc::sync_channel(1);
        let worker = std::thread::Builder::new().spawn(move || {
            let result = (host.as_str(), port)
                .to_socket_addrs()
                .map(|addresses| addresses.take(8).collect());
            RESOLVERS.fetch_sub(1, Ordering::AcqRel);
            let _ = sender.send(result);
        });
        if let Err(error) = worker {
            RESOLVERS.fetch_sub(1, Ordering::AcqRel);
            return Err(error);
        }
        receiver.recv_timeout(self.remaining()?).map_err(|_| {
            io::Error::new(
                io::ErrorKind::TimedOut,
                "migration resolution deadline exceeded",
            )
        })?
    }

    pub(crate) fn connect(
        self,
        host: &str,
        port: u16,
        connect_budget: Duration,
    ) -> io::Result<DeadlineStream> {
        let addresses = self.resolve(host, port)?;
        let mut last = io::Error::new(io::ErrorKind::NotConnected, "no migration RPC address");
        for address in addresses {
            let timeout = self.remaining()?.min(connect_budget);
            match TcpStream::connect_timeout(&address, timeout) {
                Ok(socket) => {
                    self.remaining()?;
                    return Ok(DeadlineStream {
                        socket,
                        deadline: self,
                    });
                }
                Err(error) => last = error,
            }
        }
        Err(last)
    }
}

// rustls may perform many socket operations inside one TLS read/write/handshake.
// Apply the remaining total budget at that lowest blocking boundary each time.
pub(crate) struct DeadlineStream {
    socket: TcpStream,
    deadline: Deadline,
}

impl Read for DeadlineStream {
    fn read(&mut self, bytes: &mut [u8]) -> io::Result<usize> {
        self.socket
            .set_read_timeout(Some(self.deadline.remaining()?))?;
        let count = self.socket.read(bytes)?;
        self.deadline.remaining()?;
        Ok(count)
    }
}

impl Write for DeadlineStream {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        self.socket
            .set_write_timeout(Some(self.deadline.remaining()?))?;
        let count = self.socket.write(bytes)?;
        self.deadline.remaining()?;
        Ok(count)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.socket
            .set_write_timeout(Some(self.deadline.remaining()?))?;
        self.socket.flush()?;
        self.deadline.remaining()?;
        Ok(())
    }
}
