import base64
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile

import pytest


ROOT = Path(__file__).resolve().parents[2]
RELEASE = ROOT / "tools/release"
HALT = 20


class Rollout:
    def __init__(self, work, version="v8.9.10", upgrade="v8.9.11"):
        self.work = work
        self.version = version
        self.upgrade = upgrade
        self.release = work / "tools/release"
        self.release.mkdir(parents=True)
        for name in ("stage.sh", "swap.sh", "rollback.sh", "upgrade_info.py"):
            shutil.copy2(RELEASE / name, self.release / name)
        (self.release / "env").mkdir()
        shutil.copy2(RELEASE / "env/rollout.env.example", self.release / "env")
        (self.release / "version.json").write_text(
            json.dumps({"version": version, "upgrade": upgrade})
        )
        self.bin = work / "bin"
        self.bin.mkdir()
        self.home = work / "home"
        (self.home / "config").mkdir(parents=True)
        (self.home / "data").mkdir()
        (self.home / "config/app.toml").write_text(f"halt-height = {HALT}\n")
        (self.home / "data/priv_validator_state.json").write_text(
            json.dumps({"height": str(HALT), "round": 0, "step": 0})
        )
        self.snapshot = work / "snapshots"
        self.snapshot.mkdir()
        self.old = b"#!/bin/sh\nexit 0\n"
        self.binary = b"#!/bin/sh\nexit 7\n"
        self.sidecar = b"#!/bin/sh\nexit 8\n"
        self.executable(self.bin / "paxd", self.old)
        self.executable(self.bin / "paxd.pre-rollout", self.old)
        self.executable(self.bin / f"paxd.{version}", self.binary)
        self.executable(work / f"paxd-{version}-linux-amd64", self.binary)
        self.executable(work / f"price-sidecar-{version}-linux-amd64", self.sidecar)
        self.sha = hashlib.sha256(self.binary).hexdigest()
        key = base64.b64encode(bytes([2]) * 32).decode()
        self.plan = {
            "leaders": [key],
            "receipt_keys": [
                {
                    "key_id": key,
                    "public_key": key,
                    "first_height": HALT + 1,
                    "last_height": HALT + 1,
                }
            ],
            "plan_height": HALT,
        }
        self.plan_path = work / "plan info.json"
        self.plan_path.write_text(json.dumps(self.plan))
        self.document = work / "release.json"
        self.write_release(embedded=True)
        self.log = work / "commands.jsonl"
        peers = work / "peers"
        peers.mkdir()
        commands = (
            "python3", "jq", "ldd", "systemctl", "cp", "mv", "install",
            "tar", "zstd", "ssh", "curl",
        )
        real = {name: shutil.which(name) for name in commands}
        peer = peers / "peer"
        self.executable(
            peer,
            (f"#!{sys.executable}\n" + r'''
import json
import os
from pathlib import Path
import subprocess
import sys

name = Path(sys.argv[0]).name
args = sys.argv[1:]
source = sys.stdin.read() if name == "python3" and args[:1] == ["-"] else None
with open(os.environ["ROLLOUT_COMMANDS"], "a") as output:
    output.write(json.dumps({"name": name, "args": args, "source": source}) + "\n")
real = json.loads(os.environ["ROLLOUT_REAL"])
if name in ("ssh", "curl"):
    sys.exit("network access is forbidden in rollout tests")
if name == "ldd":
    print("libc.so.6 => /lib/libc.so.6 (0x1)")
    sys.exit(0)
if name == "systemctl":
    if args[0] == "show":
        print("inactive")
    sys.exit(0)
if name == "python3":
    if source and "mode, home = sys.argv[1:3]" in source and args[1] == "verify":
        sys.exit(0)
    if source and "opener = urllib.request.build_opener" in source:
        sys.exit(0)
    sys.exit(subprocess.run([sys.executable, *args], input=source, text=True).returncode)
if name == "zstd":
    sys.stdout.buffer.write(Path(args[-1]).read_bytes())
    sys.exit(0)
if name == "tar":
    args = [arg for arg in args if arg != "--zstd"]
sys.exit(subprocess.run([real[name], *args]).returncode)
''').encode(),
        )
        for name in commands:
            (peers / name).symlink_to(peer)
        self.env = {
            **os.environ,
            "PATH": f"{peers}:{os.environ['PATH']}",
            "ROLLOUT_COMMANDS": str(self.log),
            "ROLLOUT_REAL": json.dumps(real),
            "BIN_DIR": str(self.bin),
            "RELEASE_SHA256": self.sha,
            "HALT_HEIGHT": str(HALT),
            "VALIDATOR_HOMES": "",
            "VALIDATOR_UNITS": "",
            "FULLNODE_HOMES": str(self.home),
            "FULLNODE_UNITS": "paxd.service",
        }
        keys = re.findall(
            r"^([A-Z_][A-Z0-9_]*)=$",
            (RELEASE / "env/rollout.env.example").read_text(),
            re.M,
        )
        self.inputs = dict.fromkeys(keys, "unused")
        self.inputs.update(
            RELEASE_ID="rollout",
            RELEASE_TAG=f"paxeer-network/{version}",
            RELEASE_SHA256=self.sha,
            SIDECAR_SHA256=hashlib.sha256(self.sidecar).hexdigest(),
            HALT_HEIGHT=str(HALT),
            BIN_DIR=str(self.bin),
            SNAPSHOT_DIR=str(self.snapshot),
            VALIDATOR_HOMES="",
            VALIDATOR_UNITS="",
            FULLNODE_HOMES=str(self.home),
            FULLNODE_UNITS="paxd.service",
            VALIDATOR_HOSTS="",
            FULLNODE_HOSTS="",
            ARCHIVE_HOST="",
            SIDECAR_UNITS="",
        )
        self.env_file = work / "rollout.env"
        self.write_env()

    @staticmethod
    def executable(path, content):
        path.write_bytes(content)
        path.chmod(0o755)

    def write_env(self):
        self.env_file.write_text(
            "".join(f"{key}={value}\n" for key, value in self.inputs.items())
        )

    def write_release(self, embedded):
        info = {
            "binaries": {
                "linux/amd64": (
                    "https://github.com/Sidiora-Labs/paxeer-monorepo/releases/download/"
                    f"paxeer-network/{self.version}/paxd-{self.version}-linux-amd64"
                    f"?checksum=sha256:{self.sha}"
                )
            }
        }
        if embedded:
            info.update(self.plan)
        self.document.write_text(
            json.dumps({"name": self.upgrade, "info": json.dumps(info)})
        )

    def run(self, script, *args):
        return subprocess.run(
            ["bash", str(self.release / script), *map(str, args)],
            cwd=self.work,
            env=self.env,
            capture_output=True,
            text=True,
            timeout=15,
            check=False,
        )

    def swap(self, *args):
        return self.run(
            "swap.sh", "--role", "full", "--release-doc", self.document, *args
        )

    def commands(self, name):
        if not self.log.exists():
            return []
        commands = [json.loads(line) for line in self.log.read_text().splitlines()]
        return [command for command in commands if command["name"] == name]

    def assert_identity_reads(self, runs=1):
        for field in ("version", "upgrade"):
            assert sum(
                command["args"]
                == ["-er", f".{field}", str(self.release / "version.json")]
                for command in self.commands("jq")
            ) == runs

    def node_calls(self):
        return [
            command["args"]
            for command in self.commands("python3")
            if command["args"][:2]
            == [str(self.release / "upgrade_info.py"), "node"]
        ]


