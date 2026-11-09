#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
cd "$root"

run_workflow_checks() {
	python3 - "$@" <<'PY'
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path

import yaml


check_path = Path(".github/workflows/paxeer-release-check.yml")
publish_path = Path(".github/workflows/paxeer-release-publish.yml")
check = yaml.safe_load(check_path.read_text())
publish = yaml.safe_load(publish_path.read_text())
identity = json.loads(Path("tools/release/version.json").read_text())


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def step(workflow, job, name):
    matches = [
        (index, entry)
        for index, entry in enumerate(workflow["jobs"][job]["steps"])
        if entry.get("name") == name
    ]
    require(len(matches) == 1, f"{job}: expected one {name} step")
    index, entry = matches[0]
    require("if" not in entry, f"{name}: must run unconditionally")
    require(not entry.get("continue-on-error"), f"{name}: must fail the job")
    return index, entry["run"].strip()


def triggers(workflow):
    return workflow["on"] if "on" in workflow else workflow[True]


def installs_rust(workflow, job, build_name, build_script):
    install_index, install = step(workflow, job, "Install pinned Rust toolchain")
    build_index, build = step(workflow, job, build_name)
    require(install_index < build_index, f"{job}: Rust installation follows build")
    require(build == build_script, f"{job}: unexpected build command {build!r}")
    require(
        install == "rustup toolchain install 1.91.1 --profile minimal --no-self-update",
        f"{job}: Rust installation must use the pinned minimal toolchain",
    )
    require(
        re.search(
            r'^channel\s*=\s*"1\.91\.1"\s*$',
            Path("rust-toolchain.toml").read_text(),
            re.M,
        ),
        "rust-toolchain.toml disagrees with the workflow pin",
    )


def check_installs_rust():
    installs_rust(
        check,
        "release-check",
        "Check reproducible release build",
        "tools/release/reproducible-check.sh",
    )


def publish_installs_rust():
    installs_rust(
        publish, "releaser", "Build release binaries", "tools/release/build-paxd.sh"
    )


def check_pins_cc():
    scripts = []
    for workflow, job, build_name in (
        (check, "release-check", "Check reproducible release build"),
        (publish, "releaser", "Build release binaries"),
    ):
        install_index, install = step(workflow, job, "Install native build dependencies")
        build_index, _ = step(workflow, job, build_name)
        require(install_index < build_index, f"{job}: compiler installation follows build")
        fragments = (
            "set -euo pipefail",
            ". tools/release/toolchain.env",
            "${CC_VERSION:?tools/release/toolchain.env must pin CC_VERSION}",
            'cc_package="gcc-${CC_VERSION%%.*}"',
            "sudo apt-get update",
            'apt-cache madison "$cc_package"',
            'awk -v version="$CC_VERSION"',
            'index(candidate, version "-") == 1 { print $3; exit }',
            'if [[ -z "$cc_package_version" ]]; then',
            "sudo apt-get install -y --no-install-recommends build-essential libssl-dev libsqlite3-dev",
            '"$cc_package=$cc_package_version" "g++-${CC_VERSION%%.*}=$cc_package_version"',
            'ln -sf "/usr/bin/$cc_package" "$compiler_bin/cc"',
            'ln -sf "/usr/bin/$cc_package" "$compiler_bin/gcc"',
            'ln -sf "/usr/bin/g++-${CC_VERSION%%.*}" "$compiler_bin/c++"',
            'ln -sf "/usr/bin/g++-${CC_VERSION%%.*}" "$compiler_bin/g++"',
            'export PATH="$compiler_bin:$PATH"',
            'echo "$compiler_bin" >> "$GITHUB_PATH"',
            "have_cc=$(cc -dumpfullversion)",
            "printf '%s\\n' \"$have_cc\"",
            'if [[ "$have_cc" != "$CC_VERSION" ]]; then',
            "build-paxd: toolchain.env pins cc $CC_VERSION, found $have_cc",
        )
        positions = []
        for fragment in fragments:
            require(fragment in install, f"{job}: compiler setup is missing {fragment!r}")
            positions.append(install.index(fragment))
        require(positions == sorted(positions), f"{job}: compiler setup is out of order")
        require(
            re.search(
                r'if \[\[ -z "\$cc_package_version" \]\]; then\n[^\n]*\n\s*exit 1\n\s*fi',
                install,
            ),
            f"{job}: unavailable compiler must fail installation",
        )
        pin_check = install[install.index("have_cc=$(cc -dumpfullversion)") :]
        have_cc = (
            subprocess.run(
                ["cc", "-dumpfullversion"], capture_output=True, text=True, check=True
            )
            .stdout.strip()
        )
        for expected, code in ((have_cc, 0), ("0.0.0", 1)):
            result = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", pin_check],
                env={**os.environ, "CC_VERSION": expected},
                capture_output=True,
                text=True,
                check=False,
            )
            require(
                result.returncode == code,
                f"{job}: compiler pin returned {result.returncode}, expected {code}",
            )
            require(
                result.stdout == f"{have_cc}\n",
                f"{job}: compiler version was not printed",
            )
            error = (
                f"build-paxd: toolchain.env pins cc {expected}, found {have_cc}\n"
                if code
                else ""
            )
            require(
                result.stderr == error,
                f"{job}: unexpected compiler refusal {result.stderr!r}",
            )
        scripts.append(install)
    require(scripts[0] == scripts[1], "release workflows install different C toolchains")
    pin = subprocess.run(
        [
            "bash",
            "-euo",
            "pipefail",
            "-c",
            '. tools/release/toolchain.env; printf "%s" "${CC_VERSION:?}"',
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    require(
        pin.returncode == 0,
        f"cannot load CC_VERSION from task 8.4.2: {pin.stderr.strip()}",
    )
    require(
        re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", pin.stdout),
        "CC_VERSION must pin a full compiler version",
    )


def publish_title_names_paxeer_x():
    _, script = step(publish, "releaser", "Publish GitHub release")
    require(
        '--title "Paxeer X Network $version"' in script,
        "release title must name Paxeer X Network",
    )
    require(
        'echo "### Paxeer X Network release $RELEASE_TAG"' in script,
        "release summary must name Paxeer X Network",
    )
    require(
        "Paxeer Network" not in publish_path.read_text(),
        "publish workflow retains the retired product name",
    )
    require(
        '"dist/$RELEASE_TAG"/*' in script,
        "publication must retain every release artifact",
    )


def publish_tag_must_match_version():
    require(
        triggers(publish)["push"]["tags"] == ["paxeer-network/v*"],
        "publish tag trigger changed",
    )
    validate_index, script = step(
        publish, "releaser", "Validate tag against tools/release/version.json"
    )
    build_index, _ = step(publish, "releaser", "Build release binaries")
    require(validate_index < build_index, "tag must be validated before building")
    require(identity["version"] == "v7.0.0", "release version must be v7.0.0")
    for tag, code, output in (
        ("paxeer-network/v7.0.0", 0, ""),
        (
            "paxeer-network/v7.0.1",
            1,
            "Tag paxeer-network/v7.0.1 does not match tools/release/version.json v7.0.0\n",
        ),
        (
            "v7.0.0",
            1,
            "Tag v7.0.0 does not match tools/release/version.json v7.0.0\n",
        ),
    ):
        result = subprocess.run(
            ["bash", "-c", script],
            env={**os.environ, "RELEASE_TAG": tag},
            capture_output=True,
            text=True,
            check=False,
        )
        require(
            result.returncode == code,
            f"{tag}: exit {result.returncode}, expected {code}",
        )
        require(
            result.stdout == output,
            f"{tag}: unexpected tag validation output {result.stdout!r}",
        )
        require(result.stderr == "", f"{tag}: unexpected stderr {result.stderr!r}")


def check_paths_cover_native_inputs():
    required = {
        "src/**",
        "include/**",
        "Makefile",
        "rust-toolchain.toml",
        "programs/**",
        "services/platform/Cargo.toml",
        "services/platform/Cargo.lock",
        "services/platform/hosted/feeder/**",
        "tools/price-sidecar/**",
        ".github/workflows/paxeer-release-publish.yml",
        "tools/release/version.json",
        "go.mod",
        "go.sum",
        "chain.mk",
        "daemon/**",
        "modules/**",
        "precompiles/**",
        "node/**",
        "sdk/**",
        "wasm-runtime/**",
        "tools/release/**",
        ".github/workflows/paxeer-release-check.yml",
    }
    paths = triggers(check)["pull_request"]["paths"]
    require(
        required <= set(paths),
        f"missing release PR paths: {sorted(required - set(paths))}",
    )


def identity_greps_registered_upgrade_constant():
    _, script = step(check, "release-check", "Validate version and namespaced tag")
    matches = re.findall(
        r"^if ! (grep [^\n]+); then$", script.replace("\\\n", ""), re.M
    )
    require(len(matches) == 1, "identity must check one upgrade declaration")
    require(identity["upgrade"] == "v7.0", "upgrade must be v7.0")
    pipeline = matches[0]
    source_paths = ("node/upgrades.go", "precompiles/common/precompiles.go")
    require(
        shlex.split(pipeline)
        == [
            "grep",
            "-hE",
            "^[[:space:]]*const (MergeUpgrade|AnchorStoreDeleteUpgrade|V6120Upgrade)[[:space:]]*=",
            *source_paths,
            "|",
            "grep",
            "-qF",
            "--",
            '= "$upgrade"',
        ],
        "identity must restrict the pipeline to registered declaring lines",
    )

    def assert_pipeline(upgrade, code, directory=None):
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", pipeline],
            cwd=directory,
            env={**os.environ, "upgrade": upgrade},
            capture_output=True,
            text=True,
            check=False,
        )
        require(
            result.returncode == code,
            f"{upgrade!r}: declaration pipeline exited {result.returncode}, expected {code}",
        )
        require(result.stdout == "", f"unexpected pipeline output {result.stdout!r}")
        require(result.stderr == "", f"unexpected pipeline stderr {result.stderr!r}")

    assert_pipeline(identity["upgrade"], 0)
    for upgrade in ("v7.0-unregistered", "v999.0", ""):
        assert_pipeline(upgrade, 1)

    declaring_line = re.compile(
        r"^\s*const (MergeUpgrade|AnchorStoreDeleteUpgrade|V6120Upgrade)\s*=.*$",
        re.M,
    )
    with tempfile.TemporaryDirectory(prefix="release-upgrade-declarations-") as directory:
        for source_path in source_paths:
            source = Path(source_path).read_text()
            require(
                declaring_line.search(source),
                f"{source_path}: expected a registered upgrade declaration",
            )
            without_declarations = declaring_line.sub("", source)
            decoys = (
                '\n// const MergeUpgrade = "v7.0"\n'
                '// Reference to the plan: = "v7.0"\n'
                'const UnrelatedUpgrade = "v7.0"\n'
                'var UpgradeReference = "v7.0"\n'
            )
            mutated_path = Path(directory, source_path)
            mutated_path.parent.mkdir(parents=True, exist_ok=True)
            mutated_path.write_text(without_declarations + decoys)
        assert_pipeline(identity["upgrade"], 1, directory)


tests = (
    check_installs_rust,
    publish_installs_rust,
    check_pins_cc,
    publish_title_names_paxeer_x,
    publish_tag_must_match_version,
    check_paths_cover_native_inputs,
    identity_greps_registered_upgrade_constant,
)
registered = {test.__name__: test for test in tests}
selectors = sys.argv[1:]
for selector in selectors:
    if not selector or selector not in registered:
        print(f"Unknown or empty workflow check: {selector!r}", file=sys.stderr)
        sys.exit(2)
selected = [registered[selector] for selector in selectors] if selectors else tests

passed = 0
failed = 0
for test in selected:
    try:
        test()
    except (AssertionError, OSError, KeyError, subprocess.SubprocessError) as error:
        print(f"FAIL {test.__name__}: {error}")
        failed += 1
    else:
        print(f"PASS {test.__name__}")
        passed += 1
print(f"workflows: {passed} passed, {failed} failed")
sys.exit(1 if failed else 0)
PY
}

publish_tag_must_match_version() {
	run_workflow_checks publish_tag_must_match_version "$@"
}

identity_greps_registered_upgrade_constant() {
	run_workflow_checks identity_greps_registered_upgrade_constant "$@"
}

case "${1:-}" in
publish_tag_must_match_version | identity_greps_registered_upgrade_constant)
	selected_check=$1
	shift
	"$selected_check" "$@"
	;;
*) run_workflow_checks "$@" ;;
esac
