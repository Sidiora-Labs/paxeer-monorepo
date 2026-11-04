#[path = "go_fixture.rs"]
pub(super) mod go_fixture;

use std::fmt::Write as _;
use std::fs;
use std::path::{Path, PathBuf};

use crate::release::{
    artifact_manifest, plan, release_pipeline, sha256_digest, ArtifactManifest, ImageRegistry,
    PublicationJob, ReleasePipeline,
};

pub(super) const VERSION: &str = "0.1.0";
pub(super) const ROLLBACK: &str = "0.0.9";
pub(super) const REVISION: &str = "0123456789abcdef0123456789abcdef01234567";
pub(super) const OTHER_REVISION: &str = "fedcba9876543210fedcba9876543210fedcba98";
pub(super) const SOURCE_DIGEST: &str =
    "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08";
pub(super) const ARTIFACT_COUNT: usize = 27;
pub(super) const IMAGE_COUNT: usize = 18;
pub(super) const IMAGE_TAG: &str = "sdk-v0.1.0";
pub(super) const IMAGE_PLATFORM: &str = "linux/amd64";

pub(super) fn read(relative: &str) -> String {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(relative);
    fs::read_to_string(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()))
}

pub(super) fn registries_path() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("registries.kvx")
}

pub(super) fn workflow_path() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../.github/workflows/platform.yml")
}

pub(super) fn committed_workflow() -> String {
    read("../../../.github/workflows/platform.yml")
}

pub(super) fn committed_pipeline() -> ReleasePipeline {
    release_pipeline(&read("registries.kvx"), &committed_workflow())
        .unwrap_or_else(|error| panic!("committed release pipeline refused: {error}"))
}

pub(super) fn write(path: &Path, bytes: &[u8]) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .unwrap_or_else(|error| panic!("create {}: {error}", parent.display()));
    }
    fs::write(path, bytes).unwrap_or_else(|error| panic!("write {}: {error}", path.display()));
}

struct Record {
    name: String,
    file: String,
    digest_of: &'static str,
    signature: String,
    sbom: String,
    attestation: String,
    location: String,
}

fn crate_record(package: &str) -> Record {
    let file = format!("{package}-{VERSION}.crate");
    let sbom = match package {
        "layerx-mirror" | "layerx-programs-runtime" => format!("{package}.sbom.spdx.json"),
        _ => "agent.sbom.spdx.json".to_owned(),
    };
    Record {
        name: package.to_owned(),
        signature: format!("{file}.sigstore.json"),
        sbom,
        attestation: "provenance.sigstore.json".to_owned(),
        location: format!("https://crates.io/api/v1/crates/{package}/{VERSION}/download"),
        file,
        digest_of: "built-bytes",
    }
}

fn npm_record(package: &str) -> Record {
    let bare = package.trim_start_matches("@sidiora/");
    let file = format!("sidiora-{bare}-{VERSION}.tgz");
    Record {
        name: package.to_owned(),
        signature: format!("{file}.sigstore.json"),
        sbom: format!("_sidiora-{bare}.sbom.spdx.json"),
        attestation: format!(
            "npm-provenance:https://registry.npmjs.org/-/npm/v1/attestations/%40sidiora%2F{bare}@{VERSION}"
        ),
        location: format!("https://registry.npmjs.org/{package}/-/{bare}-{VERSION}.tgz"),
        file,
        digest_of: "built-bytes",
    }
}

fn pypi_record(package: &str) -> Record {
    let wheel = format!("{}-{VERSION}-py3-none-any.whl", package.replace('-', "_"));
    Record {
        name: package.to_owned(),
        file: format!("dist/{wheel}"),
        digest_of: "built-bytes",
        signature: format!("signatures/{wheel}.sigstore.json"),
        sbom: format!("{package}.sbom.spdx.json"),
        attestation: format!(
            "pypi-integrity:https://pypi.org/integrity/{package}/{VERSION}/{wheel}/provenance"
        ),
        location: format!("simple+https://pypi.org/simple/{package}/"),
    }
}

fn go_record(package: &str) -> Record {
    let fixture = go_fixture::GoFixture::load(VERSION);
    assert_eq!(package, fixture.module);
    Record {
        name: package.to_owned(),
        file: fixture
            .location
            .rsplit('/')
            .next()
            .expect("published Go archive filename")
            .to_owned(),
        digest_of: "registry-bytes",
        signature: "tag-signature.txt".to_owned(),
        sbom: "layerx-go.sbom.spdx.json".to_owned(),
        attestation: format!(
            "go-checksum-database:sum.golang.org {package} v{VERSION} h1:fixture="
        ),
        location: fixture.location,
    }
}

