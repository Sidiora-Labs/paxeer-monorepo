"""Recorded requests for service instances in project environments."""

import copy
import importlib.util
from pathlib import Path

import pytest

spec = importlib.util.spec_from_file_location(
    "platform_project_cases", Path(__file__).with_name("test_platform_project.py"))
cases = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cases)
client = cases.client


def project_with_environments(others, names=("store", "worker")):
    node = cases.project_node(names=names)
    node["environments"]["edges"] = [
        *cases.connection(others)["edges"], *node["environments"]["edges"]]
    return node


def new_service_entries(node, others):
    instances = {name: instance for name, instance in cases.INSTANCES.items() if name != "api"}
    records = cases.read_entries(node, instances=instances)
    records.extend(cases.status_entry(name) for name in ("store", "worker"))
    writes = cases.service_writes("api")
    removals = [cases.entry("OtherEnvironmentRemove", {"s": "s-api", "e": env["id"]},
                           {"serviceDelete": True}) for env in others]
    records.extend([writes[0], *removals, *writes[1:], cases.status_entry("api")])
    return records


def assert_removals(transport, others):
    writes = [call for call in transport.calls if call[0] == "mutation"]
    assert [call[1] for call in writes] == [
        "ServiceCreate", *("OtherEnvironmentRemove" for _ in others),
        "InstanceUpdate", "ServiceConnect", "VolumeCreate", "VariablesUpsert", "DomainCreate", "Deploy"]
    removals = [call for call in writes if call[1] == "OtherEnvironmentRemove"]
    assert [call[2] for call in removals] == [{"s": "s-api", "e": env["id"]} for env in others]
    for _, _, variables, query in removals:
        assert variables["e"] != "e1"
        assert "serviceDelete(id: $s, environmentId: $e)" in query
    transport.exhausted()


def test_new_service_is_removed_from_the_other_environment(client):
    others = [{"id": "e-staging", "name": "staging"}]
    node = project_with_environments(others)
    result, transport = cases.invoke(client, new_service_entries(node, others))
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("applied api OtherEnvironmentRemove\n") == 1
    assert_removals(transport, others)


def test_two_other_environments_are_cleared_in_name_order(client):
    others = [{"id": "e-z", "name": "development"}, {"id": "e-a", "name": "staging"}]
    node = project_with_environments(list(reversed(others)))
    result, transport = cases.invoke(client, new_service_entries(node, others))
    assert result[0] == 0 and result[2] == ""
    assert result[1].count("applied api OtherEnvironmentRemove\n") == 2
    assert_removals(transport, others)


def test_existing_service_keeps_its_other_instances(client):
    others = [{"id": "e-staging", "name": "staging"}]
    node = project_with_environments(others, names=cases.NAMES)
    instances = copy.deepcopy(cases.INSTANCES)
    instances["api"]["latestDeployment"]["meta"]["branch"] = "release"
    records = cases.read_entries(node, instances=instances)
    records.extend(cases.status_entry(name) for name in ("store", "worker"))
    records.extend([cases.connect_entry(), cases.deploy_entry("api"), cases.status_entry("api")])
    result, transport = cases.invoke(client, records)
    assert result[0] == 0 and result[2] == ""
    assert transport.mutations() == ["ServiceConnect", "Deploy"]
    assert "elsewhere" not in result[1]
    transport.exhausted()


def test_single_environment_sends_no_removal(client):
    node = project_with_environments([])
    result, transport = cases.invoke(client, new_service_entries(node, []))
    assert result[0] == 0 and result[2] == ""
    assert "elsewhere" not in result[1]
    assert_removals(transport, [])


def test_plan_names_the_other_environments(client):
    others = [{"id": "e-z", "name": "development"}, {"id": "e-a", "name": "staging"}]
    node = project_with_environments(list(reversed(others)))
    instances = {name: instance for name, instance in cases.INSTANCES.items() if name != "api"}
    result, transport = cases.invoke(client, cases.read_entries(node, instances=instances), mode="--plan")
    assert result[0] == 0 and result[2] == ""
    lines = result[1].splitlines()
    create = lines.index("  create api")
    assert lines[create + 1:create + 3] == ["  elsewhere development", "  elsewhere staging"]
    assert [line for line in lines if line.startswith("  elsewhere ")] == [
        "  elsewhere development", "  elsewhere staging"]
    assert transport.mutations() == []
    transport.exhausted()


def test_removal_failure_stops_before_repository_connection(client):
    others = [{"id": "e-staging", "name": "staging"}]
    node = project_with_environments(others)
    records = new_service_entries(node, others)
    removal = next(index for index, record in enumerate(records) if record["op"] == "OtherEnvironmentRemove")
    records = records[:removal + 1]
    records[-1] = {"op": "OtherEnvironmentRemove", "variables": {"s": "s-api", "e": "e-staging"},
                   "errors": [{"message": "Service instance removal refused"}]}
    result, transport = cases.invoke(client, records)
    assert result[0] == 1
    assert "Service instance removal refused" in result[2]
    assert transport.mutations() == ["ServiceCreate", "OtherEnvironmentRemove"]
    transport.exhausted()


@pytest.mark.parametrize("mode", ["--plan", "--apply"])
def test_new_project_has_no_other_environments(client, mode):
    records = [cases.projects_entry()] if mode == "--plan" else cases.creation_entries()
    result, transport = cases.invoke(client, records, mode=mode)
    assert result[0] == 0 and result[2] == ""
    assert "elsewhere" not in result[1]
    assert "OtherEnvironmentRemove" not in transport.mutations()
    transport.exhausted()
