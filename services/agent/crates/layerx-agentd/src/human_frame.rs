use std::io::{ErrorKind, Read, Write};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum FrameViolation {
    TruncatedPrefix,
    TruncatedBody,
    Oversized { declared: u32, maximum: usize },
    LengthOverflow,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TransportError {
    ConnectionFailure(ErrorKind),
    Deadline,
    Frame(FrameViolation),
    PeerShutdown,
}

pub trait HumanFrameTransport {
    fn send(&mut self, bytes: &[u8]) -> Result<(), TransportError>;
    fn receive(&mut self) -> Result<Vec<u8>, TransportError>;
}

pub const LENGTH_PREFIX_BYTES: usize = 4;

pub fn read_frame<R: Read>(reader: &mut R, maximum: usize) -> Result<Vec<u8>, TransportError> {
    let mut prefix = [0_u8; LENGTH_PREFIX_BYTES];
    read_exact(reader, &mut prefix, true)?;
    let declared = u32::from_be_bytes(prefix);
    let length = usize::try_from(declared)
        .map_err(|_| TransportError::Frame(FrameViolation::LengthOverflow))?;
    if length > maximum {
        return Err(TransportError::Frame(FrameViolation::Oversized {
            declared,
            maximum,
        }));
    }
    let mut payload = vec![0_u8; length];
    read_exact(reader, &mut payload, false)?;
    Ok(payload)
}

/// Writes one bounded frame without changing its canonical payload.
///
/// # Errors
///
/// Returns a typed frame error before writing an excessive payload and
/// preserves deadline, shutdown, and connection failures from the writer.
pub fn write_frame<W: Write>(
    writer: &mut W,
    payload: &[u8],
    maximum: usize,
) -> Result<(), TransportError> {
    if payload.len() > maximum {
        let declared = u32::try_from(payload.len()).unwrap_or(u32::MAX);
        return Err(TransportError::Frame(FrameViolation::Oversized {
            declared,
            maximum,
        }));
    }
    let length = u32::try_from(payload.len())
        .map_err(|_| TransportError::Frame(FrameViolation::LengthOverflow))?;
    write_all(writer, &length.to_be_bytes())?;
    write_all(writer, payload)?;
    writer.flush().map_err(|error| map_io(&error))
}

fn read_exact<R: Read>(
    reader: &mut R,
    output: &mut [u8],
    prefix: bool,
) -> Result<(), TransportError> {
    let mut offset = 0;
    while offset < output.len() {
        match reader.read(&mut output[offset..]) {
            Ok(0) if offset == 0 => return Err(TransportError::PeerShutdown),
            Ok(0) => {
                let violation = if prefix {
                    FrameViolation::TruncatedPrefix
                } else {
                    FrameViolation::TruncatedBody
                };
                return Err(TransportError::Frame(violation));
            }
            Ok(read) => offset += read,
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(map_io(&error)),
        }
    }
    Ok(())
}

fn write_all<W: Write>(writer: &mut W, bytes: &[u8]) -> Result<(), TransportError> {
    let mut offset = 0;
    while offset < bytes.len() {
        match writer.write(&bytes[offset..]) {
            Ok(0) => return Err(TransportError::PeerShutdown),
            Ok(written) => offset += written,
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(map_io(&error)),
        }
    }
    Ok(())
}

fn map_io(error: &std::io::Error) -> TransportError {
    match error.kind() {
        ErrorKind::TimedOut | ErrorKind::WouldBlock => TransportError::Deadline,
        ErrorKind::BrokenPipe
        | ErrorKind::ConnectionAborted
        | ErrorKind::ConnectionReset
        | ErrorKind::NotConnected
        | ErrorKind::UnexpectedEof => TransportError::PeerShutdown,
        kind => TransportError::ConnectionFailure(kind),
    }
}
