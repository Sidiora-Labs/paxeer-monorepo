from pathlib import Path
import re
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
ATTESTOR = ROOT / "services/wallet/attestor"
SYSTEM_LIBRARIES = {"ssl", "crypto", "dl", "m"}
VARIABLE = re.compile(r"\$\(([A-Za-z_][A-Za-z_0-9]*)\)")


def _expand(value, variables):
    seen = set()
    while value not in seen:
        seen.add(value)
        expanded = VARIABLE.sub(
            lambda match: variables.get(match[1], match[0]), value
        )
        if expanded == value:
            return value
        value = expanded
    raise AssertionError(f"Cyclic make variable expansion: {value}")


def _make_outputs(source, target):
    source = source.replace("\\\n", " ")
    variables = dict(
        re.findall(r"^([A-Za-z_][A-Za-z_0-9]*)\s*[:?+]?=\s*(.*)$", source, re.M)
    )
    rules = list(
        re.finditer(
            r"^([^\s#:=][^:=\n]*):(?![=])[^\S\n]*([^\n]*)\n((?:\t[^\n]*\n)*)",
            source,
            re.M,
        )
    )
    outputs = set()
    visited = set()

    def visit(name):
        if name in visited:
            return
        visited.add(name)
        matching = [
            rule for rule in rules
            if name in _expand(rule[1], variables).split()
        ]
        assert matching, f"No Makefile rule produces {name}"
        for rule in matching:
            archives = {
                item for item in _expand(rule[1], variables).split()
                if item.endswith(".a")
            }
            if archives:
                assert rule[3].strip(), f"No build recipe for {archives}"
                outputs.update(archives)
            else:
                for prerequisite in _expand(rule[2], variables).split():
                    visit(prerequisite)

    visit(target)
    return {ROOT / path for path in outputs}


def test_attestor_job_builds_every_linked_archive():
    workflow = yaml.load(
        (ROOT / ".github/workflows/wallet-go-test.yml").read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )
    job = workflow["jobs"]["attestor"]
    produced = set()
    prerequisites = []
    reached_tests = False
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    for step in job["steps"]:
        script = step.get("run", "")
        if re.search(r"\bgo test\b", script):
            reached_tests = True
            break
        prerequisites.append(script)
        for line in script.splitlines():
            command = shlex.split(line, comments=True)
            if command[:1] != ["make"]:
                continue
            assert "if" not in step and "continue-on-error" not in step
            directory = step.get(
                "working-directory", job["defaults"]["run"]["working-directory"]
            )
            assert (ROOT / directory).resolve() == ROOT
            assert len(command) > 1 and all(
                re.fullmatch(r"[A-Za-z_0-9./-]+", target)
                and not target.startswith("-")
                for target in command[1:]
            ), f"Expected explicit, unconditional make targets: {command}"
            for target in command[1:]:
                produced.update(_make_outputs(makefile, target))
    assert reached_tests, "The attestor job must run go test"
    assert produced, "No native archive build runs before the attestor tests"
    assert "libssl-dev" in "\n".join(prerequisites)
    assert "build-essential" in "\n".join(prerequisites)

    linked = set()
    for source in sorted(ATTESTOR.rglob("*.go")):
        for directive in re.findall(
            r"^\s*#cgo\s+(?:[^:\n]*\s+)?LDFLAGS:\s*([^\n]*)",
            source.read_text(encoding="utf-8"),
            re.M,
        ):
            flags = shlex.split(directive)
            directories = [
                Path(flag[2:].replace("${SRCDIR}", str(source.parent))).resolve()
                for flag in flags if flag.startswith("-L")
            ]
            for flag in flags:
                if not flag.startswith("-l") or flag[2:] in SYSTEM_LIBRARIES:
                    continue
                archive = f"lib{flag[2:]}.a"
                linked.add(archive)
                candidates = {directory / archive for directory in directories}
                assert candidates & produced, (
                    f"{source.relative_to(ROOT)} links {archive}, but no make "
                    f"target run before go test builds it in {directories}"
                )
    assert {"liblayerx.a", "liblayerx_programs_sandbox.a"} <= linked

    chain = (ROOT / "chain.mk").read_text(encoding="utf-8")
    inventory = re.search(r"^NATIVE_ARCHIVES\s*:=\s*(.+)$", chain, re.M)
    assert inventory is not None
    archives = {
        Path(path.replace("$(PAXEER_PROJECT_HOME)", str(ROOT)))
        for path in inventory[1].split()
    }
    assert archives <= produced, "The wallet job must build the chain archive inventory"


def test_attestor_abi_copies_match_precompiles():
    canonical = ROOT / "precompiles/layerxpay/abi.json"
    embedded = ATTESTOR / "internal/policy/evm/abi/layerxpay.json"
    assert embedded.read_bytes() == canonical.read_bytes(), (
        f"{embedded.relative_to(ROOT)} differs from {canonical.relative_to(ROOT)}"
    )
