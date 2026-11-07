import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import stat
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[2]
RELEASE = ROOT / "tools/release"
EXAMPLE = RELEASE / "upgradeinfo/merge-plan-info.example.json"
HALT = 42


class Rehearsal:
    def __init__(self, directory, version="v7.0.0", upgrade="v7.0"):
        self.directory = directory
        self.release = directory / "tools/release"
        self.release.mkdir(parents=True)
        for name in (
            "rehearse-v6120.sh", "upgrade_info.py", "fleet_check.py", "feed_key.py",
            "fleet-views.json", "snapshot.sh", "rollback.sh",
            "rehearsal/rewrite_genesis.py", "rehearsal/pool_parity.py",
            "rehearsal/programme.py", "rehearsal/receipt.sh",
        ):
            destination = self.release / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(RELEASE / name, destination)
        (self.release / "version.json").write_text(
            json.dumps({"version": version, "upgrade": upgrade})
        )
        self.source = directory / "source data"
        self.source.mkdir()
        (self.source / "state").write_bytes(b"copied chain state must remain unchanged")
        self.genesis = directory / "source genesis.json"
        self.genesis.write_text("{}\n")
        self.work = directory / "rehearsal work"
        self.plan_path = directory / "plan info.json"
        shutil.copy2(EXAMPLE, self.plan_path)
        peers = directory / "bin"
        peers.mkdir()
        forbidden = (
            "#!/bin/sh\n"
            'printf "%s\\n" "$0 $*" >> "$REHEARSAL_FORBIDDEN"\n'
            "exit 97\n"
        )
        self.old_bin = peers / "previous-paxd"
        self.new_bin = peers / f"paxd-{version}-linux-amd64"
        for path in (
            self.old_bin, self.new_bin, peers / "systemctl", peers / "curl",
            peers / "pgrep", peers / "zstd",
        ):
            path.write_text(forbidden)
            path.chmod(0o755)
        (peers / "id").write_text("#!/bin/sh\n[ \"$1\" = -u ] || exit 97\necho 0\n")
        (peers / "id").chmod(0o755)
        self.forbidden = directory / "forbidden-commands.log"
        self.env = {
            **os.environ,
            "PATH": f"{peers}:{os.environ['PATH']}",
            "REHEARSAL_FORBIDDEN": str(self.forbidden),
            "PYTHONDONTWRITEBYTECODE": "1",
        }

    def run(self, command):
        return subprocess.run(
            list(map(str, command)), cwd=self.directory, env=self.env,
            capture_output=True, text=True, timeout=20, check=False,
        )

    def snapshot(self):
        return {
            str(path.relative_to(self.directory)): (
                stat.S_IMODE(path.stat().st_mode),
                path.read_bytes() if path.is_file() else None,
            )
            for path in self.directory.rglob("*")
        }

    def check(self, *options, halt=HALT):
        command = [
            "bash", self.release / "rehearse-v6120.sh", "--check",
            "--source-data", self.source, "--source-genesis", self.genesis,
            "--old-bin", self.old_bin.name, "--new-bin", self.new_bin.name,
            "--work", self.work,
        ]
        if halt is not None:
            command += ["--halt-height", halt]
        before = self.snapshot()
        result = self.run([*command, *options])
        assert self.snapshot() == before, "--check changed files or directories"
        assert not self.forbidden.exists(), "--check invoked a node, service or network peer"
        assert not self.work.exists(), "--check created the rehearsal homes"
        return result

    @staticmethod
    def commands(result, label):
        return [
            shlex.split(line)[1:]
            for line in result.stdout.splitlines()
            if line.startswith(label + " ")
        ]

    @staticmethod
    def option(command, flag):
        return command[command.index(flag) + 1]


@pytest.mark.parametrize(
    "version,upgrade",
    [("v8.9.10", "v8.9.11"), ("v7.0.0", "v7.0-anchor-store-delete")],
)
def test_rehearsal_plan_name_comes_from_version_json(tmp_path, version, upgrade):
    rehearsal = Rehearsal(tmp_path, version, upgrade)
    result = rehearsal.check()
    assert result.returncode == 0, result.stdout + result.stderr
    assert f"plan {upgrade}, version {version}" in result.stderr
    commands = rehearsal.commands(result, "node-upgrade-command")
    assert len(commands) == 4
    assert rehearsal.commands(result, "plan-info-command") == []
    for i, command in enumerate(commands):
        assert rehearsal.option(command, "--name") == upgrade
        assert rehearsal.option(command, "--home") == str(rehearsal.work / f"node{i}")
        assert "--plan-info" not in command
        written = rehearsal.run(command)
        assert written.returncode == 0, written.stdout + written.stderr
        document = json.loads(
            (rehearsal.work / f"node{i}/data/upgrade-info.json").read_text()
        )
        assert document == {"name": upgrade, "height": HALT + 1}
    assert not rehearsal.forbidden.exists()


