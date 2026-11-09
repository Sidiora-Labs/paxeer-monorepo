from fnmatch import fnmatch
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[2]
MIRROR = Path("services/interop/contracts/ethereum-mirror")


def _recipe_commands(target):
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    recipe = re.search(
        rf"^{re.escape(target)}:[^\n]*\n((?:\t[^\n]*\n|\n)*)",
        makefile,
        re.MULTILINE,
    )
    assert recipe is not None, f"Missing Makefile target: {target}"
    return [
        shlex.split(line[1:])
        for line in recipe[1].replace("\\\n", "").splitlines()
        if line.startswith("\t")
    ]


def test_ci_forge_step_uses_test_contracts():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
    )
    steps = workflow["jobs"]["solidity"]["steps"]
    commands = [step.get("run", "").strip() for step in steps]
    assert commands.count("make test-contracts") == 1
    assert commands.count("make interop-test-mirrors") == 1
    assert commands.index("make test-contracts") < commands.index(
        "make interop-test-mirrors"
    )
    for command in ("make test-contracts", "make interop-test-mirrors"):
        step = steps[commands.index(command)]
        assert "if" not in step
        assert not step.get("continue-on-error", False)
        assert "working-directory" not in step
    assert "forge test -vvv" not in commands
    assert _recipe_commands("test-contracts") == [
        [
            f"FOUNDRY_PROFILE={profile}", "forge", "test", "--offline",
            "--root", ".", "--no-dynamic-test-linking", "-vvv",
        ]
        for profile in ("names", "governance")
    ]
    mirror_commands = _recipe_commands("interop-test-mirrors")
    assert mirror_commands[-1] == [
        "forge", "test", "--root", str(MIRROR), "--evm-version", "paris",
        "--no-dynamic-test-linking", "-vvv",
    ]
    assert len(mirror_commands) == 3, "Preserve the Rust and Solana mirror suites"


def test_default_foundry_profile_has_test_dir():
    config = tomllib.loads((ROOT / "foundry.toml").read_text(encoding="utf-8"))
    default = config["profile"]["default"]
    assert default["test"] == "tests/solidity"
    suites = sorted((ROOT / default["test"]).rglob("*.t.sol"))
    assert len(suites) >= 6
    for suite in suites:
        relative = suite.relative_to(ROOT).as_posix()
        assert not any(fnmatch(relative, pattern) for pattern in default["skip"])
        assert re.search(r"function\s+test\w*\s*\(", suite.read_text())


def test_named_profiles_point_at_existing_dirs():
    config = tomllib.loads((ROOT / "foundry.toml").read_text(encoding="utf-8"))
    for name, minimum in (("names", 5), ("governance", 1)):
        profile = config["profile"][name]
        assert profile["test"] == f"tests/solidity/{name}"
        assert (ROOT / profile["src"]).is_dir()
        assert len(list((ROOT / profile["test"]).glob("*.t.sol"))) >= minimum
    mirror = tomllib.loads(
        (ROOT / MIRROR / "foundry.toml").read_text(encoding="utf-8")
    )["profile"]["default"]
    assert mirror["test"] == "test"
    assert (ROOT / MIRROR / mirror["src"]).is_dir()
    assert mirror["solc_version"] == "0.8.30"
    assert mirror["ffi"] is True, "The publisher recovery suite needs real FFI"
    suites = {path.name for path in (ROOT / MIRROR / mirror["test"]).glob("*.t.sol")}
    assert {"LayerXMirrorArchive.t.sol", "Repair2231231.t.sol"} <= suites
