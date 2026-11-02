from pathlib import Path
import re
import subprocess


def test_out_of_scope_omits_localnode_dockerfile():
    script_path = Path(__file__).resolve().with_name("check.sh")
    script = script_path.read_text(encoding="utf-8")
    match = re.search(r"^OUT_OF_SCOPE='\n(.*?)\n'$", script, re.MULTILINE | re.DOTALL)
    assert match is not None, "check.sh must declare the OUT_OF_SCOPE list"
    out_of_scope = match.group(1).splitlines()

    deleted_dockerfile = "consensus/networks/local/localnode/Dockerfile"
    assert deleted_dockerfile not in out_of_scope

    result = subprocess.run(
        ["bash", "-euc", match.group(0) + '\nprintf "%s\\n" "$OUT_OF_SCOPE"'],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert result.stdout.strip().splitlines() == out_of_scope
    assert deleted_dockerfile not in result.stdout.splitlines()
