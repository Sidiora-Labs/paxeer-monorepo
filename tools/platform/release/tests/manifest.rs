#[allow(dead_code)]
#[path = "../src/main.rs"]
mod release;

#[path = "support/manifest_fixture.rs"]
mod fixture;

use fixture::*;
use release::{
    artifact_manifest, parse_artifact_manifest, plan, release_pipeline, release_pipeline_verify,
    render_artifact_manifest, ArtifactSource,
};
use std::fs;
use std::process::Command;

#[test]
fn manifest_lists_every_declared_artifact_bound_to_its_source() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("complete", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    assert_eq!(manifest.schema, "layerx/artifact-manifest/1");
    assert_eq!(manifest.version, VERSION);
    assert_eq!(manifest.tag, format!("sdk-v{VERSION}"));
    assert_eq!(manifest.source_revision, REVISION);
    assert_eq!(manifest.source_digest, format!("sha256:{SOURCE_DIGEST}"));
    assert_eq!(manifest.rollback_version.as_deref(), Some(ROLLBACK));
    assert_eq!(manifest.artifacts.len(), ARTIFACT_COUNT);
    for publication in &pipeline.publications {
        for package in &publication.packages {
            assert!(
                manifest
                    .artifacts
                    .iter()
                    .any(|entry| entry.registry == publication.registry && entry.name == *package),
                "manifest lost {package} from {}",
                publication.registry
            );
        }
    }
    for entry in &manifest.artifacts {
        assert_eq!(entry.version, VERSION);
        assert_eq!(entry.source_revision, REVISION);
        assert_eq!(entry.rollback_version.as_deref(), Some(ROLLBACK));
        assert!(entry.digest.starts_with("sha256:") && entry.digest.len() == 71);
        assert!(!entry.signature.is_empty() && !entry.sbom.is_empty());
        assert!(!entry.attestation.is_empty());
        assert!(
            entry.location.starts_with("https://")
                || entry.location.starts_with("simple+https://")
                || entry.location.starts_with("git+https://"),
            "{}@{} has no registry location: {}",
            entry.name,
            entry.version,
            entry.location
        );
        assert!(entry.published && entry.install_check);
        assert!(!entry.artifact.contains('/'));
        assert!(entry
            .retained
            .starts_with(&format!("release-{}/", entry.registry)));
    }
    let names = manifest
        .artifacts
        .iter()
        .map(|entry| (entry.registry.clone(), entry.artifact.clone()))
        .collect::<Vec<_>>();
    let mut unique = names.clone();
    unique.sort();
    unique.dedup();
    assert_eq!(
        unique.len(),
        names.len(),
        "artifact files are not unique per registry"
    );
}

#[test]
fn manifest_round_trips_through_its_json_rendering() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("round-trip", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    let rendered =
        render_artifact_manifest(&manifest).unwrap_or_else(|error| panic!("render: {error}"));
    assert!(rendered.starts_with("{\n  \"schema\": \"layerx/artifact-manifest/1\""));
    let parsed =
        parse_artifact_manifest(&rendered).unwrap_or_else(|error| panic!("parse: {error}"));
    assert_eq!(parsed, manifest);
    let again = render_artifact_manifest(&parsed).unwrap_or_else(|error| panic!("render: {error}"));
    assert_eq!(again, rendered);
}

#[test]
fn matching_published_bytes_pass_verification() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("matching", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    let verified = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .unwrap_or_else(|error| panic!("verification refused matching bytes: {error}"));
    assert_eq!(verified.artifacts.len(), ARTIFACT_COUNT);
    assert!(verified
        .artifacts
        .iter()
        .any(|artifact| artifact.name == "layerx-sdk" && artifact.registry == "crates-io"));
    assert_eq!(verified.images.len(), IMAGE_COUNT);
    assert!(verified.images.iter().all(|image| {
        image
            .reference
            .starts_with(&format!("ghcr.io/sidiora-labs/{}@sha256:", image.name))
    }));
}

