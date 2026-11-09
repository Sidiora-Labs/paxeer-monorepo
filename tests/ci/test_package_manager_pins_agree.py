import itertools
import json
from pathlib import Path
import re
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
PNPM_PIN = re.compile(r"pnpm@(\d+\.\d+\.\d+)(?:\+sha(?:224|256|384|512)\.[0-9a-f]+)?")
DOCKERFILE_WORKSPACES = (
    ("docker/web/Dockerfile", "build", "services/wallet"),
    ("docker/wallet-gateway/Dockerfile", "build", "services/wallet"),
    ("docker/wallet-pwa/Dockerfile", "sdk", "services/wallet"),
    ("docker/wallet-pwa/Dockerfile", "builder", "apps/wallet"),
    ("apps/perps/Dockerfile", "deps", "apps/perps"),
    ("apps/perps/Dockerfile", "builder", "apps/perps"),
    ("deploy/launchpad/Dockerfile.web", "build", "apps/launchpad"),
)
IMAGE_DEPENDENCIES_FIXTURE = (
    ROOT / "apps/launchpad/qualification/image-dependencies/fixture"
)
# Keep the inventory lines as evidence, but identify producers by their owning
# jobs: reusable workflow extraction and earlier setup steps can shift lines.
WORKFLOW_REPAIR_JOBS = {
    (".github/workflows/bridge-test.yml", "bridge-app"): 143,
    (".github/workflows/ci.yml", "js-pnpm-tests"): 271,  # Inventory line 267.
    (".github/workflows/ci.yml", "js-wallet-gateway-tests"): 302,  # Inventory line 298.
    (".github/workflows/wallet-go-test.yml", "attestor"): 74,
    (".github/workflows/platform.yml", "connected-middleware"): 195,
}
WORKFLOW_PNPM_INSTALL = re.compile(
    r"\bnpm\s+install\s+(?:--global|-g)\s+[\"']?(pnpm@[^\s\"';&|]+)"
    r"|--package=[\"']?(pnpm@[^\s\"';&|]+)"
)
MANIFEST_PNPM_INSTALL = (
    "npm install --global \"$(node -p 'require(\"./package.json\").packageManager')\""
)
MATRIX_DIRECTORY = re.compile(r"\$\{\{\s*matrix\.([\w-]+)\s*\}\}")
PNPM_DIRECTORY = re.compile(
    r"\bpnpm\s+(?:--dir|-C)(?:=|\s+)(?:\"([^\"]+)\"|'([^']+)'|([^\s;&|]+))"
)


def _tracked_paths(pattern):
    return [
        ROOT / path
        for path in subprocess.check_output(
            ["git", "ls-files", "-z", "--", pattern], cwd=ROOT
        ).decode().split("\0")
        if path
    ]


def _manifest(directory):
    return json.loads((directory / "package.json").read_text())


def _release(pin, path):
    match = PNPM_PIN.fullmatch(pin)
    assert match, f"{path}: packageManager must pin an exact pnpm release: {pin!r}"
    return match[1]


def _manager_manifest(directory):
    directory = directory.resolve()
    assert directory.is_relative_to(ROOT), directory
    assert directory.is_dir(), directory
    for parent in (directory, *directory.parents):
        path = parent / "package.json"
        if path.is_file() and "packageManager" in _manifest(parent):
            return path
        if parent == ROOT:
            break
    pytest.fail(f"{directory}: no ancestor package.json declares packageManager")


def _matrix_rows(job):
    matrix = job.get("strategy", {}).get("matrix", {})
    assert isinstance(matrix, dict), "pnpm jobs need a statically resolvable matrix"
    axes = {
        key: value
        for key, value in matrix.items()
        if key not in {"include", "exclude"}
    }
    assert all(isinstance(values, list) and values for values in axes.values()), matrix
    if not axes:
        rows = matrix.get("include", [{}])
        assert rows, "pnpm job matrix has no instances"
        return rows
    rows = [dict(zip(axes, values)) for values in itertools.product(*axes.values())]
    rows = [
        row
        for row in rows
        if not any(
            all(row.get(key) == value for key, value in excluded.items())
            for excluded in matrix.get("exclude", [])
        )
    ]
    for included in matrix.get("include", []):
        compatible = [
            row
            for row in rows
            if all(key not in axes or row[key] == value for key, value in included.items())
        ]
        if compatible:
            for row in compatible:
                row.update(included)
        else:
            rows.append(included.copy())
    assert rows, "pnpm job matrix has no instances"
    return rows


