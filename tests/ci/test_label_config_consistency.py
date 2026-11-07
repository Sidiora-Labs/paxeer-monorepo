"""Keep automatic area labels, release categories and manual PR labels aligned."""

from fnmatch import fnmatchcase
from pathlib import Path
import re

import yaml


ROOT = Path(__file__).resolve().parents[2]


def load_config(path):
    return yaml.load((ROOT / path).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def labeler_globs(labeler, label):
    patterns = []
    for rule in labeler[label]:
        for condition in rule["changed-files"]:
            globs = condition["any-glob-to-any-file"]
            patterns.extend([globs] if isinstance(globs, str) else globs)
    assert patterns, f"{label} must have path rules"
    return patterns


def test_every_release_category_label_has_labeler_rule():
    labeler = load_config(".github/labeler.yml")
    release = load_config(".github/release.yml")
    categories = release["changelog"]["categories"]
    assert categories, "Release categories must not be empty"
    checked = set()
    for category in categories:
        assert category["labels"], category["title"]
        for label in category["labels"]:
            if label == "*":
                continue
            assert label in labeler, f"Release label {label} has no labeler rule"
            labeler_globs(labeler, label)
            checked.add(label)
    assert {"area:wallet", "area:ci"} <= checked


def test_wallet_paths_labelled_area_wallet():
    labeler = load_config(".github/labeler.yml")
    wallet = labeler_globs(labeler, "area:wallet")
    human = labeler_globs(labeler, "area:human")
    assert {"apps/wallet/**", "sdk/wallet/**"} <= set(wallet)
    assert not {"apps/wallet/**", "sdk/wallet/**"}.intersection(human)
    for root in ("apps/wallet", "sdk/wallet"):
        for relative in ("package.json", "src/wallet/index.ts", ".config/settings"):
            path = f"{root}/{relative}"
            assert any(fnmatchcase(path, pattern) for pattern in wallet), path
            assert not any(fnmatchcase(path, pattern) for pattern in human), path
    for path in ("services/human/main.rs", "apps/perps/index.ts", "apps/web/index.ts"):
        assert any(fnmatchcase(path, pattern) for pattern in human), path


def test_ci_paths_labelled_area_ci():
    labeler = load_config(".github/labeler.yml")
    patterns = labeler_globs(labeler, "area:ci")
    roots = (".github", "tools/ci", "scripts/ci", "tests/ci")
    assert {f"{root}/**" for root in roots} <= set(patterns)
    for root in roots:
        for relative in ("config.yml", "nested/check.py"):
            path = f"{root}/{relative}"
            assert any(fnmatchcase(path, pattern) for pattern in patterns), path


def test_enforce_labels_names_known_labels():
    workflow = load_config(".github/workflows/paxeer-enforce-labels.yml")
    enforced = [
        step["with"]["REQUIRED_LABELS_ANY"]
        for job in workflow["jobs"].values()
        for step in job["steps"]
        if "REQUIRED_LABELS_ANY" in step.get("with", {})
    ]
    assert len(enforced) == 1, "The app-hash label check must remain configured"
    labels = [label.strip() for label in enforced[0].split(",")]
    assert len(labels) == 2
    assert set(labels) == {"app-hash-breaking", "non-app-hash-breaking"}
    # Manual labels are documented in release.yml; area labels have path rules.
    configs = "\n".join(
        (ROOT / path).read_text(encoding="utf-8")
        for path in (".github/release.yml", ".github/labeler.yml")
    )
    for label in labels:
        assert re.search(
            rf"(?<![\w:-]){re.escape(label)}(?![\w:-])", configs
        ), f"Enforced label {label} is absent from the release and labeler configs"
