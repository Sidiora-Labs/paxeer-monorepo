//! Strict framing shared by the worker listener and pinned client.

/// The transport supports exactly one decimal Content-Length and no transfer coding.
#[must_use]
pub fn content_length<'a>(lines: impl Iterator<Item = &'a str>) -> Option<usize> {
    let mut length = None;
    for line in lines {
        if line.is_empty() {
            continue;
        }
        let (name, value) = line.split_once(':')?;
        if name.is_empty()
            || !name
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b"!#$%&'*+-.^_`|~".contains(&b))
            || !value.bytes().all(|b| b == b'\t' || (32..=126).contains(&b))
            || name.eq_ignore_ascii_case("transfer-encoding")
        {
            return None;
        }
        if name.eq_ignore_ascii_case("content-length") {
            let value = value.trim_matches([' ', '\t']);
            if length.is_some() || value.is_empty() || !value.bytes().all(|b| b.is_ascii_digit()) {
                return None;
            }
            length = Some(value.parse().ok()?);
        }
    }
    length
}