#[test]
fn tampered_published_artifact_halts_the_release_naming_it() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("tampered-download", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    write(
        &fixture
            .downloads
            .join("crates-io")
            .join(format!("layerx-sdk-{VERSION}.crate")),
        b"bytes the registry replaced\n",
    );
    let error = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("tampered crate passed verification"));
    assert!(
        error.starts_with(&format!(
            "release halted before promotion: 1 of {ARTIFACT_COUNT} artifacts failed verification"
        )),
        "{error}"
    );
    assert!(
        error.contains(&format!("layerx-sdk@{VERSION} from crates-io (")),
        "{error}"
    );
    assert!(
        error.contains("hash to sha256:") && error.contains("but the manifest lists sha256:"),
        "{error}"
    );
    assert!(
        !error.contains(&format!("layerx-client@{VERSION} from crates-io")),
        "{error}"
    );
}

#[test]
fn missing_published_artifact_halts_the_release_naming_it() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("missing-download", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    let missing = fixture
        .downloads
        .join("nuget")
        .join(format!("LayerX.Sdk.{VERSION}.registry.nupkg"));
    fs::remove_file(&missing)
        .unwrap_or_else(|error| panic!("remove {}: {error}", missing.display()));
    let error = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("missing nupkg passed verification"));
    assert!(
        error.contains(&format!(
            "LayerX.Sdk@{VERSION} from nuget (the downloaded is not present at"
        )),
        "{error}"
    );
}

#[test]
fn manifest_bound_to_another_revision_halts_the_release() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("other-revision", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    let error = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(OTHER_REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("revision mismatch passed verification"));
    assert_eq!(
        error,
        format!(
            "release halted before promotion: the artifact manifest binds revision {REVISION}, not the release revision {OTHER_REVISION}"
        )
    );
}

#[test]
fn unpublished_artifact_halts_the_release() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("unpublished", &pipeline);
    let record = fixture.release_dir.join("release-nuget/publication.txt");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(&record, source.replace("published=true\n", "").as_bytes());
    let manifest = fixture.manifest(&pipeline);
    let nupkg = manifest
        .artifacts
        .iter()
        .find(|entry| entry.registry == "nuget")
        .unwrap_or_else(|| panic!("manifest lost the nuget artifact"));
    assert!(!nupkg.published);
    let error = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("unpublished nupkg passed verification"));
    assert!(
        error.contains(&format!(
            "LayerX.Sdk@{VERSION} from nuget (was not published)"
        )),
        "{error}"
    );
}

#[test]
fn tampered_retained_artifact_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("tampered-retained", &pipeline);
    write(
        &fixture
            .release_dir
            .join("release-npm")
            .join(format!("sidiora-layerx-sdk-{VERSION}.tgz")),
        b"bytes that were not recorded\n",
    );
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("tampered retained tarball produced a manifest"));
    assert!(error.starts_with(&format!("retained artifact @sidiora/layerx-sdk@{VERSION} from npm (sidiora-layerx-sdk-{VERSION}.tgz) hashes to sha256:")), "{error}");
}

#[test]
fn artifact_of_an_undeclared_package_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("undeclared", &pipeline);
    let record = fixture.release_dir.join("release-pypi/artifacts.kvx");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(
        &record,
        source
            .replace("name = \"layerx-fastapi\"", "name = \"layerx-django\"")
            .as_bytes(),
    );
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("undeclared package produced a manifest"));
    assert_eq!(
        error,
        "release-pypi/artifacts.kvx lists layerx-django, which the manifest does not declare for pypi"
    );
}

#[test]
fn publication_from_another_revision_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("split-revision", &pipeline);
    let record = fixture.release_dir.join("release-swiftpm/publication.txt");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(&record, source.replace(REVISION, OTHER_REVISION).as_bytes());
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("split revision produced a manifest"));
    assert_eq!(
        error,
        format!(
            "release-swiftpm/publication.txt records revision {OTHER_REVISION}, but {REVISION} published the other registries"
        )
    );
}

#[test]
fn rollback_identity_cannot_be_the_released_version() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("rollback", &pipeline);
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(VERSION))
        .err()
        .unwrap_or_else(|| panic!("self-rollback produced a manifest"));
    assert_eq!(
        error,
        format!("rollback version {VERSION} is the version being released")
    );
    let first = artifact_manifest(&pipeline, &fixture.release_dir, None)
        .unwrap_or_else(|error| panic!("first release refused: {error}"));
    assert_eq!(first.rollback_version, None);
}

#[test]
fn workflow_without_the_verification_job_is_refused() {
    let source =
        committed_workflow().replace("  release-verification:\n", "  release-inspection:\n");
    let error = release_pipeline(&read("registries.kvx"), &source)
        .err()
        .unwrap_or_else(|| panic!("workflow without release-verification accepted"));
    assert_eq!(error, "release workflow has no job release-verification");
}