def _run_directory(step, job, workflow, row):
    default = workflow.get("defaults", {}).get("run", {}).get("working-directory", ".")
    default = job.get("defaults", {}).get("run", {}).get("working-directory", default)
    directory = MATRIX_DIRECTORY.sub(
        lambda match: str(row[match[1]]), step.get("working-directory", default)
    )
    assert "${{" not in directory, f"Unresolved working-directory: {directory}"
    return ROOT / directory


def _workflow_pnpm_cases():
    cases = {}
    recorded_locations = {}
    for path in sorted((ROOT / ".github/workflows").iterdir()):
        if path.suffix not in {".yml", ".yaml"}:
            continue
        text = path.read_text()
        workflow = yaml.safe_load(text)
        nodes = {key.value: value for key, value in yaml.compose(text).value}
        job_nodes = {key.value: value for key, value in nodes["jobs"].value}
        for job_name, job in workflow["jobs"].items():
            fields = {key.value: value for key, value in job_nodes[job_name].value}
            for index, step in enumerate(job.get("steps", [])):
                command = step.get("run", "")
                if "run" not in step:
                    continue
                step_nodes = {
                    key.value: value
                    for key, value in fields["steps"].value[index].value
                }
                run_node = step_nodes["run"]
                installs = [
                    (match.start(), match[1] or match[2])
                    for match in WORKFLOW_PNPM_INSTALL.finditer(command)
                ]
                # The repaired matrix line reads each instance's own manifest.
                # Other manifest-driven installs are outside the literal-pin sweep.
                identity = (str(path.relative_to(ROOT)), job_name)
                if (
                    identity in WORKFLOW_REPAIR_JOBS
                    and command == MANIFEST_PNPM_INSTALL
                ):
                    installs.append((0, None))
                if not installs:
                    continue
                for offset, pin in installs:
                    line = run_node.start_mark.line + 1 + command[:offset].count("\n")
                    if run_node.style in {"|", ">"}:
                        line += 1
                    location = (str(path.relative_to(ROOT)), line)
                    assert location not in cases, f"Multiple pnpm activations at {location}"
                    if identity in WORKFLOW_REPAIR_JOBS:
                        assert identity not in recorded_locations, (
                            f"Multiple pnpm activations in recorded job {identity}"
                        )
                        recorded_locations[identity] = location
                    cases[location] = pytest.param(
                        workflow,
                        job_name,
                        step,
                        pin,
                        location,
                        id=f"{location[0]}:{line}-{job_name}",
                    )
    assert set(WORKFLOW_REPAIR_JOBS) <= recorded_locations.keys(), (
        "Missing recorded workflow pnpm install"
    )
    recorded = [cases.pop(recorded_locations[identity]) for identity in WORKFLOW_REPAIR_JOBS]
    return recorded + list(cases.values())


@pytest.mark.parametrize(
    "workflow,job_name,install_step,pin,location", _workflow_pnpm_cases()
)
def test_workflow_pnpm_matches_package_manager(
    workflow, job_name, install_step, pin, location
):
    job = workflow["jobs"][job_name]
    for row in _matrix_rows(job):
        installed = pin
        if installed is None:
            directory = _run_directory(install_step, job, workflow, row)
            installed = _manifest(directory)["packageManager"]
        actual = _release(installed, location)
        manifests = set()
        for step in job["steps"]:
            command = step.get("run", "")
            # Native and npm steps do not consume the job's installed pnpm.
            if not re.search(r"\bpnpm(?:\s|$)", command):
                continue
            directory = _run_directory(step, job, workflow, row)
            explicit = list(PNPM_DIRECTORY.finditer(command))
            if explicit:
                for match in explicit:
                    target = next(value for value in match.groups() if value is not None)
                    manifests.add(_manager_manifest(directory / target))
            else:
                manifests.add(_manager_manifest(directory))
        assert manifests, f"{location}: {job_name} has no pnpm working directory"
        for manifest in sorted(manifests):
            expected = _manifest(manifest.parent)["packageManager"]
            assert actual == _release(expected, manifest), (
                f"{location[0]}:{location[1]}: {job_name} {row} installs {installed}, "
                f"but {manifest.relative_to(ROOT)} declares {expected}"
            )


