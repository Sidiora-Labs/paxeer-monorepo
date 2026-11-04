use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr, TcpStream, UdpSocket};
use std::time::{Duration, Instant};

use crate::fetch::FetchError;

/// DNS resolution uses deadline-bound sockets, including TCP fallback. No
/// detached system resolver can outlive a caller or consume another worker.
pub struct Resolver {
    servers: Vec<SocketAddr>,
    hosts: BTreeMap<String, Vec<IpAddr>>,
    search: Vec<String>,
    ndots: usize,
}

impl Resolver {
    pub fn system() -> Result<Self, FetchError> {
        let configuration =
            std::fs::read_to_string("/etc/resolv.conf").map_err(|_| FetchError::Resolve)?;
        let mut resolver = Self::from_nameservers(&[])?;
        for line in configuration.lines() {
            let words: Vec<_> = line
                .split(['#', ';'])
                .next()
                .unwrap_or_default()
                .split_whitespace()
                .collect();
            match words.as_slice() {
                ["nameserver", address, ..] => {
                    let ip = address.parse().map_err(|_| FetchError::Resolve)?;
                    resolver.servers.push(SocketAddr::new(ip, 53));
                }
                ["search", domains @ ..] => {
                    resolver.search = domains
                        .iter()
                        .take(6)
                        .map(|domain| (*domain).to_owned())
                        .collect()
                }
                ["domain", domain] if resolver.search.is_empty() => {
                    resolver.search.push((*domain).to_owned())
                }
                ["options", options @ ..] => {
                    for option in options {
                        if let Some(value) = option.strip_prefix("ndots:") {
                            resolver.ndots = value
                                .parse::<usize>()
                                .map_err(|_| FetchError::Resolve)?
                                .min(15);
                        }
                    }
                }
                _ => {}
            }
        }
        if let Ok(hosts) = std::fs::read_to_string("/etc/hosts") {
            for line in hosts.lines() {
                let mut words = line
                    .split('#')
                    .next()
                    .unwrap_or_default()
                    .split_whitespace();
                if let Some(ip) = words.next().and_then(|word| word.parse::<IpAddr>().ok()) {
                    for host in words {
                        resolver
                            .hosts
                            .entry(host.to_ascii_lowercase())
                            .or_default()
                            .push(ip);
                    }
                }
            }
        }
        Ok(resolver)
    }

    /// Explicit DNS peers are useful for private DNS deployments as well as
    /// loopback transcript servers. Literal destinations still bypass DNS.
    pub fn from_nameservers(servers: &[SocketAddr]) -> Result<Self, FetchError> {
        if servers.len() > 8 || servers.iter().any(|server| server.port() == 0) {
            return Err(FetchError::Resolve);
        }
        Ok(Self {
            servers: servers.to_vec(),
            hosts: BTreeMap::new(),
            search: Vec::new(),
            ndots: 1,
        })
    }

    pub fn resolve(
        &self,
        host: &str,
        port: u16,
        deadline: Instant,
    ) -> Result<Vec<SocketAddr>, FetchError> {
        left(deadline)?;
        if let Ok(ip) = host.parse::<IpAddr>() {
            return Ok(vec![SocketAddr::new(ip, port)]);
        }
        if let Some(ips) = self.hosts.get(&host.to_ascii_lowercase()) {
            return Ok(ips.iter().map(|ip| SocketAddr::new(*ip, port)).collect());
        }
        let absolute = host.ends_with('.');
        let host = host.trim_end_matches('.').to_ascii_lowercase();
        let first_absolute =
            absolute || host.bytes().filter(|byte| *byte == b'.').count() >= self.ndots;
        let mut names = Vec::new();
        if first_absolute {
            names.push(host.clone());
        }
        if !absolute {
            names.extend(self.search.iter().map(|suffix| format!("{host}.{suffix}")));
        }
        if !first_absolute {
            names.push(host);
        }
        for name in names {
            let mut addresses = Vec::new();
            // Both families must resolve before any address is admitted: a
            // public A record never hides a forbidden AAAA record.
            for kind in [1_u16, 28] {
                let mut current = name.clone();
                for hop in 0..8 {
                    let answer = self.query(&current, kind, deadline)?;
                    addresses.extend(answer.addresses);
                    match answer.canonical {
                        Some(canonical) if hop < 7 => current = canonical,
                        Some(_) => return Err(FetchError::Resolve),
                        None => break,
                    }
                }
            }
            if !addresses.is_empty() {
                addresses.sort_unstable();
                addresses.dedup();
                return Ok(addresses
                    .into_iter()
                    .map(|ip| SocketAddr::new(ip, port))
                    .collect());
            }
        }
        Err(FetchError::Resolve)
    }

