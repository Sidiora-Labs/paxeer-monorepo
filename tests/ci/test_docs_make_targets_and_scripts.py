"""Keep documented Make targets and sample npm scripts aligned with their owners."""

import json
from pathlib import Path
import re
import shlex


ROOT = Path(__file__).resolve().parents[2]
NEXT_PAGE = ROOT / "docs/platform/content/framework/next.md"
MAKE_PAGES = (ROOT / "wasm/contrib/devtools/README.md", NEXT_PAGE)
SAMPLE = ROOT / "services/platform/examples/samples/paid-route-next"


def test_doc_make_targets_exist():
    commands = []
    for page in MAKE_PAGES:
        content = page.read_text(encoding="utf-8")
        for command in re.finditer(
            r"\bmake[ \t]+-C[ \t]+([^\s`]+)[ \t]+([\w.-]+)", content
        ):
            directory, target = command.groups()
            makefile = ROOT / directory / "Makefile"
            assert makefile.is_file(), f"{page}: missing {makefile}"
            targets = {
                name
                for declaration in re.findall(
                    r"^([^\s#:=][^:=\n]*):(?!=)",
                    makefile.read_text(encoding="utf-8"),
                    re.MULTILINE,
                )
                for name in declaration.split()
            }
            assert target in targets, f"{page}: {target} is not a target in {makefile}"
            commands.append((directory, target))
    assert commands, "Expected documented make -C commands"


def test_doc_package_scripts_exist():
    content = NEXT_PAGE.read_text(encoding="utf-8")
    directory = None
    commands = []
    for line in content.splitlines():
        change_directory = re.match(r"^[ \t]*cd[ \t]+([^\s`]+)", line)
        if change_directory:
            directory = ((directory or ROOT) / change_directory.group(1)).resolve()
        for command in re.finditer(r"\bnpm[ \t]+run[ \t]+([\w:-]+)", line):
            assert directory is not None, f"{NEXT_PAGE}: npm command needs a directory"
            manifest = directory / "package.json"
            package = json.loads(manifest.read_text(encoding="utf-8"))
            script = command.group(1)
            assert script in package["scripts"], f"{manifest}: missing script {script}"
            commands.append((directory, script))
    assert commands, "Expected documented npm run commands"
    assert (SAMPLE, "dev") in commands

    package = json.loads((SAMPLE / "package.json").read_text(encoding="utf-8"))
    assert package["scripts"]["dev"] == "next dev"
    predev = shlex.split(package["scripts"]["predev"])
    assert predev == [
        "npm",
        "--prefix",
        "../../../../..",
        "run",
        "build:dependencies",
        "--",
        package["name"],
    ]
    assert package["scripts"]["predev"] == package["scripts"]["prebuild"]
    dependency_root = (SAMPLE / predev[2]).resolve()
    assert dependency_root == ROOT
    root_package = json.loads(
        (dependency_root / "package.json").read_text(encoding="utf-8")
    )
    assert predev[4] in root_package["scripts"]
    assert "`predev`" in content and "`build:dependencies`" in content
