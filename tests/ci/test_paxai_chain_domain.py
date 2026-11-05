import hashlib
import os
import pathlib
import re
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]
MERGED_CHAIN_DOMAIN = "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6"


def test_makefile_chain_domain_is_merged_chain_golden():
    expected = hashlib.sha256(
        b"PAXAI/chain-domain/v1" + (125).to_bytes(8, "big")
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
