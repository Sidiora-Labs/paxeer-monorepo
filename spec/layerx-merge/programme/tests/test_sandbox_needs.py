from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
MANIFEST = ROOT / "spec/layerx-merge/programme/data/sandbox-profiles.kvx"


def test_needs_rows_use_known_profiles() -> None:
    manifest = load(MANIFEST)
    profiles = {
        name.removeprefix("profile.")
        for name in manifest
        if name.startswith("profile.")
    }
    needs = {
        name.removeprefix("needs."): row
        for name, row in manifest.items()
        if name.startswith("needs.")
    }
    tasks = load(ROOT / "spec/layerx-merge/spec.kvx")
    limit = load(ROOT / "spec/workflow.kvx")["limits"]["max_task_minutes"]
    assert limit == 30
    assert profiles
    assert needs
    for task, row in needs.items():
        assert row["profile"] in profiles, task
        assert row["profile"] == tasks[f"task.{task}"]["sandbox_profile"], task
        assert row["network"] in {"none", "registry", "peer"}, task
        assert type(row["time_limit_minutes"]) is int, task
        assert row["time_limit_minutes"] == limit, task
        resources = row["resources"]
        assert isinstance(resources, list) and resources, task
        assert all(isinstance(item, str) and item.strip() for item in resources), task

    launchpad = {f"4.5.{n}" for n in range(2, 10)} | {
        f"4.10.{n}" for n in range(3, 6)
    }
    buf = {
        f"7.{slot}.{n}"
        for slot, count in ((21, 3), (22, 3), (23, 8), (24, 10), (25, 13))
        for n in range(1, count + 1)
    } | {"7.26.1", "7.26.2", "7.27.1", "7.27.2", "7.28.3", "7.29.1", "7.29.2"}
    groups = (
        (launchpad, "peer", "one-validator test network"),
        (buf, "registry", "Go module proxy for buf"),
        ({"10.11.2"}, "peer", "secp256k1 key in KMS attestor test cluster"),
        ({f"10.13.{n}" for n in range(3, 6)}, "peer", "peer socket"),
    )
    for required_tasks, network, resource in groups:
        assert required_tasks <= needs.keys()
        for task in required_tasks:
            assert needs[task]["network"] == network, task
            assert resource in needs[task]["resources"], task
