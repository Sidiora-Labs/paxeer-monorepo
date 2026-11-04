"""Negative qualification of the real frozen register, without provider calls."""
import argparse
import copy
import importlib.util
import os
import shutil
import tempfile
from pathlib import Path
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
SPEC = ROOT / "spec/layerx-merge/spec.kvx"
BUNDLE = ROOT / "sdk/launchpad/protocol/contracts/v1"
CATALOG = None
CHAIN_BUNDLE = None
EVIDENCE = None
NOT_BEFORE = None
CONTRACTS = ROOT / "contracts/launchpad"
module_spec = importlib.util.spec_from_file_location("capability_check", HERE / "capability_check.py")
check = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(check)


def bind_inputs(bundle, catalog, chain_bundle, evidence, not_before, spec=SPEC):
    global BUNDLE, CATALOG, CHAIN_BUNDLE, EVIDENCE, NOT_BEFORE, SPEC
    BUNDLE = Path(bundle).resolve()
    CATALOG = Path(catalog).resolve()
    CHAIN_BUNDLE = Path(chain_bundle).resolve()
    EVIDENCE = Path(evidence).resolve()
    NOT_BEFORE = check.epoch(not_before)
    SPEC = Path(spec).resolve()


class CapabilityTests(unittest.TestCase):
    def setUp(self):
        if any(value is None for value in (CATALOG, CHAIN_BUNDLE, EVIDENCE, NOT_BEFORE)):
            raise ValueError("explicit bundle, catalog, chain bundle and fresh evidence bindings required")
        self.document = check.load(BUNDLE / "capabilities.json")
        self.spec = SPEC
        self.authorities = {
            "workspace": ROOT,
            "contracts": CONTRACTS,
            "evidence": EVIDENCE,
        }
        self.not_before = NOT_BEFORE

    def errors(self, document=None, materialized=False):
        return check.validate(
            self.document if document is None else document,
            ROOT, self.spec, self.authorities, materialized,
            not_before=self.not_before, bundle=BUNDLE, chain_bundle=CHAIN_BUNDLE,
            catalog_input=CATALOG,
        )

    def test_real_source_register_consistency(self):
        self.assertEqual(self.errors(), [])

    def test_external_wallet_cannot_be_marked_ready(self):
        document = copy.deepcopy(self.document)
        next(c for c in document["capabilities"] if c["id"] == "external_wallet")["state"] = "ready"
        self.assertTrue(any("ready without" in e for e in self.errors(document)))

    def test_absent_deployment_cannot_be_marked_ready(self):
        document = copy.deepcopy(self.document)
        cap = next(c for c in document["capabilities"] if c["id"] == "conditional_orders")
        cap["state"] = "ready"
        self.assertTrue(any("optional deployment absent" in e for e in self.errors(document)))

    def test_waiving_required_feature_fails(self):
        document = copy.deepcopy(self.document)
        document["capabilities"][0]["requiredForRelease"] = False
        self.assertTrue(self.errors(document))

    def test_stale_source_and_unknown_catalog_fail(self):
        document = copy.deepcopy(self.document)
        document["sources"][0]["sha256"] = "0" * 64
        document["capabilities"][0]["endpointRefs"].append("IF_INVENTED")
        errors = self.errors(document)
        self.assertTrue(any("stale source hash" in e for e in errors))
        self.assertTrue(any("invalid endpoint" in e for e in errors))

    def test_missing_required_artifact_fails(self):
        document = copy.deepcopy(self.document)
        document["requiredArtifacts"].append("spec/production-system/integration/contracts/v1/nonexistent.json")
        self.assertTrue(any("missing required frozen artifact" in e for e in self.errors(document, True)))

    def test_source_traversal_fails(self):
        document = copy.deepcopy(self.document)
        document["sources"][0]["path"] = "../../outside"
        self.assertTrue(any("escapes authority" in e for e in self.errors(document)))

    def test_empty_json_runtime_proof_fails(self):
        self.assertTrue(check.proof_errors({}, "provider", "external_wallet", "digest", 1))

    def test_embedded_provider_cannot_return_as_core_requirement(self):
        document = copy.deepcopy(self.document)
        document["provider"] = {"id": "hosted_signer"}
        self.assertTrue(any("superseded embedded provider" in e for e in self.errors(document)))

    def test_unconfigured_walletconnect_cannot_be_mandatory(self):
        document = copy.deepcopy(self.document)
        document["wallet"]["walletConnect"]["requiredForCore"] = True
        self.assertTrue(any("WalletConnect cannot be" in e for e in self.errors(document)))

    def test_source_only_external_wallet_ready_is_rejected(self):
        document = copy.deepcopy(self.document)
        cap = next(c for c in document["capabilities"] if c["id"] == "external_wallet")
        cap["state"] = "ready"
        for pre in cap["prerequisites"]:
            pre["state"] = "verified"
            pre["sourceRefs"] = ["core-manifest"]
        cap["qualificationEvidence"] = {"outcome": "passed", "statusCodesOnly": False, "sourceRefs": ["core-manifest"]}
        errors = self.errors(document)
        self.assertTrue(any("runtime prerequisite missing fresh proof file" in e for e in errors))
        self.assertTrue(any("source-only qualification evidence forbidden" in e for e in errors))

    def test_runtime_evidence_file_predates_bound_run(self):
        document = copy.deepcopy(self.document)
        cap = next(c for c in document["capabilities"] if c["id"] == "external_wallet")
        prerequisite = next(p for p in cap["prerequisites"] if p["kind"] == "controller")
        prerequisite["state"] = "verified"
        prerequisite["sourceRefs"] = ["core-manifest"]
        with tempfile.TemporaryDirectory(dir=EVIDENCE) as directory:
            proof = Path(directory) / "source-only.json"
            # A real source register is deliberately inadmissible as runtime
            # proof; its file timestamp must be checked independently as well.
            shutil.copyfile(BUNDLE / "capabilities.json", proof)
            stale = self.not_before - 1
            os.utime(proof, (stale, stale))
            prerequisite["proofPath"] = str(proof.relative_to(EVIDENCE))
            errors = self.errors(document)
            self.assertTrue(any("runtime proof file predates run" in e for e in errors))
            self.assertTrue(any("runtime proof kind/scope mismatch" in e for e in errors))

    def test_misstated_scope_and_501_are_not_proof(self):
        proof = {"kind": "source", "capabilityId": "external_wallet", "outcome": "passed", "exitCode": 0, "checks": {"HTTP501": True}, "statusCodesOnly": True}
        self.assertTrue(check.proof_errors(proof, "provider", "external_wallet", "digest", 1))

    def test_empty_materialized_catalog_rejected(self):
        schemas = {ref for cap in self.document["capabilities"] for ref in cap["schemaRefs"]}
        endpoints = {ref for cap in self.document["capabilities"] for ref in cap["endpointRefs"]}
        self.assertTrue(check.catalog_errors({}, {}, {}, schemas, endpoints))

    def test_actual_materialized_bundle_maps_catalog_not_defs_count(self):
        bundle = BUNDLE
        definitions = check.load(bundle / "schema-definitions.json")["$defs"]
        catalog = check.load(bundle / "catalog-map.json")
        api = check.load(bundle / "openapi.json")
        schemas = {ref for cap in self.document["capabilities"] for ref in cap["schemaRefs"]}
        endpoints = {ref for cap in self.document["capabilities"] for ref in cap["endpointRefs"]}
        self.assertEqual(check.catalog_errors(definitions, catalog, api, schemas, endpoints), [])
        corrupt = copy.deepcopy(catalog)
        corrupt["schemas"][next(iter(corrupt["schemas"]))]["wireName"] = "AbsentDefinition"
        self.assertTrue(check.catalog_errors(definitions, corrupt, api, schemas, endpoints))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec", type=Path, default=SPEC)
    parser.add_argument("--bundle", type=Path, required=True)
    parser.add_argument("--catalog", type=Path, required=True)
    parser.add_argument("--chain-bundle", type=Path, required=True)
    parser.add_argument("--evidence-root", type=Path, required=True)
    parser.add_argument("--not-before", required=True)
    args = parser.parse_args(argv)
    bind_inputs(args.bundle, args.catalog, args.chain_bundle, args.evidence_root,
                args.not_before, spec=args.spec)
    result = unittest.TextTestRunner().run(unittest.defaultTestLoader.loadTestsFromTestCase(CapabilityTests))
    return int(not result.wasSuccessful())


if __name__ == "__main__":
    raise SystemExit(main())
