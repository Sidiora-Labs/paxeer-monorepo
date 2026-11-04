"""Generate with the manifest's pinned Docker image and retain input provenance."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[1]


def generation_inputs(root, image, schema_revision):
    config = json.loads((root / "openapitools.json").read_text())
    version = config["generator-cli"]["version"]
    if image != f"openapitools/openapi-generator-cli:v{version}":
        raise ValueError("Docker image must match openapitools.json")
    if not re.fullmatch(r"[0-9a-f]{40}", schema_revision):
        raise ValueError("input schema backend revision must be a full Git SHA")
    schema = root / "swaggers/blockscout-api-final.yaml"
    return {
        "generator_image": image,
        "generator_version": version,
        "input_schema_revision": schema_revision,
        "input_schema_sha256": hashlib.sha256(schema.read_bytes()).hexdigest(),
    }


def qualify_generation(inputs, resolved_image, reported_version):
    if reported_version.strip() != inputs["generator_version"]:
        raise ValueError("Docker executable reports a different generator version")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", resolved_image):
        raise ValueError("Docker must resolve the generator to an immutable image ID")
    return {**inputs, "resolved_generator": resolved_image}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image")
    parser.add_argument("schema_revision", help="full source backend Git revision")
    args = parser.parse_args()
    inputs = generation_inputs(ROOT, args.image, args.schema_revision)
    # Pull the exact tag, then execute its immutable ID for both version and generation.
    subprocess.run(["docker", "pull", args.image], check=True)
    resolved = subprocess.check_output(
        ["docker", "image", "inspect", "--format", "{{.Id}}", args.image], text=True
    ).strip()
    version = subprocess.check_output(
        ["docker", "run", "--rm", resolved, "version"], text=True
    )
    provenance = qualify_generation(inputs, resolved, version)
    subprocess.run(
        [
            "docker", "run", "--rm", "-v", f"{ROOT}:/local", resolved, "generate",
            "-c", "/local/openapi-generator-config.json",
            "-i", "/local/swaggers/blockscout-api-final.yaml",
            "--skip-validate-spec", "-g", "rust", "-o", "/local/crate",
            "--engine", "handlebars", "-t", "/local/rust-template-handlebars",
            "--type-mappings=integer=i64",
        ],
        check=True,
    )
    if inputs != generation_inputs(ROOT, args.image, args.schema_revision):
        raise RuntimeError("input schema changed during generation")
    (ROOT / "crate/generation-provenance.json").write_text(
        json.dumps(provenance, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