@pytest.mark.parametrize(
    "version,upgrade",
    [("v8.9.10", "v8.9.11"), ("v7.0.0", "v7.0-anchor-store-delete")],
)
def test_swap_reads_version_and_upgrade_from_version_json(tmp_path, version, upgrade):
    rollout = Rollout(tmp_path, version, upgrade)
    receipt = tmp_path / "receipt.json"
    result = rollout.swap("--receipt", receipt)
    assert result.returncode == 0, result.stdout + result.stderr
    rollout.assert_identity_reads()
    assert (rollout.bin / "paxd").read_bytes() == rollout.binary
    assert any(
        str(rollout.bin / f"paxd.{version}") in command["args"]
        for command in rollout.commands("cp")
    )
    document = json.loads((rollout.home / "data/upgrade-info.json").read_text())
    assert document["name"] == upgrade
    assert document["height"] == HALT + 1
    assert json.loads(document["info"])["receipt_keys"] == rollout.plan["receipt_keys"]
    assert (rollout.home / "config/app.toml").read_text() == "halt-height = 0\n"
    assert json.loads(receipt.read_text()) == {
        "nodes_checked": 1, "nodes_passed": 1, "result": "pass",
    }
    assert len(rollout.node_calls()) == 1
    assert "--plan-info" not in rollout.node_calls()[0]


