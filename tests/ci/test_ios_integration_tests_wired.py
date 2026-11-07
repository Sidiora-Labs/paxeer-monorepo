"""Keep the full iOS integration test suite in the platform middleware recipe."""

from pathlib import Path
import re
import shlex


ROOT = Path(__file__).resolve().parents[2]


def test_platform_recipe_runs_ios_swift_test():
    makefile = (ROOT / "services/platform/Makefile.inc").read_text()
    recipe = re.search(
        r"^platform-test-middleware:[^\n]*\n((?:\t[^\n]*\n|\n)*)",
        makefile,
        re.MULTILINE,
    )
    assert recipe is not None, "The platform middleware test recipe must exist"
    commands = [
        shlex.split(line[1:])
        for line in recipe[1].replace("\\\n", "").splitlines()
        if line.startswith("\t")
    ]
    swift_test = ["swift", "test", "--package-path", "sdk/integrations/ios"]
    assert commands.count(swift_test) == 1, (
        "The platform middleware recipe must run the entire iOS test suite once "
        "and propagate failures"
    )
    swift_build = [
        "swift", "build", "--disable-automatic-resolution",
        "--package-path", "sdk/integrations/ios",
    ]
    assert commands.index(swift_build) < commands.index(swift_test)