#[test]
fn promotion_that_does_not_need_verification_is_refused() {
    let source =
        committed_workflow().replace(", publish-nuget, release-verification]", ", publish-nuget]");
    assert_ne!(source, committed_workflow());
    let error = release_pipeline(&read("registries.kvx"), &source)
        .err()
        .unwrap_or_else(|| panic!("promotion without verification accepted"));
    assert_eq!(
        error,
        "job release-promotion must need release-verification so no unverified artifact is promoted"
    );
}

#[test]
fn verification_without_fetching_from_registries_is_refused() {
    let source = committed_workflow().replace(
        "--fetch build/release/verification",
        "--from build/release/verification",
    );
    assert_ne!(source, committed_workflow());
    let error = release_pipeline(&read("registries.kvx"), &source)
        .err()
        .unwrap_or_else(|| panic!("verification without fetch accepted"));
    assert_eq!(
        error,
        "job release-verification lacks its fetch from every registry: expected `--fetch` in the job"
    );
}

#[test]
fn plan_binds_the_verification_job_and_the_retained_manifest() {
    let pipeline = committed_pipeline();
    let text = plan(&pipeline).unwrap_or_else(|error| panic!("plan: {error}"));
    assert!(text.lines().any(|line| line == "verification_job=release-verification manifest=artifact-manifest.json workflow_artifact=release-artifact-manifest"), "{text}");
}

#[test]
fn manifest_lists_every_declared_image_pinned_by_its_registry_digest() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("images", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    assert_eq!(manifest.images.len(), IMAGE_COUNT);
    for registry in &pipeline.declarations.image_registries {
        let distribution = image_declaration(registry, "distribution");
        assert_eq!(distribution, "ghcr.io/sidiora-labs");
        for image in &registry.images {
            let entry = manifest
                .images
                .iter()
                .find(|entry| entry.name == *image && entry.registry == registry.name)
                .unwrap_or_else(|| panic!("manifest lost the image {image}"));
            assert_eq!(entry.version, VERSION);
            assert_eq!(entry.source_revision, REVISION);
            assert_eq!(entry.rollback_version.as_deref(), Some(ROLLBACK));
            assert_eq!(entry.distribution, distribution);
            assert_eq!(entry.repository, format!("{distribution}/{image}"));
            assert_eq!(
                entry.reference,
                format!("{}@{}", entry.repository, entry.digest)
            );
            assert_eq!(entry.tag, IMAGE_TAG);
            assert_eq!(entry.platform, IMAGE_PLATFORM);
            assert!(entry.digest.starts_with("sha256:") && entry.digest.len() == 71);
            assert!(entry.sbom_digest.starts_with("sha256:") && entry.sbom_digest.len() == 71);
            assert!(
                entry.attestation_digest.starts_with("sha256:")
                    && entry.attestation_digest.len() == 71
            );
            assert_eq!(entry.sbom, format!("sbom/{image}.spdx.json"));
            assert_eq!(
                entry.attestation,
                format!("attestations/{image}.provenance.json")
            );
            assert!(!entry.signature.is_empty());
            assert!(entry.published && entry.pull_check);
        }
    }
    let mut names = manifest
        .images
        .iter()
        .map(|entry| entry.name.clone())
        .collect::<Vec<_>>();
    let sorted = names.clone();
    names.sort();
    names.dedup();
    assert_eq!(names, sorted, "images are neither sorted nor unique");
}

#[test]
fn image_sbom_that_does_not_hash_to_its_record_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-sbom", &pipeline);
    write(
        &fixture
            .release_dir
            .join("release-image-ghcr/sbom/layerx-gateway.spdx.json"),
        b"{\"spdxVersion\": \"SPDX-2.3\", \"name\": \"replaced\"}\n",
    );
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("replaced image SBOM produced a manifest"));
    assert!(
        error.starts_with("retained release-image-ghcr/images.kvx layerx-gateway sbom (sbom/layerx-gateway.spdx.json) hashes to sha256:"),
        "{error}"
    );
    assert!(error.contains("but its record lists sha256:"), "{error}");
}

