#[allow(dead_code)]
#[path = "../src/main.rs"]
mod release;

use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use release::workflow;
use release::{plan, registries_declarations, release_pipeline, release_pipeline_in_repository};

const RELEASE_CONDITION: &str =
    "github.event_name == 'workflow_dispatch' || startsWith(github.ref, 'refs/tags/sdk-v')";
const TAG_CONDITION: &str = "startsWith(github.ref, 'refs/tags/sdk-v')";
static WORKFLOW_SEQUENCE: AtomicU64 = AtomicU64::new(0);

struct RepositoryWorkflows(PathBuf);

impl RepositoryWorkflows {
    fn new() -> Self {
        let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "registries-workflows-{}-{}",
            std::process::id(),
            WORKFLOW_SEQUENCE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir_all(root.join(".github/workflows"))
            .unwrap_or_else(|error| panic!("create workflow repository: {error:?}"));
        for file in ["agent.yml", "programs-conformance.yml"] {
            fs::write(
                root.join(".github/workflows").join(file),
                read(&format!("../../../.github/workflows/{file}")),
            )
            .unwrap_or_else(|error| panic!("copy committed called workflow: {error:?}"));
        }
        Self(root)
    }

    fn replace(&self, file: &str, from: &str, to: &str) {
        let path = self.0.join(".github/workflows").join(file);
        let source = fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("read called workflow: {error:?}"));
        fs::write(path, replaced(&source, from, to))
            .unwrap_or_else(|error| panic!("change called workflow: {error:?}"));
    }

    fn pipeline(&self) -> Result<release::ReleasePipeline, String> {
        release_pipeline_in_repository(&committed_manifest(), &committed_workflow(), &self.0)
    }
}

impl Drop for RepositoryWorkflows {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove workflow repository: {error:?}"));
    }
}

fn read(relative: &str) -> String {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(relative);
    fs::read_to_string(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()))
}

fn committed_manifest() -> String {
    read("registries.kvx")
}

fn committed_workflow() -> String {
    read("../../../.github/workflows/platform.yml")
}

fn committed_pipeline() -> release::ReleasePipeline {
    release_pipeline(&committed_manifest(), &committed_workflow())
        .unwrap_or_else(|error| panic!("committed release pipeline refused: {error}"))
}

#[test]
fn release_pipeline_has_no_replay_gate() {
    let mut source = committed_workflow();
    if let Some(start) = source.find("  replay-matrix:\n") {
        let end = start
            + source[start..]
                .find("  release-pipeline:\n")
                .unwrap_or_else(|| panic!("release-pipeline must follow replay-matrix"));
        source.replace_range(start..end, "");
    }
    source = source.replace(", replay-matrix]", "]");
    let document = workflow::parse(&source)
        .unwrap_or_else(|error| panic!("parse release workflow without replay: {error:?}"));
    assert!(document.path(&["jobs", "replay-matrix"]).is_none());
    let dependencies = document
        .path(&["jobs", "release-pipeline", "needs"])
        .unwrap_or_else(|| panic!("release-pipeline needs"))
        .strings();
    assert_eq!(
        dependencies,
        vec![
            "programs-acceptance",
            "agent-sanitizers",
            "agent-fuzz-corpus"
        ]
    );
    let pipeline = release_pipeline(&committed_manifest(), &source).unwrap_or_else(|error| {
        panic!("release pipeline must accept a workflow without replay: {error:?}")
    });
    assert_eq!(
        pipeline
            .gates
            .iter()
            .map(|gate| gate.job.as_str())
            .collect::<Vec<_>>(),
        dependencies
    );
    assert!(pipeline
        .gates
        .iter()
        .all(|gate| { gate.job != "replay-matrix" && gate.command != "make test-replay-golden" }));
    assert_eq!(committed_pipeline().gates, pipeline.gates);
    let rendered =
        plan(&pipeline).unwrap_or_else(|error| panic!("release plan without replay: {error:?}"));
    assert!(!rendered.contains("replay-matrix"));
    assert!(!rendered.contains("make test-replay-golden"));
}

#[test]
fn committed_manifest_declares_all_seven_registries() {
    let declarations = registries_declarations(&committed_manifest())
        .unwrap_or_else(|error| panic!("manifest refused: {error}"));
    let names = declarations
        .registries
        .iter()
        .map(|registry| registry.name.as_str())
        .collect::<Vec<_>>();
    assert_eq!(
        names,
        vec![
            "crates-io",
            "npm",
            "pypi",
            "go-modules",
            "maven-central",
            "swiftpm",
            "nuget"
        ]
    );
}

#[test]
fn every_registry_is_active_with_real_sorted_package_identities() {
    let declarations = registries_declarations(&committed_manifest())
        .unwrap_or_else(|error| panic!("manifest refused: {error}"));
    for registry in &declarations.registries {
        assert!(
            !registry.packages.is_empty(),
            "{} declares no packages",
            registry.name
        );
        let mut sorted = registry.packages.clone();
        sorted.sort();
        sorted.dedup();
        assert_eq!(
            sorted, registry.packages,
            "{} packages unsorted",
            registry.name
        );
        assert!(
            registry
                .declarations
                .contains(&("status".to_owned(), "active".to_owned())),
            "{} is not active",
            registry.name
        );
    }
    let package = |name: &str| {
        declarations
            .registries
            .iter()
            .find(|registry| registry.name == name)
            .map_or_else(
                || panic!("{name} missing"),
                |registry| registry.packages.clone(),
            )
    };
    assert!(package("npm").contains(&"@sidiora/layerx-sdk".to_owned()));
    assert!(package("crates-io").contains(&"layerx-sdk".to_owned()));
    assert!(package("pypi").contains(&"layerx-sdk".to_owned()));
    assert_eq!(
        package("go-modules"),
        vec!["github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"]
    );
    assert!(package("maven-central").contains(&"com.sidiora.layerx:layerx-sdk".to_owned()));
    assert_eq!(package("swiftpm"), vec!["LayerXSDK"]);
    assert_eq!(package("nuget"), vec!["LayerX.Sdk"]);
}

