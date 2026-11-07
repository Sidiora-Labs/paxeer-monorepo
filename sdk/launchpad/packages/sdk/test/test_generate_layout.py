import pathlib
import tempfile
import unittest

from authoritative_inputs import (
    GENERATOR, OWNER, ROOT, copy_bundle, load_module, package_path, schema_input,
)


path = ROOT / "sdk/launchpad/packages/sdk/scripts/generate.py"
generate = load_module(path, "sdk_generate")


def test_repository_root_is_monorepo():
    root = generate.repository_root()
    assert root / "sdk/launchpad/packages/sdk/scripts/generate.py" == path
    assert (root / "spec/workflow.kvx").is_file()
    assert (root / GENERATOR).is_file()


def test_default_contracts_use_launchpad_spec_prefix():
    for major in (1, 2):
        selected = generate.default_contracts(major)
        assert selected == ROOT / OWNER / f"v{major}"
        assert generate.expected_package(major) == ROOT / package_path(major)
        # Missing owner manifests are an explicit production error, not proof
        # of a usable bundle. The available schema input is independently pinned.
        assert schema_input(major)
        with unittest.TestCase().assertRaisesRegex(ValueError, "Missing codegen manifest:.*complete immutable"):
            generate.read_manifest(selected)
        with tempfile.TemporaryDirectory() as directory:
            private = pathlib.Path(directory)
            bundle = copy_bundle(major, private)
            copied = private / path.relative_to(ROOT)
            copied.parent.mkdir(parents=True, exist_ok=True)
            copied.write_bytes(path.read_bytes())
            consumer = load_module(copied, "sdk_private_generate")
            assert consumer.default_contracts(major) == bundle
            assert (bundle / "schema-definitions.json").read_bytes() == schema_input(major)
            with unittest.TestCase().assertRaisesRegex(ValueError, "Missing codegen manifest"):
                consumer.read_manifest(consumer.default_contracts(major))


def test_unsupported_major_is_explicitly_refused():
    producer = load_module(ROOT / GENERATOR, "sdk_layout_codegen")
    for major in (0, -1, 3, 99):
        with unittest.TestCase().assertRaisesRegex(ValueError, "Unsupported schema major"):
            schema_input(major)
        if major < 1:
            with unittest.TestCase().assertRaisesRegex(ValueError, "positive schema major required"):
                producer.Generator({"$defs": {}}, major=major, format_output=False)
        else:
            with unittest.TestCase().assertRaisesRegex(ValueError, "Missing codegen manifest"):
                generate.read_manifest(generate.default_contracts(major))
