"""File-transport regressions for the two production control-plane findings.

Real materializer/Graph/Store producers emit the JSON and SQLite consumed by
the qualifier and serial gate. The committed peer is explicitly isolated;
neither it nor the selected schema exercise is production migration approval.
No native check is launched, replaced, or reported as qualified by this suite.
"""

import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tomllib
import unittest

import pytest
from jsonschema import Draft202012Validator, ValidationError

import reviewed_peer

SOURCE = Path(__file__).resolve().parents[1]
ROOT = SOURCE.parents[2]
FIXTURES = SOURCE / "tests/fixtures/repair_22_33_973"
INTERFACE_PEERS = SOURCE / "interfaces/testdata/repair_22_33_963"


def module(name, path):
    loader = importlib.util.spec_from_file_location(name, path)
    loaded = importlib.util.module_from_spec(loader)
    sys.modules[name] = loaded
    loader.loader.exec_module(loaded)
    return loaded


qualifier = module("repair_973_qualifier", SOURCE / "qualify.py")
serial = module("repair_973_serial", SOURCE / "serial_gate.py")
control = qualifier._control
materializer = module("repair_973_materializer", SOURCE / "interfaces/materialize.py")
examples = module("repair_973_examples", SOURCE / "interfaces/examples_check.py")
graph_module = module("repair_973_graph", SOURCE / "dispatcher/graph.py")
legacy = module("repair_973_exact_reviews", SOURCE / "tests/test_serial_gate.py")
prelude_approval = module("repair_973_prelude_approval", SOURCE / "approve_serial_prelude.py")


@pytest.fixture(scope="module")
def recording(tmp_path_factory):
    root = tmp_path_factory.mktemp("recorded-control-plane")
    spec = root / "reviewed/production-system/spec.kvx"
    spec.parent.mkdir(parents=True)
    shutil.copyfile(FIXTURES / "peer.kvx", spec)
    graph = graph_module.Graph(spec)
    document = tomllib.loads(spec.read_text())
    plan = spec.with_name("execution-plan.json")
    reviewed_peer.project_plan(plan, graph, document, spec)
    bundle = root / "reviewed/contracts/v1"
    materializer.main([
        "--spec", str(INTERFACE_PEERS / "selected-catalog.kvx"),
        "--supplementary", str(INTERFACE_PEERS / "supplementary.json"),
        "--out", str(bundle), "--frozen-v1-out", str(bundle),
    ])
    reviewed_peer.install_review(
        root, spec, plan, graph, control,
        catalog=bundle / "inputs/catalog.json", manifest=bundle / "catalog-map.json",
    )
    return root


@pytest.fixture
def peer(recording, tmp_path):
    root = tmp_path / "source"
    shutil.copytree(recording, root)
    return root


def registry(root):
    return json.loads((root / control.REGISTRY_PATH).read_bytes())


def save_review(root, document):
    review = document["control_plane"]
    review["approval"]["bindings_sha256"] = control.digest(review["bindings"])
    reviewed_peer.write_json(root / control.REGISTRY_PATH, document)


def selected(root, *, catalog=True):
    return control.reviewed_graph(root, require_catalog=catalog)


def serial_policy(root, reviewed):
    return {
        "task": "1.99", "wave": 1,
        "input_interface_sha256": reviewed["retained"]["sha256"],
        "output_interface_sha256": reviewed["retained"]["sha256"],
        "accepted_ids": {f"1.{i:02}": f"recorded-L{i:02}" for i in range(1, 51)},
        # This deliberate refusal boundary proves both real gates reach the
        # ownership/acceptance checks without needing private production state.
        "state_directory": str(root / "forbidden-in-source-state"),
        "control_plane": {
            "spec_sha256": reviewed["bindings"]["spec"]["sha256"],
            "plan_sha256": reviewed["bindings"]["plan"]["sha256"],
            "registry_without_self_sha256": serial.digest(registry(root)),
        },
    }


def call_serial(root, reviewed, policy, output, version):
    if version == 4:
        policy["serial_integration"] = {"source_policy": {"version": 4}}
    entry = {"serial_gate_policy": policy, "policy_sha256": serial.digest(policy)}
    return serial.qualify_serial_gate(
        entry, "1.99", reviewed["graph"].tasks["1.99"], reviewed["document"], root, output
    )