#[test]
fn go_module_release_tags_the_module_directory() {
    let declarations = registries_declarations(&committed_manifest())
        .unwrap_or_else(|error| panic!("committed registry manifest must parse: {error:?}"));
    let registry = declarations
        .registries
        .iter()
        .find(|registry| registry.name == "go-modules")
        .unwrap_or_else(|| panic!("Go module registry missing"));
    let manifest = read("../../../sdk/platform/go/go.mod");
    let module = manifest
        .lines()
        .next()
        .and_then(|line| line.strip_prefix("module "))
        .unwrap_or_else(|| panic!("Go SDK manifest must start with its module declaration"));
    assert_eq!(
        module,
        "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"
    );
    assert_eq!(registry.packages, vec![module]);
    assert!(module.ends_with("/sdk/platform/go"));

    let source = committed_workflow();
    let document = workflow::parse(&source)
        .unwrap_or_else(|error| panic!("committed workflow must parse: {error:?}"));
    let job = document
        .path(&["jobs", "publish-go-modules"])
        .unwrap_or_else(|| panic!("Go module publication job missing"));
    assert_eq!(
        job.path(&["env", "LAYERX_RELEASE_PACKAGES"])
            .and_then(workflow::Node::as_str),
        Some(module)
    );
    let steps = job
        .get("steps")
        .unwrap_or_else(|| panic!("Go module publication steps missing"))
        .items();
    let signing = steps
        .iter()
        .filter_map(|step| step.get("run").and_then(workflow::Node::as_str))
        .find(|script| script.contains("git tag -s"))
        .unwrap_or_else(|| panic!("Go module signed tag publication missing"));
    assert!(signing.contains("tag=sdk/platform/go/v$LAYERX_RELEASE_VERSION"));
    assert!(signing.contains("git tag -s \"$tag\""));
    assert!(signing.contains("repository=${LAYERX_RELEASE_PACKAGES%/sdk/platform/go}"));
    assert!(signing.contains(
        "git push \"https://x-access-token:$LAYERX_RELEASE_GO_TAG_TOKEN@$repository.git\" \"refs/tags/sdk/platform/go/v$LAYERX_RELEASE_VERSION\""
    ));
    assert!(!source.contains("platform/sdk/go"));
}

#[test]
fn committed_workflow_publishes_every_declared_registry_and_nothing_else() {
    let pipeline = committed_pipeline();
    let jobs = pipeline
        .publications
        .iter()
        .map(|publication| (publication.registry.as_str(), publication.job.as_str()))
        .collect::<Vec<_>>();
    assert_eq!(
        jobs,
        vec![
            ("crates-io", "publish-crates-io"),
            ("npm", "publish-npm"),
            ("pypi", "publish-pypi"),
            ("go-modules", "publish-go-modules"),
            ("maven-central", "publish-maven-central"),
            ("swiftpm", "publish-swiftpm"),
            ("nuget", "publish-nuget"),
        ]
    );
    for (publication, registry) in pipeline
        .publications
        .iter()
        .zip(&pipeline.declarations.registries)
    {
        assert_eq!(publication.packages, registry.packages);
    }
}

#[test]
fn committed_workflow_requires_every_release_gate() {
    let pipeline = committed_pipeline();
    let gates = pipeline
        .gates
        .iter()
        .map(|gate| (gate.job.as_str(), gate.command.as_str()))
        .collect::<Vec<_>>();
    assert_eq!(
        gates,
        vec![
            ("programs-acceptance", "make programs-test"),
            ("agent-sanitizers", "make agent-test-sanitize"),
            ("agent-fuzz-corpus", "make agent-fuzz-long"),
        ]
    );
}

#[test]
fn committed_reusable_workflows_emit_release_gate_records() {
    let source = committed_workflow();
    let document = workflow::parse(&source)
        .unwrap_or_else(|error| panic!("parse committed release workflow: {error:?}"));
    for (gate, file, called_job, command) in [
        (
            "programs-acceptance",
            "programs-conformance.yml",
            "acceptance",
            "make programs-test",
        ),
        (
            "agent-sanitizers",
            "agent.yml",
            "adversarial",
            "make agent-test-sanitize",
        ),
        (
            "agent-fuzz-corpus",
            "agent.yml",
            "fuzz-corpus",
            "make agent-fuzz-long",
        ),
    ] {
        assert_eq!(
            document
                .path(&["jobs", gate, "uses"])
                .and_then(workflow::Node::as_str),
            Some(format!("./.github/workflows/{file}").as_str())
        );
        let called = workflow::parse(&read(&format!("../../../.github/workflows/{file}")))
            .unwrap_or_else(|error| panic!("parse called workflow: {error:?}"));
        let run = called
            .path(&["jobs", called_job, "steps"])
            .unwrap_or_else(|| panic!("called job steps"))
            .items()
            .iter()
            .filter_map(|step| step.get("run").and_then(workflow::Node::as_str))
            .find(|run| run.contains(command))
            .unwrap_or_else(|| panic!("gate command step"));
        assert!(
            run.contains(&format!("scripts/ci/release-gate.sh --job {gate}")),
            "{file}: {run}"
        );
        for argument in ["--spec", "--task", "--ledger", "--evidence-root"] {
            assert!(run.contains(argument), "{file}: missing {argument}");
        }
        assert!(
            run.contains(&format!("-- {command}")),
            "{file}: gate command must go through runner"
        );
    }
    let pipeline = release_pipeline(&committed_manifest(), &source).unwrap_or_else(|error| {
        panic!("committed workflow must follow the reusable release gates: {error:?}")
    });
    assert_eq!(pipeline.gates.len(), 3);
}