def test_stage_requires_tag_from_version_json(tmp_path):
    rollout = Rollout(tmp_path)
    # Stage's complete inventory requires non-empty remote-only values even locally.
    for key in (
        "VALIDATOR_HOSTS", "VALIDATOR_HOMES", "VALIDATOR_UNITS", "ARCHIVE_HOST",
        "SIDECAR_UNITS", "FULLNODE_HOSTS",
    ):
        rollout.inputs[key] = "unused"
    rollout.inputs["RELEASE_TAG"] = "paxeer-network/v6.12.0"
    rollout.write_env()
    result = rollout.run("stage.sh", "--local", "--env-file", rollout.env_file)
    assert result.returncode == 1
    assert f"RELEASE_TAG must be paxeer-network/{rollout.version}" in result.stderr
    assert not rollout.commands("install")
    rollout.inputs["RELEASE_TAG"] = f"paxeer-network/{rollout.version}"
    rollout.write_env()
    result = rollout.run("stage.sh", "--local", "--env-file", rollout.env_file)
    assert result.returncode == 0, result.stdout + result.stderr
    assert (rollout.bin / f"paxd.{rollout.version}").read_bytes() == rollout.binary
    assert (rollout.bin / "price-sidecar").read_bytes() == rollout.sidecar
    assert (rollout.bin / "paxd.pre-rollout").read_bytes() == rollout.old
    installed = [c["args"][3] for c in rollout.commands("install")]
    assert installed == [
        f"./paxd-{rollout.version}-linux-amd64",
        f"./price-sidecar-{rollout.version}-linux-amd64",
    ]
    result = rollout.run("stage.sh", "--local", "--verify", "--env-file", rollout.env_file)
    assert result.returncode == 0, result.stdout + result.stderr
    rollout.assert_identity_reads(runs=3)


def test_rollback_names_use_release_version(tmp_path):
    rollout = Rollout(tmp_path)
    archive = rollout.snapshot / f"home-pre-{rollout.version}.tar.zst"
    with tarfile.open(archive, "w") as output:
        for name, content in {
            "data/committer.db/store": b"snapshot store",
            "data/priv_validator_state.json": (
                rollout.home / "data/priv_validator_state.json"
            ).read_bytes(),
        }.items():
            member = tarfile.TarInfo(name)
            member.size = len(content)
            output.addfile(member, io.BytesIO(content))
    args = ("--local", "--env-file", rollout.env_file)
    result = rollout.run("rollback.sh", *args, "--readiness")
    assert result.returncode == 0, result.stdout + result.stderr
    assert any(str(archive) in c["args"] for c in rollout.commands("tar"))
    assert any(str(archive) in c["args"] for c in rollout.commands("zstd"))
    aside = rollout.home / f"data.{rollout.version}-rollout"
    aside.mkdir()
    result = rollout.run("rollback.sh", *args, "--execute", "--phase", "prepare")
    assert result.returncode == 1
    assert "retained data already exists" in result.stderr
    process_calls = rollout.commands("python3")
    assert any(c["args"] == ["-", "stopped", str(aside)] for c in process_calls)
    aside.rmdir()
    result = rollout.run("rollback.sh", *args, "--execute", "--phase", "prepare")
    assert result.returncode == 0, result.stdout + result.stderr
    rollout.assert_identity_reads(runs=3)