def test_qualification_missing_inputs_retained_inventory_is_authentic():
    actual = registry(ROOT)
    observed = control.retained_inventory(ROOT, actual)
    assert set(observed["files"]) == set(control.RETAINED_FILES)
    assert observed["sha256"] == control.digest(observed["files"])
    abi = json.loads((ROOT / control.FIXTURE_ROOT / "chain/abi-catalog.json").read_bytes())
    assert abi["contracts"]
    assert str(abi["chainId"]) == "125"
    assert actual["control_plane"]["status"] == "awaiting-owner-review"
    with pytest.raises(ValueError, match="requires owning domain 4 review"):
        control.reviewed_inputs(ROOT, actual, require_catalog=True)


def test_qualification_missing_inputs_real_materializer_transport(peer):
    reviewed = selected(peer)
    source = tomllib.loads((INTERFACE_PEERS / "selected-catalog.kvx").read_text())
    assert reviewed["semantic_input"]["sha256"] == materializer.semantic_digest(source)
    assert reviewed["semantic_input"]["catalog_manifest_sha256"] == reviewed_peer.sha(reviewed["catalog_manifest"])
    assert reviewed["graph"].tasks == graph_module.Graph(reviewed["spec"], reviewed["plan"]).tasks
    assert len(reviewed["graph"].tasks) == 208
    assert len(reviewed["document"]["lane"]) == 50
    definition = json.loads(reviewed["catalog_manifest"].with_name("schema-definitions.json").read_bytes())
    request = definition["$defs"]["StreamHeartbeatRequest"]
    assert set(request["required"]) == {"viewerToken", "playbackSessionId", "streamId"}
    validator = Draft202012Validator({**definition, "$ref": "#/$defs/StreamHeartbeatRequest"})
    payload = examples.Samples(definition).sample({"$ref": "#/$defs/StreamHeartbeatRequest"})
    validator.validate(json.loads(json.dumps(payload)))
    with pytest.raises(ValidationError):
        validator.validate({**payload, "unknownField": True})


@pytest.mark.parametrize("change", ["approval", "missing_digest", "spec", "plan", "catalog", "catalog_manifest", "fixture", "runner", "dispatcher"])
def test_qualification_missing_inputs_rejects_unreviewed_bytes(peer, change):
    document = registry(peer)
    bindings = document["control_plane"]["bindings"]
    if change == "approval":
        bindings["layout_sha256"] = "0" * 64
        reviewed_peer.write_json(peer / control.REGISTRY_PATH, document)
    elif change == "missing_digest":
        bindings["spec"]["sha256"] = None
        save_review(peer, document)
    else:
        paths = {
            "fixture": next(iter(document["retained_inputs"]["files"])),
            "runner": control.RUNNER_PATHS[0],
            "dispatcher": control.DISPATCHER_PATH + "/graph.py",
        }
        path = peer / (paths[change] if change in paths else bindings[change]["path"])
        path.write_bytes(path.read_bytes() + b"\n# unreviewed drift\n")
    with pytest.raises(ValueError, match="approval|digest differs"):
        selected(peer)


def test_qualification_missing_inputs_are_pinned_after_preflight(peer):
    reviewed = selected(peer)
    control.verify_inputs(peer, reviewed)
    reviewed["plan"].write_bytes(reviewed["plan"].read_bytes() + b"\n")
    with pytest.raises(ValueError, match="digest differs"):
        control.verify_inputs(peer, reviewed)


@pytest.mark.parametrize("change", ["empty_catalog", "semantic_manifest", "projection", "ownership"])
def test_qualification_missing_inputs_cannot_approve_invalid_authority(peer, change):
    document = registry(peer)
    bindings = document["control_plane"]["bindings"]
    name = "catalog" if change == "empty_catalog" else "catalog_manifest" if change == "semantic_manifest" else "plan"
    path = peer / bindings[name]["path"]
    value = json.loads(path.read_bytes())
    if change == "empty_catalog":
        value["sections"]["endpoint"] = {}
    elif change == "semantic_manifest":
        value["inputs"]["catalogSha256"] = "0" * 64
    elif change == "projection":
        value["tasks"]["1.01"]["touches"] = ["foreign-domain/**"]
    else:
        value["lanes"][0]["write_paths"] = ["foreign-domain/**"]
    reviewed_peer.write_json(path, value)
    bindings[name] = reviewed_peer.identity(peer, path)
    save_review(peer, document)
    with pytest.raises(ValueError, match="nonempty|semantic identity|projection"):
        selected(peer)


