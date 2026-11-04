"""Offline schema-root and real manifest-reader regression proof.

The schema specimen below is deliberately a contract specimen, not a recorded
physical-device release. No invented signing/OS/browser observations are used
to obtain a successful validate_release result.
"""

import ast
import copy
import hashlib
import importlib.util
import itertools
import json
from pathlib import Path

from jsonschema import Draft202012Validator
import pytest


ROOT = Path(__file__).resolve().parents[3]
MOBILE = ROOT / "apps/launchpad/qualification/mobile"
SCHEMA_PATH = MOBILE / "evidence-manifest.schema.json"
HARNESS_PATH = MOBILE / "release_harness.py"
RECORD_PATH = ROOT / "tools/qualification/audit_repairs/repair_22_33_1022.kvx"
SPEC = importlib.util.spec_from_file_location("mobile_repair_22_33_1022", HARNESS_PATH)
HARNESS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HARNESS)
PLATFORMS = (
    "ios", "android", "chrome", "firefox", "safari", "android-chrome", "ios-safari"
)
JOURNEYS = (
    "discovery", "search", "token-reading", "wallet-owned-execution",
    "portfolio-reconciliation", "profile-edit", "watchlist", "chat", "live", "push"
)
WIDTHS = (320, 375, 390, 768, 1024, 1440)


@pytest.fixture
def validator():
    schema = HARNESS.load(SCHEMA_PATH)
    Draft202012Validator.check_schema(schema)
    return Draft202012Validator(schema)


@pytest.fixture
def current_source_capture(tmp_path):
    """Produce actual hashed source bytes and JSON using the production writer."""
    capture = HARNESS.Capture(tmp_path / "capture")
    source_files = [str(path.relative_to(ROOT)) for path in (HARNESS_PATH, SCHEMA_PATH)]
    source_digest = HARNESS.source_digest(ROOT, source_files)
    artifact = capture.write("current-release-harness-source.py", HARNESS_PATH.read_bytes())
    artifacts = {platform: artifact["sha256"] for platform in PLATFORMS}
    candidate = {
        "environment": "staging",
        "syntheticAccountsOnly": True,
        "signingOwner": "schema-contract-test-only",
        "sourceRoot": str(ROOT),
        "sourceFiles": source_files,
        "sourceDigest": source_digest,
        "backendRevision": "source-contract-test-only",
        "runId": "measure_schema_contract_1022",
        "releaseArtifacts": artifacts,
    }
    bundle = {
        "schemaVersion": 4,
        "environment": "staging",
        "syntheticAccountsOnly": True,
        "releaseSetDigest": hashlib.sha256(
            json.dumps(artifacts, sort_keys=True, separators=(",", ":")).encode()
        ).hexdigest(),
        "sourceDigest": source_digest,
        "backendRevision": candidate["backendRevision"],
        "runId": candidate["runId"],
        "walletSupport": True,
        "artifacts": {platform: {"artifact": artifact} for platform in PLATFORMS},
        "journeys": [],
        "responsive": [],
        "manualAccessibility": [],
        "axeReports": [],
    }
    return capture, candidate, bundle


def schema_specimen(bundle):
    """Describe the v4 JSON shape without fabricating a native capture bundle.

    References intentionally name absent observations. JSON Schema validates
    their shape; the production harness must reject their absent bytes.
    """
    result = copy.deepcopy(bundle)
    observation = {"path": "not-captured.json", "sha256": "0" * 64}
    result["journeys"] = [
        {
            "platform": platform,
            "journey": journey,
            "artifactDigest": bundle["artifacts"][platform]["artifact"]["sha256"],
            "backendRevision": bundle["backendRevision"],
            "expected": "contract specimen only",
            "observed": "contract specimen only",
            "passed": True,
            "reviewer": "contract specimen only",
            **{key: observation for key in (
                "networkTrace", "authoritativeState", "screenshot", "focusTrace"
            )},
        }
        for platform, journey in itertools.product(PLATFORMS, JOURNEYS)
    ]
    result["responsive"] = [
        {
            "platform": platform, "width": width, "zoom": zoom,
            "orientation": direction, "geometryTrace": observation,
            "screenshot": observation, "settingsTrace": observation,
        }
        for platform, width, zoom, direction in itertools.product(
            PLATFORMS[2:], WIDTHS, (1, 2), ("portrait", "landscape")
        )
    ]
    result["manualAccessibility"] = [
        {
            "technology": technology, "journey": journey, "passed": True,
            "criticalDefects": 0, "reviewer": "contract specimen only",
            "recording": observation, "focusTrace": observation,
        }
        for technology, journey in itertools.product(
            ("NVDA+Chrome", "VoiceOver+Safari"), JOURNEYS
        )
    ]
    result["axeReports"] = [observation]
    return result


