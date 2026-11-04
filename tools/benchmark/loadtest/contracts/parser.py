"""Compatibility API for the production benchmark transaction decoder."""
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
from operations_cli import executable


def _identifier(kind, raw_response):
    completed = subprocess.run(
        [executable(), "benchmark-ops", "parse-transaction", kind],
        input=raw_response, text=True, capture_output=True, timeout=60, check=False,
    )
    if completed.returncode:
        raise ValueError(completed.stderr or "transaction identifier rejected")
    return completed.stdout.strip()


def get_code_id(raw_response):
    return int(_identifier("code_id", raw_response))


def get_contract_address(raw_response):
    return _identifier("contract_address", raw_response)


def get_proposal_id(raw_response):
    return int(_identifier("proposal_id", raw_response))


def main():
    readers = {"code_id": get_code_id, "contract_address": get_contract_address,
               "proposal_id": get_proposal_id}
    if len(sys.argv) != 3 or sys.argv[1] not in readers:
        raise ValueError("expected identifier kind and one complete JSON response")
    print(readers[sys.argv[1]](sys.argv[2]))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