def _workspaces():
    workspaces = []
    for path in _tracked_paths("**/pnpm-workspace.yaml"):
        # This npm image fixture uses the file only to supply pnpm overrides.
        if path.parent != IMAGE_DEPENDENCIES_FIXTURE:
            workspaces.append(path.parent)
    assert workspaces, "No pnpm workspaces found"
    return workspaces


def _lock_importers(path):
    importers = {}
    for document in yaml.safe_load_all(path.read_text()):
        assert isinstance(document, dict) and "lockfileVersion" in document, path
        for name, groups in document.get("importers", {}).items():
            importer = importers.setdefault(name, {})
            for group, dependencies in groups.items():
                assert group not in importer, (path, name, group)
                importer[group] = dependencies
    assert "." in importers, f"{path}: missing root importer"
    return importers


@pytest.mark.parametrize("dockerfile,stage,owner", DOCKERFILE_WORKSPACES)
def test_dockerfile_pnpm_matches_package_manager(dockerfile, stage, owner):
    manifest_path = ROOT / owner / "package.json"
    expected = _release(_manifest(manifest_path.parent)["packageManager"], manifest_path)
    assert (manifest_path.parent / "pnpm-lock.yaml").is_file(), owner
    text = (ROOT / dockerfile).read_text()
    stages = re.split(r"(?im)^FROM\s+", text)[1:]
    matching = [
        section.partition("\n")[2]
        for section in stages
        if re.search(rf"\sAS\s+{re.escape(stage)}\s*$", section.splitlines()[0], re.I)
    ]
    assert len(matching) == 1, (dockerfile, stage)
    commands = "\n".join(
        line for line in matching[0].splitlines() if not line.lstrip().startswith("#")
    ).replace("\\\n", " ")
    pins = re.findall(r"\bpnpm@([^\s\"';]+)", commands)
    assert pins, f"{dockerfile}: {stage} must activate a pinned pnpm release"
    assert re.search(
        r"corepack prepare pnpm@\S+ --activate|"
        r"npm install -g pnpm@\S+|"
        r"npm exec --yes --package=pnpm@\S+ -- pnpm\b",
        commands,
    ), f"{dockerfile}: {stage} does not activate pnpm"
    for pin in pins:
        assert _release(f"pnpm@{pin}", dockerfile) == expected, (
            f"{dockerfile}: {stage} activates pnpm@{pin}, "
            f"but {owner}/package.json pins pnpm@{expected}"
        )


def test_each_pnpm_workspace_has_a_lockfile():
    for workspace in _workspaces():
        _release(_manifest(workspace).get("packageManager", ""), workspace)
        lockfile = workspace / "pnpm-lock.yaml"
        assert lockfile.is_file(), f"{workspace}: missing pnpm-lock.yaml"
        _lock_importers(lockfile)


def test_shared_lockfile_has_one_pnpm_release():
    workspaces = set(_workspaces())
    for path in _tracked_paths("**/pnpm-lock.yaml"):
        if _manifest(path.parent).get("packageManager", "").startswith("pnpm@"):
            workspaces.add(path.parent)
    releases_by_member = {}
    for workspace in sorted(workspaces):
        expected = _release(_manifest(workspace).get("packageManager", ""), workspace)
        for importer in _lock_importers(workspace / "pnpm-lock.yaml"):
            member = (workspace / importer).resolve()
            manifest = _manifest(member)
            if "packageManager" in manifest:
                actual = _release(manifest["packageManager"], member)
                assert actual == expected, (
                    f"{workspace}/pnpm-lock.yaml: {member} pins pnpm@{actual}, "
                    f"but the lockfile owner pins pnpm@{expected}"
                )
            previous = releases_by_member.setdefault(member, expected)
            assert previous == expected, (
                f"{member} is installed by shared lockfiles using "
                f"pnpm@{previous} and pnpm@{expected}"
            )
