use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

fn parity(arguments: &[&std::ffi::OsStr]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_agent-sdk-parity"))
        .args(arguments)
        .output()
        .expect("run the parity binary")
}

fn stderr(output: &Output) -> String {
    String::from_utf8(output.stderr.clone()).expect("UTF-8 diagnostic")
}

struct ScenarioFile(PathBuf);

impl ScenarioFile {
    fn new() -> Self {
        let directory =
            std::env::temp_dir().join(format!("agent-sdk-parity-arguments-{}", std::process::id()));
        fs::create_dir(&directory).expect("create isolated scenario directory");
        Self(directory.join("scenarios.kvx"))
    }

    fn check(&self, source: &str) -> Output {
        fs::write(&self.0, source).expect("write scenario file");
        parity(&["--check-scenarios".as_ref(), self.0.as_os_str()])
    }
}

impl Drop for ScenarioFile {
    fn drop(&mut self) {
        fs::remove_dir_all(self.0.parent().expect("scenario directory"))
            .expect("remove isolated scenario directory");
    }
}

#[test]
fn parity_requires_a_node_json_rpc_url() {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    let repository = manifest.join("missing-repository-for-argument-check");
    assert!(!repository.exists());
    for invalid in [
        manifest.join("Cargo.toml").into_os_string(),
        "/usr/bin/true".into(),
        "/usr/bin/layerxd".into(),
        "./layerxd".into(),
        "file:///usr/bin/layerxd".into(),
        "ws://localhost:8545".into(),
        "ftp://localhost/rpc".into(),
        "localhost:8545".into(),
        "http:localhost:8545".into(),
        "http://".into(),
        "https://".into(),
        "http://localhost:99999".into(),
        " http://localhost:8545".into(),
        "https://localhost/rpc\n".into(),
        "http://local\nhost:8545".into(),
        "https://localhost/r\tpc".into(),
    ] {
        let output = parity(&[&invalid, repository.as_os_str()]);
        assert!(!output.status.success(), "accepted {invalid:?}");
        assert!(
            stderr(&output).contains("expected a node JSON-RPC URL"),
            "wrong refusal for {invalid:?}: {}",
            stderr(&output)
        );
        assert!(
            stderr(&output).contains("requires an http or https node JSON-RPC URL"),
            "wrong refusal for {invalid:?}: {}",
            stderr(&output)
        );
    }
    for url in [
        "http://127.0.0.1:8545",
        "https://localhost:8545/rpc",
        "http://[::1]:8545/rpc?network=parity",
    ] {
        let output = parity(&[url.as_ref(), repository.as_os_str()]);
        assert!(!output.status.success());
        let diagnostic = stderr(&output);
        assert!(diagnostic.contains("could not read"), "{url}: {diagnostic}");
        assert!(diagnostic.contains("scenarios.kvx"), "{diagnostic}");
    }
    let missing = parity(&[]);
    assert!(!missing.status.success());
    assert!(stderr(&missing).contains("missing http or https node JSON-RPC URL"));
    let missing_repository = parity(&["https://localhost/rpc".as_ref()]);
    assert!(!missing_repository.status.success());
    assert!(stderr(&missing_repository).contains("missing repository root"));
    let extra = parity(&[
        "http://localhost/rpc".as_ref(),
        repository.as_os_str(),
        "unexpected".as_ref(),
    ]);
    assert!(!extra.status.success());
    assert!(stderr(&extra).contains("unexpected parity-suite argument"));
}

#[test]
fn parity_scenarios_record_rungs() {
    let fixture = Path::new(env!("CARGO_MANIFEST_DIR")).join("scenarios.kvx");
    let source = fs::read_to_string(&fixture).expect("read parity scenarios");
    let output = parity(&["--check-scenarios".as_ref(), fixture.as_os_str()]);
    assert!(output.status.success(), "{}", stderr(&output));
    assert!(String::from_utf8(output.stdout)
        .expect("UTF-8 scenario report")
        .contains("21 parity scenarios use submitted, receipted or committed"));

    let observations: Vec<_> = source
        .lines()
        .filter(|line| line.starts_with("verification = "))
        .collect();
    assert_eq!(observations.len(), 21);
    for observation in &observations {
        assert!(
            matches!(
                *observation,
                "verification = \"submitted\""
                    | "verification = \"receipted\""
                    | "verification = \"committed\""
            ),
            "retired level in {observation}"
        );
    }
    let file = ScenarioFile::new();
    for duplicate in [
        "verification = \"StateProven\"\nverification = \"committed\"",
        "verification = \"committed\"\nverification = \"StateProven\"",
    ] {
        let candidate = source.replacen("verification = \"submitted\"", duplicate, 1);
        let output = file.check(&candidate);
        assert!(!output.status.success(), "accepted duplicate verification");
        assert!(stderr(&output).contains("duplicate field verification"));
    }
    for rung in ["submitted", "receipted", "committed"] {
        let candidate = source
            .lines()
            .map(|line| {
                if line.starts_with("verification = ") {
                    format!("verification = \"{rung}\"")
                } else {
                    line.to_owned()
                }
            })
            .collect::<Vec<_>>()
            .join("\n");
        let output = file.check(&candidate);
        assert!(output.status.success(), "{rung}: {}", stderr(&output));
    }
    let mut scenario = "";
    for (index, line) in source.lines().enumerate() {
        if let Some(name) = line
            .strip_prefix("[scenario.")
            .and_then(|name| name.strip_suffix(']'))
        {
            scenario = name;
        }
        if !line.starts_with("verification = ") {
            continue;
        }
        for retired in [
            "Unverified",
            "SequencerSigned",
            "sequencer_signed",
            "StateProven",
            "Finalised",
            "Anchored",
            "unknown",
            "Committed",
            "",
        ] {
            let candidate = source
                .lines()
                .enumerate()
                .map(|(position, line)| {
                    if position == index {
                        format!("verification = \"{retired}\"")
                    } else {
                        line.to_owned()
                    }
                })
                .collect::<Vec<_>>()
                .join("\n");
            let output = file.check(&candidate);
            assert!(
                !output.status.success(),
                "accepted {retired:?} at line {index}"
            );
            let diagnostic = stderr(&output);
            assert!(diagnostic.contains("scenario "), "{diagnostic}");
            assert!(
                diagnostic.contains(&format!("scenario {scenario}:")),
                "{diagnostic}"
            );
            assert!(
                diagnostic.contains("invalid verification rung"),
                "{diagnostic}"
            );
        }
    }
    let missing_path = parity(&["--check-scenarios".as_ref()]);
    assert!(!missing_path.status.success());
    assert!(stderr(&missing_path).contains("--check-scenarios <path>"));
    let extra = parity(&[
        "--check-scenarios".as_ref(),
        fixture.as_os_str(),
        "unexpected".as_ref(),
    ]);
    assert!(!extra.status.success());
    assert!(stderr(&extra).contains("--check-scenarios <path>"));
}
