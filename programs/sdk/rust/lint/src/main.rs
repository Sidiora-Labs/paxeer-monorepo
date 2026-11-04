//! Build-time determinism gate for `LayerX` program projects.
//!
//! The lint is a build step, not a report: it exits non-zero and names the
//! violated rule, so a program that reaches for a clock, for randomness, for
//! floating point or for a host interface outside its explicitly recorded
//! frozen ABI surface fails its build before deployment ever sees it.

use std::env;
use std::fs;
use std::path::Path;
use std::process::ExitCode;

use layerx_program_lint::cli::{parse_version, project_violations};
use layerx_program_lint::{abi_surface_violations, lint_artifact_for_abi, DeterminismViolation};

const USAGE: &str = "usage: layerx-program-lint <project-directory> [artifact.wasm]\n       layerx-program-lint --abi-version <1|2|3|4|5> <project-directory> [artifact.wasm]\n       layerx-program-lint --abi-version <1|2|3|4|5> --artifact <artifact.wasm>\n       layerx-program-lint --abi";
const USAGE_STATUS: u8 = 2;

fn main() -> ExitCode {
    let arguments: Vec<String> = env::args().skip(1).collect();
    let requested: Vec<&str> = arguments.iter().map(String::as_str).collect();
    let violations = match requested.as_slice() {
        ["--abi"] => abi_surface_violations(),
        ["--abi-version", version, "--artifact", artifact] => {
            let Some(version) = parse_version(version) else {
                return usage_refusal("ABI version must be exactly 1, 2, 3, 4 or 5");
            };
            artifact_violations(Path::new(artifact), version)
        }
        ["--artifact", _] => {
            return usage_refusal("artifact lint requires --abi-version <1|2|3|4|5>");
        }
        ["--abi-version", version, project] => {
            project_violations(Path::new(project), None, Some(version))
        }
        ["--abi-version", version, project, artifact] => {
            project_violations(Path::new(project), Some(Path::new(artifact)), Some(version))
        }
        [project] => project_violations(Path::new(project), None, None),
        [project, artifact] => {
            project_violations(Path::new(project), Some(Path::new(artifact)), None)
        }
        _ => {
            eprintln!("{USAGE}");
            return ExitCode::from(USAGE_STATUS);
        }
    };
    if violations.is_empty() {
        return ExitCode::SUCCESS;
    }
    for violation in &violations {
        eprintln!("determinism violation [{}]: {violation}", violation.name());
    }
    ExitCode::FAILURE
}

fn artifact_violations(artifact: &Path, abi_version: u16) -> Vec<DeterminismViolation> {
    match fs::read(artifact) {
        Ok(wasm) => lint_artifact_for_abi(&wasm, abi_version),
        Err(error) => vec![DeterminismViolation::UnreadablePath {
            path: artifact.to_path_buf(),
            reason: error.to_string(),
        }],
    }
}

fn usage_refusal(reason: &str) -> ExitCode {
    eprintln!("{reason}");
    eprintln!("{USAGE}");
    ExitCode::from(USAGE_STATUS)
}
