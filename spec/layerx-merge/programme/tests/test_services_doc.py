from pathlib import Path

from programme.services import load_catalogue, render_doc

DOC_PATH = Path(__file__).resolve().parents[2] / "docs/off-chain-services.md"


def test_services_doc_matches_render() -> None:
    catalogue = load_catalogue()
    rendered = render_doc(catalogue)

    assert DOC_PATH.read_bytes() == rendered.encode("utf-8")
    sections = rendered.split("\n## Domain ")[1:]
    domains = sorted({service["domain"] for service in catalogue.values()})
    assert [int(section.split("\n", 1)[0]) for section in sections] == domains
    for domain, section in zip(domains, sections, strict=True):
        services = sorted(
            (slug, service)
            for slug, service in catalogue.items()
            if service["domain"] == domain
        )
        rows = [line for line in section.splitlines() if line.startswith("| `")]
        assert len(rows) == len(services)
        for (slug, service), row in zip(services, rows, strict=True):
            assert row.split(" | ") == [
                f"| `{slug}`",
                service["route"],
                f"`{service['readiness']}`",
                f"`{service['operation_task']}` |",
            ]

    assert render_doc(dict(reversed(list(catalogue.items())))) == rendered