def test_mobile_v4_schema_root_is_the_advertised_contract(validator):
    schema = validator.schema
    assert schema["$schema"] == "https://json-schema.org/draft/2020-12/schema"
    assert schema["$ref"] == "#/$defs/releaseManifestV4"
    assert set(schema) == {"$schema", "$ref", "$defs"}
    # Preserve every pre-existing definition constraint while replacing only
    # the obsolete root. This digest is from the task's inspected base file.
    assert hashlib.sha256(
        json.dumps(schema["$defs"], sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest() == "458819c06aeccde622fa22c285a7e7f33fb678e4b55684bb4fa2b49449171d98"
    release = schema["$defs"]["releaseManifestV4"]
    assert release["additionalProperties"] is False
    assert set(release["required"]) == set(release["properties"]) == {
        "schemaVersion", "environment", "syntheticAccountsOnly", "releaseSetDigest",
        "sourceDigest", "backendRevision", "runId", "walletSupport", "artifacts",
        "journeys", "responsive", "manualAccessibility", "axeReports",
    }
    assert release["properties"]["schemaVersion"] == {"const": 4}
    assert release["properties"]["environment"] == {"const": "staging"}
    assert release["properties"]["syntheticAccountsOnly"] == {"const": True}
    assert release["properties"]["walletSupport"] == {"const": True}
    artifacts = release["properties"]["artifacts"]
    assert set(artifacts["required"]) == set(artifacts["properties"]) == set(PLATFORMS)
    assert artifacts["additionalProperties"] is False
    assert HARNESS.PLATFORMS == PLATFORMS
    assert HARNESS.JOURNEYS == JOURNEYS
    assert HARNESS.WIDTHS == WIDTHS
    assert release["properties"]["journeys"]["minItems"] == 70
    assert release["properties"]["responsive"]["minItems"] == 120
    assert release["properties"]["manualAccessibility"]["minItems"] == 20


def test_v4_contract_shape_survives_real_capture_transport(validator, current_source_capture):
    capture, candidate, bundle = current_source_capture
    specimen = schema_specimen(bundle)
    encoded = capture.write("v4-contract-specimen.json", specimen)
    transported = HARNESS.load(HARNESS.reference(capture.root, encoded))
    assert transported == specimen
    assert validator.is_valid(transported), list(validator.iter_errors(transported))
    assert transported["sourceDigest"] == HARNESS.source_digest(ROOT, candidate["sourceFiles"])
    for platform in PLATFORMS:
        assert HARNESS.digest(HARNESS.reference(capture.root, transported["artifacts"][platform]["artifact"])) == candidate["releaseArtifacts"][platform]
    # Reaching this required native-signature read proves that the unchanged
    # real consumer accepted the version/source/release-set binding prefix.
    # It must not qualify source files as signed native releases.
    with pytest.raises(KeyError, match="signature"):
        HARNESS.validate_release(capture.root, transported, candidate)


@pytest.mark.parametrize("field,value", [
    ("schemaVersion", 3), ("environment", "production"),
    ("syntheticAccountsOnly", False), ("walletSupport", False),
])
def test_schema_and_harness_reject_wrong_release_contract(
    validator, current_source_capture, field, value
):
    capture, candidate, bundle = current_source_capture
    specimen = schema_specimen(bundle)
    specimen[field] = value
    encoded = capture.write("invalid-v4-manifest.json", specimen)
    transported = HARNESS.load(HARNESS.reference(capture.root, encoded))
    assert not validator.is_valid(transported)
    with pytest.raises(ValueError, match=(
        "Actual supported wallet path" if field == "walletSupport"
        else "Observed release bundle v4"
    )):
        HARNESS.validate_release(capture.root, transported, candidate)


@pytest.mark.parametrize("field", ["sourceDigest", "releaseSetDigest", "backendRevision", "runId"])
def test_real_harness_rejects_other_source_release_backend_or_run(
    validator, current_source_capture, field
):
    capture, candidate, bundle = current_source_capture
    specimen = schema_specimen(bundle)
    specimen[field] = (
        "f" * 64 if field.endswith("Digest") else
        "measure_another_release_run" if field == "runId" else "other-backend-revision"
    )
    assert validator.is_valid(specimen)
    encoded = capture.write("rebound-manifest.json", specimen)
    with pytest.raises(ValueError, match="Current platform candidates/backend/run binding mismatch"):
        HARNESS.validate_release(
            capture.root, HARNESS.load(HARNESS.reference(capture.root, encoded)), candidate
        )


def test_legacy_root_and_unapproved_fields_are_rejected(validator, current_source_capture):
    _, _, bundle = current_source_capture
    legacy = {
        "artifactDigest": "a" * 64, "backendRevision": "old-backend",
        "platform": "ios", "signed": True, "physicalDevice": True, "journeys": [],
    }
    assert not validator.is_valid(legacy)
    specimen = schema_specimen(bundle)
    for field in ("artifactDigest", "platform", "signed", "physicalDevice", "unknown"):
        invalid = copy.deepcopy(specimen)
        invalid[field] = legacy.get(field, True)
        assert not validator.is_valid(invalid), field
    for platform in PLATFORMS:
        invalid = copy.deepcopy(specimen)
        del invalid["artifacts"][platform]
        assert not validator.is_valid(invalid), platform


@pytest.mark.parametrize("reference", [
    {"path": "/outside", "sha256": "a" * 64},
    {"path": "../outside", "sha256": "a" * 64},
    {"path": "capture", "sha256": "invalid"},
    {"path": "capture", "sha256": "a" * 64, "unsigned": True},
])
def test_schema_and_real_reference_reader_reject_invalid_artifact_references(
    validator, current_source_capture, reference
):
    capture, _, bundle = current_source_capture
    specimen = schema_specimen(bundle)
    specimen["artifacts"]["ios"]["artifact"] = reference
    assert not validator.is_valid(specimen)
    with pytest.raises(ValueError):
        HARNESS.reference(capture.root, reference)


def test_harness_reads_the_same_v4_root_fields_and_keeps_release_checks(validator):
    tree = ast.parse(HARNESS_PATH.read_text())
    validate = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "validate_release")
    bundle_fields = {
        node.slice.value for node in ast.walk(validate)
        if isinstance(node, ast.Subscript) and isinstance(node.value, ast.Name)
        and node.value.id == "bundle" and isinstance(node.slice, ast.Constant)
    }
    assert bundle_fields == set(validator.schema["$defs"]["releaseManifestV4"]["required"])
    calls = {
        node.func.id for node in ast.walk(validate)
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
    }
    assert {"require", "source_digest", "reference", "digest", "load",
            "qualify_measurements", "authoritative_financial", "finite"} <= calls
    main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "main")
    analyze_call = next(
        node for node in ast.walk(main) if isinstance(node, ast.Call)
        and isinstance(node.func, ast.Name) and node.func.id == "validate_release"
    )
    assert ast.unparse(analyze_call) == "validate_release(args.bundle.parent, load(args.bundle), plan)"


def test_repair_record_maps_proofs_and_records_full_release_limit():
    record = RECORD_PATH.read_text()
    assert "[finding.mobile-v4-schema-root]" in record
    assert "test_mobile_v4_schema_root_is_the_advertised_contract" in record
    assert "test_v4_contract_shape_survives_real_capture_transport" in record
    assert "not proven" in record
    assert "owner_domain = 4" in record
    assert "No baseline" in record
    assert "/out/run/gates.jsonl" in record
