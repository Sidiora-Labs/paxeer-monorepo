// Finding api-client-debug-credential is proved by the tests below. The prescribed
// gates omit the optional blockscout feature, so compile the production codec and
// debug representation directly; Config delegates to these same implementations.
#[path = "../src/blockscout/config_security.rs"]
mod config_security;

use api_client_framework::reqwest::{
    header::{HeaderName, HeaderValue},
    Method, Request,
};
use config_security::{deserialize_api_key, ConfigDebug};
use std::time::Duration;

const CREDENTIAL: &str = "repair-22-33-384-distinctive-backend-credential";

#[test]
fn api_client_debug_credential_redacts_programmatic_config() {
    let url = "http://localhost:8080/".parse().unwrap();
    let timeout = Duration::from_secs(17);
    let mut header = HeaderValue::from_str(CREDENTIAL).unwrap();
    assert!(!header.is_sensitive());

    for sensitive in [false, true] {
        header.set_sensitive(sensitive);
        let api_key = Some(header.clone());
        let config = ConfigDebug {
            url: &url,
            api_key: &api_key,
            http_timeout: &timeout,
            probe_url: &true,
        };
        for output in [format!("{config:?}"), format!("{config:#?}")] {
            assert!(!output.contains(CREDENTIAL));
            assert!(output.contains("[REDACTED]"));
            assert!(output.contains("localhost"));
            assert!(output.contains("17s"));
            assert!(output.contains("true"));
        }
        let header = api_key.as_ref().unwrap();
        assert_eq!(header.as_bytes(), CREDENTIAL.as_bytes());
        assert_eq!(header.is_sensitive(), sensitive);
    }
}

#[test]
fn api_client_debug_credential_deserialization_marks_headers_sensitive() {
    for credential in [CREDENTIAL, ""] {
        let json = serde_json::to_string(credential).unwrap();
        let mut deserializer = serde_json::Deserializer::from_str(&json);
        let api_key = deserialize_api_key(&mut deserializer).unwrap();
        deserializer.end().unwrap();
        let header = api_key.as_ref().unwrap();
        assert!(header.is_sensitive());
        assert_eq!(header.as_bytes(), credential.as_bytes());

        let mut request = Request::new(Method::GET, "http://localhost:8080/".parse().unwrap());
        let header_name = HeaderName::from_static("x-api-key");
        request
            .headers_mut()
            .insert(header_name.clone(), header.clone());
        let transported = request.headers().get(&header_name).unwrap();
        assert_eq!(transported.as_bytes(), credential.as_bytes());
        assert!(transported.is_sensitive());
        assert!(format!("{transported:?}").contains("Sensitive"));
        if !credential.is_empty() {
            assert!(!format!("{request:?}").contains(credential));
        }

        let config = ConfigDebug {
            url: request.url(),
            api_key: &api_key,
            http_timeout: &Duration::from_secs(30),
            probe_url: &false,
        };
        for output in [format!("{config:?}"), format!("{config:#?}")] {
            assert!(output.contains("[REDACTED]"));
            assert!(!output.contains(CREDENTIAL));
        }
    }
}

#[test]
fn api_client_debug_credential_null_remains_absent() {
    let mut deserializer = serde_json::Deserializer::from_str("null");
    let api_key = deserialize_api_key(&mut deserializer).unwrap();
    deserializer.end().unwrap();
    assert!(api_key.is_none());
    let config = ConfigDebug {
        url: &"http://localhost:8080/".parse().unwrap(),
        api_key: &api_key,
        http_timeout: &Duration::from_secs(30),
        probe_url: &false,
    };
    assert!(format!("{config:?}").contains("api_key: None"));
}

#[test]
fn api_client_debug_credential_rejects_invalid_headers() {
    for credential in [
        "credential\rvalue",
        "credential\nvalue",
        "credential\0value",
    ] {
        let json = serde_json::to_string(credential).unwrap();
        let mut deserializer = serde_json::Deserializer::from_str(&json);
        assert!(deserialize_api_key(&mut deserializer).is_err());
    }
}