@pytest.mark.parametrize("path_kind", ["alias", "traversal", "link", "hardlink"])
def test_qualification_missing_inputs_preserves_canonical_source_checks(peer, path_kind):
    reviewed = selected(peer)
    alias = peer / "unreviewed.kvx"
    shutil.copyfile(reviewed["spec"], alias)
    if path_kind == "alias":
        with pytest.raises(ValueError, match="canonical reviewed specification"):
            control.reviewed_graph(peer, spec_path=alias)
    else:
        document = registry(peer)
        if path_kind == "traversal":
            document["control_plane"]["bindings"]["spec"]["path"] = "reviewed/../unreviewed.kvx"
        elif path_kind == "link":
            reviewed["spec"].unlink()
            reviewed["spec"].symlink_to(alias)
        else:
            alias.unlink()
            os.link(reviewed["spec"], alias)
        save_review(peer, document)
        with pytest.raises(ValueError, match="unsafe"):
            selected(peer)


def test_qualification_missing_inputs_real_cli_reports_unapproved_migration(peer):
    document = registry(peer)
    document["control_plane"]["status"] = "awaiting-owner-review"
    document["control_plane"]["approval"] = None
    reviewed_peer.write_json(peer / control.REGISTRY_PATH, document)
    args = qualifier.argument_parser().parse_args([
        "--task", "0.2", "--spec", "reviewed/production-system/spec.kvx", "--require-current-source",
    ])
    assert qualifier.run(args, root=peer) == 1
    reports = list((peer / "launchpad/evidence").rglob("result.json"))
    assert len(reports) == 1
    result = json.loads(reports[0].read_bytes())
    assert result["qualified"] is False and result["status_mutated"] is False
    assert result["checks"] == []
    assert "requires owning domain 4 review" in result["errors"][0]
    args.registry = Path("unreviewed-registry.json")
    shutil.copyfile(peer / control.REGISTRY_PATH, peer / args.registry)
    assert qualifier.run(args, root=peer) == 1
    results = [json.loads(path.read_bytes()) for path in (peer / "launchpad/evidence").rglob("result.json")]
    assert any("canonical reviewed registry" in str(result["errors"]) for result in results)


def test_qualification_missing_inputs_reviewed_bundle_and_registry_arguments(peer):
    reviewed = selected(peer)
    bundle = {
        "inputs": ["{interface_root}/inputs/catalog.json"],
        "outputs": ["{interface_root}/catalog-map.json", *[control.FIXTURE_ROOT + "/" + name for name in control.FIXTURE_FILES]],
    }
    expected = {
        "inputs": {str(reviewed["catalog"].relative_to(peer)): reviewed_peer.sha(reviewed["catalog"])},
        "outputs": {
            str(path.relative_to(peer)): reviewed_peer.sha(path)
            for path in (reviewed["catalog_manifest"], *[peer / control.FIXTURE_ROOT / name for name in control.FIXTURE_FILES])
        },
    }
    bundle["expected_sha256"] = qualifier.canonical_hash(expected)
    inventory = qualifier.interface_bundle(bundle, peer, reviewed=reviewed)
    assert inventory == {**expected, "sha256": bundle["expected_sha256"]}
    actual = registry(ROOT)
    checks = actual["tasks"]["0.2"]["checks"]
    by_id = {check["id"]: check for check in checks}
    assert "--catalog" in by_id["interfaces-materialized-catalog"]["argv"]
    assert "--frozen-v1-out" in by_id["interfaces-materialized-catalog"]["argv"]
    assert "--manifest" in by_id["interfaces-deterministic-codegen"]["argv"]
    assert "--chain-bundle" in by_id["interfaces-capability-prerequisites"]["argv"]
    assert "--register" in by_id["interfaces-postgres-privileges"]["argv"]
    assert "sdk/launchpad/protocol/contracts/v1/accounting-cases.json" in by_id["interfaces-accounting"]["argv"]
    dispatcher = actual["tasks"]["0.3"]["checks"][0]
    document = {**actual, "tasks": {"0.3": {"checks": [dispatcher], "interface_bundle": bundle}}}
    normalized = qualifier.validate_registry(document, "0.3", peer)
    assert len(normalized) == 1
    argv = [control.bind(value, reviewed, peer) for value in normalized[0]["argv"]]
    assert argv[argv.index("--spec") + 1] == str(reviewed["spec"].relative_to(peer))
    assert argv[argv.index("--plan") + 1] == str(reviewed["plan"].relative_to(peer))
    document["tasks"]["0.3"]["checks"][0] = copy.deepcopy(dispatcher)
    document["tasks"]["0.3"]["checks"][0]["argv"][3] = "unreviewed.kvx"
    with pytest.raises(ValueError, match="actual registered full dispatcher harness"):
        qualifier.validate_registry(document, "0.3", peer)
    (peer / control.FIXTURE_ROOT / "accounting-cases.json").write_bytes(b"{}")
    with pytest.raises(ValueError, match="digest stale"):
        qualifier.interface_bundle(bundle, peer, reviewed=reviewed)


