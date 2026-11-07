"""Exercise the real emitter and read-only CLI without a Go toolchain."""

import copy
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    "codegen", pathlib.Path(__file__).with_name("codegen.py")
)
cg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cg)

DOC = {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$defs": {
        "Amount": {"type": "string", "pattern": "^(0|[1-9][0-9]*)$"},
        "Payload": {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "amount": {"$ref": "#/$defs/Amount"},
                "note": {"type": ["string", "null"]},
            },
            "required": ["amount", "note"],
        },
        "Proof": {
            "oneOf": [
                {
                    "type": "object",
                    "additionalProperties": False,
                    "properties": {
                        "kind": {"const": "eoa", "type": "string"},
                        "signature": {
                            "type": "string", "minLength": 132, "maxLength": 132,
                        },
                    },
                    "required": ["kind", "signature"],
                },
                {
                    "type": "object",
                    "additionalProperties": False,
                    "properties": {
                        "kind": {"const": "contract", "type": "string"},
                        "signature": {"type": "string", "maxLength": 8194},
                    },
                    "required": ["kind", "signature"],
                },
            ],
        },
    },
}

GO_TYPES = "sdk/launchpad/shared/interfaces/v1/types.gen.go"
TS_TYPES = "sdk/launchpad/packages/production-contracts/src/types.gen.ts"


def seed_frozen_input(root):
    """Bind the tiny schema's real producer output to an immutable CLI input."""
    source = root / "sdk/launchpad/protocol/contracts/v1/schema-definitions.json"
    source.parent.mkdir(parents=True)
    source.write_text(json.dumps(DOC))
    outputs = cg.Generator(copy.deepcopy(DOC), format_output=False).emit()
    manifest = source.with_name("codegen-manifest.json")
    manifest.write_text(json.dumps({
        "formatVersion": 1,
        "schemaSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "outputs": {
            name: hashlib.sha256(content.encode()).hexdigest()
            for name, content in outputs.items()
        },
    }, sort_keys=True, indent=2) + "\n")
    return source, outputs


def file_bytes(root):
    return {
        path.relative_to(root): path.read_bytes()
        for path in root.rglob("*") if path.is_file()
    }


class CodegenTests(unittest.TestCase):
    def test_deterministic_and_concrete(self):
        a = cg.Generator(copy.deepcopy(DOC), format_output=False).emit()
        b = cg.Generator(copy.deepcopy(DOC), format_output=False).emit()
        self.assertEqual(a, b)
        self.assertNotIn("interface{}", a[GO_TYPES])
        self.assertNotIn(" any", a[TS_TYPES])
        self.assertIn("Variant1", a[GO_TYPES])
        self.assertIn(" | null", a[TS_TYPES])

    def test_check_never_writes_and_detects_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            source, outputs = seed_frozen_input(root)
            argv = ["--root", str(root), "--schema", str(source),
                    "--manifest", str(source.with_name("codegen-manifest.json")), "--no-format"]
            self.assertEqual(cg.main(argv), 0)
            for name, content in outputs.items():
                self.assertEqual((root / name).read_bytes(), content.encode())
            before = file_bytes(root)
            self.assertEqual(cg.main(argv + ["--check"]), 0)
            self.assertEqual(before, file_bytes(root))
            target = root / GO_TYPES
            stale = b"stale\r\n"
            target.write_bytes(stale)
            drifted = file_bytes(root)
            self.assertEqual(cg.main(argv + ["--check"]), 1)
            self.assertEqual(target.read_bytes(), stale)
            self.assertEqual(drifted, file_bytes(root))

    def test_unresolved_or_generic_or_unsupported_fails(self):
        for invalid in (
            {"$ref": "#/$defs/Missing"},
            {"type": "object", "additionalProperties": True},
            {"type": "string", "pattern": "^(?=a)a$"},
            {"type": "array"},
            {"type": "string", "inventedConstraint": 1},
        ):
            with self.subTest(invalid=invalid), self.assertRaises(cg.GenerationError):
                cg.validate_schema(invalid, DOC["$defs"], "test")

    def test_absent_schema_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            source = pathlib.Path(directory) / "reviewed/v1/schema-definitions.json"
            self.assertEqual(cg.main([
                "--root", directory, "--schema", str(source),
                "--manifest", str(source.with_name("codegen-manifest.json")),
                "--check", "--no-format",
            ]), 1)

    def test_frozen_manifest_mismatch_is_rejected_without_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            source, _ = seed_frozen_input(root)
            changed = copy.deepcopy(DOC)
            changed["$defs"]["Amount"]["maxLength"] = 78
            source.write_text(json.dumps(changed))
            before = file_bytes(root)
            self.assertEqual(cg.main([
                "--root", str(root), "--schema", str(source),
                "--manifest", str(source.with_name("codegen-manifest.json")), "--no-format",
            ]), 1)
            self.assertEqual(before, file_bytes(root))

    def test_versioned_output_cannot_overwrite_v1(self):
        for option in (
            {"go_out": "sdk/launchpad/shared/interfaces/v1"},
            {"ts_out": "sdk/launchpad/packages/production-contracts/src"},
        ):
            with self.subTest(option=option), self.assertRaisesRegex(
                cg.GenerationError, "cannot overwrite v1"
            ):
                cg.Generator(copy.deepcopy(DOC), major=2, format_output=False, **option)


if __name__ == "__main__":
    unittest.main()