#[test]
fn reusable_workflow_without_the_gate_runner_is_refused_at_its_step() {
    let repository = RepositoryWorkflows::new();
    repository.replace(
        "programs-conformance.yml",
        "scripts/ci/release-gate.sh",
        "bash",
    );
    let error = repository
        .pipeline()
        .err()
        .unwrap_or_else(|| panic!("called gate without runner must be refused"));
    assert!(
        error.contains(
            "release gate programs-acceptance must run through scripts/ci/release-gate.sh"
        ),
        "{error}"
    );
    assert!(
        error.contains(".github/workflows/programs-conformance.yml job acceptance step"),
        "{error}"
    );
    assert!(
        error.contains("Run the aggregate Programs acceptance suite"),
        "{error}"
    );
}

#[test]
fn reusable_workflow_without_the_gate_command_is_refused() {
    let repository = RepositoryWorkflows::new();
    repository.replace(
        "agent.yml",
        "-- make agent-fuzz-long",
        "-- make agent-fuzz-wire",
    );
    let error = repository
        .pipeline()
        .err()
        .unwrap_or_else(|| panic!("called workflow missing gate command must be refused"));
    assert!(
        error.contains("release gate agent-fuzz-corpus must run `make agent-fuzz-long`"),
        "{error}"
    );
    assert!(
        error.contains(".github/workflows/agent.yml job fuzz-corpus step"),
        "{error}"
    );
    assert!(error.contains("Run committed fuzz corpus"), "{error}");
}

#[test]
fn a_runner_in_another_called_job_cannot_cover_an_unwrapped_gate() {
    let repository = RepositoryWorkflows::new();
    repository.replace(
        "agent.yml",
        "scripts/ci/release-gate.sh --job agent-fuzz-corpus",
        "bash --job agent-fuzz-corpus",
    );
    let error = repository
        .pipeline()
        .err()
        .unwrap_or_else(|| panic!("sanitizer runner cannot cover the fuzz gate"));
    assert!(
        error
            .contains("release gate agent-fuzz-corpus must run through scripts/ci/release-gate.sh"),
        "{error}"
    );
    assert!(
        error.contains(".github/workflows/agent.yml job fuzz-corpus step"),
        "{error}"
    );
}

#[test]
fn plan_is_deterministic_and_machine_readable() {
    let pipeline = committed_pipeline();
    let first = plan(&pipeline).unwrap_or_else(|error| panic!("plan: {error}"));
    let second = plan(&pipeline).unwrap_or_else(|error| panic!("plan: {error}"));
    assert_eq!(first, second);
    assert_eq!(first.lines().count(), 3 + 7 + 1 + 3 + 1);
    assert!(first.starts_with("tag_format=sdk-v{version}\n"));
    assert!(first
        .lines()
        .nth(2)
        .is_some_and(|line| line.starts_with("reference_applications=")));
    for line in first.lines().skip(3).take(7) {
        assert!(
            line.starts_with("registry="),
            "unexpected plan line: {line}"
        );
        assert!(line.contains(" signing="), "plan line lost signing: {line}");
        assert!(
            line.contains(" verification=byte-compare-against-tagged-source"),
            "plan line lost verification: {line}"
        );
        assert!(
            line.contains(" packages=") && line.contains(" publication_job=publish-"),
            "plan line lost its publication binding: {line}"
        );
    }
    let image_line = first
        .lines()
        .nth(10)
        .unwrap_or_else(|| panic!("plan lost its container registry line"));
    assert!(
        image_line.starts_with("image_registry=ghcr "),
        "unexpected image registry line: {image_line}"
    );
    assert!(
        image_line.contains(" distribution=ghcr.io/sidiora-labs")
            && image_line.contains(" signing=sigstore-keyless")
            && image_line.contains(" sbom=spdx-json")
            && image_line.contains(" platforms=linux/amd64")
            && image_line.contains(" images=layerx-agent-boundary,"),
        "plan line lost its container binding: {image_line}"
    );
    for line in first.lines().skip(11).take(3) {
        assert!(
            line.starts_with("gate=") && line.contains(" command=make "),
            "unexpected gate line: {line}"
        );
    }
    assert!(first
        .lines()
        .last()
        .is_some_and(|line| line.starts_with("verification_job=release-verification ")));
}

#[test]
fn release_plan_carries_every_cloneable_reference_application() {
    let declarations = registries_declarations(&committed_manifest())
        .unwrap_or_else(|error| panic!("manifest refused: {error}"));
    assert_eq!(
        declarations.reference_applications,
        vec![
            "@sidiora/layerx-example-buyer-agent",
            "@sidiora/layerx-example-marketplace",
            "@sidiora/layerx-example-merchant-shop",
            "@sidiora/layerx-example-paid-api",
        ]
    );
}

