"""Source qualification of the inherited image targets, without Docker or make."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
MAKEFILE = ROOT / "sdk/contrib/images/Makefile"
TARGETS = ("all", "simd-env", "simd-dlv", "simd-rmi")


def read_retired_rules():
    """Parse every line; reject prerequisites, includes and make expansions.

    The retirement file needs only ordinary rules, literal diagnostic recipes
    and .PHONY. Rejecting other syntax ensures a hidden producer cannot run
    before the refusal. Shell recipes are checked separately below.
    """
    rules = {}
    phony = ()
    active = ()
    for line in MAKEFILE.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        if line.startswith("\t"):
            assert active, "Recipe must belong to an ordinary target"
            for target in active:
                rules[target].append(line[1:])
            continue
        declaration = re.fullmatch(r"([A-Za-z.][A-Za-z0-9. -]*): *(.*)", line)
        assert declaration, f"Unexpected executable Makefile syntax: {line}"
        names = tuple(declaration[1].split())
        prerequisites = declaration[2].split()
        if names == (".PHONY",):
            assert not phony, "Duplicate .PHONY declaration"
            phony = tuple(prerequisites)
            active = ()
        else:
            assert not prerequisites, "Retired targets must not invoke prerequisites"
            for target in names:
                assert target not in rules, f"Duplicate target: {target}"
                rules[target] = []
            active = names
    return rules, phony


@pytest.mark.parametrize("target", TARGETS)
def test_image_build_context_retired(target):
    rules, phony = read_retired_rules()
    assert tuple(rules) == TARGETS, "Default all and both image consumers must retire"
    assert set(phony) == set(TARGETS), "Existing filenames must not bypass refusal"

    recipes = rules[target]
    assert len(recipes) >= 2, "Explain retirement before failing"
    diagnostics = []
    for recipe in recipes[:-1]:
        # A literal, single-quoted printf cannot execute Docker, substitutions
        # or another tool. >&2 makes the refusal visible to callers on stderr.
        diagnostic = re.fullmatch(r"@printf '%s\\n' '([^'\n]*)' >&2", recipe)
        assert diagnostic, f"Unexpected command before refusal: {recipe}"
        diagnostics.append(diagnostic[1])
    failure = re.fullmatch(r"@exit ([1-9][0-9]*)", recipes[-1])
    assert failure, "Retired targets must fail rather than report a successful build"
    assert 1 <= int(failure[1]) <= 255

    explanation = " ".join(diagnostics)
    assert "Retired" in explanation
    assert "inherited Cosmos SDK simd images" in explanation
    assert "sdk/go.mod, simapp and build/simd are absent" in explanation
    assert "both Dockerfiles are historical tooling" in explanation
    assert "For paxd, run make -f chain.mk build from the repository root" in explanation
