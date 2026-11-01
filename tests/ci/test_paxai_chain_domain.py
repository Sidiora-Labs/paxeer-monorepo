import hashlib
import os
import pathlib
import re
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]
MERGED_CHAIN_DOMAIN = "b69143b1aa0c360367c299a2a47bbbfe3546ecbdc0a78c640770d49d977f038d"


def test_makefile_chain_domain_is_merged_chain_golden():
    expected = hashlib.sha256(
        b"PAXAI/chain-domain/v1" + (229).to_bytes(8, "big")
    ).hexdigest()
    assert expected == MERGED_CHAIN_DOMAIN
    env = {**os.environ, "LC_ALL": "C"}
    env.pop("PAXAI_CHAIN_DOMAIN", None)
    for override in (None, "01" * 32):
        command = ["make", "-s", "-p", "-n"]
        if override is not None:
            command.append(f"PAXAI_CHAIN_DOMAIN={override}")
        result = subprocess.run(
            command,
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, result.stdout + result.stderr
        values = re.findall(
            r"^PAXAI_CHAIN_DOMAIN\s*[:?+!]?=\s*(\S+)\s*$", result.stdout, re.M
        )
        assert values, "make database has no PAXAI_CHAIN_DOMAIN assignment"
        assert all(re.fullmatch(r"[0-9a-f]{64}", value) for value in values)
        assert set(values) == {expected if override is None else override}

    makefile = (ROOT / "Makefile").read_text()
    assert re.search(r"^PAXAI_CHAIN_DOMAIN\s*\?=", makefile, re.M)
    assert makefile.count("$(PAXAI_CHAIN_DOMAIN)") == 3