#[test]
fn maven_release_preserves_its_typed_coordinates() {
    let pipeline = committed_pipeline();
    let maven = pipeline
        .declarations
        .registries
        .iter()
        .find(|registry| registry.name == "maven-central")
        .unwrap_or_else(|| panic!("Maven Central registry missing"));
    assert!(maven.declarations.contains(&(
        "coordinate".to_owned(),
        "com.sidiora.layerx:layerx-sdk".to_owned()
    )));
    assert!(maven
        .declarations
        .contains(&("languages".to_owned(), "java,kotlin".to_owned())));
    assert!(maven.declarations.contains(&(
        "module_name".to_owned(),
        "com.sidiora.layerx.sdk".to_owned()
    )));
    let rendered = plan(&pipeline).unwrap_or_else(|error| panic!("plan: {error}"));
    assert!(rendered.contains(
        "coordinate=com.sidiora.layerx:layerx-sdk languages=java,kotlin module_name=com.sidiora.layerx.sdk"
    ));
}

fn expect_manifest_refusal(source: &str, needle: &str) {
    let error = registries_declarations(source)
        .err()
        .unwrap_or_else(|| panic!("manifest accepted but expected refusal about {needle}"));
    assert!(
        error.contains(needle),
        "expected refusal mentioning {needle}, got: {error}"
    );
}

#[test]
fn committed_manifest_declares_the_container_registry_with_every_beta_image() {
    let declarations = registries_declarations(&committed_manifest())
        .unwrap_or_else(|error| panic!("manifest refused: {error}"));
    let names = declarations
        .image_registries
        .iter()
        .map(|registry| registry.name.as_str())
        .collect::<Vec<_>>();
    assert_eq!(names, vec!["ghcr"]);
    let ghcr = declarations
        .image_registries
        .first()
        .unwrap_or_else(|| panic!("container registry missing"));
    assert_eq!(ghcr.platforms, vec!["linux/amd64"]);
    let mut sorted = ghcr.images.clone();
    sorted.sort();
    sorted.dedup();
    assert_eq!(sorted, ghcr.images, "container images unsorted");
    for declaration in [
        ("ecosystem", "oci"),
        ("artifact", "container-image"),
        ("distribution", "ghcr.io/sidiora-labs"),
        ("signing", "sigstore-keyless"),
        ("provenance", "github-actions-attestation"),
        ("sbom", "spdx-json"),
        ("verification", "digest-pinned-registry-manifest"),
        ("status", "active"),
    ] {
        assert!(
            ghcr.declarations
                .contains(&(declaration.0.to_owned(), declaration.1.to_owned())),
            "container registry lost {}={}",
            declaration.0,
            declaration.1
        );
    }
    for image in [
        "layerx-gateway",
        "layerx-node",
        "layerx-core-boundary",
        "layerx-identity",
        "paxd",
        "paxd-node",
    ] {
        assert!(
            ghcr.images.iter().any(|declared| declared == image),
            "container registry does not declare {image}"
        );
    }
}

#[test]
fn dropping_the_container_registry_from_the_list_is_refused() {
    let source =
        committed_manifest().replace("image_registries = [\"ghcr\"]", "image_registries = []");
    expect_manifest_refusal(&source, "mandated container registries");
}

#[test]
fn missing_image_registry_section_is_refused() {
    let manifest = committed_manifest();
    let start = manifest
        .find("[image_registry.ghcr]")
        .unwrap_or_else(|| panic!("container registry section missing from fixture"));
    expect_manifest_refusal(&manifest[..start], "image registry ghcr is not declared");
}

#[test]
fn unknown_image_registry_section_is_refused() {
    let source = format!(
        "{}\n[image_registry.dockerhub]\necosystem = \"oci\"\n",
        committed_manifest()
    );
    expect_manifest_refusal(&source, "unknown image registry dockerhub");
}

#[test]
fn unknown_image_registry_declaration_is_refused() {
    let source = committed_manifest().replace(
        "[image_registry.ghcr]\necosystem = \"oci\"",
        "[image_registry.ghcr]\nmirror = \"true\"\necosystem = \"oci\"",
    );
    expect_manifest_refusal(&source, "unknown declaration image_registry.ghcr.mirror");
}

#[test]
fn missing_image_registry_declaration_is_refused() {
    let source = committed_manifest().replace("sbom = \"spdx-json\"\n", "");
    expect_manifest_refusal(&source, "missing declaration image_registry.ghcr.sbom");
}

#[test]
fn unsorted_image_list_is_refused() {
    let source = committed_manifest().replace("\"paxd\",\"paxd-node\"]", "\"paxd-node\",\"paxd\"]");
    expect_manifest_refusal(&source, "image_registry.ghcr.images");
}

#[test]
fn image_name_carrying_a_registry_path_is_refused() {
    let source =
        committed_manifest().replace("\"paxd-node\"]", "\"paxd-node\",\"sidiora-labs/paxd-web\"]");
    expect_manifest_refusal(&source, "expected a bare image name");
}

#[test]
fn non_canonical_container_registry_is_refused() {
    let source = committed_manifest().replace(
        "distribution = \"ghcr.io/sidiora-labs\"",
        "distribution = \"docker.io/sidiora\"",
    );
    expect_manifest_refusal(
        &source,
        "image_registry.ghcr.distribution is docker.io/sidiora, not the canonical container registry ghcr.io/sidiora-labs",
    );
}

#[test]
fn unknown_image_sbom_format_is_refused() {
    let source = committed_manifest().replace("sbom = \"spdx-json\"", "sbom = \"prose\"");
    expect_manifest_refusal(&source, "image_registry.ghcr.sbom is prose");
}

