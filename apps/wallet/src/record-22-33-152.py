"""Record the real service fixture once; never called by the wallet gate."""

import json
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import tomllib


def inline(value):
    if isinstance(value, dict):
        return "{ " + ", ".join(f"{key} = {inline(item)}" for key, item in value.items()) + " }"
    if isinstance(value, list):
        return "[" + ", ".join(inline(item) for item in value) + "]"
    return json.dumps(value)


def write_provenance(source):
    root = source.parents[2]
    output = source / "hosted-intent-22-33-152.recording.json"
    recording = json.loads(output.read_text())
    paths = [
        "services/human/crates/layerx-human-service/tests/support/production_fixture.rs",
        "services/human/crates/layerx-human-service/src/server/production_components.rs",
        "services/human/crates/layerx-human-service/src/server/http.rs",
        "services/human/crates/layerx-human-service/src/store/mod.rs",
    ]
    recording["provenance"] = {
        "revision": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(),
        "command": "python3 apps/wallet/src/record-22-33-152.py",
        "sources": {path: hashlib.sha256((root / path).read_bytes()).hexdigest() for path in paths},
    }
    output.write_text(json.dumps(recording, indent=2) + "\n")


def main():
    source = Path(__file__).resolve().parent
    root = source.parents[2]
    service = root / "services/human/crates/layerx-human-service"
    workspace = service.parents[1]
    manifest = tomllib.loads((service / "Cargo.toml").read_text())
    shared = tomllib.loads((workspace / "Cargo.toml").read_text())["workspace"]["dependencies"]
    dependencies = {"layerx-human-service": {"path": str(service)}}
    for section in ("dependencies", "dev-dependencies"):
        for name, original in manifest[section].items():
            value = dict(original) if isinstance(original, dict) else original
            base = service
            if isinstance(value, dict) and value.pop("workspace", False):
                inherited = shared[name]
                value = ({"version": inherited} if isinstance(inherited, str) else dict(inherited)) | value
                base = workspace
            if isinstance(value, dict) and "path" in value:
                value["path"] = str((base / value["path"]).resolve())
            dependencies[name] = value
    with tempfile.TemporaryDirectory(prefix="wallet-intent-recording-") as temporary:
        projection = Path(temporary)
        build = projection / "services/human/crates/layerx-human-service"
        build.mkdir(parents=True)
        # The unmodified fixture resolves its real Go attestor from this layout.
        (projection / "services/wallet").symlink_to(root / "services/wallet", target_is_directory=True)
        cargo = build / "Cargo.toml"
        cargo.write_text(
            '[package]\nname = "wallet-intent-recording"\nversion = "0.0.0"\nedition = "2021"\n'
            + '\n[workspace]\n\n[[bin]]\nname = "wallet-intent-recording"\npath = '
            + json.dumps(str(source / "record-22-33-152.rs"))
            + "\n\n[dependencies]\n"
            + "\n".join(f"{name} = {inline(value)}" for name, value in dependencies.items())
            + "\n"
        )
        environment = dict(os.environ)
        environment.setdefault("CARGO_TARGET_DIR", "/tmp/wallet-intent-recording-target")
        subprocess.run(
            ["cargo", "run", "--manifest-path", str(cargo), "--bin", "wallet-intent-recording", "--",
             str(source / "hosted-intent-22-33-152.recording.json")],
            env=environment, check=True,
        )
    write_provenance(source)


if __name__ == "__main__":
    main()