    fn query(&self, name: &str, kind: u16, deadline: Instant) -> Result<Answer, FetchError> {
        let mut id = [0; 2];
        openssl::rand::rand_bytes(&mut id).map_err(|_| FetchError::Resolve)?;
        let mut query = vec![id[0], id[1], 1, 0, 0, 1, 0, 0, 0, 0, 0, 0];
        if name.len() > 253 {
            return Err(FetchError::Resolve);
        }
        for label in name.split('.') {
            if label.is_empty() || label.len() > 63 {
                return Err(FetchError::Resolve);
            }
            query.push(u8::try_from(label.len()).map_err(|_| FetchError::Resolve)?);
            query.extend_from_slice(label.as_bytes());
        }
        query.push(0);
        query.extend_from_slice(&kind.to_be_bytes());
        query.extend_from_slice(&1_u16.to_be_bytes());
        for (index, server) in self.servers.iter().enumerate() {
            let attempt = if index + 1 == self.servers.len() {
                deadline
            } else {
                deadline.min(Instant::now() + Duration::from_secs(1))
            };
            match exchange(server, &query, attempt)
                .and_then(|packet| decode(&packet, &query, name, kind))
            {
                Ok(answer) => return Ok(answer),
                Err(_) if Instant::now() >= deadline => return Err(FetchError::Timeout),
                Err(_) => {}
            }
        }
        Err(FetchError::Resolve)
    }
}

fn left(deadline: Instant) -> Result<Duration, FetchError> {
    let duration = deadline.saturating_duration_since(Instant::now());
    if duration.is_zero() {
        Err(FetchError::Timeout)
    } else {
        Ok(duration)
    }
}

fn exchange(server: &SocketAddr, query: &[u8], deadline: Instant) -> Result<Vec<u8>, FetchError> {
    let bind = if server.is_ipv4() {
        "0.0.0.0:0"
    } else {
        "[::]:0"
    };
    let socket = UdpSocket::bind(bind).map_err(|_| FetchError::Resolve)?;
    socket.connect(server).map_err(|_| FetchError::Resolve)?;
    socket
        .set_write_timeout(Some(left(deadline)?))
        .map_err(|_| FetchError::Resolve)?;
    socket.send(query).map_err(|_| FetchError::Resolve)?;
    let mut packet = vec![0; 65_535];
    loop {
        socket
            .set_read_timeout(Some(left(deadline)?))
            .map_err(|_| FetchError::Resolve)?;
        let count = socket.recv(&mut packet).map_err(|_| FetchError::Resolve)?;
        if count >= 12 && packet[..2] == query[..2] {
            packet.truncate(count);
            break;
        }
    }
    if packet[2] & 2 == 0 {
        return Ok(packet);
    }
    let mut stream =
        TcpStream::connect_timeout(server, left(deadline)?).map_err(|_| FetchError::Resolve)?;
    let length = u16::try_from(query.len())
        .map_err(|_| FetchError::Resolve)?
        .to_be_bytes();
    let mut outbound = length.to_vec();
    outbound.extend_from_slice(query);
    while !outbound.is_empty() {
        stream
            .set_write_timeout(Some(left(deadline)?))
            .map_err(|_| FetchError::Resolve)?;
        let count = stream.write(&outbound).map_err(|_| FetchError::Resolve)?;
        if count == 0 {
            return Err(FetchError::Resolve);
        }
        outbound.drain(..count);
    }
    let mut size = [0; 2];
    read_exact(&mut stream, &mut size, deadline)?;
    let mut packet = vec![0; usize::from(u16::from_be_bytes(size))];
    read_exact(&mut stream, &mut packet, deadline)?;
    Ok(packet)
}

fn read_exact(
    stream: &mut TcpStream,
    mut output: &mut [u8],
    deadline: Instant,
) -> Result<(), FetchError> {
    while !output.is_empty() {
        stream
            .set_read_timeout(Some(left(deadline)?))
            .map_err(|_| FetchError::Resolve)?;
        let count = stream.read(output).map_err(|_| FetchError::Resolve)?;
        if count == 0 {
            return Err(FetchError::Resolve);
        }
        output = &mut output[count..];
    }
    Ok(())
}

