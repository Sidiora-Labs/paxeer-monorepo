use super::ResponseContent;
use serde::de::IgnoredAny;
use std::{collections::BTreeMap, error::Error, fmt};

/// A validated query cursor. Numeric values keep their original JSON spelling,
/// including token IDs wider than u64. Null cursor fields use Plug's empty value.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PageCursor {
    params: BTreeMap<String, String>,
}

#[derive(Debug)]
pub struct CursorError(String);

impl fmt::Display for CursorError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl Error for CursorError {}

impl From<serde_json::Error> for CursorError {
    fn from(error: serde_json::Error) -> Self {
        Self(error.to_string())
    }
}

// Read JSON members as slices, using serde's syntax validator to skip each value.
// Deserializing a cursor through Value first would round large numeric token IDs.
fn members(json: &str) -> Result<Vec<(String, &str)>, CursorError> {
    serde_json::from_str::<IgnoredAny>(json)?;
    let mut rest = json.trim();
    if !rest.starts_with('{') {
        return Err(CursorError("expected a JSON object".into()));
    }
    rest = rest[1..].trim_start();
    let mut result = Vec::new();
    while !rest.starts_with('}') {
        let mut key = serde_json::Deserializer::from_str(rest).into_iter::<String>();
        let name = key
            .next()
            .ok_or_else(|| CursorError("missing key".into()))??;
        rest = rest[key.byte_offset()..].trim_start();
        rest = rest
            .strip_prefix(':')
            .ok_or_else(|| CursorError("missing colon".into()))?
            .trim_start();
        let mut value = serde_json::Deserializer::from_str(rest).into_iter::<IgnoredAny>();
        value
            .next()
            .ok_or_else(|| CursorError("missing value".into()))??;
        let end = value.byte_offset();
        result.push((name, &rest[..end]));
        rest = rest[end..].trim_start();
        if let Some(next) = rest.strip_prefix(',') {
            rest = next.trim_start();
        } else {
            break;
        }
    }
    Ok(result)
}

impl PageCursor {
    /// Decode next_page_params directly from JSON, before numeric precision is lost.
    /// A terminal null is None; an empty or non-scalar cursor is an error.
    pub fn from_json(json: &str) -> Result<Option<Self>, CursorError> {
        if json.trim() == "null" {
            return Ok(None);
        }
        let mut params = BTreeMap::new();
        for (key, raw) in members(json)? {
            if key.is_empty()
                || !key
                    .bytes()
                    .all(|b| b.is_ascii_alphanumeric() || b"_-[]".contains(&b))
            {
                return Err(CursorError(format!("invalid cursor key: {key}")));
            }
            let value = match raw.as_bytes().first() {
                Some(b'"') => serde_json::from_str::<String>(raw)?,
                Some(b'{' | b'[') => {
                    return Err(CursorError(format!("cursor field {key} must be scalar")))
                }
                _ if raw == "null" => String::new(),
                _ => raw.to_owned(),
            };
            if params.insert(key.clone(), value).is_some() {
                return Err(CursorError(format!("duplicate cursor key: {key}")));
            }
        }
        if params.is_empty() {
            return Err(CursorError(
                "empty cursor would repeat the first page".into(),
            ));
        }
        Ok(Some(Self { params }))
    }

    pub fn query_pairs(&self) -> impl Iterator<Item = (&str, &str)> {
        self.params
            .iter()
            .map(|(key, value)| (key.as_str(), value.as_str()))
    }

    /// Merge into a real HTTP request, rejecting duplicate or conflicting filters.
    pub fn apply(&self, request: &mut reqwest::Request) -> Result<(), std::io::Error> {
        let mut query: BTreeMap<String, String> = BTreeMap::new();
        for (key, value) in request.url().query_pairs() {
            if query.insert(key.into_owned(), value.into_owned()).is_some() {
                return Err(std::io::Error::new(
                    std::io::ErrorKind::InvalidInput,
                    "duplicate original query parameter",
                ));
            }
        }
        for (key, value) in &self.params {
            if let Some(original) = query.get(key) {
                if original != value {
                    return Err(std::io::Error::new(
                        std::io::ErrorKind::InvalidInput,
                        format!("cursor conflicts with original filter {key}"),
                    ));
                }
            } else {
                query.insert(key.clone(), value.clone());
            }
        }
        request
            .url_mut()
            .query_pairs_mut()
            .clear()
            .extend_pairs(query);
        Ok(())
    }
}

impl<T> ResponseContent<T> {
    /// Extract the complete producer cursor from the wire response, without f64 conversion.
    pub fn next_page_cursor(&self) -> Result<Option<PageCursor>, CursorError> {
        let mut cursor = None;
        for (key, raw) in members(&self.content)? {
            if key == "next_page_params" {
                if cursor.is_some() {
                    return Err(CursorError("duplicate next_page_params".into()));
                }
                cursor = Some(PageCursor::from_json(raw)?);
            }
        }
        cursor.ok_or_else(|| CursorError("response has no next_page_params".into()))
    }
}
