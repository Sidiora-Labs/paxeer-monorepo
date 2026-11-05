import re
from pathlib import Path


def test_no_template_names_the_intent_ingester():
    deploy = Path(__file__).resolve().parents[1]
    retired_template = deploy / "intent-ingester" / "railway.env.example"
    assert not retired_template.exists(), "The retired deployment template still exists"

    templates = sorted(
        path
        for path in deploy.rglob("*")
        if path.is_file()
        and (path.name.endswith(".example") or ".example." in path.name)
    )
    assert templates, "No deployment templates found"

    retired_name = re.compile(
        r"intent[-_\s]+ingester|\b(?:layerx_)?ingester_", re.IGNORECASE
    )
    references = []
    for path in templates + [deploy / "gateway" / "README.md"]:
        relative_path = path.relative_to(deploy)
        if retired_name.search(str(relative_path)):
            references.append(str(relative_path))
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if retired_name.search(line):
                references.append(f"{relative_path}:{number}")

    assert not references, "Retired intent ingester references: " + ", ".join(references)