#[test]
fn unknown_image_signing_scheme_is_refused() {
    let source = committed_manifest().replace(
        "[image_registry.ghcr]\necosystem = \"oci\"\nartifact = \"container-image\"\ndistribution = \"ghcr.io/sidiora-labs\"\nsigning = \"sigstore-keyless\"",
        "[image_registry.ghcr]\necosystem = \"oci\"\nartifact = \"container-image\"\ndistribution = \"ghcr.io/sidiora-labs\"\nsigning = \"trust-me\"",
    );
    expect_manifest_refusal(&source, "unknown signing scheme trust-me");
}

#[test]
fn unknown_image_registry_status_is_refused() {
    let manifest = committed_manifest();
    let start = manifest
        .find("[image_registry.ghcr]")
        .unwrap_or_else(|| panic!("container registry section missing from fixture"));
    let source = format!(
        "{}{}",
        &manifest[..start],
        manifest[start..].replace("status = \"active\"", "status = \"imagined\"")
    );
    expect_manifest_refusal(&source, "image registry ghcr has unknown status imagined");
}

#[test]
fn image_platform_without_an_architecture_is_refused() {
    let source =
        committed_manifest().replace("platforms = [\"linux/amd64\"]", "platforms = [\"linux\"]");
    expect_manifest_refusal(&source, "expected <os>/<architecture>");
}

fn expect_workflow_refusal(workflow_source: &str, needle: &str) {
    let error = release_pipeline(&committed_manifest(), workflow_source)
        .err()
        .unwrap_or_else(|| panic!("workflow accepted but expected refusal about {needle}"));
    assert!(
        error.contains(needle),
        "expected refusal mentioning {needle}, got: {error}"
    );
}

fn replaced(source: &str, from: &str, to: &str) -> String {
    assert!(source.contains(from), "fixture lost the text {from:?}");
    source.replacen(from, to, 1)
}

#[test]
fn dropping_a_registry_from_the_list_is_refused() {
    let source = committed_manifest().replace(", \"nuget\"]", "]");
    expect_manifest_refusal(&source, "seven mandated registries");
}

#[test]
fn missing_registry_section_is_refused() {
    let manifest = committed_manifest();
    let start = manifest
        .find("[registry.nuget]")
        .unwrap_or_else(|| panic!("nuget section missing from fixture"));
    expect_manifest_refusal(&manifest[..start], "registry nuget is not declared");
}

#[test]
fn missing_signing_declaration_is_refused() {
    let source = committed_manifest().replace("signing = \"nuget-repository-signature\"\n", "");
    expect_manifest_refusal(&source, "missing declaration registry.nuget.signing");
}

#[test]
fn unknown_status_is_refused() {
    let manifest = committed_manifest();
    let start = manifest
        .find("[registry.nuget]")
        .unwrap_or_else(|| panic!("nuget section missing from fixture"));
    let source = format!(
        "{}{}",
        &manifest[..start],
        manifest[start..].replace("status = \"active\"", "status = \"imagined\"")
    );
    expect_manifest_refusal(&source, "unknown status imagined");
}

#[test]
fn skeleton_registry_cannot_be_published() {
    let manifest = committed_manifest();
    let start = manifest
        .find("[registry.nuget]")
        .unwrap_or_else(|| panic!("nuget section missing from fixture"));
    let source = format!(
        "{}{}",
        &manifest[..start],
        manifest[start..].replace("status = \"active\"", "status = \"skeleton\"")
    );
    let error = release_pipeline(&source, &committed_workflow())
        .err()
        .unwrap_or_else(|| panic!("skeleton registry accepted for publication"));
    assert!(error.contains("manifest status is not active"), "{error}");
}

#[test]
fn empty_package_list_is_refused() {
    let source = committed_manifest().replace("packages = [\"LayerX.Sdk\"]", "packages = []");
    expect_manifest_refusal(&source, "registry.nuget.packages");
}

#[test]
fn unsorted_package_list_is_refused() {
    let source = committed_manifest().replace(
        "packages = [\"layerx-fastapi\",\"layerx-sdk\"]",
        "packages = [\"layerx-sdk\",\"layerx-fastapi\"]",
    );
    expect_manifest_refusal(&source, "registry.pypi.packages");
}

#[test]
fn unknown_registry_section_is_refused() {
    let source = format!(
        "{}\n[registry.homebrew]\necosystem = \"ruby\"\n",
        committed_manifest()
    );
    expect_manifest_refusal(&source, "unknown registry homebrew");
}

#[test]
fn unknown_registry_declaration_is_refused() {
    let source = committed_manifest().replace(
        "[registry.npm]\necosystem = \"typescript\"",
        "[registry.npm]\nmirror = \"true\"\necosystem = \"typescript\"",
    );
    expect_manifest_refusal(&source, "unknown declaration registry.npm.mirror");
}

#[test]
fn maven_declarations_are_refused_for_other_registries() {
    let source = committed_manifest().replace(
        "[registry.nuget]\necosystem = \"dotnet\"",
        "[registry.nuget]\ncoordinate = \"invalid:coordinate\"\necosystem = \"dotnet\"",
    );
    expect_manifest_refusal(&source, "unknown declaration registry.nuget.coordinate");
}

#[test]
fn incomplete_maven_coordinates_are_refused() {
    let source =
        committed_manifest().replace("coordinate = \"com.sidiora.layerx:layerx-sdk\"\n", "");
    expect_manifest_refusal(
        &source,
        "missing declaration registry.maven-central.coordinate",
    );
}

