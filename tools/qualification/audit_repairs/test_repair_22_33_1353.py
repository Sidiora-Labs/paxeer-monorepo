"""Prove simd-env retirement from production sources without a toolchain."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
IMAGE = ROOT / "sdk/contrib/images/simd-env"


def read_retirement_targets():
    """Accept only literal rules and recipes; reject hidden make producers."""
    rules = {}
    phony = ()
    active = ()
    for line in (IMAGE.parent / "Makefile").read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        if line.startswith("\t"):
            assert active, "Recipe without a target"
            for target in active:
                rules[target].append(line[1:])
            continue
        declaration = re.fullmatch(r"([A-Za-z.][A-Za-z0-9. -]*): *(.*)", line)
        assert declaration, f"Unexpected make syntax: {line}"
        names = tuple(declaration[1].split())
        if names == (".PHONY",):
            assert not phony, "Duplicate .PHONY declaration"
            phony = tuple(declaration[2].split())
            active = ()
            continue
        assert not declaration[2], "Retired targets must have no prerequisites"
        for target in names:
            assert target not in rules, f"Duplicate target: {target}"
            rules[target] = []
        active = names
    return rules, phony


def test_environment_image_build_retired():
    source = (IMAGE / "Dockerfile").read_text(encoding="utf-8")
    lines = [line.strip() for line in source.splitlines() if line.strip()]
    assert lines, "Keep an explicit retirement notice at the original path"
    # No FROM, RUN, COPY, entrypoint or remote Docker frontend can survive.
    assert all(line.startswith("# ") for line in lines)
    assert not any(re.match(r"#\s*(syntax|escape|check)\s*=", line) for line in lines)
    assert "Retired: inherited Cosmos SDK simd environment image" in source
    assert "no executable Docker instructions" in source
    assert "sdk/go.mod, simapp and build/simd" in source
    assert "Direct docker builds are unsupported" in source
    assert "defines no build stage" in source
    assert "wrapper.sh remains historical simd tooling, not a paxd entrypoint" in source


@pytest.mark.parametrize("target", ("all", "simd-env"))
def test_environment_image_caller_refuses_before_docker(target):
    sdk_makefile = (ROOT / "sdk/Makefile").read_text(encoding="utf-8")
    caller = re.search(
        r"^localnet-build-env:\n((?:\t[^\n]*\n)+)", sdk_makefile, re.MULTILINE
    )
    assert caller, "Check the real upstream consumer"
    assert caller[1] == "\t$(MAKE) -C contrib/images simd-env\n"

    rules, phony = read_retirement_targets()
    assert next(iter(rules)) == "all", "Default invocation must also refuse"
    assert target in phony, "An existing filename must not bypass retirement"
    recipes = rules[target]
    assert len(recipes) >= 2, "Explain retirement before refusing"
    diagnostics = []
    for recipe in recipes[:-1]:
        diagnostic = re.fullmatch(r"@printf '%s\\n' '([^'\n]*)' >&2", recipe)
        assert diagnostic, f"Unexpected command before refusal: {recipe}"
        diagnostics.append(diagnostic[1])
    failure = re.fullmatch(r"@exit ([1-9][0-9]*)", recipes[-1])
    assert failure, "Retirement must propagate failure to the SDK caller"
    assert 1 <= int(failure[1]) <= 255
    explanation = " ".join(diagnostics)
    assert "Retired" in explanation
    assert "inherited Cosmos SDK simd images" in explanation
    assert "sdk/go.mod, simapp and build/simd are absent" in explanation
    assert "For paxd, run make -f chain.mk build from the repository root" in explanation


def test_environment_image_replacement_is_root_paxd_build():
    notice = (IMAGE / "Dockerfile").read_text(encoding="utf-8")
    assert "make -f chain.mk build from the repository root" in notice
    assert "Go 1.25.8" in notice
    assert "chain.mk produces build/paxd" in notice
    module = (ROOT / "go.mod").read_text(encoding="utf-8")
    assert re.search(r"^module github.com/Sidiora-Labs/Paxeer-X-Network$", module, re.M)
    assert re.search(r"^go 1\.25\.8$", module, re.M)
    chain = (ROOT / "chain.mk").read_text(encoding="utf-8")
    build = re.search(r"^build: ([^\n]*)\n((?:\t[^\n]*\n)+)", chain, re.M)
    assert build, "The documented root build target must exist"
    assert build[1].split() == ["paxd-native-libs"]
    assert build[2] == "\tgo build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd\n"
    assert (ROOT / "daemon/paxd/main.go").is_file()
