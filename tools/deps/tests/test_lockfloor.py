import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
import textwrap

import pytest


SCRIPT = Path(__file__).resolve().parents[1] / "lockfloor.py"
SPEC = importlib.util.spec_from_file_location("lockfloor", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def write(root, name, content):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(content).lstrip())
    return path


def run_checker(root, manifests):
    floors = write(root, "floors/group.json", json.dumps({"manifests": manifests}))
    return subprocess.run(
        [sys.executable, str(SCRIPT), str(floors)],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )


def test_explorer_tools_locks_meet_floors():
    root = SCRIPT.parents[2]
    floors = SCRIPT.parent / "floors/explorer-tools.json"
    manifests = json.loads(floors.read_text())["manifests"]
    expected_locks = {
        "apps/explorer/toolkit/package/yarn.lock",
        "apps/explorer/deploy/tools/essential-dapps-chains-config-generator/yarn.lock",
        "apps/explorer/deploy/tools/affected-tests/yarn.lock",
        "apps/explorer/deploy/tools/favicon-generator/yarn.lock",
        "apps/explorer/deploy/tools/multichain-config-generator/yarn.lock",
        "apps/explorer/deploy/tools/llms-txt-generator/yarn.lock",
        "apps/explorer/deploy/tools/envs-validator/yarn.lock",
        "apps/explorer/deploy/tools/feature-reporter/yarn.lock",
        "apps/explorer/deploy/tools/sitemap-generator/yarn.lock",
    }
    assert set(manifests) == expected_locks
    locks = [root / name for name in sorted(expected_locks)]
    for lock in locks:
        assert lock.is_file(), f"Missing explorer tool lockfile: {lock}"
    original_contents = {lock: lock.read_bytes() for lock in locks}

    checked = subprocess.run(
        [sys.executable, str(SCRIPT), str(floors)],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    assert (checked.returncode, checked.stdout, checked.stderr) == (0, "", "")

    for lock in locks:
        installed = subprocess.run(
            [
                "yarn",
                "--cwd",
                str(lock.parent),
                "install",
                "--frozen-lockfile",
                "--ignore-scripts",
            ],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        assert installed.returncode == 0, (
            f"Frozen install failed in {lock.parent}:\n"
            f"{installed.stdout}\n{installed.stderr}"
        )
        assert lock.read_bytes() == original_contents[lock], (
            f"Frozen install changed {lock}"
        )
    for lock in locks:
        assert lock.read_bytes() == original_contents[lock], (
            f"Explorer tool installs changed {lock}"
        )


def test_dashboard_floor_uses_shared_install_lock(tmp_path):
    root = SCRIPT.parents[2]
    manifests = json.loads(
        (SCRIPT.parent / "floors/platform-node.json").read_text()
    )["manifests"]
    shared_lock_name = "apps/web/package-lock.json"
    dashboard = Path("services/platform/hosted/dashboard/web")
    assert str(dashboard / "package.json") not in manifests
    assert manifests[shared_lock_name] == {"next": "16.4.1"}
    assert not (root / dashboard / "package-lock.json").exists()

    shared_lock = json.loads((root / shared_lock_name).read_text())
    dashboard_manifest = json.loads((root / dashboard / "package.json").read_text())
    assert dashboard_manifest["dependencies"]["next"] == (
        shared_lock["packages"]["node_modules/next"]["version"]
    )
    assert shared_lock["packages"]["node_modules/next"]["version"] == "16.4.1"

    shared_copy = tmp_path / shared_lock_name
    shared_copy.parent.mkdir(parents=True)
    shutil.copyfile(root / shared_lock_name, shared_copy)
    dashboard_copy = tmp_path / dashboard / "package-lock.json"
    dashboard_copy.parent.mkdir(parents=True)
    shutil.copyfile(root / shared_lock_name, dashboard_copy)
    configured_entry = {shared_lock_name: manifests[shared_lock_name]}
    passed = run_checker(tmp_path, configured_entry)
    assert (passed.returncode, passed.stdout, passed.stderr) == (0, "", "")

    shared_lock["packages"]["node_modules/next"]["version"] = "16.3.5"
    shared_copy.write_text(json.dumps(shared_lock))
    assert json.loads(dashboard_copy.read_text())["packages"]["node_modules/next"][
        "version"
    ] == "16.4.1"
    failed = run_checker(tmp_path, configured_entry)
    assert failed.returncode == 1
    assert failed.stdout == "apps/web/package-lock.json\tnext\t16.3.5\t16.4.1\n"
    assert failed.stderr == ""


@pytest.mark.parametrize(
    "left,right,expected",
    [
        ("1.10.0", "1.9.9", 1),
        ("2.0", "10.0", -1),
        ("1.2", "1.2.0.0", 0),
        ("v1.2.3+build.5", "1.2.3+other", 0),
        ("1.2.3-alpha", "1.2.3", -1),
        ("1.2.3-rc.10", "1.2.3-rc.2", 1),
        ("1.2.3-1", "1.2.3-alpha", -1),
        ("1.2.3-alpha", "1.2.3-alpha.1", -1),
        ("1.2.3-beta", "1.2.3-alpha", 1),
        ("1.2.3rc1", "1.2.3", -1),
        ("0.10.0-alpha", "0.9.99", 1),
        ("1.2.3", "1.2.3", 0),
    ],
)
def test_compare_versions(left, right, expected):
    assert MODULE.compare_versions(left, right) == expected
    assert MODULE.compare_versions(right, left) == -expected


@pytest.mark.parametrize("version", ["", "latest", "^1.0.0", "1.*", "1..2"])
def test_compare_versions_rejects_invalid(version):
    with pytest.raises(ValueError, match="invalid version"):
        MODULE.compare_versions(version, "1.0.0")


CASES = [
    (
        "npm", "package.json", "package-lock.json", "@demo/lib", "1.4.0",
        json.dumps({"lockfileVersion": 3, "packages": {
            "": {"name": "root", "version": "0.1.0"},
            "node_modules/@demo/lib": {"version": "1.4.0"},
            "node_modules/parent/node_modules/@demo/lib": {"version": "1.6.0"},
        }}),
    ),
    (
        "yarn", "package.json", "yarn.lock", "@demo/lib", "1.4.0",
        '''
        # yarn lockfile v1
        "@demo/lib@^1.0.0", "@demo/lib@~1.4.0":
          version "1.4.0"
          resolved "https://example.invalid/lib.tgz"
          dependencies:
            helper "^2.0.0"
            version "^3.0.0"

        "@demo/lib@^1.6.0":
          version "1.6.0"
        helper@^2.0.0:
          version "2.0.0"
        ''',
    ),
    (
        "pnpm", "package.json", "pnpm-lock.yaml", "@demo/lib", "1.4.0",
        '''
        lockfileVersion: '9.0'
        importers:
          .:
            dependencies:
              '@demo/lib':
                specifier: ^1.4.0
                version: 1.4.0
        packages:
          '@demo/lib@1.4.0':
            resolution: {integrity: sha512-example}
          '@demo/lib@1.6.0': {}
        snapshots:
          '@demo/lib@1.4.0(peer@2.0.0)': {}
          '@demo/lib@1.6.0': {}
        ''',
    ),
    (
        "go", "go.mod", "go.mod", "example.org/lib", "v1.4.0",
        '''
        module example.org/app
        go 1.23
        require (
            example.org/lib v1.4.0 // indirect
            example.org/other v0.1.0
        )
        require example.org/single v2.0.0
        ''',
    ),
    (
        "cargo", "Cargo.toml", "Cargo.lock", "demo", "1.4.0",
        '''
        version = 4
        [[package]]
        name = "demo"
        version = "1.4.0"
        source = "registry+https://example.invalid/index"
        [[package]]
        name = "demo"
        version = "1.6.0"
        ''',
    ),
    (
        "maven", "pom.xml", "pom.xml", "org.demo:lib", "1.4.0",
        '''
        <project xmlns="http://maven.apache.org/POM/4.0.0">
          <properties>
            <lib.base>1.4.0</lib.base>
            <lib.version>${lib.base}</lib.version>
          </properties>
          <dependencies>
            <dependency><groupId>org.demo</groupId><artifactId>lib</artifactId>
              <version>${lib.version}</version></dependency>
            <dependency><groupId>org.demo</groupId><artifactId>other</artifactId>
              <version>2.0.0</version></dependency>
          </dependencies>
        </project>
        ''',
    ),
    (
        "python", "requirements.txt", "requirements.txt", "demo-lib", "1.4.0",
        '''
        # resolved dependencies
        Demo_Lib[extra]==1.4.0 ; python_version >= "3.11"
        other==2.0.0 # a comment
        ''',
    ),
]


@pytest.mark.parametrize("case", CASES, ids=[case[0] for case in CASES])
def test_supported_formats_pass_and_fail(tmp_path, case):
    _, manifest, lock, package, version, content = case
    manifest = "project/" + manifest
    write(tmp_path, manifest, "{}" if manifest.endswith(".json") else "")
    write(tmp_path, "project/" + lock, content)
    result = run_checker(tmp_path, {manifest: {package: "1.4.0"}})
    assert (result.returncode, result.stdout, result.stderr) == (0, "", "")
    result = run_checker(tmp_path, {manifest: {package: "1.5.0"}})
    assert result.returncode == 1
    assert result.stdout == f"{manifest}\t{package}\t{version}\t1.5.0\n"
    assert result.stderr == ""


def test_package_lock_nested_copy_below_floor(tmp_path):
    write(tmp_path, "package.json", "{}")
    write(tmp_path, "package-lock.json", json.dumps({"packages": {
        "node_modules/lib": {"version": "3.0.0"},
        "node_modules/parent/node_modules/lib": {"version": "1.0.0"},
        "node_modules/other/node_modules/lib": {"version": "1.5.0"},
    }}))
    result = run_checker(tmp_path, {"package.json": {"lib": "2.0.0"}})
    assert result.returncode == 1
    assert result.stdout.splitlines() == [
        "package.json\tlib\t1.0.0\t2.0.0",
        "package.json\tlib\t1.5.0\t2.0.0",
    ]


@pytest.mark.parametrize("block", [False, True])
@pytest.mark.parametrize("specific", [False, True])
def test_go_replace_raises_resolved_version(tmp_path, block, specific):
    replacement = "example.org/lib" + (" v1.0.0" if specific else "")
    replacement += " => example.org/fork v1.8.0"
    directive = f"replace (\n  {replacement}\n)" if block else f"replace {replacement}"
    write(tmp_path, "go.mod", "module example.org/app\nrequire example.org/lib v1.0.0\n" + directive)
    passed = run_checker(tmp_path, {"go.mod": {"example.org/lib": "v1.5.0"}})
    assert (passed.returncode, passed.stdout, passed.stderr) == (0, "", "")
    failed = run_checker(tmp_path, {"go.mod": {"example.org/lib": "v1.9.0"}})
    assert failed.returncode == 1
    assert failed.stdout == "go.mod\texample.org/lib\tv1.8.0\tv1.9.0\n"


def test_go_version_specific_replace_takes_precedence(tmp_path):
    write(tmp_path, "go.mod", '''
        module example.org/app
        require example.org/lib v1.0.0
        replace example.org/lib => example.org/fork v1.1.0
        replace example.org/lib v1.0.0 => example.org/fork v2.0.0
    ''')
    result = run_checker(tmp_path, {"go.mod": {"example.org/lib": "v2.0.0"}})
    assert (result.returncode, result.stdout, result.stderr) == (0, "", "")


def test_go_local_replace_has_no_resolved_version(tmp_path):
    write(tmp_path, "go.mod", '''
        module example.org/app
        require example.org/lib v1.0.0
        replace example.org/lib => ../lib
    ''')
    result = run_checker(tmp_path, {"go.mod": {"example.org/lib": "v1.0.0"}})
    assert result.returncode == 1
    assert "no resolved version for example.org/lib" in result.stderr


@pytest.mark.parametrize("lock", ["package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Cargo.lock"])
def test_workspace_lock_found_in_parent(tmp_path, lock):
    manifest = "members/demo/" + ("Cargo.toml" if lock == "Cargo.lock" else "package.json")
    write(tmp_path, manifest, "{}" if lock != "Cargo.lock" else "")
    contents = {
        "package-lock.json": json.dumps({"packages": {"node_modules/demo": {"version": "2.0.0"}}}),
        "yarn.lock": 'demo@^2:\n  version "2.0.0"\n',
        "pnpm-lock.yaml": "packages:\n  demo@2.0.0: {}\n",
        "Cargo.lock": '[[package]]\nname = "demo"\nversion = "2.0.0"\n',
    }
    write(tmp_path, lock, contents[lock])
    result = run_checker(tmp_path, {manifest: {"demo": "2.0.0"}})
    assert (result.returncode, result.stdout, result.stderr) == (0, "", "")


def test_package_manager_selects_lock(tmp_path):
    write(tmp_path, "package.json", '{"packageManager": "yarn@1.22.22"}')
    write(tmp_path, "yarn.lock", 'demo@^2:\n  version "2.0.0"\n')
    write(tmp_path, "package-lock.json", '{"packages":{"node_modules/demo":{"version":"1.0.0"}}}')
    result = run_checker(tmp_path, {"package.json": {"demo": "2.0.0"}})
    assert (result.returncode, result.stdout, result.stderr) == (0, "", "")


@pytest.mark.parametrize("section", ["packages", "snapshots"])
def test_pnpm_reads_each_section_and_peer_keys(tmp_path, section):
    write(tmp_path, "package.json", "{}")
    write(tmp_path, "pnpm-lock.yaml", f"{section}:\n  demo@1.0.0(peer@3.0.0): {{}}\n  demo@2.0.0: {{}}\n")
    result = run_checker(tmp_path, {"package.json": {"demo": "1.5.0"}})
    assert result.returncode == 1
    assert result.stdout == "package.json\tdemo\t1.0.0\t1.5.0\n"


def test_pom_literal_artifact_and_same_file_properties(tmp_path):
    write(tmp_path, "pom.xml", '''
        <project><properties><minimum>1.5.0</minimum></properties><dependencies>
          <dependency><groupId>org.demo</groupId><artifactId>lib</artifactId>
            <version>1.0.0</version></dependency>
          <dependency><groupId>org.demo</groupId><artifactId>lib</artifactId>
            <version>${minimum}</version></dependency>
        </dependencies></project>
    ''')
    result = run_checker(tmp_path, {"pom.xml": {"lib": "1.5.0"}})
    assert result.returncode == 1
    assert result.stdout == "pom.xml\tlib\t1.0.0\t1.5.0\n"


@pytest.mark.parametrize("property_value", ["${missing}", "${lib.version}"])
def test_pom_unresolved_and_cyclic_properties_fail(tmp_path, property_value):
    write(tmp_path, "pom.xml", f'''<project>
      <properties><lib.version>{property_value}</lib.version></properties>
      <dependencies><dependency><artifactId>lib</artifactId>
        <version>${{lib.version}}</version></dependency></dependencies>
    </project>''')
    result = run_checker(tmp_path, {"pom.xml": {"lib": "1.0.0"}})
    assert result.returncode == 1
    assert "unresolved or cyclic property" in result.stderr


def test_multiple_manifests_report_all_violations(tmp_path):
    write(tmp_path, "first/requirements.txt", "demo==1.0.0\n")
    write(tmp_path, "second/requirements.txt", "other==2.0.0\n")
    result = run_checker(tmp_path, {
        "first/requirements.txt": {"demo": "1.5.0"},
        "second/requirements.txt": {"other": "2.5.0"},
    })
    assert result.returncode == 1
    assert result.stdout.splitlines() == [
        "first/requirements.txt\tdemo\t1.0.0\t1.5.0",
        "second/requirements.txt\tother\t2.0.0\t2.5.0",
    ]


def test_missing_package_does_not_silently_pass(tmp_path):
    write(tmp_path, "requirements.txt", "other==1.0.0\n")
    result = run_checker(tmp_path, {"requirements.txt": {"demo": "1.5.0"}})
    assert result.returncode == 1
    assert result.stdout == "requirements.txt\tdemo\t<unresolved>\t1.5.0\n"


def test_missing_lock_does_not_silently_pass(tmp_path):
    write(tmp_path, "package.json", "{}")
    result = run_checker(tmp_path, {"package.json": {"demo": "1.5.0"}})
    assert result.returncode == 1
    assert "no lock file" in result.stderr


def test_malformed_lock_fails(tmp_path):
    write(tmp_path, "package.json", "{}")
    write(tmp_path, "package-lock.json", "{invalid json")
    result = run_checker(tmp_path, {"package.json": {"demo": "1.5.0"}})
    assert result.returncode == 1
    assert "package.json:" in result.stderr


def test_invalid_floors_file_fails(tmp_path):
    floors = write(tmp_path, "floors.json", '{"manifests": []}')
    result = subprocess.run([sys.executable, str(SCRIPT), str(floors)], capture_output=True, text=True)
    assert result.returncode == 1
    assert "floors file must contain a manifests object" in result.stderr
