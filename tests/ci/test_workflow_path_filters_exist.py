from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
GLOB_CHARACTERS = "*?[]+"


def workflow_paths():
    paths = sorted(
        path
        for path in WORKFLOWS.rglob("*")
        if path.is_file() and path.suffix in {".yml", ".yaml"}
    )
    assert paths, "No workflow files found"
    return paths


def test_workflow_negated_path_filters_name_existing_files():
    checked = []
    missing = []
    for path in workflow_paths():
        workflow = yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
        triggers = workflow["on"]
        if not isinstance(triggers, dict):
            continue
        for event, configuration in triggers.items():
            if not isinstance(configuration, dict):
                continue
            for filter_name in ("paths", "paths-ignore"):
                for pattern in configuration.get(filter_name, []):
                    if not pattern.startswith("!"):
                        continue
                    target = pattern[1:]
                    if any(character in target for character in GLOB_CHARACTERS):
                        continue
                    reference = (
                        f"{path.relative_to(ROOT)}: on.{event}.{filter_name}: {pattern}"
                    )
                    checked.append(reference)
                    if not (ROOT / target).exists():
                        missing.append(reference)
    assert checked, "No literal negated path filters checked"
    assert not missing, "Negated path filters name missing files:\n" + "\n".join(missing)


def test_no_workflow_names_paxeer_beta_deploy():
    references = [
        f"{path.relative_to(ROOT)}:{number}"
        for path in workflow_paths()
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
        if "PaxeerBetaDeploy" in line
    ]
    assert not references, "Deleted deploy script references remain:\n" + "\n".join(
        references
    )