#[test]
fn non_canonical_maven_coordinates_are_refused() {
    let coordinate = committed_manifest().replace(
        "coordinate = \"com.sidiora.layerx:layerx-sdk\"",
        "coordinate = \"com.sidiora.layerx:other-sdk\"",
    );
    expect_manifest_refusal(
        &coordinate,
        "registry.maven-central.coordinate is not canonical",
    );

    let languages = committed_manifest().replace(
        "languages = [\"java\",\"kotlin\"]",
        "languages = [\"kotlin\",\"java\"]",
    );
    expect_manifest_refusal(
        &languages,
        "registry.maven-central.languages must list exactly",
    );

    let module = committed_manifest().replace(
        "module_name = \"com.sidiora.layerx.sdk\"",
        "module_name = \"com.sidiora.layerx.other\"",
    );
    expect_manifest_refusal(
        &module,
        "registry.maven-central.module_name is not canonical",
    );
}

#[test]
fn empty_declaration_is_refused() {
    let source = committed_manifest().replace("artifact = \"nupkg\"", "artifact = \"\"");
    expect_manifest_refusal(&source, "empty declaration registry.nuget.artifact");
}

#[test]
fn tag_format_without_version_placeholder_is_refused() {
    let source = committed_manifest().replace("sdk-v{version}", "sdk-latest");
    expect_manifest_refusal(&source, "tag_format");
}

#[test]
fn declared_registry_without_a_publication_job_is_refused() {
    let workflow = committed_workflow();
    let start = workflow
        .find("\n  publish-nuget:\n")
        .unwrap_or_else(|| panic!("publish-nuget job missing from the workflow"));
    let end = workflow
        .find("\n  release-promotion:\n")
        .unwrap_or_else(|| panic!("release-promotion job missing from the workflow"));
    assert!(start < end, "publish-nuget must precede release-promotion");
    let source = format!("{}{}", &workflow[..start], &workflow[end..]);
    expect_workflow_refusal(
        &source,
        "registry nuget is declared but no publication job carries LAYERX_RELEASE_REGISTRY=nuget",
    );
}

#[test]
fn publication_job_for_an_undeclared_registry_is_refused() {
    let source = format!(
        "{}\n  publish-homebrew:\n    if: {RELEASE_CONDITION}\n    needs: [release-pipeline]\n    runs-on: ubuntu-24.04\n    env:\n      LAYERX_RELEASE_REGISTRY: homebrew\n    steps:\n      - run: brew tap\n",
        committed_workflow()
    );
    expect_workflow_refusal(
        &source,
        "job publish-homebrew publishes LAYERX_RELEASE_REGISTRY=homebrew, which the manifest does not declare",
    );
}

#[test]
fn publication_outside_the_declared_job_is_refused() {
    let source = format!(
        "{}\n  side-channel:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: npm publish --workspace @sidiora/layerx-sdk\n",
        committed_workflow()
    );
    expect_workflow_refusal(
        &source,
        "job side-channel publishes to npm outside the declared publication job",
    );
}

#[test]
fn publishing_fewer_packages_than_declared_is_refused() {
    let source = replaced(
        &committed_workflow(),
        "@sidiora/layerx-next @sidiora/layerx-sdk",
        "@sidiora/layerx-next",
    );
    expect_workflow_refusal(
        &source,
        "job publish-npm publishes LAYERX_RELEASE_PACKAGES=",
    );
}

#[test]
fn explicit_undeclared_package_is_refused() {
    let source = replaced(
        &committed_workflow(),
        "cargo publish --locked --manifest-path services/interop/Cargo.toml -p layerx-mirror",
        "cargo publish --locked --manifest-path services/interop/Cargo.toml -p layerx-ucp",
    );
    expect_workflow_refusal(
        &source,
        "job publish-crates-io publishes layerx-ucp, which the manifest does not declare",
    );
}

#[test]
fn missing_signature_is_refused() {
    let source = committed_workflow().replace("cosign sign-blob", "cosign sign-nothing");
    expect_workflow_refusal(&source, "job publish-crates-io lacks its signature");
}

#[test]
fn missing_install_check_is_refused() {
    let source = committed_workflow().replace("dotnet add package", "dotnet add reference");
    expect_workflow_refusal(
        &source,
        "job publish-nuget lacks its install check from the registry",
    );
}

#[test]
fn publication_job_accepts_the_release_condition_alone() {
    let source = replaced(
        &committed_workflow(),
        &format!(
            "  publish-crates-io:\n    if: >-\n      vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''\n      && ({RELEASE_CONDITION})\n"
        ),
        &format!("  publish-crates-io:\n    if: {RELEASE_CONDITION}\n"),
    );
    let pipeline = release_pipeline(&committed_manifest(), &source).unwrap_or_else(|error| {
        panic!("publication may run under the release condition alone: {error:?}")
    });
    assert!(pipeline
        .publications
        .iter()
        .any(|publication| publication.job == "publish-crates-io"));
}

