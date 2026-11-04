use reqwest::header::HeaderValue;
use serde::{Deserialize, Deserializer};
use std::{fmt, time::Duration};

pub(super) fn deserialize_api_key<'de, D>(deserializer: D) -> Result<Option<HeaderValue>, D::Error>
where
    D: Deserializer<'de>,
{
    let string = Option::<String>::deserialize(deserializer)?;
    string
        .map(|value| {
            HeaderValue::from_str(&value).map(|mut header| {
                header.set_sensitive(true);
                header
            })
        })
        .transpose()
        .map_err(<D::Error as serde::de::Error>::custom)
}

pub(super) struct ConfigDebug<'a> {
    pub url: &'a url::Url,
    pub api_key: &'a Option<HeaderValue>,
    pub http_timeout: &'a Duration,
    pub probe_url: &'a bool,
}

impl fmt::Debug for ConfigDebug<'_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ConfigDebug")
            .field("url", self.url)
            .field("api_key", &self.api_key.as_ref().map(|_| "[REDACTED]"))
            .field("http_timeout", self.http_timeout)
            .field("probe_url", self.probe_url)
            .finish()
    }
}
