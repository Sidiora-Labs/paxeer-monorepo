from pathlib import Path

from programme.retired import load_components, render_doc

DOCUMENT_PATH = Path(__file__).resolve().parents[2] / "docs/retirements-and-callers.md"


def test_doc_matches_render() -> None:
    components = load_components()
    rendered = render_doc(components)

    assert DOCUMENT_PATH.read_bytes() == rendered.encode("utf-8")
    assert render_doc(dict(reversed(list(components.items())))) == rendered

    retired, callers = rendered.split("\n## Caller groups\n")
    retired_rows = [line for line in retired.splitlines() if line.startswith("| ")][2:]
    caller_rows = [line for line in callers.splitlines() if line.startswith("| ")][2:]
    assert len(retired_rows) == len(components)
    assert [row.split(" | ")[0] for row in retired_rows] == [
        f"| {name}" for name in sorted(components)
    ]

    domains = sorted({component["owner_domain"] for component in components.values()})
    assert len(caller_rows) == len(domains)
    for row, (name, component) in zip(retired_rows, sorted(components.items())):
        cells = row.removeprefix("| ").removesuffix(" |").split(" | ")
        expected = [
            name,
            str(component["owner_domain"]),
            component.get("address", "none"),
            component["refusal"],
            ", ".join(component["deleted_trees"]) or "none",
            component["replacement"],
        ]
        assert cells == [
            value.replace("|", "\\|").replace("\n", "<br>") for value in expected
        ], name

    for row, domain in zip(caller_rows, domains):
        names = sorted(
            name
            for name, component in components.items()
            if component["owner_domain"] == domain
        )
        assert row == f"| {domain} | {len(names)} | {', '.join(names)} |"
