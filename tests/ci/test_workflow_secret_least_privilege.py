"""Keep integration and remote dApp job secrets tied to their actual readers."""

import json
from pathlib import Path
import re
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = (
    "paxeer-integration-test.yml",
    "paxeer-dapp-tests.yml",
)
UNUSED_MATRIX_SECRETS = {
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "DAPP_TESTS_MNEMONIC",
}
SECRET_REFERENCE = re.compile(r"\bsecrets\s*(?:\.|\[)")
SCRIPT_PATH = re.compile(r"(?:\./)?[\w./-]+\.(?:sh|py|js)\b")


def workflow(name):
    path = ROOT / ".github/workflows" / name
    return yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def command_sources(command, cwd, seen):
    sources = []
    for line in command.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        sources.append(line)
        if line.startswith("cd "):
            directory = shlex.split(line)[1]
            cwd = (cwd / directory).resolve()
        for relative in SCRIPT_PATH.findall(line):
            path = (cwd / relative).resolve()
            if path.is_file() and path not in seen:
                seen.add(path)
                sources.extend(
                    command_sources(path.read_text(encoding="utf-8"), cwd, seen)
                )
        if re.search(r"\bnpx\s+hardhat\b", line):
            config = cwd / "hardhat.config.js"
            assert config.is_file(), f"Missing Hardhat configuration: {config}"
            sources.append(config.read_text(encoding="utf-8"))
    return sources


def job_reader_sources(document, job):
    sources = []
    default_directory = document.get("defaults", {}).get("run", {}).get(
        "working-directory", "."
    )
    directory = job.get("defaults", {}).get("run", {}).get(
        "working-directory", default_directory
    )
    seen = set()
    for step in job["steps"]:
        sources.append(json.dumps(step.get("with", {})))
        sources.append(step.get("if", ""))
        command = step.get("run", "")
        cwd = ROOT / step.get("working-directory", directory)
        sources.extend(command_sources(command, cwd, seen))
        if "matrix.test.scripts" in command:
            for entry in job["strategy"]["matrix"]["test"]:
                for script in entry["scripts"]:
                    sources.extend(command_sources(script, cwd, seen))
    return "\n".join(sources)


def reads_environment(source, name):
    name = re.escape(name)
    return any(
        re.search(pattern, source)
        for pattern in (
            rf"\$(?:{name}\b|\{{{name}(?:\}}|[:?+\-]))",
            rf"\benv\s*\.\s*{name}\b",
            rf"\bprocess\.env(?:\.{name}\b|\[['\"]{name}['\"]\])",
            rf"\bos\.(?:environ\[['\"]{name}['\"]\]|"
            rf"(?:getenv|environ\.get)\(['\"]{name}['\"])",
        )
    )


def test_integration_tests_job_has_no_unused_secrets():
    document = workflow("paxeer-integration-test.yml")
    job = document["jobs"]["integration-tests"]
    assert job["strategy"]["matrix"]["test"], "Integration matrix must still run"
    assert job["env"]["JOB_TIMEOUT"] == "28m"
    assert UNUSED_MATRIX_SECRETS.isdisjoint(job["env"])
    assert UNUSED_MATRIX_SECRETS.isdisjoint(document.get("env", {}))
    for name in UNUSED_MATRIX_SECRETS:
        assert name not in json.dumps(document), f"Unused integration secret: {name}"


def test_job_env_secrets_are_read():
    checked = set()
    for filename in WORKFLOWS:
        document = workflow(filename)
        assert document["jobs"], f"No jobs inspected in {filename}"
        for job_id, job in document["jobs"].items():
            secrets = {
                name
                for name, value in job.get("env", {}).items()
                if SECRET_REFERENCE.search(value)
            }
            if not secrets:
                continue
            source = job_reader_sources(document, job)
            for name in secrets:
                assert reads_environment(source, name), (
                    f"{filename}:{job_id} exposes {name} without a reader "
                    "in its steps, invoked scripts or configurations"
                )
                checked.add((filename, job_id, name))
    assert {
        ("paxeer-dapp-tests.yml", f"run-tests-{network}", "DAPP_TESTS_MNEMONIC")
        for network in ("devnet", "testnet")
    } <= checked, "Both remote dApp jobs must retain a verified mnemonic reader"


def test_remote_dapp_jobs_keep_required_mnemonic():
    document = workflow("paxeer-dapp-tests.yml")
    for network in ("devnet", "testnet"):
        job = document["jobs"][f"run-tests-{network}"]
        assert job["environment"]["name"] == network
        assert job["env"]["DAPP_TESTS_MNEMONIC"] == (
            "${{ secrets.DAPP_TESTS_MNEMONIC }}"
        )
        assert any(
            f"./tests/integration/dapp_tests/dapp_tests.sh {network}"
            in step.get("run", "")
            for step in job["steps"]
        ), f"The mnemonic must only reach a remote dApp run: {network}"


def test_paxlocal_accounts_do_not_depend_on_remote_secret():
    config = (ROOT / "tests/integration/dapp_tests/hardhat.config.js").read_text(
        encoding="utf-8"
    )
    local = re.search(r"paxlocal:\s*\{(.*?)\n    \},", config, re.DOTALL)
    assert local, "The integration scripts require the paxlocal network"
    accounts = local.group(1)
    assert "process.env.DAPP_TESTS_MNEMONIC" not in accounts
    assert re.search(r"accounts:\s*\{\s*mnemonic:", accounts)
    assert '"test test test test test test test test test test test junk"' in accounts
    assert 'path: "m/44\'/118\'/0\'/0/0"' in accounts
    assert "initialIndex: 0" in accounts
    assert "count: 1" in accounts
    remote = re.search(r"function remoteNetwork\(url\) \{(.*?)\n\}", config, re.DOTALL)
    assert remote and reads_environment(remote.group(1), "DAPP_TESTS_MNEMONIC")
    for network in ("DEVNET", "TESTNET"):
        assert f"remoteNetwork(process.env.PAXEER_{network}_EVM_RPC_URL)" in config