@pytest.mark.parametrize("version", [1, 4])
def test_serial_gate_layout_both_paths_reach_exact_fifty_boundary(peer, tmp_path, version):
    reviewed = selected(peer)
    package = reviewed["dispatcher"]
    assert Path(package.graph.__file__) == peer / control.DISPATCHER_PATH / "graph.py"
    assert package.graph.Graph is not graph_module.Graph
    policy = serial_policy(peer, reviewed)
    with pytest.raises(ValueError, match="state must be external"):
        call_serial(peer, reviewed, policy, tmp_path / "proof", version)
    caller = copy.deepcopy(reviewed["document"])
    caller["task"]["1"]["99"]["title"] = "unreviewed caller"
    with pytest.raises(ValueError, match="caller specification differs"):
        serial.canonical_control_plane(peer, caller, "1.99", reviewed["graph"].tasks["1.99"], policy)
    policy["control_plane"]["plan_sha256"] = "0" * 64
    with pytest.raises(ValueError, match="control plane differs"):
        call_serial(peer, reviewed, policy, tmp_path / "bad-proof", version)


@pytest.mark.parametrize("version", [1, 4])
@pytest.mark.parametrize("change", ["missing", "duplicate", "traversal", "unfinished", "duplicate_lane"])
def test_serial_gate_layout_preserves_fifty_lane_checks(peer, tmp_path, version, change):
    reviewed = selected(peer)
    if change in {"unfinished", "duplicate_lane"}:
        path = reviewed["spec"]
        previous = path.read_text()
        if change == "unfinished":
            changed = previous.replace('status = "done"\nrequires = ["0.3"]', 'status = "implemented"\nrequires = ["0.3"]', 1)
        else:
            changed = re.sub(
                r'(?ms)^\[task\.1\.50\]\n.*?(?=^\[|\Z)',
                lambda match: match.group().replace('lane = "L50"', 'lane = "L01"').replace('handoffs/L50/', 'handoffs/L01/'),
                previous,
            )
        assert changed != previous
        path.write_text(changed)
        graph = graph_module.Graph(path)
        document = tomllib.loads(path.read_text())
        reviewed_peer.project_plan(reviewed["plan"], graph, document, path)
        document = registry(peer)
        bindings = document["control_plane"]["bindings"]
        bindings["spec"] = reviewed_peer.identity(peer, path)
        bindings["plan"] = reviewed_peer.identity(peer, reviewed["plan"])
        bindings["layout_sha256"] = graph.layout_digest
        save_review(peer, document)
        reviewed = selected(peer)
    policy = serial_policy(peer, reviewed)
    if change == "missing":
        policy["accepted_ids"].pop("1.50")
    elif change == "duplicate":
        policy["accepted_ids"]["1.50"] = policy["accepted_ids"]["1.01"]
    elif change == "traversal":
        policy["accepted_ids"]["1.50"] = "../outside"
    with pytest.raises(ValueError, match="exact fifty"):
        call_serial(peer, reviewed, policy, tmp_path / "proof", version)


