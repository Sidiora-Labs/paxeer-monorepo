import re
from pathlib import Path

from programme.kvx import load
from programme.services import UNDECLARED_READINESS, diff_spec, load_catalogue

SPEC_PATH = Path(__file__).resolve().parents[2] / "spec.kvx"


def test_every_service_has_domain_and_tasks() -> None:
    catalogue = load_catalogue()
    spec = load(SPEC_PATH)
    assert catalogue
    failures: list[str] = []
    for slug, service in catalogue.items():
        domain = service["domain"]
        if type(domain) is not int or not 3 <= domain <= 16:
            failures.append(f"{slug}: domain {domain} is outside 3..16")
        if service["completion_task"] != f"{domain}.0.9":
            failures.append(f"{slug}: completion_task does not close domain {domain}")
        if (
            re.fullmatch(rf"{domain}\.[0-9]+\.[0-9]+", service["operation_task"])
            is None
        ):
            failures.append(f"{slug}: operation_task does not belong to domain {domain}")
        for field in ("completion_task", "operation_task"):
            if f"task.{service[field]}" not in spec:
                failures.append(f"{slug}: {field} {service[field]} is absent from spec")
        assert service["route"].strip(), slug
        readiness = service["readiness"]
        assert readiness == UNDECLARED_READINESS or readiness.startswith("/"), slug
    assert not failures, "\n".join(failures)


def test_catalogue_matches_spec_service_blocks(tmp_path: Path) -> None:
    catalogue = load_catalogue()
    spec = load(SPEC_PATH)
    expected = {
        name.removeprefix("service.")
        for name in spec
        if name.startswith("service.")
    }
    assert expected
    assert set(catalogue) == expected
    assert diff_spec(catalogue, SPEC_PATH) == (set(), set())
    for slug, service in catalogue.items():
        source = spec[f"service.{slug}"]
        assert service["domain"] == int(source["domain"]), slug
        for field in ("route", "completion_task", "operation_task"):
            assert service[field] == source[field], (slug, field)
        if "readiness" in source:
            assert service["readiness"] == source["readiness"], slug

    assert catalogue["perps-data"]["domain"] == 3
    assert catalogue["perps-data"]["readiness"] == "/healthz/ready"
    assert catalogue["launchpad-gateway"]["domain"] == 4
    assert catalogue["launchpad-indexer"]["domain"] == 4
    assert catalogue["platform-gateway"]["readiness"] == "/readyz"
    assert catalogue["explorer"]["readiness"] == "/api/health/readiness"

    omitted = next(iter(catalogue))
    reduced = {slug: service for slug, service in catalogue.items() if slug != omitted}
    assert diff_spec(reduced, SPEC_PATH) == ({omitted}, set())
    assert diff_spec({}, SPEC_PATH) == (expected, set())

    service_block = f"[service.{omitted}]"
    renamed = "catalogue-spec-difference"
    changed_spec = tmp_path / "spec.kvx"
    changed_spec.write_text(
        SPEC_PATH.read_text(encoding="utf-8").replace(
            service_block, f"[service.{renamed}]", 1
        ),
        encoding="utf-8",
    )
    assert diff_spec(catalogue, changed_spec) == ({renamed}, {omitted})
