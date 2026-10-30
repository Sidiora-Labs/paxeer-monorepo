#[allow(dead_code)]
#[path = "../src/main.rs"]
mod generator;

use std::fs;
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

fn declaration<'a>(source: &'a str, start: &str, end: &str) -> &'a str {
    source
        .split_once(start)
        .and_then(|(_, rest)| rest.split_once(end))
        .map(|(body, _)| body)
        .unwrap_or_else(|| panic!("missing generated declaration between {start:?} and {end:?}"))
}

#[test]
fn generated_submission_state_names_the_rungs() {
    let schema = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/agent-api");
    let generated = generator::agent_sdk_generator(&schema)
        .unwrap_or_else(|error| panic!("generate SDK from canonical schema: {error}"));
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("temporary output timestamp: {error}"))
        .as_nanos();
    let root = std::env::temp_dir().join(format!(
        "agent-sdk-rung-states-{}-{nonce}",
        std::process::id()
    ));
    fs::create_dir(&root).unwrap_or_else(|error| panic!("create temporary SDK directory: {error}"));
    for (relative, source) in &generated.files {
        let path = root.join(relative);
        fs::create_dir_all(
            path.parent()
                .unwrap_or_else(|| panic!("SDK parent missing")),
        )
        .unwrap_or_else(|error| panic!("create generated SDK parent: {error}"));
        fs::write(&path, source)
            .unwrap_or_else(|error| panic!("write generated {}: {error}", path.display()));
    }

    let typescript = fs::read_to_string(root.join("typescript/src/generated/client.ts"))
        .unwrap_or_else(|error| panic!("read generated TypeScript client: {error}"));
    let state = declaration(
        &typescript,
        "export type SubmissionState =",
        "\n\nexport interface VerifiedRead",
    );
    assert_eq!(
        state
            .lines()
            .map(str::trim)
            .filter(|line| !line.is_empty())
            .collect::<Vec<_>>(),
        [
            "| { kind: \"Submitted\"; txHash: string }",
            "| { kind: \"Receipted\" }",
            "| { kind: \"Committed\" }",
            "| { kind: \"Reverted\"; revertReason: string };",
        ],
        "TypeScript submission variants and payloads"
    );
    for retired in [
        "Unknown",
        "Executed",
        "Failed",
        "Pending",
        "receiptRef",
        "protocolResultCode",
        "stage",
    ] {
        assert!(
            !state.contains(retired),
            "retired TypeScript submission member: {retired}"
        );
    }

    let variants = ["Submitted", "Receipted", "Committed", "Reverted"];
    let alias =
        "SubmissionSubmitted | SubmissionReceipted | SubmissionCommitted | SubmissionReverted";
    for (relative, stub) in [
        ("python/layerx_sdk/generated/client.py", false),
        ("python/layerx_sdk/generated/client.pyi", true),
    ] {
        let python = fs::read_to_string(root.join(relative))
            .unwrap_or_else(|error| panic!("read generated {relative}: {error}"));
        assert_eq!(
            python
                .lines()
                .filter_map(|line| line.strip_prefix("class Submission"))
                .collect::<Vec<_>>(),
            ["Submitted:", "Receipted:", "Committed:", "Reverted:"],
            "{relative} must declare exactly the four submission classes"
        );
        for variant in variants {
            let class = format!("class Submission{variant}:\n");
            let body = declaration(&python, &class, "\n\n");
            let payload = match variant {
                "Submitted" => Some("tx_hash"),
                "Reverted" => Some("revert_reason"),
                _ => None,
            };
            let kind = if stub {
                format!("kind: Literal[\"{variant}\"]")
            } else {
                assert!(
                    python.contains(&format!("@dataclass(frozen=True)\n{class}")),
                    "{relative}: {variant} must be frozen"
                );
                format!("kind: Literal[\"{variant}\"] = \"{variant}\"")
            };
            let mut expected = Vec::new();
            if let Some(field) = payload {
                expected.push(format!("{field}: str"));
            }
            expected.push(kind);
            if stub {
                let arguments = payload.map_or_else(String::new, |field| format!("{field}: str, "));
                expected.push(format!("def __init__(self, {arguments}kind: Literal[\"{variant}\"] = \"{variant}\") -> None: ..."));
            }
            assert_eq!(
                body.lines().map(str::trim).collect::<Vec<_>>(),
                expected,
                "{relative}: {variant} fields and constructor"
            );
        }
        let state = declaration(&python, "SubmissionState: TypeAlias = ", "\n\n");
        if stub {
            assert_eq!(state, alias, "{relative}: submission union");
        } else {
            assert_eq!(
                state,
                format!("(\n    {alias}\n)"),
                "{relative}: submission union"
            );
        }
        let declarations = declaration(
            &python,
            "class SubmissionSubmitted:",
            "\nclass IdempotentMutation",
        );
        for retired in [
            "Unknown",
            "Executed",
            "Failed",
            "Pending",
            "receiptRef",
            "receipt_ref",
            "protocolResultCode",
            "protocol_result_code",
            "stage",
        ] {
            assert!(
                !declarations.contains(retired),
                "{relative}: retired submission member {retired}"
            );
        }
    }

    fs::remove_dir_all(&root)
        .unwrap_or_else(|error| panic!("remove temporary SDK directory: {error}"));
}