def test_serial_gate_layout_store_to_readonly_index_survives_reopen(peer, tmp_path):
    reviewed = selected(peer)
    dispatcher = reviewed["dispatcher"]
    capacity = dict(cpu=8, memory=8192, pids=100, disk=10000, executor=30, go_build=2, container_start=1)
    resources = dict(cpu=1, memory=100, pids=1, disk=10, executor=1, go_build=0, container_start=0)
    database = tmp_path / "accepted.sqlite"
    store = dispatcher.store.Store(database, capacity, boot_id="recorded-index-boot", host_boot_id="recorded-host-boot")
    bindings = dict(baseline=reviewed_peer.sha(reviewed["spec"]), candidate=reviewed_peer.sha(reviewed["plan"]), interface=reviewed["retained"]["sha256"], layout=reviewed["graph"].layout_digest)
    ids = {}
    records = {}
    try:
        for i in range(1, 51):
            task, lane = f"1.{i:02}", f"L{i:02}"
            token = store.claim(task, lane, "recorded-index-producer", bindings, resources)
            record = store.accept(token, reviewed["catalog_manifest"], reviewed_peer.sha(reviewed["catalog_manifest"]))
            ids[task], records[task] = token["attempt"], record
    finally:
        store.close()
    index = serial.ReadOnlyIndex(database)
    try:
        pin = index.pin_selected(ids)
        assert pin == serial.digest(records)
        assert len(ids) == len(set(ids.values())) == 50
        for task, attempt in ids.items():
            assert index.accepted(attempt) == records[task]
            assert index.latest_accepted(task) == records[task]
    finally:
        index.close()
    store = dispatcher.store.Store(database, capacity, boot_id="reopened-index-boot", host_boot_id="recorded-host-boot")
    try:
        token = store.claim("1.01", "L01", "recorded-index-producer", bindings, resources)
        store.accept(token, reviewed["catalog_manifest"], reviewed_peer.sha(reviewed["catalog_manifest"]))
    finally:
        store.close()
    index = serial.ReadOnlyIndex(database)
    try:
        with pytest.raises(ValueError, match="superseded"):
            index.pin_selected(ids)
    finally:
        index.close()


EXACT_CASES = [
    (kind, name)
    for kind in (legacy.ExactSerialSourcePolicy, legacy.ExactSerialSourceExtension, legacy.ApprovedSerialStagePrelude)
    for name in unittest.defaultTestLoader.getTestCaseNames(kind)
]


@pytest.mark.parametrize("kind,name", EXACT_CASES, ids=[kind.__name__ + "." + name for kind, name in EXACT_CASES])
def test_serial_gate_layout_retains_every_exact_patch_assertion(kind, name):
    # These existing filesystem/codec cases launch no native checks. debug()
    # executes every original assertion and cleanup without translating failures
    # into synthetic receipts, skipped tests, or an ignored unittest result.
    kind(name).debug()


def test_serial_gate_layout_approved_generator_preserves_patch_allocations(tmp_path):
    source = FIXTURES / "reviewed-prelude-ownership.json"
    output = tmp_path / "ownership.json"
    identity = prelude_approval.migrate_approved_prelude(source, output)
    assert output.read_bytes() == legacy.PRELUDE_OWNERSHIP_BYTES
    assert identity == serial.PRELUDED_SERIAL_OWNERSHIP_SHA256
    historical = json.loads(source.read_bytes())
    approved = json.loads(output.read_bytes())
    assert approved["path_count"] == len(approved["paths"]) == 388
    assert len(historical["paths"]) == 387
    assert [entry for entry in approved["paths"] if entry != serial.PRELUDE_CONTROL_ENTRY] == historical["paths"]
    assert approved["root_exclusive_control_paths"] == sorted(serial.PRELUDE_CONTROL_PATHS)
    review = {
        "source_policy": {
            "version": 4, "kind": serial.PRELUDED_SERIAL_POLICY_KIND, "task": "1.99",
            "ownership_artifact": str(output), "ownership_sha256": identity,
        },
    }
    paths, prelude = serial.exact_serial_prelude_paths(review, legacy.patches)
    assert paths == frozenset(entry["path"] for entry in historical["paths"])
    assert not paths.intersection(serial.PRELUDE_CONTROL_PATHS)
    assert prelude == serial.APPROVED_STAGE_PRELUDE
    task = registry(ROOT)["tasks"]["1.99"]
    assert task["serial_gate_policy"]["serial_integration"]["source_policy"]["ownership_sha256"] == identity
    assert task["policy_sha256"] == serial.digest(task["serial_gate_policy"])
    approval = json.loads((SOURCE / "bootstrap/prelude-review-22.33.973.json").read_bytes())
    assert approval["ownership_sha256"] == identity
    assert approval["source_sha256"] == reviewed_peer.sha(source)
    assert approval["control_source_sha256"] == reviewed_peer.sha(SOURCE / "control_plane.py")
    assert approval["generator_command"] == "python3 tools/launchpad/production/approve_serial_prelude.py --approval 22.33.973"
    output.write_bytes(output.read_bytes() + b"\n")
    with pytest.raises(ValueError, match="artifact differs from reviewed hash"):
        serial.exact_serial_prelude_paths(review, legacy.patches)