@pytest.mark.parametrize("embedded", [False, True])
def test_swap_forwards_plan_info_for_merge_plan(tmp_path, embedded):
    rollout = Rollout(tmp_path, "v7.0.0", "v7.0")
    rollout.write_release(embedded=embedded)
    arguments = () if embedded else ("--plan-info", rollout.plan_path)
    result = rollout.swap(*arguments)
    assert result.returncode == 0, result.stdout + result.stderr
    rollout.assert_identity_reads()
    calls = rollout.node_calls()
    assert len(calls) == 1
    if embedded:
        assert "--plan-info" not in calls[0]
    else:
        index = calls[0].index("--plan-info")
        assert calls[0][index + 1] == str(rollout.plan_path)
    node_file = rollout.home / "data/upgrade-info.json"
    document = json.loads(node_file.read_text())
    assert document["name"] == "v7.0"
    assert document["height"] == HALT + 1
    info = json.loads(document["info"])
    assert info["plan_height"] == HALT
    assert info["leaders"] == rollout.plan["leaders"]
    assert info["receipt_keys"] == rollout.plan["receipt_keys"]
    assert node_file.stat().st_mode & 0o777 == 0o600
    assert any(
        command["args"] == [
            str(rollout.release / "upgrade_info.py"), "plan-info", "--check",
            str(node_file), "--plan-height", str(HALT),
        ]
        for command in rollout.commands("python3")
    )
    result = rollout.swap("--verify")
    assert result.returncode == 0, result.stdout + result.stderr
    for invalid in ("not json", json.dumps({**info, "plan_height": HALT + 1})):
        document["info"] = invalid
        node_file.write_text(json.dumps(document))
        result = rollout.swap("--verify")
        assert result.returncode == 1
        assert "node verification failed" in result.stderr
    del document["info"]
    node_file.write_text(json.dumps(document))
    result = rollout.swap("--verify")
    assert result.returncode == 1
    assert "merge plan info is missing" in result.stderr


@pytest.mark.parametrize("plan_kind", ["missing", "invalid", "uncovered"])
def test_swap_refuses_merge_plan_without_plan_info(tmp_path, plan_kind):
    rollout = Rollout(tmp_path, "v7.0.0", "v7.0")
    rollout.write_release(embedded=False)
    arguments = ()
    if plan_kind != "missing":
        plan = {} if plan_kind == "invalid" else {
            **rollout.plan,
            "receipt_keys": [{
                **rollout.plan["receipt_keys"][0],
                "first_height": HALT + 2,
                "last_height": HALT + 2,
            }],
        }
        rollout.plan_path.write_text(json.dumps(plan))
        arguments = ("--plan-info", rollout.plan_path)
    result = rollout.swap(*arguments)
    assert result.returncode == 1
    expected = {
        "missing": "--plan-info is required",
        "invalid": "leader list must be non-empty",
        "uncovered": "no receipt key covers sequenced height",
    }
    assert expected[plan_kind] in result.stderr
    assert not rollout.node_calls()
    assert not rollout.commands("systemctl")
    assert (rollout.bin / "paxd").read_bytes() == rollout.old
    assert (rollout.home / "config/app.toml").read_text() == f"halt-height = {HALT}\n"
    assert not (rollout.home / "data/upgrade-info.json").exists()