@pytest.mark.parametrize(
    "invalid,upgrade",
    [
        (invalid, "v7.0")
        for invalid in (
            "missing", "unreadable", "malformed", "leaders", "receipt_keys",
            "coverage", "height", "sequence",
        )
    ] + [
        ("malformed", "v7.0-anchor-store-delete"),
        ("coverage", "v7.0-anchor-store-delete"),
    ],
)
def test_rehearsal_check_requires_plan_info_for_merge(tmp_path, invalid, upgrade):
    rehearsal = Rehearsal(tmp_path, upgrade=upgrade)
    options = []
    if invalid != "missing":
        plan = json.loads(rehearsal.plan_path.read_text())
        if invalid == "unreadable":
            rehearsal.plan_path.unlink()
        elif invalid == "malformed":
            rehearsal.plan_path.write_text("{broken")
        else:
            if invalid in ("leaders", "receipt_keys"):
                plan[invalid] = []
            elif invalid == "coverage":
                for key in plan["receipt_keys"]:
                    key["first_height"] = HALT + 2
            elif invalid == "height":
                plan["plan_height"] = HALT + 1
            elif invalid == "sequence":
                plan["last_seq"] = 1
            rehearsal.plan_path.write_text(json.dumps(plan))
        options = ["--plan-info", rehearsal.plan_path]
    result = rehearsal.check(*options)
    assert result.returncode != 0, result.stdout + result.stderr
    assert "--plan-info" in result.stderr
    assert rehearsal.commands(result, "node-upgrade-command") == []
    assert rehearsal.commands(result, "plan-info-command") == []


@pytest.mark.parametrize("halt", [None, HALT])
def test_rehearsal_check_accepts_example_plan_info(tmp_path, halt):
    rehearsal = Rehearsal(tmp_path)
    result = rehearsal.check("--plan-info", rehearsal.plan_path, halt=halt)
    assert result.returncode == 0, result.stdout + result.stderr
    assert "check passed (plan v7.0, version v7.0.0" in result.stderr
    assert rehearsal.plan_path.read_bytes() == EXAMPLE.read_bytes()
    assert len(rehearsal.commands(result, "node-upgrade-command")) == (0 if halt is None else 4)


@pytest.mark.parametrize("halt,wrapped", [(HALT, False), (317, True)])
def test_rehearsal_passes_plan_info_to_node_files(tmp_path, halt, wrapped):
    rehearsal = Rehearsal(tmp_path)
    example = json.loads(EXAMPLE.read_text())
    source_command = [
        "python3", rehearsal.release / "upgrade_info.py", "plan-info",
        "--plan-height", halt, "--activation-height", halt + 1,
        "--out", rehearsal.plan_path,
    ]
    for leader in example["leaders"]:
        source_command += ["--leader", leader]
    for key in example["receipt_keys"]:
        source_command += ["--receipt-key", json.dumps(key)]
    produced = rehearsal.run(source_command)
    assert produced.returncode == 0, produced.stdout + produced.stderr
    source = json.loads(rehearsal.plan_path.read_text())
    checksum = hashlib.sha256(rehearsal.new_bin.read_bytes()).hexdigest()
    sums = tmp_path / "SHA256SUMS"
    sums.write_text(f"{checksum}  {rehearsal.new_bin.name}\n")
    release_doc = tmp_path / "release.json"
    released = rehearsal.run([
        "python3", rehearsal.release / "upgrade_info.py", "release",
        "--sums", sums, "--plan-info", rehearsal.plan_path, "--out", release_doc,
    ])
    assert released.returncode == 0, released.stdout + released.stderr
    if wrapped:
        shutil.copy2(release_doc, rehearsal.plan_path)
    result = rehearsal.check("--plan-info", rehearsal.plan_path, halt=halt)
    assert result.returncode == 0, result.stdout + result.stderr
    plan_commands = rehearsal.commands(result, "plan-info-command")
    assert len(plan_commands) == 1
    plan_command = plan_commands[0]
    assert plan_command[:3] == ["python3", str(rehearsal.release / "upgrade_info.py"), "plan-info"]
    assert rehearsal.option(plan_command, "--plan-height") == str(halt)
    generated_path = rehearsal.work / "merge-plan-info.json"
    assert rehearsal.option(plan_command, "--out") == str(generated_path)
    generated = rehearsal.run(plan_command)
    assert generated.returncode == 0, generated.stdout + generated.stderr
    plan = json.loads(generated_path.read_text())
    assert plan == source
    commands = rehearsal.commands(result, "node-upgrade-command")
    assert len(commands) == 4
    for i, command in enumerate(commands):
        assert rehearsal.option(command, "--plan-info") == str(generated_path)
        assert rehearsal.option(command, "--halt-height") == str(halt)
        assert rehearsal.option(command, "--name") == "v7.0"
        home = rehearsal.work / f"node{i}"
        assert rehearsal.option(command, "--home") == str(home)
        written = rehearsal.run([
            *command, "--release-doc", release_doc, "--sha256", checksum,
        ])
        assert written.returncode == 0, written.stdout + written.stderr
        path = home / "data/upgrade-info.json"
        document = json.loads(path.read_text())
        assert document["name"] == "v7.0"
        assert document["height"] == halt + 1
        info = json.loads(document["info"])
        assert {key: info[key] for key in plan} == plan
        release_info = json.loads(json.loads(release_doc.read_text())["info"])
        assert info["binaries"] == release_info["binaries"]
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert not rehearsal.forbidden.exists()