def test_serial_gate_layout_approval_refuses_changed_source(tmp_path):
    source = tmp_path / "reviewed.json"
    source.write_bytes((FIXTURES / "reviewed-prelude-ownership.json").read_bytes() + b"\n")
    output = tmp_path / "ownership.json"
    with pytest.raises(ValueError, match="artifact differs from reviewed hash"):
        prelude_approval.migrate_approved_prelude(source, output)
    assert not output.exists()


@pytest.fixture
def prelude_review(tmp_path):
    stage = tmp_path / "stage"
    target = stage / serial.APPROVED_STAGE_PRELUDE["path"]
    target.parent.mkdir(parents=True)
    target.write_bytes(legacy.PRELUDE_BEFORE_BYTES)
    target.chmod(0o644)
    artifact = tmp_path / "artifact"
    (artifact / "blobs").mkdir(parents=True)
    identity = prelude_approval.migrate_approved_prelude(
        FIXTURES / "reviewed-prelude-ownership.json", artifact / "ownership.json"
    )
    (artifact / "blobs" / serial.APPROVED_STAGE_PRELUDE["blob_sha256"]).write_bytes(legacy.PRELUDE_AFTER_BYTES)
    policy = {
        "version": 4, "kind": serial.PRELUDED_SERIAL_POLICY_KIND, "task": "1.99",
        "ownership_artifact": str(artifact / "ownership.json"), "ownership_sha256": identity,
    }
    return stage, artifact, {"source_policy": policy, "artifact_directory": str(artifact)}


def test_serial_gate_layout_prelude_and_hash_refusals_are_distinct(prelude_review):
    stage, artifact, review = prelude_review
    ownership = artifact / "ownership.json"
    data = ownership.read_bytes()
    ownership.write_bytes(data + b"\n")
    with pytest.raises(ValueError, match="prelude not present"):
        serial.reconcile_serial_patch(stage, artifact, review, legacy.patches)
    ownership.write_bytes(data)
    serial.apply_approved_stage_prelude(stage, review, legacy.patches)
    ownership.write_bytes(data + b"\n")
    with pytest.raises(ValueError, match="artifact differs from reviewed hash"):
        serial.reconcile_serial_patch(stage, artifact, review, legacy.patches)
    assert (stage / serial.APPROVED_STAGE_PRELUDE["path"]).read_bytes() == legacy.PRELUDE_AFTER_BYTES


def test_serial_gate_layout_new_control_entry_cannot_be_patched(prelude_review):
    stage, artifact, review = prelude_review
    serial.apply_approved_stage_prelude(stage, review, legacy.patches)
    before = legacy.patches._tree(stage)
    manifest = {
        "version": 4, "baseline_digest": serial.digest(before),
        "source_policy_sha256": serial.digest(review["source_policy"]),
        "entries": [{"path": serial.PRELUDE_CONTROL_ENTRY["path"], "before": None, "after": None}],
    }
    reviewed_peer.write_json(artifact / "manifest.json", manifest)
    review.update(manifest_sha256=reviewed_peer.sha(artifact / "manifest.json"), allowed_paths=[serial.PRELUDE_CONTROL_ENTRY["path"]])
    with pytest.raises(ValueError, match="unreviewed exact serial path"):
        serial.reconcile_serial_patch(stage, artifact, review, legacy.patches)
    assert legacy.patches._tree(stage) == before