fn word(packet: &[u8], offset: usize) -> Result<u16, FetchError> {
    let bytes = packet.get(offset..offset + 2).ok_or(FetchError::Resolve)?;
    Ok(u16::from_be_bytes([bytes[0], bytes[1]]))
}

fn read_name(packet: &[u8], offset: &mut usize) -> Result<String, FetchError> {
    let mut cursor = *offset;
    let mut jumped = false;
    let mut name = String::new();
    for _ in 0..128 {
        let size = *packet.get(cursor).ok_or(FetchError::Resolve)?;
        if size & 0xc0 == 0xc0 {
            let target = usize::from(word(packet, cursor)? & 0x3fff);
            if target >= cursor {
                return Err(FetchError::Resolve);
            }
            if !jumped {
                *offset = cursor + 2;
                jumped = true;
            }
            cursor = target;
        } else if size == 0 {
            if !jumped {
                *offset = cursor + 1;
            }
            return Ok(name);
        } else {
            if size > 63 {
                return Err(FetchError::Resolve);
            }
            cursor += 1;
            let label = packet
                .get(cursor..cursor + usize::from(size))
                .ok_or(FetchError::Resolve)?;
            let label = std::str::from_utf8(label).map_err(|_| FetchError::Resolve)?;
            if !label
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'))
            {
                return Err(FetchError::Resolve);
            }
            if !name.is_empty() {
                name.push('.');
            }
            name.push_str(&label.to_ascii_lowercase());
            if name.len() > 253 {
                return Err(FetchError::Resolve);
            }
            cursor += usize::from(size);
        }
    }
    Err(FetchError::Resolve)
}

struct Answer {
    addresses: Vec<IpAddr>,
    canonical: Option<String>,
}

fn decode(packet: &[u8], query: &[u8], name: &str, kind: u16) -> Result<Answer, FetchError> {
    if packet.len() < 12
        || packet[..2] != query[..2]
        || packet[2] & 0xfa != 0x80
        || word(packet, 4)? != 1
    {
        return Err(FetchError::Resolve);
    }
    let rcode = packet[3] & 15;
    if !matches!(rcode, 0 | 3) {
        return Err(FetchError::Resolve);
    }
    let mut offset = 12;
    if read_name(packet, &mut offset)? != name
        || word(packet, offset)? != kind
        || word(packet, offset + 2)? != 1
    {
        return Err(FetchError::Resolve);
    }
    offset += 4;
    let count = word(packet, 6)?;
    if count > 128 {
        return Err(FetchError::Resolve);
    }
    let mut addresses = Vec::new();
    let mut aliases = Vec::new();
    for _ in 0..count {
        let owner = read_name(packet, &mut offset)?;
        let record_kind = word(packet, offset)?;
        let class = word(packet, offset + 2)?;
        let length = usize::from(word(packet, offset + 8)?);
        offset += 10;
        let end = offset.checked_add(length).ok_or(FetchError::Resolve)?;
        let data = packet.get(offset..end).ok_or(FetchError::Resolve)?;
        if class == 1 {
            match (record_kind, data) {
                (1, [a, b, c, d]) if kind == 1 => {
                    addresses.push((owner, IpAddr::V4(Ipv4Addr::new(*a, *b, *c, *d))))
                }
                (28, data) if kind == 28 && data.len() == 16 => {
                    addresses.push((
                        owner,
                        IpAddr::V6(Ipv6Addr::from(
                            <[u8; 16]>::try_from(data).map_err(|_| FetchError::Resolve)?,
                        )),
                    ));
                }
                (5, _) => {
                    let mut cursor = offset;
                    let canonical = read_name(packet, &mut cursor)?;
                    if cursor != end {
                        return Err(FetchError::Resolve);
                    }
                    aliases.push((owner, canonical));
                }
                _ => {}
            }
        }
        offset = end;
    }
    if rcode == 3 {
        if count != 0 {
            return Err(FetchError::Resolve);
        }
        return Ok(Answer {
            addresses: Vec::new(),
            canonical: None,
        });
    }
    let mut canonical = name.to_owned();
    for _ in 0..8 {
        if let Some((_, target)) = aliases.iter().find(|(owner, _)| *owner == canonical) {
            canonical = target.clone();
        } else {
            let addresses: Vec<_> = addresses
                .into_iter()
                .filter(|(owner, _)| *owner == canonical)
                .map(|(_, ip)| ip)
                .collect();
            let next = (addresses.is_empty() && canonical != name).then_some(canonical);
            return Ok(Answer {
                addresses,
                canonical: next,
            });
        }
    }
    Err(FetchError::Resolve)
}