#[test]
fn publication_job_accepts_configuration_presence_guards() {
    let source = committed_workflow();
    let guarded = format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX != '' && ({RELEASE_CONDITION})");
    let document = workflow::parse(&source)
        .unwrap_or_else(|error| panic!("parse committed workflow: {error:?}"));
    assert_eq!(
        document
            .path(&["jobs", "publish-crates-io", "if"])
            .and_then(workflow::Node::as_str),
        Some(guarded.as_str())
    );
    let pipeline = release_pipeline(&committed_manifest(), &source).unwrap_or_else(|error| {
        panic!("committed publication configuration guard must be accepted: {error:?}")
    });
    for expression in [
        format!("({RELEASE_CONDITION}) && vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''"),
        format!(
            "vars.LAYERX_RELEASE_CRATES_IO_INDEX != '' && ({RELEASE_CONDITION}) && vars.LAYERX_RELEASE_SIGNING_IDENTITY != ''"
        ),
    ] {
        let changed = replaced(
            &source,
            &format!(
                "      vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''\n      && ({RELEASE_CONDITION})"
            ),
            &format!("      {expression}"),
        );
        let changed_pipeline = release_pipeline(&committed_manifest(), &changed)
            .unwrap_or_else(|error| panic!("configuration guard {expression} refused: {error}"));
        assert_eq!(changed_pipeline.publications, pipeline.publications);
    }
}

#[test]
fn publication_job_refuses_guards_other_than_configuration_presence() {
    for expression in [
        format!("github.event_name == 'schedule' && ({RELEASE_CONDITION})"),
        format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX == 'enabled' && ({RELEASE_CONDITION})"),
        format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX != 'disabled' && ({RELEASE_CONDITION})"),
        format!("secrets.LAYERX_RELEASE_CRATES_IO_TOKEN != '' && ({RELEASE_CONDITION})"),
        format!("env.LAYERX_RELEASE_CRATES_IO_INDEX != '' && ({RELEASE_CONDITION})"),
        format!("vars. != '' && ({RELEASE_CONDITION})"),
        format!("vars.1INVALID != '' && ({RELEASE_CONDITION})"),
        format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX.EXTRA != '' && ({RELEASE_CONDITION})"),
        format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX != '' || ({RELEASE_CONDITION})"),
        format!("vars.LAYERX_RELEASE_CRATES_IO_INDEX != '' && {RELEASE_CONDITION}"),
        format!("({RELEASE_CONDITION}) && vars.LAYERX_RELEASE_CRATES_IO_INDEX != '' || true"),
        format!("({RELEASE_CONDITION}) && ({RELEASE_CONDITION})"),
        "vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''".to_owned(),
    ] {
        let source = replaced(
            &committed_workflow(),
            &format!(
                "      vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''\n      && ({RELEASE_CONDITION})"
            ),
            &format!("      {expression}"),
        );
        expect_workflow_refusal(
            &source,
            &format!(
                "job publish-crates-io must run under the release condition `{RELEASE_CONDITION}` alone or as a conjunction of `({RELEASE_CONDITION})` and configuration presence guards `vars.NAME != ''`"
            ),
        );
    }
}

#[test]
fn verification_job_accepts_configuration_presence_disjunctions() {
    let source = committed_workflow();
    let guards =
        "(vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT != '' || vars.LAYERX_TEN_LINE_REGISTRY_URL != '')";
    let guarded = format!("{guards} && ({RELEASE_CONDITION})");
    let document = workflow::parse(&source)
        .unwrap_or_else(|error| panic!("parse committed workflow: {error:?}"));
    assert_eq!(
        document
            .path(&["jobs", "release-verification", "if"])
            .and_then(workflow::Node::as_str),
        Some(guarded.as_str())
    );
    let pipeline = release_pipeline(&committed_manifest(), &source).unwrap_or_else(|error| {
        panic!("committed verification configuration guards must be accepted: {error:?}")
    });
    assert_eq!(pipeline.verification.job, "release-verification");
    for expression in [
        RELEASE_CONDITION.to_owned(),
        format!("vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT != '' && ({RELEASE_CONDITION})"),
        format!("({RELEASE_CONDITION}) && {guards}"),
        format!("{guards} && ({RELEASE_CONDITION}) && vars.LAYERX_RELEASE_SIGNING_IDENTITY != ''"),
    ] {
        let changed = replaced(
            &source,
            &format!("      {guards}\n      && ({RELEASE_CONDITION})"),
            &format!("      {expression}"),
        );
        let changed_pipeline = release_pipeline(&committed_manifest(), &changed)
            .unwrap_or_else(|error| panic!("verification guard {expression} refused: {error}"));
        assert_eq!(changed_pipeline.verification, pipeline.verification);
    }
    let publication = replaced(
        &source,
        &format!("      vars.LAYERX_RELEASE_CRATES_IO_INDEX != ''\n      && ({RELEASE_CONDITION})"),
        &format!("      {guarded}"),
    );
    let changed_pipeline =
        release_pipeline(&committed_manifest(), &publication).unwrap_or_else(|error| {
            panic!("publication must share the verification presence disjunction rule: {error:?}")
        });
    assert_eq!(changed_pipeline.publications, pipeline.publications);
}