#[test]
fn image_that_is_not_declared_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-undeclared", &pipeline);
    let record = fixture.release_dir.join("release-image-ghcr/images.kvx");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(
        &record,
        source
            .replace("name = \"layerx-faucet\"", "name = \"layerx-mint\"")
            .as_bytes(),
    );
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("undeclared image produced a manifest"));
    assert_eq!(
        error,
        "release-image-ghcr/images.kvx lists layerx-mint, which the manifest does not declare for ghcr"
    );
}

#[test]
fn declared_image_without_a_record_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-missing", &pipeline);
    let record = fixture.release_dir.join("release-image-ghcr/images.kvx");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    let start = source
        .find("[image.paxd]")
        .unwrap_or_else(|| panic!("fixture lost the paxd image record"));
    let end = source
        .find("[image.paxd-node]")
        .unwrap_or_else(|| panic!("fixture lost the paxd-node image record"));
    write(
        &record,
        format!("{}{}", &source[..start], &source[end..]).as_bytes(),
    );
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("missing image record produced a manifest"));
    assert_eq!(
        error,
        "release-image-ghcr/images.kvx lists no record for the declared image paxd"
    );
}

#[test]
fn image_published_from_another_revision_is_refused_at_emission() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-revision", &pipeline);
    let record = fixture
        .release_dir
        .join("release-image-ghcr/publication.txt");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(&record, source.replace(REVISION, OTHER_REVISION).as_bytes());
    let error = artifact_manifest(&pipeline, &fixture.release_dir, Some(ROLLBACK))
        .err()
        .unwrap_or_else(|| panic!("split image revision produced a manifest"));
    assert_eq!(
        error,
        format!(
            "release-image-ghcr/publication.txt records revision {OTHER_REVISION}, but {REVISION} published the other registries"
        )
    );
}

#[test]
fn unpublished_image_halts_the_release() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-unpublished", &pipeline);
    let record = fixture
        .release_dir
        .join("release-image-ghcr/publication.txt");
    let source = fs::read_to_string(&record)
        .unwrap_or_else(|error| panic!("read {}: {error}", record.display()));
    write(&record, source.replace("published=true\n", "").as_bytes());
    let manifest = fixture.manifest(&pipeline);
    assert!(manifest.images.iter().all(|entry| !entry.published));
    let error = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("unpublished images passed verification"));
    assert!(
        error.contains(&format!(
            "{IMAGE_COUNT} of {IMAGE_COUNT} images failed verification"
        )),
        "{error}"
    );
    assert!(
        error.contains(&format!(
            "layerx-node@{VERSION} from ghcr (was not published)"
        )),
        "{error}"
    );
}

#[test]
fn image_reference_moved_off_its_digest_halts_the_release() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-moved", &pipeline);
    let manifest = fixture.manifest(&pipeline);
    let mut moved = manifest.clone();
    let entry = moved
        .images
        .iter_mut()
        .find(|entry| entry.name == "layerx-core-boundary")
        .unwrap_or_else(|| panic!("manifest lost the core boundary image"));
    entry.reference = format!("{}:{IMAGE_TAG}", entry.repository);
    let error = release_pipeline_verify(
        &moved,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("tag-pinned image reference passed verification"));
    assert!(
        error.contains(&format!("1 of {IMAGE_COUNT} images failed verification")),
        "{error}"
    );
    assert!(
        error.contains(&format!(
            "layerx-core-boundary@{VERSION} from ghcr (artifact manifest image layerx-core-boundary@{VERSION} reference is ghcr.io/sidiora-labs/layerx-core-boundary:{IMAGE_TAG}, expected the digest-pinned ghcr.io/sidiora-labs/layerx-core-boundary@sha256:"
        )),
        "{error}"
    );
    let rendered =
        render_artifact_manifest(&moved).unwrap_or_else(|error| panic!("render: {error}"));
    let refused = parse_artifact_manifest(&rendered)
        .err()
        .unwrap_or_else(|| panic!("tag-pinned image reference parsed"));
    assert!(refused.contains("expected the digest-pinned"), "{refused}");
}

#[test]
fn manifest_without_images_is_refused_by_the_validator() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("image-empty", &pipeline);
    let mut manifest = fixture.manifest(&pipeline);
    manifest.images.clear();
    let rendered =
        render_artifact_manifest(&manifest).unwrap_or_else(|error| panic!("render: {error}"));
    let error = parse_artifact_manifest(&rendered)
        .err()
        .unwrap_or_else(|| panic!("manifest without images parsed"));
    assert_eq!(error, "artifact manifest lists no images");
    let halted = release_pipeline_verify(
        &manifest,
        ArtifactSource::Directory(&fixture.downloads),
        Some(REVISION),
    )
    .err()
    .unwrap_or_else(|| panic!("manifest without images passed verification"));
    assert_eq!(
        halted,
        "release halted before promotion: the artifact manifest lists no images"
    );
}

