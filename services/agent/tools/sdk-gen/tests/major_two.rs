#[allow(dead_code)]
#[path = "../src/main.rs"]
mod generator;

use std::io::Write as _;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

fn generate() -> generator::Generated {
    let schema = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../schema/agent-api");
    generator::agent_sdk_generator(&schema)
        .unwrap_or_else(|error| panic!("generate major-2 SDK from canonical schema: {error}"))
}

fn output<'a>(generated: &'a generator::Generated, path: &str) -> &'a str {
    generated
        .files
        .get(Path::new(path))
        .unwrap_or_else(|| panic!("missing generated output {path}"))
}

#[test]
fn generated_typescript_declares_contract_major_two_and_rungs() {
    let generated = generate();
    let typescript = output(&generated, "typescript/src/generated/client.ts");
    let metadata = typescript
        .split_once("const PACKAGE_METADATA = Object.freeze({")
        .and_then(|(_, rest)| rest.split_once("});"))
        .map(|(metadata, _)| metadata)
        .unwrap_or_else(|| panic!("generated package metadata missing"));
    assert_eq!(
        metadata
            .lines()
            .map(str::trim)
            .filter(|line| line.starts_with("contractMajor:"))
            .collect::<Vec<_>>(),
        ["contractMajor: 2,"]
    );
    let rungs = typescript
        .split_once("export enum Rung {")
        .and_then(|(_, rest)| rest.split_once('}'))
        .map(|(body, _)| body)
        .unwrap_or_else(|| panic!("generated Rung enum missing"));
    assert_eq!(
        rungs
            .lines()
            .map(str::trim)
            .filter(|line| !line.is_empty())
            .collect::<Vec<_>>(),
        ["submitted = 0,", "receipted = 1,", "committed = 2,"]
    );
    assert!(output(&generated, "python/layerx_sdk/generated/client.py")
        .contains("\"contract_major\": 2,"));
}

#[test]
fn generated_bindings_use_65_byte_signatures() {
    let generated = generate();
    let typescript = output(&generated, "typescript/src/generated/client.ts");
    assert!(
        typescript.contains("export type SignatureBytes = Uint8Array & { readonly length: 65 };")
    );
    assert!(typescript.contains("export type AddressBytes = Uint8Array & { readonly length: 20 };"));
    assert!(typescript.contains("value.length !== 65"));
    assert!(typescript.contains("value.length !== 20"));
    for request in ["SignRequest", "SubmitRequest"] {
        let body = typescript
            .split_once(&format!("export interface {request} {{"))
            .and_then(|(_, rest)| rest.split_once('}'))
            .map(|(body, _)| body)
            .unwrap_or_else(|| panic!("missing TypeScript {request}"));
        assert!(body.contains("signature: SignatureBytes;"));
    }
    let stub = output(&generated, "python/layerx_sdk/generated/client.pyi");
    for (name, length) in [("SignatureBytes", 65), ("AddressBytes", 20)] {
        let body = stub
            .split_once(&format!("class {name}(bytes):"))
            .and_then(|(_, rest)| rest.split_once("\n\n"))
            .map(|(body, _)| body)
            .unwrap_or_else(|| panic!("missing Python {name} stub"));
        assert!(body.contains(&format!("def __len__(self) -> Literal[{length}]: ...")));
    }
    let python = output(&generated, "python/layerx_sdk/generated/client.py");
    let assertions = r#"
for binding, size in ((SignatureBytes, 65), (AddressBytes, 20)):
    payload = bytes(range(size))
    value = binding(payload)
    assert isinstance(value, binding)
    assert isinstance(value, bytes)
    assert len(value) == size
    assert value == payload
    for length in (0, size - 1, size + 1, size * 2):
        try:
            binding(bytes(length))
        except ValueError:
            pass
        else:
            raise AssertionError((binding.__name__, length))
    for invalid in (size, "00" * size, [0] * size, bytearray(size)):
        try:
            binding(invalid)
        except ValueError:
            pass
        else:
            raise AssertionError((binding.__name__, type(invalid)))
for request in (SignRequest, SubmitRequest):
    assert request.__annotations__["signature"] is SignatureBytes
assert layerx_sdk_py_package()["contract_major"] == 2
assert tuple(member.name for member in Rung) == ("SUBMITTED", "RECEIPTED", "COMMITTED")
"#;
    let mut child = Command::new("python3")
        .arg("-")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("execute generated Python bindings: {error}"));
    let mut input = child
        .stdin
        .take()
        .unwrap_or_else(|| panic!("Python stdin missing"));
    write!(input, "{python}\n{assertions}")
        .unwrap_or_else(|error| panic!("write generated Python bindings: {error}"));
    drop(input);
    let result = child
        .wait_with_output()
        .unwrap_or_else(|error| panic!("wait for generated Python bindings: {error}"));
    assert!(
        result.status.success(),
        "generated Python binding validation failed: {}",
        String::from_utf8_lossy(&result.stderr)
    );
}