#[test]
fn verification_job_refuses_guards_other_than_configuration_presence() {
    let source = committed_workflow();
    for expression in [
        format!("github.event_name == 'schedule' && ({RELEASE_CONDITION})"),
        format!(
            "(vars.SNAPSHOT != '' || github.event_name == 'schedule') && ({RELEASE_CONDITION})"
        ),
        format!("(vars.SNAPSHOT != '' || vars.URL == 'enabled') && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || secrets.URL != '') && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || env.URL != '') && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || vars.1INVALID != '') && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || true) && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' ||) && ({RELEASE_CONDITION})"),
        format!("(|| vars.URL != '') && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || vars.URL != '') || ({RELEASE_CONDITION})"),
        format!("vars.SNAPSHOT != '' || vars.URL != '' && ({RELEASE_CONDITION})"),
        format!("(vars.SNAPSHOT != '' || vars.URL != '') && {RELEASE_CONDITION}"),
        "(vars.SNAPSHOT != '' || vars.URL != '')".to_owned(),
    ] {
        let changed = replaced(
            &source,
            &format!(
                "      (vars.LAYERX_TEN_LINE_REGISTRY_SNAPSHOT != '' || vars.LAYERX_TEN_LINE_REGISTRY_URL != '')\n      && ({RELEASE_CONDITION})"
            ),
            &format!("      {expression}"),
        );
        expect_workflow_refusal(
            &changed,
            &format!(
                "job release-verification must run under the release condition `{RELEASE_CONDITION}` alone or as a conjunction of `({RELEASE_CONDITION})` and configuration presence guards `vars.NAME != ''` or parenthesised disjunctions of those guards `(vars.NAME != '' || vars.OTHER != '')`"
            ),
        );
    }
}

#[test]
fn release_pipeline_job_refuses_configuration_presence_guards() {
    let source = replaced(
        &committed_workflow(),
        &format!("  release-pipeline:\n    if: {RELEASE_CONDITION}\n"),
        &format!("  release-pipeline:\n    if: vars.SNAPSHOT != '' && ({RELEASE_CONDITION})\n"),
    );
    expect_workflow_refusal(
        &source,
        &format!("job release-pipeline must run under the release condition `{RELEASE_CONDITION}`"),
    );
}

#[test]
fn publication_step_gated_on_anything_but_the_tag_is_refused() {
    let source = replaced(
        &committed_workflow(),
        &format!("        if: {TAG_CONDITION}\n"),
        "        if: github.event_name == 'schedule'\n",
    );
    expect_workflow_refusal(&source, "may gate publication steps");
}

#[test]
fn schedule_only_release_gate_is_refused() {
    let source = replaced(
        &committed_workflow(),
        &format!("  programs-acceptance:\n    name: Programs acceptance release gate\n    if: {RELEASE_CONDITION}\n"),
        "  programs-acceptance:\n    name: Programs acceptance release gate\n    if: github.event_name == 'schedule'\n",
    );
    expect_workflow_refusal(
        &source,
        "release gate programs-acceptance is conditional on `github.event_name == 'schedule'`",
    );
}

#[test]
fn release_gate_run_outside_the_gate_runner_is_refused() {
    let source = replaced(
        &committed_workflow(),
        "    uses: ./.github/workflows/agent.yml\n    with:\n      lane: sanitizers",
        "    runs-on: ubuntu-24.04\n    steps:\n      - name: Run sanitizer gate directly\n        run: make agent-test-sanitize",
    );
    expect_workflow_refusal(
        &source,
        "release gate agent-sanitizers must run through scripts/ci/release-gate.sh",
    );
}

#[test]
fn release_without_every_gate_is_refused() {
    let source = replaced(
        &committed_workflow(),
        "needs: [programs-acceptance, agent-sanitizers, agent-fuzz-corpus",
        "needs: [programs-acceptance, agent-sanitizers",
    );
    expect_workflow_refusal(
        &source,
        "job release-pipeline must need the release gate agent-fuzz-corpus",
    );
}

#[test]
fn partial_promotion_is_refused() {
    let source = replaced(
        &committed_workflow(),
        ", publish-nuget, release-verification]",
        ", release-verification]",
    );
    expect_workflow_refusal(
        &source,
        "job release-promotion must need publish-nuget so no partial promotion is presented as a release",
    );
}

#[test]
fn missing_release_tag_trigger_is_refused() {
    let source = replaced(
        &committed_workflow(),
        "tags: [\"sdk-v*\"]",
        "tags: [\"v*\"]",
    );
    expect_workflow_refusal(
        &source,
        "on.push.tags must carry the release tag pattern sdk-v*",
    );
}

#[test]
fn workflow_parser_reads_the_block_yaml_subset() {
    let document = workflow::parse(
        "name: Example\non:\n  push:\n    tags: [\"sdk-v*\"]\njobs:\n  build:\n    runs-on: ubuntu-24.04\n    steps:\n      - name: Run\n        run: |\n          echo one\n          echo two\n      - uses: actions/checkout@abc\n        with:\n          persist-credentials: false\n",
    )
    .unwrap_or_else(|error| panic!("parse: {error}"));
    assert_eq!(
        document.get("name").and_then(workflow::Node::as_str),
        Some("Example")
    );
    assert_eq!(
        document
            .path(&["on", "push", "tags"])
            .map(workflow::Node::strings),
        Some(vec!["sdk-v*"])
    );
    let steps = document
        .path(&["jobs", "build", "steps"])
        .map(workflow::Node::items)
        .unwrap_or_default();
    assert_eq!(steps.len(), 2);
    assert_eq!(
        steps[0].get("run").and_then(workflow::Node::as_str),
        Some("echo one\necho two\n")
    );
    assert_eq!(
        steps[1]
            .path(&["with", "persist-credentials"])
            .and_then(workflow::Node::as_str),
        Some("false")
    );
}

#[test]
fn workflow_parser_refuses_unsupported_yaml() {
    assert!(workflow::parse("jobs:\n\tbuild: x\n").is_err());
    assert!(workflow::parse("defaults: &shared\n  a: b\n").is_err());
    assert!(workflow::parse("jobs:\n  a: 1\n  a: 2\n").is_err());
}
