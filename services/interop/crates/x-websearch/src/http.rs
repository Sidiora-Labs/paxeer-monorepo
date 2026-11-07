use std::io::{self, Read};
use std::net::TcpStream;
use std::time::{Duration, Instant};

use crate::fetch::FetchError;

pub(crate) const MAX_RESPONSE_HEAD: usize = 65_536;
const MAX_CHUNK_LINE: usize = 1_024;
const MAX_CHUNK_METADATA: usize = 262_144;
const READ_CHUNK: usize = 16_384;

pub(crate) trait TimedRead: Read {
    fn set_read_timeout(&self, timeout: Duration) -> io::Result<()>;
}

impl TimedRead for native_tls::TlsStream<TcpStream> {
    fn set_read_timeout(&self, timeout: Duration) -> io::Result<()> {
        self.get_ref().set_read_timeout(Some(timeout))
    }
}

pub(crate) struct Wire<S> {
    stream: S,
    buffer: Vec<u8>,
    start: usize,
    deadline: Instant,
    head_bytes: usize,
    buffer_peak: usize,
}

impl<S: TimedRead> Wire<S> {
    pub(crate) fn new(stream: S, deadline: Instant) -> Self {
        Self {
            stream,
            buffer: Vec::with_capacity(READ_CHUNK),
            start: 0,
            deadline,
            head_bytes: 0,
            buffer_peak: 0,
        }
    }

    pub(crate) const fn buffer_peak(&self) -> usize {
        self.buffer_peak
    }

    fn available(&self) -> &[u8] {
        &self.buffer[self.start..]
    }

    fn fill(&mut self) -> Result<usize, FetchError> {
        if self.start > 0 {
            self.buffer.drain(..self.start);
            self.start = 0;
        }
        let mut chunk = [0_u8; READ_CHUNK];
        loop {
            let left = self.deadline.saturating_duration_since(Instant::now());
            if left.is_zero() {
                return Err(FetchError::Timeout);
            }
            self.stream
                .set_read_timeout(left)
                .map_err(|_| FetchError::Transport)?;
            match self.stream.read(&mut chunk) {
                Ok(count) => {
                    self.buffer.extend_from_slice(&chunk[..count]);
                    self.buffer_peak = self.buffer_peak.max(self.buffer.len());
                    return Ok(count);
                }
                Err(error) if error.kind() == io::ErrorKind::Interrupted => {}
                Err(error)
                    if matches!(
                        error.kind(),
                        io::ErrorKind::WouldBlock | io::ErrorKind::TimedOut
                    ) =>
                {
                    return Err(FetchError::Timeout);
                }
                Err(_) => return Err(FetchError::Transport),
            }
        }
    }

    fn until(&mut self, marker: &[u8], maximum: usize) -> Result<String, FetchError> {
        loop {
            if let Some(end) = self
                .available()
                .windows(marker.len())
                .position(|part| part == marker)
            {
                if end > maximum {
                    return Err(FetchError::MalformedResponse);
                }
                let text = std::str::from_utf8(&self.available()[..end])
                    .map_err(|_| FetchError::MalformedResponse)?
                    .to_owned();
                self.start += end + marker.len();
                return Ok(text);
            }
            if self.available().len() > maximum + marker.len() - 1 || self.fill()? == 0 {
                return Err(FetchError::MalformedResponse);
            }
        }
    }

    fn charge(used: &mut usize, count: usize, limit: usize) -> Result<(), FetchError> {
        *used = used
            .checked_add(count)
            .ok_or(FetchError::MalformedResponse)?;
        if *used > limit {
            return Err(FetchError::MalformedResponse);
        }
        Ok(())
    }

    pub(crate) fn head(&mut self) -> Result<String, FetchError> {
        let head = self.until(b"\r\n\r\n", MAX_RESPONSE_HEAD)?;
        Self::charge(&mut self.head_bytes, head.len() + 4, MAX_RESPONSE_HEAD)?;
        Ok(head)
    }

    pub(crate) fn take(&mut self, count: usize) -> Result<Vec<u8>, FetchError> {
        while self.available().len() < count {
            if self.fill()? == 0 {
                return Err(FetchError::MalformedResponse);
            }
        }
        let bytes = self.available()[..count].to_vec();
        self.start += count;
        Ok(bytes)
    }

    pub(crate) fn until_close(&mut self, limit: usize) -> Result<Vec<u8>, FetchError> {
        loop {
            if self.available().len() > limit {
                return Err(FetchError::BodyTooLarge);
            }
            if self.fill()? == 0 {
                return Ok(self.available().to_vec());
            }
        }
    }

    pub(crate) fn chunked(&mut self, limit: usize) -> Result<Vec<u8>, FetchError> {
        let mut body = Vec::new();
        let mut metadata = 0;
        loop {
            let line = self.until(b"\r\n", MAX_CHUNK_LINE)?;
            Self::charge(&mut metadata, line.len() + 2, MAX_CHUNK_METADATA)?;
            let size = line.split(';').next().unwrap_or_default().trim();
            if size.is_empty() || size.len() > 16 {
                return Err(FetchError::MalformedResponse);
            }
            let size =
                usize::from_str_radix(size, 16).map_err(|_| FetchError::MalformedResponse)?;
            if size == 0 {
                let mut trailers = 0;
                loop {
                    let line = self.until(b"\r\n", MAX_RESPONSE_HEAD)?;
                    Self::charge(&mut trailers, line.len() + 2, MAX_RESPONSE_HEAD)?;
                    if line.is_empty() {
                        return Ok(body);
                    }
                }
            }
            if body.len().saturating_add(size) > limit {
                return Err(FetchError::BodyTooLarge);
            }
            Self::charge(&mut metadata, 2, MAX_CHUNK_METADATA)?;
            body.extend_from_slice(&self.take(size)?);
            if self.take(2)? != b"\r\n" {
                return Err(FetchError::MalformedResponse);
            }
        }
    }
}