fn maven_records(package: &str) -> Vec<Record> {
    let (group, name) = package
        .split_once(':')
        .unwrap_or_else(|| panic!("maven coordinate {package} has no group"));
    let directory = format!("{}/{name}/{VERSION}", group.replace('.', "/"));
    ["jar", "pom"]
        .into_iter()
        .map(|extension| {
            let file = format!("staging/{directory}/{name}-{VERSION}.{extension}");
            Record {
                name: package.to_owned(),
                signature: format!("{file}.asc"),
                sbom: format!("{name}.sbom.spdx.json"),
                attestation: "provenance.sigstore.json".to_owned(),
                location: format!(
                    "https://repo1.maven.org/maven2/{directory}/{name}-{VERSION}.{extension}"
                ),
                file,
                digest_of: "built-bytes",
            }
        })
        .collect()
}

fn swift_record(package: &str) -> Record {
    Record {
        name: package.to_owned(),
        file: format!("LayerXSDK-{VERSION}.tar"),
        digest_of: "built-bytes",
        signature: "tag-signature.txt".to_owned(),
        sbom: "LayerXSDK.sbom.spdx.json".to_owned(),
        attestation: "provenance.sigstore.json".to_owned(),
        location: format!("git+https://github.com/Sidiora-Labs/LayerXSDK#{VERSION}"),
    }
}

fn nuget_record(package: &str) -> Record {
    Record {
        name: package.to_owned(),
        file: format!("LayerX.Sdk.{VERSION}.registry.nupkg"),
        digest_of: "registry-bytes",
        signature: "repository-signature.txt".to_owned(),
        sbom: "LayerX.Sdk.sbom.spdx.json".to_owned(),
        attestation: "provenance.sigstore.json".to_owned(),
        location: format!(
            "https://api.nuget.org/v3-flatcontainer/layerx.sdk/{VERSION}/layerx.sdk.{VERSION}.nupkg"
        ),
    }
}

fn records(registry: &str, packages: &[String]) -> Vec<Record> {
    packages
        .iter()
        .flat_map(|package| match registry {
            "crates-io" => vec![crate_record(package)],
            "npm" => vec![npm_record(package)],
            "pypi" => vec![pypi_record(package)],
            "go-modules" => vec![go_record(package)],
            "maven-central" => maven_records(package),
            "swiftpm" => vec![swift_record(package)],
            "nuget" => vec![nuget_record(package)],
            other => panic!("no fixture records for registry {other}"),
        })
        .collect()
}

pub(super) struct Fixture {
    pub(super) release_dir: PathBuf,
    pub(super) downloads: PathBuf,
}

fn write_publication(pipeline: &ReleasePipeline, publication: &PublicationJob, bundle: &Path) {
    let registry = pipeline
        .declarations
        .registries
        .iter()
        .find(|registry| registry.name == publication.registry)
        .unwrap_or_else(|| panic!("registry {} undeclared", publication.registry));
    let distribution = registry
        .declarations
        .iter()
        .find(|(key, _)| key == "distribution")
        .map_or_else(
            || panic!("registry {} has no distribution", registry.name),
            |(_, value)| value.clone(),
        );
    write(
        &bundle.join("publication.txt"),
        format!(
            "registry={}\nversion={VERSION}\ndistribution={distribution}\ntarget={distribution}\npackages={}\nrevision={REVISION}\npublished=true\ninstall_check=pass\n",
            publication.registry,
            publication.packages.join(" ")
        )
        .as_bytes(),
    );
}

pub(super) fn image_declaration(registry: &ImageRegistry, key: &str) -> String {
    registry
        .declarations
        .iter()
        .find(|(declared, _)| declared == key)
        .map_or_else(
            || panic!("image registry {} has no {key}", registry.name),
            |(_, value)| value.clone(),
        )
}

struct ImageRecord {
    name: String,
    digest: String,
    signature: String,
    sbom: String,
    sbom_digest: String,
    attestation: String,
    attestation_digest: String,
}

fn image_records(registry: &ImageRegistry, bundle: &Path) -> Vec<ImageRecord> {
    registry
        .images
        .iter()
        .map(|name| {
            let signature = format!("signatures/{name}.sigstore.json");
            let sbom = format!("sbom/{name}.spdx.json");
            let attestation = format!("attestations/{name}.provenance.json");
            let sbom_bytes = format!("{{\"spdxVersion\": \"SPDX-2.3\", \"name\": \"{name}\"}}\n");
            let attestation_bytes =
                format!("{{\"predicateType\": \"slsaprovenance\", \"image\": \"{name}\"}}\n");
            write(
                &bundle.join(&signature),
                format!("signature of {name}\n").as_bytes(),
            );
            write(&bundle.join(&sbom), sbom_bytes.as_bytes());
            write(&bundle.join(&attestation), attestation_bytes.as_bytes());
            ImageRecord {
                digest: sha256_digest(format!("{name} image manifest\n").as_bytes()),
                signature,
                sbom,
                sbom_digest: sha256_digest(sbom_bytes.as_bytes()),
                attestation,
                attestation_digest: sha256_digest(attestation_bytes.as_bytes()),
                name: name.clone(),
            }
        })
        .collect()
}

