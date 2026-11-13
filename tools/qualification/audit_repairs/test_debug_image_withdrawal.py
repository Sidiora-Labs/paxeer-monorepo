"""Prove withdrawal of the obsolete debug image without Docker or a toolchain."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]
IMAGE = ROOT / "sdk/contrib/images/simd-dlv"
MAKEFILE = IMAGE.parent / "Makefile"
TARGETS = ("all", "simd-env", "simd-dlv", "simd-rmi")


def test_debug_image_build_recipe_withdrawn():
    """The direct Docker consumer can no longer read the invalid producer."""
    recipe = IMAGE / "Dockerfile"
    assert not recipe.exists(), "The obsolete image recipe must be withdrawn"
    assert not recipe.is_symlink(), "A dangling recipe link is not withdrawal"
    assert {path.name for path in IMAGE.iterdir()} == {"README.md", "wrapper.sh"}


def test_debug_image_build_entrypoints_refuse():
    """Parse the real make consumers and every command before their refusal."""
    rules = {}
    phony = ()
    active = ()
    for line in MAKEFILE.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        if line.startswith("\t"):
            assert active, "Every recipe must belong to an ordinary target"
            for target in active:
                rules[target].append(line[1:])
            continue
        declaration = re.fullmatch(r"([A-Za-z.][A-Za-z0-9. -]*): *(.*)", line)
        assert declaration, f"Unexpected executable make syntax: {line}"
        names = tuple(declaration[1].split())
        prerequisites = tuple(declaration[2].split())
        if names == (".PHONY",):
            assert not phony, "Duplicate .PHONY declaration"
            phony = prerequisites
            active = ()
        else:
            assert not prerequisites, "No producer may run before retirement"
            for target in names:
                assert target not in rules, f"Duplicate target: {target}"
                rules[target] = []
            active = names

    assert tuple(rules) == TARGETS, "The default and explicit consumers must retire"
    assert len(phony) == len(TARGETS)
    assert set(phony) == set(TARGETS), "A filename must not bypass refusal"
    for target in TARGETS:
        recipes = rules[target]
        assert len(recipes) >= 2, "Retirement must explain why the build is refused"
        diagnostics = []
        for recipe in recipes[:-1]:
            diagnostic = re.fullmatch(r"@printf '%s\\n' '([^'\n]*)' >&2", recipe)
            assert diagnostic, f"Unexpected command before refusal: {recipe}"
            diagnostics.append(diagnostic[1])
        failure = re.fullmatch(r"@exit ([1-9][0-9]*)", recipes[-1])
        assert failure, "A retired image build must exit nonzero"
        assert 1 <= int(failure[1]) <= 255
        explanation = " ".join(diagnostics)
        assert "Retired" in explanation
        assert "inherited Cosmos SDK simd images" in explanation
        assert "sdk/go.mod, simapp and build/simd are absent" in explanation
        assert "make -f chain.mk build from the repository root" in explanation


def test_debug_image_build_documentation_discloses_withdrawal():
    readme = (IMAGE / "README.md").read_text(encoding="utf-8")
    assert "`simd` Delve image is withdrawn" in readme
    assert "Its `Dockerfile`\nhas been removed" in readme
    assert "[`sdk/contrib/images/Makefile`](../Makefile)" in readme
    assert "retirement diagnostic and a nonzero exit" in readme
    assert "`make -f chain.mk build` from the repository root" in readme
    assert "does not provide a supported paxd debug image" in readme
    assert "inherited tooling, without an image build recipe" in readme
