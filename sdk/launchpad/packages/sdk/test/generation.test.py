"""Reproduce pinned schema/type bytes twice from recovered immutable inputs."""

import hashlib
import json
import pathlib
import tempfile

from authoritative_inputs import GENERATOR, ROOT, copy_bundle, load_module, package_path


V1_OUTPUTS = ("src/types.gen.ts", "src/schemas.gen.json", "release-manifest.json")
V2_OUTPUTS = ("src/types.gen.ts", "src/schemas.gen.json", "release-manifest.json")


def test_clean_generations_match_checked_in_outputs(major, outputs):
    with tempfile.TemporaryDirectory(prefix=f"L24-v{major}-codegen-") as temporary:
        generations = []
        for name in ("clean-a", "clean-b"):
            private = pathlib.Path(temporary) / name
            bundle = copy_bundle(major, private)
            producer = load_module(private / GENERATOR, "sdk_clean_codegen")
            document = json.loads((bundle / "schema-definitions.json").read_bytes())
            for definition, schema in document["$defs"].items():
                producer.validate_schema(schema, document["$defs"], definition)
            generated = producer.Generator(document, major=major, format_output=False).emit()
            package = private / package_path(major)
            release = json.loads((package / "release-manifest.json").read_bytes())
            if major == 1:
                assert hashlib.sha256(producer.frozen_v1_generator()).hexdigest() == release["generatorSha256"]
            else:
                assert release["generatorSha256"] == "939b35c965e0469ace271c9f2ffbf8b96836a901b7866d6ce64cdb80010df35e"
                assert hashlib.sha256((private / GENERATOR).read_bytes()).hexdigest() == "61d7122716d50b1fa318aa2668c248b68e3c474d4e291f4f580c097562803009"
            ts_root = "sdk/launchpad/packages/production-contracts" + ("" if major == 1 else "-v2") + "/src/"
            for filename in ("types.gen.ts", "schemas.gen.json"):
                data = generated[ts_root + filename].encode()
                assert hashlib.sha256(data).hexdigest() == release["outputs"][filename], filename
                (package / "src" / filename).write_bytes(data)
            reproduced = {
                **release,
                "outputs": {
                    filename: hashlib.sha256((package / "src" / filename).read_bytes()).hexdigest()
                    for filename in release["outputs"]
                },
            }
            (package / "release-manifest.json").write_text(json.dumps(reproduced, sort_keys=True, indent=2) + "\n")
            generations.append((generated, {path: (package / path).read_bytes() for path in outputs}))
        assert generations[0] == generations[1], "clean generations differ"
        for path, data in generations[0][1].items():
            assert (ROOT / package_path(major) / path).read_bytes() == data, "checked-in output stale: " + path
        print(json.dumps({"version": major, "cleanGenerations": 2, "identical": True, "outputs": list(outputs)}, sort_keys=True))


def test_versioned_clean_generations():
    test_clean_generations_match_checked_in_outputs(1, V1_OUTPUTS)
    test_clean_generations_match_checked_in_outputs(2, V2_OUTPUTS)


if __name__ == "__main__":
    test_versioned_clean_generations()