fn write_image_bundle(registry: &ImageRegistry, release_dir: &Path) {
    let bundle = release_dir.join(format!("release-image-{}", registry.name));
    let distribution = image_declaration(registry, "distribution");
    write(
        &bundle.join("publication.txt"),
        format!(
            "registry={}\nversion={VERSION}\ndistribution={distribution}\nimages={}\nrevision={REVISION}\npublished=true\npull_check=pass\n",
            registry.name,
            registry.images.join(" ")
        )
        .as_bytes(),
    );
    let mut images = String::new();
    for record in image_records(registry, &bundle) {
        write!(
            images,
            "[image.{}]\nname = \"{}\"\ntag = \"{IMAGE_TAG}\"\ndigest = \"{}\"\nplatform = \"{IMAGE_PLATFORM}\"\nsignature = \"{}\"\nsbom = \"{}\"\nsbom_digest = \"{}\"\nattestation = \"{}\"\nattestation_digest = \"{}\"\n\n",
            record.name,
            record.name,
            record.digest,
            record.signature,
            record.sbom,
            record.sbom_digest,
            record.attestation,
            record.attestation_digest
        )
        .unwrap_or_else(|error| panic!("write image record: {error}"));
    }
    write(&bundle.join("images.kvx"), images.as_bytes());
}

impl Fixture {
    pub(super) fn build(name: &str, pipeline: &ReleasePipeline) -> Self {
        let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join("artifact-manifest")
            .join(name);
        if root.exists() {
            fs::remove_dir_all(&root)
                .unwrap_or_else(|error| panic!("remove {}: {error}", root.display()));
        }
        let release_dir = root.join("release");
        let downloads = root.join("downloads");
        write(
            &release_dir.join("release-pipeline/version.txt"),
            format!("version={VERSION}\n").as_bytes(),
        );
        write(
            &release_dir.join("release-pipeline/source-digest.txt"),
            format!("{SOURCE_DIGEST}  -\n").as_bytes(),
        );
        write(
            &release_dir.join("release-pipeline/release-plan.txt"),
            plan(pipeline)
                .unwrap_or_else(|error| panic!("plan: {error}"))
                .as_bytes(),
        );
        for publication in &pipeline.publications {
            let bundle = release_dir.join(format!("release-{}", publication.registry));
            write_publication(pipeline, publication, &bundle);
            let mut artifacts = String::new();
            for (index, record) in records(&publication.registry, &publication.packages)
                .iter()
                .enumerate()
            {
                let bytes = if publication.registry == "go-modules" {
                    go_fixture::ARCHIVE.to_vec()
                } else {
                    format!(
                        "{} {} {} published bytes\n",
                        publication.registry, record.name, record.file
                    )
                    .into_bytes()
                };
                let path = bundle.join(&record.file);
                write(&path, &bytes);
                let basename = Path::new(&record.file)
                    .file_name()
                    .and_then(|name| name.to_str())
                    .unwrap_or_else(|| panic!("record file {} has no name", record.file));
                write(
                    &downloads.join(&publication.registry).join(basename),
                    &bytes,
                );
                write(
                    &bundle.join(&record.signature),
                    format!("signature of {}\n", record.file).as_bytes(),
                );
                write(
                    &bundle.join(&record.sbom),
                    b"{\"spdxVersion\": \"SPDX-2.3\"}\n",
                );
                if !record.attestation.contains(':') {
                    write(
                        &bundle.join(&record.attestation),
                        b"{\"provenance\": true}\n",
                    );
                }
                write!(
                    artifacts,
                    "[artifact.{}]\nname = \"{}\"\nversion = \"{VERSION}\"\nfile = \"{}\"\ndigest = \"{}\"\ndigest_of = \"{}\"\nsignature = \"{}\"\nsbom = \"{}\"\nattestation = \"{}\"\nlocation = \"{}\"\n\n",
                    index + 1,
                    record.name,
                    record.file,
                    sha256_digest(&bytes),
                    record.digest_of,
                    record.signature,
                    record.sbom,
                    record.attestation,
                    record.location
                )
                .unwrap_or_else(|error| panic!("write artifact record: {error}"));
            }
            write(&bundle.join("artifacts.kvx"), artifacts.as_bytes());
        }
        for registry in &pipeline.declarations.image_registries {
            write_image_bundle(registry, &release_dir);
        }
        Self {
            release_dir,
            downloads,
        }
    }

    pub(super) fn manifest(&self, pipeline: &ReleasePipeline) -> ArtifactManifest {
        artifact_manifest(pipeline, &self.release_dir, Some(ROLLBACK))
            .unwrap_or_else(|error| panic!("artifact manifest refused: {error}"))
    }
}
