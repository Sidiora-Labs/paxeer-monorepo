"""Launch the fixture's shared, bounded HTTP and bank-process implementation."""
import os
from pathlib import Path


def main():
    # One implementation serves Python, shell and native clients. A missing
    # binary fails startup rather than silently serving an unfunded success.
    os.execvp("rosetta-fixture", [
        "rosetta-fixture", "serve",
        "--config", str(Path(__file__).resolve().parent),
        "--listen", "0.0.0.0:8000",
    ])


if __name__ == "__main__":
    main()
