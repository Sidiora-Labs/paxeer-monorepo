use std::process::Command;

fn startup_error(url: &str, flag: Option<&str>) -> String {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"));
    command.env_clear().env("LAYERX_INTEROP_SEQUENCER_URL", url);
    if let Some(flag) = flag {
        command.env("LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK", flag);
    }
    let output = command.output().expect("interop gateway runs");
    assert!(
        !output.status.success(),
        "gateway started without its configuration"
    );
    String::from_utf8_lossy(&output.stderr).into_owned()
}

const PRIVATE: &str = "http://router.railway.internal:9443";
const REFUSED: &str =
    "LAYERX_INTEROP_SEQUENCER_URL may use http:// only as http://<name>.railway.internal:<port>";
const PASSED: &str = "LAYERX_INTEROP_CONFIG is required";

#[test]
fn flag_off_refuses_plain_sequencer_url() {
    for flag in [None, Some("0")] {
        let stderr = startup_error(PRIVATE, flag);
        assert!(
            stderr.contains("component endpoint must use HTTPS"),
            "{stderr}"
        );
    }
}

#[test]
fn flag_on_accepts_private_plain_sequencer_url() {
    let stderr = startup_error(PRIVATE, Some("1"));
    assert!(stderr.contains(PASSED), "{stderr}");
}

#[test]
fn flag_on_refuses_public_plain_sequencer_url() {
    let stderr = startup_error("http://example.com:9443", Some("1"));
    assert!(stderr.contains(REFUSED), "{stderr}");
}

#[test]
fn flag_on_refuses_plain_sequencer_url_without_port() {
    for url in [
        "http://router.railway.internal",
        "http://router.railway.internal:",
        "http://router.railway.internal:9443/",
    ] {
        let stderr = startup_error(url, Some("1"));
        assert!(stderr.contains(REFUSED), "{url}: {stderr}");
    }
}

#[test]
fn malformed_flag_fails_closed() {
    for flag in ["", "true", "yes", " 1"] {
        let stderr = startup_error(PRIVATE, Some(flag));
        assert!(
            stderr.contains("LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK must be 0 or 1"),
            "{flag:?}: {stderr}"
        );
    }
}

#[test]
fn flag_on_keeps_https_sequencer_url() {
    let stderr = startup_error("https://router.railway.internal:9443", Some("1"));
    assert!(stderr.contains(PASSED), "{stderr}");
}

#[test]
fn arguments_are_refused_before_configuration() {
    for args in [
        vec!["--verify-readiness-receipt", "/missing/receipt.json"],
        vec!["--verify-readiness-receipt"],
        vec!["--unknown"],
        vec![""],
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"))
            .env_clear()
            .args(&args)
            .output()
            .expect("interop gateway runs");
        assert_eq!(output.status.code(), Some(1), "{args:?}");
        assert_eq!(
            String::from_utf8_lossy(&output.stderr),
            "layerx interoperability gateway refused startup: invalid interoperability gateway arguments\n",
            "{args:?}"
        );
    }
}