fn tool() -> Command {
    Command::new(env!("CARGO_BIN_EXE_layerx-platform-release"))
}

#[test]
fn command_line_emits_the_manifest_and_verifies_downloaded_artifacts() {
    let pipeline = committed_pipeline();
    let fixture = Fixture::build("command-line", &pipeline);
    let output = fixture.release_dir.join("artifact-manifest.json");
    let emitted = tool()
        .arg("manifest")
        .arg("--release-dir")
        .arg(&fixture.release_dir)
        .arg("--output")
        .arg(&output)
        .args(["--rollback-version", "none", "--registries"])
        .arg(registries_path())
        .arg("--workflow")
        .arg(workflow_path())
        .output()
        .unwrap_or_else(|error| panic!("run manifest: {error}"));
    assert!(
        emitted.status.success(),
        "{}",
        String::from_utf8_lossy(&emitted.stderr)
    );
    let stdout = String::from_utf8_lossy(&emitted.stdout);
    assert!(stdout.contains(&format!("artifact manifest: {ARTIFACT_COUNT} artifacts and {IMAGE_COUNT} images of {VERSION} (sdk-v{VERSION}) bound to revision {REVISION}")), "{stdout}");
    let manifest = parse_artifact_manifest(
        &fs::read_to_string(&output)
            .unwrap_or_else(|error| panic!("read {}: {error}", output.display())),
    )
    .unwrap_or_else(|error| panic!("emitted manifest refused: {error}"));
    assert_eq!(manifest.rollback_version, None);
    let verified = tool()
        .arg("verify")
        .arg("--manifest")
        .arg(&output)
        .arg("--from")
        .arg(&fixture.downloads)
        .args(["--source-revision", REVISION])
        .output()
        .unwrap_or_else(|error| panic!("run verify: {error}"));
    assert!(
        verified.status.success(),
        "{}",
        String::from_utf8_lossy(&verified.stderr)
    );
    let stdout = String::from_utf8_lossy(&verified.stdout);
    assert!(
        stdout.contains(&format!(
            "verified LayerXSDK@{VERSION} from swiftpm sha256:"
        )),
        "{stdout}"
    );
    assert!(stdout.contains(&format!("release verification: {ARTIFACT_COUNT} artifacts of {VERSION} match the manifest bound to revision {REVISION}")), "{stdout}");
    assert!(
        stdout.contains(&format!(
            "pinned layerx-gateway@{VERSION} from ghcr ghcr.io/sidiora-labs/layerx-gateway@sha256:"
        )),
        "{stdout}"
    );
    assert!(stdout.contains(&format!("release verification: {IMAGE_COUNT} images of {VERSION} are pinned by their registry digest in the manifest bound to revision {REVISION}")), "{stdout}");
    write(
        &fixture
            .downloads
            .join("maven-central")
            .join(format!("layerx-sdk-{VERSION}.pom")),
        b"<project>tampered</project>\n",
    );
    let halted = tool()
        .arg("verify")
        .arg("--manifest")
        .arg(&output)
        .arg("--from")
        .arg(&fixture.downloads)
        .args(["--source-revision", REVISION])
        .output()
        .unwrap_or_else(|error| panic!("run verify: {error}"));
    assert!(!halted.status.success());
    let stderr = String::from_utf8_lossy(&halted.stderr);
    assert!(stderr.contains(&format!("release halted before promotion: 1 of {ARTIFACT_COUNT} artifacts failed verification: com.sidiora.layerx:layerx-sdk@{VERSION} from maven-central (the downloaded layerx-sdk-{VERSION}.pom hash to sha256:")), "{stderr}");
    let both = tool()
        .arg("verify")
        .arg("--manifest")
        .arg(&output)
        .arg("--from")
        .arg(&fixture.downloads)
        .arg("--fetch")
        .arg(&fixture.downloads)
        .output()
        .unwrap_or_else(|error| panic!("run verify: {error}"));
    assert!(!both.status.success());
    assert!(String::from_utf8_lossy(&both.stderr)
        .contains("verify needs exactly one of --from or --fetch"));
}
